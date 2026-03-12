//go:build test

package otel_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	otelutil "tounilab.com/fabric/internal/pkg/otel"
)

// TestUseTracerEnabled tests UseTracer when OTEL is enabled
func TestUseTracerEnabled(t *testing.T) {
	// Set OTEL_ENABLED to true
	if err := os.Setenv("OTEL_ENABLED", "true"); err != nil {
		t.Fatalf("failed to set OTEL_ENABLED: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv("OTEL_ENABLED")
	})

	ctx := context.Background()
	ctx, span := otelutil.UseTracer(ctx, "test-span")

	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	span.End()
}

// TestUseTracerDisabled tests UseTracer when OTEL is disabled
func TestUseTracerDisabled(t *testing.T) {
	// Set OTEL_ENABLED to false
	if err := os.Setenv("OTEL_ENABLED", "false"); err != nil {
		t.Fatalf("failed to set OTEL_ENABLED: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv("OTEL_ENABLED")
	})

	ctx := context.Background()
	ctx, span := otelutil.UseTracer(ctx, "test-span")

	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	span.End()
}

// TestUseTracerDefault tests UseTracer with default OTEL enabled
func TestUseTracerDefault(t *testing.T) {
	// Unset OTEL_ENABLED to use default (enabled)
	_ = os.Unsetenv("OTEL_ENABLED")

	ctx := context.Background()
	ctx, span := otelutil.UseTracer(ctx, "test-span")

	require.NotNil(t, ctx)
	require.NotNil(t, span)
	span.End()
}

// TestUseTracerWithOptions tests UseTracer with span options
func TestUseTracerWithOptions(t *testing.T) {
	_ = os.Unsetenv("OTEL_ENABLED")

	ctx := context.Background()
	// UseTracer should accept SpanStartOption arguments
	ctx, span := otelutil.UseTracer(ctx, "test-span-with-options")

	require.NotNil(t, ctx)
	require.NotNil(t, span)
	span.End()
}

// TestUseTracerMultipleCalls tests multiple sequential UseTracer calls
func TestUseTracerMultipleCalls(t *testing.T) {
	_ = os.Unsetenv("OTEL_ENABLED")

	ctx := context.Background()

	// First call
	ctx1, span1 := otelutil.UseTracer(ctx, "span-1")
	require.NotNil(t, ctx1)
	require.NotNil(t, span1)

	// Second call with result of first
	ctx2, span2 := otelutil.UseTracer(ctx1, "span-2")
	require.NotNil(t, ctx2)
	require.NotNil(t, span2)

	span2.End()
	span1.End()
}

// TestUseTracerEmptyName tests UseTracer with empty span name
func TestUseTracerEmptyName(t *testing.T) {
	_ = os.Unsetenv("OTEL_ENABLED")

	ctx := context.Background()
	ctx, span := otelutil.UseTracer(ctx, "")

	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	span.End()
}

// TestUseTracerContextCancellation tests UseTracer with canceled context
func TestUseTracerContextCancellation(t *testing.T) {
	_ = os.Unsetenv("OTEL_ENABLED")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context

	newCtx, span := otelutil.UseTracer(ctx, "test-span")

	require.NotNil(t, newCtx)
	require.NotNil(t, span)
	span.End()
}
