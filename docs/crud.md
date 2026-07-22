# CRUD 生态

Servora CRUD 是一组显式组合的资源 API 组件。它从 Proto resource descriptor 生成类型化资源名和字段身份，在 service 边界规范化 List、FieldMask 与字段生命周期，在 data 层把后端中立查询绑定到 Ent。

它不是自动生成 repository 的 Active Record 框架，也不接管授权、租户、事务、业务错误或存储模型。

## 组件与职责

| 组件 | 路径 | 职责 |
|---|---|---|
| 公共协议 | `api/protos/servora/crud/v1` | CRUD 框架错误 reason、服务端 page-token payload |
| Go 生成器 | `cmd/protoc-gen-servora-crud` | 资源 descriptor、typed field path、资源名 helper |
| Go runtime | `core/crud` | ResourcePlan、ListPreparer、FieldMask、字段生命周期、page token、响应清理 |
| 读映射 | `core/crud/mapper` | PO → 资源 PB 的同名映射、converter、改名和 post hook |
| Ent adapter | `contrib/db/entgo/crud` | ListFields、filter/order/keyset/Count、masked Clear |
| Ent 软删除 | `contrib/db/entgo/mixin` | tombstone 字段、默认过滤、Delete 改写、显式 bypass |
| TypeScript runtime | `@servora/proto-utils/crud` | update mask、filter/order builder、pager、资源名错误 |

标准数据流：

```text
RPC request
  -> service: ResourcePlan / ListPreparer
  -> biz: typed key + ListQuery + business scope
  -> data: Ent predicate + ListFields + scope fingerprint
  -> Ent adapter / explicit Ent mutation
  -> ResourceMapper
  -> service: ResourcePlan.ToResponse
```

所有静态组件都应在 provider 构造期创建一次。descriptor、mapper 或 ListFields 配置错误应阻止应用启动，不应延迟到第一条请求。

## 定义标准资源

资源必须完整声明 `google.api.resource`，并且有唯一的 `string name` `IDENTIFIER` 字段，才会进入标准 CRUD 生成路径。

```proto
import "google/api/field_behavior.proto";
import "google/api/resource.proto";
import "google/protobuf/timestamp.proto";

message User {
  option (google.api.resource) = {
    type: "example.servora.dev/User"
    pattern: "tenants/{tenant}/users/{user}"
    singular: "user"
    plural: "users"
  };

  string name = 1 [(google.api.field_behavior) = IDENTIFIER];
  optional string display_name = 2 [(google.api.field_behavior) = OPTIONAL];
  optional string email = 3 [(google.api.field_behavior) = REQUIRED];
  optional string tenant_plan = 4 [
    (google.api.field_behavior) = OPTIONAL,
    (google.api.field_behavior) = IMMUTABLE
  ];
  optional string nickname = 5 [(google.api.field_behavior) = OPTIONAL];
  optional string temporary_password = 6 [
    (google.api.field_behavior) = OPTIONAL,
    (google.api.field_behavior) = INPUT_ONLY
  ];
  string etag = 7;
  google.protobuf.Timestamp create_time = 100
      [(google.api.field_behavior) = OUTPUT_ONLY];
  google.protobuf.Timestamp update_time = 101
      [(google.api.field_behavior) = OUTPUT_ONLY];
  google.protobuf.Timestamp delete_time = 102
      [(google.api.field_behavior) = OUTPUT_ONLY];
  google.protobuf.Timestamp purge_time = 103
      [(google.api.field_behavior) = OUTPUT_ONLY];
}
```

客户端可写的 singular scalar 应使用 explicit presence。这样框架可以区分“未提供”“Set 零值”和 FieldMask 选中但值 absent 的 Clear。

生成链：

```bash
make gen     # Go message、service stub、CRUD companion
make gen.ts  # TypeScript message/client 与 CRUD companion
```

Go companion 提供：

- `UserCRUDDescriptor()`；
- `UserFields` 和 descriptor-backed `UserFieldPath`；
- `UserName`、`NewUserName`、`ParseUserName` 等 canonical name helper。

生成 companion 不依赖 `core/crud`，也不生成 service、biz、repository、Ent schema、setter、授权或事务代码。

## ResourcePlan 与 service 边界

在 service provider 中从生成 descriptor 构造不可变 Plan：

```go
plan := crud.MustBuildResourcePlan[*userv1.User](userv1.UserCRUDDescriptor())
```

service 使用 generated name helper 或 Plan 解析 canonical name；不要手拼资源名或字段路径。

