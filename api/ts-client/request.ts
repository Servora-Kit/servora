/**
 * @servora/api-client — 通用 HTTP 请求处理器
 *
 * 为 proto 生成的 TypeScript client 提供标准 RequestHandler 实现，包含：
 * - Bearer token 自动注入
 * - Token 自动刷新（单例防重复）
 * - 结构化 ApiError（区分 http / network / timeout）
 * - 全局 onError 回调（延迟到宏任务，避免与 toast.promise 竞态）
 *
 * 消费方需自行安装 peer dependency: ofetch
 */
import { ofetch } from 'ofetch'
import type { FetchOptions } from 'ofetch'

export type RequestType = {
  path: string
  method: string
  body: string | null
}

export type RequestMeta = {
  service: string
  method: string
}

export type RequestHandler = (
  request: RequestType,
  meta: RequestMeta,
) => Promise<unknown>

export interface TokenStore {
  getAccessToken: () => string | null
  getRefreshToken: () => string | null
  setTokens: (accessToken: string, refreshToken: string) => void
  clear: () => void
}

export interface RequestHandlerOptions {
  baseUrl?: string
  tokenStore?: TokenStore
  contextHeaders?: (meta: RequestMeta) => Record<string, string>
  timeoutMs?: number
  onError?: (error: ApiError, meta: RequestMeta) => void
  autoRefreshToken?: boolean
}

export type ApiErrorKind = 'http' | 'network' | 'timeout'

export class ApiError extends Error {
  readonly kind: ApiErrorKind
  readonly httpStatus?: number
  readonly responseBody?: unknown
  readonly service: string
  readonly method: string

  constructor(opts: {
    kind: ApiErrorKind
    message: string
    httpStatus?: number
    responseBody?: unknown
    service: string
    method: string
    cause?: unknown
  }) {
    super(opts.message, { cause: opts.cause })
    this.name = 'ApiError'
    this.kind = opts.kind
    this.httpStatus = opts.httpStatus
    this.responseBody = opts.responseBody
    this.service = opts.service
    this.method = opts.method
  }
}

function ensureLeadingSlash(path: string): string {
  return path.startsWith('/') ? path : `/${path}`
}

const REFRESH_PATH = '/v1/auth/refresh-token'
const AUTH_PATH_PREFIX = '/v1/auth/'

export function createRequestHandler(
  options: RequestHandlerOptions = {},
): RequestHandler {
  const {
    baseUrl = '',
    tokenStore,
    contextHeaders,
    timeoutMs = 30_000,
    onError,
    autoRefreshToken = false,
  } = options

  let refreshPromise: Promise<boolean> | null = null

  async function tryRefreshToken(): Promise<boolean> {
    if (!tokenStore) return false
    const refreshToken = tokenStore.getRefreshToken()
    if (!refreshToken) return false

    if (refreshPromise) return refreshPromise

    refreshPromise = (async () => {
      try {
        const data = await ofetch<{
          accessToken: string
          refreshToken: string
        }>(REFRESH_PATH, {
          baseURL: baseUrl,
          method: 'POST',
          body: { refreshToken },
          timeout: timeoutMs,
        })
        tokenStore.setTokens(data.accessToken, data.refreshToken)
        return true
      } catch {
        tokenStore.clear()
        return false
      } finally {
        refreshPromise = null
      }
    })()

    return refreshPromise
  }

  async function doRequest(
    request: RequestType,
    meta: RequestMeta,
    isRetry = false,
  ): Promise<unknown> {
    const headers: Record<string, string> = {
      Accept: 'application/json',
    }

    if (request.body) {
      headers['Content-Type'] = 'application/json'
    }

    if (tokenStore) {
      const token = tokenStore.getAccessToken()
      if (token) {
        headers['Authorization'] = `Bearer ${token}`
      }
    }

    if (contextHeaders) {
      Object.assign(headers, contextHeaders(meta))
    }

    const fetchOptions: FetchOptions = {
      baseURL: baseUrl,
      method: request.method as FetchOptions['method'],
      headers,
      timeout: timeoutMs,
    }

    if (request.body) {
      fetchOptions.body = request.body
    }

    try {
      return await ofetch(ensureLeadingSlash(request.path), fetchOptions)
    } catch (err: unknown) {
      const fetchError = err as {
        response?: { status: number; _data?: unknown }
        message?: string
      }

      if (fetchError.response) {
        const status = fetchError.response.status
        const body = fetchError.response._data

        const apiErr = new ApiError({
          kind: 'http',
          message: `HTTP ${status} on ${meta.service}.${meta.method}`,
          httpStatus: status,
          responseBody: body,
          service: meta.service,
          method: meta.method,
          cause: err,
        })

        const normalizedPath = ensureLeadingSlash(request.path)
        if (
          autoRefreshToken &&
          status === 401 &&
          !isRetry &&
          !normalizedPath.startsWith(AUTH_PATH_PREFIX)
        ) {
          const refreshed = await tryRefreshToken()
          if (refreshed) {
            return doRequest(request, meta, true)
          }
        }

        // 延迟到宏任务再调 onError，让 promise 链先跑完（含 toast.promise 的 _mark），
        // 避免全局 handler 与 sonner.promise error callback 同时弹出双重 toast。
        if (onError) {
          const _err = apiErr
          setTimeout(() => onError(_err, meta), 0)
        }
        throw apiErr
      }

      const message = (fetchError.message ?? '').toLowerCase()
      const kind: ApiErrorKind = message.includes('timeout')
        ? 'timeout'
        : 'network'

      const apiErr = new ApiError({
        kind,
        message: `${kind} error on ${meta.service}.${meta.method}: ${fetchError.message}`,
        service: meta.service,
        method: meta.method,
        cause: err,
      })
      if (onError) {
        const _err = apiErr
        setTimeout(() => onError(_err, meta), 0)
      }
      throw apiErr
    }
  }

  return doRequest
}
