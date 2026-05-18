package kratosv2

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	kratoslog "github.com/go-kratos/kratos/v2/log"
)

func TestAdapter_LevelAndKV(t *testing.T) {
	var buf bytes.Buffer
	l := Logger(slog.NewJSONHandler(&buf, nil))
	_ = l.Log(kratoslog.LevelError, "msg", "boom", "code", 500)
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["level"] != "ERROR" {
		t.Fatalf("level=%v", m["level"])
	}
	if m["msg"] != "boom" || m["code"].(float64) != 500 {
		t.Fatalf("kv lost: %v", m)
	}
}
