//go:build test

package v1_test

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	db "tounilab.com/vessel/db/v1"
	mgr "tounilab.com/vessel/manager/v1"
)

// hostileValues are values a password may legitimately hold that are syntax in a
// config file: a value must reach its field unchanged, never parse as part of it (#85).
var hostileValues = []string{
	"*not-an-alias",
	"&anchor",
	`"double quoted"`,
	"'single quoted'",
	`back\slash`,
	"has #hash",
	"colon: space",
	"{[flow]}",
	"- dash",
	"tab\there",
	"line\nbreak",
	"null",
	"~",
	"true",
	"007",
	"${DB_USER}",
}

// writeConfig writes a config file into a fresh working directory, since the manager
// refuses a path outside it, and returns its relative path.
func writeConfig(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return name
}

var postgresConfigs = map[string]string{
	"config.yaml": `
health_interval: ${HEALTH_INTERVAL:5s}
entries:
  - name: primary
    type: readwrite
    postgres:
      user: ${DB_USER:app}
      password: ${DB_PASSWORD}
      host: localhost
      port: ${DB_PORT:5432}
      database: app
`,
	"config.json": `{
  "entries": [{
    "name": "primary",
    "type": "readwrite",
    "postgres": {
      "user": "${DB_USER:app}",
      "password": "${DB_PASSWORD}",
      "host": "localhost",
      "port": ${DB_PORT:5432},
      "database": "app"
    }
  }]
}`,
	"config.toml": `
[[entries]]
name = "primary"
type = "readwrite"

[entries.postgres]
user = "${DB_USER:app}"
password = "${DB_PASSWORD}"
host = "localhost"
port = ${DB_PORT:5432}
database = "app"
`,
}

func TestLoadConfig_SubstitutedValuesAreNotSyntax(t *testing.T) {
	for name, content := range postgresConfigs {
		for _, value := range hostileValues {
			t.Run(name+"/"+value, func(t *testing.T) {
				path := writeConfig(t, name, content)
				cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
					mgr.WithEnvVars(map[string]string{"DB_PASSWORD": value, "DB_PORT": "6543"}),
				})
				if err != nil {
					t.Fatalf("load: %v", err)
				}
				pg := cfg.Entries[0].Postgres
				if pg.Password != value {
					t.Errorf("password = %q, want %q", pg.Password, value)
				}
				// The port still decodes as a number, and an unset variable with a
				// default still takes it.
				if pg.Port != 6543 {
					t.Errorf("port = %d, want 6543", pg.Port)
				}
				if pg.User != "app" {
					t.Errorf("user = %q, want the default app", pg.User)
				}
			})
		}
	}
}

func TestLoadConfig_YAMLTypesAnUnquotedValueAsWritten(t *testing.T) {
	path := writeConfig(t, "config.yaml", postgresConfigs["config.yaml"])
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "secret", "HEALTH_INTERVAL": "45s"}),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HealthInterval != 45*time.Second {
		t.Errorf("health_interval = %v, want 45s", cfg.HealthInterval)
	}
	if cfg.Entries[0].Postgres.Port != 5432 {
		t.Errorf("port = %d, want the default 5432", cfg.Entries[0].Postgres.Port)
	}
}

func TestLoadConfig_YAMLQuotedPlaceholderStaysAString(t *testing.T) {
	path := writeConfig(t, "config.yaml", `
entries:
  - name: primary
    type: readwrite
    postgres:
      user: "${DB_USER}"
      password: '${DB_PASSWORD}'
      host: localhost
      port: 5432
      database: app
`)
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_USER": "1234", "DB_PASSWORD": "it's"}),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if pg := cfg.Entries[0].Postgres; pg.User != "1234" || pg.Password != "it's" {
		t.Errorf("user, password = %q, %q; want 1234, it's", pg.User, pg.Password)
	}
}

func TestLoadConfig_UnresolvedPlaceholderStaysLiteral(t *testing.T) {
	for name, content := range postgresConfigs {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, name, content)
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{mgr.WithEnvVars(map[string]string{})})
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if got := cfg.Entries[0].Postgres.Password; got != "${DB_PASSWORD}" {
				t.Errorf("password = %q, want the placeholder left as written", got)
			}
		})
	}
}

func TestLoadConfig_ErrorsDoNotQuoteSubstitutedValues(t *testing.T) {
	const secret = "hunter2"
	for name, content := range postgresConfigs {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, name, content)
			// The password lands where a number belongs, so the decoder rejects it.
			_, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "x", "DB_PORT": secret}),
			})
			if err == nil {
				t.Fatal("load succeeded with a non-numeric port")
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error quotes the substituted value: %v", err)
			}
		})
	}
}

// leaks reports whether err, or any error it wraps, hands out secret: through its
// message, or through a String method such as a TOML decode error's source excerpt.
func leaks(err error, secret string) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), secret) {
		return true
	}
	if s, ok := err.(fmt.Stringer); ok && strings.Contains(s.String(), secret) {
		return true
	}
	switch wrapped := err.(type) {
	case interface{ Unwrap() error }:
		return leaks(wrapped.Unwrap(), secret)
	case interface{ Unwrap() []error }:
		return slices.ContainsFunc(wrapped.Unwrap(), func(e error) bool { return leaks(e, secret) })
	}
	return false
}

func TestLoadConfig_ErrorChainDoesNotCarrySubstitutedValues(t *testing.T) {
	const secret = "s3cr3t-full-password"
	// The password is valid; the port, written in the file, is not. The TOML decoder's
	// error keeps the source it read, password line included.
	path := writeConfig(t, "config.toml", `
[[entries]]
name = "primary"
type = "readwrite"

[entries.postgres]
user = "app"
password = "${DB_PASSWORD}"
host = "localhost"
port = 'not-a-number'
database = "app"
`)
	_, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_PASSWORD": secret}),
	})
	if err == nil {
		t.Fatal("load succeeded with a non-numeric port")
	}
	if leaks(err, secret) {
		t.Errorf("the error chain hands out the substituted value: %v", err)
	}
}

func TestLoadConfig_RejectsAValueThatIsNotUTF8(t *testing.T) {
	const value = "pass\xffword"
	for name, content := range postgresConfigs {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, name, content)
			// Escaping would turn the byte into U+FFFD and connect with another
			// password; inserted raw, YAML would pass it on as it is.
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"DB_PASSWORD": value}),
			})
			if err == nil {
				t.Fatalf("load succeeded, password = %q", cfg.Entries[0].Postgres.Password)
			}
			if !strings.Contains(err.Error(), "${DB_PASSWORD}") {
				t.Errorf("error does not name the variable: %v", err)
			}
			if leaks(err, "pass") {
				t.Errorf("error quotes the value: %v", err)
			}
		})
	}
}

