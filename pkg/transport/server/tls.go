package server

import (
	"crypto/tls"
	"fmt"

	conf "github.com/Servora-Kit/servora/api/gen/go/conf/v1"
)

// MustLoadTLS 从配置加载 TLS 证书。
// 如果加载失败会 panic，因为 TLS 配置错误是严重的启动时错误。
func MustLoadTLS(c *conf.TLSConfig) *tls.Config {
	if c == nil || !c.Enable {
		return nil
	}
	if c.CertPath == "" || c.KeyPath == "" {
		panic("TLS enabled but cert_path or key_path is empty")
	}
	cert, err := tls.LoadX509KeyPair(c.CertPath, c.KeyPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load TLS certificate: %v", err))
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}
