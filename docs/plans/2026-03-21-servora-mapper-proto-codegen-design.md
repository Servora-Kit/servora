# 设计文档：Servora proto-first mapper 与 protoc 代码生成体系

**日期：** 2026-03-21
**最后更新：** 2026-03-21
**状态：** Phase 0–5 全部完成 · 设计冻结（见 §16 各阶段注记）

---

## 1. 背景

Servora 当前已经明确走向：

- 以 **proto-first** 作为长期演进方向；
- `pkg/` 中沉淀框架能力，而不是把框架价值埋进某个业务服务；
- `authz` 与 `audit` 已经收敛到“**pkg 实现功能 + proto 注解声明规则 + codegen 生成薄胶水**”这一路线；
- 未来不仅要解决单个服务的 mapper 丑陋问题，还要解决整个脚手架层面的模型重复、映射重复、接线重复问题。

当前痛点主要有三类：

1. ~~**模型重复**：`ent/gorm`、biz model、proto message 常常三份高度相似；~~ **[已解决]** entity 层已消灭，proto message 直通 biz，仅保留 `resource proto <-> ent entity` 两层；
2. **映射重复**：data 层 repo 初始化时不断重复注册 converter、写字段 rename、补 enum/time/json 等规则；
3. ~~**透传边界不清**：到底哪些 proto 可以直接进入 biz，哪些不能，当前没有统一规范。~~ **[已解决]** 资源型 proto（`User`、`Application`）直通 biz，RPC wrapper 不透传；敏感字段（password、client_secret_hash）不在 proto 中定义，通过 repo 独立参数传递。

Servora 需要一套比 go-wind-admin 更完整、比 go-utils/mapper 更工程化的方案：

- 保留 `pkg` 作为功能实现中心；
- 通过 proto 注解与 protoc 插件把映射规则声明化；
- 生成代码保持轻量，不生成大量业务逻辑；
- 支持 repo 内部持有 mapper，并允许少量 custom hook 处理复杂字段。

---

## 2. 目标与非目标

### 2.1 目标

本设计目标是建立一套完整的 Servora mapper/codegen 体系，包括：

1. `pkg/mapper` 运行时能力重构与增强；
2. proto 注解体系；
3. `protoc-gen-servora-mapper` 插件；
4. generated wiring / validator / helper 代码布局；
5. data 层 repo 接入约定；
6. proto 透传边界规范；
7. 为后续 repo skeleton / patch helper / query helper 生成打基础。

### 2.2 非目标

本轮设计**不追求**：

1. 自动生成完整 service/biz/data 三层实现；
2. 自动生成复杂业务逻辑；
3. 在 proto 中表达复杂业务计算、跨字段业务规则；
4. 将 proto 与具体 Go/Ent 包路径深度耦合；
5. 通过 codegen 消灭所有手写代码。

---

## 3. 参考实现与参照路径

本设计明确参考以下已有实现，实施阶段可直接对照：

### 3.1 `go-utils` mapper 基线

- `/Users/horonlee/projects/go/tx7do/go-utils/mapper/mapper.go`

该文件提供了极简的 `CopierMapper[DTO, ENTITY]`：

- `NewCopierMapper`
- `AppendConverter`
- `AppendConverters`
- `ToEntity`
- `ToDTO`

它适合作为 Servora mapper 的**底层心智模型基线**：简单、泛型化、围绕 `copier.TypeConverter` 扩展。

### 3.2 `go-wind-admin` 的 repo 内 mapper 初始化模式

- `/Users/horonlee/projects/go/tx7do/go-wind-admin/backend/app/admin/service/internal/data/menu_repo.go`
- `/Users/horonlee/projects/go/tx7do/go-wind-admin/backend/app/admin/service/internal/data/operation_audit_log_repo.go`

这两个 repo 展示了当前最值得借鉴的模式：

- mapper 在 repo 内 `new`；
- enum converter 在 repo 初始化时注册；
- time/timestamppb converter 在 repo 初始化时注册；
- repo 是 mapper 的拥有者，service/biz 不感知 mapper。

### 3.3 Servora 当前 mapper 能力与问题来源

- `/Users/horonlee/projects/go/servora/pkg/mapper/converter.go`
- `/Users/horonlee/projects/go/servora/pkg/mapper/copier_proto.go`
- `/Users/horonlee/projects/go/servora/pkg/mapper/copier_db.go`
- `/Users/horonlee/projects/go/servora/app/iam/service/internal/data/mapper.go`

现状说明（Phase 0 实施后）：

