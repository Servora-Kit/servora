package data

import (
	"database/sql"
	"errors"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	"github.com/Servora-Kit/servora/app/servora/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/governance/registry"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/redis"
	"github.com/Servora-Kit/servora/pkg/transport/client"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var ProviderSet = wire.NewSet(registry.NewDiscovery, NewDBClient, NewRedis, NewData, NewAuthRepo, NewUserRepo, NewTestRepo)

type Data struct {
	entClient *ent.Client
	log       *logger.Helper
	client    client.Client
	redis     *redis.Client
}

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
		log:       logger.NewHelper(l, logger.WithModule("core/data/servora-service")),
		client:    client,
		redis:     redisClient,
	}, cleanup, nil
}

func NewDBClient(cfg *conf.Data, app *conf.App, l logger.Logger) (*ent.Client, error) {
	driver, err := newEntDriver(cfg)
	if err != nil {
		return nil, err
	}

	opts := []ent.Option{
		ent.Driver(driver),
		ent.Log(logger.EntLogFuncFrom(l, "ent/data/servora-service")),
	}
	if strings.EqualFold(app.GetEnv(), "dev") {
		opts = append(opts, ent.Debug())
	}

	return ent.NewClient(opts...), nil
}

func newEntDriver(cfg *conf.Data) (*entsql.Driver, error) {
	var driverName string
	var entDialect string

	switch strings.ToLower(cfg.Database.GetDriver()) {
	case "mysql":
		driverName = "mysql"
		entDialect = dialect.MySQL
	case "postgres", "postgresql":
		driverName = "postgres"
		entDialect = dialect.Postgres
	case "sqlite":
		driverName = "sqlite3"
		entDialect = dialect.SQLite
	default:
		return nil, errors.New("unsupported db driver: " + cfg.Database.GetDriver())
	}

	db, err := sql.Open(driverName, cfg.Database.GetSource())
	if err != nil {
		return nil, err
	}

	return entsql.OpenDB(entDialect, db), nil
}

func NewRedis(cfg *conf.Data, l logger.Logger) (*redis.Client, func(), error) {
	redisConfig := redis.NewConfigFromProto(cfg.Redis)
	if redisConfig == nil {
		return nil, nil, errors.New("redis configuration is required")
	}

	return redis.NewClient(redisConfig, logger.With(l, logger.WithModule("redis/data/servora-service")))
}
