import { createFileRoute, Link, useSearch } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { Button } from '#/components/ui/button'
import { iamClients } from '#/api'

export const Route = createFileRoute('/_auth/verify-email')({
  validateSearch: (search: Record<string, unknown>) => ({
    token: (search.token as string) || '',
    email: (search.email as string) || '',
  }),
  component: VerifyEmailPage,
})

type Status = 'verifying' | 'success' | 'expired' | 'error'

function VerifyEmailPage() {
  const { token, email } = useSearch({ from: '/_auth/verify-email' })
  const [status, setStatus] = useState<Status>('verifying')
  const [resending, setResending] = useState(false)
  const [resendMsg, setResendMsg] = useState('')
  const verified = useRef(false)

  useEffect(() => {
    if (!token || verified.current) return
    verified.current = true

    iamClients.authn
      .VerifyEmail({ token })
      .then(() => setStatus('success'))
      .catch((err: unknown) => {
        const apiErr = err as { responseBody?: { reason?: string } }
        if (apiErr.responseBody?.reason === 'TOKEN_EXPIRED') {
          setStatus('expired')
        } else {
          setStatus('error')
        }
      })
  }, [token])

  async function handleResend() {
    if (!email) return
    setResending(true)
    setResendMsg('')
    try {
      await iamClients.authn.RequestEmailVerification({ email })
      setResendMsg('验证邮件已重新发送，请检查收件箱')
    } catch {
      setResendMsg('发送失败，请稍后重试')
    } finally {
      setResending(false)
    }
  }

  return (
    <div className="flex flex-col items-center gap-6 text-center">
      {status === 'verifying' && (
        <>
          <div className="flex size-20 items-center justify-center rounded-full bg-muted">
            <svg
              className="size-8 animate-spin text-primary"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                className="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                strokeWidth="4"
              />
              <path
                className="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
              />
            </svg>
          </div>
          <div>
            <h1 className="text-2xl font-bold text-foreground">正在验证邮箱</h1>
            <p className="mt-2 text-muted-foreground">请稍候…</p>
          </div>
        </>
      )}

      {status === 'success' && (
        <>
          <div className="flex size-20 items-center justify-center rounded-full bg-success/15">
            <svg
              className="size-10 text-success"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M5 13l4 4L19 7"
              />
            </svg>
          </div>
          <div>
            <h1 className="text-2xl font-bold text-foreground">邮箱验证成功</h1>
            <p className="mt-2 text-muted-foreground">
              您的邮箱已验证，现在可以登录了。
            </p>
          </div>
          <Link to="/login" search={{ redirect: '' }} className="w-full">
            <Button className="h-10 w-full">去登录</Button>
          </Link>
        </>
      )}

      {(status === 'expired' || status === 'error') && (
        <>
          <div className="flex size-20 items-center justify-center rounded-full bg-destructive/10">
            <svg
              className="size-10 text-destructive"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </div>
          <div>
            <h1 className="text-2xl font-bold text-foreground">
              {status === 'expired' ? '验证链接已过期' : '验证失败'}
            </h1>
            <p className="mt-2 text-muted-foreground">
              {status === 'expired'
                ? '该验证链接已过期（有效期 24 小时），请重新发送验证邮件。'
                : '验证链接无效，请检查邮件或重新发送。'}
            </p>
          </div>

          {resendMsg && (
            <p className="text-sm text-muted-foreground">{resendMsg}</p>
          )}

          <div className="flex w-full flex-col gap-3">
            {email && (
              <Button
                variant="outline"
                className="h-10 w-full"
                onClick={handleResend}
                disabled={resending}
              >
                {resending ? '发送中...' : '重新发送验证邮件'}
              </Button>
            )}
            <Link to="/login" search={{ redirect: '' }} className="w-full">
              <Button variant="ghost" className="h-10 w-full">
                去登录
              </Button>
            </Link>
          </div>
        </>
      )}
    </div>
  )
}
