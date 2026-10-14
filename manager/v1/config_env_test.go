//go:build test

package v1_test

import (
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
