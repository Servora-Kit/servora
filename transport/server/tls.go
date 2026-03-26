package server

import (
	"crypto/tls"
	"fmt"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	sharedtls "github.com/Servora-Kit/servora/transport/shared/tls"
)

// MustLoadTLS 从配置加载 TLS 证书。
// 如果加载失败会 panic，因为 TLS 配置错误是严重的启动时错误。
func MustLoadTLS(c *conf.TLSConfig) *tls.Config {
	tlsCfg, err := sharedtls.BuildServerTLS(c)
	if err != nil {
		panic(fmt.Sprintf("failed to load server TLS config: %v", err))
	}
	return tlsCfg
}
