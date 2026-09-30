package v1

import (
	"context"
	"fmt"
	"log/slog"
)

// statementMessage is the message goose logs before each SQL statement. Those go to
// Debug: a baseline migration would otherwise put a whole schema in the Info log. If
// goose renames it, statements land at Info, which is noisy but not wrong.
const statementMessage = "executing statement"

// WithSlog sends the Migrator's progress to logger, with goose's structured fields
// (source, version, duration_seconds, current_version): one Info line per applied
// migration and per run, and each executed statement at Debug. Without it the
// Migrator logs nothing; Up and Status return what happened either way.
func WithSlog(logger *slog.Logger) Option {
	return func(o *options) { o.logger = logger }
}

// statementsAtDebug passes goose's records to the application's handler, lowering the
// per-statement ones to Debug.
type statementsAtDebug struct {
	slog.Handler
}

func (h statementsAtDebug) Handle(ctx context.Context, r slog.Record) error {
	if r.Message == statementMessage && r.Level > slog.LevelDebug {
		r.Level = slog.LevelDebug
		// slog.Logger asked Enabled at the original level; ask again at the new one.
		if !h.Enabled(ctx, r.Level) {
			return nil
		}
	}
	if err := h.Handler.Handle(ctx, r); err != nil {
		return fmt.Errorf("migrate: log: %w", err)
	}
	return nil
}

func (h statementsAtDebug) WithAttrs(attrs []slog.Attr) slog.Handler {
	return statementsAtDebug{h.Handler.WithAttrs(attrs)}
}

func (h statementsAtDebug) WithGroup(name string) slog.Handler {
	return statementsAtDebug{h.Handler.WithGroup(name)}
}
