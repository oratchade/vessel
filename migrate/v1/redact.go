package v1

import (
	db "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/internal/redact"
)

// redactError is err with secrets redacted from its message and its chain cut, keeping
// this package's sentinels for a caller's errors.Is.
func redactError(err error, secrets []string) error {
	return redact.Error(err, secrets, ErrNoMigrations, ErrMySQLParseTime, ErrUnsupportedDriver)
}

// passwords are the password cfg connects with, none when it holds none.
func passwords(cfg db.DBConfig) []string {
	if password := redact.Password(cfg); password != "" {
		return []string{password}
	}
	return nil
}
