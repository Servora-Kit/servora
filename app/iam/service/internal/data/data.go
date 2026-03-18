package data

import (
	"context"
	"errors"
	"fmt"
	"strings"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/migrate"

	entdrv "github.com/Servora-Kit/servora/pkg/ent"
	"github.com/Servora-Kit/servora/pkg/governance/registry"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/mail"
	"github.com/Servora-Kit/servora/pkg/redis"
	"github.com/Servora-Kit/servora/pkg/transport/client"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var ProviderSet = wire.NewSet(registry.NewDiscovery, NewEntDriver, NewDBClient, NewRedis, NewData, NewAuthnRepo, NewAuthZRepo, NewUserRepo, NewTestRepo, NewTenantRepo, NewOrganizationRepo, NewOTPRepo, NewMailSender, NewApplicationRepo, NewOIDCStorage, NewSeeder, NewRbacRoleRepo, NewRbacPermissionRepo, NewRbacMenuRepo, NewPositionRepo, NewDictRepo)

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

// Ent 返回当前上下文的 ent 客户端。若处于 RunInEntTx 启动的事务中，则返回事务客户端；否则返回默认客户端。
func (d *Data) Ent(ctx context.Context) *ent.Client {
	if c, ok := ctx.Value(txKey{}).(*ent.Client); ok {
		return c
	}
	return d.entClient
}

// RunInEntTx 在 ent 事务中执行 fn。事务客户端通过 context 传递，使用 Ent(ctx) 的仓库方法会自动参与该事务。
func (d *Data) RunInEntTx(ctx context.Context, fn func(ctx context.Context) error) error {
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

func NewDBClient(driver *entsql.Driver, app *conf.App, l logger.Logger) (*ent.Client, error) {
	opts := []ent.Option{
		ent.Driver(driver),
		ent.Log(logger.EntLogFuncFrom(l, "ent/data/iam-service")),
	}
	if strings.EqualFold(app.GetEnv(), "dev") {
		opts = append(opts, ent.Debug())
	}

	ec := ent.NewClient(opts...)

	ctx := context.Background()
	if err := ec.Schema.Create(ctx, migrate.WithDropIndex(true)); err != nil {
		return nil, errors.New("ent auto-migrate: " + err.Error())
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

func NewMailSender(c *conf.Mail) mail.Sender {
	return mail.NewSender(c)
}
