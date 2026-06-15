package apperror

import (
	"fmt"
)

// ErrType defines the class of error (Validation, NotFound, etc.)
type ErrType string

const (
	TypeValidation ErrType = "VALIDATION"
	TypeNotFound   ErrType = "NOT_FOUND"
	TypeConflict   ErrType = "CONFLICT"
	TypeInternal   ErrType = "INTERNAL"
)

// AppError is an error structure carrying type, code, message, and the original error.
type AppError struct {
	Type    ErrType `json:"-"`             // Internal class classification, hidden in JSON response
	Code    string  `json:"code"`          // Domain-specific error code (e.g. INSUFFICIENT_FUNDS)
	Message string  `json:"message"`       // User-friendly error message
	Err     error   `json:"-"`             // Root cause error for debugging/logging, hidden in JSON
}

// Error implements the standard Go error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying root cause error, implementing Go's standard unwrap interface
func (e *AppError) Unwrap() error {
	return e.Err
}

// New creates a new custom AppError
func New(errType ErrType, code string, message string, err error) *AppError {
	return &AppError{
		Type:    errType,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// NewValidationError creates a validation class error
func NewValidationError(code string, message string) *AppError {
	return New(TypeValidation, code, message, nil)
}

// NewNotFoundError creates a resource not found class error
func NewNotFoundError(code string, message string) *AppError {
	return New(TypeNotFound, code, message, nil)
}

// NewConflictError creates a resource conflict class error (e.g., duplicate/locking keys)
func NewConflictError(code string, message string) *AppError {
	return New(TypeConflict, code, message, nil)
}

// NewInternalError creates a system internal class error wrapping the root cause
func NewInternalError(code string, message string, err error) *AppError {
	return New(TypeInternal, code, message, err)
}
