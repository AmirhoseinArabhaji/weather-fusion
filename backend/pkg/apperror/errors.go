// Package apperror defines domain-level error types and HTTP status mapping.
package apperror

import (
	"errors"
	"net/http"
)

// Sentinel errors
var (
	ErrNotFound      = errors.New("resource not found")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrConflict      = errors.New("resource already exists")
	ErrBadRequest    = errors.New("bad request")
	ErrInternal      = errors.New("internal server error")
	ErrTimeout       = errors.New("request timeout")
	ErrRateLimited   = errors.New("rate limit exceeded")
	ErrProviderDown  = errors.New("weather provider unavailable")
)

// AppError wraps a sentinel error with a human-readable message and optional details.
type AppError struct {
	Code    int
	Err     error
	Message string
}

func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Err.Error()
}

func (e *AppError) Unwrap() error { return e.Err }

func New(code int, err error, message string) *AppError {
	return &AppError{Code: code, Err: err, Message: message}
}

// Helper constructors
func NotFound(message string) *AppError {
	return New(http.StatusNotFound, ErrNotFound, message)
}

func Unauthorized(message string) *AppError {
	return New(http.StatusUnauthorized, ErrUnauthorized, message)
}

func Forbidden(message string) *AppError {
	return New(http.StatusForbidden, ErrForbidden, message)
}

func Conflict(message string) *AppError {
	return New(http.StatusConflict, ErrConflict, message)
}

func BadRequest(message string) *AppError {
	return New(http.StatusBadRequest, ErrBadRequest, message)
}

func Internal(message string) *AppError {
	return New(http.StatusInternalServerError, ErrInternal, message)
}

func RateLimited(message string) *AppError {
	return New(http.StatusTooManyRequests, ErrRateLimited, message)
}

func HTTPStatus(err error) int {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return http.StatusInternalServerError
}
