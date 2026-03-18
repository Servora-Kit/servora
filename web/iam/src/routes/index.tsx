import { createFileRoute, redirect } from '@tanstack/react-router'
import { isAuthenticated } from '#/stores/auth'

export const Route = createFileRoute('/')({
  beforeLoad: () => {
    if (!isAuthenticated()) {
      throw redirect({ to: '/login' as string })
    }
    throw redirect({ to: '/dashboard' as string })
  },
})
