import { useMatches, Link } from '@tanstack/react-router'
import { useQueries } from '@tanstack/react-query'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '#/components/ui/breadcrumb'
import { Fragment } from 'react'
import { iamClients } from '#/api'

interface BreadcrumbSegment {
  label: string
  href: string
}

const ROUTE_LABELS: Record<string, string> = {
  dashboard: '概览',
  organizations: '组织',
  applications: '应用',
  users: '用户',
  tenants: '租户',
  settings: '设置',
  members: '成员管理',
  profile: '个人信息',
  security: '安全',
  roles: '角色管理',
  permissions: '权限码管理',
  menus: '菜单管理',
  platform: '平台管理',
}

/** Map paramName → { queryKey, queryFn } for entity name resolution */
function getEntityQueryConfig(paramName: string, paramValue: string) {
  switch (paramName) {
    case 'orgId':
      return {
        queryKey: ['organization', paramValue] as const,
        queryFn: () => iamClients.organization.GetOrganization({ id: paramValue }),
      }
    case 'userId':
      return {
        queryKey: ['user', paramValue] as const,
        queryFn: () => iamClients.user.GetUser({ id: paramValue }),
      }
    case 'tenantId':
      return {
        queryKey: ['tenant', paramValue] as const,
        queryFn: () => iamClients.tenant.GetTenant({ id: paramValue }),
      }
    case 'appId':
      return {
        queryKey: ['application', paramValue] as const,
        queryFn: () => iamClients.application.GetApplication({ id: paramValue }),
      }
    default:
      return null
  }
}

function extractEntityName(paramName: string, data: unknown): string | undefined {
  if (!data) return undefined
  switch (paramName) {
    case 'orgId': {
      const d = data as { organization?: { displayName?: string; name?: string } }
      return d.organization?.displayName || d.organization?.name || undefined
    }
    case 'userId': {
      const d = data as { user?: { name?: string } }
      return d.user?.name || undefined
    }
    case 'tenantId': {
      const d = data as { tenant?: { displayName?: string; name?: string } }
      return d.tenant?.displayName || d.tenant?.name || undefined
    }
    case 'appId': {
      const d = data as { application?: { name?: string } }
      return d.application?.name || undefined
    }
    default:
      return undefined
  }
}

/** Determine if a URL segment looks like a UUID or other dynamic ID */
function looksLikeDynamicId(segment: string): boolean {
  // UUID v4 pattern
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(segment)
}

export function AppBreadcrumb() {
  const matches = useMatches()

  const leafMatch = matches.length > 0 ? matches[matches.length - 1] : null

  // Use actual resolved pathname (e.g., /organizations/some-uuid/members)
  // instead of routeId which may use {param} or $param syntax depending on TanStack Router version.
  const pathname = leafMatch?.pathname ?? ''
  const params = (leafMatch?.params ?? {}) as Record<string, string>

  // Build a reverse map: paramValue → paramName
  // e.g., { 'some-uuid': 'orgId' }
  const valueToParamName = new Map(
    Object.entries(params)
      .filter(([, v]) => v && looksLikeDynamicId(v))
      .map(([k, v]) => [v, k]),
  )

  // Split pathname into logical path segments (strip leading slash)
  const pathParts = pathname.split('/').filter(Boolean)

  // Collect unique dynamic segments for entity queries
  const dynamicSegmentsMap = new Map<string, { paramName: string; paramValue: string }>()
  pathParts.forEach((segment) => {
    const paramName = valueToParamName.get(segment)
    if (paramName && !dynamicSegmentsMap.has(paramName)) {
      dynamicSegmentsMap.set(paramName, { paramName, paramValue: segment })
    }
  })
  const dynamicSegments = [...dynamicSegmentsMap.values()]

  // Fetch entity display names — deduped with queries already made by the page
  const entityQueries = useQueries({
    queries: dynamicSegments.map(({ paramName, paramValue }) => {
      const config = getEntityQueryConfig(paramName, paramValue)
      if (!config || !paramValue) {
        return {
          queryKey: ['_noop', paramName, paramValue] as const,
          queryFn: (): null => null,
          enabled: false,
        }
      }
      return { ...config, enabled: true, staleTime: 60_000 }
    }),
  })

  // Map: paramValue → resolved display name
  const entityNameByValue = new Map<string, string>()
  dynamicSegments.forEach(({ paramName, paramValue }, idx) => {
    const name = extractEntityName(paramName, entityQueries[idx]?.data)
    if (name) {
      entityNameByValue.set(paramValue, name)
    }
  })

  // Build breadcrumb segments from the actual URL parts
  const segments: BreadcrumbSegment[] = []
  let accumulatedPath = ''

  for (const part of pathParts) {
    accumulatedPath += '/' + part

    const isDynamic = valueToParamName.has(part)
    const label = isDynamic
      ? (entityNameByValue.get(part) ?? part.slice(0, 8)) // fallback to truncated ID while loading
      : (ROUTE_LABELS[part] ?? part)

    segments.push({ label, href: accumulatedPath })
  }

  if (segments.length === 0) return null

  return (
    <Breadcrumb>
      <BreadcrumbList>
        {segments.map((seg, idx) => (
          <Fragment key={seg.href}>
            {idx > 0 && <BreadcrumbSeparator />}
            <BreadcrumbItem>
              {idx < segments.length - 1 ? (
                <BreadcrumbLink asChild>
                  <Link to={seg.href as '/'}>{seg.label}</Link>
                </BreadcrumbLink>
              ) : (
                <BreadcrumbPage>{seg.label}</BreadcrumbPage>
              )}
            </BreadcrumbItem>
          </Fragment>
        ))}
      </BreadcrumbList>
    </Breadcrumb>
  )
}
