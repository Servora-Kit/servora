package ent

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"go.uber.org/zap"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
)

func TestNewDriverRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     *corev1.Data
		options []DriverOption
		want    string
	}{
		{name: "nil data", want: "data configuration is nil"},
		{name: "nil database", cfg: &corev1.Data{}, want: "database configuration is nil"},
		{name: "empty driver", cfg: dataConfig("", "source"), want: "database driver is empty"},
		{name: "unsupported driver", cfg: dataConfig("oracle", "source"), want: "unsupported database driver"},
		{name: "empty source", cfg: dataConfig("sqlite", ""), want: "database source is empty"},
		{name: "nil external database", cfg: dataConfig("sqlite", ""), options: []DriverOption{WithDB(nil)}, want: "external sql.DB is nil"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewDriver(test.cfg, test.options...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewDriver() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestNewDriverUsesBorrowedDBWithoutClosingIt(t *testing.T) {
	t.Parallel()

	db := sql.OpenDB(stubConnector{})
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("prime stub DB: %v", err)
	}

	drv, err := NewDriver(
		dataConfig("postgresql", ""),
		WithDB(db),
		WithTracing(zap.NewNop()),
	)
	if err != nil {
		t.Fatalf("NewDriver: %v", err)
	}
	if got := drv.Dialect(); got != dialect.Postgres {
		t.Fatalf("Dialect() = %q, want %q", got, dialect.Postgres)
	}
	if err := drv.Close(); err != nil {
		t.Fatalf("Close borrowed driver: %v", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("borrowed DB was closed: %v", err)
	}
}

func TestManagedDriverCloseOwnership(t *testing.T) {
	t.Parallel()

	inner := &closeTrackingDriver{}
	if err := manageDriver(inner, false).Close(); err != nil {
		t.Fatalf("borrowed Close: %v", err)
	}
	if inner.closeCalls != 0 {
		t.Fatalf("borrowed driver close calls = %d, want 0", inner.closeCalls)
	}
	if err := manageDriver(inner, true).Close(); err != nil {
		t.Fatalf("owned Close: %v", err)
	}
	if inner.closeCalls != 1 {
		t.Fatalf("owned driver close calls = %d, want 1", inner.closeCalls)
	}
}

func dataConfig(driverName, source string) *corev1.Data {
	return &corev1.Data{Database: &corev1.Data_Database{Driver: driverName, Source: source}}
}

type closeTrackingDriver struct {
	dialect.Driver
	closeCalls int
}

func (driver *closeTrackingDriver) Close() error {
	driver.closeCalls++
	return nil
}

func (driver *closeTrackingDriver) Dialect() string { return dialect.SQLite }

type stubConnector struct{}

func (stubConnector) Connect(context.Context) (driver.Conn, error) { return &stubConn{}, nil }
func (stubConnector) Driver() driver.Driver                        { return stubDriver{} }

type stubDriver struct{}

func (stubDriver) Open(string) (driver.Conn, error) { return &stubConn{}, nil }

type stubConn struct{}

func (*stubConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (*stubConn) Close() error                        { return nil }
func (*stubConn) Begin() (driver.Tx, error)           { return nil, errors.New("not implemented") }
func (*stubConn) Ping(context.Context) error          { return nil }
func (*stubConn) ResetSession(context.Context) error  { return nil }
func (*stubConn) IsValid() bool                       { return true }

var _ driver.Pinger = (*stubConn)(nil)
var _ driver.SessionResetter = (*stubConn)(nil)
var _ driver.Validator = (*stubConn)(nil)
var _ io.Closer = (*stubConn)(nil)
