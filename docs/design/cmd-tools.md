# Servora 命令工具设计分析（更新版：`svr gen dao` Ent-only）

## 背景与本次决策

基于当前仓库实现与团队维护成本考量，本次将命令策略收敛为：

1. 服务级 GORM 生成命令已统一为 `make gen.gorm`（替代旧命名）。
2. `svr gen dao` 只支持 **Ent** 生成能力。
3. 中心化 CLI **不再支持 gorm-gen**；需要 gorm-gen 的服务继续在服务内自行维护（例如 `cmd/genDao`）。

---

## 结论

这个方向是正确的，原因很直接：

- Ent 生成是代码驱动、仓库内可静态判定，适合中心化 CLI。
- gorm-gen 依赖数据库连接与服务本地配置，中心化后会放大环境差异与失败面。
- 统一入口应优先处理“可确定、可重复、低外部依赖”的生成流程。

因此：

- `svr gen dao` = Ent 入口（统一规范）
- `make gen.gorm` / `cmd/genDao` = gorm-gen 本地入口（服务自治）

---

## 仓库证据（当前状态）

### 1) Make 目标关系

- `app.mk:129`：`gen: wire api openapi gen.ent`
- `app.mk:132`：`gen.gorm:`
- `app.mk:138`：`gen.ent:`

说明：当前 `make gen` 默认只串 Ent，不串 gorm-gen。

### 2) Ent 可检测标记（本仓）

- `app/servora/service/internal/data/schema/user.go`
- `app/servora/service/internal/data/generate.go:3`（包含 `entgo.io/ent/cmd/ent` 的 `go:generate`）

### 3) gorm-gen 本地入口（本仓）

- `app/servora/service/cmd/genDao/genDao.go:78`（`gen.NewGenerator(...)`）
- `app/servora/service/internal/data/gorm/dao/gen.go`

---

## `svr gen dao` 新设计（Ent-only）

## 命令契约

```bash
svr gen dao [--service <name>|--all] [--dry-run] [--fail-fast]
```

> 不再提供 `--orm` 选项。

## 识别逻辑（推荐）

采用单一入口条件即可：

1. 存在 `internal/data/generate.go` 且包含 `entgo.io/ent/cmd/ent`

满足该条件即判定该服务支持 `svr gen dao`。

## 执行映射

- Ent 服务：执行等价于服务目录下 `make gen.ent`
- 非 Ent 服务：输出明确提示并标记 `unsupported`

提示文案应包含：

- 检测失败原因（缺 generate.go 或 generate.go 不含 ent 生成入口）
- 如果该服务使用 gorm-gen，应该执行 `make gen.gorm` 或服务本地 `cmd/genDao`

---

## 与 gorm-gen 的职责边界

## 中心化（`svr`）

- 只负责 Ent 生成与批量调度。
- **新增**：`svr gen gorm` 提供中心化 gorm-gen 生成能力（通过读取服务配置）

## 服务本地（`make gen.gorm` / `cmd/genDao`）

- 负责数据库连接相关的 gorm-gen 生成。
- 由服务自己维护连接配置、模型生成策略与失败处理。
- **更新**：`make gen.gorm` 可以调用 `svr gen gorm` 作为底层实现

这样做的好处：

- 降低 CLI 复杂度与跨环境故障。
- 避免把数据库可用性耦合到中心化命令。
- 允许不同服务按自身数据库场景演进 gorm-gen 逻辑。
- **新增**：消除服务内 `cmd/gen/gorm-gen.go` 重复代码

---

## 迁移策略

## Phase 1（已完成）

- [x] `gen.gorm` 目标命名统一
- [x] 文档命令引用同步至 `gen.gorm`

## Phase 2（进行中）

- [ ] `svr gen dao` 明确为 Ent-only
- [ ] 识别逻辑采用 `generate.go(ent)` 单条件
- [ ] 输出针对 gorm-gen 的“本地执行指引”

## Phase 3（稳定后）

- [ ] 评估是否将 `make gen.ent` 的中心化入口逐步切到 `svr gen dao`
- [ ] 保持 `make gen.gorm` 作为服务自治能力长期存在

---

## 风险与防护

1. **误把非 Ent 目录结构当成 Ent 服务**
   - 防护：仅以 `generate.go` 中的 ent 生成入口作为判定依据。

