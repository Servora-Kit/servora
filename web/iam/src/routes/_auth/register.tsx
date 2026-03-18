import '@cap.js/widget'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import { iamClients } from '#/api'

export const Route = createFileRoute('/_auth/register')({
  component: RegisterPage,
})

function RegisterPage() {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [passwordConfirm, setPasswordConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [mounted, setMounted] = useState(false)
  const capTokenRef = useRef<string>('')
  const widgetRef = useRef<HTMLElement>(null)

  useEffect(() => { setMounted(true) }, [])

  // Cap widget 派发的是 "solve" 事件，detail 为 { token: string }
  const handleCapWidget = useCallback((node: HTMLElement | null) => {
    if (!node) return
    const listener = (e: Event) => {
      const token = (e as CustomEvent<{ token: string }>).detail?.token
      if (token) {
        capTokenRef.current = token
        setError('') // 完成验证后清除「请先完成人机验证」提示
      }
    }
    node.addEventListener('solve', listener)
  }, [])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')

    if (password !== passwordConfirm) {
      setError('两次输入的密码不一致')
      return
    }
    if (!capTokenRef.current) {
      setError('请先完成人机验证')
      return
    }

    setLoading(true)
    try {
      await iamClients.authn.SignupByEmail({
        name,
        email,
        password,
        passwordConfirm,
        capToken: capTokenRef.current,
      })
      void navigate({ to: '/register-success', search: { email } })
    } catch (err: unknown) {
      const apiErr = err as { responseBody?: { message?: string; reason?: string } }
      const reason = apiErr.responseBody?.reason
      if (reason === 'INVALID_CAPTCHA') {
        setError('人机验证已过期，请重新验证')
        capTokenRef.current = ''
        // Reset widget by re-mounting — trigger a re-render via key
        if (widgetRef.current) {
          const parent = widgetRef.current.parentElement
          if (parent) {
            const clone = widgetRef.current.cloneNode(true) as HTMLElement
            parent.replaceChild(clone, widgetRef.current)
          }
        }
      } else {
        setError(apiErr.responseBody?.message ?? '注册失败，请稍后重试')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-3xl font-bold text-foreground">创建账号</h1>
        <p className="mt-2 text-muted-foreground">注册 Servora IAM 管理平台</p>
      </div>

      {error && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="space-y-2">
          <Label htmlFor="name">用户名</Label>
          <Input
            id="name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="至少 5 个字符"
            required
            minLength={5}
            autoFocus
            className="h-10"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="email">邮箱</Label>
          <Input
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.com"
            required
            className="h-10"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="password">密码</Label>
          <Input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="6-20 个字符"
            required
            minLength={6}
            maxLength={20}
            className="h-10"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="passwordConfirm">确认密码</Label>
          <Input
            id="passwordConfirm"
            type="password"
            value={passwordConfirm}
            onChange={(e) => setPasswordConfirm(e.target.value)}
            placeholder="再次输入密码"
            required
            minLength={6}
            maxLength={20}
            className="h-10"
          />
        </div>

        {/* Cap PoW CAPTCHA — 仅客户端渲染以避免 SSR hydration mismatch */}
        <div className="flex justify-center">
          {mounted && (
            <cap-widget
              ref={(node: HTMLElement | null) => {
                ;(widgetRef as React.MutableRefObject<HTMLElement | null>).current = node
                handleCapWidget(node)
              }}
              data-cap-api-endpoint="/v1/cap/"
            />
          )}
        </div>

        <Button type="submit" className="mt-2 h-10 w-full" disabled={loading}>
          {loading ? '注册中...' : '注册'}
        </Button>
      </form>

      <p className="text-center text-sm text-muted-foreground">
        已有账号？{' '}
        <Link to="/login" search={{ redirect: '' }} className="font-medium text-primary hover:underline">
          立即登录
        </Link>
      </p>
    </div>
  )
}
