## 上下文

当前 servora 项目中，每个使用 GORM GEN 的微服务都维护着独立的 `cmd/gen/gorm-gen.go` 文件。这些文件的核心逻辑高度相似：
1. 从配置文件加载数据库连接信息
2. 连接数据库
3. 配置 GORM GEN 生成器
4. 执行生成到 `internal/data/gorm/{dao,po}`

这种重复导致：
- 代码维护成本高（每个服务都要同步更新）
- 升级 GORM GEN 版本时需要修改多处
- 新服务需要复制粘贴相同的代码

项目已有 `pkg/bootstrap/config/loader.go` 用于统一加载服务配置，可以复用。设计文档 `docs/design/cmd-tools.md` 已完成，明确了命令契约和实现要点。

**约束**：
- 必须向后兼容现有的 `make gen.gorm` 工作流
- 不能依赖 `internal/data/gorm/` 目录的存在（首次生成时不存在）
- 必须支持 MySQL/PostgreSQL/SQLite 三种数据库
- 错误提示必须清晰，帮助开发者快速定位问题

## 目标 / 非目标

**目标：**
1. 实现 `svr gen gorm <服务名...>` 命令，支持单个或多个服务批量生成
2. 复用 `pkg/bootstrap/config/loader.go` 加载服务配置
3. 提供清晰的错误提示（服务不存在、配置缺失、数据库连接失败）
4. 支持 `--dry-run` 预览模式
5. 使用 `charmbracelet/lipgloss` 美化 CLI 输出，参考 Ech0 项目设计
6. 在无参数执行时进入 `huh` 交互式服务选择
7. 更新 `app.mk` 使 `make gen.gorm` 调用新命令
8. 保持向后兼容，不破坏现有工作流

**非目标：**
1. 不支持 `--all` 自动发现（因为无法通过目录判定哪些服务使用 GORM）
2. 不修改 GORM GEN 的生成逻辑（只是封装调用）
3. 不强制删除服务内的 `cmd/gen/gorm-gen.go`（可选清理）
4. 不支持自定义生成策略（如指定表、字段映射等，保持简单）
5. 不在本次实现完整 TUI 应用，仅实现命令行内的轻量交互

## 决策

### 1. 命令行框架选择：Cobra

**决策**：使用 `github.com/spf13/cobra` 作为命令行框架

**理由**：
- Cobra 是 Go 生态标准的 CLI 框架（Kubernetes、Docker 等都在使用）
- 支持子命令、标志、自动生成帮助文档
- 与 Kratos CLI 风格一致

**替代方案**：
- `flag` 标准库：功能太简单，不支持子命令
- `urfave/cli`：功能类似但社区不如 Cobra 活跃

### 2. 配置加载：复用 pkg/bootstrap/config/loader.go

**决策**：直接调用 `pkg/bootstrap/config/loader.LoadBootstrap()` 加载服务配置

**理由**：
- 已有实现，支持配置中心（Nacos/Consul/etcd）和环境变量覆盖
- 返回完整的 `*conf.Bootstrap` 结构，包含数据库配置
- 避免重复实现配置加载逻辑

**实现细节**：
```go
// internal/discovery/config.go
func LoadServiceConfig(serviceName string) (*ServiceConfig, error) {
    servicePath := fmt.Sprintf("app/%s/service", serviceName)
    configPath := filepath.Join(servicePath, "configs")
    
    bc, c, err := config.LoadBootstrap(configPath, serviceName)
    if err != nil {
        return nil, fmt.Errorf("load config failed: %w", err)
    }
    defer c.Close()
    
    return &ServiceConfig{
        Name:      serviceName,
        Path:      servicePath,
        Bootstrap: bc,
    }, nil
}
```

### 3. 目录结构：模块化设计

**决策**：采用以下目录结构

```
cmd/svr/
  main.go
  internal/
    root/
      root.go              # 根命令
    cmd/
      gen/
        gen.go             # gen 命令组
        gorm.go            # gorm 子命令
    discovery/
      config.go            # 配置加载
    generator/
      gorm.go              # GORM GEN 封装
    ux/
      output.go            # 统一输出
```