- entity 层已消灭，`service/mapper.go` 已删除；
- `data/mapper.go` 已改为 `ent → proto` 直接映射（手写 `ForwardMapper` 函数）；
- `pkg/mapper` 已有 converter 基础，但还未形成完整的“runtime + codegen + registry + validator”体系；
- IAM 中 `Profile` 的 JSON 字段映射已通过 `profileFromJSON` 手写解决，但未走 custom hook 机制；
- mapper 只剩 data 层一处（`ent → proto`），是后续 codegen 的唯一替换目标。

---

## 4. 核心结论

### 4.1 总体路线

Servora 采用：

> **pkg 实现功能 + proto 注解声明规则 + protoc 插件生成薄胶水 + repo 持有 mapper**

而不是：

- 只学 go-wind-admin 手工写 repo mapper；
- 或者走大而全的三层代码生成器。

### 4.2 codegen 的职责边界

`protoc-gen-servora-mapper` 的职责不是“替开发者写业务代码”，而是：

1. 读取 proto 注解；
2. 生成 mapper wiring / registration / validation 代码；
3. 将内置 converter 选择、字段 rename、custom hook key 等规则前置化；
4. 在编译期或初始化阶段尽量发现配置不完整问题。

### 4.3 `pkg/mapper` 的职责边界

所有实际功能保留在 `pkg/mapper`：

- copier mapper；
- builtin converter；
- preset；
- custom registry；
- generated plan apply；
- hook 校验；
- future: patch/apply helper runtime、typed JSON converter、query/filter helper runtime（见 §15.1）。

### 4.4 data 层 repo 的职责边界

repo 继续作为 mapper 持有者：

- repo 内创建 mapper；
- repo 内应用 generated wiring；
- repo 内注册 custom hook；
- repo 内处理 ent/ORM 相关特殊逻辑。

service/biz 层不直接关心 mapper 的实现细节。

---

## 5. Proto 透传边界

这是实施前必须先定清楚的前置规则。

### 5.1 允许透传的对象

默认允许透传到 biz 的，是**资源型 proto / DTO 型 proto**，例如：

- `User`
- `Menu`
- `OperationAuditLog`
- `AuditEvent`

它们代表资源本身，而不是 RPC 包装壳。

### 5.2 不建议直接透传的对象

以下对象不应直接作为 biz 核心模型：

- `CreateXxxRequest`
- `UpdateXxxRequest`
- `DeleteXxxRequest`
- `ListXxxRequest`
- `GetXxxRequest`
- 带 paging / filtering / view mask 的 RPC wrapper

原因：

1. 它们携带明显的 transport 语义；
2. 它们包含 patch / update_mask / query_by 等 RPC 交互细节；
3. 若直接打穿到 biz，会把 transport 约束污染到内部处理逻辑。

### 5.3 默认映射关系

proto-first 默认映射关系为：

```text
resource proto <-> ent entity
```

而不是强制：

```text
resource proto <-> biz model <-> ent entity
```

如无必要，不再强制为所有资源建一层独立 biz model。

### 5.4 复杂字段的处理原则

复杂字段不依赖“透传”硬吃，而是统一走：

- builtin converter（能覆盖则优先）
- custom hook（覆盖不了则扩展）
- repo 内手写特殊逻辑（再复杂时最后兜底）

典型场景：

- typed JSON / profile
- 多字段拼装
- ORM 特定结构
- patch 特殊更新

---

## 6. 整体架构

### 6.1 设计总览

```text
proto message + servora.mapper annotations
        ↓
protoc-gen-servora-mapper
        ↓
generated mapper plan / wiring / validators / helper stubs
        ↓
pkg/mapper runtime（preset / builtin converters / custom registry / apply / validate）
        ↓
repo new mapper + apply generated wiring + register custom hooks
        ↓
repo 使用 mapper 进行 proto resource <-> ent entity 转换
```

### 6.2 分层职责

| 层 | 职责 |
|---|---|
| proto | 声明资源模型与 mapper 规则 |
| protoc 插件 | 生成 wiring、validator、helper glue |
| `pkg/mapper` | 提供运行时功能与扩展机制 |
| data repo | 持有 mapper、注册 custom hook、处理 ent 特殊逻辑 |
| biz/service | 使用资源型 proto，不感知 mapper 细节 |

---

## 7. `pkg/mapper` 运行时设计

Servora 的运行时设计应直接参考 `go-utils/mapper` 的简洁风格，但需要补齐工程化能力。

### 7.1 核心类型：`CopierMapper[P, E]`

保留与 `go-utils/mapper` 接近的核心对象：

- `NewCopierMapper[P, E]()`
- `AppendConverter(converter copier.TypeConverter)`
- `AppendConverters(converters []copier.TypeConverter)`
- `ToEntity(proto *P) (*E, error)`
- `ToProto(entity *E) (*P, error)`
- `MustToEntity(proto *P) *E`
- `MustToProto(entity *E) *P`

