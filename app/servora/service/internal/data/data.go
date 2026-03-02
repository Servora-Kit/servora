package data

import (
	"errors"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/horonlee/servora/api/gen/go/conf/v1"
	"github.com/horonlee/servora/app/servora/service/internal/data/ent"
	"github.com/horonlee/servora/pkg/governance/registry"
	"github.com/horonlee/servora/pkg/logger"
	"github.com/horonlee/servora/pkg/redis"
	"github.com/horonlee/servora/pkg/transport/client"

	"github.com/glebarez/sqlite"
	"github.com/google/wire"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(registry.NewDiscovery, NewDB, NewEntClient, NewRedis, NewData, NewAuthRepo, NewUserRepo, NewTestRepo)

// Data .
type Data struct {
	entClient *ent.Client
	log       *logger.Helper
	client    client.Client
	redis     *redis.Client
}

// NewData .
func NewData(entClient *ent.Client, c *conf.Data, l logger.Logger, client client.Client, redisClient *redis.Client) (*Data, func(), error) {
	_ = c
	cleanup := func() {
		logger.NewHelper(l).Info("closing the data resources")
		if err := entClient.Close(); err != nil {
			logger.NewHelper(l).Warnf("failed to close ent client: %v", err)
		}
	}
	return &Data{
		entClient: entClient,
		log:       logger.NewHelper(l, logger.WithModule("data/data/servora-service")),
		client:    client,
		redis:     redisClient,
	}, cleanup, nil
}

func NewEntClient(db *gorm.DB, cfg *conf.Data, app *conf.App, l logger.Logger) (*ent.Client, error) {
	dbConn, err := db.DB()
	if err != nil {
		return nil, err
	}

	opts := []ent.Option{
		ent.Driver(entsql.OpenDB(entDialect(cfg.Database.GetDriver()), dbConn)),
		ent.Log(logger.EntLogFuncFrom(l, "ent/data/servora-service")),
	}
	if strings.EqualFold(app.GetEnv(), "dev") {
		opts = append(opts, ent.Debug())
	}

	client := ent.NewClient(opts...)
	return client, nil
}

func entDialect(driver string) string {
	switch strings.ToLower(driver) {
	case "mysql":
		return dialect.MySQL
	case "postgres", "postgresql":
		return dialect.Postgres
	default:
		return dialect.SQLite
	}
}

func NewDB(cfg *conf.Data, l logger.Logger) (*gorm.DB, error) {
	gormLogger := logger.GormLoggerFrom(l, "gorm/data/servora-service")
	dbLog := logger.NewHelper(l,
		logger.WithModule("data/db/servora-service"),
		logger.WithField("operation", "NewDB"),
	)

	var dialector gorm.Dialector
	switch strings.ToLower(cfg.Database.GetDriver()) {
	case "mysql":
		dialector = mysql.Open(cfg.Database.GetSource())
	case "sqlite":
		dialector = sqlite.Open(cfg.Database.GetSource())
	case "postgres", "postgresql":
		dialector = postgres.Open(cfg.Database.GetSource())
	default:
		return nil, errors.New("connect db fail: unsupported db driver")
	}

	var db *gorm.DB
	var err error
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(dialector, &gorm.Config{
			Logger: gormLogger,
		})
		if err == nil {
			return db, nil
		}
		if i < maxRetries-1 {
			delay := time.Duration(1<<uint(i)) * time.Second
			dbLog.Warnf("database connection failed (attempt %d/%d), retrying in %v: %v", i+1, maxRetries, delay, err)
			time.Sleep(delay)
		}
	}
	return nil, err
}

func NewRedis(cfg *conf.Data, l logger.Logger) (*redis.Client, func(), error) {
	redisConfig := redis.NewConfigFromProto(cfg.Redis)
	if redisConfig == nil {
		return nil, nil, errors.New("redis configuration is required")
	}

	return redis.NewClient(redisConfig, logger.With(l, logger.WithModule("redis/data/servora-service")))
}
