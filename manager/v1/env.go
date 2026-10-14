package v1

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
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

// expand resolves one placeholder as written: to its variable's value, found in a
// source (fromSource), else to its default, else to itself. A value that is not valid
// UTF-8 is refused without being quoted: escaped by an encoder, its bytes would become
// U+FFFD and the manager would connect with another credential.
func (r *envResolver) expand(match []byte) (val string, fromSource bool, err error) {
	parts := envVarRe.FindSubmatch(match)
	key := string(parts[1])
	if val, ok := r.lookup(key); ok {
		if !utf8.ValidString(val) {
			return "", false, fmt.Errorf("the value of ${%s} is not valid UTF-8", key)
		}
		r.substituted = append(r.substituted, val)
		return val, true, nil
	}
	if parts[2] != nil {
		return string(parts[2]), false, nil
	}
	return string(match), false, nil // no source, no default → leave as-is
}

// placeholders stands a number in for each placeholder while a file parses, so no value
// ever meets the file's syntax (#85): a number is valid inside a string and as a bare
// value, in YAML, JSON and TOML alike. Values, defaults included, go into the parsed
// data afterwards, and an encoder, not this package, escapes them.
type placeholders struct {
	resolver *envResolver
	token    *regexp.Regexp    // a token: a prefix the file does not contain, then six digits
	byToken  map[string]string // token → the placeholder as written
}

// tokenize replaces each placeholder in data with a token. The same placeholder gets the
// same token, so a YAML alias still names its anchor.
func tokenize(data []byte, resolver *envResolver) ([]byte, *placeholders) {
	prefix := 700_000_000_000 // twelve digits, so a token fits an int64 and has no leading zero
	for bytes.Contains(data, []byte(strconv.Itoa(prefix))) {
		prefix++
	}
	p := &placeholders{
		resolver: resolver,
		token:    regexp.MustCompile(strconv.Itoa(prefix) + `\d{6}`),
		byToken:  map[string]string{},
	}
	byText := map[string]string{}
	return envVarRe.ReplaceAllFunc(data, func(match []byte) []byte {
		token, ok := byText[string(match)]
		if !ok {
			token = fmt.Sprintf("%d%06d", prefix, len(byText))
			byText[string(match)] = token
			p.byToken[token] = string(match)
		}
		return []byte(token)
	}), p
}

// value is what the placeholder behind token expands to.
func (p *placeholders) value(token string) (val string, fromSource bool, err error) {
	return p.resolver.expand([]byte(p.byToken[token]))
}

// expandText replaces each token in s with its value, and lists the placeholders whose
// value came from a source.
func (p *placeholders) expandText(s string) (string, []string, error) {
	var fromSource []string
	var firstErr error
	expanded := p.token.ReplaceAllStringFunc(s, func(token string) string {
		val, found, err := p.value(token)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if found {
			fromSource = append(fromSource, p.byToken[token])
		}
		return val
	})
	return expanded, fromSource, firstErr
}

// restore puts the placeholders back into a parser's message in place of their tokens.
func (p *placeholders) restore(msg string) string {
	return p.token.ReplaceAllStringFunc(msg, func(token string) string { return p.byToken[token] })
}

// decodeYAML parses the file with tokens standing in for placeholders, puts each value
// into the scalars holding its token, then decodes. An unquoted scalar is trimmed and
// typed from its expanded text, as if written there, so a port or a duration still
// decodes; a quoted one stays a string, as it is. A value that would read as null stays
// a string too, unless empty: a password "null" is a password.
func decodeYAML(data []byte, resolver *envResolver, out any) error {
	if !resolver.hasAnySource() {
		if err := yaml.Unmarshal(data, out); err != nil {
			return fmt.Errorf("parse yaml: %w", err)
		}
		return nil
	}
	tokenized, p := tokenize(data, resolver)
	var root yaml.Node
	if err := yaml.Unmarshal(tokenized, &root); err != nil {
		return fmt.Errorf("parse yaml: %s", p.restore(err.Error()))
	}
	if root.Kind == 0 {
		return nil // an empty file, which yaml.Unmarshal into a struct also accepts
	}
	lines := map[int][]string{}
	if err := p.expandYAML(&root, lines); err != nil {
		return err
	}
	if err := root.Decode(out); err != nil {
		return fmt.Errorf("decode yaml: %w", withholdYAML(err, lines))
	}
	return nil
}

