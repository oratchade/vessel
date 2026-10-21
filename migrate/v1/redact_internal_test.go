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
	"testing/fstest"

	db "tounilab.com/vessel/db/v1"
)

// #96: the migrator opens its connection lazily, so a driver error quoting the
// connection string can come back from New, Up, Status or Version. What it returns names
// no password, and a caller's errors.Is still finds this package's sentinels and context
// errors.
func TestRedactError(t *testing.T) {
	secrets := passwords(&db.PostgresConfig{Password: "s3cret"})
	leaky := fmt.Errorf("migrate: up: %w: %w",
		&url.Error{Op: "parse", URL: "postgres://app:s3cret@db.internal", Err: syscall.EINVAL}, context.Canceled)

	err := redactError(leaky, secrets)
	var leaked *url.Error
	if strings.Contains(err.Error(), "s3cret") || errors.As(err, &leaked) {
		t.Errorf("err = %v; want the password out of its message and its chain", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v; want the context error kept", err)
	}
	for _, sentinel := range []error{ErrNoMigrations, ErrMySQLParseTime, ErrUnsupportedDriver} {
		if !errors.Is(redactError(fmt.Errorf("x s3cret: %w", sentinel), secrets), sentinel) {
			t.Errorf("%v: want it kept", sentinel)
		}
	}
	if redactError(leaky, passwords(db.SQLiteConfig{FilePath: "x.db"})) != leaky {
		t.Error("a config without a password should leave the error as written")
	}
}

// New's own refusals keep their sentinels through the redaction.
func TestNew_KeepsItsSentinels(t *testing.T) {
	_, err := New(db.MysqlConfig{Host: "db.internal", Port: 3306, User: "app", Password: "s3cret", Database: "app"},
		fstest.MapFS{"00001_init.sql": {Data: []byte("-- +goose Up\nSELECT 1;\n")}})
	if !errors.Is(err, ErrMySQLParseTime) || strings.Contains(err.Error(), "s3cret") {
		t.Errorf("err = %v; want ErrMySQLParseTime, with no password", err)
	}
}