2. **开发者误以为 `svr gen dao` 还能处理 gorm-gen**
   - 防护：帮助文档和错误提示明确“Ent-only”。

3. **混合 ORM 服务认知混乱**
   - 防护：统一原则：Ent 走 `svr`，gorm-gen 走服务本地命令。

---

## 外部参考（用于支撑本决策）

- Ent 官方实践强调 `ent/schema` + `generate.go` 的生成入口模式。
- Kratos 风格 CLI 强调命令分组与工具职责清晰。
- GoFrame 的 `gen` 命令体系强调“生成命令统一入口”，但本项目按依赖特征将 DB-introspective 生成留在服务本地更稳妥。

---

## 最终建议

按你这次的决策执行即可：

- `svr gen dao` 收敛为 Ent-only（并明确文案）
- gorm-gen 坚持服务本地自治（`make gen.gorm` / `cmd/genDao`）

这会显著降低中心化 CLI 的长期维护复杂度，同时保留 gorm-gen 的灵活性。

---

## `svr gen gorm` 设计（中心化方案）

### 背景与动机

虽然原决策将 gorm-gen 留在服务本地自治，但通过**读取服务配置文件**，可以实现中心化生成，消除重复代码。

**核心洞察**：
- gorm-gen 依赖数据库连接 → 从服务配置读取数据库信息
- 每个服务的 `cmd/gen/gorm-gen.go` 逻辑几乎相同 → 可以抽象为通用生成器
- 配置加载逻辑已存在 → 复用 `pkg/bootstrap/config/loader.go`

### 命令契约

```bash
svr gen gorm <服务名> [--dry-run]
```

**参数说明**：
- `<服务名>` - 必需，指定要生成的服务（如 `servora`）
- `--dry-run` - 可选，预览生成路径而不实际生成

**不支持 `--all` 的原因**：
- 首次生成时 `internal/data/gorm/` 目录不存在
- 无法通过目录结构判定哪些服务使用 GORM
- 避免误操作（如为不需要 GORM 的服务生成代码）

### 工作流程

```
1. 验证服务存在
   ↓
2. 加载服务配置（pkg/bootstrap/config/loader.go）
   ↓
3. 提取数据库配置（data.database）
   ↓
4. 连接数据库（支持 MySQL/PostgreSQL/SQLite）
   ↓
5. 配置 GORM GEN 生成器
   ↓
6. 生成 DAO 和 PO 到 internal/data/gorm/{dao,po}
   ↓
7. 输出生成结果
```

### 实现要点

#### 1. 配置加载

复用 `pkg/bootstrap/config/loader.go`：

```go
// internal/discovery/config.go
package discovery

import (
    "fmt"
    "path/filepath"
    
    "github.com/horonlee/servora/pkg/bootstrap/config"
    confv1 "github.com/horonlee/servora/api/gen/go/conf/v1"
)

type ServiceConfig struct {
    Name        string
    Path        string
    ConfigPath  string
    Bootstrap   *confv1.Bootstrap
}

// LoadServiceConfig 加载服务配置
func LoadServiceConfig(serviceName string) (*ServiceConfig, error) {
    servicePath := fmt.Sprintf("app/%s/service", serviceName)
    configPath := filepath.Join(servicePath, "configs")
    
    // 使用 pkg/bootstrap/config/loader.go
    bc, c, err := config.LoadBootstrap(configPath, serviceName)
    if err != nil {
        return nil, fmt.Errorf("load config failed: %w", err)
    }
    defer c.Close()
    
    return &ServiceConfig{
        Name:       serviceName,
        Path:       servicePath,
        ConfigPath: configPath,
        Bootstrap:  bc,
    }, nil
}
```

#### 2. GORM GEN 生成器封装

