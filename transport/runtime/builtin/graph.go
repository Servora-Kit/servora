package builtin

import "github.com/Servora-Kit/servora/transport/runtime"

// NewGraph 创建挂载内建 transport 插件的 runtime graph。
func NewGraph() (*runtime.Graph, error) {
	r, err := NewRegistry()
	if err != nil {
		return nil, err
	}
	return runtime.NewGraph(r), nil
}
