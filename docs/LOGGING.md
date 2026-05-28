# Logging

This guide covers how to use vessel's built-in logger adapters and how
to choose between them. For the `Logger` interface design and how to
write a custom adapter, see
[ARCHITECTURE.md](./ARCHITECTURE.md#logging).

**Target Audience**: Application developers; AI agents.

## Overview

`db.NewDB` takes a `db.Logger` as its second argument. vessel ships
four adapters covering the most common Go logging libraries:

| Adapter            | Logging library                  | Constructor             |
| ------------------ | -------------------------------- | ----------------------- |
| slog               | Go stdlib `log/slog`             | `db.NewSlogAdapter`     |
| logrus             | `github.com/sirupsen/logrus`     | `db.NewLogrusAdapter`   |
| zap                | `go.uber.org/zap`                | `db.NewZapAdapter`      |
| apex/log           | `github.com/apex/log`            | `db.NewApexAdapter`     |

Passing `nil` for the logger argument is supported — vessel will log
nothing in that case. This is fine for tests and for cases where
tracing already gives you the observability you need.

## What vessel logs

Vessel logs through `SafeLogger` around driver operations. The exact number of
log lines depends on the code path and dialect, but the current behavior is:

- **Debug** — successful queries and transaction lifecycle events.
- **Warn** — slow successful queries and application-level query failures such
  as validation or constraint errors.
- **Error** — connection failures, query execution failures, scan/close errors,
  and unclassified/system errors.

Vessel logs structured fields such as `db_driver`, `operation`, `table`,
`duration_ms`, `rows_returned`, `error_type`, and `correlation_id`. It does not
log connection passwords or row data. SQL strings are not included in the normal
success/error helpers.

## slog (stdlib)

```go
import (
    "log/slog"
    "os"

    db "tounilab.com/vessel/db/v1"
)

handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
})
logger := db.NewSlogAdapter(slog.New(handler))

database, err := db.NewDB(cfg, logger)
```

slog is the recommended default for new projects in Go 1.21+. It has
no third-party dependencies, structured-logging support is first-class,
and it composes with any handler that satisfies `slog.Handler`.

## logrus

```go
import (
    "github.com/sirupsen/logrus"

    db "tounilab.com/vessel/db/v1"
)

l := logrus.New()
l.SetFormatter(&logrus.JSONFormatter{})
l.SetLevel(logrus.InfoLevel)

logger := db.NewLogrusAdapter(l)

database, err := db.NewDB(cfg, logger)
```

Use this if your existing service is already on logrus. There is no
reason to migrate an existing logrus service to slog just to use
vessel.

## zap

```go
import (
    "go.uber.org/zap"

    db "tounilab.com/vessel/db/v1"
)

z, err := zap.NewProduction()
if err != nil {
    panic(err)
}
defer z.Sync()

logger := db.NewZapAdapter(z)

database, err := db.NewDB(cfg, logger)
```

zap is the right choice when log throughput matters and you've
verified that your bottleneck is in the logging path. For most
services, slog is fast enough and has fewer dependencies.

## apex/log

```go
import (
    "os"

    apex "github.com/apex/log"
    "github.com/apex/log/handlers/json"

    db "tounilab.com/vessel/db/v1"
)

apex.SetHandler(json.New(os.Stdout))
apex.SetLevel(apex.InfoLevel)

logger := db.NewApexAdapter(apex.Log)

database, err := db.NewDB(cfg, logger)
```

apex/log is the least common of the four; use it only if you're
already standardized on it.

## No logger (silent)

```go
database, err := db.NewDB(cfg, nil)
```

`nil` is a valid `Logger`. Vessel emits nothing. Useful for tests and for
cases where OTel tracing covers your observability needs.

## Structured logging context

vessel uses key-value logging throughout. When wired to slog,
logrus, zap, or apex, your output will include structured fields:

```json
{
  "time": "2026-05-27T14:33:11Z",
  "level": "DEBUG",
  "msg": "postgres.SELECT: query executed successfully",
  "db_driver": "postgres",
  "operation": "select",
  "table": "users",
  "duration_ms": "12ms",
  "rows_returned": 1,
  "correlation_id": ""
}
```

Field names come from Vessel before they reach the adapter, so they remain
consistent across slog, logrus, zap, and apex/log.

## Custom logger

If you need to wire vessel to a logging library not in the table
above (zerolog, klog, an internal logger), implement the
`db.Logger` interface directly:

```go
type Logger interface {
    Debug(msg string, keyvals ...any)
    Info(msg string, keyvals ...any)
    Warn(msg string, keyvals ...any)
    Error(msg string, keyvals ...any)
    With(fields ...any) Logger
}
```

The interface contract is also summarized in
[ARCHITECTURE.md](./ARCHITECTURE.md#logging).

## Choosing a level for vessel logs

Common starting points:

- **Production HTTP service**: `Info` or `Warn` level. Normal successful
  queries are logged at `Debug`, while slow queries and failures remain visible.
- **Background worker**: `Info` level, or `Debug` during initial rollout.
- **Local development**: `Debug` for lifecycle and query-success events. Use
  query-preview methods when you need to inspect generated SQL.
- **CI tests**: `Warn` or `Error` to avoid polluting test output.

## Best practices

### ✅ DO

- Pass the same logger you use elsewhere in the service. vessel logs
  should integrate with your application's log stream.
- Use structured fields (key-value) rather than string formatting
  in your application code; vessel does the same internally.
- Set log level via your application's configuration. vessel does
  not own log level — your `slog.Handler`, `logrus.Logger`, etc.
  does.
- Pair logs with tracing. The two are complementary: traces show
  causal flow, logs show specific events with context.

### ❌ DON'T

- Don't log the connection config struct. It contains the password.
- Don't wrap vessel's logger output in a higher-level "logging
  framework" that re-parses fields. Pass the underlying logger
  through directly.
- Don't use `panic` or `os.Exit` in a custom logger's methods.
  vessel will not catch them.
- Don't change log levels at runtime by reaching into vessel
  internals. Configure the underlying logger.

## Troubleshooting

### No vessel logs appear

1. Verify the logger you passed to `NewDB` is configured to a level
   that includes the levels vessel emits.
2. Confirm the logger's underlying writer is reachable (file
   permissions, network endpoint, etc.).
3. Pass `nil` temporarily to confirm vessel is otherwise working;
   if everything works with `nil`, the issue is in your logger
   wiring, not vessel.

### Logs are too noisy

Most vessel chatter is at `Debug`. Set the underlying logger to
`Info` and the noise disappears. If you still see noise at `Info`,
it likely indicates something vessel actually thinks is worth
your attention.

### Log fields are inconsistent

Vessel uses the same key-value fields before passing them to adapters. If you
see inconsistencies, check adapter behavior and handler configuration,
especially logrus/zap field encoders or custom field-name remapping.

## See Also

- [ARCHITECTURE.md](./ARCHITECTURE.md#logging) —
  Logger interface design
- [OBSERVABILITY.md](./OBSERVABILITY.md) — tracing pairs naturally
  with structured logs
- [CONFIGURATION.md](./CONFIGURATION.md) — `NewDB` and config
  examples
