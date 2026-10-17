package v1

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"

	db "tounilab.com/vessel/db/v1"
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
// source (fromSource), else to its default, else to itself. A value or a default that
// is not valid UTF-8 is refused without being quoted: escaped by an encoder, its bytes
// would change and the manager would connect with another credential.
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
		if !utf8.Valid(parts[2]) {
			return "", false, fmt.Errorf("the default of ${%s} is not valid UTF-8", key)
		}
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
	token    *regexp.Regexp    // a token: a prefix the file neither holds nor decodes to, then six digits
	byToken  map[string]string // token → the placeholder as written
}

// tokenizeDecoded tokenizes data for parse, and checks the parser takes each token
// whole: data is parsed with stand-ins too, each a token with every digit shifted
// (0 to 1, ..., 9 to 0), and both parses must read the same once each is read back as
// its placeholder. Where an escape or a number around a token reads part of it, what is
// left of the token differs in every digit between the two, even when what the escape
// made of its part does not. Text in the file can also spell a token, as an escape or a
// TOML integer such as 700_000_000_000_000_000 does: then tokens are chosen once more,
// clear of what both parses read, and only a second difference fails the load.
func tokenizeDecoded(
	data []byte, resolver *envResolver, parse func([]byte) (any, error),
) ([]byte, *placeholders, error) {
	avoid := data
	var p *placeholders
	var read string
	for range 2 {
		prefix := tokenPrefix(avoid)
		standIn, first := tokenize(data, resolver, shiftDigits(prefix))
		decoded, err := parse(standIn)
		if err != nil {
			return nil, first, err
		}
		var tokenized []byte
		tokenized, p = tokenize(data, resolver, prefix)
		again, err := parse(tokenized)
		if err != nil {
			return nil, p, err
		}
		if read = fmt.Sprint(p.readBack(again)); read == fmt.Sprint(first.readBack(decoded)) {
			return tokenized, p, nil
		}
		avoid = fmt.Appendf(avoid, "%v %v", decoded, again)
	}
	return nil, p, p.unreadable(read)
}

// tokenPrefix is the first prefix from 700000000000 found in avoid neither as it is nor
// with its digits shifted, and holding its first digit nowhere else either way, so text
// before a token cannot run into it to spell another. Twelve digits, so a token fits an
// int64 and has no leading zero.
func tokenPrefix(avoid []byte) string {
	for prefix := 700_000_000_000; ; prefix++ {
		text := strconv.Itoa(prefix)
		if spelled := func(t string) bool {
			return bytes.Contains(avoid, []byte(t)) || strings.Count(t, t[:1]) > 1
		}; !spelled(text) && !spelled(shiftDigits(text)) {
			return text
		}
	}
}

// shiftDigits is digits with each one shifted: 0 to 1, ..., 8 to 9, 9 to 0.
func shiftDigits(digits string) string {
	return strings.Map(func(r rune) rune { return '0' + (r-'0'+1)%10 }, digits)
}

// readBack is v with each token in its keys and values read back as its placeholder.
func (p *placeholders) readBack(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			out[p.restore(key)] = p.readBack(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = p.readBack(child)
		}
		return out
	}
	return p.restore(fmt.Sprint(v))
}

// unreadable is the error for a file whose two parses differ. It names the placeholders
// missing from what was read: one the syntax read part of is no longer there.
func (p *placeholders) unreadable(read string) error {
	var lost []string
	for _, placeholder := range p.byToken {
		if !strings.Contains(read, placeholder) {
			lost = append(lost, placeholder)
		}
	}
	if len(lost) == 0 {
		return errors.New("text in the file reads as the stand-in for a placeholder")
	}
	slices.Sort(lost)
	return fmt.Errorf("%s: the file's syntax reads part of the placeholder there, as an escape "+
		"or a number around it does, so no value can go there", strings.Join(lost, ", "))
}

