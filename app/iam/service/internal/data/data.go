package data

import (
	"context"
	"errors"
	"fmt"
	"strings"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	iamconf "github.com/Servora-Kit/servora/api/gen/go/iam/conf/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"

	entdrv "github.com/Servora-Kit/servora/pkg/ent"
	"github.com/Servora-Kit/servora/pkg/governance/registry"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/redis"
	"github.com/Servora-Kit/servora/pkg/transport/client"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var ProviderSet = wire.NewSet(registry.NewDiscovery, NewEntDriver, NewDBClient, NewPlatformRootID, NewRedis, NewData, NewAuthnRepo, NewAuthZRepo, NewUserRepo, NewTestRepo, NewOrganizationRepo, NewProjectRepo)

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
		log:       logger.NewHelper(l, logger.WithModule("core/data/iam-service")),
		client:    client,
		redis:     redisClient,
	}, cleanup, nil
}

type txKey struct{}

// Ent returns the ent client for the current context. If a transaction is
// active (started by InTx), returns the transactional client; otherwise the
// default client.
func (d *Data) Ent(ctx context.Context) *ent.Client {
	if c, ok := ctx.Value(txKey{}).(*ent.Client); ok {
		return c
	}
	return d.entClient
}

// InTx executes fn within a database transaction. The transactional client is
// propagated through the context so that repo methods using Ent(ctx) will
// automatically participate in the transaction.
func (d *Data) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := d.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if v := recover(); v != nil {
			_ = tx.Rollback()
			panic(v)
		}
	}()
	txCtx := context.WithValue(ctx, txKey{}, tx.Client())
	if err := fn(txCtx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			return fmt.Errorf("%w: rolling back transaction: %v", err, rerr)
		}
		return err
	}
	return tx.Commit()
}

func NewEntDriver(cfg *conf.Data) (*entsql.Driver, error) {
	return entdrv.NewDriver(cfg)
}

func NewDBClient(driver *entsql.Driver, app *conf.App, bizConf *iamconf.Biz, l logger.Logger) (*ent.Client, error) {
	opts := []ent.Option{
		ent.Driver(driver),
		ent.Log(logger.EntLogFuncFrom(l, "ent/data/iam-service")),
	}
	if strings.EqualFold(app.GetEnv(), "dev") {
		opts = append(opts, ent.Debug())
	}

	ec := ent.NewClient(opts...)

	ctx := context.Background()
	if err := ec.Schema.Create(ctx); err != nil {
		return nil, errors.New("ent auto-migrate: " + err.Error())
	}

	if _, err := seedPlatform(ctx, ec); err != nil {
		return nil, errors.New("seed platform: " + err.Error())
	}

	if err := seedPlatformAdmin(ctx, ec, bizConf.GetSeed()); err != nil {
		seedLog := logger.NewHelper(l, logger.WithModule("seed/data/iam-service"))
		seedLog.Warnf("seed platform admin: %v", err)
	}

	return ec, nil
}

func NewRedis(cfg *conf.Data, l logger.Logger) (*redis.Client, func(), error) {
	redisConfig := redis.NewConfigFromProto(cfg.Redis)
	if redisConfig == nil {
		return nil, nil, errors.New("redis configuration is required")
	}

	return redis.NewClient(redisConfig, logger.With(l, logger.WithModule("redis/data/iam-service")))
}
