package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

const defaultMinVersion = tls.VersionTLS12

// ServerConfigOptions 描述服务端 TLS 配置来源。
type ServerConfigOptions struct {
	CertPath   string
	KeyPath    string
	MinVersion uint16
}

// ClientConfigOptions 描述客户端 TLS 配置来源。
type ClientConfigOptions struct {
	CAPath             string
	CertPath           string
	KeyPath            string
	ServerName         string
	InsecureSkipVerify bool
	MinVersion         uint16
}

// NewServerConfig 构造服务端 TLS 配置。
func NewServerConfig(opts ServerConfigOptions) (*tls.Config, error) {
	if opts.CertPath == "" || opts.KeyPath == "" {
		return nil, fmt.Errorf("tls cert_path and key_path are required")
	}

	cert, err := tls.LoadX509KeyPair(opts.CertPath, opts.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("load x509 key pair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   normalizeMinVersion(opts.MinVersion),
	}, nil
}

// MustServerConfig 构造服务端 TLS 配置，失败时 panic。
func MustServerConfig(opts ServerConfigOptions) *tls.Config {
	cfg, err := NewServerConfig(opts)
	if err != nil {
		panic(err)
	}
	return cfg
}

// NewClientConfig 构造客户端 TLS 配置。
func NewClientConfig(opts ClientConfigOptions) (*tls.Config, error) {
	if (opts.CertPath == "") != (opts.KeyPath == "") {
		return nil, fmt.Errorf("tls cert_path and key_path must both be set for mTLS")
	}

	cfg := &tls.Config{
		InsecureSkipVerify: opts.InsecureSkipVerify, //nolint:gosec
		MinVersion:         normalizeMinVersion(opts.MinVersion),
		ServerName:         opts.ServerName,
	}

	if opts.CAPath != "" {
		rootCAs, err := LoadCertPoolFromPEMFile(opts.CAPath)
		if err != nil {
			return nil, err
		}
		cfg.RootCAs = rootCAs
	}

	if opts.CertPath != "" {
		cert, err := tls.LoadX509KeyPair(opts.CertPath, opts.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("load x509 key pair: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	return cfg, nil
}

// LoadCertPoolFromPEMFile 从 PEM 文件加载证书池。
func LoadCertPoolFromPEMFile(path string) (*x509.CertPool, error) {
	caPEM, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ca file: %w", err)
	}

	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("append ca file: invalid pem data")
	}

	return rootCAs, nil
}

func normalizeMinVersion(v uint16) uint16 {
	if v == 0 {
		return defaultMinVersion
	}
	return v
}
