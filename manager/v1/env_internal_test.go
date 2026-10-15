//go:build test

package v1

import (
	"log/slog"
	"net/url"
	"strings"
	"testing"

	db "tounilab.com/vessel/db/v1"
)

// TestRedactingLogger_RedactsEveryPart: a value can reach a log line through its
// message, a field given to With, or a Stringer such as a connection URL.
func TestRedactingLogger_RedactsEveryPart(t *testing.T) {
	var logs strings.Builder
	logger := redactingLogger{
		logger: db.NewSlogAdapter(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))),
		forms:  redactions([]string{"s3cret"}),
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
