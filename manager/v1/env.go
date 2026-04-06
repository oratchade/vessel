package v1

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// envVarRe matches ${VAR} and ${VAR:default} patterns.
// Only braced syntax is recognized — bare $VAR is never expanded.
var envVarRe = regexp.MustCompile(`\$\{([^:}]+)(?::([^}]*))?\}`)

// EnvOption configures how environment variables are resolved during config loading.
// Multiple options can be combined; they are evaluated in order of precedence:
//
//  1. WithEnvVars (explicit map — highest priority)
//  2. WithEnvFile (file-scoped vars)
//  3. WithEnvPrefix (filtered process env — lowest priority)
//
// If no option is provided, no expansion occurs (secure by default).
type EnvOption func(*envResolver)

// envResolver holds the lookup chain built from EnvOptions.
type envResolver struct {
	// Explicit key→value pairs (highest priority)
	vars map[string]string
	// Allowed prefixes for process env lookup
	prefixes []string
	// File-scoped vars (loaded from .env files)
	fileVars map[string]string
}

// WithEnvVars provides an explicit set of variables for expansion.
// Only these exact keys are available. This has the highest priority.
//
// Example:
//
//	fabricmgr.NewDBManager(ctx, path, logger,
//	    fabricmgr.WithEnvVars(map[string]string{
//	        "DB_HOST":     "localhost",
//	        "DB_PASSWORD": os.Getenv("DB_PASSWORD"),
//	    }),
//	)
func WithEnvVars(vars map[string]string) EnvOption {
	return func(r *envResolver) {
		r.vars = vars
	}
}

// WithEnvPrefix allows expansion of process environment variables
// whose names start with any of the given prefixes.
// Variables not matching any prefix are never resolved from the process env.
//
// Example:
//
//	fabricmgr.NewDBManager(ctx, path, logger,
//	    fabricmgr.WithEnvPrefix("DB_", "FABRIC_"),
//	)
func WithEnvPrefix(prefixes ...string) EnvOption {
	return func(r *envResolver) {
		r.prefixes = prefixes
	}
}

// WithEnvFile loads variables from a key=value file (e.g. .env).
// Lines starting with # are comments. Empty lines are skipped.
// These variables are isolated from the process environment.
//
// Example:
//
//	fabricmgr.NewDBManager(ctx, path, logger,
//	    fabricmgr.WithEnvFile(".env"),
//	)
func WithEnvFile(path string) EnvOption {
	return func(r *envResolver) {
		vars, err := parseEnvFile(path)
		if err == nil {
			r.fileVars = vars
		}
		// Silently ignore missing/unreadable env files — defaults still apply
	}
}

// newEnvResolver creates a resolver from the provided options.
func newEnvResolver(opts []EnvOption) *envResolver {
	r := &envResolver{}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// lookup resolves a variable name through the priority chain:
// explicit vars → file vars → prefix-filtered process env.
// Returns the value and whether it was found.
func (r *envResolver) lookup(key string) (string, bool) {
	// 1. Explicit vars (highest priority)
	if r.vars != nil {
		if val, ok := r.vars[key]; ok {
			return val, true
		}
	}

	// 2. File-scoped vars
	if r.fileVars != nil {
		if val, ok := r.fileVars[key]; ok {
			return val, true
		}
	}

	// 3. Prefix-filtered process env
	if len(r.prefixes) > 0 {
		for _, prefix := range r.prefixes {
			if strings.HasPrefix(key, prefix) {
				if val, ok := os.LookupEnv(key); ok {
					return val, true
				}
				return "", false
			}
		}
	}

	return "", false
}

// hasAnySource returns true if at least one env option was configured.
func (r *envResolver) hasAnySource() bool {
	return r.vars != nil || r.fileVars != nil || len(r.prefixes) > 0
}

// expandEnvVars replaces ${VAR} and ${VAR:default} patterns in data
// using the resolver's lookup chain.
//
// Behavior:
//   - ${VAR} with var found → replaced with value
//   - ${VAR} with var not found → left as literal ${VAR} (fail-loud)
//   - ${VAR:default} with var found → replaced with value
//   - ${VAR:default} with var not found → replaced with default
//   - ${VAR:} with var not found → replaced with empty string
func expandEnvVars(data []byte, resolver *envResolver) []byte {
	if resolver == nil || !resolver.hasAnySource() {
		return data
	}

	return envVarRe.ReplaceAllFunc(data, func(match []byte) []byte {
		parts := envVarRe.FindSubmatch(match)
		key := string(parts[1])
		hasDefault := parts[2] != nil
		defaultVal := parts[2]

		if val, ok := resolver.lookup(key); ok {
			return []byte(val)
		}
		if hasDefault {
			return defaultVal
		}
		return match // no source, no default → leave as-is
	})
}

// stripQuotes removes surrounding single or double quotes from a value.
func stripQuotes(val string) string {
	if len(val) < 2 {
		return val
	}
	if (val[0] == '"' && val[len(val)-1] == '"') ||
		(val[0] == '\'' && val[len(val)-1] == '\'') {
		return val[1 : len(val)-1]
	}
	return val
}

// parseEnvFile reads a key=value file. Lines starting with # are comments.
// Values can be optionally quoted with single or double quotes.
func parseEnvFile(path string) (map[string]string, error) {
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("parseEnvFile: invalid path contains directory traversal")
	}

	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open env file: %w", err)
	}
	defer func() { _ = f.Close() }()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = stripQuotes(val)

		vars[key] = val
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read env file: %w", err)
	}

	return vars, nil
}
