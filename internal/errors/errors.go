// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package errors

import (
	"errors"
	"fmt"
)

// Kind defines the category of error.
type Kind int

const (
	KindUnknown Kind = iota
	KindInternal
	KindValidation
	KindNotFound
	KindPermission
	KindConflict
	KindUnavailable
	KindTimeout
)

func (k Kind) String() string {
	switch k {
	case KindInternal:
		return "internal"
	case KindValidation:
		return "validation"
	case KindNotFound:
		return "not_found"
	case KindPermission:
		return "permission"
	case KindConflict:
		return "conflict"
	case KindUnavailable:
		return "unavailable"
	case KindTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// Error represents a structured error in the flywall system.
type Error struct {
	Kind       Kind
	Message    string
	Underlying error
	Attributes map[string]any
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Underlying)
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Underlying
}

// New creates a new Error of the specified kind.
func New(kind Kind, msg string) error {
	return &Error{
		Kind:    kind,
		Message: msg,
	}
}

// Errorf creates a new Error of the specified kind with a formatted message.
func Errorf(kind Kind, format string, args ...any) error {
	return &Error{
		Kind:    kind,
		Message: fmt.Sprintf(format, args...),
	}
}

// Wrap wraps an existing error as a new Error of the specified kind.
func Wrap(err error, kind Kind, msg string) error {
	if err == nil {
		return nil
	}
	return &Error{
		Kind:       kind,
		Message:    msg,
		Underlying: err,
	}
}

// Wrapf wraps an existing error as a new Error of the specified kind with a formatted message.
func Wrapf(err error, kind Kind, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return &Error{
		Kind:       kind,
		Message:    fmt.Sprintf(format, args...),
		Underlying: err,
	}
}

// Attr attaches an attribute to an error. If the error is not an *Error, it wraps it as KindInternal.
func Attr(err error, key string, val any) error {
	if err == nil {
		return nil
	}

	var e *Error
	if !errors.As(err, &e) {
		e = &Error{
			Kind:       KindInternal,
			Message:    err.Error(),
			Underlying: err,
		}
	}

	if e.Attributes == nil {
		e.Attributes = make(map[string]any)
	}
	e.Attributes[key] = val
	return e
}

// GetKind returns the Kind of the error, or KindUnknown if it's not a flywall error.
func GetKind(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindUnknown
}

// GetAttributes returns all attributes associated with the error and its chain.
func GetAttributes(err error) map[string]any {
	attrs := make(map[string]any)
	var e *Error

	// We use errors.As in a loop to collect all attributes in the chain
	// although typically we only have one flywall error in the chain.
	tempErr := err
	for tempErr != nil {
		if errors.As(tempErr, &e) {
			for k, v := range e.Attributes {
				if _, ok := attrs[k]; !ok {
					attrs[k] = v
				}
			}
			tempErr = e.Underlying
		} else {
			break
		}
	}

	return attrs
}

// Is reports whether any error in err's chain matches target.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target, and if so, sets target to that error value and returns true.
func As(err error, target any) bool {
	return errors.As(err, target)
}

// Unwrap returns the result of calling the Unwrap method on err, if err's type contains an Unwrap method returning error.
func Unwrap(err error) error {
	return errors.Unwrap(err)
}
