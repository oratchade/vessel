//go:build test

package v1_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	mgr "tounilab.com/vessel/manager/v1"
)

// The fuzz targets write text before and after a placeholder in each kind of string and
// check that a load never panics and never hands a field what stood in for it: a token
// is eighteen digits, so a field holding ten or more in a row came from one, as long as
// the text written around it holds no long run itself. Their seeds run with every
// go test; go test -fuzz explores from them.

var (
	standInDigits = regexp.MustCompile(`[0-9]{10,}`)
	writtenDigits = regexp.MustCompile(`[0-9]{5,}`)
)

// fuzzSeeds are text the file's syntax reads around a placeholder: escapes, the start of
// a number, quotes, a comment.
func fuzzSeeds(f *testing.F) {
	for _, seed := range []string{
		"", "\\u", "\\uD8", "\\u00", "\\x", "\\U", "\\UD8", "\\\\", "\\", "\\N", "\\_",
		"0x", "0o", "1e", ".", "-", "+", "_", "#", "'", `"`, "${Y}", "\\u${Y}",
	} {
		f.Add(seed, "")
		f.Add("", seed)
	}
}

func fuzzLoad(t *testing.T, before, after, name string, file func(string) string) {
	t.Helper()
	if writtenDigits.MatchString(before + after) {
		t.Skip()
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(file(before+"${X}"+after)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	cfg, err := mgr.LoadConfigForTest(name, []mgr.EnvOption{
		mgr.WithEnvVars(map[string]string{"X": "vv", "Y": "ww"}),
	})
	if err != nil || len(cfg.Entries) == 0 || cfg.Entries[0].Postgres == nil {
		return
	}
	if password := cfg.Entries[0].Postgres.Password; standInDigits.MatchString(password) {
		t.Fatalf("password %q holds a stand-in, from %q", password, before+"${X}"+after)
	}
}

func FuzzLoadConfig_JSONString(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, before, after string) {
		fuzzLoad(t, before, after, "config.json", func(password string) string {
			return jsonPassword(`"` + password + `"`)
		})
	})
}

func FuzzLoadConfig_YAMLString(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, before, after string) {
		fuzzLoad(t, before, after, "config.yaml", func(password string) string {
			return yamlPassword(`"` + password + `"`)
		})
		fuzzLoad(t, before, after, "config.yaml", yamlPassword)
	})
}

func FuzzLoadConfig_TOMLString(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, before, after string) {
		fuzzLoad(t, before, after, "config.toml", func(password string) string {
			return tomlWith(`password = "`+password+`"`, "5432")
		})
		fuzzLoad(t, before, after, "config.toml", func(password string) string {
			return tomlWith(`password = '`+password+`'`, "5432")
		})
	})
}

// fuzzKeySeeds are keys written beside a key holding ${K}: the same text spelled with an
// escape, the field it expands to in another case, and stand-in or token digits.
func fuzzKeySeeds(f *testing.F) {
	for _, key := range []string{"", "${K}", "$\\u007bK}", "postgres", "Postgres", "<", `"`, "81111", "70000\\u003"} {
		f.Add(key, "\\uD8")
		f.Add(key, "")
	}
}

// fuzzKeys loads a file holding a fuzzed key beside ${K}, with text before a placeholder
// under ${K}, four times: the loads must agree, and none may hand a field a stand-in.
func fuzzKeys(t *testing.T, key, before, name string, file func(key, password string) string) {
	t.Helper()
	if writtenDigits.MatchString(key + before) {
		t.Skip()
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(file(key, before+"${X}")), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var first string
	for i := range 4 {
		cfg, err := mgr.LoadConfigForTest(name, []mgr.EnvOption{
			mgr.WithEnvVars(map[string]string{"X": "vv", "K": "postgres"}),
		})
		loaded, _ := json.Marshal(cfg)
		got := fmt.Sprintf("%v %s", err, loaded)
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatalf("loads disagree:\n%s\n%s", first, got)
		}
	}
	if standInDigits.MatchString(first) {
		t.Fatalf("a stand-in reached the result: %s", first)
	}
}

func FuzzLoadConfig_JSONKey(f *testing.F) {
	fuzzKeySeeds(f)
	f.Fuzz(func(t *testing.T, key, before string) {
		fuzzKeys(t, key, before, "config.json", func(key, password string) string {
			return `{"entries": [{"name": "primary", "type": "readwrite", "${K}": {"user": "app", "password": "` +
				password + `", "host": "localhost", "port": 5432, "database": "app"}, "` + key + `": {}}]}`
		})
	})
}

func FuzzLoadConfig_YAMLKey(f *testing.F) {
	fuzzKeySeeds(f)
	f.Fuzz(func(t *testing.T, key, before string) {
		fuzzKeys(t, key, before, "config.yaml", func(key, password string) string {
			return "entries:\n  - name: primary\n    type: readwrite\n    ${K}: {user: app, password: \"" + password +
				"\", host: localhost, port: 5432, database: app}\n    \"" + key + "\": {}\n"
		})
	})
}

func FuzzLoadConfig_TOMLKey(f *testing.F) {
	fuzzKeySeeds(f)
	f.Fuzz(func(t *testing.T, key, before string) {
		fuzzKeys(t, key, before, "config.toml", func(key, password string) string {
			return "[[entries]]\nname = \"primary\"\ntype = \"readwrite\"\n\"" + key + "\" = {}\n[entries.${K}]\n" +
				"user = \"app\"\npassword = \"" + password + "\"\nhost = \"localhost\"\nport = 5432\ndatabase = \"app\"\n"
		})
	})
}
