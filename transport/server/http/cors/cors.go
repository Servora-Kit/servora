package cors

import (
	"fmt"
	"net/http"
	"strings"

	corsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/cors/v1"
)

// Middleware 创建 CORS 中间件。
// corsConfig 应已经过 corsv1.CORS.ApplyDefaults()（由业务方在
// bootstrap.Scan 时自动完成），本函数不再回填默认值。
// 当 corsConfig 为 nil 或 Enable=false 时返回透传中间件。
func Middleware(corsConfig *corsv1.CORS) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if corsConfig == nil || !corsConfig.GetEnable() {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			setCORSHeaders(w, corsConfig, origin)
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IsEnabled reports whether the supplied CORS configuration is active.
func IsEnabled(corsConfig *corsv1.CORS) bool {
	return corsConfig != nil && corsConfig.GetEnable() && len(corsConfig.GetAllowedOrigins()) > 0
}

// GetAllowedOrigins exposes the configured origin list for logging.
func GetAllowedOrigins(corsConfig *corsv1.CORS) []string {
	if corsConfig == nil {
		return nil
	}
	return corsConfig.GetAllowedOrigins()
}

func setCORSHeaders(w http.ResponseWriter, c *corsv1.CORS, origin string) {
	if isOriginAllowed(origin, c.GetAllowedOrigins()) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	if methods := c.GetAllowedMethods(); len(methods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))
	}
	if headers := c.GetAllowedHeaders(); len(headers) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(headers, ", "))
	}
	if exposed := c.GetExposedHeaders(); len(exposed) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(exposed, ", "))
	}
	if c.GetAllowCredentials() {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if d := c.GetMaxAge(); d != nil && d.AsDuration() > 0 {
		w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", int64(d.AsDuration().Seconds())))
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
		if strings.HasPrefix(allowed, "*.") {
			suffix := strings.TrimPrefix(allowed, "*.")
			if strings.HasSuffix(origin, suffix) {
				parts := strings.Split(strings.TrimSuffix(origin, suffix), ".")
				if len(parts) == 2 {
					return true
				}
			}
		}
	}
	return false
}