// tokenize replaces each placeholder in data with a token: prefix, then six digits, all
// shifted when prefix is a shifted one. The same placeholder gets the same token, so a
// YAML alias still names its anchor.
func tokenize(data []byte, resolver *envResolver, prefix string) ([]byte, *placeholders) {
	p := &placeholders{
		resolver: resolver,
		token:    regexp.MustCompile(prefix + `\d{6}`),
		byToken:  map[string]string{},
	}
	shifted := prefix[0] != '7'
	byText := map[string]string{}
	return envVarRe.ReplaceAllFunc(data, func(match []byte) []byte {
		token, ok := byText[string(match)]
		if !ok {
			token = fmt.Sprintf("%s%06d", prefix, len(byText))
			if shifted {
				token = prefix + shiftDigits(token[len(prefix):])
			}
			byText[string(match)] = token
			p.byToken[token] = string(match)
		}
		return []byte(token)
	}), p
}

// value is what the placeholder behind token expands to.
func (p *placeholders) value(token string) (val string, fromSource bool, err error) {
	placeholder, ok := p.byToken[token]
	if !ok {
		return token, false, nil // digits no placeholder stands behind
	}
	return p.resolver.expand([]byte(placeholder))
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
	tokenized, p, err := tokenizeDecoded(data, resolver, func(standIn []byte) (any, error) {
		var root yaml.Node
		err := yaml.Unmarshal(standIn, &root)
		return scalarTexts(&root), err
	})
	if err != nil {
		return fmt.Errorf("parse yaml: %s", p.restore(err.Error()))
	}
	var root yaml.Node
	if err := yaml.Unmarshal(tokenized, &root); err != nil {
		return fmt.Errorf("parse yaml: %s", p.restore(err.Error()))
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

// scalarTexts lists the text of n and each node under it: what expandYAML reads, with
// escapes decoded and no tag resolved, as a stand-in token is no value a tag accepts.
func scalarTexts(n *yaml.Node) []string {
	texts := append(make([]string, 0, 1+len(n.Content)), n.Value)
	for _, child := range n.Content {
		texts = append(texts, scalarTexts(child)...)
	}
	return texts
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
	p.redactDecoded(n)
	if n.Style == 0 {
		retypePlain(n)
	}
	return nil
}

// redactDecoded records for redaction the value a !!binary scalar hands its field: its
// base64 text decoded, a form the environment never held.
func (p *placeholders) redactDecoded(n *yaml.Node) {
	if decoded, err := base64.StdEncoding.DecodeString(n.Value); err == nil && n.ShortTag() == "!!binary" {
		p.resolver.substituted = append(p.resolver.substituted, string(decoded))
	}
}

// retypePlain types an unquoted scalar from its expanded text, as if written there:
// trimmed, as YAML would trim it, and a string if it would read as null but is not
// empty, or as the merge key <<, which would pull another mapping's fields in.
func retypePlain(n *yaml.Node) {
	n.Value = strings.Trim(n.Value, " \t\r\n")
	n.Tag = ""
	// yaml.v3 merges at an untagged <<, though ShortTag calls it a string.
	if n.Value == "<<" || n.ShortTag() == "!!null" && n.Value != "" {
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
	tokenized, p, err := tokenizeDecoded(data, resolver, parseJSON)
	if err != nil {
		return err
	}
	tree, err := parseJSON(tokenized)
	if err != nil {
		return err
	}
	if key, repeated := repeatedJSONKey(json.NewDecoder(bytes.NewReader(tokenized))); repeated {
		return fmt.Errorf("parse json: key %q written twice in one object", p.restore(key))
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
		return withholdJSON(err, paths)
	}
	return nil
}

// withholdJSON replaces a type error at a key holding a substituted value with one
// naming its placeholders.
func withholdJSON(err error, paths map[string][]string) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		// JSON matches a key to its field ignoring case, so PORT is the port.
		field := withoutIndices(typeErr.Field)
		for at, placeholders := range paths {
			if strings.EqualFold(at, field) {
				return errors.New(withheld(field, placeholders))
			}
		}
	}
	return wrapDecode("json", err)
}

// withoutIndices is a decoder's field path as paths records it, without array indices:
// Go 1.27 writes entries.0.postgres.port where earlier releases write
// entries.postgres.port.
func withoutIndices(field string) string {
	return strings.Join(slices.DeleteFunc(strings.Split(field, "."), func(segment string) bool {
		_, err := strconv.Atoi(segment)
		return err == nil
	}), ".")
}

// parseJSON parses one JSON value into a generic tree.
func parseJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // a token is a number too large for a float64
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("parse json: invalid character after top-level value")
	}
	return tree, nil
}

