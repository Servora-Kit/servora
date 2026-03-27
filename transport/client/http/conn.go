package http

import khttp "github.com/go-kratos/kratos/v2/transport/http"

// Connection HTTP 连接封装，实现 runtime.Connection。
type Connection struct {
	cli      *khttp.Client
	endpoint string
}

func NewConnection(cli *khttp.Client, endpoint string) *Connection {
	return &Connection{cli: cli, endpoint: endpoint}
}

func (h *Connection) Value() any      { return h.cli }
func (h *Connection) Close() error    { return h.cli.Close() }
func (h *Connection) IsHealthy() bool { return h.cli != nil }

// Endpoint 返回 HTTP 连接基础地址，便于上层拼接路径。
func (h *Connection) Endpoint() string { return h.endpoint }
