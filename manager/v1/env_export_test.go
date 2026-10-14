//go:build test

package v1

import "tounilab.com/vessel/manager/v1/config"

// LoadConfigForTest exposes loadConfig for testing.
func LoadConfigForTest(path string, opts []EnvOption) (*config.ManagerConfig, error) {
	return loadConfig(path, opts)
}

// ExpandEnvVarsForTest exposes expandEnvVarsWith, values inserted as they are.
func ExpandEnvVarsForTest(data []byte, opts []EnvOption) []byte {
	return expandEnvVarsWith(data, newEnvResolver(opts), func(val string) string { return val })
}
