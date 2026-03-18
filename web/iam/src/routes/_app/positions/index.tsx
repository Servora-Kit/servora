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
import type { positionservicev1_PositionInfo } from '@servora/api-client/iam/service/v1/index'

export const Route = createFileRoute('/_app/positions/')({
  component: PositionsPage,
})

function PositionsPage() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const { data, isLoading } = useQuery({
    queryKey: ['positions', 'list', page, pageSize],
    queryFn: () =>
      iamClients.position.ListPositions({
        pagination: { page: { page, pageSize } },
      }),
  })

  const positions = data?.positions ?? []
  const total = data?.pagination?.page?.total ?? 0

  const columns: ColumnDef<positionservicev1_PositionInfo, unknown>[] = [
    {
      accessorKey: 'code',
      header: '职位码',
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
      accessorKey: 'description',
      header: '描述',
      cell: ({ row }) => (
        <span className="text-muted-foreground">{row.original.description || '-'}</span>
      ),
    },
    {
      accessorKey: 'sort',
      header: '排序',
      cell: ({ row }) => (
        <span className="text-muted-foreground">{row.original.sort ?? 0}</span>
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
      id: 'actions',
      header: '操作',
      cell: ({ row }) => (
        <div className="flex gap-2">
          <EditPositionButton
            position={row.original}
            onUpdated={() => void queryClient.invalidateQueries({ queryKey: ['positions'] })}
          />
          <DeletePositionButton
            position={row.original}
            onDeleted={() => void queryClient.invalidateQueries({ queryKey: ['positions'] })}
          />
        </div>
      ),
    },
  ]

  return (
    <Page
      title="职位管理"
      description="管理租户内的职位定义，可关联至具体组织节点。"
      extra={
        <CreatePositionButton
          onCreated={() => void queryClient.invalidateQueries({ queryKey: ['positions'] })}
        />
      }
    >
      <DataTable
        columns={columns}
        data={positions}
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

function CreatePositionButton({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [sort, setSort] = useState(0)
  const [loading, setLoading] = useState(false)

  async function handleSubmit() {
    if (!code || !name) return
    setLoading(true)
    try {
      await iamClients.position.CreatePosition({ code, name, description: description || undefined, sort })
      setOpen(false)
      setCode('')
      setName('')
      setDescription('')
      setSort(0)
      onCreated()
      toast.success('职位创建成功')
    } catch {
      toast.error('创建失败，请检查职位码是否重复')
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <Plus className="size-4" />
        新增职位
      </Button>
      <FormDrawer
        open={open}
        onOpenChange={setOpen}
        title="新增职位"
        loading={loading}
        onSubmit={handleSubmit}
        submitLabel="创建"
      >
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>职位码</Label>
            <Input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="如：engineer"
            />
            <p className="text-xs text-muted-foreground">租户内唯一，创建后不可修改</p>
          </div>
          <div className="space-y-2">
            <Label>名称</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="如：工程师"
            />
          </div>
          <div className="space-y-2">
            <Label>描述（可选）</Label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="职位职责说明"
            />
          </div>
          <div className="space-y-2">
            <Label>排序</Label>
            <Input
              type="number"
              value={sort}
              onChange={(e) => setSort(Number(e.target.value))}
            />
          </div>
        </div>
      </FormDrawer>
    </>
  )
}

function EditPositionButton({
  position,
  onUpdated,
}: {
  position: positionservicev1_PositionInfo
  onUpdated: () => void
}) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState(position.name ?? '')
  const [description, setDescription] = useState(position.description ?? '')
  const [sort, setSort] = useState(position.sort ?? 0)
  const [loading, setLoading] = useState(false)

  async function handleSubmit() {
    setLoading(true)
    try {
      await iamClients.position.UpdatePosition({
        id: position.id ?? '',
        name,
        description: description || undefined,
        sort,
      })
      setOpen(false)
      onUpdated()
      toast.success('职位已更新')
    } catch {
      toast.error('更新失败')
    } finally {
      setLoading(false)
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
        title="编辑职位"
        loading={loading}
        onSubmit={handleSubmit}
        submitLabel="保存"
      >
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>名称</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label>描述（可选）</Label>
            <Input value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label>排序</Label>
            <Input
              type="number"
              value={sort}
              onChange={(e) => setSort(Number(e.target.value))}
            />
          </div>
        </div>
      </FormDrawer>
    </>
  )
}

function DeletePositionButton({
  position,
  onDeleted,
}: {
  position: positionservicev1_PositionInfo
  onDeleted: () => void
}) {
  const [open, setOpen] = useState(false)

  async function handleConfirm() {
    try {
      await iamClients.position.DeletePosition({ id: position.id ?? '' })
      setOpen(false)
      onDeleted()
      toast.success('职位已删除')
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
        title="删除职位"
        description={`确定要删除职位「${position.name}」吗？此操作不可撤销。`}
        confirmLabel="删除"
        onConfirm={handleConfirm}
        destructive
      />
    </>
  )
}
