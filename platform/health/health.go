// Package health 提供组件化的健康探针能力。
//
// 服务可在 HTTP server 构建时按需引入 /healthz (liveness) 和 /readyz (readiness) 端点。
// Liveness 始终返回 200（进程存活即可），Readiness 执行所有注册的 Checker。
//
// 使用示例：
//
//	h := health.NewHandler(
//	    health.PingChecker("redis", redisClient),
//	    health.PingChecker("db", db),
//	)
//	srv := http.NewServer(http.WithHealthCheck(h))
package health

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const (
	// DefaultCheckTimeout 是 readiness checker 的默认执行超时时间。
	DefaultCheckTimeout = 3 * time.Second
)

// Checker 定义健康检查接口。
type Checker interface {
	// Name 返回检查项名称，用于响应体中的 key。
	Name() string
	// Check 执行健康检查。返回 nil 表示健康，返回 error 表示不健康。
	Check(ctx context.Context) error
}

// Pinger 定义 Ping 方法接口，兼容 redis.Client 和 sql.DB。
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler 管理健康探针端点。
type Handler struct {
	checkers     []Checker
	checkTimeout time.Duration
}

// NewHandler 创建健康探针 Handler。
// checkers 为可选参数，用于 readiness 检查。
func NewHandler(checkers ...Checker) *Handler {
	return &Handler{
		checkers:     checkers,
		checkTimeout: DefaultCheckTimeout,
	}
}

// LivenessHandler 返回 liveness 探针的 http.HandlerFunc。
// 始终返回 HTTP 200，不执行任何 checker。
func (h *Handler) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	}
}

// ReadinessHandler 返回 readiness 探针的 http.HandlerFunc。
// 执行所有注册的 checker，全部通过返回 200，任一失败返回 503。
func (h *Handler) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(h.checkers) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ready",
				"checks": map[string]string{},
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), h.checkTimeout)
		defer cancel()

		checks := make(map[string]string, len(h.checkers))
		allHealthy := true

		for _, checker := range h.checkers {
			if err := checker.Check(ctx); err != nil {
				checks[checker.Name()] = err.Error()
				allHealthy = false
			} else {
				checks[checker.Name()] = "ok"
			}
		}

		status := "ready"
		httpStatus := http.StatusOK
		if !allHealthy {
			status = "not_ready"
			httpStatus = http.StatusServiceUnavailable
		}

		writeJSON(w, httpStatus, map[string]any{
			"status": status,
			"checks": checks,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// pingChecker 通过 Pinger 接口实现 Checker。
type pingChecker struct {
	name   string
	pinger Pinger
}

// PingChecker 创建基于 Ping 方法的 Checker。
// 兼容 redis.Client、sql.DB 等实现了 Ping(ctx) error 的类型。
func PingChecker(name string, pinger Pinger) Checker {
	return &pingChecker{name: name, pinger: pinger}
}

func (c *pingChecker) Name() string {
	return c.name
}

func (c *pingChecker) Check(ctx context.Context) error {
	return c.pinger.Ping(ctx)
}
