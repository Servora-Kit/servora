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
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import type {
  dictservicev1_DictTypeInfo,
  dictservicev1_DictItemInfo,
} from '@servora/api-client/iam/service/v1/index'

export const Route = createFileRoute('/_app/system/dict/')({
  component: DictPage,
})

function DictPage() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [selectedType, setSelectedType] = useState<dictservicev1_DictTypeInfo | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['dict', 'types', page, pageSize],
    queryFn: () => iamClients.dict.ListDictTypes({ pagination: { page: { page, pageSize } } }),
  })

  const dictTypes = data?.dictTypes ?? []
  const total = data?.pagination?.page?.total ?? 0

  const columns: ColumnDef<dictservicev1_DictTypeInfo, unknown>[] = [
    {
      accessorKey: 'code',
      header: '字典码',
      cell: ({ row }) => (
        <button
          className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono hover:bg-muted/80 transition-colors text-left"
          onClick={() => setSelectedType(row.original)}
        >
          {row.original.code}
        </button>
      ),
    },
    {
      accessorKey: 'name',
      header: '名称',
      cell: ({ row }) => (
        <button
          className="font-medium hover:underline text-left"
          onClick={() => setSelectedType(row.original)}
        >
          {row.original.name}
        </button>
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
        <DeleteDictTypeButton
          dictType={row.original}
          onDeleted={() => {
            if (selectedType?.id === row.original.id) setSelectedType(null)
            void queryClient.invalidateQueries({ queryKey: ['dict'] })
          }}
        />
      ),
    },
  ]

  return (
    <Page
      title="字典管理"
      description="管理系统字典类型及选项，用于下拉框、标签等配置化场景。点击字典码或名称查看字典项。"
      extra={
        <CreateDictTypeButton
          onCreated={() => void queryClient.invalidateQueries({ queryKey: ['dict'] })}
        />
      }
    >
      <div className="grid gap-4 lg:grid-cols-2">
        <div>
          <DataTable
            columns={columns}
            data={dictTypes}
            isLoading={isLoading}
            page={page}
            pageSize={pageSize}
            total={total}
            onPageChange={setPage}
            onPageSizeChange={setPageSize}
          />
        </div>
        <div>
          {selectedType ? (
            <DictItemsPanel
              dictType={selectedType}
              onChanged={() => void queryClient.invalidateQueries({ queryKey: ['dict', 'items', selectedType.id] })}
            />
          ) : (
            <div className="flex h-48 items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
              点击左侧字典类型查看字典项
            </div>
          )}
        </div>
      </div>
    </Page>
  )
}

function DictItemsPanel({ dictType, onChanged }: { dictType: dictservicev1_DictTypeInfo; onChanged: () => void }) {
  const queryClient = useQueryClient()
  const typeId = dictType.id ?? ''

  const { data, isLoading } = useQuery({
    queryKey: ['dict', 'items', typeId],
    queryFn: () => iamClients.dict.ListDictItems({ dictTypeIdOrCode: typeId }),
    enabled: !!typeId,
  })

  const items = data?.items ?? []

  const columns: ColumnDef<dictservicev1_DictItemInfo, unknown>[] = [
    {
      accessorKey: 'label',
      header: '显示标签',
      cell: ({ row }) => <span>{row.original.label}</span>,
    },
    {
      accessorKey: 'value',
      header: '值',
      cell: ({ row }) => (
        <code className="text-xs text-muted-foreground">{row.original.value}</code>
      ),
    },
    {
      accessorKey: 'sort',
      header: '排序',
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.sort ?? 0}</span>,
    },
    {
      accessorKey: 'isDefault',
      header: '默认',
      cell: ({ row }) =>
        row.original.isDefault ? (
          <Badge variant="secondary">默认</Badge>
        ) : null,
    },
    {
      id: 'actions',
      header: '',
      cell: ({ row }) => (
        <DeleteDictItemButton
          item={row.original}
          onDeleted={() => {
            void queryClient.invalidateQueries({ queryKey: ['dict', 'items', typeId] })
            onChanged()
          }}
        />
      ),
    },
  ]

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
        <CardTitle className="text-base">
          <span className="font-medium">{dictType.name}</span>
          <code className="ml-2 rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
            {dictType.code}
          </code>
        </CardTitle>
        <CreateDictItemButton
          typeId={typeId}
          onCreated={() => {
            void queryClient.invalidateQueries({ queryKey: ['dict', 'items', typeId] })
            onChanged()
          }}
        />
      </CardHeader>
      <CardContent className="pt-0">
        <DataTable columns={columns} data={items} isLoading={isLoading} />
      </CardContent>
    </Card>
  )
}

