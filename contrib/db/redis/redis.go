package redis

import (
	"context"
	stdtls "crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	redispb "github.com/Servora-Kit/servora/api/gen/go/servora/contrib/db/redis/v1"
	svrtls "github.com/Servora-Kit/servora/security/tls"
	goredis "github.com/redis/go-redis/v9"
)

type Config struct {
	Network      string
	Addr         string
	Username     string
	Password     string
	DB           int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	TLSConfig    *stdtls.Config
}

func configFromProto(cfg *redispb.Redis) (*Config, error) {
	if cfg == nil {
		return nil, nil
	}
	if err := cfg.Apply(); err != nil {
		return nil, fmt.Errorf("redis: config: %w", err)
	}

	config := &Config{
		Addr:         cfg.GetAddr(),
		Network:      cfg.GetNetwork(),
		Username:     cfg.GetUserName(),
		Password:     cfg.GetPassword(),
		DB:           int(cfg.GetDb()),
		DialTimeout:  cfg.GetDialTimeout().AsDuration(),
		ReadTimeout:  cfg.GetReadTimeout().AsDuration(),
		WriteTimeout: cfg.GetWriteTimeout().AsDuration(),
	}

	var serverName string
	if cfg.GetTls().GetEnable() {
		host, _, err := net.SplitHostPort(cfg.GetAddr())
		if err != nil || host == "" {
			return nil, fmt.Errorf("redis TLS requires addr in host:port form: %q", cfg.GetAddr())
		}
		serverName = host
	}
	tlsConfig, err := svrtls.BuildClientTLSForServer(cfg.GetTls(), serverName)
	if err != nil {
		return nil, fmt.Errorf("build redis TLS config: %w", err)
	}
	config.TLSConfig = tlsConfig
	return config, nil
}

// New creates a Redis client from the shared Redis configuration, verifies the connection, and returns cleanup.
func New(cfg *redispb.Redis) (*goredis.Client, func(), error) {
	config, err := configFromProto(cfg)
	if err != nil {
		return nil, nil, err
	}
	return newClient(config)
}

func newClient(cfg *Config) (*goredis.Client, func(), error) {
	if cfg == nil {
		return nil, nil, errors.New("redis config is nil")
	}

	rdb := goredis.NewClient(newRedisOptions(cfg))

	ctx, cancel := context.WithTimeout(context.Background(), rdb.Options().DialTimeout)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, nil, fmt.Errorf("redis ping: %w", err)
	}

	cleanup := func() {
		_ = rdb.Close()
	}

	return rdb, cleanup, nil
}

func newRedisOptions(cfg *Config) *goredis.Options {
	return &goredis.Options{
		Network:      cfg.Network,
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		TLSConfig:    cfg.TLSConfig,
	}
}
