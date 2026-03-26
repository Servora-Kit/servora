package http

import stdhttp "net/http"

// Connection HTTP 连接封装，实现 runtime.Connection。
type Connection struct {
	cli      *stdhttp.Client
	endpoint string
}

func NewConnection(cli *stdhttp.Client, endpoint string) *Connection {
	return &Connection{cli: cli, endpoint: endpoint}
}

func (h *Connection) Value() any      { return h.cli }
func (h *Connection) Close() error    { return nil }
func (h *Connection) IsHealthy() bool { return h.cli != nil }

// Endpoint 返回 HTTP 连接基础地址，便于上层拼接路径。
func (h *Connection) Endpoint() string { return h.endpoint }
