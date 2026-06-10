/**
 * @servora/proto-utils — Proto/Kratos API 查询工具
 *
 * Kratos gRPC-gateway 的 ListRequest 约定：
 *   filter   = JSON 序列化的过滤条件对象，空值字段会被省略
 *   order_by = JSON 序列化的排序字段数组，前缀 "-" 表示降序
 *   update_mask = 逗号分隔的字段名，用于 Proto FieldMask 部分更新
 *
 * 所有函数均返回新值，不修改入参。
 */

/** 过滤掉 null、undefined、空字符串的字段 */
function omitEmpty(obj: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(obj).filter(
      ([, v]) => v !== null && v !== undefined && v !== "",
    ),
  );
}

/**
 * 将过滤条件序列化为 Kratos ListRequest.filter 字符串。
 *
 * - null/undefined 入参返回 undefined（不传 filter 参数）
 * - 自动过滤空值字段（null、undefined、空字符串）
 * - 过滤后为空对象时返回 undefined
 *
 * @example
 * makeFilter({ name: 'foo', status: null })
 * // → '{"name":"foo"}'
 *
 * makeFilter({})
 * // → undefined
 */
export function makeFilter(
  fields?: null | Record<string, unknown>,
): string | undefined {
  if (fields == null) return undefined;
  const cleaned = omitEmpty(fields);
  if (Object.keys(cleaned).length === 0) return undefined;
  return JSON.stringify(cleaned);
}

/**
 * 将排序字段序列化为 Kratos ListRequest.order_by 字符串。
 *
 * - 前缀 "-" 表示降序，无前缀表示升序
 * - 默认按 "-created_at" 降序（当入参为 null/undefined 时）
 * - 空数组时返回 undefined
 *
 * @example
 * makeOrderBy(['-created_at', 'name'])
 * // → '["-created_at","name"]'
 *
 * makeOrderBy()
 * // → '["-created_at"]'
 */
export const DEFAULT_ORDER_BY = ["-created_at"];

export function makeOrderBy(fields?: null | string[]): string | undefined {
  if (fields == null) return undefined;
  if (fields.length === 0) return undefined;
  return JSON.stringify(fields);
}

/**
 * 生成 Proto FieldMask 字符串，用于 UpdateRequest.update_mask。
 *
 * - 自动包含 "id" 字段（Proto UpdateRequest 的标准约定）
 * - 去重，保持传入顺序，id 追加到末尾
 * - 不修改入参数组
 *
 * @example
 * makeUpdateMask(['name', 'avatar'])
 * // → 'name,avatar,id'
 *
 * makeUpdateMask(['name', 'id', 'avatar'])
 * // → 'name,avatar,id'  （id 不重复出现）
 */
export function makeUpdateMask(fields: string[]): string {
  const unique = [...new Set(fields.filter((f) => f !== "id"))];
  return [...unique, "id"].join(",");
}
