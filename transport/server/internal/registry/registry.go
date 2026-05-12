package registry

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

type RegistryEndpointInput struct {
	Scheme   string
	BindAddr string
	Endpoint string
	Host     string
	Query    url.Values
}

// ResolveRegistryEndpoint 组装服务注册端点 URL。
//
// scheme 由调用方负责完整提供（含 TLS 升级，如 "grpcs"、"https"）。
// query 会合并到结果 URL 的查询参数中，传 nil 表示无额外参数。
// host 为空时返回 nil（不向注册中心注册端点）。
func ResolveRegistryEndpoint(in RegistryEndpointInput) (*url.URL, error) {
	if raw := strings.TrimSpace(in.Endpoint); raw != "" {
		ep, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("parse registry endpoint: %w", err)
		}
		if ep.Host == "" {
			return nil, fmt.Errorf("parse registry endpoint: missing host")
		}
		return ep, nil
	}

	host := strings.TrimSpace(in.Host)
	if host == "" {
		return nil, nil
	}

	_, port, err := net.SplitHostPort(strings.TrimSpace(in.BindAddr))
	if err != nil || port == "" {
		return nil, fmt.Errorf("parse registry bind addr: %w", err)
	}

	return &url.URL{
		Scheme:   strings.TrimSpace(in.Scheme),
		Host:     net.JoinHostPort(host, port),
		RawQuery: in.Query.Encode(),
	}, nil
}
