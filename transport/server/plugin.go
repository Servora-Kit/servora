package server

import (
	"fmt"

	"github.com/Servora-Kit/servora/transport/runtime"
)

// RegisterPlugins 批量注册 server plugin。
func RegisterPlugins(r *runtime.Registry, plugins ...runtime.ServerPlugin) error {
	if r == nil {
		return fmt.Errorf("runtime registry is nil")
	}
	for _, p := range plugins {
		if p == nil {
			continue
		}
		if err := r.RegisterServer(p); err != nil {
			return fmt.Errorf("register server plugin %q: %w", p.Type(), err)
		}
	}
	return nil
}