// TestLoadConfig_YAMLPlaceholderPlacements loads a placeholder from every place a YAML
// file could hold one before #85, each with a value that is YAML syntax.
func TestLoadConfig_YAMLPlaceholderPlacements(t *testing.T) {
	const value = "*a, b: {c} # d"
	placements := map[string]string{
		"flow mapping": `
entries:
  - name: primary
    type: readwrite
    postgres: {user: app, password: ${DB_PASSWORD}, host: localhost, port: ${DB_PORT:5432}, database: app}
`,
		"flow sequence": `
entries: [{name: primary, type: readwrite, postgres: {user: app, password: "${DB_PASSWORD}", host: localhost, port: 5432, database: app}}]
`,
		"anchor and alias": `
base: &${ANCHOR:pg}
  user: app
  password: ${DB_PASSWORD}
  host: localhost
  port: 5432
  database: app
entries:
  - name: primary
    type: readwrite
    postgres: *${ANCHOR:pg}
`,
		"key": `
entries:
  - name: primary
    type: readwrite
    postgres:
      user: app
      ${PASSWORD_KEY:password}: ${DB_PASSWORD}
      host: localhost
      port: 5432
      database: app
`,
		"comment": `
# rotate ${DB_PASSWORD} monthly
entries:
  - name: primary
    type: readwrite
    postgres:
      user: app
      password: ${DB_PASSWORD} # ${DB_PASSWORD}
      host: localhost
      port: 5432
      database: app
`,
	}
	for name, content := range placements {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, "config.yaml", content)
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"DB_PASSWORD": value, "ANCHOR": "pg", "PASSWORD_KEY": "password"}),
			})
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			pg := cfg.Entries[0].Postgres
			if pg.Password != value || pg.Port != 5432 {
				t.Errorf("password, port = %q, %d; want %q, 5432", pg.Password, pg.Port, value)
			}
		})
	}
}

// tomlWith is a TOML config whose password line is password, and whose port is port.
func tomlWith(password, port string) string {
	return `
[[entries]]
name = "primary"
type = "readwrite"

[entries.postgres]
user = "app"
` + password + `
host = "localhost"
port = ` + port + `
database = "app"
`
}

// TestLoadConfig_TOMLPlaceholderPlacements loads a placeholder from each kind of TOML
// string and from a comment, with values that are syntax in some of them: each value
// goes into the parsed string, so each loads as it is.
func TestLoadConfig_TOMLPlaceholderPlacements(t *testing.T) {
	values := append([]string{"x' #", "x''' #", "\nleading newline", "a\nb"}, hostileValues...)
	placements := map[string]struct {
		line   string
		values []string
	}{
		"basic string":       {`password = "${DB_PASSWORD}"`, values},
		"multi-line basic":   {`password = """${DB_PASSWORD}"""`, values},
		"literal string":     {`password = '${DB_PASSWORD}'`, values},
		"multi-line literal": {`password = '''${DB_PASSWORD}'''`, values},
		// A comment keeps the placeholder as written: nothing reads it.
		"comment": {"password = \"${DB_PASSWORD}\" # was ${DB_PASSWORD}", values},
	}
	for name, p := range placements {
		for _, value := range p.values {
			t.Run(name+"/"+value, func(t *testing.T) {
				path := writeConfig(t, "config.toml", tomlWith(p.line, "${DB_PORT:5432}"))
				cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
					mgr.WithEnvVars(map[string]string{"DB_PASSWORD": value}),
				})
				if err != nil {
					t.Fatalf("load: %v", err)
				}
				pg := cfg.Entries[0].Postgres
				if pg.Password != value || pg.Port != 5432 {
					t.Errorf("password, port = %q, %d; want %q, 5432", pg.Password, pg.Port, value)
				}
			})
		}
	}
}

// TestLoadConfig_BareValueThatIsNotAScalarFails: outside quotes a value is typed as if
// written there; one that is not a number, boolean or null is a string, never syntax,
// so a numeric field rejects it, naming the placeholder and not the value.
func TestLoadConfig_BareValueThatIsNotAScalarFails(t *testing.T) {
	cases := map[string]struct {
		name, content, value string
	}{
		"TOML": {"config.toml", tomlWith(`password = "x"`, "${DB_PASSWORD}"), "1\nuser = \"root\""},
		"JSON": {
			"config.json", `{"entries": [{"name": "primary", "type": "readwrite",
  "postgres": {"user": "app", "password": "x", "host": "localhost", "port": ${DB_PASSWORD}, "database": "app"}}]}`,
			`1, "user": "root"`,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, c.name, c.content)
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"DB_PASSWORD": c.value}),
			})
			if err == nil {
				pg := cfg.Entries[0].Postgres
				t.Fatalf("load succeeded: user, password, port = %q, %q, %d", pg.User, pg.Password, pg.Port)
			}
			if !strings.Contains(err.Error(), "${DB_PASSWORD}") {
				t.Errorf("error does not name the placeholder: %v", err)
			}
			if leaks(err, c.value) {
				t.Errorf("error quotes the value: %v", err)
			}
		})
	}
}

// TestLoadConfig_ErrorsWithholdTheDecoderOnASubstitutedLine puts a value where the
// decoder rejects it. Decoders quote part of what they reject: YAML its first seven
// characters, TOML its first character, JSON a whole number. Redacting the value
// misses a part of it, so the message for that line names the placeholder instead.
func TestLoadConfig_ErrorsWithholdTheDecoderOnASubstitutedLine(t *testing.T) {
	const pg = `
entries:
  - name: primary
    type: readwrite
    postgres: {user: app, password: x, host: localhost, port: 5432, database: app}
`
	cases := map[string]struct {
		name, content, value, quoted string
	}{
		"YAML, value first":     {"config.yaml", "read_workers: ${SECRET}\n" + pg, "abcdefghijk-secret", "abcdefg"},
		"YAML, value inside":    {"config.yaml", "read_workers: x-${SECRET}\n" + pg, "abcdefghijk-secret", "abcde"},
		"TOML, first character": {"config.toml", tomlWith(`password = "x"`, "${SECRET}"), "hunter2xyz", "'h'"},
		"TOML, whole number":    {"config.toml", tomlWith(`password = "x"`, "${SECRET}"), "99999999", "99999999"},
		"JSON, whole number": {
			"config.json", `{"entries": [{"name": "primary", "type": "readwrite",
  "postgres": {"user": "app", "password": "x", "host": "localhost", "port": ${SECRET}, "database": "app"}}]}`,
			"99999999", "99999999",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, c.name, c.content)
			_, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"SECRET": c.value}),
			})
			if err == nil {
				t.Fatal("load succeeded")
			}
			if leaks(err, c.quoted) {
				t.Errorf("error quotes part of the value: %v", err)
			}
			if !strings.Contains(err.Error(), "${SECRET}") {
				t.Errorf("error does not name the placeholder: %v", err)
			}
		})
	}
}