**理由**：
- 命令层（`cmd/`）只做参数校验和流程编排
- 业务逻辑下沉到独立模块（`discovery/`, `generator/`）
- 便于测试和扩展（未来可添加 `svr gen dao`, `svr new svc` 等）

### 4. 支持多服务批量生成

**决策**：支持在命令行指定多个服务名进行批量生成

```bash
svr gen gorm servora sayhello another-service
```

**理由**：
- 虽然不支持 `--all` 自动发现，但可以显式指定多个服务
- 配合 Makefile 变量功能，可以批量生成多个微服务
- 保持命令简单明确，避免误操作

**实现方式**：
```go
// 命令定义
cmd := &cobra.Command{
    Use:   "gorm <service-name...>",
    Args:  cobra.MinimumNArgs(1),  // 至少需要一个服务名
    RunE: func(cmd *cobra.Command, args []string) error {
        for _, serviceName := range args {
            if err := generateForService(serviceName, dryRun); err != nil {
                // 记录错误但继续处理其他服务
            }
        }
    },
}
```

**Makefile 集成示例**：
```makefile
# 批量生成多个服务
GORM_SERVICES := servora sayhello
gen.gorm.all:
	@cd $(REPO_ROOT) && svr gen gorm $(GORM_SERVICES)
```

### 5. 不支持 --all 自动发现

**决策**：不支持 `--all` 自动发现所有使用 GORM 的服务

**理由**：
- 首次生成时 `internal/data/gorm/` 目录不存在，无法通过目录判定
- 避免误操作（为不需要 GORM 的服务生成代码）
- 显式指定服务名更安全明确

**替代方案**：
- 通过检测 `go.mod` 中是否依赖 `gorm.io/gen`：不可靠，可能已安装但未使用
- 通过配置文件标记：增加配置复杂度
- **推荐**：使用多服务名参数 + Makefile 变量管理需要生成的服务列表

### 6. CLI 输出美化

**决策**：使用 `charmbracelet/lipgloss` 美化 CLI 输出，参考 Ech0 项目设计

**理由**：
- 提升用户体验，输出更美观易读
- `lipgloss` 是 Go 生态成熟的 CLI 样式库
- Ech0 项目已有成功实践，可以参考其设计模式

**实现要点**：
```go
// internal/ux/output.go
import "github.com/charmbracelet/lipgloss"

var (
    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#FF7F7F"))
    
    successStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#B8E6B8"))
    
    errorStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#FF6B6B"))
)

func PrintSuccess(title, msg string) {
    fmt.Println(successStyle.Render("✓ " + title + ": " + msg))
}

func PrintError(title, msg string) {
    fmt.Println(errorStyle.Render("✗ " + title + ": " + msg))
}
```

**样式系统约束**：
- 使用统一样式 token：`Title`、`Info`、`Success`、`Warn`、`Error`、`Path`、`Counter`、`Summary`
- 进度展示以“微服务”为最小单位（例如 `[2/5] generating sayhello`）
- 汇总展示必须包含成功/失败数量和失败详情列表

### 7. 交互式入口（无参数）

**决策**：当用户执行 `svr gen gorm` 不带任何服务名时，进入 `huh` 交互式流程。

**理由**：
- 降低命令记忆成本，提升可发现性
- 允许用户在终端中多选服务，适配批量场景

**交互流程**：
1. 扫描 `app/*/service` 获取候选服务
2. 展示多选列表（支持全选/反选）
3. 显示确认步骤（将要生成的服务列表）
4. 用户确认后按批量模式执行

### 8. 批量失败策略

**决策**：多服务模式采用“继续执行 + 最终汇总”策略。

**执行规则**：
- 单个服务失败时，记录 `{service, error}` 并继续执行后续服务
- 执行结束后输出汇总：成功数、失败数、每个失败服务及错误信息
- 退出码规则：全部成功返回 `0`；存在任一失败返回 `1`

### 9. 错误处理策略

**决策**：提供 4 类明确的错误提示

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

**理由**：
- 明确的错误提示减少开发者调试时间
- 提供可操作的解决建议
- 避免模糊的错误信息（如 "generation failed"）

### 10. 与 make gen.gorm 的集成

**决策**：更新 `app.mk` 中的 `gen.gorm` 目标调用 `svr gen gorm`

