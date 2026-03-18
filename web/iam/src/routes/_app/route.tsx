import { useEffect } from 'react'
import { createFileRoute, Outlet, redirect, useNavigate } from '@tanstack/react-router'
import { AppShell } from '#/layout/app-shell'
import { LoginExpiredDialog } from '#/components/login-expired-dialog'
import { isAuthenticated } from '#/stores/auth'

// Single unified layout for all authenticated users.
// Menu visibility is controlled entirely by backend permission codes.
export const Route = createFileRoute('/_app')({
  beforeLoad: ({ location }) => {
    if (typeof window === 'undefined') return
    if (!isAuthenticated()) {
      throw redirect({ to: '/login' as string, search: { redirect: location.pathname } })
    }
  },
  component: AppLayout,
})

function AppLayout() {
  const navigate = useNavigate()

  useEffect(() => {
    if (!isAuthenticated()) {
      void navigate({ to: '/login' as string, search: { redirect: window.location.pathname } })
    }
  }, [navigate])

  if (typeof window !== 'undefined' && !isAuthenticated()) {
    return null
  }

  return (
    <AppShell>
      <Outlet />
      <LoginExpiredDialog />
    </AppShell>
  )
}