// TestLoadConfig_DefaultIsAValue: a default goes into the parsed data like a variable's
// value, so a quoted one is text as written, escapes included, and an unquoted one is
// typed as if written there.
func TestLoadConfig_DefaultIsAValue(t *testing.T) {
	path := writeConfig(t, "config.yaml", `
entries:
  - name: primary
    type: readwrite
    postgres: {user: app, password: "${DB_PASSWORD:a\tb}", host: localhost, port: ${DB_PORT:5432}, database: app}
`)
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{mgr.WithEnvVars(map[string]string{})})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if pg := cfg.Entries[0].Postgres; pg.Password != `a\tb` || pg.Port != 5432 {
		t.Errorf("password, port = %q, %d; want a\\tb as written, and 5432", pg.Password, pg.Port)
	}
}

// TestLoadConfig_TOMLKeyFromTheEnvironmentIsOneKey: a key from the environment names a
// field, and a dotted one is a single key, which would otherwise reach into another
// table.
func TestLoadConfig_TOMLKeyFromTheEnvironmentIsOneKey(t *testing.T) {
	const entry = "[[entries]]\nname = \"primary\"\ntype = \"readwrite\"\n"
	const postgres = "[entries.postgres]\nuser = \"app\"\npassword = \"x\"\nhost = \"localhost\"\nport = 5432\ndatabase = \"app\"\n"
	cases := map[string]struct {
		content, key, want string
	}{
		"names a field":   {entry + postgres + "${KEY} = \"verify-full\"\n", "ssl_mode", "verify-full"},
		"dotted, one key": {entry + "${KEY} = \"verify-full\"\n" + postgres, "postgres.ssl_mode", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, "config.toml", c.content)
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"KEY": c.key}),
			})
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if got := cfg.Entries[0].Postgres.SSLMode; got != c.want {
				t.Errorf("ssl_mode = %q, want %q", got, c.want)
			}
		})
	}
}

// TestLoadConfig_WithholdingSurvivesMultiLineValues puts a value with line breaks above
// a value the decoder rejects: the error must still name the rejected one, and quote
// nothing of it.
func TestLoadConfig_WithholdingSurvivesMultiLineValues(t *testing.T) {
	cases := map[string]struct {
		password, secret, quoted string
	}{
		"value with line breaks":       {`password = '''${ML}'''`, "hunter2xyz", "'h'"},
		"default with line breaks":     {`password = """${UNSET:l1` + "\n" + `l2}"""`, "hunter2xyz", "'h'"},
		"number the decoder reformats": {`password = '''${ML}'''`, "99_999_999", "99999999"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, "config.toml", tomlWith(c.password, "${SECRET}"))
			_, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"ML": "l1\nl2\nl3", "SECRET": c.secret}),
			})
			if err == nil {
				t.Fatal("load succeeded")
			}
			if leaks(err, c.quoted) || !strings.Contains(err.Error(), "${SECRET}") {
				t.Errorf("error should name ${SECRET} and quote nothing of its value: %v", err)
			}
		})
	}
}

// TestLoadConfig_WithholdsAJSONErrorAtTheEndOfTheLine: a bare value the decoder rejects,
// followed by a line break, is named, not quoted.
func TestLoadConfig_WithholdsAJSONErrorAtTheEndOfTheLine(t *testing.T) {
	path := writeConfig(t, "config.json", `{"entries": [{"name": "primary", "type": "readwrite",
  "postgres": {"user": "app", "password": "x", "host": "localhost", "port": ${SECRET}
  , "database": "app"}}]}`)
	_, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"SECRET": "tru"}),
	})
	if err == nil {
		t.Fatal("load succeeded")
	}
	if !strings.Contains(err.Error(), "${SECRET}") {
		t.Errorf("error should name ${SECRET}: %v", err)
	}
}

// TestLoadConfig_TOMLLineBreakAfterAQuoteIsKept: four quotes open a multi-line literal
// string whose first character is a quote, so a value starting with a line break keeps
// it; TOML drops only a line break written right after the opening quotes.
func TestLoadConfig_TOMLLineBreakAfterAQuoteIsKept(t *testing.T) {
	path := writeConfig(t, "config.toml", tomlWith(`password = ''''${DB_PASSWORD}'''`, "5432"))
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "\nx"}),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Entries[0].Postgres.Password; got != "'\nx" {
		t.Errorf("password = %q, want \"'\\nx\"", got)
	}
}

// TestLoadConfig_DigitsBeforeAPlaceholderStayText: a file holding 700000000000 to
// 700000000069 would push a token's prefix to 700000000070, whose 70 the 7000000000
// written before a placeholder could complete into a token starting in that text.
func TestLoadConfig_DigitsBeforeAPlaceholderStayText(t *testing.T) {
	var taken strings.Builder
	for i := range 70 {
		fmt.Fprintf(&taken, "# %d\n", 700_000_000_000+i)
	}
	path := writeConfig(t, "config.yaml", taken.String()+`
entries:
  - name: primary
    type: readwrite
    postgres: {user: app, password: "7000000000${DB_PASSWORD}", host: localhost, port: 5432, database: app}
`)
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "secret"}),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Entries[0].Postgres.Password; got != "7000000000secret" {
		t.Errorf("password = %q, want 7000000000secret", got)
	}
}

