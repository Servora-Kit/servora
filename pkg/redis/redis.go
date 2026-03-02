package redis

import (
	"context"
	"errors"
	"time"

	conf "github.com/horonlee/servora/api/gen/go/conf/v1"
	"github.com/horonlee/servora/pkg/logger"
	"github.com/redis/go-redis/v9"
)

const (
	DefaultDialTimeout  = 5 * time.Second
	DefaultReadTimeout  = 3 * time.Second
	DefaultWriteTimeout = 3 * time.Second
)

type Client struct {
	rdb *redis.Client
	log *logger.Helper
}

type Config struct {
	Addr         string
	Username     string
	Password     string
	DB           int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func NewConfigFromProto(cfg *conf.Data_Redis) *Config {
	if cfg == nil {
		return nil
	}

	config := &Config{
		Addr:     cfg.GetAddr(),
		Username: cfg.GetUserName(),
		Password: cfg.GetPassword(),
		DB:       int(cfg.GetDb()),
	}

	if cfg.GetDialTimeout() != nil {
		config.DialTimeout = cfg.GetDialTimeout().AsDuration()
	} else {
		config.DialTimeout = DefaultDialTimeout
	}

	if cfg.GetReadTimeout() != nil {
		config.ReadTimeout = cfg.GetReadTimeout().AsDuration()
	} else {
		config.ReadTimeout = DefaultReadTimeout
	}

	if cfg.GetWriteTimeout() != nil {
		config.WriteTimeout = cfg.GetWriteTimeout().AsDuration()
	} else {
		config.WriteTimeout = DefaultWriteTimeout
	}

	return config
}

func NewClient(cfg *Config, l logger.Logger) (*Client, func(), error) {
	if cfg == nil {
		return nil, nil, errors.New("redis config is nil")
	}

	dialTimeout := cfg.DialTimeout
	if dialTimeout == 0 {
		dialTimeout = DefaultDialTimeout
	}
	readTimeout := cfg.ReadTimeout
	if readTimeout == 0 {
		readTimeout = DefaultReadTimeout
	}
	writeTimeout := cfg.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = DefaultWriteTimeout
	}

	baseLogger := logger.With(l, logger.WithModule("redis/pkg"))
	setupLog := logger.NewHelper(baseLogger, logger.WithField("operation", "NewClient"))
	cleanupLog := logger.NewHelper(baseLogger, logger.WithField("operation", "cleanup"))

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})

	// 使用带超时的 context 进行连接测试
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		setupLog.Errorf("redis ping failed: %v", err)
		return nil, nil, err
	}
	setupLog.Infof("redis client initialized")

	cleanup := func() {
		cleanupLog.Infof("closing redis connection")
		rdb.Close()
	}

	return &Client{
		rdb: rdb,
		log: logger.NewHelper(baseLogger),
	}, cleanup, nil
}

// Ping 测试连接
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Set 存储键值对
func (c *Client) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return c.rdb.Set(ctx, key, value, expiration).Err()
}

// Get 获取值
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// Del 删除键
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Has 判断键是否存在
func (c *Client) Has(ctx context.Context, key string) bool {
	_, err := c.rdb.Get(ctx, key).Result()
	return err == nil
}

// Keys 按模式查找键
func (c *Client) Keys(ctx context.Context, pattern string) ([]string, error) {
	return c.rdb.Keys(ctx, pattern).Result()
}

// SAdd 向集合添加成员
func (c *Client) SAdd(ctx context.Context, key string, members ...any) error {
	return c.rdb.SAdd(ctx, key, members...).Err()
}

// SMembers 获取集合所有成员
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.rdb.SMembers(ctx, key).Result()
}

// Expire 设置键过期时间
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.rdb.Expire(ctx, key, expiration).Err()
}