### Create

```go
prepared, err := plan.PrepareCreate(req.GetUser())
if err != nil {
    return nil, err
}
created, err := usecase.CreateUser(ctx, key, prepared)
if err != nil {
    return nil, err
}
return plan.ToResponse(created)
```

`PrepareCreate`：

- 校验 Create 请求中 `REQUIRED` 字段存在且为非空值；
- 清除客户端写入的 `IDENTIFIER` 和 `OUTPUT_ONLY` 字段；
- 保留 `INPUT_ONLY` 值供 biz 处理；
- 返回克隆后的 `PreparedCreate`，不把 RPC wrapper 传入 biz。

### Update 与 FieldMask

```go
prepared, err := plan.PrepareUpdate(
    req.GetUser(),
    req.GetUpdateMask(),
    crud.UpdateOptions{
        AllowMissing: req.GetAllowMissing(),
        Etag:         req.GetUser().GetEtag(),
    },
)
if err != nil {
    return nil, err
}
```

Update 规则：

- 省略 mask 时，根据 explicit presence 生成隐式 mask；普通无 presence 标量的默认值不表示更新意图。
- 显式 mask 使用 Proto `snake_case` 路径；重复路径会规范化，祖先/后代重叠、未知路径和非法元素遍历会被拒绝。
- `*` 只能单独出现，并展开为全部可写 leaf。
- mask 命中 present 值表示 Set；命中 optional/message absent 表示 Clear；repeated/map 采用 Replace，空集合表示 Replace(empty)。
- `OUTPUT_ONLY` 输入不进入写 mask；`INPUT_ONLY` 可以参与写入，但会从响应清除。
- `IMMUTABLE` 字段不会进入可变写 mask。`PreparedUpdate.ValidateImmutable(current)` 负责验证调用方表达的同值比较意图。
- mask 命中 `REQUIRED` 字段时，该值仍必须 present 且非空。
- `AllowMissing` 与 `Etag` 只是规范化后的业务输入；upsert、并发控制和 not-found 分支由 biz 决定。

Create/Update 最终仍由具体 repository 调用 Ent 原生 setter 或 mutation。框架不会反射写入数据库。

### Response 清理

所有资源响应经过：

```go
clean, err := plan.ToResponse(resource)
items, err := plan.ToResponses(resources)
```

该步骤验证 canonical `name`、克隆资源并递归清除 `INPUT_ONLY` 字段。不要直接返回 repository 的资源对象。

## ListPreparer：RPC 查询到 ListQuery

service 只把标准 RPC 字段交给 `ListPreparer`：

```go
preparer, err := crud.NewListPreparer()
if err != nil {
    return err
}

query, err := preparer.PrepareList(plan, crud.ListInput{
    Collection:   req.GetParent(),
    PageSize:     req.GetPageSize(),
    PageToken:    req.GetPageToken(),
    Skip:         req.GetSkip(),
    Filter:       req.GetFilter(),
    OrderBy:      req.GetOrderBy(),
    IncludeTotal: req.GetIncludeTotal(),
})
```

`crud.ListQuery` 是不可变、后端中立的客户端查询意图。它不含 Ent predicate、授权范围或 SQL。biz 应将它和独立的业务 scope 一起传给 repository：

```go
type UserRepo interface {
    ListUsers(
        context.Context,
        crud.ListQuery,
        UserScope,
    ) (crud.ListResult[*userv1.User], error)
}
```

### Filter、order 和限制

Filter 使用 AIP-160 的确定性子集。实际可用字段和操作同时受 resource descriptor 与 repository `ListFields` 限制。客户端文本不会直接拼入 SQL。

默认安全限制：

- filter：8 KiB、128 AST 节点、8 层深度、64 个 OR term；
- order_by：2 KiB、8 个 term；
- page size：使用框架默认值与上限。

应用默认值和资源覆盖通过 `WithApplicationDefaults`、`WithResourceOverrides` 配置。关闭限制必须使用 `Unlimited*`、`WithoutMax*` 等显式 API；零值不是“无限制”的隐式表达。

### 分页语义

