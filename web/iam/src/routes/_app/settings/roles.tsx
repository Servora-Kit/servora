import { createFileRoute } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { Plus, Pencil, Trash2 } from 'lucide-react'
import { useStore } from '@tanstack/react-store'
import { iamClients } from '#/api'
import { scopeStore } from '#/stores/scope'
import { Page } from '#/components/page'
import { DataTable } from '#/components/data-table'
import { FormDrawer } from '#/components/form-drawer'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import { Badge } from '#/components/ui/badge'
import { ConfirmDialog } from '#/components/confirm-dialog'
import { toast } from '#/lib/toast'
import { usePermissions } from '#/hooks/use-permissions'
import type { rbacservicev1_RoleInfo } from '@servora/api-client/iam/service/v1/index'

export const Route = createFileRoute('/_app/settings/roles')({
  component: TenantRolesPage,
})

function TenantRolesPage() {
  const queryClient = useQueryClient()
  const currentTenantId = useStore(scopeStore, (s) => s.currentTenantId)
  const { hasPermission } = usePermissions()
  const canManage = hasPermission('rbac:role:manage')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const { data, isLoading } = useQuery({
    queryKey: ['rbac', 'roles', 'tenant', currentTenantId, page, pageSize],
    queryFn: () =>
      iamClients.rbac.ListRoles({
        tenantId: currentTenantId ?? undefined,
        pagination: { page: { page, pageSize } },
      }),
    enabled: !!currentTenantId,
  })

  const roles = data?.roles ?? []
  const total = data?.pagination?.page?.total ?? 0

  const columns: ColumnDef<rbacservicev1_RoleInfo, unknown>[] = [
    {
      accessorKey: 'code',
      header: '角色码',
      cell: ({ row }) => (
        <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{row.original.code}</code>
      ),
    },
    {
      accessorKey: 'name',
      header: '名称',
      cell: ({ row }) => <span className="font-medium">{row.original.name}</span>,
    },
    {
      accessorKey: 'type',
      header: '类型',
      cell: ({ row }) => (
        <Badge variant={row.original.type === 'BUILTIN' ? 'secondary' : 'outline'}>
          {row.original.type === 'BUILTIN' ? '内置' : '自定义'}
        </Badge>
      ),
    },
    {
      accessorKey: 'status',
      header: '状态',
      cell: ({ row }) => (
        <Badge variant={row.original.status === 'ACTIVE' ? 'default' : 'secondary'}>
          {row.original.status === 'ACTIVE' ? '启用' : '禁用'}
        </Badge>
      ),
    },
    ...(canManage
      ? [
          {
            id: 'actions',
            header: '操作',
            cell: ({ row }: { row: { original: rbacservicev1_RoleInfo } }) => {
              const role = row.original
              if (role.isProtected) {
                return <span className="text-xs text-muted-foreground">受保护</span>
              }
              return (
                <div className="flex gap-2">
                  <EditRoleButton
                    role={role}
                    onUpdated={() => void queryClient.invalidateQueries({ queryKey: ['rbac', 'roles'] })}
                  />
                  <DeleteRoleButton
                    role={role}
                    onDeleted={() => void queryClient.invalidateQueries({ queryKey: ['rbac', 'roles'] })}
                  />
                </div>
              )
            },
          } as ColumnDef<rbacservicev1_RoleInfo, unknown>,
        ]
      : []),
  ]

  return (
    <Page
      title="角色管理"
      description="管理当前租户的自定义角色，配置角色权限。内置角色（平台管理员、租户所有者等）由平台统一管理。"
      extra={
        canManage && currentTenantId ? (
          <CreateRoleButton
            tenantId={currentTenantId}
            onCreated={() => void queryClient.invalidateQueries({ queryKey: ['rbac', 'roles'] })}
          />
        ) : null
      }
    >
      <DataTable
        columns={columns}
        data={roles}
        isLoading={isLoading}
        page={page}
        pageSize={pageSize}
        total={total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />
    </Page>
  )
}

function CreateRoleButton({
  tenantId,
  onCreated,
}: {
  tenantId: string
  onCreated: () => void
}) {
  const [open, setOpen] = useState(false)
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [loading] = useState(false)

  async function handleSubmit() {
    if (!code || !name) return
    try {
      await iamClients.rbac.CreateRole({ code, name, tenantId, permissionIds: [] })
      setOpen(false)
      setCode('')
      setName('')
      onCreated()
      toast.success('角色创建成功')
    } catch {
      toast.error('创建失败，请检查角色码是否重复')
    }
  }

  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <Plus className="size-4" />
        新增角色
      </Button>
      <FormDrawer
        open={open}
        onOpenChange={setOpen}
        title="新增自定义角色"
        loading={loading}
        onSubmit={handleSubmit}
        submitLabel="创建"
      >
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>角色码</Label>
            <Input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="如：custom_reviewer"
            />
            <p className="text-xs text-muted-foreground">租户内唯一标识符</p>
          </div>
          <div className="space-y-2">
            <Label>角色名称</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="如：代码审核员"
            />
          </div>
        </div>
      </FormDrawer>
    </>
  )
}

function EditRoleButton({
  role,
  onUpdated,
}: {
  role: rbacservicev1_RoleInfo
  onUpdated: () => void
}) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState(role.name ?? '')
  const [loading] = useState(false)

  async function handleSubmit() {
    try {
      await iamClients.rbac.UpdateRole({ id: role.id ?? '', name, permissionIds: [] })
      setOpen(false)
      onUpdated()
      toast.success('角色已更新')
    } catch {
      toast.error('更新失败')
    }
  }

  return (
    <>
      <Button variant="ghost" size="sm" onClick={() => setOpen(true)}>
        <Pencil className="size-3.5" />
      </Button>
      <FormDrawer
        open={open}
        onOpenChange={setOpen}
        title="编辑角色"
        loading={loading}
        onSubmit={handleSubmit}
        submitLabel="保存"
      >
        <div className="space-y-2">
          <Label>角色名称</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} />
        </div>
      </FormDrawer>
    </>
  )
}

function DeleteRoleButton({
  role,
  onDeleted,
}: {
  role: rbacservicev1_RoleInfo
  onDeleted: () => void
}) {
  const [open, setOpen] = useState(false)
  async function handleConfirm() {
    try {
      await iamClients.rbac.DeleteRole({ id: role.id ?? '' })
      setOpen(false)
      onDeleted()
      toast.success('角色已删除')
    } catch {
      toast.error('删除失败')
    }
  }

  return (
    <>
      <Button variant="ghost" size="sm" className="text-destructive" onClick={() => setOpen(true)}>
        <Trash2 className="size-3.5" />
      </Button>
      <ConfirmDialog
        open={open}
        onOpenChange={setOpen}
        title="删除角色"
        description={`确定要删除角色「${role.name}」吗？`}
        confirmLabel="删除"
        onConfirm={handleConfirm}
        destructive
      />
    </>
  )
}
