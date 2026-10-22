//go:build test

package v1

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"syscall"
	"testing"

	"tounilab.com/vessel/db/v1/plugin"
)

// leakyFactory stands in for a driver whose error quotes the connection string it was
// given, as go-mssqldb's url.Error did for a host it could not parse (#96).
type leakyFactory struct{}

func (leakyFactory) Name() string { return "postgres" }

func (leakyFactory) Create(_ context.Context, cfg any) (any, error) {
	dsn := cfg.(DBConfig).DSN()
	return nil, fmt.Errorf("connect: %w: %w", &url.Error{Op: "parse", URL: dsn, Err: syscall.EINVAL}, context.DeadlineExceeded)
}

// TestNewDB_RedactsThePassword: what NewDB returns names no password, in its message or
// anywhere errors.As can reach, while the context error a caller retries on survives.
func TestNewDB_RedactsThePassword(t *testing.T) {
	if err := plugin.Register(leakyFactory{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = plugin.Unregister("postgres") })

	for _, cfg := range []DBConfig{
		PostgresConfig{Host: "db.internal", Port: 5432, User: "app", Password: "s3cret", Database: "app"},
		&PostgresConfig{Host: "db.internal", Port: 5432, User: "app", Password: "s3cret", Database: "app"},
	} {
		_, err := NewDB(cfg, nil)
		if err == nil || strings.Contains(err.Error(), "s3cret") {
			t.Fatalf("%T: err = %v; want one without the password", cfg, err)
		}
		var leaked *url.Error
		if errors.As(err, &leaked) {
			t.Errorf("%T: the chain reaches the driver's error, which holds the password", cfg)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("%T: err = %v; want the context error kept", cfg, err)
		}
	}
}

// TestRedactConfigError: every engine's config, by value or pointer, gives its password;
// one without a password, or of a type this package does not know, is left as written.
func TestRedactConfigError(t *testing.T) {
	quote := func(pw string) error {
		return &url.Error{Op: "parse", URL: "sqlserver://u:" + pw + "@h", Err: syscall.EINVAL}
	}
	for _, cfg := range []DBConfig{
		PostgresConfig{Password: "s3cret"},
		&PostgresConfig{Password: "s3cret"},
		MysqlConfig{Password: "s3cret"},
		&MysqlConfig{Password: "s3cret"},
		MSSQLConfig{Password: "s3cret"},
		&MSSQLConfig{Password: "s3cret"},
	} {
		if err := redactConfigError(quote("s3cret"), cfg); strings.Contains(err.Error(), "s3cret") {
			t.Errorf("%T: %v", cfg, err)
		}
	}
	for _, cfg := range []DBConfig{PostgresConfig{}, SQLiteConfig{FilePath: "x.db"}, (*MSSQLConfig)(nil)} {
		original := quote("x")
		if err := redactConfigError(original, cfg); err != original {
			t.Errorf("%T: err = %v; want it unchanged", cfg, err)
		}
	}
}
