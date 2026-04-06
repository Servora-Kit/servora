package config

import (
	"strings"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

// ListenConfig 是从 proto Server_Listen 解析后的结构化监听参数。
type ListenConfig struct {
	Network string
	Addr    string
	Timeout *durationpb.Duration // nil 表示未配置，由各 server 框架使用默认值
}

// ParseListenConfig 从 proto 配置解析监听参数。listen 为 nil 时返回零值。
func ParseListenConfig(listen *conf.Server_Listen) ListenConfig {
	if listen == nil {
		return ListenConfig{}
	}
	return ListenConfig{
		Network: strings.TrimSpace(listen.GetNetwork()),
		Addr:    strings.TrimSpace(listen.GetAddr()),
		Timeout: listen.GetTimeout(),
	}
}
