package logger

import ("bytes";"context";"encoding/json";"log/slog";"testing")

func TestScope_TagsEntries(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))
	l := Scope(base, "kafka/broker/infra")
	l.InfoContext(context.Background(), "hi")
	var m map[string]any
	_ = json.Unmarshal(buf.Bytes(), &m)
	if m[ScopeKey] != "kafka/broker/infra" {
		t.Fatalf("scope = %v, want kafka/broker/infra", m[ScopeKey])
	}
}
