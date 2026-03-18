import { createFileRoute } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { iamClients } from '#/api'
import { Page } from '#/components/page'
import { Button } from '#/components/ui/button'
import { FormDrawer } from '#/components/form-drawer'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '#/components/ui/select'
import { Label } from '#/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import { ArrowRightLeft } from 'lucide-react'
import { toast } from '#/lib/toast'

export const Route = createFileRoute('/_app/tenants/$tenantId/members')({
  component: TenantOwnershipPage,
})

function TenantOwnershipPage() {
  const { tenantId } = Route.useParams()
  const queryClient = useQueryClient()
  const [transferOpen, setTransferOpen] = useState(false)
  const [transferTargetId, setTransferTargetId] = useState<string | undefined>(undefined)
  const [transferLoading, setTransferLoading] = useState(false)

  const { data: tenantData, isLoading } = useQuery({
    queryKey: ['platform-tenant', tenantId],
    queryFn: () => iamClients.tenant.GetTenant({ id: tenantId }),
  })

  const tenant = tenantData?.tenant

  const { data: usersData } = useQuery({
    queryKey: ['platform-tenant-users', tenantId],
    queryFn: () =>
      iamClients.user.ListUsers({
        pagination: { page: { page: 1, pageSize: 100 } },
      }),
    enabled: !!tenantId,
    staleTime: 60_000,
  })
  const userOptions = (usersData?.users ?? []).filter(
    (u) => u.id !== tenant?.ownerUserId,
  )

  function invalidate() {
    void queryClient.invalidateQueries({ queryKey: ['platform-tenant', tenantId] })
  }

  async function handleTransferOwnership() {
    if (!transferTargetId) return
    setTransferLoading(true)
    try {
      await iamClients.tenant.TransferOwnership({
        tenantId,
        newOwnerUserId: transferTargetId,
      })
      setTransferOpen(false)
      setTransferTargetId(undefined)
      invalidate()
      toast.success('所有权已转让')
    } finally {
      setTransferLoading(false)
    }
  }

  return (
    <Page
      title="租户所有权"
      description="查看和管理租户所有权归属。租户成员由其所属组织决定。"
      extra={
        <Button variant="outline" onClick={() => setTransferOpen(true)}>
          <ArrowRightLeft className="size-4" />
          转让所有权
        </Button>
      }
    >
      {isLoading ? (
        <div className="text-muted-foreground text-sm">加载中…</div>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">当前 Owner</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1 text-sm">
            <div className="flex gap-2">
              <span className="w-24 text-muted-foreground">租户 ID</span>
              <span className="font-mono text-xs">{tenant?.id ?? '-'}</span>
            </div>
            <div className="flex gap-2">
              <span className="w-24 text-muted-foreground">Owner 用户 ID</span>
              <span className="font-mono text-xs">{tenant?.ownerUserId ?? '-'}</span>
            </div>
          </CardContent>
        </Card>
      )}

      <div className="mt-4 rounded-md border bg-muted/40 p-4 text-sm text-muted-foreground">
        <p className="font-medium text-foreground">关于租户成员</p>
        <p className="mt-1">
          租户没有独立的成员列表。租户的所有用户来自其旗下各组织的成员，请前往各组织的成员页进行管理。
        </p>
      </div>

      <FormDrawer
        open={transferOpen}
        onOpenChange={setTransferOpen}
        title="转让租户所有权"
        loading={transferLoading}
        onSubmit={handleTransferOwnership}
        submitLabel="确认转让"
      >
        <p className="text-sm text-muted-foreground">
          转让后原 owner 将失去租户控制权。请选择接受所有权的用户（必须是此租户下任意组织的成员）。
        </p>
        <div className="space-y-2">
          <Label>选择新 Owner</Label>
          <Select value={transferTargetId} onValueChange={setTransferTargetId}>
            <SelectTrigger>
              <SelectValue placeholder="选择目标用户" />
            </SelectTrigger>
            <SelectContent>
              {userOptions.length === 0 && (
                <div className="py-4 text-center text-xs text-muted-foreground">暂无可选用户</div>
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
      </FormDrawer>
    </Page>
  )
}
