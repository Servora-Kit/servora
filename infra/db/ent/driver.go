package ent

import (
	"database/sql"
	"fmt"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
)

// NewDriver creates an ent SQL driver from the shared data configuration.
// Callers must blank-import the appropriate database/sql driver, e.g.:
//
//	_ "github.com/go-sql-driver/mysql"
//	_ "github.com/lib/pq"
//	_ "github.com/mattn/go-sqlite3"
func NewDriver(cfg *conf.Data) (*entsql.Driver, error) {
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
		return nil, fmt.Errorf("unsupported db driver: %s", cfg.Database.GetDriver())
	}

	db, err := sql.Open(driverName, cfg.Database.GetSource())
	if err != nil {
		return nil, err
	}

	return entsql.OpenDB(entDialect, db), nil
}
