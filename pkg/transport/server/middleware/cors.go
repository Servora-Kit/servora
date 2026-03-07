package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/conf/v1"
)

// options 包含 CORS 中间件的内部配置选项
type options struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// defaultOptions 返回默认的 CORS 配置
func defaultOptions() options {
	return options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposedHeaders:   []string{},
		AllowCredentials: false,
		MaxAge:           24 * time.Hour,
	}
}

// parseConfig 将 conf.CORS 转换为内部 options，处理空字段和默认值
func parseConfig(corsConfig *conf.CORS) options {
	if corsConfig == nil || !corsConfig.GetEnable() {
		return options{} // 返回空配置表示禁用 CORS
	}

	opts := defaultOptions()

	if len(corsConfig.GetAllowedOrigins()) > 0 {
		opts.AllowedOrigins = corsConfig.GetAllowedOrigins()
	}
	if len(corsConfig.GetAllowedMethods()) > 0 {
		opts.AllowedMethods = corsConfig.GetAllowedMethods()
	}
	if len(corsConfig.GetAllowedHeaders()) > 0 {
		opts.AllowedHeaders = corsConfig.GetAllowedHeaders()
	}
	if len(corsConfig.GetExposedHeaders()) > 0 {
		opts.ExposedHeaders = corsConfig.GetExposedHeaders()
	}

	opts.AllowCredentials = corsConfig.GetAllowCredentials()

	if corsConfig.MaxAge != nil {
		opts.MaxAge = corsConfig.MaxAge.AsDuration()
	}

	return opts
}

// Middleware 创建 CORS 中间件，直接接受 conf.CORS 配置
// 如果 corsConfig 为 nil 或 Enable 为 false，返回透传中间件
func Middleware(corsConfig *conf.CORS) func(http.Handler) http.Handler {
	opts := parseConfig(corsConfig)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 如果 CORS 禁用（AllowedOrigins 为空），直接透传
			if len(opts.AllowedOrigins) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			origin := r.Header.Get("Origin")
			// 设置响应头
			setCORSHeaders(w, opts, origin)
			// 处理预检请求
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IsEnabled 检查 CORS 配置是否启用（用于日志等场景）
func IsEnabled(corsConfig *conf.CORS) bool {
	if corsConfig == nil || !corsConfig.GetEnable() {
		return false
	}
	opts := parseConfig(corsConfig)
	return len(opts.AllowedOrigins) > 0
}

// GetAllowedOrigins 获取配置的允许源列表（用于日志等场景）
func GetAllowedOrigins(corsConfig *conf.CORS) []string {
	opts := parseConfig(corsConfig)
	return opts.AllowedOrigins
}

// setCORSHeaders 设置 CORS 响应头
func setCORSHeaders(w http.ResponseWriter, opts options, origin string) {
	// 检查源是否被允许
	allowedOrigin := ""
	if isOriginAllowed(origin, opts.AllowedOrigins) {
		allowedOrigin = origin
	}
	if allowedOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
	}
	// 设置允许的方法
	if len(opts.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(opts.AllowedMethods, ", "))
	}
	// 设置允许的头部
	if len(opts.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(opts.AllowedHeaders, ", "))
	}
	// 设置暴露的头部
	if len(opts.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(opts.ExposedHeaders, ", "))
	}
	// 设置是否允许凭证
	if opts.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	// 设置预检请求缓存时间
	if opts.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", int64(opts.MaxAge.Seconds())))
	}
}

// isOriginAllowed 检查源是否被允许
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}

	for _, allowedOrigin := range allowedOrigins {
		if allowedOrigin == "*" {
			return true
		}
		if allowedOrigin == origin {
			return true
		}
		// 支持通配符匹配，如 *.example.com
		if strings.HasPrefix(allowedOrigin, "*.") {
			suffix := strings.TrimPrefix(allowedOrigin, "*.")
			if strings.HasSuffix(origin, suffix) {
				// 检查是否只有一个点，防止匹配过长
				parts := strings.Split(strings.TrimSuffix(origin, suffix), ".")
				if len(parts) == 2 {
					return true
				}
			}
		}
	}

	return false
}
