import { createFileRoute } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { iamClients } from '#/api'
import { Page } from '#/components/page'
import { DataTable } from '#/components/data-table'
import { FormDrawer } from '#/components/form-drawer'
import { ConfirmDialog } from '#/components/confirm-dialog'
import { Button } from '#/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '#/components/ui/select'
import { Label } from '#/components/ui/label'
import { Badge } from '#/components/ui/badge'
import { UserPlus, Trash2, Loader2 } from 'lucide-react'
import { toast } from '#/lib/toast'

export const Route = createFileRoute('/_app/organizations/$orgId/members')({
  component: OrgMembersPage,
})

// Organization roles: admin > member. No "owner" or "viewer" at org level.
const ASSIGNABLE_ROLES = ['admin', 'member']
const PAGE_SIZE = 50

interface Member {
  userId?: string
  userName?: string
  userEmail?: string
  role?: string
}

function roleBadgeVariant(role: string): 'default' | 'secondary' | 'outline' {
  if (role === 'admin') return 'default'
  if (role === 'member') return 'secondary'
  return 'outline'
}

function OrgMembersPage() {
  const { orgId } = Route.useParams()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)

  const { data, isLoading } = useQuery({
    queryKey: ['org-members', orgId, page],
    queryFn: () =>
      iamClients.organization.ListMembers({
        organizationId: orgId,
        pagination: { page: { page, pageSize: PAGE_SIZE } },
      }),
  })

  const members = data?.members ?? []
  const total = data?.pagination?.page?.total ?? 0

  const [roleChange, setRoleChange] = useState<{
    userId: string
    name: string
    oldRole: string
    newRole: string
  } | null>(null)
  const [removeTarget, setRemoveTarget] = useState<{ userId: string; name: string } | null>(null)
  const [inviteOpen, setInviteOpen] = useState(false)
  const [inviteUserId, setInviteUserId] = useState<string | undefined>(undefined)
  const [inviteRole, setInviteRole] = useState('member')
  const [inviteLoading, setInviteLoading] = useState(false)

  // Fetch tenant users only when the invite drawer is open
  const { data: usersData, isLoading: usersLoading } = useQuery({
    queryKey: ['tenant-users-for-invite', orgId],
    queryFn: () =>
      iamClients.user.ListUsers({
        pagination: { page: { page: 1, pageSize: 100 } },
      }),
    enabled: inviteOpen,
    staleTime: 60_000,
  })
  const currentMemberIds = new Set(members.map((m) => m.userId))
  const userOptions = (usersData?.users ?? []).filter((u) => !currentMemberIds.has(u.id))

  function invalidate() {
    void queryClient.invalidateQueries({ queryKey: ['org-members', orgId] })
  }

  async function handleRoleConfirm() {
    if (!roleChange) return
    const change = roleChange
    setRoleChange(null)
    await toast.promise(
      iamClients.organization
        .UpdateMemberRole({
          organizationId: orgId,
          userId: change.userId,
          role: change.newRole,
        })
        .then(() => invalidate()),
      { loading: '更新角色...', success: `已将 ${change.name} 的角色改为 ${change.newRole}` },
    )
  }

  async function handleRemoveConfirm() {
    if (!removeTarget) return
    const target = removeTarget
    setRemoveTarget(null)
    await toast.promise(
      iamClients.organization
        .RemoveMember({
          organizationId: orgId,
          userId: target.userId,
        })
        .then(() => invalidate()),
      { loading: '移除中...', success: `已移除成员「${target.name}」` },
    )
  }

  async function handleInvite() {
    if (!inviteUserId) return
    setInviteLoading(true)
    try {
      await iamClients.organization.AddMember({
        organizationId: orgId,
        userId: inviteUserId,
        role: inviteRole,
      })
      setInviteOpen(false)
      setInviteUserId(undefined)
      setInviteRole('member')
      invalidate()
      toast.success('成员已添加')
    } finally {
      setInviteLoading(false)
    }
  }

  const columns: ColumnDef<Member, unknown>[] = [
    {
      accessorKey: 'userName',
      header: '用户',
      cell: ({ row }) => (
        <span className="font-medium text-foreground">{row.original.userName ?? '-'}</span>
      ),
    },
    {
      accessorKey: 'userEmail',
      header: '邮箱',
      cell: ({ row }) => (
        <span className="text-muted-foreground">{row.original.userEmail ?? '-'}</span>
      ),
    },
    {
      accessorKey: 'role',
      header: '角色',
      cell: ({ row }) => {
        const m = row.original
        const isAdmin = m.role === 'admin'

        if (isAdmin) {
          return <Badge variant={roleBadgeVariant('admin')}>admin</Badge>
        }

        return (
          <Select
            value={m.role ?? 'member'}
            onValueChange={(v) =>
              setRoleChange({
                userId: m.userId ?? '',
                name: m.userName ?? '',
                oldRole: m.role ?? '',
                newRole: v,
              })
            }
          >
            <SelectTrigger className="h-7 w-28 text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ASSIGNABLE_ROLES.map((r) => (
                <SelectItem key={r} value={r}>
                  {r}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )
      },
    },
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => {
        const m = row.original
        return (
          <Button
            variant="ghost"
            size="icon-xs"
            onClick={() =>
              setRemoveTarget({ userId: m.userId ?? '', name: m.userName ?? '' })
            }
          >
            <Trash2 className="size-3.5 text-destructive" />
          </Button>
        )
      },
    },
  ]

  return (
    <Page
      title="成员管理"
      description="管理组织成员。组织角色：admin > member。"
      extra={
        <Button onClick={() => setInviteOpen(true)}>
          <UserPlus className="size-4" />
          添加成员
        </Button>
      }
    >
      <DataTable
        columns={columns}
        data={members}
        isLoading={isLoading}
        page={page}
        pageSize={PAGE_SIZE}
        total={total}
        onPageChange={setPage}
        onPageSizeChange={() => {}}
      />

      <FormDrawer
        open={inviteOpen}
        onOpenChange={setInviteOpen}
        title="添加成员"
        loading={inviteLoading}
        onSubmit={handleInvite}
        submitLabel="添加"
      >
        <div className="space-y-2">
          <Label>选择用户</Label>
          <Select value={inviteUserId} onValueChange={setInviteUserId} disabled={usersLoading}>
            <SelectTrigger>
              {usersLoading ? (
                <span className="flex items-center gap-2 text-muted-foreground">
                  <Loader2 className="size-3.5 animate-spin" />
                  加载中...
                </span>
              ) : (
                <SelectValue placeholder="从租户用户中选择" />
              )}
            </SelectTrigger>
            <SelectContent position="popper">
              {!usersLoading && userOptions.length === 0 && (
                <SelectItem value="__empty__" disabled>
                  暂无可选用户（该租户所有成员均已在此组织中）
                </SelectItem>
              )}
              {userOptions.map((u) => (
                <SelectItem key={u.id} value={u.id ?? ''}>
                  <span className="font-medium">{u.name || u.id}</span>
                  {u.email && (
                    <span className="ml-2 text-xs text-muted-foreground">{u.email}</span>
                  )}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>角色</Label>
          <Select value={inviteRole} onValueChange={setInviteRole}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ASSIGNABLE_ROLES.map((r) => (
                <SelectItem key={r} value={r}>
                  {r}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </FormDrawer>

      <ConfirmDialog
        open={!!roleChange}
        onOpenChange={(open) => {
          if (!open) setRoleChange(null)
        }}
        title="确认角色变更"
        description={
          roleChange
            ? `确认将 ${roleChange.name} 的角色从 ${roleChange.oldRole} 改为 ${roleChange.newRole}？`
            : ''
        }
        onConfirm={handleRoleConfirm}
      />

      <ConfirmDialog
        open={!!removeTarget}
        onOpenChange={(open) => {
          if (!open) setRemoveTarget(null)
        }}
        title="移除成员"
        description={
          removeTarget ? `确认将 ${removeTarget.name} 从组织中移除？此操作不可撤销。` : ''
        }
        onConfirm={handleRemoveConfirm}
        destructive
        confirmLabel="移除"
      />
    </Page>
  )
}
