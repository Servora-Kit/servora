export type ApiErrorKind = "http" | "network" | "timeout" | "cancelled";

export class ApiError extends Error {
  readonly kind: ApiErrorKind;
  readonly httpStatus?: number;
  readonly responseBody?: unknown;
  readonly service: string;
  readonly method: string;

  constructor(options: {
    kind: ApiErrorKind;
    message: string;
    httpStatus?: number;
    responseBody?: unknown;
    service: string;
    method: string;
    cause?: unknown;
  }) {
    super(options.message, { cause: options.cause });
    this.name = "ApiError";
    this.kind = options.kind;
    this.httpStatus = options.httpStatus;
    this.responseBody = options.responseBody;
    this.service = options.service;
    this.method = options.method;
  }
}

export interface KratosErrorBody {
  code: number;
  reason: string;
  message: string;
  metadata?: Record<string, string>;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isStringRecord(value: unknown): value is Record<string, string> {
  if (!isRecord(value)) return false;

  for (const key in value) {
    if (Object.hasOwn(value, key) && typeof value[key] !== "string") {
      return false;
    }
  }
  return true;
}

export function parseKratosErrorBody(value: unknown): KratosErrorBody | null {
  try {
    if (!isRecord(value)) return null;
    if (!Number.isInteger(value.code)) return null;
    if (typeof value.reason !== "string") return null;
    if (typeof value.message !== "string") return null;
    if (Object.hasOwn(value, "metadata") && !isStringRecord(value.metadata)) {
      return null;
    }

    return value as unknown as KratosErrorBody;
  } catch {
    return null;
  }
}

export function parseKratosError(error: ApiError): KratosErrorBody | null {
  if (error.kind !== "http") return null;
  return parseKratosErrorBody(error.responseBody);
}

export function isKratosReason(error: ApiError, reason: string): boolean {
  return parseKratosError(error)?.reason === reason;
}
