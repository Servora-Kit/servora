import { env } from '#/env'
import { createIamClients } from '#/service/request/clients'
import type { ApiError, TokenStore } from '#/service/request/clients'
import { scopeStore, setCurrentOrganizationId, orgIdFromPath, clearScope } from '#/stores/scope'
import { authStore, setTokens as storeSetTokens, setLoginExpired, clearAuth } from '#/stores/auth'
import { toast } from '#/lib/toast'

const SCOPE_ERROR_REASONS = new Set([
  'MISSING_TENANT_ID',
  'INVALID_TENANT_ID',
  'MISSING_ORGANIZATION_SCOPE',
  'INVALID_ORGANIZATION_ID',
])

export const tokenStore: TokenStore = {
  getAccessToken() {
    return authStore.state.accessToken
  },
  getRefreshToken() {
    return authStore.state.refreshToken
  },
  setTokens(accessToken: string, refreshToken: string) {
    storeSetTokens(accessToken, refreshToken)
  },
  clear() {
    clearAuth()
  },
}

export const iamClients = createIamClients({
  baseUrl: env.VITE_API_BASE_URL ?? '',
  tokenStore,
  timeoutMs: 30_000,
  autoRefreshToken: true,
  contextHeaders: () => {
    const { currentTenantId, currentOrganizationId } = scopeStore.state
    const headers: Record<string, string> = {}
    if (currentTenantId) {
      headers['X-Tenant-ID'] = currentTenantId
    }
    if (currentOrganizationId) {
      headers['X-Organization-ID'] = currentOrganizationId
    }
    return headers
  },
  onError(error: ApiError) {
    if (error.httpStatus === 401) {
      setLoginExpired(true)
      return
    }

    if (error.httpStatus === 403) {
      const body = error.responseBody as { reason?: string; message?: string } | null | undefined
      const reason = body?.reason
      if (reason === 'AUTHZ_DENIED') {
        const msg = body?.message ?? ''
        const isScopeStale =
          msg.includes('not found') ||
          msg.includes('does not exist') ||
          msg.includes('invalid')
        if (isScopeStale) {
          clearScope()
          setLoginExpired(true)
          return
        }
        // Genuine permission denial — show a toast, don't force re-login.
        return
      }
    }

    if (error.httpStatus === 400) {
      const body = error.responseBody as { reason?: string } | null | undefined
      const reason = body?.reason
      if (typeof reason === 'string' && SCOPE_ERROR_REASONS.has(reason)) {
        if (typeof window !== 'undefined') {
          const orgId = orgIdFromPath(window.location.pathname)
          if (orgId) {
            setCurrentOrganizationId(orgId)
          }
        }
        // scope 错误静默处理，不弹 toast
        return
      }
    }

    // 全局兜底：展示后端错误（已被 toast.promise 标记过的会自动跳过）
    toast.fromApiError(error)
  },
})