- `page_token` 是 opaque continuation token，客户端只能原样回传。
- `skip` 可单独使用，也可在 token 恢复 cursor 后额外跳过资源；它表示资源数量，不是页数。
- token fingerprint 绑定资源类型、collection、规范化 filter、最终稳定排序、比较 profile 和业务 scope；不绑定 `page_size` 或当前 `skip`。
- filter、order、collection、scope 或 tombstone 可见范围改变后，旧 token 会返回 `INVALID_PAGE_TOKEN`。
- token 不授予权限。每页查询都必须重新应用当前 authn/authz 与业务 scope。
- 默认 codec 是 deterministic Proto binary + unpadded Base64URL，未签名。需要完整性或保密性时，通过 `PageTokenCodec` 替换；不要让客户端依赖内部 payload。
- Timestamp cursor 同时保存 UTC instant 与原始整分钟时区 offset；offset 只用于重建 SQL keyset 参数，避免 SQLite 文本时间在 Proto UTC 归一化后改变续页比较值。非法或错配 offset 会在数据库访问前返回 `INVALID_PAGE_TOKEN`。

`include_total=true` 才执行 Count。`optional int64 total_size` 用 presence 区分“未计算”和“已计算且为 0”。Count 必须复用相同 collection、业务 scope 与 filter，但不应用 order、cursor、skip 或 page size。

## ResourceMapper：只做读投影

普通 PO → 资源 PB 使用 `core/crud/mapper`：

```go
resourceMapper, err := mapper.NewResourceMapper[*userv1.User, ent.User](
    mapper.WithResourceName(func(value *ent.User) (string, error) {
        return userv1.NewUserName(value.TenantID, value.ResourceID).Format()
    }),
)
```

默认反射复制同名且类型兼容的字段。其它情况显式组合：

- `WithConverters`：类型转换；
- `WithFieldMapping`：PO 字段改名到 generated typed field path；
- `WithPostToDTOHook`：relation、JSON、oneof、computed 等本地补充；
- `WithResourceName`：设置 canonical `IDENTIFIER` name。

`TryToDTO`/`TryToDTOs` 返回映射错误；`ToDTO`/`ToDTOs` 在程序合同破坏时 panic。批量转换保持 1:1 顺序并拒绝 nil 元素。

ResourceMapper 不提供反向写映射，不决定 filter/order，也不加载 relation。Create/Update 继续使用 repository 的显式 setter。

## Ent ListFields 与执行

`ListFields` 是一个资源在一个 repository 上实际开放的查询能力。它必须在 provider/NewRepo 中构造一次：

```go
fields, err := entcrud.NewListFields[*ent.User](
    entcrud.Columns(entuser.ValidColumn),
    entcrud.Bind(userv1.UserFields.DisplayName, entuser.FieldDisplayName).
        Filter().Order(),
    entcrud.Bind(userv1.UserFields.Email, entuser.FieldEmail).
        Filter(),
    entcrud.Bind(userv1.UserFields.Nickname, entuser.FieldNickname).
        Filter().Order().Nullable(),
    entcrud.Bind(userv1.UserFields.CreateTime, entuser.FieldCreateTime).
        Filter().Order(),
    entcrud.DefaultOrder(userv1.UserFields.CreateTime, crud.OrderDescending),
    entcrud.CursorKey[int](entuser.FieldID, crud.OrderAscending),
)
```

规则：

- 只有显式 `Bind(...).Filter()`/`.Order()` 的字段可以被客户端查询。
- `Columns(entuser.ValidColumn)` 拒绝拼错或跨表列名。
- `Nullable()` 固定采用升降序都 `NULLS LAST` 的比较合同。
- `DefaultOrder` 只在客户端未提供对应顺序时生效。
- `CursorKey` 是后端私有的非空唯一 tie-breaker；公共 PB 不必暴露数据库 ID。
- `WithComparisonProfile`、`WithQueryConverter`、`WithCursorConverter` 用于显式存储差异；profile 改变会使旧 token 失效。
- 常见 JSON 列路径使用 `JSONPath`；relation、computed、数据库函数或其它特化语义使用 `Custom`/`CustomOrder`。

repository 先把 biz scope 映射为 Ent predicate，再调用一次 adapter：

```go
result, err := entcrud.List(
    ctx,
    client.User.Query().Where(entuser.TenantIDEQ(scope.TenantID())),
    query,
    fields,
    scope.Fingerprint(),
)
```

adapter 绑定类型化 filter、补全稳定排序、校验 token、执行 keyset/skip/Count，并用隐藏 SELECT alias 从 Ent entity 原生提取 cursor。它不创建、提交或传播事务。

### Clear 到存储表示

mask 选中但 optional 值 absent 时，`ClearHelper` 默认对同名 nullable Ent 字段调用 `ClearField`。字段改名或非空存储表示必须显式覆盖：

