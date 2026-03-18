import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { iamClients } from '#/api'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'

export const Route = createFileRoute('/_app/settings/security')({
  component: SecurityPage,
})

function SecurityPage() {
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setSuccess(false)

    if (newPassword !== confirmPassword) {
      setError('新密码和确认密码不匹配')
      return
    }

    setLoading(true)
    try {
      await iamClients.authn.ChangePassword({
        currentPassword,
        newPassword,
        newPasswordConfirm: confirmPassword,
      })
      setSuccess(true)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err: unknown) {
      const apiErr = err as { responseBody?: { message?: string } }
      setError(apiErr.responseBody?.message ?? '修改密码失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">安全设置</h1>
      <Card>
        <CardHeader><CardTitle className="text-base">修改密码</CardTitle></CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && <div className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div>}
            {success && <div className="rounded-md bg-success/10 px-3 py-2 text-sm text-success">密码修改成功</div>}
            <div className="space-y-2">
              <Label>当前密码</Label>
              <Input type="password" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)} required />
            </div>
            <div className="space-y-2">
              <Label>新密码</Label>
              <Input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} required />
            </div>
            <div className="space-y-2">
              <Label>确认新密码</Label>
              <Input type="password" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} required />
            </div>
            <Button type="submit" disabled={loading}>{loading ? '修改中...' : '修改密码'}</Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