// TestLoadConfig_TextDecodingToATokenIsText: an escape or a TOML integer can spell a
// token's digits without the file holding them as written; they stay what they are.
// ${DB_PASSWORD} takes the first token, 700000000000000000.
func TestLoadConfig_TextDecodingToATokenIsText(t *testing.T) {
	cases := map[string]struct {
		file, content, name string
		priority            int
	}{
		"JSON escape, a token in use": {
			"config.json", `{"entries": [{"name": "70000\u0030000000000000",
  "type": "readwrite", "priority": 1,
  "postgres": {"user": "app", "password": "${DB_PASSWORD}", "host": "localhost", "port": 5432, "database": "app"}}]}`,
			"700000000000000000", 1,
		},
		"JSON escape, a token not in use": {
			"config.json", `{"entries": [{"name": "70000\u0030000000000001",
  "type": "readwrite", "priority": 1,
  "postgres": {"user": "app", "password": "${DB_PASSWORD}", "host": "localhost", "port": 5432, "database": "app"}}]}`,
			"700000000000000001", 1,
		},
		"YAML escape": {"config.yaml", `
entries:
  - name: "70000\x30000000000000"
    type: readwrite
    priority: 1
    postgres: {user: app, password: "${DB_PASSWORD}", host: localhost, port: 5432, database: app}
`, "700000000000000000", 1},
		"TOML integer with underscores": {"config.toml", `
[[entries]]
name = "primary"
type = "readwrite"
priority = 700_000_000_000_000_000

[entries.postgres]
user = "app"
password = "${DB_PASSWORD}"
host = "localhost"
port = 5432
database = "app"
`, "primary", 700_000_000_000_000_000},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, c.file, c.content)
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "secret"}),
			})
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			e := cfg.Entries[0]
			if e.Name != c.name || *e.Priority != c.priority || e.Postgres.Password != "secret" {
				t.Errorf("name, priority, password = %q, %d, %q; want %q, %d, secret",
					e.Name, *e.Priority, e.Postgres.Password, c.name, c.priority)
			}
		})
	}
}

// TestLoadConfig_YAMLDefaultCannotForgeAToken: a default next to text that, together,
// spell what a token looks like still loads as that text, never as another variable.
func TestLoadConfig_YAMLDefaultCannotForgeAToken(t *testing.T) {
	path := writeConfig(t, "config.yaml", `
entries:
  - name: primary
    type: readwrite
    postgres:
      user: ${DB_USER}
      password: ${UNSET:vessel}env000000
      host: localhost
      port: 5432
      database: app
`)
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_USER": "bob"}),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Entries[0].Postgres.Password; got != "vesselenv000000" {
		t.Errorf("password = %q, want the default and the text after it", got)
	}
}

// TestLoadConfig_TOMLKeepsLeadingSpacesAfterALineEndingBackslash: in a multi-line basic
// string TOML drops the whitespace after a line-ending backslash; a value placed there
// keeps its leading spaces.
func TestLoadConfig_TOMLKeepsLeadingSpacesAfterALineEndingBackslash(t *testing.T) {
	path := writeConfig(t, "config.toml", tomlWith("password = \"\"\"a\\\n${DB_PASSWORD}\"\"\"", "5432"))
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "  lead"}),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Entries[0].Postgres.Password; got != "a  lead" {
		t.Errorf("password = %q, want \"a  lead\"", got)
	}
}

// TestLoadConfig_YAMLUnquotedValueIsTrimmedAsIfWritten: written unquoted, YAML drops the
// spaces and line breaks around a value, as it always did for a substituted one; a
// secret mounted from a file often ends with a line break. A quoted value keeps them.
func TestLoadConfig_YAMLUnquotedValueIsTrimmedAsIfWritten(t *testing.T) {
	path := writeConfig(t, "config.yaml", `
entries:
  - name: primary
    type: readwrite
    postgres:
      user: "${DB_USER}"
      password: ${DB_PASSWORD}
      host: localhost
      port: ${DB_PORT}
      database: app
`)
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_USER": " app\n", "DB_PASSWORD": "  secret \t\n", "DB_PORT": "6543\n"}),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	pg := cfg.Entries[0].Postgres
	if pg.User != " app\n" || pg.Password != "secret" || pg.Port != 6543 {
		t.Errorf("user, password, port = %q, %q, %d; want \" app\\n\", secret, 6543", pg.User, pg.Password, pg.Port)
	}
}

// TestNewDBManager_ValidationErrorDoesNotQuoteSubstitutedValues: validation quotes an
// entry's name. Another value is part of that name, so the name is redacted whole only
// when the longer value goes first.
func TestNewDBManager_ValidationErrorDoesNotQuoteSubstitutedValues(t *testing.T) {
	path := writeConfig(t, "config.yaml", `
entries:
  - name: ${ENTRY}
    type: unknown
    postgres: {user: "${DB_USER}", password: x, host: localhost, port: 5432, database: app}
`)
	_, err := mgr.NewDBManager(t.Context(), path, nil,
		mgr.WithEnvVars(map[string]string{"ENTRY": "abcdef", "DB_USER": "abc"}))
	if err == nil {
		t.Fatal("NewDBManager accepted an unknown entry type")
	}
	if leaks(err, "abc") || leaks(err, "def") {
		t.Errorf("error quotes a substituted value: %v", err)
	}
}

// TestLoadConfig_NoEnvOptionExpandsNothing: without an EnvOption a placeholder stays as
// written, a default included, in every format (secure by default).
func TestLoadConfig_NoEnvOptionExpandsNothing(t *testing.T) {
	for name, content := range map[string]string{
		"config.yaml": `
entries:
  - name: primary
    type: readwrite
    postgres: {user: "${DB_USER:app}", password: "${DB_PASSWORD}", host: localhost, port: 5432, database: app}
`,
		"config.json": `{"entries": [{"name": "primary", "type": "readwrite", "postgres": {"user": "${DB_USER:app}",
  "password": "${DB_PASSWORD}", "host": "localhost", "port": 5432, "database": "app"}}]}`,
		"config.toml": tomlWith(`password = "${DB_PASSWORD}"`+"\n"+`ssl_mode = "${DB_USER:app}"`, "5432"),
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, name, content)
			cfg, err := mgr.LoadConfigForTest(path, nil)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			pg := cfg.Entries[0].Postgres
			defaulted := pg.User
			if name == "config.toml" {
				defaulted = pg.SSLMode
			}
			if pg.Password != "${DB_PASSWORD}" || defaulted != "${DB_USER:app}" {
				t.Errorf("password, defaulted = %q, %q; want both placeholders as written", pg.Password, defaulted)
			}
		})
	}
}

