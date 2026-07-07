//go:build test

package v1

import (
	"reflect"
	"testing"
)

// namedString exercises the convertible-but-not-assignable string destination path.
type namedString string

// TestSetFieldFromValueNumericToString ensures numeric driver values scanned
// into string fields are formatted as decimal text, not rune-converted
// (reflect's int64→string conversion yields the Unicode code point, e.g.
// int64(65) → "A").
func TestSetFieldFromValueNumericToString(t *testing.T) {
	tests := []struct {
		name string
		cv   any
		want string
	}{
		{"int64", int64(65), "65"},
		{"int32", int32(97), "97"},
		{"int", int(120), "120"},
		{"uint64", uint64(66), "66"},
		{"float64", float64(1.5), "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst struct{ Field string }
			f := reflect.ValueOf(&dst).Elem().Field(0)

			if err := setFieldFromValue(f, tt.cv); err != nil {
				t.Fatalf("setFieldFromValue(%T) failed: %v", tt.cv, err)
			}
			if dst.Field != tt.want {
				t.Errorf("setFieldFromValue(%v) = %q; want %q", tt.cv, dst.Field, tt.want)
			}
		})
	}
}

// TestSetFieldFromValueStringDestinations ensures string-typed sources still
// reach string destinations, including named string types via conversion.
func TestSetFieldFromValueStringDestinations(t *testing.T) {
	t.Run("string to string", func(t *testing.T) {
		var dst struct{ Field string }
		f := reflect.ValueOf(&dst).Elem().Field(0)

		if err := setFieldFromValue(f, "hello"); err != nil {
			t.Fatalf("setFieldFromValue failed: %v", err)
		}
		if dst.Field != "hello" {
			t.Errorf("got %q; want %q", dst.Field, "hello")
		}
	})

	t.Run("bytes to string", func(t *testing.T) {
		var dst struct{ Field string }
		f := reflect.ValueOf(&dst).Elem().Field(0)

		if err := setFieldFromValue(f, []byte("world")); err != nil {
			t.Fatalf("setFieldFromValue failed: %v", err)
		}
		if dst.Field != "world" {
			t.Errorf("got %q; want %q", dst.Field, "world")
		}
	})

	t.Run("string to named string", func(t *testing.T) {
		var dst struct{ Field namedString }
		f := reflect.ValueOf(&dst).Elem().Field(0)

		if err := setFieldFromValue(f, "hello"); err != nil {
			t.Fatalf("setFieldFromValue failed: %v", err)
		}
		if dst.Field != "hello" {
			t.Errorf("got %q; want %q", dst.Field, "hello")
		}
	})

	t.Run("int64 to named string", func(t *testing.T) {
		var dst struct{ Field namedString }
		f := reflect.ValueOf(&dst).Elem().Field(0)

		if err := setFieldFromValue(f, int64(65)); err != nil {
			t.Fatalf("setFieldFromValue failed: %v", err)
		}
		if dst.Field != "65" {
			t.Errorf("got %q; want %q", dst.Field, "65")
		}
	})
}

// TestSetFieldFromValueNumericConversionsPreserved ensures the reflect.Convert
// fast path still applies for numeric widenings (int64 → int, int64 → float64).
func TestSetFieldFromValueNumericConversionsPreserved(t *testing.T) {
	t.Run("int64 to int", func(t *testing.T) {
		var dst struct{ Field int }
		f := reflect.ValueOf(&dst).Elem().Field(0)

		if err := setFieldFromValue(f, int64(42)); err != nil {
			t.Fatalf("setFieldFromValue failed: %v", err)
		}
		if dst.Field != 42 {
			t.Errorf("got %d; want 42", dst.Field)
		}
	})

	t.Run("int64 to float64", func(t *testing.T) {
		var dst struct{ Field float64 }
		f := reflect.ValueOf(&dst).Elem().Field(0)

		if err := setFieldFromValue(f, int64(7)); err != nil {
			t.Fatalf("setFieldFromValue failed: %v", err)
		}
		if dst.Field != 7 {
			t.Errorf("got %v; want 7", dst.Field)
		}
	})
}
