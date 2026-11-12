package setting

import (
	"errors"
	"net/url"
)

// ErrorKind names the failures the error dialog explains in plain words. Anything else is shown
// as "Something went wrong" with the original text.
type ErrorKind int

const (
	ErrorOther        ErrorKind = iota
	ErrorInvalidKey             // the service did not accept the API key
	ErrorNetwork                // the service could not be reached
	ErrorUnknownUser            // wallhaven has no such user, or the account is private
	ErrorNoCollection           // the wallhaven account has no collections to sync
)

// KnownError is an error that the error dialog can explain. Error() is the original text, so logs
// and tests see what they always saw.
type KnownError struct {
	Kind   ErrorKind
	Source string // the service's name as the person knows it: "Pexels", "wallhaven"
	Err    error
}

func (e *KnownError) Error() string { return e.Err.Error() }
func (e *KnownError) Unwrap() error { return e.Err }

// Known tags err as a failure of the given kind from the given service.
func Known(kind ErrorKind, source string, err error) error {
	if err == nil {
		return nil
	}
	return &KnownError{Kind: kind, Source: source, Err: err}
}

// Classify returns the kind and the service of err. A network failure is recognised even when
// nothing tagged it.
func Classify(err error) (ErrorKind, string) {
	var known *KnownError
	if errors.As(err, &known) {
		return known.Kind, known.Source
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return ErrorNetwork, ""
	}
	return ErrorOther, ""
}
