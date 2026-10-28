package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
)

// ValidationError carries per-field messages (Hebrew, user facing).
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("validation failed: %v", e.Fields) }

func NewValidation() *ValidationError { return &ValidationError{Fields: map[string]string{}} }

func (e *ValidationError) Add(field, msg string) { e.Fields[field] = msg }

// OrNil returns nil when no field failed, so callers can `return v.OrNil()`.
func (e *ValidationError) OrNil() error {
	if len(e.Fields) == 0 {
		return nil
	}
	return e
}

// ConflictError is returned when a booking would overlap existing ones.
type ConflictError struct {
	Conflicts []Conflict
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("booking conflicts with %d existing booking(s)", len(e.Conflicts))
}
