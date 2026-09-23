package cors

import (
	"fmt"
	"net/http"
	"strings"

	corsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/cors/v1"
)

var (
	defaultOrigins = []string{"*"}
	defaultMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	defaultHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
)

type options struct {
	origins     []string
	methods     string
	headers     string
	exposed     string
	credentials bool
	maxAge      string
}

func effectiveOrigins(config *corsv1.CORS) []string {
	if !IsEnabled(config) {
		return nil
	}
	if len(config.GetAllowedOrigins()) == 0 {
		return defaultOrigins
	}
	return config.GetAllowedOrigins()
}

// Middleware 创建 CORS 中间件；有效默认在构造时计算，不在请求中分配。
func Middleware(config *corsv1.CORS) func(http.Handler) http.Handler {
	if !IsEnabled(config) {
		return func(next http.Handler) http.Handler { return next }
	}
	methods := config.GetAllowedMethods()
	if len(methods) == 0 {
		methods = defaultMethods
	}
	headers := config.GetAllowedHeaders()
	if len(headers) == 0 {
		headers = defaultHeaders
	}
	origins := effectiveOrigins(config)
	if len(config.GetAllowedOrigins()) != 0 {
		origins = append([]string(nil), origins...)
	}
	policy := options{
		origins:     origins,
		methods:     strings.Join(methods, ", "),
		headers:     strings.Join(headers, ", "),
		exposed:     strings.Join(config.GetExposedHeaders(), ", "),
		credentials: config.GetAllowCredentials(),
	}
	if d := config.GetMaxAge(); d != nil && d.AsDuration() > 0 {
		policy.maxAge = fmt.Sprintf("%d", int64(d.AsDuration().Seconds()))
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setCORSHeaders(w, &policy, r.Header.Get("Origin"))
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IsEnabled 报告 CORS 是否由配置显式启用。
func IsEnabled(config *corsv1.CORS) bool {
	return config != nil && config.GetEnable()
}

// GetAllowedOrigins 返回与中间件相同的有效来源策略，供日志使用。
func GetAllowedOrigins(config *corsv1.CORS) []string {
	if !IsEnabled(config) {
		return nil
	}
	if len(config.GetAllowedOrigins()) == 0 {
		return append([]string(nil), defaultOrigins...)
	}
	return config.GetAllowedOrigins()
}

func setCORSHeaders(w http.ResponseWriter, policy *options, origin string) {
	if isOriginAllowed(origin, policy.origins) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Access-Control-Allow-Methods", policy.methods)
	w.Header().Set("Access-Control-Allow-Headers", policy.headers)
	if policy.exposed != "" {
		w.Header().Set("Access-Control-Expose-Headers", policy.exposed)
	}
	if policy.credentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if policy.maxAge != "" {
		w.Header().Set("Access-Control-Max-Age", policy.maxAge)
	}
}

func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}
	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
		if after, ok := strings.CutPrefix(allowed, "*."); ok {
			suffix := after
			if before, ok := strings.CutSuffix(origin, suffix); ok {
				if strings.Count(before, ".") == 1 {
					return true
				}
			}
		}
	}
	return false
}
