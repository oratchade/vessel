package v1

import "tounilab.com/vessel/internal/redact"

// redactConfigError is err with cfg's password redacted from its message and its chain
// cut (#96): a driver's error can quote the connection string it was given.
func redactConfigError(err error, cfg DBConfig) error {
	if password := redact.Password(cfg); password != "" {
		return redact.Cut(err, []string{password})
	}
	return err
}