```go
// internal/generator/gorm.go
package generator

import (
    "fmt"
    "path/filepath"
    "strings"
    
    "gorm.io/gen"
    "gorm.io/driver/mysql"
    "gorm.io/driver/postgres"
    "github.com/glebarez/sqlite"
    "gorm.io/gorm"
    
    confv1 "github.com/horonlee/servora/api/gen/go/conf/v1"
)

type GormGenerator struct {
    ServiceName string
    ServicePath string
    DatabaseCfg *confv1.Data_Database
    DryRun      bool
}

func (g *GormGenerator) connectDB() (*gorm.DB, error) {
    if g.DatabaseCfg == nil {
        return nil, fmt.Errorf("database config is nil")
    }
    
    var dialector gorm.Dialector
    driver := strings.ToLower(g.DatabaseCfg.GetDriver())
    
    switch driver {
    case "mysql":
        dialector = mysql.Open(g.DatabaseCfg.GetSource())
    case "sqlite":
        dialector = sqlite.Open(g.DatabaseCfg.GetSource())
    case "postgres", "postgresql":
        dialector = postgres.Open(g.DatabaseCfg.GetSource())
    default:
        return nil, fmt.Errorf("unsupported db driver: %s", driver)
    }
    
    return gorm.Open(dialector, &gorm.Config{})
}

func (g *GormGenerator) Generate() error {
    // 连接数据库
    db, err := g.connectDB()
    if err != nil {
        return fmt.Errorf("connect db failed: %w", err)
    }
    
    // 构建输出路径
    daoPath := filepath.Join(g.ServicePath, "internal/data/gorm/dao")
    poPath := filepath.Join(g.ServicePath, "internal/data/gorm/po")
    
    if g.DryRun {
        fmt.Printf("[DRY-RUN] Would generate to:\n")
        fmt.Printf("  DAO: %s\n", daoPath)
        fmt.Printf("  PO:  %s\n", poPath)
        return nil
    }
    
    // 配置生成器
    generator := gen.NewGenerator(gen.Config{
        OutPath:       daoPath,
        ModelPkgPath:  poPath,
        Mode:          gen.WithDefaultQuery | gen.WithQueryInterface,
        FieldNullable: true,
    })
    
    generator.UseDB(db)
    
    // 生成所有表
    generator.ApplyBasic(generator.GenerateAllTable()...)
    
    // 执行生成
    generator.Execute()
    
    fmt.Printf("✓ Generated GORM code for service '%s'\n", g.ServiceName)
    fmt.Printf("  DAO: %s\n", daoPath)
    fmt.Printf("  PO:  %s\n", poPath)
    
    return nil
}
```

#### 3. 命令实现

```go
// internal/cmd/gen/gorm.go
package gen

import (
    "fmt"
    "os"
    "path/filepath"
    
    "github.com/spf13/cobra"
    "github.com/horonlee/servora/cmd/svr/internal/discovery"
    "github.com/horonlee/servora/cmd/svr/internal/generator"
)

func NewGormCmd() *cobra.Command {
    var dryRun bool
    
    cmd := &cobra.Command{
        Use:   "gorm <service-name>",
        Short: "Generate GORM DAO and PO code for a service",
        Long: `Generate GORM GEN DAO and PO code by reading service config.

This command reads the service's config.yaml to get database connection info,
then generates type-safe DAO and PO code to internal/data/gorm/{dao,po}.

Examples:
  svr gen gorm servora          # Generate for servora service
  svr gen gorm servora --dry-run # Preview without generating`,
        Args: cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            serviceName := args[0]
            return generateForService(serviceName, dryRun)
        },
    }
    
    cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without generating")
    
    return cmd
}

