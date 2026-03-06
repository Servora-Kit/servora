## 新增需求

## 修改需求

### 需求:所有 proto 文件必须显式声明 go_package

每个 proto 文件必须包含 `option go_package` 声明，明确指定生成的 Go 代码的 package 路径和别名。

#### 场景:添加 go_package 声明

- **当** proto 文件缺少 `option go_package` 声明
- **那么** 必须添加格式为 `option go_package = "github.com/Servora-Kit/servora/api/gen/go/<path>;<alias>";` 的声明

#### 场景:go_package 路径格式

- **当** 为 proto 文件添加 `option go_package`
- **那么** 路径必须遵循格式：`github.com/Servora-Kit/servora/api/gen/go/<proto_path>;<package_alias>`
- **那么** `<proto_path>` 必须与 proto 文件的目录结构对应（如 `auth/service/v1`）
- **那么** `<package_alias>` 必须使用版本号作为别名（如 `v1`）

#### 场景:更新现有 go_package 声明

- **当** proto 文件已有 `option go_package` 声明使用旧路径 `github.com/horonlee/servora`
- **那么** 必须将路径更新为 `github.com/Servora-Kit/servora`
- **那么** 必须保持 package 别名不变

## 移除需求
