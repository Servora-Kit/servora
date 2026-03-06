# CI/CD 设计文档

本文档说明 Servora 框架的双分支 CI/CD 架构设计。

## 概述

Servora 采用双分支策略：
- **main 分支**：框架核心代码
- **example 分支**：完整示例项目

两个分支需要不同的 CI/CD 流程。

## main 分支 CI/CD

### 触发条件

- Push 到 main 分支
- Pull Request 到 main 分支

### 工作流程

```yaml
name: Main Branch CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Lint Go code
        run: make lint.go
      - name: Lint Proto files
        run: make lint.proto

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Run tests
        run: make test

  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Build CLI tool
        run: make build
```

### 检查项

1. **代码检查**
   - `make lint.go`：Go 代码 lint
   - `make lint.proto`：Proto 文件 lint

2. **单元测试**
   - `make test`：运行 pkg/ 和 cmd/ 的单元测试

3. **构建验证**
   - `make build`：验证 CLI 工具可以正常构建

### 不包含的检查

- ❌ 服务启动测试（main 分支没有完整服务）
- ❌ 集成测试（main 分支没有数据库配置）
- ❌ 前端测试（main 分支没有前端代码）

## example 分支 CI/CD

### 触发条件

- Push 到 example 分支
- Pull Request 到 example 分支

### 工作流程

```yaml
name: Example Branch CI

on:
  push:
    branches: [example]
  pull_request:
    branches: [example]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Lint Go code
        run: make lint.go
      - name: Lint Proto files
        run: make lint.proto

  test:
    runs-on: ubuntu-latest
    services:
      mysql:
        image: mysql:8.0
        env:
          MYSQL_ROOT_PASSWORD: test
          MYSQL_DATABASE: servora_test
        ports:
          - 3306:3306
      redis:
        image: redis:7
        ports:
          - 6379:6379
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Run unit tests
        run: make test
      - name: Run integration tests
        run: make test.integration
        env:
          DB_HOST: localhost
          DB_PORT: 3306
          DB_USER: root
          DB_PASSWORD: test
          DB_NAME: servora_test
          REDIS_HOST: localhost
          REDIS_PORT: 6379

  smoke-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Start services
        run: make compose.dev
      - name: Wait for services
        run: sleep 10
      - name: Health check
        run: |
          curl -f http://localhost:8000/health || exit 1
          curl -f http://localhost:8001/health || exit 1
      - name: Stop services
        run: make compose.dev.down
```

### 检查项

1. **代码检查**
   - `make lint.go`：Go 代码 lint
   - `make lint.proto`：Proto 文件 lint

2. **单元测试**
   - `make test`：运行所有单元测试

3. **集成测试**
   - 使用 GitHub Actions services 启动 MySQL 和 Redis
   - 运行集成测试，验证数据库交互

4. **冒烟测试**
   - 启动完整的 Docker Compose 环境
   - 执行健康检查，验证服务可以正常启动
   - 验证 HTTP 端点可访问

### 未来扩展

- **API 测试**：使用 Postman/Newman 测试 API 端点
- **性能测试**：使用 k6 进行负载测试
- **前端测试**：运行 Vue 组件测试和 E2E 测试

## 冒烟测试 vs 集成测试

### 冒烟测试（Smoke Test）

- **目的**：快速验证系统的基本功能是否正常
- **范围**：最关键的功能路径
- **执行时间**：快速（几分钟）
- **示例**：
  - 服务能否启动
  - 健康检查端点是否响应
  - 数据库连接是否正常

### 集成测试（Integration Test）

- **目的**：验证多个组件协同工作
- **范围**：完整的业务流程
- **执行时间**：较长（可能需要十几分钟）
- **示例**：
  - 用户注册流程
  - 数据库事务处理
  - 缓存一致性
  - 服务间通信

## GitHub Actions 行为

### 测试失败时

- ❌ PR 检查状态显示为失败
- 🚫 无法合并 PR（如果启用了分支保护）
- 📧 提交者收到邮件通知
- 💬 PR 页面显示失败的检查详情

### 查看失败详情

1. 在 PR 页面点击 "Details" 查看失败的 job
2. 查看日志输出，定位失败原因
3. 修复问题后重新推送，CI 会自动重新运行

## 实施优先级

### 第一阶段（当前）

- ✅ Git Hooks 本地验证
- ⏳ 基础 CI 流程（lint + test）

### 第二阶段（未来）

- ⏳ 集成测试
- ⏳ 冒烟测试
- ⏳ 自动化部署

### 第三阶段（可选）

- ⏳ 性能测试
- ⏳ 安全扫描
- ⏳ 依赖更新自动化

## 注意事项

1. **Proto lint 输出较长**：当前不在 CI 中启用 proto lint，因为输出信息较多
2. **测试数据库**：集成测试使用 GitHub Actions services 提供的临时数据库
3. **并行执行**：lint、test、build 可以并行执行以加快速度
4. **缓存优化**：使用 actions/cache 缓存 Go modules 和构建产物
