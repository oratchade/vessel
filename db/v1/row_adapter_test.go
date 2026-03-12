//go:build test

package v1_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSQLNullStringConversion(t *testing.T) {
	testCases := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"nil value", nil, false},
		{"string value", "hello", true},
		{"bytes value", []byte("world"), true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var ns sql.NullString
			if tc.input == nil {
				ns.Valid = false
			} else if b, ok := tc.input.([]byte); ok {
				ns = sql.NullString{String: string(b), Valid: true}
			} else if s, ok := tc.input.(string); ok {
				ns = sql.NullString{String: s, Valid: true}
			}
			assert.Equal(t, tc.expected, ns.Valid)
		})
	}
}

func TestSQLNullInt64Conversion(t *testing.T) {
	testCases := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"nil value", nil, false},
		{"valid string", "123", true},
		{"zero", "0", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var ni sql.NullInt64
			if tc.input == nil {
				ni.Valid = false
			} else if s, ok := tc.input.(string); ok && s != "invalid" {
				ni = sql.NullInt64{Int64: 123, Valid: true}
			}
			assert.Equal(t, tc.expected, ni.Valid)
		})
	}
}

func TestSQLNullBoolConversion(t *testing.T) {
	testCases := []struct {
		name  string
		valid bool
	}{
		{"true", true},
		{"false", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var nb sql.NullBool
			nb.Valid = tc.valid
			assert.Equal(t, tc.valid, nb.Valid)
		})
	}
}

func TestSQLNullByteConversion(t *testing.T) {
	testCases := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"single byte", byte('a'), true},
		{"byte array", []byte("test"), true},
		{"nil", nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var nb sql.NullByte
			if tc.input == nil {
				nb.Valid = false
			} else {
				nb.Valid = true
			}
			assert.Equal(t, tc.expected, nb.Valid)
		})
	}
}

func TestSQLNullFloat64Conversion(t *testing.T) {
	testCases := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"valid float", "3.14", true},
		{"zero", "0.0", true},
		{"nil", nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var nf sql.NullFloat64
			if tc.input == nil {
				nf.Valid = false
			} else {
				nf.Valid = true
			}
			assert.Equal(t, tc.expected, nf.Valid)
		})
	}
}
