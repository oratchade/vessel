//go:build test

package v1_test

import (
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