// TestLoadConfig_ValueAfterEscapesAndQuoteRuns puts a placeholder after an escaped quote
// and after a multi-line string closed by four quotes, where reading the file's text
// could lose track of the string it is in.
func TestLoadConfig_ValueAfterEscapesAndQuoteRuns(t *testing.T) {
	const value = `x", "port": 1, "y": "`
	cases := map[string]struct{ name, content string }{
		"JSON escaped quote": {"config.json", `{"entries": [{"name": "primary", "type": "readwrite",
  "postgres": {"user": "a\"b", "password": "${DB_PASSWORD}", "host": "localhost", "port": 5432, "database": "app"}}]}`},
		"TOML escaped quote": {"config.toml", tomlWith(`ssl_mode = "a\"b"`+"\n"+`password = "${DB_PASSWORD}"`, "5432")},
		// A closing run of four quotes ends the string after the fourth; a placeholder
		// on the same line, in an inline table, must be read as in a string.
		"TOML closing run of four double quotes": {
			"config.toml", tomlWith(`extra = {a = """a"""", b = "${DB_PASSWORD}"}`+"\n"+`password = "${DB_PASSWORD}"`, "5432"),
		},
		"TOML closing run of four single quotes": {
			"config.toml", tomlWith(`extra = {a = '''a'''', b = "${DB_PASSWORD}"}`+"\n"+`password = "${DB_PASSWORD}"`, "5432"),
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, c.name, c.content)
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"DB_PASSWORD": value}),
			})
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if pg := cfg.Entries[0].Postgres; pg.Password != value || pg.Port != 5432 {
				t.Errorf("password, port = %q, %d; want %q, 5432", pg.Password, pg.Port, value)
			}
		})
	}
}

// TestLoadConfig_YAMLParseErrorNamesThePlaceholder: a token never reaches a message.
func TestLoadConfig_YAMLParseErrorNamesThePlaceholder(t *testing.T) {
	path := writeConfig(t, "config.yaml", "pg: *${ANCHOR}\n")
	_, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"ANCHOR": "pg"}),
	})
	if err == nil || !strings.Contains(err.Error(), "${ANCHOR}") {
		t.Errorf("error should name ${ANCHOR}: %v", err)
	}
}

// TestLoadConfig_YAMLEmptyUnquotedValueIsNull: an empty value reads as null, as it would
// written there, so a numeric field keeps its zero value.
func TestLoadConfig_YAMLEmptyUnquotedValueIsNull(t *testing.T) {
	path := writeConfig(t, "config.yaml", "read_workers: ${WORKERS}\n"+`
entries:
  - name: primary
    type: readwrite
    postgres: {user: app, password: x, host: localhost, port: 5432, database: app}
`)
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"WORKERS": ""}),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ReadWorkers != 0 {
		t.Errorf("read_workers = %d, want 0", cfg.ReadWorkers)
	}
}

// TestNewDBManager_SetupErrorDoesNotCarrySubstitutedValues: the PostgreSQL driver's
// parse error keeps the connection string, password included, for errors.As to reach.
func TestNewDBManager_SetupErrorDoesNotCarrySubstitutedValues(t *testing.T) {
	const secret = "credential-secret"
	path := writeConfig(t, "config.yaml", `
entries:
  - name: primary
    type: readwrite
    postgres: {user: app, password: "${DB_PASSWORD}", host: localhost, port: 5432, database: app, ssl_mode: invalid}
`)
	_, err := mgr.NewDBManager(t.Context(), path, nil,
		mgr.WithEnvVars(map[string]string{"DB_PASSWORD": secret}))
	if err == nil {
		t.Fatal("NewDBManager accepted an invalid ssl_mode")
	}
	var parseErr *pgconn.ParseConfigError
	if errors.As(err, &parseErr) || leaks(err, secret) {
		t.Errorf("the error chain hands out the connection string: %v", err)
	}
}

// TestNewDBManager_ValidationDoesNotQuoteTheRejectedType: decoding can change a value
// (YAML trims it) and quoting it in a message escapes it, so redacting the value as
// substituted misses it there. Validation names the field instead of quoting it.
func TestNewDBManager_ValidationDoesNotQuoteTheRejectedType(t *testing.T) {
	for _, value := range []string{" credential-secret ", `credential"secret`} {
		t.Run(value, func(t *testing.T) {
			path := writeConfig(t, "config.yaml", `
entries:
  - name: primary
    type: ${ENTRY_TYPE}
    postgres: {user: app, password: x, host: localhost, port: 5432, database: app}
`)
			_, err := mgr.NewDBManager(t.Context(), path, nil,
				mgr.WithEnvVars(map[string]string{"ENTRY_TYPE": value}))
			if err == nil {
				t.Fatal("NewDBManager accepted an unknown entry type")
			}
			if leaks(err, "credential") {
				t.Errorf("error quotes the type: %v", err)
			}
		})
	}
}

// TestLoadConfig_AdjacentValuesStayValues: two values side by side in a TOML multi-line
// literal string, which together spell its closing quotes and a comment. Each is text in
// the parsed string, so together they are too.
func TestLoadConfig_AdjacentValuesStayValues(t *testing.T) {
	path := writeConfig(t, "config.toml", tomlWith(`password = '''${PART_A}${PART_B}'''`, "5432"))
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"PART_A": "x''", "PART_B": "' #"}),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Entries[0].Postgres.Password; got != "x''' #" {
		t.Errorf("password = %q, want \"x''' #\"", got)
	}
}

// TestLoadConfig_DefaultIsNotSyntax: a default is a value like any other, so one cannot
// open a string for the next placeholder. Placeholders glued together outside quotes
// fail the load instead of loading as something else.
func TestLoadConfig_DefaultIsNotSyntax(t *testing.T) {
	path := writeConfig(t, "config.toml", tomlWith(`password = ${QUOTE:"}${DB_PASSWORD}${QUOTE:"}`, "5432"))
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "has spaces"}),
	})
	if err == nil {
		t.Fatalf("load succeeded, password = %q", cfg.Entries[0].Postgres.Password)
	}
}

// TestLoadConfig_YAMLTagCannotHoldAPlaceholder: a tag is syntax, so a placeholder there
// fails the load, naming it.
func TestLoadConfig_YAMLTagCannotHoldAPlaceholder(t *testing.T) {
	path := writeConfig(t, "config.yaml", `
entries:
  - name: primary
    type: readwrite
    postgres: {user: app, password: x, host: localhost, port: !!${TAG} 5432, database: app}
`)
	_, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"TAG": "int"}),
	})
	if err == nil || !strings.Contains(err.Error(), "${TAG}") {
		t.Errorf("error should name ${TAG}: %v", err)
	}
}

// jsonWithPort is a JSON config whose port is written as port.
func jsonWithPort(port string) string {
	return `{"entries": [{"name": "primary", "type": "readwrite",
  "postgres": {"user": "app", "password": "x", "host": "localhost", "port": ` + port + `, "database": "app"}}]}`
}

