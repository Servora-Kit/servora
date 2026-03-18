import { Store } from '@tanstack/store'
import type { rbacservicev1_MenuInfo } from '@servora/api-client/iam/service/v1/index'

export interface AccessState {
  permissionCodes: string[]
  accessMenus: rbacservicev1_MenuInfo[]
  loaded: boolean
}

export const accessStore = new Store<AccessState>({
  permissionCodes: [],
  accessMenus: [],
  loaded: false,
})

export function setPermissionCodes(codes: string[]): void {
  accessStore.setState((prev) => ({ ...prev, permissionCodes: codes }))
}

export function setAccessMenus(menus: rbacservicev1_MenuInfo[]): void {
  accessStore.setState((prev) => ({ ...prev, accessMenus: menus }))
}

export function setAccessLoaded(loaded: boolean): void {
  accessStore.setState((prev) => ({ ...prev, loaded }))
}

export function clearAccess(): void {
  accessStore.setState(() => ({ permissionCodes: [], accessMenus: [], loaded: false }))
}
