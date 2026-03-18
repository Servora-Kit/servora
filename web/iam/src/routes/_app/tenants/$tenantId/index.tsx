import { createFileRoute, Link } from '@tanstack/react-router'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import { Button } from '#/components/ui/button'

export const Route = createFileRoute('/_app/tenants/$tenantId/')({
  component: TenantDetailPage,
})

function TenantDetailPage() {
  const { tenantId } = Route.useParams()

  return (
    <div className="space-y-4">
      <header className="flex items-start justify-between gap-4">
        <h1 className="text-xl font-semibold">租户详情</h1>
        <Button variant="outline" size="sm" asChild>
          <Link to="/tenants/$tenantId/members" params={{ tenantId }}>
            成员管理
          </Link>
        </Button>
      </header>
      <Card>
        <CardHeader><CardTitle className="text-base">基本信息</CardTitle></CardHeader>
        <CardContent className="text-sm">
          <div className="flex gap-2">
            <span className="w-20 text-muted-foreground">ID</span>
            <span className="font-mono text-xs">{tenantId}</span>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
