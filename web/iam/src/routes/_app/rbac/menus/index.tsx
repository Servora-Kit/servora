import { createFileRoute } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2, ChevronRight } from 'lucide-react'
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
import type { rbacservicev1_MenuInfo } from '@servora/api-client/iam/service/v1/index'

export const Route = createFileRoute('/_app/rbac/menus/')({
  component: MenusPage,
})

const TYPE_BADGE: Record<string, { label: string; variant: 'default' | 'secondary' | 'outline' }> = {
  CATALOG: { label: '目录', variant: 'secondary' },
  MENU: { label: '菜单', variant: 'default' },
  BUTTON: { label: '按钮', variant: 'outline' },
}

function flattenMenus(menus: rbacservicev1_MenuInfo[], depth = 0): Array<rbacservicev1_MenuInfo & { _depth: number }> {
  const result: Array<rbacservicev1_MenuInfo & { _depth: number }> = []
  for (const m of menus) {
    result.push({ ...m, _depth: depth })
    if (m.children && m.children.length > 0) {
      result.push(...flattenMenus(m.children, depth + 1))
    }
  }
  return result
}

function MenusPage() {
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['rbac', 'menus'],
    queryFn: () => iamClients.rbac.ListMenus({}),
  })

  const flatMenus = flattenMenus(data?.menus ?? [])

  const columns: ColumnDef<rbacservicev1_MenuInfo & { _depth: number }, unknown>[] = [
    {
      accessorKey: 'name',
      header: '菜单名称',
      cell: ({ row }) => {
        const depth = row.original._depth
        return (
          <div className="flex items-center gap-1" style={{ paddingLeft: depth * 16 }}>
            {depth > 0 && <ChevronRight className="size-3 text-muted-foreground shrink-0" />}
            <span className="font-medium">{row.original.name}</span>
          </div>
        )
      },
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
      accessorKey: 'path',
      header: '路径',
      cell: ({ row }) => (
        <code className="text-xs text-muted-foreground">{row.original.path || '-'}</code>
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
        <DeleteMenuButton
          menu={row.original}
          onDeleted={() => void queryClient.invalidateQueries({ queryKey: ['rbac', 'menus'] })}
        />
      ),
    },
  ]

  return (
    <Page
      title="菜单管理"
      description="管理动态菜单树。菜单与权限码关联，用户的菜单由其权限码决定。"
      extra={
        <CreateMenuButton
          parentOptions={flatMenus.filter((m) => m.type === 'CATALOG')}
          onCreated={() => void queryClient.invalidateQueries({ queryKey: ['rbac', 'menus'] })}
        />
      }
    >
      <DataTable
        columns={columns}
        data={flatMenus}
        isLoading={isLoading}
      />
    </Page>
  )
}

function CreateMenuButton({
  parentOptions,
  onCreated,
}: {
  parentOptions: rbacservicev1_MenuInfo[]
  onCreated: () => void
}) {
  const [open, setOpen] = useState(false)
  const [type, setType] = useState<'CATALOG' | 'MENU'>('MENU')
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [component, setComponent] = useState('')
  const [sort, setSort] = useState(0)
  const [parentId, setParentId] = useState('')
  const [loading] = useState(false)

  async function handleSubmit() {
    if (!name) return
    try {
      await iamClients.rbac.CreateMenu({
        type,
        name,
        path: path || undefined,
        component: component || undefined,
        parentId: parentId || undefined,
        sort,
      })
      setOpen(false)
      setName('')
      setPath('')
      setComponent('')
      setParentId('')
      setSort(0)
      onCreated()
      toast.success('菜单已创建')
    } catch {
      toast.error('创建失败')
    }
  }

  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <Plus className="size-4" />
        新增菜单
      </Button>
      <FormDrawer
        open={open}
        onOpenChange={setOpen}
        title="新增菜单"
        loading={loading}
        onSubmit={handleSubmit}
        submitLabel="创建"
      >
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>类型</Label>
            <div className="flex gap-2">
              {(['CATALOG', 'MENU'] as const).map((t) => (
                <Button
                  key={t}
                  type="button"
                  variant={type === t ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setType(t)}
                >
                  {t === 'CATALOG' ? '目录' : '菜单'}
                </Button>
              ))}
            </div>
          </div>
          <div className="space-y-2">
            <Label>名称</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="菜单名称" />
          </div>
          {type === 'MENU' && (
            <>
              <div className="space-y-2">
                <Label>路径</Label>
                <Input value={path} onChange={(e) => setPath(e.target.value)} placeholder="/example/path" />
              </div>
              <div className="space-y-2">
                <Label>组件</Label>
                <Input
                  value={component}
                  onChange={(e) => setComponent(e.target.value)}
                  placeholder="example/index"
                />
              </div>
            </>
          )}
          <div className="space-y-2">
            <Label>父级目录</Label>
            <select
              value={parentId}
              onChange={(e) => setParentId(e.target.value)}
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs"
            >
              <option value="">— 顶级菜单 —</option>
              {parentOptions.map((p) => (
                <option key={p.id} value={p.id ?? ''}>
                  {p.name}
                </option>
              ))}
            </select>
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

function DeleteMenuButton({
  menu,
  onDeleted,
}: {
  menu: rbacservicev1_MenuInfo
  onDeleted: () => void
}) {
  const [open, setOpen] = useState(false)
  async function handleConfirm() {
    try {
      await iamClients.rbac.DeleteMenu({ id: menu.id ?? '' })
      setOpen(false)
      onDeleted()
      toast.success('菜单已删除')
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
        title="删除菜单"
        description={`确定要删除菜单「${menu.name}」吗？子菜单也会受到影响。`}
        confirmLabel="删除"
        onConfirm={handleConfirm}
        destructive
      />
    </>
  )
}
