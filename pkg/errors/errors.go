// Package errors provides a production-ready error handling wrapper around
// github.com/cockroachdb/errors, offering structured errors with stack traces,
// error codes, severity levels, and metadata support.
//
// This package follows the same wrapper pattern as pkg/logger, providing a
// clean, domain-specific API while leveraging battle-tested error handling
// capabilities from cockroachdb/errors.
//
// Basic usage:
//
//	import "github.com/kannancmohan/go-prototype-authz-cerbos/pkg/errors"
//
//	// Create a new error with stack trace
//	err := errors.New(errors.ErrCodeNotFound, "user not found")
//
//	// Wrap an existing error with additional context
//	err := errors.Wrap(dbErr, errors.ErrCodeInternal, "failed to fetch user")
//
//	// Add metadata to errors
//	err := errors.ValidationError("invalid email", map[string]string{
//	    "field": "email",
//	    "value": "invalid-email",
//	})
//
//	// Log errors with automatic metadata extraction
//	err.Log(logger).WithMetadata("user_id", userID)
//
//	// Check error types
//	if errors.IsNotFound(err) {
//	    // Handle not found error
//	}
package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/cockroachdb/errors"
	"go.opentelemetry.io/otel/trace"
)

// ErrorCode represents a domain-specific error code
type ErrorCode string

// Severity represents the severity level of an error
type Severity string

const (
	// SeverityError represents a critical error that should be logged and alerted
	SeverityError Severity = "error"
	// SeverityWarning represents a warning that should be logged but may not require immediate action
	SeverityWarning Severity = "warning"
	// SeverityInfo represents an informational message
	SeverityInfo Severity = "info"
)

// Error represents a structured error with code, message, severity, metadata, and stack trace
type Error struct {
	// Code is the domain-specific error code
	Code ErrorCode `json:"code"`
	// Message is the human-readable error message
	Message string `json:"message"`
	// Severity indicates the error severity level
	Severity Severity `json:"severity"`
	// meta contains structured error details (lowercase to avoid conflict with method)
	meta map[string]any
	// cause is the underlying error (if any)
	cause error
}

// Error implements the error interface
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.cause != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.cause)
	}

	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for error chain traversal
//
//nolint:wrapcheck // Unwrap should not wrap errors to preserve error chain traversal
func (e *Error) Unwrap() error {
	if e == nil || e.cause == nil {
		return nil
	}

	// Try to extract the original error from cockroachdb/errors wrapper
	if wrapped, ok := e.cause.(interface{ Unwrap() error }); ok {
		if unwrapped := wrapped.Unwrap(); unwrapped != nil {
			// If the unwrapped error is still a wrapper, keep unwrapping
			if _, isWrapper := unwrapped.(interface{ Unwrap() error }); isWrapper {
				return unwrapped
			}

			return unwrapped
		}
	}

	// Try Cause() method if available (some cockroachdb/errors types have this)
	if causer, ok := e.cause.(interface{ Cause() error }); ok {
		if cause := causer.Cause(); cause != nil {
			return cause
		}
	}

	return e.cause
}

// Is implements errors.Is support for error comparison
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}

	return e.Code == t.Code
}

// As implements errors.As support for error type checking
func (e *Error) As(target any) bool {
	switch t := target.(type) {
	case **Error:
		*t = e
		return true
	default:
		return false
	}
}

// StackTrace returns the formatted stack trace as a string
func (e *Error) StackTrace() string {
	if e == nil || e.cause == nil {
		return ""
	}

	// Format the error with verbose output to get stack trace
	// cockroachdb/errors includes stack traces in %+v formatting
	return fmt.Sprintf("%+v", e.cause)
}

// New creates a new error with the given code and message, automatically capturing a stack trace
func New(code ErrorCode, message string) *Error {
	// Create base error with stack trace using cockroachdb/errors
	baseErr := errors.New(message)
	baseErr = errors.WithStackDepth(baseErr, 1)

	return &Error{
		Code:     code,
		Message:  message,
		Severity: SeverityError,
		meta:     make(map[string]any),
		cause:    baseErr,
	}
}

