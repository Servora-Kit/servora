## 1. 项目初始化

- [ ] 1.1 添加 `github.com/spf13/cobra` 依赖到 go.mod
- [ ] 1.2 添加 `github.com/charmbracelet/lipgloss` 依赖到 go.mod
- [ ] 1.3 添加 `github.com/charmbracelet/huh` 依赖（用于无参数交互）
- [ ] 1.4 创建 `cmd/svr/` 目录结构
- [ ] 1.5 创建 `cmd/svr/main.go` 主入口文件
- [ ] 1.6 创建 `cmd/svr/internal/root/root.go` 根命令

## 2. 配置加载模块

- [ ] 2.1 创建 `cmd/svr/internal/discovery/config.go`
- [ ] 2.2 实现 `ServiceConfig` 结构体
- [ ] 2.3 实现 `LoadServiceConfig()` 函数，复用 `pkg/bootstrap/config/loader.LoadBootstrap()`
- [ ] 2.4 添加服务存在性验证逻辑
- [ ] 2.5 添加配置文件存在性验证逻辑
- [ ] 2.6 添加数据库配置验证逻辑

## 3. GORM GEN 生成器封装

- [ ] 3.1 创建 `cmd/svr/internal/generator/gorm.go`
- [ ] 3.2 实现 `GormGenerator` 结构体
- [ ] 3.3 实现 `connectDB()` 方法，支持 MySQL/PostgreSQL/SQLite
- [ ] 3.4 实现 `Generate()` 方法，配置并执行 GORM GEN
- [ ] 3.5 实现 `--dry-run` 预览模式逻辑
- [ ] 3.6 添加生成成功的输出提示

## 4. 命令行接口

- [ ] 4.1 创建 `cmd/svr/internal/cmd/gen/gen.go` gen 命令组
- [ ] 4.2 创建 `cmd/svr/internal/cmd/gen/gorm.go` gorm 子命令
- [ ] 4.3 实现命令参数解析（支持多个服务名、--dry-run 标志）
- [ ] 4.4 实现 `generateForService()` 函数，编排配置加载和生成流程
- [ ] 4.5 实现多服务批量生成逻辑（循环处理，记录成功/失败）
- [ ] 4.6 实现无参数时进入 huh 交互（服务多选 + 确认）
- [ ] 4.7 添加服务不存在的错误处理和提示
- [ ] 4.8 添加配置文件不存在的错误处理和提示
- [ ] 4.9 添加数据库配置缺失的错误处理和提示
- [ ] 4.10 添加数据库连接失败的错误处理和提示
- [ ] 4.11 添加批量生成的总结输出（成功 X 个，失败 Y 个 + 每个失败错误详情）
- [ ] 4.12 实现退出码策略（全成功=0，部分/全部失败=1）
- [ ] 4.13 实现失败错误分类（service-not-found/config-invalid/db-connect-failed/generation-failed）
- [ ] 4.14 实现标准化汇总格式输出（`Summary: success=X failed=Y` + 失败项模板）
- [ ] 4.15 实现交互模式空选择退出（输出 `No services selected`，退出码 0）
- [ ] 4.16 实现交互模式取消退出（输出 `Cancelled by user`，退出码 0）

## 5. CLI 输出美化

- [ ] 5.1 创建 `cmd/svr/internal/ux/output.go`
- [ ] 5.2 参考 Ech0 项目，定义 lipgloss 样式（titleStyle, successStyle, errorStyle, highlightStyle）
- [ ] 5.3 实现 `PrintSuccess()` 函数（绿色 ✓ 标记）
- [ ] 5.4 实现 `PrintError()` 函数（红色 ✗ 标记）
- [ ] 5.5 实现 `PrintInfo()` 函数（普通信息输出）
- [ ] 5.6 实现 `PrintProgress()` 函数（进度指示器）
- [ ] 5.7 使用 `lipgloss.AdaptiveColor` 支持浅色/深色主题
- [ ] 5.8 在生成器和命令中使用美化输出函数
- [ ] 5.9 进度展示以“微服务”为最小单位（例如 `[2/5] generating xxx`）

## 6. 更新 app.mk

- [ ] 6.1 备份当前 `app.mk` 中的 `gen.gorm` 目标
- [ ] 6.2 更新 `gen.gorm` 目标，调用 `svr gen gorm $(SERVICE_NAME)`
- [ ] 6.3 确保 `SERVICE_NAME` 变量正确提取服务名称
- [ ] 6.4 添加输出提示 "Generating GORM DAO/PO..."

## 7. 测试验证

- [ ] 7.1 在 `app/servora/service` 目录下测试 `svr gen gorm servora`
- [ ] 7.2 测试多服务批量生成 `svr gen gorm servora sayhello`
- [ ] 7.3 验证生成的 DAO 和 PO 代码位置正确
- [ ] 7.4 验证生成的代码格式正确（WithDefaultQuery、WithQueryInterface、FieldNullable）
- [ ] 7.5 测试 `--dry-run` 模式
- [ ] 7.6 测试错误场景（服务不存在、配置缺失、数据库连接失败）
- [ ] 7.7 验证 CLI 输出美化效果（颜色、样式、emoji）
- [ ] 7.8 在浅色和深色终端主题下测试输出
- [ ] 7.9 测试无参数交互模式（服务选择、确认、取消）
- [ ] 7.10 测试批量模式中单服务失败后继续执行并正确汇总
- [ ] 7.11 验证退出码策略（全成功=0，存在失败=1）
- [ ] 7.12 验证失败分类准确性（4 种错误类型映射）
- [ ] 7.13 验证汇总输出格式可解析（固定模板）
- [ ] 7.14 测试交互模式空选择与取消行为（均退出码 0）
- [ ] 7.15 在 `app/servora/service` 目录下测试 `make gen.gorm`
- [ ] 7.16 验证 `make gen.gorm` 与直接调用 `svr gen gorm` 结果一致

## 8. 文档更新

- [ ] 8.1 更新 `docs/design/cmd-tools.md`，标记 Phase 1 完成
- [ ] 8.2 更新 `README.md`，添加 `svr gen gorm` 命令说明
- [ ] 8.3 在 README 中明确退出码语义（全成功=0，任意失败=1）
- [ ] 8.4 更新 `AGENTS.md`，添加 `svr gen gorm` 使用指南
- [ ] 8.5 更新 `app/servora/service/AGENTS.md`，说明新的生成方式

## 9. 清理（可选）

- [ ] 9.1 评估是否删除 `app/servora/service/cmd/gen/gorm-gen.go`
- [ ] 9.2 如果删除，更新相关文档说明
- [ ] 9.3 如果保留，添加注释说明推荐使用 `svr gen gorm`
