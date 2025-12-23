package errors

import (
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
)

// Common error codes organized by domain
const (
	// Authentication/Authorization errors
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"
	ErrCodeAuthFailed   ErrorCode = "AUTH_FAILED"
	ErrCodeTokenExpired ErrorCode = "TOKEN_EXPIRED"
	ErrCodeInvalidToken ErrorCode = "INVALID_TOKEN"

	// Validation errors
	ErrCodeValidation    ErrorCode = "VALIDATION_ERROR"
	ErrCodeInvalidInput  ErrorCode = "INVALID_INPUT"
	ErrCodeMissingField  ErrorCode = "MISSING_FIELD"
	ErrCodeInvalidFormat ErrorCode = "INVALID_FORMAT"

	// Resource errors
	ErrCodeNotFound          ErrorCode = "NOT_FOUND"
	ErrCodeConflict          ErrorCode = "CONFLICT"
	ErrCodeAlreadyExists     ErrorCode = "ALREADY_EXISTS"
	ErrCodeResourceExhausted ErrorCode = "RESOURCE_EXHAUSTED"

	// System errors
	ErrCodeInternal    ErrorCode = "INTERNAL_ERROR"
	ErrCodeTimeout     ErrorCode = "TIMEOUT"
	ErrCodeUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeRateLimited ErrorCode = "RATE_LIMITED"

	// External service errors
	ErrCodeExternalAPI     ErrorCode = "EXTERNAL_API_ERROR"
	ErrCodeExternalTimeout ErrorCode = "EXTERNAL_TIMEOUT"
)

