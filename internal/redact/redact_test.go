//go:build test

package redact

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"syscall"
	"testing"
)

// TestError_KeepsCausesThatCarryNoInput: a caller retries on an errno or a
// context error; those reach it, and nothing else of the chain does.
func TestError_KeepsCausesThatCarryNoInput(t *testing.T) {
	for _, cause := range []error{syscall.ECONNREFUSED, context.DeadlineExceeded, context.Canceled} {
		err := Error(fmt.Errorf("dial s3cret: %w", cause), []string{"s3cret"})
		if !errors.Is(err, cause) || strings.Contains(err.Error(), "s3cret") {
			t.Errorf("Error(%v) = %v; want the cause kept and the value redacted", cause, err)
		}
	}
	var wrapped *url.Error
	if errors.As(Error(&url.Error{Op: "parse", URL: "s3cret", Err: syscall.EINVAL}, []string{"s3cret"}), &wrapped) {
		t.Error("the redacted error reaches a cause that holds the value")
	}
}

// TestForms_CoverAQueryEscapedValue: a connection string's parameter holds a value
// query-escaped.
func TestForms_CoverAQueryEscapedValue(t *testing.T) {
	if got := Message("sslmode=secret+mode%26x", Forms([]string{"secret mode&x"})); got != "sslmode=[REDACTED]" {
		t.Errorf("Message = %q, want sslmode=[REDACTED]", got)
	}
}

type withPassword struct{ Password string }

type withoutPassword struct{ User string }

// TestPassword: a config's Password field, by value or pointer; none for a nil pointer,
// a struct without one, or a value that is not a struct.
func TestPassword(t *testing.T) {
	for cfg, want := range map[any]string{
		withPassword{"s3cret"}: "s3cret", &withPassword{"s3cret"}: "s3cret",
		(*withPassword)(nil): "", withoutPassword{"app"}: "", "s3cret": "", 42: "",
	} {
		if got := Password(cfg); got != want {
			t.Errorf("Password(%#v) = %q, want %q", cfg, got, want)
		}
	}
}
