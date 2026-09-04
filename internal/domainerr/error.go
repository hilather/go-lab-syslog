package domainerr

import (
	"errors"
	"fmt"
)

// Error is a catalogued domain error.
type Error struct {
	Code   Code
	Detail string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Detail
}

// New constructs a domain error.
func New(code Code, detail string) *Error {
	return &Error{Code: code, Detail: detail}
}

// Newf constructs a domain error with a formatted detail.
func Newf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// Is reports whether err is a domain error with the given code.
func Is(err error, code Code) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

// As extracts a domain error from err.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
