# Observability

This guide covers OpenTelemetry tracing in vessel: how it is enabled,
what it emits, and how to wire it to a tracing backend like Jaeger.

For internal instrumentation design (where in the code the spans are
created), see [ARCHITECTURE.md](./ARCHITECTURE.md#observability).

**Target Audience**: Application developers wiring vessel to a tracing
backend; AI agents.

## Overview

Vessel instruments database operations with OpenTelemetry traces by default.
When the application has not configured an OTel SDK, OpenTelemetry's default
provider is no-op and spans are not exported.

This means:

- You do not need to enable anything in Vessel to "turn tracing on."
- You enable tracing by configuring an OTel SDK in your application.
- If you configure an SDK, vessel's spans show up automatically.
- Set `OTEL_ENABLED=false` to force Vessel to use its own no-op tracer even
  when the process has a global OTel provider.

## Quick start: Jaeger via Docker

The fastest way to see traces locally is to run Jaeger in Docker, point
the OTel exporter at its OTLP/gRPC endpoint, and run a query.

### Run Jaeger

```bash
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest
```

- Jaeger UI: `http://localhost:16686`
- OTLP gRPC endpoint: `localhost:4317`

### Configure the OTel SDK in your application

```go
package main

import (
    "context"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

    db "tounilab.com/vessel/db/v1"
)

func initTracing(ctx context.Context) (func(context.Context) error, error) {
    exp, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint("localhost:4317"),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    res, _ := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceName("orders-api"),
            semconv.ServiceVersion("1.2.3"),
        ),
    )

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithResource(res),
    )
    otel.SetTracerProvider(tp)

    return tp.Shutdown, nil
}

func main() {
    ctx := context.Background()
    shutdown, err := initTracing(ctx)
    if err != nil {
        panic(err)
    }
    defer shutdown(ctx)

    database, err := db.NewDB(db.PostgresConfig{
        // ... config ...
    }, nil)
    if err != nil {
        panic(err)
    }
    defer database.Close()

    // Any query through vessel now produces spans automatically.
    _, _ = database.GetRaw(ctx, "users", []string{"id"}, nil, nil, nil)
}
```

That's it. Open the Jaeger UI and you'll see spans for the queries
vessel executed.

## Captured operations

Vessel span names are dialect-qualified method names. The table below shows the
shape; replace `postgres` with `mysql`, `sqlite`, or `mssql` for other
dialects.

| Span name                    | Triggered by                                  |
| ---------------------------- | --------------------------------------------- |
| `postgres.Get`               | `Get` and fluent `Select(...).Get`            |
| `postgres.GetRaw`            | `GetRaw` and fluent `Select(...).GetRaw`      |
| `postgres.Query`             | `Query`                                       |
| `postgres.QueryRaw`          | `QueryRaw`                                    |
| `postgres.Exec`              | raw `Exec`                                    |
| `postgres.Insert`            | `Insert`, bulk insert, and fluent insert exec |
| `postgres.Upsert`            | `Upsert` and fluent upsert                    |
| `postgres.Update`            | `Update` and fluent update exec               |
| `postgres.Delete`            | `Delete` and fluent delete exec               |
| `postgres.Begin`             | `Begin`                                       |
| `postgres.WithTransaction`   | `WithTransaction`                             |
| `postgres.Commit`            | transaction commit                            |
| `postgres.Rollback`          | transaction rollback                          |
| `db.ScanRowsTo`              | typed row scanning helper                     |

If you observe an operation that is not in this list and that you'd
expect to be traced, that's a doc gap — file an issue with the
operation name and your call site.

## Span attributes

Each span carries attributes that describe the operation. The exact
attribute set is dialect-aware and is intentionally narrow: no SQL
parameters are recorded, and no row data is attached to spans.

Common attributes include:

- OpenTelemetry semantic convention database attributes such as `db.system` and
  `db.operation.name`.
- Operation-specific attributes added by the driver code where available, such
  as table names and row counts.

On error, the span records the error and sets status `Error`. Do not rely on
spans for SQL text or parameter capture; use explicit, reviewed application
logging if you need that in a controlled environment.

> **Verification note**: Exact attribute names are subject to change
> across vessel versions. Check `internal/pkg/otel/` for the
> authoritative list.

## Disabling tracing explicitly

If your application initializes the OTel SDK for other libraries but you
specifically want Vessel to be silent, set:

```bash
OTEL_ENABLED=false
```

You can also use OTel's standard sampling/filtering mechanisms:

```go
// Drop spans from the vessel tracer name
sdktrace.NewTracerProvider(
    sdktrace.WithSampler(myFilteringSampler{deny: "vessel"}),
)
```

For the common case of "I don't want any tracing in this process," not
initializing an OTel SDK is enough.

## Sampling

vessel respects whatever sampler you configure on your global
`TracerProvider`. For high-traffic services, a probabilistic sampler
is typical:

```go
sdktrace.NewTracerProvider(
    sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.01)), // 1%
    sdktrace.WithBatcher(exp),
)
```

Be aware: head-based sampling at 1% means most slow queries will not
appear in traces. If you need to capture slow queries specifically,
combine sampling with a separate slow-query log path rather than
relying on traces alone.

## Metrics

Metrics collection is currently scoped to internal pool stats exposed
through `database.PoolStats()`. There is no OTel metrics exporter
wired into vessel yet; if you want pool metrics in your metrics
backend, scrape `PoolStats()` periodically in your application code.

```go
go func() {
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        stats, err := database.PoolStats()
        if err != nil {
            continue
        }
        myMetrics.Gauge("db.pool.open", float64(stats.OpenConnections))
        myMetrics.Gauge("db.pool.inuse", float64(stats.InUse))
        myMetrics.Gauge("db.pool.idle", float64(stats.Idle))
        myMetrics.Counter("db.pool.wait_count", float64(stats.WaitCount))
    }
}()
```

## Multi-service traces

Because vessel propagates `context.Context` through every call,
traces compose naturally with HTTP/gRPC instrumentation in calling
services. A request span from an inbound HTTP handler becomes the
parent of vessel's dialect-qualified database spans without any extra wiring
on vessel's side — provided your HTTP middleware uses
`otelhttp` (or equivalent) and passes the propagated context to
vessel.

```go
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context() // Already has the parent span from otelhttp

    user, err := h.db.GetRaw(ctx, "users", ...) // Child span of HTTP span
    // ...
}
```

This is the entire point of vessel taking `ctx` on every call —
break the context chain by passing `context.Background()` and the
trace tree fragments.

## Best practices

### ✅ DO

- Initialize the OTel SDK once at application startup.
- Pass the request context through to every vessel call so spans
  compose into trace trees.
- Use sampling for high-traffic services; do not export 100% of
  spans to a backend you have not capacity-tested.
- Add `service.name` and `service.version` resource attributes so
  spans are searchable in your tracing backend.

### ❌ DON'T

- Don't pass `context.Background()` into a vessel call inside a
  request handler. You'll lose the parent span and the trace tree
  fragments.
- Don't expect SQL parameters to appear in spans. They are
  deliberately not attached to avoid PII leakage.
- Don't rely on traces as the only source of slow-query data when
  sampling is enabled. Combine with a slow-query log path.
- Don't initialize multiple `TracerProvider`s. The OTel SDK is a
  process-global; one is enough.

## Troubleshooting

### No spans appear in Jaeger

1. Verify the SDK is initialized **before** the first vessel call.
   `otel.SetTracerProvider` registers the provider globally;
   anything that runs before that line uses the no-op provider.
2. Confirm the OTLP exporter is connecting:
   `OTEL_LOG_LEVEL=debug` shows the SDK's own logs.
3. Check that the context passed to vessel originates from the
   instrumented handler, not a fresh `context.Background()`.

### Spans appear but are not nested under the HTTP request

The context chain is broken somewhere between the HTTP handler and
the vessel call. Walk the call stack and confirm the context is
threaded through every intermediate function.

### Span volume is too high

Add a sampler at the `TracerProvider` level. vessel does not have
its own sampling configuration; OTel SDK sampling is the single
source of truth.

## See Also

- [ARCHITECTURE.md](./ARCHITECTURE.md#observability) —
  internal instrumentation design
- [CONFIGURATION.md](./CONFIGURATION.md) — connection pool tuning;
  pool saturation often shows up as long span durations
- [LOGGING.md](./LOGGING.md) — pairs naturally with tracing for
  full observability
