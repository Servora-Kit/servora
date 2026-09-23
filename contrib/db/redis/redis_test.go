package redis

import (
	"strings"
	"testing"
	"time"

	redispb "github.com/Servora-Kit/servora/api/gen/go/servora/contrib/db/redis/v1"
	tlspb "github.com/Servora-Kit/servora/api/gen/go/servora/security/tls/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestConfigFromProtoPlaintext(t *testing.T) {
	t.Parallel()

	cfg, err := configFromProto(&redispb.Redis{Network: "tcp4", Addr: "localhost:6379"})
	if err != nil {
		t.Fatalf("configFromProto() error = %v", err)
	}
	if cfg.TLSConfig != nil {
		t.Fatalf("TLSConfig = %#v, want nil", cfg.TLSConfig)
	}
	if cfg.DialTimeout != 5*time.Second || cfg.ReadTimeout != 3*time.Second || cfg.WriteTimeout != 3*time.Second {
		t.Fatalf("timeouts = (%v, %v, %v), want Proto defaults", cfg.DialTimeout, cfg.ReadTimeout, cfg.WriteTimeout)
	}
	if got := newRedisOptions(cfg).Network; got != "tcp4" {
		t.Fatalf("go-redis network = %q, want tcp4", got)
	}
}

func TestConfigFromProtoPreservesExplicitZeroTimeout(t *testing.T) {
	config, err := configFromProto(&redispb.Redis{
		Addr:         "localhost:6379",
		DialTimeout:  durationpb.New(0),
		ReadTimeout:  durationpb.New(0),
		WriteTimeout: durationpb.New(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	options := newRedisOptions(config)
	if options.DialTimeout != 0 || options.ReadTimeout != 0 || options.WriteTimeout != 0 {
		t.Fatalf("explicit zero was overwritten: dial=%v read=%v write=%v", options.DialTimeout, options.ReadTimeout, options.WriteTimeout)
	}
}

func TestConfigFromProtoTLSUsesAddrHost(t *testing.T) {
	t.Parallel()

	cfg, err := configFromProto(&redispb.Redis{
		Addr: "redis.internal:6380",
		Tls:  &tlspb.TLS{Enable: true},
	})
	if err != nil {
		t.Fatalf("configFromProto() error = %v", err)
	}
	if cfg.TLSConfig == nil {
		t.Fatal("TLSConfig = nil, want enabled TLS")
	}
	if cfg.TLSConfig.ServerName != "redis.internal" {
		t.Fatalf("ServerName = %q, want redis.internal", cfg.TLSConfig.ServerName)
	}
	if cfg.TLSConfig.InsecureSkipVerify {
		t.Fatal("TLSConfig unexpectedly disables certificate verification")
	}
	if got := newRedisOptions(cfg).TLSConfig; got != cfg.TLSConfig {
		t.Fatal("go-redis options did not receive the configured TLSConfig")
	}
}

func TestConfigFromProtoTLSRejectsAddrWithoutPort(t *testing.T) {
	t.Parallel()

	_, err := configFromProto(&redispb.Redis{
		Addr: "redis.internal",
		Tls:  &tlspb.TLS{Enable: true},
	})
	if err == nil || !strings.Contains(err.Error(), "host:port") {
		t.Fatalf("configFromProto() error = %v, want host:port error", err)
	}
}

func TestNewClientReportsConnectionFailure(t *testing.T) {
	t.Parallel()

	client, cleanup, err := newClient(&Config{
		Addr:         "127.0.0.1:1",
		DialTimeout:  20 * time.Millisecond,
		ReadTimeout:  20 * time.Millisecond,
		WriteTimeout: 20 * time.Millisecond,
	})
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatal("NewClient() error = nil, want connection failure")
	}
	if client != nil || cleanup != nil {
		t.Fatalf("NewClient() returned client=%t cleanup=%t, want both nil", client != nil, cleanup != nil)
	}
	if !strings.Contains(err.Error(), "redis ping") {
		t.Fatalf("NewClient() error = %v, want redis ping context", err)
	}
}
