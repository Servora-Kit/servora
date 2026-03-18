import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { Plus, Users, Trash2 } from 'lucide-react'
import { iamClients } from '#/api'
import { toast } from '#/lib/toast'
import { Page } from '#/components/page'
import { DataTable } from '#/components/data-table'
import { FormDrawer } from '#/components/form-drawer'
import { ConfirmDialog } from '#/components/confirm-dialog'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'

export const Route = createFileRoute('/_app/organizations/')({
  component: OrganizationListPage,
})

type Org = NonNullable<Awaited<ReturnType<typeof iamClients.organization.ListOrganizations>>['organizations']>[number]

function OrganizationListPage() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null)

  function invalidate() {
    void queryClient.invalidateQueries({ queryKey: ['organizations'] })
  }

  async function handleDeleteConfirm() {
    if (!deleteTarget) return
    const target = deleteTarget
    setDeleteTarget(null)
    await toast.promise(
      iamClients.organization.DeleteOrganization({ id: target.id }).then(() => invalidate()),
      { loading: '删除中...', success: `已删除「${target.name}」` },
    )
  }

  const columns: ColumnDef<Org, unknown>[] = [
    {
      accessorKey: 'name',
      header: '名称',
      cell: ({ row }) => (
        <Link
          to="/organizations/$orgId"
          params={{ orgId: row.original.id ?? '' }}
          className="font-medium text-foreground hover:underline"
        >
          {row.original.displayName || row.original.name}
        </Link>
      ),
    },
    {
      accessorKey: 'createdAt',
      header: '创建时间',
      cell: ({ row }) => (
        <span className="text-muted-foreground">
          {row.original.createdAt ? new Date(row.original.createdAt).toLocaleDateString('zh-CN') : '-'}
        </span>
      ),
    },
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => {
        const o = row.original
        return (
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="icon-xs" asChild>
              <Link to="/organizations/$orgId/members" params={{ orgId: o.id ?? '' }}>
                <Users className="size-3.5" />
              </Link>
            </Button>
            <Button
              variant="ghost"
              size="icon-xs"
              onClick={() => setDeleteTarget({ id: o.id ?? '', name: o.displayName || o.name || '' })}
            >
              <Trash2 className="size-3.5 text-destructive" />
            </Button>
          </div>
        )
      },
    },
  ]

  const { data, isLoading } = useQuery({
    queryKey: ['organizations', 'list', page, pageSize],
    queryFn: () =>
      iamClients.organization.ListOrganizations({
        pagination: { page: { page, pageSize } },
      }),
  })

  const orgs = data?.organizations ?? []
  const total = data?.pagination?.page?.total ?? 0

  return (
    <Page
      title="组织"
      description="管理组织及其成员。"
      extra={<CreateOrgButton onCreated={invalidate} />}
    >
      <DataTable
        columns={columns}
        data={orgs}
        isLoading={isLoading}
        page={page}
        pageSize={pageSize}
        total={total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}
        title="删除组织"
        description={deleteTarget ? `确认删除组织「${deleteTarget.name}」？此操作不可撤销，相关项目和成员数据将一并清除。` : ''}
        onConfirm={handleDeleteConfirm}
        destructive
        confirmLabel="删除"
      />
    </Page>
  )
}

function CreateOrgButton({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit() {
    setLoading(true)
    try {
      await iamClients.organization.CreateOrganization({ name, slug: '', displayName: name, type: 'COMPANY', sort: 0 })
      setOpen(false)
      setName('')
      onCreated()
      toast.success('组织创建成功')
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <Plus className="size-4" />
        创建组织
      </Button>
      <FormDrawer open={open} onOpenChange={setOpen} title="创建组织" loading={loading} onSubmit={handleSubmit} submitLabel="创建">
        <div className="space-y-2">
          <Label>名称</Label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="如：我的团队"
            required
          />
        </div>
      </FormDrawer>
    </>
  )
}