func generateForService(serviceName string, dryRun bool) error {
    // 1. 验证服务存在
    servicePath := fmt.Sprintf("app/%s/service", serviceName)
    if _, err := os.Stat(servicePath); os.IsNotExist(err) {
        return fmt.Errorf("service '%s' not found at %s", serviceName, servicePath)
    }
    
    // 2. 加载服务配置
    cfg, err := discovery.LoadServiceConfig(serviceName)
    if err != nil {
        return fmt.Errorf("load config: %w", err)
    }
    
    // 3. 验证数据库配置
    if cfg.Bootstrap.Data == nil || cfg.Bootstrap.Data.Database == nil {
        return fmt.Errorf("no database config found in %s/configs/config.yaml", servicePath)
    }
    
    // 4. 执行生成
    gen := &generator.GormGenerator{
        ServiceName: serviceName,
        ServicePath: servicePath,
        DatabaseCfg: cfg.Bootstrap.Data.Database,
        DryRun:      dryRun,
    }
    
    return gen.Generate()
}
```

### 错误处理

**明确的错误提示**：

1. **服务不存在**：
   ```
   Error: service 'xxx' not found at app/xxx/service
   
   Available services:
     - servora
     - sayhello
   ```

2. **配置文件不存在**：
   ```
   Error: config file not found at app/servora/service/configs/config.yaml
   
   Please ensure the service has a valid config.yaml file.
   ```

3. **数据库配置缺失**：
   ```
   Error: no database config found in app/servora/service/configs/config.yaml
   
   Please add data.database configuration:
     data:
       database:
         driver: mysql
         source: "user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4"
   ```

4. **数据库连接失败**：
   ```
   Error: connect db failed: dial tcp 127.0.0.1:3306: connect: connection refused
   
   Please check:
     1. Database server is running
     2. Connection string is correct in config.yaml
     3. Network connectivity to database
   ```

### 与 `make gen.gorm` 的集成

更新 `app.mk` 中的 `gen.gorm` 目标：

```makefile
# app.mk
.PHONY: gen.gorm
gen.gorm:
	@echo "Generating GORM DAO/PO..."
	@cd $(REPO_ROOT) && svr gen gorm $(SERVICE_NAME)
```

**优势**：
- 开发者仍可使用熟悉的 `make gen.gorm`
- 底层统一调用 `svr gen gorm`，消除重复代码
- 服务内 `cmd/gen/gorm-gen.go` 可逐步移除

### 迁移路径

#### Phase 1：实现 `svr gen gorm`
- [ ] 实现配置加载逻辑（复用 `pkg/bootstrap/config/loader.go`）
- [ ] 实现 GORM GEN 生成器封装
- [ ] 实现命令行接口
- [ ] 添加错误处理和用户友好提示

#### Phase 2：更新服务
- [ ] 在 `app.mk` 中更新 `gen.gorm` 目标调用 `svr gen gorm`
- [ ] 测试现有服务的生成流程
- [ ] 更新文档和示例

#### Phase 3：清理（可选）
- [ ] 删除各服务的 `cmd/gen/gorm-gen.go`（保留也不影响）
- [ ] 统一文档中的生成命令说明

### 优势总结

✅ **消除重复代码**：不再需要每个服务维护 `cmd/gen/gorm-gen.go`  
✅ **统一生成逻辑**：便于维护和升级 GORM GEN 版本  
✅ **保留配置自治**：每个服务独立配置数据库连接  
✅ **向后兼容**：`make gen.gorm` 仍可使用  
✅ **明确错误提示**：数据库连接失败时给出清晰指引  
✅ **支持预览**：`--dry-run` 模式预览生成路径

### 注意事项

1. **数据库可用性**：生成时需要数据库可访问
2. **配置文件标准化**：假定配置在 `configs/config.yaml`
3. **首次生成**：需要手动创建 `internal/data/gorm/` 目录（生成器会自动创建子目录）
4. **环境变量**：支持通过环境变量覆盖配置（由 `pkg/bootstrap/config/loader.go` 处理）

---

## `svr` 命令行程序骨架设计（可扩展优先）

为支持后续持续新增命令（如 `svr new api`、`svr new svc`），`svr` 采用“命令分组 + 模块化注册 + 模板层解耦”的骨架。

### 设计目标

1. 新增子命令时避免修改大量已有代码。
2. 命令层只做参数校验和流程编排，业务逻辑下沉到独立模块。
3. 输出与错误码统一，方便接入 CI 与自动化脚本。

### 命令树（v1）

```bash
svr
├── gen
│   ├── dao        # Ent-only：批量或单服务执行 ent 生成
│   └── gorm       # GORM GEN：为指定服务生成 DAO/PO（读取服务配置）
├── new
│   ├── api        # 创建 API/proto 骨架
│   └── svc        # 创建微服务骨架（可选 with-ent）
├── version
└── help
```

### 目录骨架（建议）

```text
cmd/svr/
  main.go
  internal/
    root/
      root.go                # 根命令与全局 flags
    cmd/
      gen/
        gen.go               # 注册 gen 命令组
        dao.go               # Ent-only 生成入口
        gorm.go              # GORM GEN 生成入口
      new/
        new.go               # 注册 new 命令组
        api.go               # 创建 api/proto 骨架
        svc.go               # 创建服务骨架
    discovery/
      services.go            # 扫描 app/*/service
      ent.go                 # Ent 能力判定（generate.go 含 ent 入口）
      config.go              # 服务配置加载（复用 pkg/bootstrap/config/loader.go）
    generator/
      gorm.go                # GORM GEN 生成逻辑封装
    scaffold/
      renderer.go            # 模板渲染与文件输出
      templates/
        api/
        service/
    ux/
      output.go              # 统一输出（success/skip/fail）
      errors.go              # 统一错误码与提示文案
