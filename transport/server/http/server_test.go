package http

import (
	"log/slog"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/encoding"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	corsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/cors/v1"
	"github.com/Servora-Kit/servora/transport/server/http/health"
)

func TestJSONCodecUsesProtoJSONForMessages(t *testing.T) {
	NewServer()
	codec := encoding.GetCodec("json")
	if codec == nil {
		t.Fatal("expected json codec to be registered")
	}

	const maxInt64 = int64(9223372036854775807)
	encoded, err := codec.Marshal(wrapperspb.Int64(maxInt64))
	if err != nil {
		t.Fatalf("marshal int64 wrapper: %v", err)
	}
	if got, want := string(encoded), `"9223372036854775807"`; got != want {
		t.Fatalf("expected ProtoJSON int64 string %s, got %s", want, got)
	}

	var decoded wrapperspb.Int64Value
	if err := codec.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal int64 wrapper: %v", err)
	}
	if decoded.Value != maxInt64 {
		t.Fatalf("expected %d, got %d", maxInt64, decoded.Value)
	}
}

func TestJSONCodecPreservesStandardJSONForNonMessages(t *testing.T) {
	NewServer()
	type payload struct {
		DisplayName string `json:"display_name"`
		Count       int    `json:"count"`
	}

	codec := encoding.GetCodec("json")
	if codec == nil {
		t.Fatal("expected json codec to be registered")
	}

	encoded, err := codec.Marshal(payload{DisplayName: "Ada", Count: 7})
	if err != nil {
		t.Fatalf("marshal ordinary JSON payload: %v", err)
	}
	if got, want := string(encoded), `{"display_name":"Ada","count":7}`; got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}

	var decoded payload
	if err := codec.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal ordinary JSON payload: %v", err)
	}
	if decoded.DisplayName != "Ada" || decoded.Count != 7 {
		t.Fatalf("unexpected decoded payload: %+v", decoded)
	}
}

func TestProtoJSONCodecRemainsUsable(t *testing.T) {
	codec := encoding.GetCodec("protojson")
	if codec == nil {
		t.Fatal("expected protojson codec to be registered")
	}

	const maxInt64 = int64(9223372036854775807)
	encoded, err := codec.Marshal(wrapperspb.Int64(maxInt64))
	if err != nil {
		t.Fatalf("marshal int64 wrapper: %v", err)
	}

	var decoded wrapperspb.Int64Value
	if err := codec.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal int64 wrapper: %v", err)
	}
	if decoded.Value != maxInt64 {
		t.Fatalf("expected %d, got %d", maxInt64, decoded.Value)
	}
}

