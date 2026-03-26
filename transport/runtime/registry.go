package runtime

import (
	"fmt"
	"strings"
	"sync"
)

// Registry 维护 client/server 插件注册表。
type Registry struct {
	mu      sync.RWMutex
	servers map[string]ServerPlugin
	clients map[string]ClientPlugin
}

func NewRegistry() *Registry {
	return &Registry{
		servers: make(map[string]ServerPlugin),
		clients: make(map[string]ClientPlugin),
	}
}

func (r *Registry) RegisterServer(p ServerPlugin) error {
	if p == nil {
		return ErrPluginNil
	}
	typ := strings.TrimSpace(p.Type())
	if typ == "" {
		return ErrPluginTypeEmpty
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.servers[typ]; exists {
		return fmt.Errorf("%w: server type=%s", ErrPluginAlreadyRegistered, typ)
	}
	r.servers[typ] = p
	return nil
}

func (r *Registry) RegisterClient(p ClientPlugin) error {
	if p == nil {
		return ErrPluginNil
	}
	typ := strings.TrimSpace(p.Type())
	if typ == "" {
		return ErrPluginTypeEmpty
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.clients[typ]; exists {
		return fmt.Errorf("%w: client type=%s", ErrPluginAlreadyRegistered, typ)
	}
	r.clients[typ] = p
	return nil
}

func (r *Registry) Server(typ string) (ServerPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.servers[strings.TrimSpace(typ)]
	return p, ok
}

func (r *Registry) Client(typ string) (ClientPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.clients[strings.TrimSpace(typ)]
	return p, ok
}
