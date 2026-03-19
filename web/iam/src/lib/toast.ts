/**
 * 统一消息提示工具，基于 sonner 封装。
 *
 * 设计原则：
 * - toast.promise()  → 自动处理 loading / success / error，并标记 error 已处理，防止全局 onError 重复弹出
 * - toast.fromApiError() → 全局 onError 钩子调用，自动解析后端 reason/message 字段
 * - 401 / 已处理的 error 静默忽略
 */

import { toast as sonner } from 'sonner'
import type { ExternalToast } from 'sonner'
import type { ApiError } from '@servora/api-client/request'

// ---------- 防重复机制 ----------
// 用 WeakSet 标记已被 toast.promise 处理过的 error，避免全局 onError 再次弹出
const _handled = new WeakSet<Error>()

function _mark(err: unknown) {
  if (err instanceof Error) _handled.add(err)
}

export function isHandledError(err: unknown): boolean {
  return err instanceof Error && _handled.has(err)
}

// ---------- 后端错误解析 ----------
export interface ApiErrorBody {
  code?: number
  reason?: string
  message?: string
}

function extractMessage(err: ApiError): string {
  if (err.kind === 'network') return '网络连接失败，请检查网络设置'
  if (err.kind === 'timeout') return '请求超时，请稍后重试'

  const body = err.responseBody as ApiErrorBody | null | undefined
  if (body?.message) return body.message
  if (err.httpStatus) return `请求失败（HTTP ${err.httpStatus}）`
  return '未知错误'
}

// ---------- 对外 API ----------
const toast = {
  success(message: string, opts?: ExternalToast) {
    sonner.success(message, opts)
  },

  error(message: string, opts?: ExternalToast) {
    sonner.error(message, opts)
  },

  warning(message: string, opts?: ExternalToast) {
    sonner.warning(message, opts)
  },

  info(message: string, opts?: ExternalToast) {
    sonner.info(message, opts)
  },

  /**
   * 包装 Promise，自动显示 loading → success/error toast。
   * error 会被标记为已处理，全局 onError 不会重复弹出。
   */
  promise<T>(
    promise: Promise<T>,
    opts: {
      loading: string
      success: string | ((data: T) => string)
      error?: string | ((err: unknown) => string)
    },
  ): Promise<T> {
    // 标记 error，防止全局 onError 重复 toast
    const wrapped = promise.catch((err: unknown) => {
      _mark(err)
      throw err
    })

    void sonner.promise(wrapped, {
      loading: opts.loading,
      success: opts.success as string,
      error: opts.error
        ? typeof opts.error === 'string'
          ? opts.error
          : (err: unknown) => (opts.error as (e: unknown) => string)(err)
        : (err: unknown) => {
            const body = (err as Partial<ApiError>)?.responseBody as
              | ApiErrorBody
              | null
              | undefined
            return body?.message ?? '操作失败'
          },
    })

    // 返回原始 promise（让调用方可以 await）
    return promise
  },

  /**
   * 全局 onError 钩子调用。
   * - 401 由登录过期 Dialog 处理，静默忽略
   * - 已被 toast.promise 标记的 error 静默忽略
   */
  fromApiError(err: ApiError) {
    if (err.httpStatus === 401) return
    if (isHandledError(err)) return
    sonner.error(extractMessage(err))
  },
}

export { toast }
