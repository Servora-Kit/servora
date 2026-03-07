package server

// PluginKind 定义可插拔服务器类型。
type PluginKind string

const (
	PluginWebSocket PluginKind = "websocket"
	PluginMCP       PluginKind = "mcp"
	PluginGraphQL   PluginKind = "graphql"
)

// ServerPlugin 定义可插拔服务器接口，用于扩展 WebSocket/MCP/GraphQL 等协议。
type ServerPlugin interface {
	Server
	Kind() PluginKind
}
