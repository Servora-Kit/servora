package tls

import (
	stdtls "crypto/tls"
	"fmt"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
)

// BuildServerTLS 从 conf.TLSConfig 构造服务端 *tls.Config。
// 当 c 为 nil 或 enable=false 时返回 (nil, nil)，调用方据此决定是否启用 TLS。
func BuildServerTLS(c *conf.TLSConfig) (*stdtls.Config, error) {
	if c == nil || !c.GetEnable() {
		return nil, nil
	}
	return NewServerConfig(ServerConfigOptions{
		CertPath: c.GetCertPath(),
		KeyPath:  c.GetKeyPath(),
	})
}

// BuildClientTLS 从 conf.TLSConfig 构造客户端 *tls.Config。
// 当 c 为 nil 或 enable=false 时返回 (nil, nil)，调用方据此决定是否启用 TLS。
func BuildClientTLS(c *conf.TLSConfig) (*stdtls.Config, error) {
	if c == nil || !c.GetEnable() {
		return nil, nil
	}
	return NewClientConfig(ClientConfigOptions{
		CAPath:   c.GetCaPath(),
		CertPath: c.GetCertPath(),
		KeyPath:  c.GetKeyPath(),
	})
}

// MustBuildServerTLS 是 BuildServerTLS 的 panic 版本，TLS 配置非法时直接 panic。
func MustBuildServerTLS(c *conf.TLSConfig) *stdtls.Config {
	tlsCfg, err := BuildServerTLS(c)
	if err != nil {
		panic(fmt.Sprintf("build server TLS config: %v", err))
	}
	return tlsCfg
}