// TestLoadConfig_BareValueIsTypedAsIfWritten: a whole bare value outside quotes reads as
// it would written there: spaces around it don't count, TOML takes its bases and
// underscores, and JSON's null is null. Booleans are TestLoadConfig_BareBooleanIsTyped.
func TestLoadConfig_BareValueIsTypedAsIfWritten(t *testing.T) {
	cases := map[string]struct {
		name, content, value string
		port                 uint16
	}{
		"JSON spaces":      {"config.json", jsonWithPort("${DB_PORT}"), " 6543\n", 6543},
		"JSON null":        {"config.json", jsonWithPort("${DB_PORT}"), "null", 0},
		"TOML hexadecimal": {"config.toml", tomlWith(`password = "x"`, "${DB_PORT}"), "0x1F", 31},
		"TOML underscores": {"config.toml", tomlWith(`password = "x"`, "${DB_PORT}"), "6_543", 6543},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, c.name, c.content)
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"DB_PORT": c.value}),
			})
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if got := cfg.Entries[0].Postgres.Port; got != c.port {
				t.Errorf("port = %d, want %d", got, c.port)
			}
		})
	}
}

// TestLoadConfig_GluedBarePlaceholderFails: a token glued to other text outside quotes
// is part of a number that is not a value, so the load fails, naming the placeholder.
func TestLoadConfig_GluedBarePlaceholderFails(t *testing.T) {
	path := writeConfig(t, "config.json", jsonWithPort("54${DB_PORT}"))
	_, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_PORT": "32"}),
	})
	if err == nil || !strings.Contains(err.Error(), "${DB_PORT}") {
		t.Errorf("error should name ${DB_PORT}: %v", err)
	}
}

// TestLoadConfig_JSONRejectsDataAfterTheValue, as json.Unmarshal does.
func TestLoadConfig_JSONRejectsDataAfterTheValue(t *testing.T) {
	path := writeConfig(t, "config.json", jsonWithPort("${DB_PORT}")+` {"entries": []}`)
	if _, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_PORT": "5432"}),
	}); err == nil {
		t.Error("load accepted a second JSON value")
	}
}

// TestLoadConfig_FileHoldingTokenDigitsLoadsThem: tokens start with digits the file does
// not contain, so a literal value that looks like one stays itself.
func TestLoadConfig_FileHoldingTokenDigitsLoadsThem(t *testing.T) {
	path := writeConfig(t, "config.yaml", `
entries:
  - name: primary
    type: readwrite
    postgres: {user: "700000000000000000", password: "${DB_PASSWORD}", host: localhost, port: 5432, database: app}
`)
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "secret"}),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if pg := cfg.Entries[0].Postgres; pg.User != "700000000000000000" || pg.Password != "secret" {
		t.Errorf("user, password = %q, %q; want 700000000000000000, secret", pg.User, pg.Password)
	}
}

// TestLoadConfig_WithholdsAQuotedValueInANumericField: a value quoted into a string that
// lands in a numeric field is named, not quoted.
func TestLoadConfig_WithholdsAQuotedValueInANumericField(t *testing.T) {
	for name, content := range map[string]string{
		"config.json": jsonWithPort(`"${SECRET}"`),
		"config.toml": tomlWith(`password = "x"`, `"${SECRET}"`),
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, name, content)
			_, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"SECRET": "99999999"}),
			})
			if err == nil || !strings.Contains(err.Error(), "${SECRET}") || leaks(err, "99999999") {
				t.Errorf("error should name ${SECRET} and quote nothing of it: %v", err)
			}
		})
	}
}

// TestLoadConfig_BareBooleanIsTyped: true or false outside quotes reaches a boolean field.
func TestLoadConfig_BareBooleanIsTyped(t *testing.T) {
	for name, content := range map[string]string{
		"config.json": `{"write_batching_enabled": ${BATCH}, "entries": [{"name": "primary", "type": "readwrite",
  "postgres": {"user": "app", "password": "x", "host": "localhost", "port": 5432, "database": "app"}}]}`,
		"config.toml": "write_batching_enabled = ${BATCH}\n" + tomlWith(`password = "x"`, "5432"),
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, name, content)
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"BATCH": "true"}),
			})
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if !cfg.WriteBatchingEnabled {
				t.Error("write_batching_enabled = false, want true")
			}
		})
	}
}

// TestLoadConfig_WithholdsAnErrorAtAKeyFromTheEnvironment: a rejected value under a key
// that came from the environment names that key's placeholder.
func TestLoadConfig_WithholdsAnErrorAtAKeyFromTheEnvironment(t *testing.T) {
	path := writeConfig(t, "config.toml", tomlWith(`password = "x"`+"\n"+`${KEY} = "not a number"`, "5432"))
	_, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"KEY": "pool_max_conns"}),
	})
	if err == nil || !strings.Contains(err.Error(), "${KEY}") {
		t.Errorf("error should name ${KEY}: %v", err)
	}
}

// TestLoadConfig_TOMLBareValueFollowsTOMLIntegers: TOML has no leading zeros and only
// lowercase base prefixes, so 05432 and 0X1F are not integers written there: they fail
// the load rather than reading as octal or hexadecimal.
func TestLoadConfig_TOMLBareValueFollowsTOMLIntegers(t *testing.T) {
	for _, value := range []string{"05432", "0X1F", "0x_1F", "1__000", "+0x1F"} {
		t.Run(value, func(t *testing.T) {
			path := writeConfig(t, "config.toml", tomlWith(`password = "x"`, "${DB_PORT}"))
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"DB_PORT": value}),
			})
			if err == nil {
				t.Fatalf("load succeeded, port = %d", cfg.Entries[0].Postgres.Port)
			}
			if !strings.Contains(err.Error(), "${DB_PORT}") {
				t.Errorf("error should name ${DB_PORT}: %v", err)
			}
		})
	}
}

