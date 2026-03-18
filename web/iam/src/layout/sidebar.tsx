import { useState } from 'react'
import { Link, useLocation } from '@tanstack/react-router'
import { useStore } from '@tanstack/react-store'
import { ChevronsLeft, ChevronsRight, ChevronRight, Minus } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { cn } from '#/lib/utils'
import { ScrollArea } from '#/components/ui/scroll-area'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '#/components/ui/tooltip'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '#/components/ui/collapsible'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '#/components/ui/popover'
import {
  layoutStore,
  toggleSidebar,
  SIDEBAR_WIDTH,
  SIDEBAR_COLLAPSED_WIDTH,
} from '#/stores/layout'

export interface SubMenuItem {
  label: string
  href: string
  icon?: LucideIcon
}

export interface MenuItem {
  label: string
  /** If omitted the item is a parent-only group trigger */
  href?: string
  /** Optional icon; falls back to a neutral icon if omitted */
  icon?: LucideIcon
  children?: SubMenuItem[]
}

interface SidebarProps {
  title: string
  titleHref: string
  menuGroups: MenuItem[][]
  bottom?: React.ReactNode
}

// ─── Leaf item (no children) ────────────────────────────────────────────────

function LeafItem({
  item,
  collapsed,
  pathname,
}: {
  item: MenuItem & { href: string }
  collapsed: boolean
  pathname: string
}) {
  const active = pathname === item.href || pathname.startsWith(`${item.href}/`)
  const Icon = item.icon ?? Minus

  const link = (
    <Link
      to={item.href}
      className={cn(
        'flex items-center gap-3 rounded-lg px-3 text-[13px] font-medium transition-colors',
        'h-[36px]',
        collapsed && 'justify-center px-0',
        active
          ? 'bg-sidebar-accent text-sidebar-accent-foreground'
          : 'text-sidebar-foreground/80 hover:bg-accent hover:text-sidebar-foreground',
      )}
    >
      <Icon className="size-4 shrink-0" />
      {!collapsed && <span className="truncate">{item.label}</span>}
    </Link>
  )

  if (collapsed) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>{link}</TooltipTrigger>
        <TooltipContent side="right" sideOffset={8}>
          {item.label}
        </TooltipContent>
      </Tooltip>
    )
  }
  return link
}

// ─── Sub-item row ────────────────────────────────────────────────────────────

function SubItem({ item, pathname }: { item: SubMenuItem; pathname: string }) {
  const active = pathname === item.href || pathname.startsWith(`${item.href}/`)
  const Icon = item.icon
  return (
    <Link
      to={item.href}
      className={cn(
        'flex h-[34px] items-center rounded-lg pl-10 pr-3 text-[13px] transition-colors',
        active
          ? 'text-primary font-medium'
          : 'text-sidebar-foreground/70 hover:bg-accent hover:text-sidebar-foreground',
      )}
    >
      {Icon ? (
        <Icon
          className={cn(
            'mr-2 size-3.5 shrink-0 transition-colors',
            active ? 'text-primary' : 'text-sidebar-foreground/40',
          )}
        />
      ) : (
        <span
          className={cn(
            'mr-2 size-1.5 rounded-full shrink-0 transition-colors',
            active ? 'bg-primary' : 'bg-sidebar-foreground/30',
          )}
        />
      )}
      {item.label}
    </Link>
  )
}

// ─── Group item (has children) ───────────────────────────────────────────────

