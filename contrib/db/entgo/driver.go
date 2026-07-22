package ent

import (
	"database/sql"
	"fmt"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"go.uber.org/zap"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
)

// DriverOption configures NewDriver.
type DriverOption interface {
	apply(*driverConfig) error
}

type driverOption func(*driverConfig) error

func (option driverOption) apply(config *driverConfig) error { return option(config) }

type driverConfig struct {
	db     *sql.DB
	trace  bool
	log    *zap.Logger
	ownsDB bool
}

// WithDB uses an existing database/sql pool. The returned Ent driver borrows
// the pool: closing the Ent driver does not close db.
func WithDB(db *sql.DB) DriverOption {
	return driverOption(func(config *driverConfig) error {
		if db == nil {
			return fmt.Errorf("external sql.DB is nil")
		}
		config.db = db
		config.ownsDB = false
		return nil
	})
}

// WithTracing enables trace-correlated SQL logging. A nil logger disables
// output while preserving the same driver composition.
func WithTracing(log *zap.Logger) DriverOption {
	return driverOption(func(config *driverConfig) error {
		config.trace = true
		config.log = log
		return nil
	})
}

// NewDriver creates an Ent SQL driver from the shared data configuration.
// Without WithDB, NewDriver opens and owns a database/sql pool, so Close closes
// it. WithDB supplies a borrowed pool whose lifecycle remains with the caller.
// Callers opening an internal pool must blank-import its database/sql driver.
func NewDriver(cfg *corev1.Data, options ...DriverOption) (dialect.Driver, error) {
	if cfg == nil {
		return nil, fmt.Errorf("data configuration is nil")
	}
	if cfg.Database == nil {
		return nil, fmt.Errorf("database configuration is nil")
	}
	driverName := strings.ToLower(strings.TrimSpace(cfg.Database.GetDriver()))
	if driverName == "" {
		return nil, fmt.Errorf("database driver is empty")
	}

	sqlDriver, entDialect, err := resolveDriver(driverName)
	if err != nil {
		return nil, err
	}
	config := driverConfig{}
	for index, option := range options {
		if option == nil {
			return nil, fmt.Errorf("driver option %d is nil", index)
		}
		if err := option.apply(&config); err != nil {
			return nil, fmt.Errorf("apply driver option %d: %w", index, err)
		}
	}
	if config.db == nil {
		source := strings.TrimSpace(cfg.Database.GetSource())
		if source == "" {
			return nil, fmt.Errorf("database source is empty")
		}
		config.db, err = sql.Open(sqlDriver, source)
		if err != nil {
			return nil, fmt.Errorf("open %s database: %w", driverName, err)
		}
		config.ownsDB = true
	}

	result := manageDriver(entsql.OpenDB(entDialect, config.db), config.ownsDB)
	if config.trace {
		result = wrapWithTracing(result, config.log)
	}
	return result, nil
}

func resolveDriver(name string) (sqlDriver, entDialect string, err error) {
	switch name {
	case "mysql":
		return "mysql", dialect.MySQL, nil
	case "postgres", "postgresql", "pgx":
		return "pgx", dialect.Postgres, nil
	case "sqlite", "sqlite3":
		return "sqlite3", dialect.SQLite, nil
	default:
		return "", "", fmt.Errorf("unsupported database driver %q", name)
	}
}

type managedDriver struct {
	dialect.Driver
	owns bool
}

func manageDriver(inner dialect.Driver, owns bool) dialect.Driver {
	return &managedDriver{Driver: inner, owns: owns}
}

func (driver *managedDriver) Close() error {
	if !driver.owns {
		return nil
	}
	return driver.Driver.Close()
}