// HTTPStatusCode returns the HTTP status code for an error code
func (c ErrorCode) HTTPStatusCode() int {
	switch c {
	case ErrCodeUnauthorized, ErrCodeAuthFailed, ErrCodeTokenExpired, ErrCodeInvalidToken:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeValidation, ErrCodeInvalidInput, ErrCodeMissingField, ErrCodeInvalidFormat:
		return http.StatusBadRequest
	case ErrCodeConflict, ErrCodeAlreadyExists:
		return http.StatusConflict
	case ErrCodeRateLimited:
		return http.StatusTooManyRequests
	case ErrCodeTimeout, ErrCodeExternalTimeout:
		return http.StatusRequestTimeout
	case ErrCodeUnavailable:
		return http.StatusServiceUnavailable
	case ErrCodeResourceExhausted:
		return http.StatusInsufficientStorage
	case ErrCodeExternalAPI:
		return http.StatusBadGateway
	case ErrCodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// NotFoundError creates a not found error
func NotFoundError(resource string) *Error {
	err := errors.Newf("%s not found", resource)
	err = errors.WithStack(err)

	return &Error{
		Code:     ErrCodeNotFound,
		Message:  fmt.Sprintf("%s not found", resource),
		Severity: SeverityError,
		meta: map[string]any{
			"resource": resource,
		},
		cause: err,
	}
}

// ValidationError creates a validation error
func ValidationError(message string) *Error {
	return &Error{
		Code:     ErrCodeValidation,
		Message:  message,
		Severity: SeverityError,
		meta:     make(map[string]any),
		cause:    errors.New(message),
	}
}

// ValidationErrorWithDetails creates a validation error with field-level details
func ValidationErrorWithDetails(message string, fields map[string]string) *Error {
	return &Error{
		Code:     ErrCodeValidation,
		Message:  message,
		Severity: SeverityError,
		meta: map[string]any{
			"validation_errors": fields,
		},
		cause: errors.New(message),
	}
}

// AuthError creates an authentication error
func AuthError(message string) *Error {
	return &Error{
		Code:     ErrCodeUnauthorized,
		Message:  message,
		Severity: SeverityError,
		meta:     make(map[string]any),
		cause:    errors.New(message),
	}
}

// ForbiddenError creates a forbidden error
func ForbiddenError(message string) *Error {
	return &Error{
		Code:     ErrCodeForbidden,
		Message:  message,
		Severity: SeverityError,
		meta:     make(map[string]any),
		cause:    errors.New(message),
	}
}

// InternalError creates an internal server error
func InternalError(message string, cause error) *Error {
	if cause == nil {
		cause = errors.New(message)
	} else {
		cause = errors.Wrap(cause, message)
	}

	return &Error{
		Code:     ErrCodeInternal,
		Message:  message,
		Severity: SeverityError,
		meta:     make(map[string]any),
		cause:    cause,
	}
}

// ConflictError creates a conflict error
func ConflictError(message string) *Error {
	return &Error{
		Code:     ErrCodeConflict,
		Message:  message,
		Severity: SeverityError,
		meta:     make(map[string]any),
		cause:    errors.New(message),
	}
}

// TimeoutError creates a timeout error
func TimeoutError(operation string, duration any) *Error {
	return &Error{
		Code:     ErrCodeTimeout,
		Message:  fmt.Sprintf("%s timed out after %v", operation, duration),
		Severity: SeverityError,
		meta: map[string]any{
			"operation": operation,
			"duration":  duration,
		},
		cause: errors.Newf("%s timed out", operation),
	}
}

// RateLimitError creates a rate limit error
func RateLimitError(retryAfter any) *Error {
	return &Error{
		Code:     ErrCodeRateLimited,
		Message:  "rate limit exceeded",
		Severity: SeverityError,
		meta: map[string]any{
			"retry_after": retryAfter,
		},
		cause: errors.New("rate limit exceeded"),
	}
}

// ExternalAPIError creates an external service error
func ExternalAPIError(service string, cause error) *Error {
	if cause == nil {
		cause = errors.Newf("external API error: %s", service)
	} else {
		cause = errors.Wrap(cause, fmt.Sprintf("external API error: %s", service))
	}

	return &Error{
		Code:     ErrCodeExternalAPI,
		Message:  fmt.Sprintf("external service %s error", service),
		Severity: SeverityError,
		meta: map[string]any{
			"service": service,
		},
		cause: cause,
	}
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code == ErrCodeNotFound
	}

	return false
}

// IsValidation checks if an error is a validation error
func IsValidation(err error) bool {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code == ErrCodeValidation || appErr.Code == ErrCodeInvalidInput ||
			appErr.Code == ErrCodeMissingField || appErr.Code == ErrCodeInvalidFormat
	}

	return false
}

// IsAuth checks if an error is an authentication error
func IsAuth(err error) bool {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code == ErrCodeUnauthorized || appErr.Code == ErrCodeAuthFailed ||
			appErr.Code == ErrCodeTokenExpired || appErr.Code == ErrCodeInvalidToken
	}

	return false
}

// IsForbidden checks if an error is a forbidden error
func IsForbidden(err error) bool {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code == ErrCodeForbidden
	}

	return false
}

// IsInternal checks if an error is an internal error
func IsInternal(err error) bool {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code == ErrCodeInternal
	}

	return false
}

// IsTimeout checks if an error is a timeout error
func IsTimeout(err error) bool {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code == ErrCodeTimeout || appErr.Code == ErrCodeExternalTimeout
	}

	return false
}

// IsRateLimited checks if an error is a rate limit error
func IsRateLimited(err error) bool {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code == ErrCodeRateLimited
	}

	return false
}

// GetCode returns the error code from an error
func GetCode(err error) ErrorCode {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}

	return ErrCodeInternal
}

// GetStatusCode returns the HTTP status code for an error
func GetStatusCode(err error) int {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code.HTTPStatusCode()
	}

	return http.StatusInternalServerError
}

// GetSeverity returns the severity level of an error
func GetSeverity(err error) Severity {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Severity
	}

	return SeverityError
}

// GetMetadata returns the metadata from an error
func GetMetadata(err error) map[string]any {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Metadata()
	}

	return nil
}