```go
clear, err := entcrud.NewClearHelper[*ent.UserMutation](
    entcrud.ClearToValue(userv1.UserFields.DisplayName, func(m *ent.UserMutation) error {
        m.SetDisplayName("")
        return nil
    }),
    entcrud.RenameClear[*ent.UserMutation](
        userv1.UserFields.TemporaryPassword,
        entuser.FieldPasswordHash,
    ),
)

if err := clear.Apply(prepared.Resource(), prepared.WriteMask(), update.Mutation()); err != nil {
    return err
}
```

present 值和 repeated/map Replace 仍由 repository setter 处理。

## 浮点与输入验证

`float`/`double` filter、order 和 cursor 只支持有限值。NaN、`+Inf`、`-Inf` 不在跨方言合同内，adapter 会在进入 SQL 或编码 token 前拒绝。

业务字段值的精确校验应使用 Protovalidate 等字段级契约。CRUD 的 `REQUIRED` 校验主要补充 FieldMask-aware 的 present-and-non-empty 语义；它不是通用 validator。

## 框架错误与业务错误

`servora.crud.v1.CrudErrorReason` 只由框架自己的输入/合同路径产生：

- `INVALID_RESOURCE_NAME`；
- `INVALID_PAGE_TOKEN`；
- `INVALID_FILTER`；
- `INVALID_ORDER_BY`；
- `INVALID_FIELD_MASK`；
- `INVALID_FIELD_VALUE`；
- `INTERNAL`。

not-found、already-exists、唯一冲突、etag mismatch、allow-missing、业务前置条件和授权隐藏策略都不是框架错误。应用应在自己的业务 Proto 中定义 reason，由 data 使用 `ent.IsNotFound`、`ent.IsConstraintError` 或业务 sentinel 识别存储事实，由 biz 决定对外语义。

Ent adapter 对数据库执行错误不做归类，也不替换错误链。repository 增加上下文时应保留 `%w`，使 `errors.Is`/`errors.As` 仍能到达原始 Ent/driver 错误。

## 软删除：存储能力与公共 API 分开

### Ent 存储层能力

在需要软删除的 Ent schema 中显式启用：

```go
func (User) Mixin() []ent.Mixin {
    return []ent.Mixin{mixin.SoftDeleteMixin{}}
}
```

Mixin 提供：

- nullable `delete_time`；
- private `deleted_by`；
- nullable `purge_time`；
- 普通 Query 默认排除 tombstone；
- Delete/DeleteOne 改写为设置 `delete_time`；
- `mixin.SkipSoftDelete(ctx)` 显式绕过默认过滤和 Delete 改写；
- `mixin.WithDeletedBy(ctx, actor)` 为删除写入 canonical actor name。

它只标记当前模型记录，不遍历 edges，不修改外键关系，不自动生成回收站、Undelete、Expunge 或清理任务。

### AIP-164 公共合同

需要公开可恢复删除语义的资源，推荐显式采用 AIP-164：

- 资源暴露 OUTPUT_ONLY `delete_time`/`purge_time`；
- Delete 返回 tombstone 资源；
- 有权限的 Get 可以读取 tombstone；
- List 默认隐藏，`show_deleted=true` 时包含 tombstone；
- 提供 `Undelete`，需要时另行设计 `Expunge`。

启用 `SoftDeleteMixin` 不等于采用 AIP-164。普通 Delete 返回 `google.protobuf.Empty` 和返回目标资源都受生成器支持；框架只按响应形态清理输出，不从 Mixin、字段名或 RPC 集合推导公共删除策略。

`show_deleted` 等可见范围由 biz 决定，data 通过显式 bypass 执行。它改变结果集时必须进入业务 scope fingerprint。

### 唯一约束

Mixin 不创建“仅 active row 唯一”的索引。应用按方言和业务语义声明：

- PostgreSQL/SQLite：优先 partial unique index，例如 `WHERE delete_time IS NULL`；
- MySQL：使用适合业务语义的 tombstone/生成列复合唯一方案，并先验证重复 tombstone 行为。

唯一冲突仍按业务错误处理。

## Secret 与存储私有字段

明文 secret 只应存在于 Create/Rotate 请求和 biz 处理期间：

1. 公共资源把明文字段标为 `INPUT_ONLY`；
2. biz 执行 hash/verify；
3. repository 只接收并持久化不同名的私有 hash 字段；
4. hash 字段不进入公共 Proto、resource descriptor、前端元数据、ResourceMapper 映射或 ListFields。