func TestNewServer_NoOptions(t *testing.T) {
	srv := NewServer()
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestDefaultCodecsRegistered(t *testing.T) {
	NewServer()
	if encoding.GetCodec("json") == nil {
		t.Fatal("expected json codec to be registered")
	}
	if encoding.GetCodec("protojson") == nil {
		t.Fatal("expected protojson codec to be registered")
	}
}

func TestNewServer_WithConfig(t *testing.T) {
	cfg := &corev1.Server_HTTP{
		Listen: &corev1.Server_Listen{
			Network: "tcp4",
			Addr:    ":8080",
			Timeout: durationpb.New(30 * time.Second),
		},
	}
	srv := NewServer(WithConfig(cfg))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithNilConfig(t *testing.T) {
	srv := NewServer(WithConfig(nil))
	if srv == nil {
		t.Fatal("expected non-nil server with nil config")
	}
}

func TestNewServer_WithLogger(t *testing.T) {
	logger := slog.Default()
	srv := NewServer(WithLogger(logger))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithNilLogger(t *testing.T) {
	srv := NewServer(WithLogger(nil))
	if srv == nil {
		t.Fatal("expected non-nil server with nil logger")
	}
}

func TestNewServer_WithMiddleware(t *testing.T) {
	srv := NewServer(WithMiddleware(recovery.Recovery()))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithEmptyMiddleware(t *testing.T) {
	srv := NewServer(WithMiddleware())
	if srv == nil {
		t.Fatal("expected non-nil server with empty middleware")
	}
}

func TestNewServer_WithCORS(t *testing.T) {
	corsConf := &corsv1.CORS{
		Enable:         true,
		AllowedOrigins: []string{"*"},
	}
	srv := NewServer(WithCORS(corsConf))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithCORSDisabled(t *testing.T) {
	corsConf := &corsv1.CORS{Enable: false}
	srv := NewServer(WithCORS(corsConf))
	if srv == nil {
		t.Fatal("expected non-nil server with disabled CORS")
	}
}

func TestNewServer_WithNilCORS(t *testing.T) {
	srv := NewServer(WithCORS(nil))
	if srv == nil {
		t.Fatal("expected non-nil server with nil CORS")
	}
}

func TestNewServer_WithServices(t *testing.T) {
	called := false
	srv := NewServer(WithServices(func(s *khttp.Server) {
		called = true
		_ = s
	}))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if !called {
		t.Fatal("expected registrar to be called")
	}
}

func TestNewServer_WithMultipleServices(t *testing.T) {
	callCount := 0
	srv := NewServer(WithServices(
		func(s *khttp.Server) { callCount++ },
		func(s *khttp.Server) { callCount++ },
		func(s *khttp.Server) { callCount++ },
	))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if callCount != 3 {
		t.Fatalf("expected 3 registrars called, got %d", callCount)
	}
}

func TestNewServer_FullOptions(t *testing.T) {
	cfg := &corev1.Server_HTTP{
		Listen: &corev1.Server_Listen{
			Addr:    ":8080",
			Timeout: durationpb.New(10 * time.Second),
		},
	}
	corsConf := &corsv1.CORS{
		Enable:         true,
		AllowedOrigins: []string{"http://localhost"},
	}
	srv := NewServer(
		WithConfig(cfg),
		WithLogger(slog.Default()),
		WithMiddleware(recovery.Recovery()),
		WithCORS(corsConf),
	)
	if srv == nil {
		t.Fatal("expected non-nil server with full options")
	}
}

func TestNewServer_WithHealthCheck(t *testing.T) {
	h := health.NewHandler()
	srv := NewServer(WithHealthCheck(h))
	if srv == nil {
		t.Fatal("expected non-nil server with health check")
	}
}

func TestNewServer_WithNilHealthCheck(t *testing.T) {
	srv := NewServer(WithHealthCheck(nil))
	if srv == nil {
		t.Fatal("expected non-nil server with nil health check")
	}
}

func TestNewServer_WithAdvertiseHost_EndpointUsesAdvertiseHost(t *testing.T) {
	cfg := &corev1.Server_HTTP{
		Listen:    &corev1.Server_Listen{Addr: "0.0.0.0:0"},
		Advertise: &corev1.Server_Advertise{Host: "host.docker.internal"},
	}

	srv := NewServer(WithConfig(cfg))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}

	ep, err := srv.Endpoint()
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}
	if got, want := ep.Host, "host.docker.internal:0"; got != want {
		t.Fatalf("expected host %q, got %q", want, got)
	}
	if got, want := ep.Scheme, "http"; got != want {
		t.Fatalf("expected scheme %q, got %q", want, got)
	}
}

func TestNewServer_WithAdvertiseEndpoint_EndpointUsesExplicitValue(t *testing.T) {
	cfg := &corev1.Server_HTTP{
		Listen:    &corev1.Server_Listen{Addr: ":0"},
		Advertise: &corev1.Server_Advertise{Endpoint: "https://example.internal:18443?isSecure=true"},
	}

	srv := NewServer(WithConfig(cfg))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}

	ep, err := srv.Endpoint()
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}
	if got, want := ep.String(), "https://example.internal:18443?isSecure=true"; got != want {
		t.Fatalf("expected endpoint %q, got %q", want, got)
	}
}
