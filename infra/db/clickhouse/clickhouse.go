// Package clickhouse provides a framework-level ClickHouse connection helper.
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
	"log/slog"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	clickhousepb "github.com/Servora-Kit/servora/api/gen/go/servora/infra/db/clickhouse/v1"
	svrtls "github.com/Servora-Kit/servora/security/tls"
)

// NewConnOptional opens a ClickHouse connection from generated infra config.
//
// Return semantics:
//   - (nil, nil)  — ClickHouse is not configured (cfg nil or no addrs).
//   - (nil, err)  — configured but connection/ping failed; callers can fail-fast or degrade.
//   - (conn, nil) — connected successfully.
//
// The caller is responsible for closing the connection via conn.Close().
func NewConnOptional(ctx context.Context, cfg *clickhousepb.ClickHouse, l *slog.Logger) (driver.Conn, error) {
	log := loggerOrDefault(l).With("scope", "clickhouse/db/infra")

	if cfg == nil || len(cfg.GetAddrs()) == 0 {
		log.Info("ClickHouse not configured")
		return nil, nil
	}
	if err := cfg.ApplyConf(); err != nil {
		return nil, err
	}

	opts := &clickhouse.Options{
		Addr: cfg.GetAddrs(),
		Auth: clickhouse.Auth{
			Database: cfg.GetDatabase(),
			Username: cfg.GetUsername(),
			Password: cfg.GetPassword(),
		},
		DialTimeout:      cfg.GetDialTimeout().AsDuration(),
		ReadTimeout:      cfg.GetReadTimeout().AsDuration(),
		MaxOpenConns:     int(cfg.GetMaxOpenConns()),
		MaxIdleConns:     int(cfg.GetMaxIdleConns()),
		ConnMaxLifetime:  cfg.GetConnMaxLifetime().AsDuration(),
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	}

	if cfg.GetTls().GetEnable() {
		tlsCfg, err := svrtls.NewClientConfig(svrtls.ClientConfigOptions{
			CAPath:   cfg.GetTls().GetCaPath(),
			CertPath: cfg.GetTls().GetCertPath(),
			KeyPath:  cfg.GetTls().GetKeyPath(),
		})
		if err != nil {
			return nil, fmt.Errorf("build ClickHouse TLS config: %w", err)
		}
		opts.TLS = tlsCfg
	}

	applyCompression(opts, cfg.GetCompression(), log)

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open ClickHouse: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.GetDialTimeout().AsDuration())
	defer cancel()
	if err := conn.Ping(pingCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping ClickHouse: %w", err)
	}

	log.Info("ClickHouse connected")
	return conn, nil
}

// applyCompression normalises the compress string and sets the appropriate
// compression option. Warns on unrecognised values.
func applyCompression(opts *clickhouse.Options, raw string, log *slog.Logger) {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "", "none":
		// no compression
	case "lz4":
		opts.Compression = &clickhouse.Compression{Method: clickhouse.CompressionLZ4}
	case "zstd":
		opts.Compression = &clickhouse.Compression{Method: clickhouse.CompressionZSTD}
	default:
		log.Warn("unknown compress value, falling back to no compression", "value", raw)
	}
}

func loggerOrDefault(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.Default()
}