与 `go-utils/mapper` 不同之处：

1. 默认提供 error-return 版本；
2. `Must*` 只作为便捷层；
3. 为 generated wiring 留出 plan/apply 能力。

### 7.2 Builtin converter 体系

在现有 `pkg/mapper/converter.go` 基础上，统一收敛成可被 codegen 引用的 builtin 集合。

首批 builtin 应覆盖：

1. `time.Time <-> *time.Time`
2. `time.Time <-> *timestamppb.Timestamp`
3. `string <-> *string`
4. `int64 <-> *int64`
5. `pb enum <-> entity string enum`
6. future-ready: `duration`, `wrapperspb`, typed JSON

### 7.3 Preset 体系

Preset 是“能力组”，不是具体字段实现。

已实现预置：

- `proto_time` — `time.Time <-> *timestamppb.Timestamp`
- `time_ptr` — `time.Time <-> *time.Time`
- `pointer` — `string <-> *string`, `int64 <-> *int64`
- `common_proto_entity` — 组合以上三者

不作为静态 preset 的：

- ~~`proto_enum`~~ — enum converter 需要 per-enum 的 name/value map，无法作为通用 preset 工厂。`EnumConverter[DTO, Entity]` 已在 `converter.go` 中提供，由 repo 按需实例化并通过 custom hook 或 `AppendConverters` 注册
- `well_known_types`（后续可扩）

Preset 的作用：

- 让 proto 注解不必为每个普通字段重复声明；
- 让 generator 只负责选 preset；
- 让 runtime 控制真实 converter 组合。

### 7.4 Custom hook registry

这是比 go-wind-admin 更强的关键能力。

需要提供统一的 hook registry，例如：

- `Register(name string, converters ...copier.TypeConverter)`
- `Get(name string) ([]copier.TypeConverter, bool)`
- `MustGet(name string) []copier.TypeConverter`

作用：

1. proto 注解可以声明 `custom = "user_profile"`；
2. generator 只生成对 `user_profile` 的引用；
3. repo 初始化时再注册真实 custom converter；
4. runtime/validator 可检查 custom hook 是否缺失。

### 7.5 Mapper plan

为了让 generator 与 runtime 解耦，定义 `MapperPlan` 中间结构，包含：

- `Presets []string` — 启用的 preset 名称
- `FieldMapping map[string]string` — 字段重命名（entity field → proto field）
- `FieldConverters map[string]ConverterKind` — 字段级 builtin converter 声明（proto field → converter kind）
- `IgnoredFields []string` — 排除的字段名列表
- `CustomHooks []string` — 自定义 hook 名称
- optional future sections（patch/query）

`ConverterKind` 是 `string` 类型常量，对应 proto `ConverterKind` 枚举。`ApplyPlan` 通过 `builtinConverterFactories` map 解析 kind 到实际 converter。`ENUM_STRING` 不在 factory map 中——enum converter 需 per-enum 数据，必须通过 custom hook 或 `AppendConverters` 提供。

generated code 输出的是“plan 数据”，不是把实现细节硬编码在生成文件里。

### 7.6 Validation

runtime 需要统一提供：

- 校验 plan 中声明的 preset 是否存在；
- 校验 field converter 配置是否合法；
- 校验 custom hook 是否已注册；
- 启动期给出清晰错误，而不是 silent fallback。

---

## 8. Proto annotation 设计

### 8.1 设计原则

1. proto 负责声明**稳定、通用、框架级规则**；
2. 不把 proto 做成复杂 DSL；
3. 复杂映射问题统一降级到 custom hook；
4. 尽量避免 proto 直接写 Go 包路径或 ent 内部实现细节。

### 8.2 Message 级注解

Message 级至少需要：

1. `enabled`
   - 是否参与 mapper codegen
2. `presets`
   - 启用哪些 builtin 能力组
3. future-ready:
   - `patch_helper`
   - `query_helper`
   - `repo_binding_key`（若后续需要）

### 8.3 Field 级注解

Field 级至少需要：

1. `rename`
   - 目标字段名
2. `converter`
   - 选择 builtin converter kind
3. `custom`
   - 声明 custom hook key
4. future-ready:
   - `ignore`
   - `readonly`
   - `writeonly`
   - `patch_strategy`

### 8.4 Builtin converter kind 首批建议

首批枚举建议覆盖：

- `CONVERTER_UNSPECIFIED`
- `TIMESTAMP_TIME`
- `TIME_PTR`
- `STRING_PTR`
- `INT64_PTR`
- `ENUM_STRING`
- future-ready:
  - `DURATION_DURATIONPB`
  - `JSON_TYPED`
  - `WRAPPER_VALUE`