function CreateDictTypeButton({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [loading] = useState(false)

  async function handleSubmit() {
    if (!code || !name) return
    try {
      await iamClients.dict.CreateDictType({ code, name, sort: 0 })
      setOpen(false)
      setCode('')
      setName('')
      onCreated()
      toast.success('字典类型创建成功')
    } catch {
      toast.error('创建失败，请检查字典码是否重复')
    }
  }

  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <Plus className="size-4" />
        新增字典类型
      </Button>
      <FormDrawer
        open={open}
        onOpenChange={setOpen}
        title="新增字典类型"
        loading={loading}
        onSubmit={handleSubmit}
        submitLabel="创建"
      >
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>字典码</Label>
            <Input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="如：gender"
            />
            <p className="text-xs text-muted-foreground">全局唯一标识，创建后不可修改</p>
          </div>
          <div className="space-y-2">
            <Label>名称</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="如：性别"
            />
          </div>
        </div>
      </FormDrawer>
    </>
  )
}

function CreateDictItemButton({ typeId, onCreated }: { typeId: string; onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [label, setLabel] = useState('')
  const [value, setValue] = useState('')
  const [sort, setSort] = useState(0)
  const [isDefault, setIsDefault] = useState(false)
  const [loading] = useState(false)

  async function handleSubmit() {
    if (!label || !value) return
    try {
      await iamClients.dict.CreateDictItem({
        dictTypeId: typeId,
        label,
        value,
        sort,
        isDefault,
      })
      setOpen(false)
      setLabel('')
      setValue('')
      setSort(0)
      setIsDefault(false)
      onCreated()
      toast.success('字典项创建成功')
    } catch {
      toast.error('创建失败，请检查值是否重复')
    }
  }

  return (
    <>
      <Button variant="outline" size="sm" onClick={() => setOpen(true)}>
        <Plus className="size-3.5" />
        新增字典项
      </Button>
      <FormDrawer
        open={open}
        onOpenChange={setOpen}
        title="新增字典项"
        loading={loading}
        onSubmit={handleSubmit}
        submitLabel="创建"
      >
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>显示标签</Label>
            <Input
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="如：男"
            />
          </div>
          <div className="space-y-2">
            <Label>值</Label>
            <Input
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder="如：male"
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
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="isDefault"
              checked={isDefault}
              onChange={(e) => setIsDefault(e.target.checked)}
              className="h-4 w-4"
            />
            <Label htmlFor="isDefault">设为默认值</Label>
          </div>
        </div>
      </FormDrawer>
    </>
  )
}

function DeleteDictTypeButton({
  dictType,
  onDeleted,
}: {
  dictType: dictservicev1_DictTypeInfo
  onDeleted: () => void
}) {
  const [open, setOpen] = useState(false)
  async function handleConfirm() {
    try {
      await iamClients.dict.DeleteDictType({ id: dictType.id ?? '' })
      setOpen(false)
      onDeleted()
      toast.success('字典类型已删除')
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
        title="删除字典类型"
        description={`确定要删除字典「${dictType.name}」吗？所有字典项将一并删除。`}
        confirmLabel="删除"
        onConfirm={handleConfirm}
        destructive
      />
    </>
  )
}

function DeleteDictItemButton({
  item,
  onDeleted,
}: {
  item: dictservicev1_DictItemInfo
  onDeleted: () => void
}) {
  const [open, setOpen] = useState(false)
  async function handleConfirm() {
    try {
      await iamClients.dict.DeleteDictItem({ id: item.id ?? '' })
      setOpen(false)
      onDeleted()
      toast.success('字典项已删除')
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
        title="删除字典项"
        description={`确定要删除字典项「${item.label}」吗？`}
        confirmLabel="删除"
        onConfirm={handleConfirm}
        destructive
      />
    </>
  )
}