```

### 扩展机制（新增命令的标准方式）

每个命令组暴露统一注册函数：

```go
// 伪代码示意
func Register(parent *cobra.Command) {
    parent.AddCommand(NewCmd())
}
```

新增一个命令的流程固定为：

1. 在 `internal/cmd/<group>/` 新建命令文件。
2. 在对应组的 `Register(...)` 中注册。
3. 如涉及发现/脚手架能力，分别放入 `discovery/` 或 `scaffold/`，避免命令层膨胀。

### v1 命令契约（最小可用）

#### `svr gen dao`

- `svr gen dao [--service <name>|--all] [--dry-run] [--fail-fast]`
- 识别规则：仅 `internal/data/generate.go` 中包含 `entgo.io/ent/cmd/ent`
- 非 Ent 服务输出 `unsupported` 并给出 gorm 本地命令指引

#### `svr gen gorm`

- `svr gen gorm <服务名> [--dry-run]`
- **必须指定服务名**（不支持 `--all`，因为首次生成时 gorm 目录不存在）
- 工作流程：
  1. 读取服务配置：`app/<服务名>/service/configs/config.yaml`
  2. 使用 `pkg/bootstrap/config/loader.go` 加载配置
  3. 连接数据库（使用配置中的 `data.database`）
  4. 生成 DAO 和 PO 到 `app/<服务名>/service/internal/data/gorm/{dao,po}`
- 错误处理：
  - 服务不存在 → 明确提示
  - 配置文件不存在 → 提示配置路径
  - 数据库连接失败 → 提示检查数据库配置和可用性
  - 生成失败 → 输出详细错误信息

**示例输出**：
```bash
$ svr gen gorm servora
✓ Loaded config from app/servora/service/configs
✓ Connected to database (mysql)
✓ Generated GORM code for service 'servora'
  DAO: app/servora/service/internal/data/gorm/dao
  PO:  app/servora/service/internal/data/gorm/po

$ svr gen gorm servora --dry-run
[DRY-RUN] Would generate to:
  DAO: app/servora/service/internal/data/gorm/dao
  PO:  app/servora/service/internal/data/gorm/po
```

#### `svr new api`

- `svr new api --name <name> [--version v1] [--http] [--grpc] [--dry-run]`
- 生成 `api/protos/<name>/service/<version>/` 基础 proto 骨架

#### `svr new svc`

- `svr new svc --name <name> [--with-ent] [--standalone] [--dry-run]`
- 生成 `app/<name>/service` 基础目录与 `Makefile(include ../../../app.mk)`
- `--with-ent` 时附带 `internal/data/generate.go` 与最小 schema 模板

### 与 Make 的协作边界（更新）

- Make 保持稳定流水线入口（`api/wire/openapi/ent`）。
- `svr` 负责高逻辑命令：服务发现、脚手架、策略执行。
- **gorm-gen 协作模式**：
  - `svr gen gorm <服务名>` - 中心化生成入口（消除重复代码）
  - `make gen.gorm` - 服务本地便捷入口（内部调用 `svr gen gorm`）
  - 服务内 `cmd/gen/gorm-gen.go` 可逐步移除（可选）

### 演进路线（补充）

#### v1

- [ ] 完成 `svr` 根命令骨架与 `gen/new` 分组
- [ ] 完成 `svr gen dao`（Ent-only）
- [ ] 完成 `svr gen gorm`（中心化 GORM GEN 生成）
- [ ] 完成 `svr new api` 与 `svr new svc` 的最小模板生成

#### v1.5

- [ ] 增加 `svr doctor`（工具链/目录规范检查）
- [ ] 增加统一 JSON 输出模式（便于 CI 解析）

#### v2

- [ ] 模板版本化与可配置模板源
- [ ] 可选插件机制（仅在确有需求时启用）
