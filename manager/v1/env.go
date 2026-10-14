package v1

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// expand resolves one placeholder as written: to its variable's value, found in a
// source (fromSource), else to its default, else to itself.
func (r *envResolver) expand(match []byte) (val string, fromSource bool) {
	parts := envVarRe.FindSubmatch(match)
	key := string(parts[1])
	if val, ok := r.lookup(key); ok {
		if !utf8.ValidString(val) {
			// Escaped, the bytes would become U+FFFD and the manager would connect
			// with another credential: refuse the value, without quoting it.
			if !slices.Contains(r.invalid, key) {
				r.invalid = append(r.invalid, key)
			}
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

// decodeYAML stands an inert token in for each value from the environment while YAML
// parses the file, so the file parses as written wherever a placeholder sits (a flow
// collection, an anchor, a key), then puts each value into the scalars holding its
// token: a value can never become YAML syntax (#85). An unquoted scalar is then
// trimmed and typed from its expanded text, as if written there, so a port or a
// duration still decodes; a quoted one stays a string, as it is. A value that would
// read as null stays a string too, unless empty: a password "null" is a password. A
// default, written by the file's author, goes in before parsing as written, as in JSON
// and TOML.
func decodeYAML(data []byte, resolver *envResolver, out any) error {
	tokens := &yamlTokens{byToken: map[string]yamlPlaceholder{}, lines: map[int][]string{}}
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
		return fmt.Errorf("decode yaml: %w", withholdYAML(err, tokens.lines))
	}
	return nil
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
			typeErr.Errors[i] = withheld(line, lines[line])
		}
	}
	return err
}

// withheld is the message for a decoder error on a line holding the values of
// placeholders, which names them instead of quoting what the decoder read.
func withheld(line int, placeholders []string) string {
	return fmt.Sprintf("line %d: the value of %s is not valid here", line, strings.Join(placeholders, ", "))
}

// addLine records that line holds the value of placeholder.
func addLine(lines map[int][]string, line int, placeholder string) {
	if !slices.Contains(lines[line], placeholder) {
		lines[line] = append(lines[line], placeholder)
	}
}

// yamlTokens maps the tokens standing in for placeholders while YAML parses a file.
type yamlTokens struct {
	re      *regexp.Regexp // matches a token; nil when no placeholder was replaced
	byToken map[string]yamlPlaceholder
	lines   map[int][]string // placeholders whose value sits on each line
}

type yamlPlaceholder struct {
	text string // as written, for error messages
	val  string // the value from a source it expands to
}