### 8.5 明确不放进 proto 的内容

首版不建议放入：

1. 多字段表达式拼装；
2. 复杂 JSON path 操作；
3. 具体 ent 包路径；
4. 复杂 ORM 更新逻辑；
5. 业务领域规则。

这些都不应由 mapper annotation 承担。

---

## 9. `protoc-gen-servora-mapper` 插件设计

### 9.1 插件目标

插件负责把 proto annotation 转换为：

1. mapper plan 常量/构造代码；
2. generated wiring helper；
3. generated validator；
4. future helper scaffold（patch/query）。

### 9.2 插件不负责

插件不负责：

1. 生成完整 repo 实现；
2. 生成完整 biz/service 逻辑；
3. 生成复杂 ent query；
4. 实现 runtime converter 本体；
5. 接管 repo 的最终组织方式。

### 9.3 插件输入

插件输入包括：

- proto message 本身；
- `servora.mapper` message/field 注解；
- buf / protoc 上下文；
- optional 外部配置（用于补充复杂例外）。

### 9.4 插件输出

> **实施决策（Plan-only）：** 插件只生成 `XxxMapperPlan()` 函数，返回 `*mapper.MapperPlan`。不生成工厂函数、不引入任何 ORM 依赖。
>
> 原因：插件是 `pkg` 级框架工具，不应绑定具体 ORM（ent/gorm）。生成代码只依赖 `pkg/mapper`，repo 层手写一行 `mapper.ApplyPlan(xxxpb.XxxMapperPlan(), m, presets, hooks)` 完成装配。

插件输出到 generated Go 文件，包含：

1. `XxxMapperPlan()` —— 返回 `*mapper.MapperPlan`（presets / field mapping / custom hooks）
2. future-ready: helper stubs（patch/query）

~~原始设计中的 `RegisterGeneratedXxxMapper` 和 `ValidateXxxMapperHooks` 已由 `mapper.ApplyPlan` 和 `plan.Validate` 统一替代。~~

### 9.5 插件与外部配置的关系

完整方案采用：

> **proto 为主，外部配置补充复杂例外**

原因：

1. 维持 proto-first；
2. 避免 proto annotation 过重；
3. 复杂 JSON / 多字段映射 / repo 定制覆盖仍有容身之处；
4. 便于后续逐步增强，而不是一次性把 DSL 做爆炸。

外部配置只用于补充，不应成为主要 source of truth。

---

## 10. 生成代码布局

### 10.1 建议原则

生成代码应保持：

- 薄；
- 稳定；
- 只做装配；
- 可预测；
- 不与手写 repo 实现混杂。

### 10.2 建议内容

每个带 `enabled: true` 的资源 message 生成：

1. `XxxMapperPlan()` 函数

生成文件位置：

```text
api/gen/go/<package>/xxx_mapper.gen.go
```

实际示例（`User` 资源）：

```text
api/gen/go/user/service/v1/user_mapper.gen.go    → UserMapperPlan()
api/gen/go/application/service/v1/application_mapper.gen.go → ApplicationMapperPlan()
```

生成代码只依赖 `pkg/mapper.MapperPlan`，不引入任何 ORM 类型。

### 10.3 与手写代码的边界

generated code 不拥有 mapper 实例；
repo 才拥有 mapper 实例。

repo 的典型使用方式：

```go
m := mapper.NewCopierMapper[userpb.User, ent.User]()
m.AppendConverters(uuidStringConverter)  // ORM 特有 converter
hooks := mapper.NewHookRegistry()
hooks.Register("user_profile", ...)       // custom hook 实现
mapper.ApplyPlan(userpb.UserMapperPlan(), m, mapper.DefaultPresets(), hooks)
```

ORM 绑定在 repo 层，不在生成代码中。

---

## 11. Data 层接入设计

### 11.1 统一模式

延续 go-wind-admin 的经验，Servora 明确采用：

> **repo 内部 new mapper，并在 repo 初始化时完成 generated wiring + custom hook 注册**

### 11.2 典型初始化流程

```text
NewXxxRepo()
  -> new CopierMapper[P, E]
  -> apply generated plan / presets / builtin converters
  -> register service-specific custom hooks
  -> validate hook completeness
  -> repo ready
```

### 11.3 为什么不让 service/biz 持有 mapper

原因：

1. mapper 的主要复杂度来自存储模型；
2. custom hook 往往与 data/ORM 强相关；
3. 让 service/biz 感知 mapper 只会扩大泄漏面；
4. 与 go-wind-admin 的好经验一致。

### 11.4 与当前 Servora 的关系