function GroupItem({
  item,
  collapsed,
  pathname,
}: {
  item: MenuItem
  collapsed: boolean
  pathname: string
}) {
  const Icon = item.icon ?? Minus
  const children = item.children ?? []
  const isChildActive = children.some(
    (c) => pathname === c.href || pathname.startsWith(`${c.href}/`),
  )
  const [open, setOpen] = useState(isChildActive)

  if (collapsed) {
    return (
      <Popover>
        <Tooltip>
          <TooltipTrigger asChild>
            <PopoverTrigger asChild>
              <button
                type="button"
                className={cn(
                  'flex h-[36px] w-full items-center justify-center rounded-lg px-0 transition-colors',
                  isChildActive
                    ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                    : 'text-sidebar-foreground/80 hover:bg-accent hover:text-sidebar-foreground',
                )}
              >
                <Icon className="size-4 shrink-0" />
              </button>
            </PopoverTrigger>
          </TooltipTrigger>
          <TooltipContent side="right" sideOffset={8}>
            {item.label}
          </TooltipContent>
        </Tooltip>
        <PopoverContent side="right" align="start" sideOffset={4} className="w-44 p-1">
          <p className="px-2 py-1 text-xs font-semibold text-muted-foreground">{item.label}</p>
          {children.map((child) => {
            const active = pathname === child.href || pathname.startsWith(`${child.href}/`)
            return (
              <Link
                key={child.href}
                to={child.href}
                className={cn(
                  'flex h-8 items-center rounded-md px-2 text-[13px] transition-colors',
                  active
                    ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium'
                    : 'text-foreground/80 hover:bg-accent',
                )}
              >
                {child.label}
              </Link>
            )
          })}
        </PopoverContent>
      </Popover>
    )
  }

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger asChild>
        <button
          type="button"
          className={cn(
            'group flex h-[36px] w-full items-center gap-3 rounded-lg px-3 text-[13px] font-medium transition-colors',
            isChildActive
              ? 'bg-sidebar-accent text-sidebar-accent-foreground'
              : 'text-sidebar-foreground/80 hover:bg-accent hover:text-sidebar-foreground',
          )}
        >
          <Icon className="size-4 shrink-0" />
          <span className="flex-1 truncate text-left">{item.label}</span>
          <ChevronRight
            className={cn(
              'size-3.5 shrink-0 transition-transform duration-200',
              open && 'rotate-90',
            )}
          />
        </button>
      </CollapsibleTrigger>
      <CollapsibleContent className="overflow-hidden data-[state=closed]:animate-collapsible-up data-[state=open]:animate-collapsible-down">
        <div className="py-0.5">
          {children.map((child) => (
            <SubItem key={child.href} item={child} pathname={pathname} />
          ))}
        </div>
      </CollapsibleContent>
    </Collapsible>
  )
}

// ─── Sidebar ─────────────────────────────────────────────────────────────────

export function Sidebar({ title, titleHref, menuGroups, bottom }: SidebarProps) {
  const collapsed = useStore(layoutStore, (s) => s.sidebarCollapsed)
  const { pathname } = useLocation()
  const width = collapsed ? SIDEBAR_COLLAPSED_WIDTH : SIDEBAR_WIDTH

  return (
    <aside
      className="fixed left-0 top-0 z-30 flex h-dvh flex-col border-r border-sidebar-border bg-sidebar transition-all duration-200"
      style={{ width }}
    >
      {/* Logo */}
      <Link
        to={titleHref}
        className={cn(
          'flex h-[50px] shrink-0 items-center gap-2 overflow-hidden border-b border-sidebar-border px-3 transition-all',
          collapsed && 'justify-center px-0',
        )}
      >
        <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground text-sm font-bold">
          S
        </div>
        {!collapsed && (
          <span className="truncate text-sm font-semibold text-sidebar-foreground">
            {title}
          </span>
        )}
      </Link>

      {/* Menu */}
      <ScrollArea className="flex-1 overflow-hidden">
        <nav className="flex flex-col gap-0.5 p-2">
          {menuGroups.map((group, gi) => (
            <div key={gi}>
              {group.map((item) => {
                if (item.children && item.children.length > 0) {
                  return (
                    <GroupItem
                      key={item.label}
                      item={item}
                      collapsed={collapsed}
                      pathname={pathname}
                    />
                  )
                }
                return (
                  <LeafItem
                    key={item.href}
                    item={item as MenuItem & { href: string }}
                    collapsed={collapsed}
                    pathname={pathname}
                  />
                )
              })}
            </div>
          ))}
        </nav>
      </ScrollArea>

      {/* Bottom slot */}
      {bottom && <div className="shrink-0">{bottom}</div>}

      {/* Collapse toggle */}
      <div className="relative shrink-0 border-t border-sidebar-border p-2">
        <button
          type="button"
          onClick={toggleSidebar}
          className={cn(
            'flex items-center justify-center rounded-md p-1.5 text-sidebar-foreground/60 transition-colors hover:bg-accent-hover hover:text-sidebar-foreground',
            collapsed ? 'mx-auto' : '',
          )}
          aria-label={collapsed ? '展开侧边栏' : '折叠侧边栏'}
        >
          {collapsed ? <ChevronsRight className="size-4" /> : <ChevronsLeft className="size-4" />}
        </button>
      </div>
    </aside>
  )
}
