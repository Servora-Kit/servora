package http

import (
	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/telemetry"
	"github.com/Servora-Kit/servora/platform/health"
	"github.com/Servora-Kit/servora/platform/swagger"
)

// ServerConfig 封装 HTTP server plugin 的完整配置，含 proto 配置和运行时注入项。
// 通过 Builder 构造后作为 runtime.ServerBuildInput.Config 传入 Plugin.Build。
type ServerConfig struct {
	HTTP           *conf.Server_HTTP
	CORS           *conf.CORS
	Metrics        *telemetry.Metrics
	HealthHandler  *health.Handler
	SwaggerSpec    []byte
	SwaggerOptions []swagger.Option
}