Servora 不提供 `SecretHash` 框架类型，也不会替业务选择 hash 算法或轮换策略。

## 事务与复杂领域

CRUD 不提供 `WithTx`、`TxFromContext` 或事务 runner。事务边界、rollback/commit、outbox、跨表一致性与 Saga 都归 biz/data。adapter 在调用方传入的 Ent builder/client 上执行；若该 builder 已绑定业务事务，它自然参与，但 adapter 不感知也不管理事务。

推荐资源与存储模型保持浅层、显式。语义内聚的 value object 可以使用浅层 message，并由 repository 映射到平铺列。复杂状态机、聚合根、多表事务、relation、审计/outbox、领域事件、Bulk、长任务和专用搜索应使用手写 usecase/custom RPC，不应被强塞进标准 Update/List。

## TypeScript 客户端

`protoc-gen-servora-crud target=ts` 生成资源名与字段常量，不复制 Proto 类型或 HTTP transport：

```ts
import { UserName, UserFields, UserUpdateFields } from "./user.crud"
import {
  buildFilter,
  buildOrderBy,
  makeUpdateMask,
  firstPage,
  advancePager,
} from "@servora/proto-utils/crud"

const name = UserName.format({ tenant: "acme", user: "alice" })
const filter = buildFilter(UserFields, {
  field: UserFields.email,
  op: "=",
  value: "alice@example.com",
})
const orderBy = buildOrderBy(UserFields, [
  { field: UserFields.createTime, direction: "desc" },
])
const updateMask = makeUpdateMask(UserUpdateFields, {
  nickname: undefined, // own key present：显式 Clear
})
```

`UserName.parse` 抛出 `ResourceNameError`，`tryParse` 返回 `null`。`makeUpdateMask` 以 own-key presence 判断意图，因此显式 `undefined` 仍会选中字段。

生成的 HTTP client 通过 `createRequestHandler` 接入 transport。ProtoJSON response 可显式使用 `responseType: "json"`；Kratos 错误由 `ApiError` 保留响应体供业务解析。

## 后端支持与验证

| 层/方言 | 状态 | 验证方式 |
|---|---|---|
| `core/crud` | 支持，ORM 中立 | 无数据库单元测试 |
| `core/crud/mapper` | 支持，ORM 中立 | 无数据库单元测试 |
| Ent + SQLite | 支持 | 本地完整 live contract |
| Ent + PostgreSQL | 支持 | 本地方言敏感 live contract |
| Ent + MySQL | 实验性 | 无数据库 SQL-builder/dialect 测试；未完成 live contract verification |
| GORM CRUD adapter | 不提供 | 后续独立设计与验证 |

CI 不启动数据库、中间件、Docker service 或 Testcontainers。无外部服务路径：

```bash
make lint
make test                 # go test -short ./...
make web.typecheck
make web.build
```

发布前由开发者自行启动依赖，并显式运行：

```bash
SERVORA_ENT_SQLITE_DSN='file:servora_crud_live?mode=memory&cache=shared&_fk=1' \
  make test.ent.sqlite

SERVORA_ENT_POSTGRES_DSN='postgres://user:password@127.0.0.1:5432/db?sslmode=disable' \
  make test.ent.postgres
```

测试只消费 DSN，不创建或销毁容器。SQLite 与 PostgreSQL 两条结果都应记录为 release gate；MySQL live test 不是当前发布门槛。

## 参考应用

可运行黄金路径位于 [`servora-platform/app/example`](https://github.com/Servora-Kit/servora-platform/tree/main/app/example)：

- 单一 `example.servora.dev/User`，pattern `tenants/{tenant}/users/{user}`；
- service → biz → data → Ent 分层；
- 标准 Get/List/Create/Update/Delete + AIP-164 Undelete；
- filter/order/page token/skip/optional total；
- nullable `nickname` 的 Clear→NULL、非空 `display_name` 的 Clear→空字符串；
- INPUT_ONLY `temporary_password` → private `password_hash`；
- Vue 控制台只消费 generated TypeScript client 与 `@servora/proto-utils`。

本地运行：

```bash
cd servora-platform/app/example/service
make run

cd ../web
pnpm dev
```

服务默认监听 HTTP `127.0.0.1:28080` 与 gRPC `127.0.0.1:28081`，Vue Vite proxy 将 `/v1` 转发到 HTTP facade。参考应用默认使用进程内共享的命名 SQLite URI；进程退出后数据消失，且该配置不进入 CI 数据库生命周期。
