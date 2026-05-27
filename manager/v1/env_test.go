//go:build test

package v1_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/vessel/manager/v1"
)

func TestExpandEnvVars_WithEnvVars(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		vars     map[string]string
		expected string
	}{
		{
			name:     "simple substitution",
			input:    "host: ${DB_HOST}",
			vars:     map[string]string{"DB_HOST": "localhost"},
			expected: "host: localhost",
		},
		{
			name:     "with default, var present",
			input:    "host: ${DB_HOST:fallback}",
			vars:     map[string]string{"DB_HOST": "prod-db"},
			expected: "host: prod-db",
		},
		{
			name:     "with default, var missing",
			input:    "host: ${DB_HOST:fallback}",
			vars:     map[string]string{},
			expected: "host: fallback",
		},
		{
			name:     "empty default, var missing",
			input:    "password: ${DB_PASSWORD:}",
			vars:     map[string]string{},
			expected: "password: ",
		},
		{
			name:     "no default, var missing → left as-is",
			input:    "host: ${UNDEFINED_VAR}",
			vars:     map[string]string{},
			expected: "host: ${UNDEFINED_VAR}",
		},
		{
			name:     "multiple vars",
			input:    "${DB_HOST}:${DB_PORT:5432}",
			vars:     map[string]string{"DB_HOST": "myhost"},
			expected: "myhost:5432",
		},
		{
			name:     "bare $VAR is not expanded",
			input:    "host: $DB_HOST",
			vars:     map[string]string{"DB_HOST": "localhost"},
			expected: "host: $DB_HOST",
		},
		{
			name:     "no vars configured → no expansion",
			input:    "host: ${DB_HOST:default}",
			vars:     nil,
			expected: "host: ${DB_HOST:default}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var opts []v1.EnvOption
			if tc.vars != nil {
				opts = append(opts, v1.WithEnvVars(tc.vars))
			}
			result := v1.ExpandEnvVarsForTest([]byte(tc.input), opts)
			assert.Equal(t, tc.expected, string(result))
		})
	}
}

func TestExpandEnvVars_WithEnvPrefix(t *testing.T) {
	// Set a process env var for this test
	t.Setenv("DB_TEST_HOST", "from-env")
	t.Setenv("SECRET_KEY", "should-not-leak")

	testCases := []struct {
		name     string
		input    string
		prefixes []string
		expected string
	}{
		{
			name:     "prefix match → expanded",
			input:    "host: ${DB_TEST_HOST}",
			prefixes: []string{"DB_"},
			expected: "host: from-env",
		},
		{
			name:     "prefix mismatch → not expanded",
			input:    "key: ${SECRET_KEY}",
			prefixes: []string{"DB_"},
			expected: "key: ${SECRET_KEY}",
		},
		{
			name:     "prefix mismatch with default → uses default",
			input:    "key: ${SECRET_KEY:safe-default}",
			prefixes: []string{"DB_"},
			expected: "key: safe-default",
		},
		{
			name:     "multiple prefixes",
			input:    "host: ${DB_TEST_HOST}, key: ${SECRET_KEY}",
			prefixes: []string{"DB_", "SECRET_"},
			expected: "host: from-env, key: should-not-leak",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := []v1.EnvOption{v1.WithEnvPrefix(tc.prefixes...)}
			result := v1.ExpandEnvVarsForTest([]byte(tc.input), opts)
			assert.Equal(t, tc.expected, string(result))
		})
	}
}

func TestExpandEnvVars_WithEnvFile(t *testing.T) {
	// Create temp env file
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	content := `# Comment line
DB_FILE_HOST=filehost
DB_FILE_PORT=9999
QUOTED_VAR="quoted-value"
SINGLE_QUOTED='single'
`
	err := os.WriteFile(envFile, []byte(content), 0o600)
	require.NoError(t, err)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "file var found",
			input:    "host: ${DB_FILE_HOST}",
			expected: "host: filehost",
		},
		{
			name:     "file var with port",
			input:    "port: ${DB_FILE_PORT:5432}",
			expected: "port: 9999",
		},
		{
			name:     "file var not found, uses default",
			input:    "ssl: ${DB_FILE_SSL:disable}",
			expected: "ssl: disable",
		},
		{
			name:     "quoted value stripped",
			input:    "val: ${QUOTED_VAR}",
			expected: "val: quoted-value",
		},
		{
			name:     "single quoted value stripped",
			input:    "val: ${SINGLE_QUOTED}",
			expected: "val: single",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := []v1.EnvOption{v1.WithEnvFile(envFile)}
			result := v1.ExpandEnvVarsForTest([]byte(tc.input), opts)
			assert.Equal(t, tc.expected, string(result))
		})
	}
}

func TestExpandEnvVars_PriorityOrder(t *testing.T) {
	// Explicit vars > file vars > prefix env
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	err := os.WriteFile(envFile, []byte("DB_HOST=from-file\n"), 0o600)
	require.NoError(t, err)

	t.Setenv("DB_HOST", "from-process-env")

	opts := []v1.EnvOption{
		v1.WithEnvVars(map[string]string{"DB_HOST": "from-explicit"}),
		v1.WithEnvFile(envFile),
		v1.WithEnvPrefix("DB_"),
	}

	result := v1.ExpandEnvVarsForTest([]byte("host: ${DB_HOST}"), opts)
	assert.Equal(t, "host: from-explicit", string(result))
}

func TestExpandEnvVars_NoOptions_NoExpansion(t *testing.T) {
	t.Setenv("DB_HOST", "should-not-appear")

	result := v1.ExpandEnvVarsForTest([]byte("host: ${DB_HOST:default}"), nil)
	// Secure by default: no options → no expansion, but defaults still apply? No — no expansion at all.
	assert.Equal(t, "host: ${DB_HOST:default}", string(result))
}

func TestExpandEnvVars_MissingEnvFile_Ignored(t *testing.T) {
	opts := []v1.EnvOption{v1.WithEnvFile("/nonexistent/.env")}
	result := v1.ExpandEnvVarsForTest([]byte("host: ${DB_HOST:fallback}"), opts)
	// File load fails silently, no file vars available, default used
	assert.Equal(t, "host: ${DB_HOST:fallback}", string(result))
}
