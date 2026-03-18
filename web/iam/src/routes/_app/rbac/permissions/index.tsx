import { createFileRoute } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2 } from 'lucide-react'
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
import type { rbacservicev1_PermissionInfo } from '@servora/api-client/iam/service/v1/index'

export const Route = createFileRoute('/_app/rbac/permissions/')({
  component: PermissionsPage,
})

function PermissionsPage() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const { data, isLoading } = useQuery({
    queryKey: ['rbac', 'permissions', page, pageSize],
    queryFn: () =>
      iamClients.rbac.ListPermissions({
        pagination: { page: { page, pageSize } },
      }),
  })

  const permissions = data?.permissions ?? []
  const total = data?.pagination?.page?.total ?? 0

  const columns: ColumnDef<rbacservicev1_PermissionInfo, unknown>[] = [
    {
      accessorKey: 'code',
      header: '权限码',
      cell: ({ row }) => (
        <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">
          {row.original.code}
        </code>
      ),
    },
    {
      accessorKey: 'name',
      header: '名称',
      cell: ({ row }) => <span className="font-medium">{row.original.name}</span>,
    },
    {
      accessorKey: 'groupName',
      header: '分组',
      cell: ({ row }) => (
        <span className="text-muted-foreground">{row.original.groupName || '-'}</span>
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
    {
      id: 'apis',
      header: '关联 API',
      cell: ({ row }) => {
        const apis = row.original.apis ?? []
        return (
          <span className="text-xs text-muted-foreground">
            {apis.length > 0 ? `${apis.length} 个` : '-'}
          </span>
        )
      },
    },
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => (
        <DeletePermissionButton
          permission={row.original}
          onDeleted={() => void queryClient.invalidateQueries({ queryKey: ['rbac', 'permissions'] })}
        />
      ),
    },
  ]

  return (
    <Page
      title="权限码管理"
      description="管理系统权限码定义。权限码格式：resource:action（如 user:create）。API 鉴权仍由 OpenFGA 负责，此表为元数据。"
      extra={
        <CreatePermissionButton
          onCreated={() => void queryClient.invalidateQueries({ queryKey: ['rbac', 'permissions'] })}
        />
      }
    >
      <DataTable
        columns={columns}
        data={permissions}
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

function CreatePermissionButton({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [loading] = useState(false)

  async function handleSubmit() {
    if (!code || !name) return
    try {
      await iamClients.rbac.CreatePermission({ code, name, description: description || undefined })
      setOpen(false)
      setCode('')
      setName('')
      setDescription('')
      onCreated()
      toast.success('权限码创建成功')
    } catch {
      toast.error('创建失败，请检查权限码是否重复')
    }
  }

  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <Plus className="size-4" />
        新增权限码
      </Button>
      <FormDrawer
        open={open}
        onOpenChange={setOpen}
        title="新增权限码"
        loading={loading}
        onSubmit={handleSubmit}
        submitLabel="创建"
      >
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>权限码</Label>
            <Input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="如：report:view"
            />
            <p className="text-xs text-muted-foreground">格式：resource:action，全局唯一</p>
          </div>
          <div className="space-y-2">
            <Label>名称</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="如：查看报表"
            />
          </div>
          <div className="space-y-2">
            <Label>描述（可选）</Label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="描述此权限的用途"
            />
          </div>
        </div>
      </FormDrawer>
    </>
  )
}

function DeletePermissionButton({
  permission,
  onDeleted,
}: {
  permission: rbacservicev1_PermissionInfo
  onDeleted: () => void
}) {
  const [open, setOpen] = useState(false)
  async function handleConfirm() {
    try {
      await iamClients.rbac.DeletePermission({ id: permission.id ?? '' })
      setOpen(false)
      onDeleted()
      toast.success('权限码已删除')
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
        title="删除权限码"
        description={`确定要删除权限码「${permission.code}」吗？相关角色将失去此权限。`}
        confirmLabel="删除"
        onConfirm={handleConfirm}
        destructive
      />
    </>
  )
}
