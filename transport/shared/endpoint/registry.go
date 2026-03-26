package endpoint

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ResolveRegistryEndpoint 统一解析服务注册端点，优先 endpoint，其次 host+bindAddr 组装。
func ResolveRegistryEndpoint(scheme, bindAddr, endpoint, host string, secure bool) (*url.URL, error) {
	if raw := strings.TrimSpace(endpoint); raw != "" {
		ep, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("parse registry endpoint: %w", err)
		}
		if ep.Host == "" {
			return nil, fmt.Errorf("parse registry endpoint: missing host")
		}
		return ep, nil
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return nil, nil
	}

	_, port, err := net.SplitHostPort(strings.TrimSpace(bindAddr))
	if err != nil || port == "" {
		return nil, fmt.Errorf("parse registry bind addr: %w", err)
	}

	scheme = normalizeScheme(scheme, secure)
	q := url.Values{}
	q.Set("isSecure", strconv.FormatBool(secure))

	return &url.URL{
		Scheme:   scheme,
		Host:     net.JoinHostPort(host, port),
		RawQuery: q.Encode(),
	}, nil
}

func normalizeScheme(scheme string, secure bool) string {
	scheme = strings.TrimSpace(strings.ToLower(scheme))
	if scheme == "" {
		if secure {
			return "https"
		}
		return "http"
	}
	if !secure {
		return scheme
	}
	switch scheme {
	case "grpc":
		return "grpcs"
	case "http":
		return "https"
	default:
		return scheme
	}
}
