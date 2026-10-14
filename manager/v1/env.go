package v1

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
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
	// Values expansion took from the sources above, redacted from load errors
	substituted []string
	// Variables left unexpanded because their value is not valid UTF-8
	invalid []string
}

// WithEnvVars provides an explicit set of variables for expansion.
// Only these exact keys are available. This has the highest priority.
//
// Example:
//
//	vesselmgr.NewDBManager(ctx, path, logger,
//	    vesselmgr.WithEnvVars(map[string]string{
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
//	vesselmgr.NewDBManager(ctx, path, logger,
//	    vesselmgr.WithEnvPrefix("DB_", "VESSEL_"),
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
//	vesselmgr.NewDBManager(ctx, path, logger,
//	    vesselmgr.WithEnvFile(".env"),
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

// expandEnvVarsWith replaces ${VAR} and ${VAR:default} patterns in data
// using the resolver's lookup chain, each value found in a source passed through
// encode. A default goes in as written: it is already in the file's syntax.
//
// Behavior:
//   - ${VAR} with var found → replaced with value
//   - ${VAR} with var not found → left as literal ${VAR} (fail-loud)
//   - ${VAR:default} with var found → replaced with value
//   - ${VAR:default} with var not found → replaced with default
//   - ${VAR:} with var not found → replaced with empty string
func expandEnvVarsWith(data []byte, resolver *envResolver, encode func(string) string) []byte {
	if resolver == nil || !resolver.hasAnySource() {
		return data
	}

	return envVarRe.ReplaceAllFunc(data, func(match []byte) []byte {
		val, fromSource := resolver.expand(match)
		if fromSource {
			return []byte(encode(val))
		}
		return []byte(val)
	})
}

// expand resolves one placeholder as written: to its variable's value, found in a
// source (fromSource), else to its default, else to itself.
func (r *envResolver) expand(match []byte) (val string, fromSource bool) {
	parts := envVarRe.FindSubmatch(match)
	key := string(parts[1])
	if val, ok := r.lookup(key); ok {
		if !utf8.ValidString(val) {
			// Escaped, the bytes would become U+FFFD and the manager would connect
			// with another credential: refuse the value, without quoting it.
			r.invalid = append(r.invalid, key)
			return string(match), false
		}
		r.substituted = append(r.substituted, val)
		return val, true
	}
	if parts[2] != nil {
		return string(parts[2]), false
	}
	return string(match), false // no source, no default → leave as-is
}

// decodeYAML stands an inert token in for each placeholder while YAML parses the file,
// so the file parses as written wherever a placeholder sits (a flow collection, an
// anchor, a key), then puts each value into the scalars holding its token: a value can
// never become YAML syntax (#85). An unquoted scalar is then typed from its expanded
// text, as if written there, so a port or a duration still decodes; a quoted one stays
// a string. A value that would read as null stays a string too, unless empty: a
// password "null" is a password.
func decodeYAML(data []byte, resolver *envResolver, out any) error {
	tokens := &yamlTokens{byToken: map[string]yamlPlaceholder{}}
	if resolver.hasAnySource() {
		data = tokens.replace(data, resolver)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse yaml: %s", tokens.restore(err.Error()))
	}
	if root.Kind == 0 {
		return nil // an empty file, which yaml.Unmarshal into a struct also accepts
	}
	tokens.expand(&root)
	if err := root.Decode(out); err != nil {
		return fmt.Errorf("decode yaml: %w", err)
	}
	return nil
}

// yamlTokens maps the tokens standing in for placeholders while YAML parses a file.
type yamlTokens struct {
	re      *regexp.Regexp // matches a token; nil when no placeholder was replaced
	byToken map[string]yamlPlaceholder
}

type yamlPlaceholder struct {
	text string // as written, for error messages
	val  string // what it expands to
}

// replace swaps each placeholder in data for a token of letters and digits that data
// does not otherwise contain. The same placeholder gets the same token, so an alias
// still names its anchor.
func (t *yamlTokens) replace(data []byte, resolver *envResolver) []byte {
	prefix := "vesselenv"
	for bytes.Contains(data, []byte(prefix)) {
		prefix += "x"
	}
	t.re = regexp.MustCompile(prefix + `\d{6}`)
	byText := map[string]string{}
	return envVarRe.ReplaceAllFunc(data, func(match []byte) []byte {
		token, ok := byText[string(match)]
		if !ok {
			token = fmt.Sprintf("%s%06d", prefix, len(byText))
			byText[string(match)] = token
			val, _ := resolver.expand(match)
			t.byToken[token] = yamlPlaceholder{text: string(match), val: val}
		}
		return []byte(token)
	})
}

// restore puts the placeholders back into a parser message in place of their tokens.
func (t *yamlTokens) restore(msg string) string {
	if t.re == nil {
		return msg
	}
	return t.re.ReplaceAllStringFunc(msg, func(token string) string { return t.byToken[token].text })
}

// expand puts each placeholder's value into the scalars under n that hold its token.
func (t *yamlTokens) expand(n *yaml.Node) {
	for _, child := range n.Content {
		t.expand(child)
	}
	if t.re == nil || n.Kind != yaml.ScalarNode || !t.re.MatchString(n.Value) {
		return
	}
	n.Value = t.re.ReplaceAllStringFunc(n.Value, func(token string) string { return t.byToken[token].val })
	if n.Style == 0 {
		n.Tag = ""
		if n.ShortTag() == "!!null" && n.Value != "" {
			n.Tag = "!!str"
		}
	}
}

// escapeQuoted escapes val for a double-quoted JSON or TOML string, which read the
// same escapes, so the value cannot end the string. A value substituted outside
// quotes, like a port, has nothing to escape. A TOML single-quoted string reads no
// escapes, so a value placed there arrives escaped: use double quotes.
func escapeQuoted(val string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(val) // a string always encodes
	quoted := strings.TrimSuffix(b.String(), "\n")
	// JSON leaves DEL as it is; TOML requires it escaped.
	return strings.ReplaceAll(quoted[1:len(quoted)-1], "\x7f", `\u007f`)
}

// redact replaces each substituted value in msg, longest first so a value holding
// another is replaced whole: a decoder quotes the text it rejects, and that text may be
// a secret (#85). A short value can redact more than itself; a message with too much
// redacted is the safe side.
func redact(msg string, values []string) string {
	sorted := slices.Clone(values)
	slices.SortFunc(sorted, func(a, b string) int { return len(b) - len(a) })
	for _, val := range sorted {
		if val != "" {
			msg = strings.ReplaceAll(msg, val, "[REDACTED]")
		}
	}
	return msg
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
