import { useEffect, useState } from 'react'
import { useStore } from '@tanstack/react-store'
import { useQuery } from '@tanstack/react-query'
import { useNavigate, useLocation } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import {
  LayoutDashboard,
  Building2,
  AppWindow,
  Users,
} from 'lucide-react'
import { TooltipProvider } from '#/components/ui/tooltip'
import { Sidebar } from '#/layout/sidebar'
import type { MenuItem } from '#/layout/sidebar'
import { Header } from '#/layout/header'
import { Tabbar } from '#/layout/tabbar'
import { authStore, clearAuth, setUser } from '#/stores/auth'
import {
  scopeStore,
  setCurrentTenantId,
  setCurrentOrganizationId,
  clearScope,
} from '#/stores/scope'
import {
  accessStore,
  setPermissionCodes,
  setAccessMenus,
  setAccessLoaded,
  clearAccess,
} from '#/stores/access'
import { layoutStore, SIDEBAR_WIDTH, SIDEBAR_COLLAPSED_WIDTH } from '#/stores/layout'
import { addTab, resetTabs } from '#/stores/tabbar'
import { iamClients } from '#/api'
import { getIcon } from '#/lib/icon-registry'
import type { rbacservicev1_MenuInfo } from '@servora/api-client/iam/service/v1/index'

// Fallback static menu (used while dynamic menus are loading or as a baseline)
const FALLBACK_MENU_MAIN: MenuItem[] = [
  { label: '概览', href: '/dashboard', icon: LayoutDashboard },
  {
    label: '组织管理',
    icon: Building2,
    children: [{ label: '组织列表', href: '/organizations' }],
  },
  {
    label: '应用管理',
    icon: AppWindow,
    children: [{ label: '应用列表', href: '/applications' }],
  },
  {
    label: '用户管理',
    icon: Users,
    children: [{ label: '用户列表', href: '/users' }],
  },
]


// Icon name for a menu item is stored in meta JSON: { icon: "LayoutDashboard", ... }
function extractIcon(menu: rbacservicev1_MenuInfo) {
  if (!menu.meta) return undefined
  try {
    const parsed = JSON.parse(menu.meta) as { icon?: string }
    return getIcon(parsed.icon)
  } catch {
    return undefined
  }
}

/**
 * Resolve path template variables using scope values.
 * e.g., "/organizations/{orgId}/members" + { orgId: "abc" } → "/organizations/abc/members"
 * If a required variable is missing (empty), the path is returned as-is (will render as placeholder).
 */
function resolvePathTemplate(path: string, vars: Record<string, string>): string {
  return path.replace(/\{(\w+)\}/g, (match, key: string) => vars[key] || match)
}

/**
 * Convert a flat list of backend menus into a MenuItem[] tree.
 * Backend already returns them in a nested structure (children array).
 * Filter to only CATALOG and MENU types (not BUTTON).
 * Resolves path template variables (e.g., {orgId}) using provided scope vars.
 */
function buildMenuTree(menus: rbacservicev1_MenuInfo[], scopeVars: Record<string, string>): MenuItem[] {
  return menus
    .filter((m) => m.type === 'CATALOG' || m.type === 'MENU')
    .map((m): MenuItem | null => {
      const children = m.children
        ?.filter((c) => c.type === 'MENU' && c.path)
        .map((c) => ({
          label: c.name ?? '',
          href: resolvePathTemplate(c.path ?? '', scopeVars),
          icon: extractIcon(c),
        }))
      // Skip CATALOG nodes that have no visible children — prevents orphan group headers
      if (m.type === 'CATALOG' && (!children || children.length === 0)) return null
      const rawPath = m.type === 'MENU' && !children?.length ? (m.path ?? undefined) : undefined
      return {
        label: m.name ?? '',
        href: rawPath ? resolvePathTemplate(rawPath, scopeVars) : undefined,
        icon: extractIcon(m),
        children: children && children.length > 0 ? children : undefined,
      }
    })
    .filter((m): m is MenuItem => m !== null)
}

/**
 * Split menu tree into main and settings groups.
 * Convention: sort < 10 = main menu groups, sort >= 10 = secondary groups (settings, etc.)
 * All menus are shown; visibility is controlled entirely by backend permission codes.
 */
function splitMenuGroups(menus: rbacservicev1_MenuInfo[], scopeVars: Record<string, string>): [MenuItem[], MenuItem[]] {
  const topLevel = menus.filter((m) => m.parentId == null)
  const main = topLevel.filter((m) => (m.sort ?? 0) < 10)
  const secondary = topLevel.filter((m) => (m.sort ?? 0) >= 10)
  return [buildMenuTree(main, scopeVars), buildMenuTree(secondary, scopeVars)]
}

