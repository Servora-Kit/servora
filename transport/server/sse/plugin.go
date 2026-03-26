package sse

import (
	"context"
	"net/url"

	"github.com/Servora-Kit/servora/transport/runtime"
)

// Plugin 为 SSE 保留 runtime graph 扩展位。
// SSE 当前是 HTTP handler 形态，不独立监听端口，因此返回 no-op runtime server。
type Plugin struct{}

const Type = "sse"

func (p *Plugin) Type() string { return Type }

func (p *Plugin) Build(context.Context, runtime.ServerBuildInput) (runtime.Server, error) {
	return noopServer{}, nil
}

type noopServer struct{}

func (noopServer) Start(context.Context) error { return nil }
func (noopServer) Stop(context.Context) error  { return nil }
func (noopServer) Endpoint() (*url.URL, error) { return nil, nil }