// Wrap wraps an existing error with additional context, code, and message, preserving the stack trace
func Wrap(err error, code ErrorCode, message string) *Error {
	if err == nil {
		return nil
	}

	// If err is already an *Error, preserve its metadata and severity
	var existingErr *Error
	if errors.As(err, &existingErr) {
		return &Error{
			Code:     code,
			Message:  message,
			Severity: existingErr.Severity,
			meta:     existingErr.meta,
			cause:    existingErr.cause,
		}
	}

	// Wrap the error with cockroachdb/errors to capture stack trace
	wrappedErr := errors.Wrap(err, message)
	wrappedErr = errors.WithStackDepth(wrappedErr, 1)

	return &Error{
		Code:     code,
		Message:  message,
		Severity: SeverityError,
		meta:     make(map[string]any),
		cause:    wrappedErr,
	}
}

// WithMetadata adds a metadata key-value pair to the error and returns a new error instance
func (e *Error) WithMetadata(key string, value any) *Error {
	// Create a copy to maintain immutability
	meta := make(map[string]any, len(e.meta)+1)
	for k, v := range e.meta {
		meta[k] = v
	}
	meta[key] = value

	return &Error{
		Code:     e.Code,
		Message:  e.Message,
		Severity: e.Severity,
		meta:     meta,
		cause:    e.cause,
	}
}

// WithMetadataMap adds multiple metadata key-value pairs to the error
func (e *Error) WithMetadataMap(metadata map[string]any) *Error {
	// Create a copy to maintain immutability
	newMeta := make(map[string]any, len(e.meta)+len(metadata))
	for k, v := range e.meta {
		newMeta[k] = v
	}
	for k, v := range metadata {
		newMeta[k] = v
	}

	return &Error{
		Code:     e.Code,
		Message:  e.Message,
		Severity: e.Severity,
		meta:     newMeta,
		cause:    e.cause,
	}
}

// Metadata returns the error's metadata map
func (e *Error) Metadata() map[string]any {
	// Return a copy to prevent external modification
	meta := make(map[string]any, len(e.meta))
	maps.Copy(meta, e.meta)

	return meta
}

// MarshalJSON implements json.Marshaler for custom JSON serialization
func (e *Error) MarshalJSON() ([]byte, error) {
	result := map[string]any{
		"code":     e.Code,
		"message":  e.Message,
		"severity": e.Severity,
	}
	if len(e.meta) > 0 {
		result["metadata"] = e.meta
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal error to JSON: %w", err)
	}

	return data, nil
}

// WithContext enriches the error with context values (trace IDs, request IDs, etc.)
func (e *Error) WithContext(ctx context.Context) *Error {
	if ctx == nil {
		return e
	}

	//nolint:mnd // +2 reserves space for trace_id and span_id - standard pattern for map pre-allocation
	meta := make(map[string]any, len(e.meta)+2)

	// Copy existing metadata
	maps.Copy(meta, e.meta)

	// Extract trace context if available
	if span := trace.SpanFromContext(ctx); span != nil {
		spanCtx := span.SpanContext()
		if spanCtx.IsValid() {
			meta["trace_id"] = spanCtx.TraceID().String()
			meta["span_id"] = spanCtx.SpanID().String()
		}
	}

	// Extract common context values
	if reqID := ctx.Value("request_id"); reqID != nil {
		meta["request_id"] = reqID
	}
	if userID := ctx.Value("user_id"); userID != nil {
		meta["user_id"] = userID
	}

	return &Error{
		Code:     e.Code,
		Message:  e.Message,
		Severity: e.Severity,
		meta:     meta,
		cause:    e.cause,
	}
}

// WithSeverity sets the error severity level
func (e *Error) WithSeverity(severity Severity) *Error {
	return &Error{
		Code:     e.Code,
		Message:  e.Message,
		Severity: severity,
		meta:     e.meta,
		cause:    e.cause,
	}
}
