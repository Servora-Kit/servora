// Package clickhouse provides a framework-level ClickHouse connection helper
// following the Optional-init pattern established by pkg/broker/kafka.
//
// Usage:
//
//	conn, err := clickhouse.NewConnOptional(ctx, cfg, logger)
//	if err != nil {
//	    // configured but failed to connect — fail-fast or degrade
//	}
//	if conn == nil {
//	    // not configured — handle gracefully
//	}
//	defer conn.Close()
package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Servora-Kit/servora/obs/logging"
	svrtls "github.com/Servora-Kit/servora/security/tls"
)

// Config carries the minimum knobs NewConnOptional needs to open a ClickHouse
// connection. It is defined locally (not borrowed from any proto) because
// ClickHouse-specific configuration lives in consumer-side proto (e.g.
// servora-platform/audit) and the framework no longer owns those fields.
type Config struct {
	Addrs           []string
	Database        string
	Username        string
	Password        string
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	TLS             bool
	TLSSkipVerify   bool
	Compress        string // "" | "none" | "lz4" | "zstd"
}

// NewConnOptional opens a ClickHouse connection from the supplied Config.
//
// Return semantics:
//   - (nil, nil)  — ClickHouse is not configured (cfg nil or no addrs).
//   - (nil, err)  — configured but connection/ping failed; callers can fail-fast or degrade.
//   - (conn, nil) — connected successfully.
//
// The caller is responsible for closing the connection via conn.Close().
func NewConnOptional(ctx context.Context, cfg *Config, l logger.Logger) (driver.Conn, error) {
	log := logger.For(l, "clickhouse/db/infra")

	if cfg == nil || len(cfg.Addrs) == 0 {
		log.Info("ClickHouse not configured")
		return nil, nil
	}

	dialTimeout := durationOrDefault(cfg.DialTimeout, 10*time.Second, "dial_timeout", log)
	readTimeout := durationOrDefault(cfg.ReadTimeout, 30*time.Second, "read_timeout", log)
	connMaxLifetime := durationOrDefault(cfg.ConnMaxLifetime, 5*time.Minute, "conn_max_lifetime", log)

	maxOpenConns := 10
	if cfg.MaxOpenConns > 0 {
		maxOpenConns = cfg.MaxOpenConns
	}
	maxIdleConns := 5
	if cfg.MaxIdleConns > 0 {
		maxIdleConns = cfg.MaxIdleConns
	}

	opts := &clickhouse.Options{
		Addr: cfg.Addrs,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout:      dialTimeout,
		ReadTimeout:      readTimeout,
		MaxOpenConns:     maxOpenConns,
		MaxIdleConns:     maxIdleConns,
		ConnMaxLifetime:  connMaxLifetime,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	}

	if cfg.TLS {
		tlsCfg, err := svrtls.NewClientConfig(svrtls.ClientConfigOptions{
			InsecureSkipVerify: cfg.TLSSkipVerify,
		})
		if err != nil {
			return nil, fmt.Errorf("build ClickHouse TLS config: %w", err)
		}
		opts.TLS = tlsCfg
	}

	applyCompression(opts, cfg.Compress, log)

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open ClickHouse: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	if err := conn.Ping(pingCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping ClickHouse: %w", err)
	}

	log.Info("ClickHouse connected")
	return conn, nil
}

// durationOrDefault returns d when positive, otherwise def (with a warn log).
func durationOrDefault(d time.Duration, def time.Duration, name string, log *logger.Helper) time.Duration {
	if d <= 0 {
		if d < 0 {
			log.Warnf("%s=%v is non-positive, using default %v", name, d, def)
		}
		return def
	}
	return d
}

// applyCompression normalises the compress string and sets the appropriate
// compression option. Warns on unrecognised values.
func applyCompression(opts *clickhouse.Options, raw string, log *logger.Helper) {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "", "none":
		// no compression
	case "lz4":
		opts.Compression = &clickhouse.Compression{Method: clickhouse.CompressionLZ4}
	case "zstd":
		opts.Compression = &clickhouse.Compression{Method: clickhouse.CompressionZSTD}
	default:
		log.Warnf("unknown compress value %q, falling back to no compression (valid: lz4, zstd, none)", raw)
	}
}
