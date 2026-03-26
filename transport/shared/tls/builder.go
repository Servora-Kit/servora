package tls

import (
	"crypto/tls"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/security/tlsutil"
)

func BuildServerTLS(c *conf.TLSConfig) (*tls.Config, error) {
	if c == nil || !c.GetEnable() {
		return nil, nil
	}
	return tlsutil.NewServerConfig(tlsutil.ServerConfigOptions{
		CertPath: c.GetCertPath(),
		KeyPath:  c.GetKeyPath(),
	})
}

func BuildClientTLS(c *conf.TLSConfig) (*tls.Config, error) {
	if c == nil || !c.GetEnable() {
		return nil, nil
	}
	return tlsutil.NewClientConfig(tlsutil.ClientConfigOptions{
		CAPath:   c.GetCaPath(),
		CertPath: c.GetCertPath(),
		KeyPath:  c.GetKeyPath(),
	})
}
