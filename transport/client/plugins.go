package client

import (
	"fmt"

	"github.com/Servora-Kit/servora/transport/runtime"
)

// RegisterPlugins 批量注册 client plugin。
func RegisterPlugins(r *runtime.Registry, plugins ...runtime.ClientPlugin) error {
	if r == nil {
		return fmt.Errorf("runtime registry is nil")
	}
	for _, p := range plugins {
		if p == nil {
			continue
		}
		if err := r.RegisterClient(p); err != nil {
			return fmt.Errorf("register client plugin %q: %w", p.Type(), err)
		}
	}
	return nil
}