```makefile
# app.mk
.PHONY: gen.gorm
gen.gorm:
	@echo "Generating GORM DAO/PO..."
	@cd $(REPO_ROOT) && svr gen gorm $(SERVICE_NAME)
```

**理由**：
- 开发者仍可使用熟悉的 `make gen.gorm`
- 底层统一调用中心化命令
- 服务内 `cmd/gen/gorm-gen.go` 可逐步移除（可选）

**替代方案**：
- 保留 `cmd/gen/gorm-gen.go`，让 `make gen.gorm` 继续调用它：无法消除重复代码
- 直接删除 `make gen.gorm`，强制使用 `svr gen gorm`：破坏现有工作流

## 风险 / 权衡

### 风险 1：数据库连接失败导致生成中断

**风险**：生成时需要数据库可访问，如果数据库未启动或网络不通，生成会失败

**缓解措施**：
- 提供清晰的错误提示，说明如何检查数据库连接
- 支持 `--dry-run` 模式，可以在不连接数据库的情况下预览生成路径
- 文档中明确说明生成前需要确保数据库可访问

### 风险 2：配置文件路径假设

**风险**：假定配置文件在 `configs/config.yaml`，如果服务使用不同路径会失败

**缓解措施**：
- 项目已标准化配置路径为 `configs/config.yaml`
- 错误提示中明确显示期望的配置路径
- 未来可扩展支持 `--config` 参数指定自定义路径（非本次范围）

### 风险 3：首次生成时目录不存在

**风险**：首次生成时 `internal/data/gorm/` 目录不存在，可能导致生成失败

**缓解措施**：
- GORM GEN 会自动创建输出目录
- 如果需要，在生成前检查并创建父目录
- 文档中说明首次生成的注意事项

### 权衡 1：不支持自定义生成策略

**权衡**：当前设计只支持生成所有表，不支持指定表、字段映射等自定义策略

**理由**：
- 保持命令简单，覆盖 80% 的使用场景
- 复杂场景仍可使用服务内的 `cmd/gen/gorm-gen.go`
- 未来可扩展支持配置文件指定生成策略

### 权衡 2：依赖 Cobra 框架

**权衡**：引入 `github.com/spf13/cobra` 依赖

**理由**：
- Cobra 是 Go 生态标准，依赖稳定
- 项目已有其他第三方依赖（Kratos、Wire 等），增加一个 CLI 框架合理
- 为未来扩展其他命令（`svr new svc`, `svr doctor` 等）打下基础

## 迁移计划

### Phase 1：实现核心功能（本次变更）

1. 添加 `github.com/spf13/cobra` 依赖
2. 实现 `cmd/svr` 命令行工具骨架
3. 实现 `internal/discovery/config.go` 配置加载
4. 实现 `internal/generator/gorm.go` 生成器封装
5. 实现 `internal/cmd/gen/gorm.go` 命令
6. 更新 `app.mk` 中的 `gen.gorm` 目标
7. 更新文档（README.md, AGENTS.md）

### Phase 2：验证与清理（后续可选）

1. 在 `servora` 服务上测试生成流程
2. 验证 `make gen.gorm` 工作正常
3. 可选：删除 `app/servora/service/cmd/gen/gorm-gen.go`
4. 可选：为其他服务应用相同流程

### 回滚策略

如果新命令出现问题：
1. 恢复 `app.mk` 中的 `gen.gorm` 目标为原实现
2. 服务内的 `cmd/gen/gorm-gen.go` 仍然存在，可直接使用
3. 无破坏性变更，回滚成本低

## 开放问题

1. **是否需要支持配置文件指定生成策略？**
   - 当前设计：生成所有表
   - 可能需求：只生成指定表、自定义字段映射
   - 决策：暂不支持，保持简单，未来可扩展

2. **是否需要支持 --config 参数指定配置路径？**
   - 当前设计：固定使用 `configs/config.yaml`
   - 可能需求：支持自定义配置路径
   - 决策：暂不支持，项目已标准化配置路径

3. **是否需要在生成前验证数据库连接？**
   - 当前设计：直接连接数据库，失败时报错
   - 可能需求：先 ping 数据库，再执行生成
   - 决策：不需要，GORM 连接失败已有明确错误
