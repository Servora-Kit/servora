/**
 * Kratos 错误解析工具
 *
 * Kratos 统一错误格式：{ code, reason, message, metadata? }
 * 所有使用 protoc-gen-go-errors 生成的服务均输出此格式。
 */
import type { ApiError } from "./request.js";

export interface KratosErrorBody {
  code: number;
  reason: string;
  message: string;
  metadata?: Record<string, string>;
}

/**
 * 从 ApiError 中提取 Kratos 结构化错误体。
 * 非 HTTP 错误、或响应体不符合格式时返回 null。
 */
export function parseKratosError(err: ApiError): KratosErrorBody | null {
  if (err.kind !== "http" || !err.responseBody) return null;
  const body = err.responseBody as Record<string, unknown>;
  if (
    typeof body.code !== "number" ||
    typeof body.reason !== "string" ||
    typeof body.message !== "string"
  )
    return null;
  return body as unknown as KratosErrorBody;
}

/**
 * 判断 ApiError 是否为指定的 Kratos reason。
 *
 * @example
 * if (isKratosReason(err, 'EMAIL_NOT_VERIFIED')) { ... }
 */
export function isKratosReason(err: ApiError, reason: string): boolean {
  return parseKratosError(err)?.reason === reason;
}

/**
 * 提取用户可读的错误消息，含 network / timeout 降级处理。
 *
 * @param fallback 当无法提取有效消息时的兜底文案，默认 '操作失败'
 */
export function kratosMessage(err: ApiError, fallback = "操作失败"): string {
  if (err.kind === "network") return "网络连接失败，请检查网络设置";
  if (err.kind === "timeout") return "请求超时，请稍后重试";
  return (
    parseKratosError(err)?.message ??
    (err.httpStatus ? `请求失败（HTTP ${err.httpStatus}）` : fallback)
  );
}
