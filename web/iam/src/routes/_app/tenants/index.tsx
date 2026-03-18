import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { iamClients } from '#/api'
import { Page } from '#/components/page'
import { DataTable } from '#/components/data-table'
import { FormDrawer } from '#/components/form-drawer'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import { Plus } from 'lucide-react'
import { toast } from '#/lib/toast'

export const Route = createFileRoute('/_app/tenants/')({
  component: TenantListPage,
})

type Tenant = NonNullable<Awaited<ReturnType<typeof iamClients.tenant.ListTenants>>['tenants']>[number]

const KIND_LABEL: Record<string, string> = { business: '企业', personal: '个人' }

const columns: ColumnDef<Tenant, unknown>[] = [
  {
    accessorKey: 'name',
    header: '名称',
    cell: ({ row }) => (
      <Link
        to="/tenants/$tenantId"
        params={{ tenantId: row.original.id ?? '' }}
        className="font-medium text-foreground hover:underline"
      >
        {row.original.displayName || row.original.name}
      </Link>
    ),
  },
  {
    accessorKey: 'kind',
    header: '类型',
    cell: ({ row }) => {
      const kind = row.getValue('kind') as string
      return <span className="text-muted-foreground">{KIND_LABEL[kind] ?? kind}</span>
    },
  },
  {
    accessorKey: 'status',
    header: '状态',
    cell: ({ row }) => <span className="text-muted-foreground">{row.getValue('status')}</span>,
  },
]

function TenantListPage() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const { data, isLoading } = useQuery({
    queryKey: ['tenants', 'list', page, pageSize],
    queryFn: () => iamClients.tenant.ListTenants({ pagination: { page: { page, pageSize } } }),
  })

  const tenants = data?.tenants ?? []
  const total = data?.pagination?.page?.total ?? 0

  return (
    <Page
      title="租户管理"
      description="管理平台中的所有租户。"
      extra={<CreateTenantButton onCreated={() => void queryClient.invalidateQueries({ queryKey: ['tenants'] })} />}
    >
      <DataTable
        columns={columns}
        data={tenants}
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

function CreateTenantButton({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit() {
    setLoading(true)
    try {
      await iamClients.tenant.CreateTenant({ name, displayName: name, slug: '', kind: 'business', domain: '' })
      setOpen(false)
      setName('')
      onCreated()
      toast.success('租户创建成功')
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <Plus className="size-4" />
        创建租户
      </Button>
      <FormDrawer open={open} onOpenChange={setOpen} title="创建租户" loading={loading} onSubmit={handleSubmit} submitLabel="创建">
        <div className="space-y-2">
          <Label>租户名称</Label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            placeholder="如：Acme Corp"
          />
        </div>
      </FormDrawer>
    </>
  )
}
