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
	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/logging"
	"github.com/Servora-Kit/servora/security/tlsutil"
)

// NewConnOptional opens a ClickHouse connection from the Data config.
//
// Return semantics:
//   - (nil, nil)  — ClickHouse is not configured (Data.ClickHouse absent or no addrs).
//   - (nil, err)  — configured but connection/ping failed; callers can fail-fast or degrade.
//   - (conn, nil) — connected successfully.
//
// The caller is responsible for closing the connection via conn.Close().
func NewConnOptional(ctx context.Context, cfg *conf.Data, l logger.Logger) (driver.Conn, error) {
	log := logger.For(l, "clickhouse/db/infra")

	if cfg == nil || cfg.Clickhouse == nil || len(cfg.Clickhouse.Addrs) == 0 {
		log.Info("ClickHouse not configured")
		return nil, nil
	}

	chCfg := cfg.Clickhouse

	dialTimeout := durationOrDefault(chCfg.DialTimeout, 10*time.Second, "dial_timeout", log)
	readTimeout := durationOrDefault(chCfg.ReadTimeout, 30*time.Second, "read_timeout", log)
	connMaxLifetime := durationOrDefault(chCfg.ConnMaxLifetime, 5*time.Minute, "conn_max_lifetime", log)

	maxOpenConns := 10
	if chCfg.MaxOpenConns > 0 {
		maxOpenConns = int(chCfg.MaxOpenConns)
	}
	maxIdleConns := 5
	if chCfg.MaxIdleConns > 0 {
		maxIdleConns = int(chCfg.MaxIdleConns)
	}

	opts := &clickhouse.Options{
		Addr: chCfg.Addrs,
		Auth: clickhouse.Auth{
			Database: chCfg.Database,
			Username: chCfg.Username,
			Password: chCfg.Password,
		},
		DialTimeout:      dialTimeout,
		ReadTimeout:      readTimeout,
		MaxOpenConns:     maxOpenConns,
		MaxIdleConns:     maxIdleConns,
		ConnMaxLifetime:  connMaxLifetime,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	}

	if chCfg.Tls {
		tlsCfg, err := tlsutil.NewClientConfig(tlsutil.ClientConfigOptions{
			InsecureSkipVerify: chCfg.TlsSkipVerify,
		})
		if err != nil {
			return nil, fmt.Errorf("build ClickHouse TLS config: %w", err)
		}
		opts.TLS = tlsCfg
	}

	applyCompression(opts, chCfg.Compress, log)

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

// durationOrDefault extracts a proto Duration, falling back to def when nil or <= 0.
func durationOrDefault(d interface{ AsDuration() time.Duration }, def time.Duration, name string, log *logger.Helper) time.Duration {
	if d == nil {
		return def
	}
	v := d.AsDuration()
	if v <= 0 {
		log.Warnf("%s=%v is non-positive, using default %v", name, v, def)
		return def
	}
	return v
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
