export class ResourceNameError extends Error {
  override readonly name = "ResourceNameError";

  constructor(message: string) {
    super(message);
  }
}

export type FieldCollection =
  | readonly string[]
  | Readonly<Record<string, string>>;

export type FieldOf<Fields extends FieldCollection> = Fields extends readonly (
  infer Field extends string
)[]
  ? Field
  : Fields extends Readonly<Record<string, infer Field extends string>>
    ? Field
    : never;

export type FieldKeyOf<Fields extends FieldCollection> =
  Fields extends readonly (infer Field extends string)[]
    ? Field
    : Fields extends Readonly<Record<infer Key extends string, string>>
      ? Key
      : never;

function fieldEntries(fields: FieldCollection): ReadonlyArray<readonly [string, string]> {
  return Array.isArray(fields)
    ? fields.map((field) => [field, field] as const)
    : Object.entries(fields as Readonly<Record<string, string>>);
}

function fieldValues(fields: FieldCollection): readonly string[] {
  return fieldEntries(fields).map(([, path]) => path);
}

function allowedFields(fields: FieldCollection): ReadonlySet<string> {
  return new Set(fieldValues(fields));
}

/**
 * Builds a comma-separated Proto FieldMask from fields present on values.
 * Presence is based on own keys, so an explicit `undefined` still selects a
 * field and can represent a Clear intent at the caller boundary.
 */
export function makeUpdateMask<
  const Fields extends FieldCollection,
  Values extends Partial<Record<FieldKeyOf<Fields>, unknown>>,
>(fields: Fields, values: Values): string {
  const entries = fieldEntries(fields);
  const allowedKeys = new Set(entries.map(([key]) => key));
  const selected = new Set(Object.keys(values));
  const unknown = [...selected].filter((field) => !allowedKeys.has(field));
  if (unknown.length > 0) {
    throw new RangeError(`unknown update field: ${unknown.join(", ")}`);
  }
  return entries
    .filter(([key]) => selected.has(key))
    .map(([, path]) => path)
    .join(",");
}

export type FilterOperator = "=" | "!=" | "<" | "<=" | ">" | ">=" | ":";

const rawFilterValueSymbol: unique symbol = Symbol("RawFilterValue");

export type RawFilterValue = Readonly<{
  [rawFilterValueSymbol]: true;
  text: string;
}>;

/** Use for validated enum symbols or AIP functions such as duration("5s"). */
export function rawFilterValue(text: string): RawFilterValue {
  if (text.length === 0) {
    throw new RangeError("raw filter value must not be empty");
  }
  return Object.freeze({ [rawFilterValueSymbol]: true as const, text });
}

export type FilterValue =
  | string
  | number
  | bigint
  | boolean
  | Date
  | null
  | RawFilterValue;

export type FilterExpression<Field extends string> =
  | Readonly<{
      field: Field;
      operator: FilterOperator;
      value: FilterValue;
    }>
  | Readonly<{ and: readonly FilterExpression<Field>[] }>
  | Readonly<{ or: readonly FilterExpression<Field>[] }>;

/** Serializes a typed expression into the AIP-160 filter text accepted by CRUD. */
export function buildFilter<const Fields extends FieldCollection>(
  fields: Fields,
  expression?: FilterExpression<FieldOf<Fields>> | null,
): string | undefined {
  if (expression == null) return undefined;
  return renderFilter(expression, allowedFields(fields));
}

function renderFilter(
  expression: FilterExpression<string>,
  fields: ReadonlySet<string>,
): string {
  if ("field" in expression) {
    if (!fields.has(expression.field)) {
      throw new RangeError(`unknown filter field: ${expression.field}`);
    }
    if (expression.value === null && !["=", "!="].includes(expression.operator)) {
      throw new RangeError("null filter values only support = and !=");
    }
    return `${expression.field} ${expression.operator} ${renderFilterValue(expression.value)}`;
  }
  const [operator, children] = "and" in expression
    ? (["AND", expression.and] as const)
    : (["OR", expression.or] as const);
  if (children.length === 0) {
    throw new RangeError(`${operator} filter group must not be empty`);
  }
  return `(${children.map((child) => renderFilter(child, fields)).join(` ${operator} `)})`;
}

function renderFilterValue(value: FilterValue): string {
  if (value === null) return "null";
  if (typeof value === "string") return JSON.stringify(value);
  if (typeof value === "boolean" || typeof value === "bigint") {
    return value.toString();
  }
  if (typeof value === "number") {
    if (!Number.isFinite(value)) {
      throw new RangeError("filter numbers must be finite");
    }
    return value.toString();
  }
  if (value instanceof Date) {
    if (Number.isNaN(value.getTime())) {
      throw new RangeError("filter date must be valid");
    }
    return `timestamp(${JSON.stringify(value.toISOString())})`;
  }
  return value.text;
}

export type OrderDirection = "asc" | "desc";

export type OrderTerm<Field extends string> = Readonly<{
  field: Field;
  direction?: OrderDirection;
}>;

/** Serializes typed terms into canonical AIP-132 order_by text. */
export function buildOrderBy<const Fields extends FieldCollection>(
  fields: Fields,
  terms?: readonly OrderTerm<FieldOf<Fields>>[] | null,
): string | undefined {
  if (terms == null || terms.length === 0) return undefined;
  const allowed = allowedFields(fields);
  const seen = new Set<string>();
  return terms
    .map(({ field, direction = "asc" }) => {
      if (!allowed.has(field)) {
        throw new RangeError(`unknown order field: ${field}`);
      }
      if (seen.has(field)) {
        throw new RangeError(`duplicate order field: ${field}`);
      }
      seen.add(field);
      return direction === "desc" ? `${field} desc` : field;
    })
    .join(",");
}

export type Pager = Readonly<{
  pageToken?: string;
  exhausted: boolean;
}>;

export const firstPage: Pager = Object.freeze({ exhausted: false });

/** Returns the state for the response's next page without mutating prior state. */
export function advancePager(nextPageToken?: string | null): Pager {
  return nextPageToken
    ? Object.freeze({ pageToken: nextPageToken, exhausted: false })
    : Object.freeze({ exhausted: true });
}

/** Applies the current token to a cloned request object. */
export function applyPager<Request extends Readonly<Record<string, unknown>>>(
  pager: Pager,
  request: Request,
): Request & { pageToken?: string } {
  const next = { ...request } as Request & { pageToken?: string };
  if (pager.pageToken === undefined) {
    delete next.pageToken;
  } else {
    next.pageToken = pager.pageToken;
  }
  return next;
}
