## 新增需求

### 需求:Plugin Kind 常量

系统必须定义 `PluginKind` 类型和预留的协议常量，用于标识可插拔服务器类型。

#### 场景:定义 PluginKind 类型

- **当** 导入 `pkg/transport/server` 包
- **那么** 必须能够使用 `PluginKind` 类型

#### 场景:预留 WebSocket 常量

- **当** 访问 `PluginWebSocket` 常量
- **那么** 必须返回 `"websocket"` 字符串值

#### 场景:预留 MCP 常量

- **当** 访问 `PluginMCP` 常量
- **那么** 必须返回 `"mcp"` 字符串值

#### 场景:预留 GraphQL 常量

- **当** 访问 `PluginGraphQL` 常量
- **那么** 必须返回 `"graphql"` 字符串值

---

### 需求:ServerPlugin 接口

系统必须定义 `ServerPlugin` 接口，继承 `Server` 接口并添加 `Kind()` 方法，用于可插拔协议扩展。

#### 场景:接口定义

- **当** 实现 `ServerPlugin` 接口
- **那么** 必须同时实现 `Server` 接口（`Start`、`Stop`、`Endpoint`）和 `Kind() PluginKind` 方法

#### 场景:接口可扩展

- **当** 未来需要添加 WebSocket Server 实现
- **那么** 必须能够通过实现 `ServerPlugin` 接口来扩展，无需修改现有代码

---

### 需求:仅定义不实现

系统禁止在本次变更中实现具体的 Plugin（WebSocket/MCP/GraphQL），仅定义接口和常量。

#### 场景:无具体实现

- **当** 检查 `pkg/transport/server/plugin.go` 文件
- **那么** 必须只包含类型定义和常量，禁止包含任何具体的 Server 实现
