import { env } from '#/env'
import { createIamClients } from '#/service/request/clients'
import type { ApiError, TokenStore } from '#/service/request/clients'
import { scopeStore, setCurrentOrganizationId, orgIdFromPath } from '#/stores/scope'

const ACCESS_TOKEN_KEY = 'iam_access_token'
const REFRESH_TOKEN_KEY = 'iam_refresh_token'

const SCOPE_ERROR_REASONS = new Set([
  'MISSING_ORGANIZATION_SCOPE',
  'INVALID_ORGANIZATION_ID',
  'INVALID_PROJECT_ID',
])

export const tokenStore: TokenStore = {
  getAccessToken() {
    return localStorage.getItem(ACCESS_TOKEN_KEY)
  },
  getRefreshToken() {
    return localStorage.getItem(REFRESH_TOKEN_KEY)
  },
  setTokens(accessToken: string, refreshToken: string) {
    localStorage.setItem(ACCESS_TOKEN_KEY, accessToken)
    localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken)
  },
  clear() {
    localStorage.removeItem(ACCESS_TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
  },
}

export const iamClients = createIamClients({
  baseUrl: env.VITE_API_BASE_URL ?? '',
  tokenStore,
  timeoutMs: 30_000,
  autoRefreshToken: true,
  contextHeaders: () => {
    const { currentOrganizationId, currentProjectId } = scopeStore.state
    const headers: Record<string, string> = {}
    if (currentOrganizationId) {
      headers['X-Organization-ID'] = currentOrganizationId
    }
    if (currentProjectId) {
      headers['X-Project-ID'] = currentProjectId
    }
    return headers
  },
  onError(error: ApiError) {
    if (error.httpStatus === 401) {
      tokenStore.clear()
      const currentPath = window.location.pathname
      window.location.href = `/login?redirect=${encodeURIComponent(currentPath)}`
      return
    }

    if (error.httpStatus === 400) {
      const body = error.responseBody as { reason?: string } | null | undefined
      const reason = body?.reason
      if (typeof reason === 'string' && SCOPE_ERROR_REASONS.has(reason)) {
        const orgId = orgIdFromPath(window.location.pathname)
        if (orgId) {
          setCurrentOrganizationId(orgId)
        }
      }
    }
  },
})