> **实施状态（Phase 4 完成后）：** IAM 服务已完全对齐本节设计。
>
> - `userRepo`、`authnRepo`、`applicationRepo`、`oidcStorage` 均在 struct 中持有 mapper 字段
> - mapper 在 `NewXxxRepo()` / `NewOIDCStorage()` 中创建，通过 `newUserMapper()` / `newApplicationMapper()` 工厂函数
> - 工厂函数内部调用 `mapper.ApplyPlan(xxxpb.XxxMapperPlan(), m, presets, hooks)` 完成 generated plan 装配
> - `profileFromJSON` 通过 post-processing hook 模式处理（见 §12.3.1）
> - 不再存在包级 `var` 全局 mapper 单例

---

## 12. Custom hook 设计

### 12.1 适用场景

以下场景统一走 custom hook：

1. typed JSON / profile
2. 多字段组合或拆分
3. entity edge 特殊处理
4. repo 特定 patch 行为
5. 存储实现细节很强的转换

### 12.2 Hook 设计原则

1. hook 必须命名；
2. hook 应尽量绑定“语义键”，而不是具体 repo 名字；
3. hook 是例外机制，不是默认机制；
4. 只在 builtin converter/preset 无法覆盖时使用。

### 12.3 例子

对于 `Profile` 这类复杂 JSON 字段，可声明：

- `custom = "user_profile"`

然后在 repo 初始化时注册 `user_profile` 对应 converter。

### 12.3.1 Post-processing hook 变体

当 copier 的 `TypeConverter` 无法处理某些转换（如 `map[string]any → *UserProfile` 结构化映射），允许 hook 注册为空：

```go
hooks.Register("user_profile") // 声明依赖，满足 plan validation
```

实际转换在 repo 的 post-processing helper 中完成：

```go
func mapUser(m *mapper.CopierMapper[userpb.User, ent.User], u *ent.User) *userpb.User {
    pb := m.MustToProto(u)
    if pb != nil && u.Profile != nil {
        pb.Profile = profileFromJSON(u.Profile)
    }
    return pb
}
```

这是合法的 hook 形态：proto 注解声明了依赖关系，plan validation 确保 hook key 被注册，但具体实现方式由 repo 决定（copier converter 或 post-processing）。

### 12.4 比 go-wind-admin 更好的地方

go-wind-admin 中这类逻辑更多是“repo 作者自己知道怎么 append”。

Servora 要求：

- proto 明确声明 custom key；
- generator 显式生成依赖；
- runtime 校验未注册错误；
- repo 只负责补实现，不负责记忆隐式约定。

---

## 13. 错误处理与校验

### 13.1 设计目标

尽量把错误前移到：

- 编译期（annotation 非法）
- 初始化期（hook 未注册、preset 不存在）

而不是运行时 silent failure。

### 13.2 需要覆盖的错误

1. message 启用了 mapper，但字段注解非法；
2. 使用了不存在的 preset；
3. 使用了不存在的 builtin converter；
4. 声明了 custom hook 但 repo 未注册；
5. field rename 冲突；
6. 外部配置与 proto 注解冲突；
7. generated plan 与 runtime 版本不兼容。

### 13.3 报错原则

错误必须：

- 带 message/field 名称；
- 指出哪个注解或 hook key 出问题；
- 能指导实施者快速修复。

---

## 14. 为什么这套方案比 go-wind-admin 更好

### 14.1 不只是“repo 写法更优雅”

go-wind-admin 的优势主要在于项目级实践：

- repo 持有 mapper
- enum/time converter 注册清晰
- pb 更靠近 data 层

但它仍主要依赖手工接线。

Servora 的提升点是：

- 将规则声明化；
- 将 wiring 生成化；
- 将 custom 依赖显式化；
- 将遗漏校验系统化。

### 14.2 不只是复制 `go-utils/mapper`

`go-utils/mapper` 是很好的简洁内核，但太薄：

- 无 registry
- 无 preset
- 无 generated plan
- 无 validator
- 默认 panic

Servora 保留它的核心风格，但补上框架化必须的部分。

### 14.3 更贴合 Servora 长期路线

Servora 已经在 authz/audit 上收敛为：

```text
pkg runtime + proto annotation + codegen glue
```

mapper 如果也进入这条路线，整个框架方法论会更一致。

---

## 15. 与未来能力的衔接

本设计不是只为 mapper 服务，而是为更广义的 resource modeling / codegen 体系铺路。

### 15.1 可自然演进的能力

后续可在本设计上继续扩展（按推荐优先级排列）：

#### 15.1.1 patch/apply helper codegen

**问题：** 当前 `UpdateUser` 等方法需要手写大量 `if u.Xxx != "" { update.SetXxx(u.Xxx) }` 逻辑，每个可选字段一行，容易遗漏且与 proto 定义重复。

