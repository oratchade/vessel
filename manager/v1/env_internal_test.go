//go:build test

package v1

import (
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"testing"

	db "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/internal/redact"
	"tounilab.com/vessel/manager/v1/config"
)

// TestRedactingLogger_RedactsEveryPart: a value can reach a log line through its
// message, a field given to With, or a Stringer such as a connection URL.
func TestRedactingLogger_RedactsEveryPart(t *testing.T) {
	var logs strings.Builder
	logger := redactingLogger{
		logger: db.NewSlogAdapter(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))),
		forms:  redact.Forms([]string{"s3cret"}),
	}.With("user", "s3cret")
	for _, log := range []func(string, ...any){logger.Debug, logger.Info, logger.Warn, logger.Error} {
		log("connecting with s3cret failed",
			"url", &url.URL{Scheme: "sqlserver", User: url.UserPassword("app", "s3cret"), Host: "localhost"})
	}
	if strings.Count(logs.String(), "[REDACTED]") != 12 || strings.Contains(logs.String(), "s3cret") {
		t.Errorf("want every part of every line redacted:\n%s", logs.String())
	}
}

// TestWithoutIndices: Go 1.27 writes array indices into a JSON type error's field path,
// earlier releases do not; both must match the path recorded for a placeholder.
func TestWithoutIndices(t *testing.T) {
	for _, field := range []string{"entries.postgres.port", "entries.0.postgres.port", "entries.12.postgres.port"} {
		if got := withoutIndices(field); got != "entries.postgres.port" {
			t.Errorf("withoutIndices(%q) = %q, want entries.postgres.port", field, got)
		}
	}
}

// TestRedactingLogger_RedactsEveryKindOfField: a field can hold bytes, a slice, a map or
// a slog.Attr, whose key and value must stay paired.
func TestRedactingLogger_RedactsEveryKindOfField(t *testing.T) {
	var logs strings.Builder
	logger := redactingLogger{
		logger: db.NewSlogAdapter(slog.New(slog.NewJSONHandler(&logs, nil))),
		forms:  redact.Forms([]string{"s3cret"}),
	}
	logger.Info("connecting",
		"bytes", []byte("s3cret"), "slice", []string{"s3cret"}, "map", map[string]string{"pw": "s3cret"},
		slog.String("attr", "s3cret"), "port", 5432)
	got := logs.String()
	if strings.Contains(got, "s3cret") || strings.Contains(got, "BADKEY") || !strings.Contains(got, `"port":5432`) {
		t.Errorf("want every field redacted, paired, numbers kept:\n%s", got)
	}
}

// TestPasswords: every engine with a password gives it; an empty one is no secret.
func TestPasswords(t *testing.T) {
	cfg := &config.ManagerConfig{Entries: []config.ConfigEntry{
		{Postgres: &db.PostgresConfig{Password: "pg"}},
		{MySQL: &db.MysqlConfig{Password: "my"}},
		{MSSQL: &db.MSSQLConfig{Password: "ms"}},
		{SQLite: &db.SQLiteConfig{FilePath: "x.db"}},
		{Postgres: &db.PostgresConfig{}},
	}}
	if got := passwords(cfg); !slices.Equal(got, []string{"pg", "my", "ms"}) {
		t.Errorf("passwords = %q, want pg, my, ms", got)
	}
}
