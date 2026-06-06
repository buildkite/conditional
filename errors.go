package conditional

import (
	"errors"
	"fmt"
)

// ErrorKind classifies conditional failures without depending on exact server
// error text.
type ErrorKind string

const (
	// ErrorKindParse indicates that the expression could not be parsed.
	ErrorKindParse ErrorKind = "parse"
	// ErrorKindValidation indicates that validation failed before evaluation.
	ErrorKindValidation ErrorKind = "validation"
	// ErrorKindEvaluation indicates that evaluation failed.
	ErrorKindEvaluation ErrorKind = "evaluation"
	// ErrorKindResult indicates that the expression did not evaluate to a bool.
	ErrorKindResult ErrorKind = "result"
)

// Error is a typed conditional error.
type Error struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Message == "" {
		return string(e.Kind)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

// Unwrap returns the underlying cause, if any.
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is reports whether err has the same error kind as target.
func (e *Error) Is(target error) bool {
	targetError, ok := target.(*Error)
	return ok && e.Kind == targetError.Kind
}

// IsErrorKind reports whether err contains a conditional Error with kind.
func IsErrorKind(err error, kind ErrorKind) bool {
	return errors.Is(err, &Error{Kind: kind})
}
