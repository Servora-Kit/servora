## 修改需求

### 需求:gen.gorm 目标调用中心化命令

`app.mk` 中的 `gen.gorm` 目标必须调用 `svr gen gorm` 命令，而不是执行服务本地的生成脚本。

#### 场景:执行 make gen.gorm

- **当** 开发者在服务目录下执行 `make gen.gorm`
- **那么** 系统切换到仓库根目录
- **那么** 系统执行 `svr gen gorm <服务名>`
- **那么** 生成结果与直接调用 `svr gen gorm` 相同

#### 场景:服务名称自动识别

- **当** 开发者在 `app/servora/service` 目录下执行 `make gen.gorm`
- **那么** 系统自动识别服务名称为 "servora"
- **那么** 系统执行 `svr gen gorm servora`

#### 场景:输出提示

- **当** 开发者执行 `make gen.gorm`
- **那么** 系统首先输出 "Generating GORM DAO/PO..."
- **那么** 系统然后输出 `svr gen gorm` 的执行结果

### 需求:向后兼容

更新后的 `gen.gorm` 目标必须保持与原有工作流的兼容性。

#### 场景:开发者体验不变

- **当** 开发者执行 `make gen.gorm`
- **那么** 生成的 DAO 和 PO 代码位置与之前相同
- **那么** 生成的代码格式与之前相同
- **那么** 命令执行成功时退出码为 0
- **那么** 命令执行失败时退出码非 0

#### 场景:错误处理

- **当** `svr gen gorm` 执行失败
- **那么** `make gen.gorm` 也失败并返回相同的错误
- **那么** 错误信息清晰显示失败原因
