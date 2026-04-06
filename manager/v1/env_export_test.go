//go:build test

package v1

// ExpandEnvVarsForTest exposes expandEnvVars for testing.
func ExpandEnvVarsForTest(data []byte, opts []EnvOption) []byte {
	return expandEnvVars(data, newEnvResolver(opts))
}