// replace swaps each placeholder whose value comes from a source for a token of
// letters and digits, and puts any other in as it expands. A token starts with a random
// prefix, which neither the file nor a default next to the text around it can spell.
// The same placeholder gets the same token, so an alias still names its anchor.
func (t *yamlTokens) replace(data []byte, resolver *envResolver) []byte {
	prefix := "vesselenv" + rand.Text()
	t.re = regexp.MustCompile(prefix + `\d{6}`)
	byText := map[string]string{}
	return envVarRe.ReplaceAllFunc(data, func(match []byte) []byte {
		if token, ok := byText[string(match)]; ok {
			return []byte(token)
		}
		val, fromSource := resolver.expand(match)
		if !fromSource {
			return []byte(val) // a default, or the placeholder as written: file syntax
		}
		token := fmt.Sprintf("%s%06d", prefix, len(byText))
		byText[string(match)] = token
		t.byToken[token] = yamlPlaceholder{text: string(match), val: val}
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
	n.Value = t.re.ReplaceAllStringFunc(n.Value, func(token string) string {
		placeholder := t.byToken[token]
		addLine(t.lines, n.Line, placeholder.text)
		return placeholder.val
	})
	if n.Style == 0 {
		// Written there, YAML would drop the spaces and line breaks around it.
		n.Value = strings.Trim(n.Value, " \t\r\n")
		n.Tag = ""
		if n.ShortTag() == "!!null" && n.Value != "" {
			n.Tag = "!!str"
		}
	}
}

// expandInText expands the placeholders of a JSON or TOML file by where each one
// sits, so a value can never become syntax (#85):
//   - in a double-quoted string, the value is escaped for it;
//   - in a TOML single-quoted string, which reads no escapes, it goes in as it is, and
//     a value the string cannot hold fails the load;
//   - outside quotes, only a plain value goes in: a number, a boolean, a date, a word;
//   - in a TOML comment, the placeholder stays as written.
//
// A default goes in as written: it is already in the file's syntax.
func expandInText(data []byte, resolver *envResolver) ([]byte, map[int][]string, error) {
	x := &textExpansion{resolver: resolver, line: 1, openedAt: -1, lines: map[int][]string{}}
	if !resolver.hasAnySource() {
		return data, x.lines, nil
	}
	matches := envVarRe.FindAllIndex(data, -1)
	for i := 0; i < len(data); {
		for len(matches) > 0 && matches[0][0] < i {
			matches = matches[1:] // began inside an escape sequence: not a placeholder
		}
		if len(matches) > 0 && matches[0][0] == i {
			if err := x.placeholder(data[i:matches[0][1]]); err != nil {
				return nil, nil, err
			}
			i, matches = matches[0][1], matches[1:]
			continue
		}
		i += x.copyText(data[i:])
	}
	return x.out.Bytes(), x.lines, nil
}

// textExpansion is where expandInText is in a file, and what it has written.
type textExpansion struct {
	resolver *envResolver
	out      bytes.Buffer
	at       textPlace
	line     int // in the expanded text, where the decoder counts lines
	openedAt int // where the last multi-line string's content began in out
	lines    map[int][]string
}

// placeholder writes what match expands to where the text is now, and records the
// lines its value from a source spans.
func (x *textExpansion) placeholder(match []byte) error {
	// TOML drops a newline right after a multi-line string's opening quotes.
	val, fromSource, err := expandAt(x.at, x.out.Len() == x.openedAt, match, x.resolver)
	if err != nil {
		return err
	}
	// A value or default with line breaks moves the lines below it.
	breaks := strings.Count(val, "\n")
	if fromSource {
		for l := x.line; l <= x.line+breaks; l++ {
			addLine(x.lines, l, string(match))
		}
	}
	x.out.WriteString(val)
	x.line += breaks
	return nil
}

// copyText copies the first bytes of rest, notes where the next byte sits, and
// returns how many it copied.
func (x *textExpansion) copyText(rest []byte) int {
	n, next := scan(x.at, rest)
	x.out.Write(rest[:n])
	x.line += bytes.Count(rest[:n], []byte("\n"))
	if next != x.at && (next == multiBasicString || next == multiLiteralString) {
		x.openedAt = x.out.Len()
	}
	x.at = next
	return n
}

// decodeText expands a JSON or TOML file and decodes it into out. On a line holding a
// substituted value, the decoder's error gives way to one naming the placeholder:
// TOML quotes the first character of a value it rejects, either decoder a number.
func decodeText(data []byte, resolver *envResolver, isTOML bool, out any) error {
	expanded, lines, err := expandInText(data, resolver)
	if err != nil {
		return err
	}
	if isTOML {
		err = toml.Unmarshal(expanded, out)
	} else {
		err = json.Unmarshal(expanded, out)
	}
	if err == nil {
		return nil
	}
	if line := errorLine(err, expanded); len(lines[line]) > 0 {
		return errors.New(withheld(line, lines[line]))
	}
	return fmt.Errorf("decode: %w", err)
}

// errorLine is the line a JSON or TOML decoder's error points at, or 0 if none.
func errorLine(err error, data []byte) int {
	var tomlErr *toml.DecodeError
	if errors.As(err, &tomlErr) {
		row, _ := tomlErr.Position()
		return row
	}
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	var offset int64
	switch {
	case errors.As(err, &syntaxErr):
		offset = syntaxErr.Offset
	case errors.As(err, &typeErr):
		offset = typeErr.Offset
	default:
		return 0
	}
	// Offset counts the bytes read up to and including the one at fault: a syntax error
	// found on the line break after a value would count that line break.
	end := min(max(int(offset)-1, 0), len(data))
	return 1 + bytes.Count(data[:end], []byte("\n"))
}

// textPlace is where a byte of a JSON or TOML file sits.
type textPlace int

const (
	outside textPlace = iota
	comment
	basicString        // "..."
	multiBasicString   // """..."""
	literalString      // '...'
	multiLiteralString // '''...'''
)

// plainValue is what a value outside quotes may be: a word (a boolean, inf, a bare key)
// or a number, date or time, which alone may hold a dot: elsewhere a dot makes a dotted
// key, which reaches into another table.
var plainValue = regexp.MustCompile(`^[+-]?([A-Za-z0-9_-]+|[0-9][0-9A-Za-z_.:+-]*)$`)

func expandAt(at textPlace, opened bool, match []byte, resolver *envResolver) (string, bool, error) {
	if at == comment {
		return string(match), false, nil
	}
	val, fromSource := resolver.expand(match)
	if !fromSource {
		return val, false, nil
	}
	switch at {
	case basicString:
		return escapeQuoted(val), true, nil
	case multiBasicString:
		// After a line-ending backslash TOML drops whitespace up to the next character,
		// so leading spaces are escaped; tabs and line breaks already are.
		escaped := escapeQuoted(val)
		trimmed := strings.TrimLeft(escaped, " ")
		return strings.Repeat(`\u0020`, len(escaped)-len(trimmed)) + trimmed, true, nil
	}
	if why := unfit(at, opened, val); why != "" {
		return "", true, fmt.Errorf("%s sits %s", match, why)
	}
	return val, true, nil
}

// unfit says why val cannot go in as it is at place at, or "" when it can. opened is
// whether it would follow a multi-line string's opening quotes, after which TOML drops
// a newline.
func unfit(at textPlace, opened bool, val string) string {
	const useDoubleQuotes = "which cannot hold its value: use double quotes"
	switch at {
	case literalString:
		if strings.Contains(val, "'") || hasControl(val, "\t") {
			return "in a single-quoted TOML string, " + useDoubleQuotes
		}
	case multiLiteralString:
		if !fitsMultiLineLiteral(val, opened) {
			return "in a multi-line literal TOML string, " + useDoubleQuotes
		}
	default:
		if !plainValue.MatchString(val) {
			return "outside quotes, where only a number, a boolean, a date or a word can go: quote it"
		}
	}
	return ""
}

func fitsMultiLineLiteral(val string, opened bool) bool {
	leadingNewline := opened && strings.HasPrefix(strings.TrimPrefix(val, "\r"), "\n")
	return !strings.Contains(val, "'''") && !hasControl(val, "\t\n\r") && !leadingNewline
}

// hasControl reports whether val holds a control character other than those allowed.
func hasControl(val, allowed string) bool {
	return strings.ContainsFunc(val, func(r rune) bool {
		return (r < 0x20 || r == 0x7f) && !strings.ContainsRune(allowed, r)
	})
}

// A transition moves to place next on reading prefix, which is width bytes long.
type transition struct {
	prefix string
	next   textPlace
	width  int // closesRun: the whole run of quotes that ends a multi-line string
}

const closesRun = 0

// transitions lists what leaves place at, in TOML. JSON is read the same way: what
// only TOML has (comments, single quotes, triple quotes) never sits outside a string in
// valid JSON.
func transitions(at textPlace) []transition {
	switch at {
	case outside:
		return []transition{
			{"#", comment, 1},
			{`"""`, multiBasicString, 3},
			{`"`, basicString, 1},
			{"'''", multiLiteralString, 3},
			{"'", literalString, 1},
		}
	case comment:
		return []transition{{"\n", outside, 1}}
	case basicString:
		return []transition{{`\`, basicString, 2}, {`"`, outside, 1}, {"\n", outside, 1}}
	case multiBasicString:
		return []transition{{`\`, multiBasicString, 2}, {`"""`, outside, closesRun}}
	case literalString:
		return []transition{{"'", outside, 1}, {"\n", outside, 1}}
	default:
		return []transition{{"'''", outside, closesRun}}
	}
}

// scan reads the first bytes of rest, at place at, and returns how many it read and
// where the next byte sits.
func scan(at textPlace, rest []byte) (int, textPlace) {
	for _, t := range transitions(at) {
		if bytes.HasPrefix(rest, []byte(t.prefix)) {
			width := t.width
			if width == closesRun {
				width = closingRun(rest, t.prefix[0])
			}
			return min(width, len(rest)), t.next
		}
	}
	return 1, at
}

// closingRun is the length of the run of quotes that closes a multi-line string: up to
// two quotes before the closing three belong to the string.
func closingRun(rest []byte, quote byte) int {
	n := 0
	for n < len(rest) && n < 5 && rest[n] == quote {
		n++
	}
	return n
}

// escapeQuoted escapes val for a double-quoted JSON or TOML string, which read the
// same escapes, so the value cannot end the string.
func escapeQuoted(val string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(val) // a string always encodes
	quoted := strings.TrimSuffix(b.String(), "\n")
	// JSON leaves DEL as it is; TOML requires it escaped.
	return strings.ReplaceAll(quoted[1:len(quoted)-1], "\x7f", `\u007f`)
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
