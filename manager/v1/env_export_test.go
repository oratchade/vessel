//go:build test

package v1

import "tounilab.com/vessel/manager/v1/config"

// LoadConfigForTest exposes loadConfig for testing.
func LoadConfigForTest(path string, opts []EnvOption) (*config.ManagerConfig, error) {
	cfg, _, err := loadConfig(path, opts)
	return cfg, err
}

// ExpandEnvVarsForTest runs the resolution rules (envResolver.expand) over each
// placeholder in data, inserting what it resolves to as text. Production never
// expands text; this exercises the rules and the sources' precedence:
//   - ${VAR} with var found → replaced with value
//   - ${VAR} with var not found → left as literal ${VAR} (fail-loud)
//   - ${VAR:default} with var found → replaced with value
//   - ${VAR:default} with var not found → replaced with default
//   - ${VAR:} with var not found → replaced with empty string
func ExpandEnvVarsForTest(data []byte, opts []EnvOption) []byte {
	resolver := newEnvResolver(opts)
	if !resolver.hasAnySource() {
		return data
	}
	return envVarRe.ReplaceAllFunc(data, func(match []byte) []byte {
		val, _, err := resolver.expand(match)
		if err != nil {
			return match
		}
		return []byte(val)
	})
}
