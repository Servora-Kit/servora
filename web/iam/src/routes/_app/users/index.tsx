import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useStore } from '@tanstack/react-store'
import type { ColumnDef } from '@tanstack/react-table'
import { iamClients } from '#/api'
import { scopeStore } from '#/stores/scope'
import { Page } from '#/components/page'
import { DataTable } from '#/components/data-table'
import { FormDrawer } from '#/components/form-drawer'
import { Button } from '#/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '#/components/ui/select'
import { Label } from '#/components/ui/label'
import { Input } from '#/components/ui/input'
import { UserPlus } from 'lucide-react'
import { toast } from '#/lib/toast'
import { Badge } from '#/components/ui/badge'

export const Route = createFileRoute('/_app/users/')({
  component: UserListPage,
})

type User = NonNullable<Awaited<ReturnType<typeof iamClients.user.ListUsers>>['users']>[number]
type Org = NonNullable<Awaited<ReturnType<typeof iamClients.organization.ListOrganizations>>['organizations']>[number]

function buildColumns(orgMap: Map<string, string>): ColumnDef<User, unknown>[] {
  return [
    {
      accessorKey: 'name',
      header: '用户名',
      cell: ({ row }) => (
        <Link
          to="/users/$userId"
          params={{ userId: row.original.id ?? '' }}
          className="font-medium text-foreground hover:underline"
        >
          {row.getValue('name')}
        </Link>
      ),
    },
    {
      accessorKey: 'email',
      header: '邮箱',
      cell: ({ row }) => <span className="text-muted-foreground">{row.getValue('email')}</span>,
    },
    {
      id: 'organizations',
      header: '所属组织',
      cell: ({ row }) => {
        const orgIds = row.original.organizationIds ?? []
        if (orgIds.length === 0) {
          return <span className="text-xs text-muted-foreground">—</span>
        }
        return (
          <div className="flex flex-wrap gap-1">
            {orgIds.map((id) => (
              <Badge key={id} variant="secondary" className="text-xs font-normal">
                {orgMap.get(id) ?? id.slice(0, 8)}
              </Badge>
            ))}
          </div>
        )
      },
    },
  ]
}

function UserListPage() {
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const queryClient = useQueryClient()

  const currentTenantId = useStore(scopeStore, (s) => s.currentTenantId)

  const { data, isLoading } = useQuery({
    queryKey: ['users', 'list', page, pageSize],
    queryFn: () => iamClients.user.ListUsers({ pagination: { page: { page, pageSize } } }),
  })

  const { data: orgsData } = useQuery({
    queryKey: ['organizations', 'list-for-create-user', currentTenantId],
    queryFn: () =>
      iamClients.organization.ListOrganizations({
        pagination: { page: { page: 1, pageSize: 100 } },
      }),
    enabled: !!currentTenantId,
    staleTime: 60_000,
  })

  const organizations = orgsData?.organizations ?? []

  const orgMap = useMemo(() => {
    const m = new Map<string, string>()
    for (const o of organizations as Org[]) {
      if (o.id) m.set(o.id, o.displayName || o.name || o.id)
    }
    return m
  }, [organizations])

  const columns = useMemo(() => buildColumns(orgMap), [orgMap])

  const users = data?.users ?? []
  const total = data?.pagination?.page?.total ?? 0

  const [createOpen, setCreateOpen] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [form, setForm] = useState({
    name: '',
    email: '',
    password: '',
    organizationId: '',
  })

  function resetForm() {
    setForm({ name: '', email: '', password: '', organizationId: '' })
  }

  async function handleCreate() {
    if (!form.name || !form.email || !form.password || !form.organizationId) return
    setCreateLoading(true)
    try {
      await iamClients.user.CreateUser({
        name: form.name,
        email: form.email,
        password: form.password,
        organizationId: form.organizationId,
      })
      setCreateOpen(false)
      resetForm()
      void queryClient.invalidateQueries({ queryKey: ['users', 'list'] })
      toast.success('用户已创建')
    } finally {
      setCreateLoading(false)
    }
  }

  return (
    <Page
      title="用户"
      extra={
        <Button onClick={() => setCreateOpen(true)}>
          <UserPlus className="size-4" />
          创建用户
        </Button>
      }
    >
      <DataTable
        columns={columns}
        data={users}
        isLoading={isLoading}
        page={page}
        pageSize={pageSize}
        total={total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <FormDrawer
        open={createOpen}
        onOpenChange={(open) => {
          setCreateOpen(open)
          if (!open) resetForm()
        }}
        title="创建用户"
        loading={createLoading}
        onSubmit={handleCreate}
        submitLabel="创建"
      >
        <div className="space-y-2">
          <Label htmlFor="create-user-name">用户名</Label>
          <Input
            id="create-user-name"
            placeholder="输入用户名（至少 2 个字符）"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="create-user-email">邮箱</Label>
          <Input
            id="create-user-email"
            type="email"
            placeholder="user@example.com"
            value={form.email}
            onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="create-user-password">密码</Label>
          <Input
            id="create-user-password"
            type="password"
            placeholder="至少 6 个字符"
            value={form.password}
            onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))}
          />
        </div>
        <div className="space-y-2">
          <Label>所属组织</Label>
          <Select
            value={form.organizationId}
            onValueChange={(v) => setForm((f) => ({ ...f, organizationId: v }))}
          >
            <SelectTrigger>
              <SelectValue placeholder="选择组织（必填）" />
            </SelectTrigger>
            <SelectContent>
              {organizations.length === 0 && (
                <div className="py-4 text-center text-xs text-muted-foreground">
                  当前租户暂无组织
                </div>
              )}
              {organizations.map((o) => (
                <SelectItem key={o.id} value={o.id ?? ''}>
                  {o.displayName || o.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">
            用户将以 member 身份加入所选组织，并自动成为当前租户的成员。
          </p>
        </div>
      </FormDrawer>
    </Page>
  )
}
