# AGENTS.md - app/iam/service/internal/data/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 概览
数据访问层，包含 Ent ORM 实现和数据仓库。

## 结构
```
data/
├── ent/              # Ent ORM生成代码（10文件）
│   ├── user.go       # User实体（608行）
│   ├── user_query.go # 查询构建器（527行）
│   ├── mutation.go   # Mutation API（630行）
│   ├── enttest/      # 测试辅助
│   ├── hook/         # 钩子
│   ├── migrate/      # 迁移
│   ├── predicate/    # 谓词
│   └── runtime/      # 运行时
├── gorm/             # GORM实现（并行ORM）
│   ├── dao/          # 数据访问对象
│   └── po/           # 持久化对象
├── schema/           # Ent schema定义
│   └── user.go       # User schema（36行 → 生成1765行）
├── data.go           # 数据层Wire提供者
└── *_test.go         # 单元测试
```

## WHERE TO LOOK

| 任务 | 位置 | 说明 |
|------|------|------|
| 定义实体 | schema/user.go | Ent schema定义（小文件） |
| 查询数据 | ent/user_query.go | 类型安全查询构建器 |
| 修改数据 | ent/mutation.go | CRUD + 字段setter/getter |
| 测试ORM | enttest/ | 测试辅助+内存数据库 |
| 仓库实现 | data.go | 实现biz/层仓库接口 |

## Ent使用模式

### Schema → 生成代码
```go
// schema/user.go (36行)
func (User) Fields() []ent.Field {
    return []ent.Field{
        field.String("name"),
        field.String("email").Unique(),
        // ...
    }
}

// 生成 → ent/user.go (608行) + user_query.go (527行) + mutation.go (630行)
```

### 查询模式
```go
// 类型安全查询
users, err := client.User.
    Query().
    Where(user.EmailContains("@example.com")).
    Order(ent.Asc(user.FieldCreatedAt)).
    Limit(10).
    All(ctx)
```

### 与biz/层集成
data.go 实现 biz/ 定义的仓库接口，使用 Ent 客户端操作数据库。

## 生成命令
```bash
make gen.ent  # 从 schema/ 生成 ent/ 代码
```

## 注意事项
- ent/ 下全是生成代码，不要手改
- 大文件（>500行）都是Ent生成的，复杂度源于全功能ORM API
- schema/ 定义很小（36行），生成代码很大（1765行）
- 使用 enttest/ 进行单元测试，支持内存数据库
- 与 gorm/ 并行存在，项目同时使用两种ORM