**方案：** 在 proto field 级注解中增加 `patchable: true` 标记，由 `protoc-gen-servora-mapper` 生成 `ApplyPatch(update *ent.XxxUpdateOne, pb *xxxpb.Xxx)` 函数。生成代码读取 proto 字段是否被设置（presence / zero-value 判断），自动调用对应 ent setter。

**衔接点：**
- 复用已有的 `mapper.v1.mapper_field` 注解体系，新增 `patchable` bool 字段
- 复用已有的 `FieldMapping`（rename）和 `FieldConverters`（类型转换）
- 生成代码位于同一 `xxx_mapper.gen.go` 文件

#### 15.1.2 create/update field set 规则

**问题：** 同一个 proto message 同时用于 Create 和 Update，但两种操作的"哪些字段必填 / 可选 / 只读"规则不同。当前只能在 biz 层手写校验。

**方案：** 在 proto field 注解中增加 `writable` 枚举（`CREATE_ONLY` / `UPDATE_ONLY` / `ALWAYS` / `READ_ONLY`），由插件生成 `ValidateForCreate(pb *xxxpb.Xxx) error` 和 `ValidateForUpdate(pb *xxxpb.Xxx) error` 校验函数。

**衔接点：**
- 扩展 `mapper.v1.mapper_field` 注解
- 校验函数可在 biz 层或 service 层调用
- 与 `patchable`（15.1.1）配合：`READ_ONLY` 字段自动排除在 patch 之外

#### 15.1.3 query/filter helper codegen

**问题：** `ListUsers` 等查询接口若要支持字段过滤（如 `?status=active&role=admin`），需要在 data 层手写 `if filter.Status != "" { query = query.Where(user.StatusEQ(filter.Status)) }` 等重复代码。

**方案：** 在 proto field 注解中增加 `filterable: true` 标记，由插件生成 `ApplyFilters(query *ent.XxxQuery, filter *xxxpb.XxxFilter) *ent.XxxQuery` 函数。支持基本比较操作（EQ / IN / LIKE / GT / LT）。

**衔接点：**
- 需要新增 `XxxFilter` proto message 或在 `ListXxxRequest` 中内嵌 filter 字段
- 与已有 `pagination.v1` 注解体系配合
- 生成代码依赖 ent 的 predicate 包，因此属于 ORM 相关生成（与当前 ORM-agnostic 的 mapper plan 不同，需要独立的生成模板或配置项）

#### 15.1.4 typed JSON 标准能力

**问题：** `User.Profile` 等字段在 ent 中是 `map[string]any`，在 proto 中是结构化 message（`UserProfile`）。当前通过 post-processing hook 手写转换（见 §12.3.1），每个 JSON 字段都需要一套 `xxxFromJSON` / `xxxToJSON`。

**方案：** 定义标准化的 typed JSON 转换协议：
- proto 注解标记 `json_type: "UserProfile"` 关联 JSON 字段与其 typed message
- `pkg/mapper` 提供 `TypedJSONConverter[T proto.Message]`，基于 `protojson` 自动完成 `map[string]any ↔ T` 转换
- 消灭 `profileFromJSON` 等手写 helper

**衔接点：**
- 替代当前的 post-processing hook 模式
- 可作为新的 `ConverterKind`（如 `CONVERTER_KIND_TYPED_JSON`）纳入 `FieldConverters` 体系
- 运行时在 `pkg/mapper`，生成时在 `protoc-gen-servora-mapper`

#### 15.1.5 repo skeleton 辅助生成

**问题：** 新增一个资源时，需要手写 `NewXxxRepo()` + mapper 初始化 + CRUD 方法骨架，模式高度重复。

**方案：** 提供 `svr gen repo <service> <resource>` CLI 命令（扩展现有 `cmd/svr`），读取 proto message 的 mapper 注解，生成：
- `data/xxx.go` repo 骨架（struct 定义 + mapper 字段 + `NewXxxRepo` 构造 + CRUD 接口骨架）
- `biz/xxx.go` usecase 骨架（接口定义 + 基础实现）
- Wire provider 注册

**衔接点：**
- 不是 protoc 插件，而是 CLI 工具（一次性脚手架，生成后可自由修改）
- 读取已有 `XxxMapperPlan()` 函数确定 mapper 配置
- 与 `patch`（15.1.1）、`filter`（15.1.3）生成的 helper 自然组合

#### 15.1.6 资源治理级 annotation 体系

**问题：** 随着 mapper / authz / audit 三套注解体系成熟，proto message 上承载的元信息越来越多，但缺少统一的"资源级"元数据（如资源名称、所属域、版本、生命周期策略）。

