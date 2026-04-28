package logger_test

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
)

// captureLogger records each Log() invocation as a flat key→value map so
// the example can inspect what valuers actually emitted.
type captureLogger struct {
	captured []map[string]any
}

func (c *captureLogger) Log(_ log.Level, keyvals ...any) error {
	m := make(map[string]any, len(keyvals)/2)
	for i := 0; i+1 < len(keyvals); i += 2 {
		m[fmt.Sprint(keyvals[i])] = keyvals[i+1]
	}
	c.captured = append(c.captured, m)
	return nil
}

// ExampleHelper_WithContext demonstrates the only correct way to make a
// kratos Valuer pull values out of the request context. Without
// WithContext(ctx), the valuer is invoked with context.Background() and
// yields the zero value — which is exactly what causes empty trace_id /
// span_id fields when business code calls helper.Warnf(...) directly.
func ExampleHelper_WithContext() {
	type ctxKey string
	const userIDKey ctxKey = "user_id"

	userIDValuer := log.Valuer(func(ctx context.Context) any {
		if v, ok := ctx.Value(userIDKey).(string); ok {
			return v
		}
		return ""
	})

	capture := &captureLogger{}
	base := log.With(capture, "user_id", userIDValuer)
	helper := log.NewHelper(base)

	// Wrong: no WithContext → valuer sees context.Background() → empty.
	helper.Info("call without ctx")

	// Right: WithContext(ctx) → valuer sees the request ctx → resolved.
	ctx := context.WithValue(context.Background(), userIDKey, "alice")
	helper.WithContext(ctx).Info("call with ctx")

	fmt.Printf("without ctx: user_id=%q\n", capture.captured[0]["user_id"])
	fmt.Printf("with ctx:    user_id=%q\n", capture.captured[1]["user_id"])
	// Output:
	// without ctx: user_id=""
	// with ctx:    user_id="alice"
}
