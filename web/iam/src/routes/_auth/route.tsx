import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_auth')({
  component: AuthLayout,
})

function AuthLayout() {
  return (
    <div className="flex min-h-dvh">
      <div className="relative hidden w-[48%] flex-col items-center justify-center bg-linear-to-br from-primary/90 via-primary to-primary/80 lg:flex">
        <div className="relative z-10 flex flex-col items-center gap-4 text-center">
          <div className="flex size-16 items-center justify-center rounded-2xl bg-white/15 text-white text-2xl font-bold backdrop-blur-sm">
            S
          </div>
          <h1 className="text-3xl font-bold tracking-tight text-white">
            Servora
          </h1>
          <p className="max-w-xs text-base text-white/80">
            身份与访问管理平台
          </p>
        </div>
        <div className="absolute inset-0 bg-[url('data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iNjAiIGhlaWdodD0iNjAiIHZpZXdCb3g9IjAgMCA2MCA2MCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48ZyBmaWxsPSJub25lIiBmaWxsLXJ1bGU9ImV2ZW5vZGQiPjxnIGZpbGw9IiNmZmYiIGZpbGwtb3BhY2l0eT0iLjA1Ij48cGF0aCBkPSJNMzYgMzRWMGgydjM0aDM0djJIMzZ6bS0yIDBoMnYyaC0ydi0yem0tMi0yaC0ydi0yaC0ydi0yaC0ydi0yaC0ydi0ySDJ2LTJINHYySDJ2MmgydjJoMnYyaDJ2Mmg0djJoMnYyaDJ2Mmg0di0yaDJ2LTJoMnYtMmgydi0yaDJ2LTJoMnYtMmgydi0yaDItMnYyaC0ydjJoLTJ2MmgtMnYyaC0ydjJoLTR2MmgtMnYyaC0ydjJoLTJ2Mmgtall2MnoiLz48L2c+PC9nPjwvc3ZnPg==')] opacity-30" />
      </div>

      <div className="flex flex-1 items-center justify-center bg-background p-6 lg:p-12">
        <div className="w-full max-w-md">
          <Outlet />
        </div>
      </div>
    </div>
  )
}
