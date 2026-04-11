package grpc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/protobuf/types/known/durationpb"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
)

func TestNewServer_NoOptions(t *testing.T) {
	srv := NewServer()
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithConfig(t *testing.T) {
	cfg := &conf.Server_GRPC{
		Listen: &conf.Server_Listen{
			Network: "tcp4",
			Addr:    ":9000",
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
	logger := log.DefaultLogger
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

func TestNewServer_WithServices(t *testing.T) {
	called := false
	srv := NewServer(WithServices(func(s *kgrpc.Server) {
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
		func(s *kgrpc.Server) { callCount++ },
		func(s *kgrpc.Server) { callCount++ },
		func(s *kgrpc.Server) { callCount++ },
	))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if callCount != 3 {
		t.Fatalf("expected 3 registrars called, got %d", callCount)
	}
}

func TestNewServer_FullOptions(t *testing.T) {
	cfg := &conf.Server_GRPC{
		Listen: &conf.Server_Listen{
			Addr:    ":9000",
			Timeout: durationpb.New(10 * time.Second),
		},
	}
	srv := NewServer(
		WithConfig(cfg),
		WithLogger(log.DefaultLogger),
		WithMiddleware(recovery.Recovery()),
	)
	if srv == nil {
		t.Fatal("expected non-nil server with full options")
	}
}

func TestNewServer_WithTLSConfig_EndpointUsesGRPCS(t *testing.T) {
	tmp := t.TempDir()
	certPath, keyPath := writeSelfSignedPair(t, tmp)

	cfg := &conf.Server_GRPC{
		Listen: &conf.Server_Listen{Addr: ":0"},
		Tls: &conf.TLSConfig{
			Enable:   true,
			CertPath: certPath,
			KeyPath:  keyPath,
		},
	}

	srv := NewServer(WithConfig(cfg))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}

	ep, err := srv.Endpoint()
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}
	if ep.Scheme != "grpcs" {
		t.Fatalf("expected endpoint scheme grpcs, got %q", ep.Scheme)
	}
}

func TestNewServer_WithRegistryHost_EndpointUsesRegistryHost(t *testing.T) {
	cfg := &conf.Server_GRPC{
		Listen:   &conf.Server_Listen{Addr: "0.0.0.0:0"},
		Registry: &conf.Server_Registry{Host: "host.docker.internal"},
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
	if got, want := ep.Scheme, "grpc"; got != want {
		t.Fatalf("expected scheme %q, got %q", want, got)
	}
	if got, want := ep.Query().Get("isSecure"), "false"; got != want {
		t.Fatalf("expected isSecure=%q, got %q", want, got)
	}
}

func TestNewServer_WithRegistryEndpoint_EndpointUsesExplicitValue(t *testing.T) {
	cfg := &conf.Server_GRPC{
		Listen:   &conf.Server_Listen{Addr: ":0"},
		Registry: &conf.Server_Registry{Endpoint: "grpc://example.internal:18011?isSecure=false"},
	}

	srv := NewServer(WithConfig(cfg))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}

	ep, err := srv.Endpoint()
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}
	if got, want := ep.String(), "grpc://example.internal:18011?isSecure=false"; got != want {
		t.Fatalf("expected endpoint %q, got %q", want, got)
	}
}

func writeSelfSignedPair(t *testing.T, dir string) (string, string) {
	t.Helper()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
	}

	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "servora.test",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"servora.test"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &privKey.PublicKey, privKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	certPath := filepath.Join(dir, "server-cert.pem")
	keyPath := filepath.Join(dir, "server-key.pem")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	return certPath, keyPath
}