// TestLoadConfig_JSONKeyWrittenTwiceFails: decoding into the config merged two objects
// under one key and took the last of two values; the generic tree keeps only the last
// object, dropping the first one's fields, so a repeated key fails the load.
func TestLoadConfig_JSONKeyWrittenTwiceFails(t *testing.T) {
	for name, entry := range map[string]string{
		"object": `"postgres": {"user": "app", "password": "${DB_PASSWORD}", "host": "localhost"},
  "postgres": {"port": 5432, "database": "app"}`,
		"value": `"name": "secondary", "postgres": {"user": "app", "password": "${DB_PASSWORD}", "host": "localhost",
  "port": 5432, "database": "app"}`,
		"nested value": `"postgres": {"user": "app", "password": "${DB_PASSWORD}", "host": "localhost",
  "port": 5432, "database": "app", "database": "other"}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, "config.json", `{"entries": [{"name": "primary", "type": "readwrite", `+entry+`}]}`)
			_, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "secret"}),
			})
			if err == nil || !strings.Contains(err.Error(), "written twice") {
				t.Errorf("want the repeated key refused: %v", err)
			}
		})
	}
}

// TestLoadConfig_SameKeyInSiblingObjectsLoads: a key repeats only within one object; two
// entries each have their own name.
func TestLoadConfig_SameKeyInSiblingObjectsLoads(t *testing.T) {
	entry := `{"name": "%s", "type": "readwrite", "postgres": {"user": "app", "password": "${DB_PASSWORD}",
  "host": "localhost", "port": 5432, "database": "app"}}`
	path := writeConfig(t, "config.json",
		`{"entries": [`+fmt.Sprintf(entry, "primary")+", "+fmt.Sprintf(entry, "secondary")+`]}`)
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "secret"}),
	})
	if err != nil || len(cfg.Entries) != 2 || cfg.Entries[1].Postgres.Password != "secret" {
		t.Fatalf("cfg, err = %+v, %v; want two entries", cfg, err)
	}
}

// TestLoadConfig_KeysEqualButForCaseFail: JSON and TOML both match a key to its field
// ignoring case, so of host and Host the last written wins; the generic tree the values
// go into keeps no order, so neither may win.
func TestLoadConfig_KeysEqualButForCaseFail(t *testing.T) {
	for name, content := range map[string]string{
		"config.toml": tomlWith(`password = "${DB_PASSWORD}"`+"\n"+`Host = "other.internal"`, "5432"),
		"config.json": `{"entries": [{"name": "primary", "type": "readwrite", "postgres": {"user": "app",
  "password": "${DB_PASSWORD}", "host": "localhost", "Host": "other.internal", "port": 5432, "database": "app"}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, name, content)
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "secret"}),
			})
			if err == nil {
				t.Fatalf("load succeeded, host = %q", cfg.Entries[0].Postgres.Host)
			}
			if !strings.Contains(err.Error(), "Host") || !strings.Contains(err.Error(), "host") {
				t.Errorf("error should name both keys: %v", err)
			}
		})
	}
}

// TestLoadConfig_KeyFromTheEnvironmentCannotRepeatAKey: two equal keys in parsed data
// leave one value to chance, so a key from the environment that repeats another fails
// the load, naming its placeholder.
func TestLoadConfig_KeyFromTheEnvironmentCannotRepeatAKey(t *testing.T) {
	for name, content := range map[string]string{
		"config.toml": tomlWith(`password = "literal"`+"\n"+`${KEY} = "from the environment"`, "5432"),
		"config.json": `{"entries": [{"name": "primary", "type": "readwrite", "postgres": {"user": "app",
  "password": "literal", "${KEY}": "from the environment", "host": "localhost", "port": 5432, "database": "app"}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, name, content)
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"KEY": "password"}),
			})
			if err == nil {
				t.Fatalf("load succeeded, password = %q", cfg.Entries[0].Postgres.Password)
			}
			if !strings.Contains(err.Error(), "${KEY}") {
				t.Errorf("error should name ${KEY}: %v", err)
			}
		})
	}
}

// TestLoadConfig_WithholdsAJSONErrorAtACaseVariantKey: JSON matches a key to its field
// ignoring case, so an error at PORT is the error at port.
func TestLoadConfig_WithholdsAJSONErrorAtACaseVariantKey(t *testing.T) {
	path := writeConfig(t, "config.json", `{"entries": [{"name": "primary", "type": "readwrite",
  "postgres": {"user": "app", "password": "x", "host": "localhost", "PORT": ${SECRET}, "database": "app"}}]}`)
	_, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"SECRET": "123456789\n"}),
	})
	if err == nil || !strings.Contains(err.Error(), "${SECRET}") || leaks(err, "123456789") {
		t.Errorf("error should name ${SECRET} and quote nothing of it: %v", err)
	}
}

// TestNewDBManager_ErrorsDoNotCarryAValueAsDecodedOrEncoded: decoding trims an unquoted
// value and a message quoting it escapes it; each form is redacted, not only the value
// as the environment holds it.
func TestNewDBManager_ErrorsDoNotCarryAValueAsDecodedOrEncoded(t *testing.T) {
	cases := map[string]struct {
		postgres string
		vars     map[string]string
	}{
		"entry name with a quote": {
			"{user: app, password: x, host: localhost, port: 5432, database: app}",
			map[string]string{"ENTRY": `secret"name`, "ENTRY_TYPE": "unknown"},
		},
		"entry name YAML trims": {
			"{user: app, password: x, host: localhost, port: 5432, database: app}",
			map[string]string{"ENTRY": " secretname\n", "ENTRY_TYPE": "unknown"},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, "config.yaml", `
entries:
  - name: ${ENTRY}
    type: ${ENTRY_TYPE}
    postgres: `+c.postgres+"\n")
			_, err := mgr.NewDBManager(t.Context(), path, nil, mgr.WithEnvVars(c.vars))
			if err == nil {
				t.Fatal("NewDBManager succeeded")
			}
			if leaks(err, "secret") {
				t.Errorf("error carries a form of a substituted value: %v", err)
			}
		})
	}
}

// TestLoadConfig_EmptyYAMLLoadsNothing: an empty file, or one with only comments, loads
// an empty config, as it does without an EnvOption.
func TestLoadConfig_EmptyYAMLLoadsNothing(t *testing.T) {
	for name, content := range map[string]string{"empty": "", "comments only": "# ${DB_PASSWORD}\n"} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, "config.yaml", content)
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
				mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "x"}),
			})
			if err != nil || len(cfg.Entries) != 0 {
				t.Errorf("cfg, err = %+v, %v; want an empty config", cfg, err)
			}
		})
	}
}

// TestLoadConfig_YAMLKeyFromTheEnvironmentIsNoMergeKey: a key whose value is << is that
// key, never YAML's merge key pulling an anchor's fields into the entry.
func TestLoadConfig_YAMLKeyFromTheEnvironmentIsNoMergeKey(t *testing.T) {
	path := writeConfig(t, "config.yaml", `
defaults: &defaults {write_queue_size: 7}
entries:
  - name: primary
    type: readwrite
    ${KEY}: *defaults
    postgres: {user: app, password: x, host: localhost, port: 5432, database: app}
`)
	cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"KEY": "<<"}),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Entries[0].WriteQueueSize; got != nil {
		t.Errorf("write_queue_size = %d, want it unset: the key merged the anchor", *got)
	}
}