**方案：** 定义 `servora.resource.v1` 注解，在 message 级声明：
- `resource_name`：规范化资源名（如 `iam.user`）
- `domain`：所属业务域
- `lifecycle`：软删除 / 硬删除 / 归档策略
- `audit_level`：审计级别（与 audit 注解联动）
- `authz_object_type`：授权对象类型（与 authz 注解联动）

**衔接点：**
- 统一已有 `servora.mapper`、`servora.authz`、`servora.audit` 三套注解的 message 级元信息
- 为未来的资源发现、API 文档生成、治理面板提供结构化数据源
- 渐进式引入：先声明注解，各子系统逐步读取

### 15.2 不建议立即扩展的能力

当前不建议同时做：

1. **自动生成完整 repo 实现** — 与 15.1.5 的 skeleton 不同，全自动 repo 会锁死 ORM 细节，丧失灵活性
2. **自动生成 biz/service** — 业务逻辑不应由模板驱动
3. **自动生成复杂 query DSL** — 15.1.3 的 filter helper 足够覆盖 80% 场景，复杂查询应手写
4. **大而全的 ORM 适配器矩阵** — 当前只支持 ent，不需要提前抽象多 ORM 适配层

先把 mapper/runtime/plugin 跑顺，再按 15.1 的优先级逐项推进。

---

## 16. 实施建议（简版）

> 本节只给实施骨架，不展开为详细任务拆解；后续可交由其他 LLM 或实施计划文档继续细化。

### 16.1 Phase 0：透传边界与设计冻结 ✅ 已完成

> **实施记录：** 2026-03-21 完成。详见 `docs/plans/archive/2026-03-21-eliminate-entity-proto-passthrough-design.md` 和 `docs/plans/archive/2026-03-21-eliminate-entity-impl-plan.md`。
>
> 主要变更：
> - 消灭 `biz/entity` 包，proto message 直通 biz 层
> - `User`、`Application` 资源型 proto 透传到 biz；RPC wrapper 不透传
> - 敏感字段（password、client_secret_hash）通过 repo 独立参数传递，不在 proto 中定义
> - data/mapper.go 重写为 `ent → proto` 手写映射
> - service/mapper.go 删除，service 层直接透传 proto
> - 编译、lint、12 个测试全通过

~~1. 冻结“资源型 proto 可透传，RPC wrapper 不可透传”的规范；~~
~~2. 冻结 `resource proto <-> ent entity` 为默认映射关系；~~
~~3. 明确 custom hook 的责任边界。~~

### 16.2 Phase 1：重构 `pkg/mapper` ✅ 已完成

> **实施记录：** 2026-03-21 完成。详见 `docs/plans/archive/2026-03-21-mapper-runtime-annotation-impl-plan.md` Task 1–4。
>
> 主要变更：
> - `CopierProtoMapper` + `CopierDBMapper` 合并为统一 `CopierMapper[P, E]`，API 返回 `(result, error)` 而非吞掉 error
> - 新增 `PresetRegistry`：命名 converter 组（`proto_time`、`time_ptr`、`pointer`、`common_proto_entity`），支持 `Collect` 批量获取
> - 新增 `HookRegistry`：命名 custom converter hooks，`Register`/`Get`/`MustGet`/`CheckRequired`
> - 新增 `MapperPlan` + `ApplyPlan[P,E]`：声明式配置 mapper（presets + field mapping + custom hooks），带 `Validate` 前置校验
> - IAM `applicationMapper` 已迁移至 `CopierMapper` 运行时，30 个 pkg/mapper 测试 + 12 个 IAM 测试全通过

~~1. 引入增强版 `CopierMapper`；~~
~~2. 整理 builtin converter；~~
~~3. 建立 preset；~~
~~4. 建立 custom hook registry；~~
~~5. 建立 plan/apply/validate 机制。~~

### 16.3 Phase 2：定义 proto annotation ✅ 已完成

> **实施记录：** 2026-03-21 完成。详见 `docs/plans/archive/2026-03-21-mapper-runtime-annotation-impl-plan.md` Task 5–6。
>
> 主要变更：
> - 新增 `api/protos/mapper/v1/mapper.proto`：定义 `ConverterKind` 枚举、`MapperMessageRule`（message 级）、`MapperFieldRule`（field 级）
> - Extension numbers：message options `50200`、field options `50201`（与 authz `50100` 号段分离）
> - `User` proto message 已添加 mapper 注解示例：presets、rename、converter、custom hook
> - Go 生成代码 `api/gen/go/mapper/v1/mapper.pb.go` 已输出，编译通过

~~1. 新增 `servora.mapper` annotation proto；~~
~~2. 支持 message 级与 field 级最小集合；~~
~~3. 预留 future-ready 字段但不一次性实现全部行为。~~

