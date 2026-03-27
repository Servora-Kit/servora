package http

import khttp "github.com/go-kratos/kratos/v2/transport/http"

// Session HTTP 连接封装，实现 runtime.Connection。
type Session struct {
	cli      *khttp.Client
	endpoint string
}

func NewSession(cli *khttp.Client, endpoint string) *Session {
	return &Session{cli: cli, endpoint: endpoint}
}

func (h *Session) Value() any      { return h.cli }
func (h *Session) Close() error    { return h.cli.Close() }
func (h *Session) IsHealthy() bool { return h.cli != nil }

// Endpoint 返回 HTTP 连接基础地址，便于上层拼接路径。
func (h *Session) Endpoint() string { return h.endpoint }
