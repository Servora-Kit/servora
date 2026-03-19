import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { iamClients } from '#/api'
import { toast } from '#/lib/toast'
import { DataState } from '#/components/data-state'
import { ConfirmDialog } from '#/components/confirm-dialog'
import { Button } from '#/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import { Separator } from '#/components/ui/separator'

export const Route = createFileRoute('/_app/users/$userId')({
  component: UserDetailPage,
})

function UserDetailPage() {
  const { userId } = Route.useParams()
  const navigate = useNavigate()

  const { data: user, isLoading, isError, refetch } = useQuery({
    queryKey: ['user', userId],
    queryFn: () => iamClients.user.GetUser({ id: userId }),
  })

  const [deleteOpen, setDeleteOpen] = useState(false)

  async function handleDelete() {
    await toast.promise(
      iamClients.user.PurgeUser({ id: userId }).then(() => navigate({ to: '/users' })),
      { loading: '删除中...', success: '用户已删除' },
    )
  }

  return (
    <div className="space-y-6">
      <DataState isLoading={isLoading} isError={isError} isEmpty={!user} onRetry={() => void refetch()}>
        <h1 className="text-xl font-semibold">{user?.user?.username}</h1>
        <Card>
          <CardHeader><CardTitle className="text-base">用户信息</CardTitle></CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div className="flex gap-2"><span className="w-20 text-muted-foreground">用户名</span><span>{user?.user?.username}</span></div>
            <div className="flex gap-2"><span className="w-20 text-muted-foreground">邮箱</span><span>{user?.user?.email}</span></div>
            <div className="flex gap-2"><span className="w-20 text-muted-foreground">角色</span><span>{user?.user?.role}</span></div>
          </CardContent>
        </Card>
        <Separator />
        <Card className="border-destructive/50">
          <CardHeader><CardTitle className="text-base text-destructive">危险操作</CardTitle></CardHeader>
          <CardContent>
            <p className="mb-3 text-sm text-muted-foreground">删除用户将级联删除所有成员关系。</p>
            <Button variant="destructive" onClick={() => setDeleteOpen(true)}>删除用户</Button>
          </CardContent>
        </Card>
      </DataState>
      <ConfirmDialog open={deleteOpen} onOpenChange={setDeleteOpen} title="删除用户" description="确认删除此用户？此操作不可撤销。" onConfirm={handleDelete} destructive confirmLabel="删除" />
    </div>
  )
}