### 16.4 Phase 3：实现 `protoc-gen-servora-mapper` ✅ 已完成

> **实施记录：** 2026-03-21 完成。详见 `docs/plans/archive/2026-03-21-protoc-gen-servora-mapper-impl-plan.md` Task 1–3。
>
> 主要变更：
> - 实现 `cmd/protoc-gen-servora-mapper/main.go`：读取 `mapper.v1.mapper` / `mapper.v1.mapper_field` annotation，为每个 `enabled: true` 的 message 输出 `XxxMapperPlan()` 函数
> - **Plan-only 设计：** 生成代码只返回 `*mapper.MapperPlan`，不依赖任何 ORM（ent/gorm）。ORM 绑定在 repo 层手写
> - Buf 集成：`buf.authz.gen.yaml` 和 `buf.mapper.gen.yaml` 已合并到 `buf.go.gen.yaml`，`make api` 一次生成全部（Go struct + authz rules + mapper plans）
> - `Application` proto 已添加 mapper annotation，生成 `ApplicationMapperPlan()`

~~1. 读取 annotation；~~
~~2. 生成 mapper plan；~~
~~3. 生成 apply/register helper；~~
~~4. 生成 validator；~~
~~5. 打通 `make api`/Buf 生成链路。~~

### 16.5 Phase 4：在代表性资源上落地 ✅ 已完成

> **实施记录：** 2026-03-21 完成。详见 `docs/plans/archive/2026-03-21-protoc-gen-servora-mapper-impl-plan.md` Task 4–7。
>
> 主要变更：
> - mapper 从包级 `var` 全局单例迁移到 repo struct 字段，对齐设计文档 §11 和 go-wind-admin 模式
> - `userRepo`、`authnRepo`、`applicationRepo`、`oidcStorage` 均在构造函数中创建并持有 mapper 实例
> - `uuid.UUID <-> string` 和 `int <-> int32` 提升为 `pkg/mapper` 内置 converter（`NewUUIDStringConverterPair`、`NewIntInt32ConverterPair`），纳入 `common_proto_entity` preset
> - proto 新增 `CONVERTER_KIND_UUID_STRING = 6`、`CONVERTER_KIND_INT_INT32 = 7`
> - 插件补全对 `field.converter` 和 `field.ignore` 的处理，生成 `FieldConverters` 和 `IgnoredFields`
> - `MapperPlan` 新增 `FieldConverters map[string]ConverterKind` 和 `IgnoredFields []string`，`ApplyPlan` 通过 `builtinConverterFactories` 解析
> - `data/mapper.go` 不再需要任何手动 converter 变量，`newApplicationMapper()` 只需 4 行（new + ApplyPlan）
> - `user_profile` hook 采用 post-processing 模式（见 §12.3.1）
> - 全量验证通过：build / test / lint (0 issues) / regen 幂等 / pkg/mapper 33 tests / 服务启动无 panic

~~1. repo 内 new mapper + generated wiring + custom hook 的完整闭环。~~

### 16.6 Phase 5：迁移现有手写 mapper ✅ 已完成

> **实施记录：** 2026-03-21 完成。IAM 作为框架唯一参考实现案例，所有 mapper 已全部迁移到 generated plan 模式。
>
> - `userMapper`、`applicationMapper` 均通过 `ApplyPlan(xxxpb.XxxMapperPlan(), ...)` 装配
> - 不再存在任何 `NewForwardMapper`、`AllBuiltinConverters()`、手动 `WithFieldMapping()` 调用
> - 新服务只需：① proto 加 annotation ② `make api` ③ repo 中 `newXxxMapper()` + `ApplyPlan` 即可

~~1. 优先迁移框架级共享模式；~~
~~2. 再迁移服务内重复 repo；~~
~~3. 对复杂 JSON 字段保留 custom hook；~~
~~4. 避免一次性“全仓重写”。~~

---

## 17. 最终结论

Servora 的 mapper/codegen 路线不应停留在 go-wind-admin 式的“项目级 repo mapper 组织”，也不应走向大而全的自动生成器。

正确路线是：

- 以 `go-utils/mapper` 为极简内核参照；
- 以 go-wind-admin 的 repo 内初始化模式为组织参照；
- 在 Servora 中补齐 `runtime + annotation + protoc plugin + generated wiring + validator + custom hook registry`；
- 保持 codegen 薄、功能在 `pkg`、repo 持有 mapper、复杂逻辑走 custom hook；
- 明确资源型 proto 的透传边界，默认采用 `resource proto <-> ent entity`。

这样才能让 Servora 在 mapper 这一层真正形成与 authz/audit 一致的框架方法论，并为后续更广义的资源建模与代码生成体系打下稳定基础。