// TestLoadConfig_RejectsADefaultThatIsNotUTF8: the parser no longer reads a default, so
// it no longer refuses invalid bytes in one; YAML handed them on and TOML re-encoded
// them into another credential.
func TestLoadConfig_RejectsADefaultThatIsNotUTF8(t *testing.T) {
	for name, content := range map[string]string{
		"config.yaml": "entries:\n  - name: primary\n    type: readwrite\n" +
			"    postgres: {user: app, password: \"${DB_PASSWORD:a\xffb}\", host: localhost, port: 5432, database: app}\n",
		"config.toml": tomlWith("password = \"${DB_PASSWORD:a\xffb}\"", "5432"),
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, name, content)
			cfg, err := mgr.LoadConfigForTest(path, []mgr.EnvOption{mgr.WithEnvVars(map[string]string{})})
			if err == nil {
				t.Fatalf("load succeeded, password = %q", cfg.Entries[0].Postgres.Password)
			}
			if !strings.Contains(err.Error(), "${DB_PASSWORD") {
				t.Errorf("error should name the placeholder: %v", err)
			}
		})
	}
}

// mssqlNamedInstance is an MSSQL entry whose host, a named instance, fails to parse with
// an error quoting the connection string, password included.
func mssqlNamedInstance(entryType, password string) string {
	return `
entries:
  - name: primary
    type: ` + entryType + `
    mssql: {user: "${DB_USER}", password: ` + password + `, host: "srv\\INSTANCE", port: 1433, database: app, encrypt: "true"}
`
}

// TestNewDBManager_SetupRedactsPasswordsOnly: once the config is decoded the password is
// the field known to be secret; setup errors and logs redact it in every form, and keep
// the user, host and database an operator needs to tell what failed.
func TestNewDBManager_SetupRedactsPasswordsOnly(t *testing.T) {
	for _, entryType := range []string{"readonly", "readwrite"} {
		t.Run(entryType, func(t *testing.T) {
			path := writeConfig(t, "config.yaml", mssqlNamedInstance(entryType, `"${DB_PASSWORD}"`))
			var logs strings.Builder
			logger := db.NewSlogAdapter(slog.New(slog.NewTextHandler(&logs, nil)))
			_, err := mgr.NewDBManager(t.Context(), path, logger, mgr.WithEnvVars(map[string]string{
				"DB_USER": "visible-user", "DB_PASSWORD": "pass word&x",
			}))
			if err == nil {
				t.Fatal("NewDBManager accepted a host it cannot parse")
			}
			if strings.Count(logs.String(), "level=ERROR") < 2 {
				t.Fatalf("want the entry's and the manager's error lines:\n%s", logs.String())
			}
			for _, out := range []string{err.Error(), logs.String()} {
				if strings.Contains(out, "pass") || !strings.Contains(out, "visible-user") {
					t.Errorf("want the password redacted and the user kept:\n%s", out)
				}
			}
		})
	}
}

// TestNewDBManager_RedactsAPasswordAsDecoded: !!binary hands the field its base64 text
// decoded, and that is the password redacted.
func TestNewDBManager_RedactsAPasswordAsDecoded(t *testing.T) {
	path := writeConfig(t, "config.yaml", mssqlNamedInstance("readwrite", `!!binary "${DB_PASSWORD}"`))
	_, err := mgr.NewDBManager(t.Context(), path, nil, mgr.WithEnvVars(map[string]string{
		"DB_USER":     "app",
		"DB_PASSWORD": base64.StdEncoding.EncodeToString([]byte("decoded secret")),
	}))
	if err == nil || !strings.Contains(err.Error(), "INSTANCE") {
		t.Fatalf("want the error about the host: %v", err)
	}
	if leaks(err, "decoded") {
		t.Errorf("error carries the decoded password: %v", err)
	}
}

// TestNewDBManager_LogsKeepWhatIsNotAPassword: values from the environment such as a
// user postgres are ordinary words; a log line keeps them, and so the driver's name.
func TestNewDBManager_LogsKeepWhatIsNotAPassword(t *testing.T) {
	path := writeConfig(t, "config.yaml", `
entries:
  - name: ${ENTRY}
    type: readwrite
    postgres: {user: "${DB_USER}", password: "${DB_PASSWORD}", host: 127.0.0.1, port: 1, database: app, ssl_mode: disable}
`)
	var logs strings.Builder
	logger := db.NewSlogAdapter(slog.New(slog.NewTextHandler(&logs, nil)))
	_, err := mgr.NewDBManager(t.Context(), path, logger, mgr.WithEnvVars(map[string]string{
		"ENTRY": "registry", "DB_USER": "postgres", "DB_PASSWORD": "pw-secret",
	}))
	if err == nil {
		t.Fatal("NewDBManager connected to a closed port")
	}
	for _, want := range []string{"postgres.Ping: failed", "user=postgres", "entry_name=registry"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("logs lack %q:\n%s", want, logs.String())
		}
	}
	if strings.Contains(logs.String(), "pw-secret") {
		t.Errorf("logs carry the password:\n%s", logs.String())
	}
}

// TestNewDBManager_SetupErrorKeepsItsErrno: the redacted error still tells a refused
// connection from others, whether or not a value came from the environment.
func TestNewDBManager_SetupErrorKeepsItsErrno(t *testing.T) {
	path := writeConfig(t, "config.yaml", `
entries:
  - name: primary
    type: readwrite
    postgres: {user: app, password: "${DB_PASSWORD}", host: 127.0.0.1, port: 1, database: app, ssl_mode: disable}
`)
	_, err := mgr.NewDBManager(t.Context(), path, nil, mgr.WithEnvVars(map[string]string{"DB_PASSWORD": "pw"}))
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Errorf("errors.Is(err, ECONNREFUSED) = false: %v", err)
	}
}

// TestNewDBManager_WithoutEnvOptionSetupErrorIsAsBefore: with nothing substituted, a
// setup error is the driver's own, chain and all.
func TestNewDBManager_WithoutEnvOptionSetupErrorIsAsBefore(t *testing.T) {
	path := writeConfig(t, "config.yaml", `
entries:
  - name: primary
    type: readwrite
    postgres: {user: app, password: x, host: localhost, port: 5432, database: app, ssl_mode: invalid}
`)
	_, err := mgr.NewDBManager(t.Context(), path, nil)
	var parseErr *pgconn.ParseConfigError
	if !errors.As(err, &parseErr) {
		t.Errorf("want the driver's ParseConfigError: %v", err)
	}
}