const ROUTE_TITLES: Record<string, string> = {
  '/dashboard': '概览',
  '/organizations': '组织列表',
  '/applications': '应用列表',
  '/users': '用户列表',
  '/settings/profile': '个人设置',
  '/settings/security': '安全设置',
  '/settings/roles': '角色管理',
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const user = useStore(authStore, (s) => s.user)
  const collapsed = useStore(layoutStore, (s) => s.sidebarCollapsed)
  const currentTenantId = useStore(scopeStore, (s) => s.currentTenantId)
  const currentOrgId = useStore(scopeStore, (s) => s.currentOrganizationId)
  const accessMenus = useStore(accessStore, (s) => s.accessMenus)
  const navigate = useNavigate()
  const location = useLocation()

  const [scopeReady, setScopeReady] = useState(() => !!scopeStore.state.currentTenantId)

  useEffect(() => {
    resetTabs({ path: '/dashboard', title: '概览' })
  }, [])

  // Fetch current user info on mount if not yet loaded
  useEffect(() => {
    if (user || !authStore.state.accessToken) return
    iamClients.user
      .CurrentUserInfo({})
      .then((info) => {
        setUser({ id: info.id ?? '', name: info.name ?? '', email: info.email ?? '', role: info.role ?? '' })
      })
      .catch(() => {})
  }, [user])

  // Auto-fetch tenant on mount; mark scope ready after resolution.
  useEffect(() => {
    if (currentTenantId) {
      setScopeReady(true)
      return
    }
    if (!authStore.state.accessToken) {
      setScopeReady(true)
      return
    }
    iamClients.tenant
      .ListTenants({ pagination: { page: { page: 1, pageSize: 100 } } })
      .then((res) => {
        const firstId = res.tenants?.[0]?.id
        if (firstId) setCurrentTenantId(firstId)
      })
      .catch(() => {})
      .finally(() => setScopeReady(true))
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Fetch permission codes and navigation once scope is ready
  useEffect(() => {
    if (!scopeReady || !authStore.state.accessToken) return
    Promise.all([
      iamClients.rbac.GetMyPermissionCodes({}),
      iamClients.rbac.GetNavigation({}),
    ])
      .then(([permRes, navRes]) => {
        setPermissionCodes(permRes.codes ?? [])
        setAccessMenus(navRes.menus ?? [])
        setAccessLoaded(true)
      })
      .catch(() => {
        // Keep access empty on error; fallback menus will be shown
        setAccessLoaded(true)
      })
  }, [scopeReady])

  // Clear access store on logout / scope clear
  useEffect(() => {
    if (!currentTenantId) {
      clearAccess()
    }
  }, [currentTenantId])

  // Org query
  const { data: orgs } = useQuery({
    queryKey: ['organizations', 'list-for-switcher', currentTenantId],
    queryFn: () =>
      iamClients.organization.ListOrganizations({
        pagination: { page: { page: 1, pageSize: 100 } },
      }),
    enabled: !!currentTenantId,
    staleTime: 60_000,
  })

  const orgItems =
    orgs?.organizations?.map((o) => ({ id: o.id ?? '', name: o.displayName || o.name || '' })) ?? []

  useEffect(() => {
    if (currentOrgId || orgItems.length === 0) return
    setCurrentOrganizationId(orgItems[0].id)
  }, [currentOrgId, orgItems])

  function handleLogout() {
    clearAuth()
    clearScope()
    clearAccess()
    void navigate({ to: '/login' as string })
  }

  // Sync route to tabbar
  useEffect(() => {
    const path = location.pathname
    const baseMatch = Object.keys(ROUTE_TITLES).find(
      (k) => path === k || path.startsWith(`${k}/`),
    )
    if (baseMatch) {
      addTab({ path: baseMatch, title: ROUTE_TITLES[baseMatch] })
    } else {
      addTab({ path, title: path.split('/').filter(Boolean).pop() ?? path })
    }
  }, [location.pathname])

  // Scope variables for resolving path templates like {orgId}, {tenantId}
  const scopeVars: Record<string, string> = {
    orgId: currentOrgId ?? '',
    tenantId: currentTenantId ?? '',
  }

  // Build dynamic menus or fall back to static ones
  const [menuMain, menuSettings] =
    accessMenus.length > 0
      ? splitMenuGroups(accessMenus, scopeVars)
      : [FALLBACK_MENU_MAIN, []]

  const sidebarWidth = collapsed ? SIDEBAR_COLLAPSED_WIDTH : SIDEBAR_WIDTH

  return (
    <TooltipProvider>
      <div className="min-h-dvh bg-background-deep">
        <Sidebar
          title="Servora IAM"
          titleHref="/dashboard"
          menuGroups={[menuMain, menuSettings]}
        />

        <div
          className="flex min-h-dvh flex-col transition-all duration-200"
          style={{ marginLeft: sidebarWidth }}
        >
          <Header user={user} onLogout={handleLogout} />
          <Tabbar />
          <main className="flex-1 p-4">
            {scopeReady ? children : (
              <div className="flex h-48 items-center justify-center">
                <Loader2 className="size-5 animate-spin text-muted-foreground" />
              </div>
            )}
          </main>
        </div>
      </div>
    </TooltipProvider>
  )
}
