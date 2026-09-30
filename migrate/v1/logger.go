package v1

import (
	"context"
	"log/slog"

	db "tounilab.com/vessel/db/v1"
)

// statementMessage is the message goose logs before each SQL statement. Those go to
// Debug: a baseline migration would otherwise put a whole schema in the Info log. If
// goose renames it, statements land at Info, which is noisy but not wrong.
const statementMessage = "executing statement"

// WithLogger sends the Migrator's progress to logger, with goose's structured fields
// (source, version, duration): one Info line per applied migration and per run, and
// each executed statement at Debug. Without it the Migrator logs nothing; Up and
// Status return what happened either way.
func WithLogger(logger db.Logger) Option {
	return func(o *options) { o.logger = logger }
}

// loggerHandler is a slog.Handler writing to a Vessel db.Logger, so goose's structured
// logging reaches whatever logger the application gave Vessel.
type loggerHandler struct {
	logger db.Logger
}

func (h *loggerHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *loggerHandler) Handle(_ context.Context, r slog.Record) error {
	args := make([]any, 0, 2*r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		args = append(args, a.Key, a.Value.Any())
		return true
	})
	switch {
	case r.Message == statementMessage || r.Level < slog.LevelInfo:
		h.logger.Debug(r.Message, args...)
	case r.Level >= slog.LevelError:
		h.logger.Error(r.Message, args...)
	case r.Level >= slog.LevelWarn:
		h.logger.Warn(r.Message, args...)
	default:
		h.logger.Info(r.Message, args...)
	}
	return nil
}

func (h *loggerHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	fields := make([]any, 0, 2*len(attrs))
	for _, a := range attrs {
		fields = append(fields, a.Key, a.Value.Any())
	}
	return &loggerHandler{logger: h.logger.With(fields...)}
}

// WithGroup keeps the handler as is. ponytail: goose logs no groups; flatten them with
// a key prefix if it ever does.
func (h *loggerHandler) WithGroup(string) slog.Handler { return h }