// expandYAML puts each placeholder's value into the scalars under n that hold its token,
// and records the lines holding a value from a source.
func (p *placeholders) expandYAML(n *yaml.Node, lines map[int][]string) error {
	for _, child := range n.Content {
		if err := p.expandYAML(child, lines); err != nil {
			return err
		}
	}
	if token := p.token.FindString(n.Tag); token != "" {
		return fmt.Errorf("line %d: %s sits in a YAML tag, which cannot hold a placeholder", n.Line, p.byToken[token])
	}
	if n.Kind != yaml.ScalarNode || !p.token.MatchString(n.Value) {
		return nil
	}
	val, fromSource, err := p.expandText(n.Value)
	if err != nil {
		return err
	}
	for _, placeholder := range fromSource {
		addLine(lines, n.Line, placeholder)
	}
	n.Value = val
	if n.Style == 0 {
		retypePlain(n)
	}
	return nil
}

// retypePlain types an unquoted scalar from its expanded text, as if written there:
// trimmed, as YAML would trim it, and a string if it would read as null but is not
// empty.
func retypePlain(n *yaml.Node) {
	n.Value = strings.Trim(n.Value, " \t\r\n")
	n.Tag = ""
	if n.ShortTag() == "!!null" && n.Value != "" {
		n.Tag = "!!str"
	}
}

// withholdYAML replaces each message of a YAML type error that points at a line holding
// a substituted value: the decoder quotes up to seven characters of what it rejects,
// wherever the value sits in it.
func withholdYAML(err error, lines map[int][]string) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return err
	}
	for i, msg := range typeErr.Errors {
		var line int
		if _, scanErr := fmt.Sscanf(msg, "line %d:", &line); scanErr == nil && len(lines[line]) > 0 {
			typeErr.Errors[i] = withheld(fmt.Sprintf("line %d", line), lines[line])
		}
	}
	return err
}

// withheld is the message for a decoder error at a place holding the values of
// placeholders, which names them instead of quoting what the decoder read.
func withheld(at string, placeholders []string) string {
	return fmt.Sprintf("%s: the value of %s is not valid here", at, strings.Join(placeholders, ", "))
}

// addLine records that line, or key path, at holds the value of placeholder.
func addLine[K comparable](lines map[K][]string, at K, placeholder string) {
	if !slices.Contains(lines[at], placeholder) {
		lines[at] = append(lines[at], placeholder)
	}
}

// decodeJSON parses the file into a generic tree with tokens standing in for
// placeholders, puts the values in, and decodes the tree, written back by encoding/json.
func decodeJSON(data []byte, resolver *envResolver, out any) error {
	if !resolver.hasAnySource() {
		return wrapDecode("json", json.Unmarshal(data, out))
	}
	tokenized, p := tokenize(data, resolver)
	dec := json.NewDecoder(bytes.NewReader(tokenized))
	dec.UseNumber() // a token is a number too large for a float64
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return fmt.Errorf("parse json: %s", p.restore(err.Error()))
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("parse json: invalid character after top-level value")
	}
	paths := map[string][]string{}
	expanded, err := p.expandTree(tree, "", paths, jsonScalar)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(expanded)
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	if err := json.Unmarshal(encoded, out); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && len(paths[typeErr.Field]) > 0 {
			return errors.New(withheld(typeErr.Field, paths[typeErr.Field]))
		}
		return wrapDecode("json", err)
	}
	return nil
}

// decodeTOML is decodeJSON for TOML, written back by go-toml.
func decodeTOML(data []byte, resolver *envResolver, out any) error {
	if !resolver.hasAnySource() {
		return wrapDecode("toml", toml.Unmarshal(data, out))
	}
	tokenized, p := tokenize(data, resolver)
	var tree map[string]any
	if err := toml.Unmarshal(tokenized, &tree); err != nil {
		return fmt.Errorf("parse toml: %s", p.restore(err.Error()))
	}
	paths := map[string][]string{}
	expanded, err := p.expandTree(tree, "", paths, tomlScalar)
	if err != nil {
		return err
	}
	encoded, err := toml.Marshal(expanded)
	if err != nil {
		return fmt.Errorf("encode toml: %w", err)
	}
	if err := toml.Unmarshal(encoded, out); err != nil {
		var decodeErr *toml.DecodeError
		if errors.As(err, &decodeErr) {
			if at := strings.Join(decodeErr.Key(), "."); len(paths[at]) > 0 {
				return errors.New(withheld(at, paths[at]))
			}
		}
		return wrapDecode("toml", err)
	}
	return nil
}

