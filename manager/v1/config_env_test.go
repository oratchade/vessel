//go:build test

package v1_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	for ; err != nil; err = errors.Unwrap(err) {
		if strings.Contains(err.Error(), secret) {
			return true
		}
		if s, ok := err.(fmt.Stringer); ok && strings.Contains(s.String(), secret) {
			return true
		}
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
// string and from a comment, each with values that are syntax in some of them.
func TestLoadConfig_TOMLPlaceholderPlacements(t *testing.T) {
	placements := map[string]struct {
		line   string
		values []string
	}{
		"basic string":     {`password = "${DB_PASSWORD}"`, hostileValues},
		"multi-line basic": {`password = """${DB_PASSWORD}"""`, append([]string{"\nleading newline"}, hostileValues...)},
		// A literal string reads no escapes, so a backslash is a backslash.
		"literal string": {`password = '${DB_PASSWORD}'`, []string{`back\slash`, `"double"`, "*star", "tab\there"}},
		"multi-line literal": {
			`password = '''${DB_PASSWORD}'''`,
			[]string{`back\slash`, "line\nbreak", "it's", "ends with '", "ends with ''"},
		},
		// A comment keeps the placeholder as written: a value there could end it.
		"comment": {
			"password = \"${DB_PASSWORD}\" # was ${DB_PASSWORD}",
			[]string{"x\nport = 1", `back\slash`},
		},
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

// TestLoadConfig_RefusesAValueItsPlaceCannotHold covers values that used to load as
// something else: a TOML literal string ended early, or a value outside quotes became
// syntax. The load fails, naming the placeholder and not the value.
func TestLoadConfig_RefusesAValueItsPlaceCannotHold(t *testing.T) {
	cases := map[string]struct {
		name, content, value string
	}{
		"TOML literal string, quote":   {"config.toml", tomlWith(`password = '${DB_PASSWORD}'`, "5432"), "x' #"},
		"TOML literal string, newline": {"config.toml", tomlWith(`password = '${DB_PASSWORD}'`, "5432"), "a\nb"},
		"TOML multi-line literal":      {"config.toml", tomlWith(`password = '''${DB_PASSWORD}'''`, "5432"), "a'''b"},
		// TOML drops a newline right after the opening quotes.
		"TOML multi-line literal, leading newline": {
			"config.toml", tomlWith(`password = '''${DB_PASSWORD}'''`, "5432"), "\nleading newline",
		},
		"TOML outside quotes": {"config.toml", tomlWith(`password = "x"`, "${DB_PASSWORD}"), "1\nuser = \"root\""},
		"JSON outside quotes": {
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

// TestLoadConfig_YAMLDefaultIsFileSyntax: a default is written by the file's author, so
// it goes in as written, as in JSON and TOML, and YAML reads it in its place.
func TestLoadConfig_YAMLDefaultIsFileSyntax(t *testing.T) {
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
	if pg := cfg.Entries[0].Postgres; pg.Password != "a\tb" || pg.Port != 5432 {
		t.Errorf("password, port = %q, %d; want the escape read, a\\tb, and 5432", pg.Password, pg.Port)
	}
}

// TestLoadConfig_WithholdingCountsLinesInTheDecodedText puts a value with line breaks
// above a value the decoder rejects. The decoder counts lines in the expanded text, so
// the withheld line must be counted there too, or the message quotes the value.
func TestLoadConfig_WithholdingCountsLinesInTheDecodedText(t *testing.T) {
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

// TestLoadConfig_WithholdsAJSONErrorAtTheEndOfTheLine: a JSON syntax error found on the
// line break after a value points one byte past it, on the next line.
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

// TestLoadConfig_TOMLLineBreakAfterAQuoteIsKept: in ''''${P}''' the fourth quote is the
// string's first character, so a line break at the start of the value follows it and
// TOML keeps it; only one right after the opening quotes is dropped.
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

// TestLoadConfig_YAMLDefaultCannotForgeAToken: a default goes in before parsing, next to
// the text around it, so together they must not spell a token standing in for another
// variable's value.
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
// string, TOML drops the whitespace after a line-ending backslash, so a value placed
// there loses its leading spaces unless they are escaped.
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