// repeatedJSONKey reads one value from dec, which parseJSON has read whole, and finds a
// key written twice in one object within it. Decoding into the config merged two
// objects under one key; the generic tree keeps only the last of them.
func repeatedJSONKey(dec *json.Decoder) (string, bool) {
	open, _ := dec.Token()
	if open != json.Delim('{') && open != json.Delim('[') {
		return "", false
	}
	keys := map[string]bool{}
	for dec.More() {
		if open == json.Delim('{') {
			token, _ := dec.Token()
			key, _ := token.(string)
			if keys[key] {
				return key, true
			}
			keys[key] = true
		}
		if key, repeated := repeatedJSONKey(dec); repeated {
			return key, true
		}
	}
	_, _ = dec.Token() // the closing delimiter
	return "", false
}

// decodeTOML is decodeJSON for TOML, written back by go-toml.
func decodeTOML(data []byte, resolver *envResolver, out any) error {
	if !resolver.hasAnySource() {
		return wrapDecode("toml", toml.Unmarshal(data, out))
	}
	tokenized, p, err := tokenizeDecoded(data, resolver, parseTOML)
	if err != nil {
		return err
	}
	tree, err := parseTOML(tokenized)
	if err != nil {
		return err
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

// parseTOML parses a TOML document into a generic tree.
func parseTOML(data []byte) (any, error) {
	var tree map[string]any
	if err := toml.Unmarshal(data, &tree); err != nil {
		return nil, fmt.Errorf("parse toml: %w", err)
	}
	return tree, nil
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
	asWritten := map[string]string{} // expanded key → the key as written, placeholders restored
	for written, child := range m {
		key, fromSource, err := p.expandText(written)
		if err != nil {
			return nil, err
		}
		// JSON and TOML match a key to its field ignoring case, and map order is random,
		// so of two keys naming one field either could win: refuse them.
		for other, earlier := range asWritten {
			if strings.EqualFold(other, key) {
				return nil, fmt.Errorf("%s: keys %s and %s name the same field",
					cmp.Or(path, "top level"), earlier, p.restore(written))
			}
		}
		asWritten[key] = p.restore(written)
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

// expandBare puts a value in place of a bare token, typed by scalar; any other bare
// value is left as it is.
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

// tomlInteger is a TOML integer literal: no leading zero, lowercase base prefixes, no
// sign on a prefixed one, and an underscore only between two digits.
var tomlInteger = regexp.MustCompile(
	`^([+-]?(0|[1-9](_?[0-9])*)|0x[0-9A-Fa-f](_?[0-9A-Fa-f])*|0o[0-7](_?[0-7])*|0b[01](_?[01])*)$`)

// tomlScalar is jsonScalar for TOML: an integer as TOML writes one, true or false.
// ponytail: no float, as no config field is one; add it with the first.
func tomlScalar(val string) any {
	trimmed := strings.Trim(val, " \t\r\n")
	if tomlInteger.MatchString(trimmed) {
		// Go reads every literal tomlInteger matches, with the same value.
		if i, err := strconv.ParseInt(trimmed, 0, 64); err == nil {
			return i
		}
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

// redactError is err with each of values redacted, or err itself when there are none. It
// is a new error rather than a wrapper, since a decoder's error can hold what it read,
// as a TOML error holds its source; it unwraps only to the causes that carry no input.
func redactError(err error, values []string) error {
	if len(values) == 0 {
		return err
	}
	return redacted(err, values)
}

// redacted is err as a redactedError, its message redacted of values.
func redacted(err error, values []string) error {
	redacted := &redactedError{msg: redact(err.Error(), redactions(values))}
	if errno := syscall.Errno(0); errors.As(err, &errno) {
		redacted.causes = append(redacted.causes, errno)
	}
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		if errors.Is(err, cause) {
			redacted.causes = append(redacted.causes, cause)
		}
	}
	return redacted
}

// redactedError is an error's message redacted. Its causes are what a caller retries
// on, an errno or a context error, which hold no value.
type redactedError struct {
	msg    string
	causes []error
}

func (e *redactedError) Error() string   { return e.msg }
func (e *redactedError) Unwrap() []error { return e.causes }

// redactions are the forms of the substituted values to replace in a message, longest
// first so a value holding another is replaced whole: a decoder quotes the text it
// rejects, and that text may be a secret (#85). A short value can redact more than
// itself; a message with too much redacted is the safe side.
func redactions(values []string) []string {
	forms := make([]string, 0, len(values))
	for _, val := range values {
		forms = append(forms, redactedForms(val)...)
	}
	forms = slices.DeleteFunc(forms, func(form string) bool { return form == "" })
	slices.SortFunc(forms, func(a, b string) int { return len(b) - len(a) })
	return forms
}

// redact replaces each of forms in msg, in order.
func redact(msg string, forms []string) string {
	for _, form := range forms {
		msg = strings.ReplaceAll(msg, form, "[REDACTED]")
	}
	return msg
}

// redactingLogger redacts the config's passwords from every message and field it logs.
// The manager hands its logger to the drivers and entries, which log their own errors
// for as long as the manager runs, a connection string among them.
type redactingLogger struct {
	logger db.Logger
	forms  []string
}

func (l redactingLogger) Debug(msg string, args ...any) {
	l.logger.Debug(redact(msg, l.forms), l.redactAll(args)...)
}

func (l redactingLogger) Info(msg string, args ...any) {
	l.logger.Info(redact(msg, l.forms), l.redactAll(args)...)
}

func (l redactingLogger) Warn(msg string, args ...any) {
	l.logger.Warn(redact(msg, l.forms), l.redactAll(args)...)
}

func (l redactingLogger) Error(msg string, args ...any) {
	l.logger.Error(redact(msg, l.forms), l.redactAll(args)...)
}

func (l redactingLogger) With(fields ...any) db.Logger {
	return redactingLogger{logger: l.logger.With(l.redactAll(fields)...), forms: l.forms}
}

// redactAll redacts each argument.
func (l redactingLogger) redactAll(args []any) []any {
	out := make([]any, len(args))
	for i, arg := range args {
		out[i] = l.redactArg(arg)
	}
	return out
}

// redactArg redacts one argument. Text, bytes, an error or a Stringer become their text,
// redacted; a slog.Attr keeps its key. Anything else is redacted as fmt prints it, and
// kept as it is when that holds no value, so a number stays a number.
func (l redactingLogger) redactArg(arg any) any {
	switch v := arg.(type) {
	case string:
		return redact(v, l.forms)
	case []byte:
		return redact(string(v), l.forms)
	case error:
		return redact(v.Error(), l.forms)
	case slog.Attr: // before fmt.Stringer, which it is
		return slog.Any(v.Key, l.redactArg(v.Value.Any()))
	case fmt.Stringer:
		return redact(v.String(), l.forms)
	}
	if text := fmt.Sprint(arg); redact(text, l.forms) != text {
		return redact(text, l.forms)
	}
	return arg
}

// redactedForms are the forms a value can take in a message: as the environment holds
// it, trimmed as a decoder trims an unquoted value, escaped as %q prints it, and escaped
// as a connection string's user, password or parameter.
func redactedForms(val string) []string {
	forms := make([]string, 0, 8)
	for _, v := range []string{val, strings.TrimSpace(val)} {
		quoted := strconv.Quote(v)
		forms = append(forms, v, quoted[1:len(quoted)-1], url.User(v).String(), url.QueryEscape(v))
	}
	slices.Sort(forms)
	return slices.Compact(forms)
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
