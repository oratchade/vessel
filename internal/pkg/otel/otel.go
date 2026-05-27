package otel

import (
	"context"
	"os"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Package-level state for OTEL initialization.
// These globals are intentionally scoped to the package and protected by sync.Once.
// They're necessary for singleton pattern initialization of OTEL components.
//
//nolint:gochecknoglobals
var (
	once       sync.Once
	tracer     trace.Tracer
	enabled    bool
	noopTracer trace.Tracer
)

// initTracer initializes OTEL components on first use with sync.Once.
// This prevents uninitialized global state and satisfies gochecknoglobals.
func initTracer() {
	once.Do(func() {
		enabled = isOTELEnabled()
		tracer = otel.Tracer("tounilab.com/vessel")
		noopTracer = noop.NewTracerProvider().Tracer("noop")
	})
}

func isOTELEnabled() bool {
	envVal := os.Getenv("OTEL_ENABLED")
	if envVal == "" {
		return true // Enabled by default
	}
	return !strings.EqualFold(envVal, "false")
}

// UseTracer returns a span context and span if OTEL is enabled,
// otherwise returns a no-op span. This allows tracing to be easily toggled.
func UseTracer(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	initTracer()

	var resultCtx context.Context
	var resultSpan trace.Span

	if !enabled {
		resultCtx, resultSpan = noopTracer.Start(ctx, name, opts...)
	} else {
		resultCtx, resultSpan = tracer.Start(ctx, name, opts...)
	}

	return resultCtx, resultSpan
}
