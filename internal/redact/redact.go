// Package redact removes secrets from what leaves vessel: error messages, error chains
// and log lines. The manager, the drivers' direct constructors and the migrator share it,
// so a password is redacted the same way on every path (#86, #96).
package redact

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// Error is err with each of values redacted, or err itself when there are none. It
// is a new error rather than a wrapper, since a decoder's error can hold what it read,
// as a TOML error holds its source; it unwraps only to the causes that carry no input.
func Error(err error, values []string, keep ...error) error {
	if len(values) == 0 {
		return err
	}
	return Cut(err, values, keep...)
}

// Cut is err as a new error, its message redacted of values, that unwraps only to the
// causes holding no input: an errno, a context error, or one of keep (a package's own
// sentinels, which a caller tests with errors.Is).
func Cut(err error, values []string, keep ...error) error {
	redacted := &redactedError{msg: Message(err.Error(), Forms(values))}
	if errno := syscall.Errno(0); errors.As(err, &errno) {
		redacted.causes = append(redacted.causes, errno)
	}
	for _, cause := range append([]error{context.Canceled, context.DeadlineExceeded}, keep...) {
		if errors.Is(err, cause) {
			redacted.causes = append(redacted.causes, cause)
		}
	}
	return redacted
}

// redactedError is an error's message redacted. Its causes are what a caller retries
// on, an errno or a context error, which hold no value.
type redactedError struct {
	msg    string
	causes []error
}

func (e *redactedError) Error() string   { return e.msg }
func (e *redactedError) Unwrap() []error { return e.causes }

// Forms are the forms of the substituted values to replace in a message, longest
// first so a value holding another is replaced whole: a decoder quotes the text it
// rejects, and that text may be a secret (#85). A short value can redact more than
// itself; a message with too much redacted is the safe side.
func Forms(values []string) []string {
	forms := make([]string, 0, len(values))
	for _, val := range values {
		forms = append(forms, redactedForms(val)...)
	}
	forms = slices.DeleteFunc(forms, func(form string) bool { return form == "" })
	slices.SortFunc(forms, func(a, b string) int { return len(b) - len(a) })
	return forms
}

// Message replaces each of forms in msg, in order.
func Message(msg string, forms []string) string {
	for _, form := range forms {
		msg = strings.ReplaceAll(msg, form, "[REDACTED]")
	}
	return msg
}

// redactedForms are the forms a value can take in a message: as the environment holds
// it, trimmed as a decoder trims an unquoted value, escaped as %q prints it, and escaped
// as a connection string's user, password or parameter.
func redactedForms(val string) []string {
	forms := make([]string, 0, 8)
	for _, v := range []string{val, strings.TrimSpace(val)} {
		quoted := strconv.Quote(v)
		forms = append(forms, v, quoted[1:len(quoted)-1], url.User(v).String(), url.QueryEscape(v))
	}
	slices.Sort(forms)
	return slices.Compact(forms)
}