func wrapDecode(format string, err error) error {
	if err != nil {
		return fmt.Errorf("decode %s: %w", format, err)
	}
	return nil
}

// expandTree puts placeholder values into a decoded JSON or TOML tree: as text into a
// string or a key, and as the value scalar types in place of a bare token. A token glued
// to other text outside quotes fails. paths records, per key path, the placeholders
// whose value from a source sits there, for decode errors to name.
func (p *placeholders) expandTree(v any, path string, paths map[string][]string, scalar func(string) any) (any, error) {
	switch v := v.(type) {
	case map[string]any:
		return p.expandMap(v, path, paths, scalar)
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			var err error
			if out[i], err = p.expandTree(child, path, paths, scalar); err != nil {
				return nil, err
			}
		}
		return out, nil
	case string:
		expanded, fromSource, err := p.expandText(v)
		for _, placeholder := range fromSource {
			addLine(paths, path, placeholder)
		}
		return expanded, err
	default:
		return p.expandBare(v, path, paths, scalar)
	}
}

// expandMap is expandTree for a table or an object, whose keys can hold values too.
func (p *placeholders) expandMap(
	m map[string]any, path string, paths map[string][]string, scalar func(string) any,
) (any, error) {
	out := make(map[string]any, len(m))
	for key, child := range m {
		key, fromSource, err := p.expandText(key)
		if err != nil {
			return nil, err
		}
		at := key
		if path != "" {
			at = path + "." + key
		}
		for _, placeholder := range fromSource {
			addLine(paths, at, placeholder)
		}
		if out[key], err = p.expandTree(child, at, paths, scalar); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// expandBare puts a value in place of a bare token: a number, a boolean or a date
// otherwise.
func (p *placeholders) expandBare(v any, path string, paths map[string][]string, scalar func(string) any) (any, error) {
	text := fmt.Sprint(v)
	token := p.token.FindString(text)
	if token == "" {
		return v, nil
	}
	if token != text {
		return nil, fmt.Errorf("%s: %s sits next to other text outside quotes: quote it", path, p.byToken[token])
	}
	val, fromSource, err := p.value(token)
	if err != nil {
		return nil, err
	}
	if fromSource {
		addLine(paths, path, p.byToken[token])
	}
	return scalar(val), nil
}

// jsonNumber is a JSON number literal.
var jsonNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// jsonScalar types a value placed outside quotes in JSON as if written there: a number,
// true, false or null, around which spaces don't count; anything else is a string, as
// it is, never structure.
func jsonScalar(val string) any {
	trimmed := strings.Trim(val, " \t\r\n")
	if b, ok := boolLiteral(trimmed); ok {
		return b
	}
	switch {
	case trimmed == "null":
		return nil
	case jsonNumber.MatchString(trimmed):
		return json.Number(trimmed)
	}
	return val
}

// tomlScalar is jsonScalar for TOML: an integer, with TOML's bases and underscores, true
// or false. ponytail: no float, as no config field is one; add it with the first.
func tomlScalar(val string) any {
	trimmed := strings.Trim(val, " \t\r\n")
	if i, err := strconv.ParseInt(trimmed, 0, 64); err == nil {
		return i
	}
	if b, ok := boolLiteral(trimmed); ok {
		return b
	}
	return val
}

// boolLiteral reads true or false, spelled as JSON and TOML spell them.
func boolLiteral(s string) (value, ok bool) {
	switch s {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// redactError is err as a new error with each substituted value redacted, or err itself
// when nothing was substituted. A new error rather than a wrapper: a decoder's error can
// hold what it read, as a TOML error holds its source.
func redactError(err error, substituted []string) error {
	if len(substituted) == 0 {
		return err
	}
	return errors.New(redact(err.Error(), substituted))
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
