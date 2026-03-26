package runtime

import (
	"context"
	"fmt"
	"strings"
)

// ServerNode 描述一个待构建的 server 节点。
type ServerNode struct {
	Type  string
	Input ServerBuildInput
}

// ClientNode 描述一个待构建的 client 节点。
type ClientNode struct {
	Type  string
	Input ClientBuildInput
}

// GraphInput 为 runtime graph 的统一输入。
type GraphInput struct {
	Servers []ServerNode
	Clients []ClientNode
}

// GraphOutput 为 runtime graph 的统一输出。
type GraphOutput struct {
	Servers map[string]Server
	Clients map[string]ClientFactory
}

// Graph 基于 registry 进行 transport 运行时编排。
type Graph struct {
	registry *Registry
}

func NewGraph(r *Registry) *Graph {
	if r == nil {
		r = NewRegistry()
	}
	return &Graph{registry: r}
}

func (g *Graph) Build(ctx context.Context, in GraphInput) (GraphOutput, error) {
	out := GraphOutput{
		Servers: make(map[string]Server, len(in.Servers)),
		Clients: make(map[string]ClientFactory, len(in.Clients)),
	}

	for _, node := range in.Servers {
		typ := strings.TrimSpace(node.Type)
		plugin, ok := g.registry.Server(typ)
		if !ok {
			return GraphOutput{}, fmt.Errorf("%w: server type=%s", ErrPluginNotFound, typ)
		}
		srv, err := plugin.Build(ctx, node.Input)
		if err != nil {
			return GraphOutput{}, fmt.Errorf("build server plugin %q: %w", typ, err)
		}
		out.Servers[typ] = srv
	}

	for _, node := range in.Clients {
		typ := strings.TrimSpace(node.Type)
		plugin, ok := g.registry.Client(typ)
		if !ok {
			return GraphOutput{}, fmt.Errorf("%w: client type=%s", ErrPluginNotFound, typ)
		}
		factory, err := plugin.Build(ctx, node.Input)
		if err != nil {
			return GraphOutput{}, fmt.Errorf("build client plugin %q: %w", typ, err)
		}
		out.Clients[typ] = factory
	}

	return out, nil
}
