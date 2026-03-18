import { useStore } from '@tanstack/react-store'
import { accessStore } from '#/stores/access'

export function usePermissions() {
  const codes = useStore(accessStore, (s) => s.permissionCodes)

  const hasPermission = (code: string): boolean => codes.includes(code)
  const hasAnyPermission = (...checkCodes: string[]): boolean =>
    checkCodes.some((c) => codes.includes(c))
  const hasAllPermissions = (...checkCodes: string[]): boolean =>
    checkCodes.every((c) => codes.includes(c))

  return { hasPermission, hasAnyPermission, hasAllPermissions, codes }
}
