import { createFileRoute } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { Plus, Pencil, Trash2 } from 'lucide-react'
import { iamClients } from '#/api'
import { Page } from '#/components/page'
import { DataTable } from '#/components/data-table'
import { FormDrawer } from '#/components/form-drawer'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import { Badge } from '#/components/ui/badge'
import { ConfirmDialog } from '#/components/confirm-dialog'
import { toast } from '#/lib/toast'
import type { rbacservicev1_RoleInfo } from '@servora/api-client/iam/service/v1/index'

export const Route = createFileRoute('/_app/rbac/roles/')({
  component: RolesPage,
})

const TYPE_BADGE: Record<string, { label: string; variant: 'default' | 'secondary' | 'outline' }> = {
  BUILTIN: { label: '内置', variant: 'secondary' },
  CUSTOM: { label: '自定义', variant: 'outline' },
}

function RolesPage() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const { data, isLoading } = useQuery({
    queryKey: ['rbac', 'roles', page, pageSize],
    queryFn: () =>
      iamClients.rbac.ListRoles({
        pagination: { page: { page, pageSize } },
      }),
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
      cell: ({ row }) => {
        const t = row.original.type ?? ''
        const cfg = TYPE_BADGE[t] ?? { label: t, variant: 'outline' as const }
        return <Badge variant={cfg.variant}>{cfg.label}</Badge>
      },
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
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => {
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
    },
  ]

  return (
    <Page
      title="角色管理"
      description="管理平台内置角色及自定义角色，配置角色权限。"
      extra={
        <CreateRoleButton
          onCreated={() => void queryClient.invalidateQueries({ queryKey: ['rbac', 'roles'] })}
        />
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

function CreateRoleButton({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [loading] = useState(false)

  async function handleSubmit() {
    if (!code || !name) return
    try {
      await iamClients.rbac.CreateRole({ code, name, permissionIds: [] })
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
        title="新增角色"
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
              placeholder="如：custom_viewer"
            />
            <p className="text-xs text-muted-foreground">全局唯一标识符，创建后不可修改</p>
          </div>
          <div className="space-y-2">
            <Label>角色名称</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="如：只读用户"
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
        description={`确定要删除角色「${role.name}」吗？此操作不可撤销。`}
        confirmLabel="删除"
        onConfirm={handleConfirm}
        destructive
      />
    </>
  )
}
