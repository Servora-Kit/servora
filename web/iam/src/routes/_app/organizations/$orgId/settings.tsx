import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { iamClients } from '#/api'
import { toast } from '#/lib/toast'
import { DataState } from '#/components/data-state'
import { ConfirmDialog } from '#/components/confirm-dialog'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import { Separator } from '#/components/ui/separator'

export const Route = createFileRoute('/_app/organizations/$orgId/settings')({
  component: OrgSettingsPage,
})

function OrgSettingsPage() {
  const { orgId } = Route.useParams()
  const navigate = useNavigate()

  const { data: org, isLoading, isError, refetch } = useQuery({
    queryKey: ['organization', orgId],
    queryFn: () => iamClients.organization.GetOrganization({ id: orgId }),
  })

  const [displayName, setDisplayName] = useState('')
  const [saving, setSaving] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)

  const orgData = org?.organization
  const initialized = orgData && !displayName
  if (initialized) {
    setDisplayName(orgData.displayName ?? '')
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    try {
      await iamClients.organization.UpdateOrganization({ id: orgId, name: org?.organization?.name ?? '', displayName, type: 'COMPANY', sort: 0 })
      void refetch()
      toast.success('保存成功')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete() {
    await toast.promise(
      iamClients.organization.DeleteOrganization({ id: orgId }).then(() =>
        navigate({ to: '/organizations' }),
      ),
      { loading: '删除中...', success: '组织已删除' },
    )
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">组织设置</h1>

      <DataState isLoading={isLoading} isError={isError} isEmpty={!org} onRetry={() => void refetch()}>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">基本信息</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSave} className="space-y-4">
              <div className="space-y-2">
                <Label>显示名</Label>
                <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
              </div>
              <Button type="submit" disabled={saving}>
                {saving ? '保存中...' : '保存'}
              </Button>
            </form>
          </CardContent>
        </Card>

        <Separator />

        <Card className="border-destructive/50">
          <CardHeader>
            <CardTitle className="text-base text-destructive">危险操作</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="mb-3 text-sm text-muted-foreground">
              删除组织将同时移除所有成员关系。此操作不可撤销。
            </p>
            <Button variant="destructive" onClick={() => setDeleteOpen(true)}>
              删除组织
            </Button>
          </CardContent>
        </Card>
      </DataState>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="删除组织"
        description={`请输入组织名称「${org?.organization?.name ?? ''}」以确认删除。`}
        onConfirm={handleDelete}
        destructive
        confirmLabel="删除"
        confirmInput={org?.organization?.name ?? ''}
      />
    </div>
  )
}
