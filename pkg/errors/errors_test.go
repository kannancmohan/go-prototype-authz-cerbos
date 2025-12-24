package errors

import (
	"context"
	"encoding/json"
	"errors"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	cockroachdbErrors "github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

// TestErrorCreation tests basic error creation
func TestErrorCreation(t *testing.T) {
	tests := []struct {
		name        string
		code        ErrorCode
		message     string
		wantCode    ErrorCode
		wantMessage string
	}{
		{
			name:        "create internal error",
			code:        ErrCodeInternal,
			message:     "something went wrong",
			wantCode:    ErrCodeInternal,
			wantMessage: "something went wrong",
		},
		{
			name:        "create not found error",
			code:        ErrCodeNotFound,
			message:     "user not found",
			wantCode:    ErrCodeNotFound,
			wantMessage: "user not found",
		},
		{
			name:        "create validation error",
			code:        ErrCodeValidation,
			message:     "invalid input",
			wantCode:    ErrCodeValidation,
			wantMessage: "invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.code, tt.message)

			require.NotNil(t, err)
			assert.Equal(t, tt.wantCode, err.Code)
			assert.Equal(t, tt.wantMessage, err.Message)
			assert.Equal(t, SeverityError, err.Severity)
			assert.NotNil(t, err.meta)
			assert.Error(t, err.cause)
		})
	}
}

// TestErrorWrapping tests error wrapping functionality
func TestErrorWrapping(t *testing.T) {
	baseErr := stderrors.New("database connection failed")

	wrapped := Wrap(baseErr, ErrCodeInternal, "failed to fetch user")

	require.NotNil(t, wrapped)
	assert.Equal(t, ErrCodeInternal, wrapped.Code)
	assert.Equal(t, "failed to fetch user", wrapped.Message)
	assert.Equal(t, SeverityError, wrapped.Severity)

	// Unwrap should return the cause chain (may be wrapped by cockroachdb/errors)
	unwrapped := wrapped.Unwrap()
	require.Error(t, unwrapped)

	// The unwrapped error should contain the original error message
	assert.Contains(t, unwrapped.Error(), "database connection failed")

	// Test error message includes both messages
	assert.Contains(t, wrapped.Error(), "failed to fetch user")
	assert.Contains(t, wrapped.Error(), "database connection failed")
}

// TestErrorWrappingNil tests wrapping nil error
func TestErrorWrappingNil(t *testing.T) {
	result := Wrap(nil, ErrCodeInternal, "should not panic")
	assert.Nil(t, result)
}

// TestErrorInterface tests error interface implementation
func TestErrorInterface(t *testing.T) {
	err := New(ErrCodeNotFound, "resource not found")

	// Test Error() method - should contain code and message
	errorMsg := err.Error()
	assert.Contains(t, errorMsg, "NOT_FOUND")
	assert.Contains(t, errorMsg, "resource not found")

	// Test that it implements error interface
	var _ error = err
}

// TestErrorUnwrap tests error unwrapping
func TestErrorUnwrap(t *testing.T) {
	baseErr := errors.New("base error")
	wrapped := Wrap(baseErr, ErrCodeInternal, "wrapped error")

	unwrapped := wrapped.Unwrap()
	require.Error(t, unwrapped)

	// The unwrapped error should contain the original error message
	assert.Contains(t, unwrapped.Error(), "base error")
}

// TestErrorIs tests errors.Is support
func TestErrorIs(t *testing.T) {
	err1 := New(ErrCodeNotFound, "not found")
	err2 := New(ErrCodeNotFound, "different message")
	err3 := New(ErrCodeInternal, "internal error")

	// Same code should match
	assert.True(t, err1.Is(err2))

	// Different code should not match
	assert.False(t, err1.Is(err3))

	// Different type should not match
	assert.False(t, err1.Is(errors.New("some error")))
}

// TestErrorAs tests errors.As support
func TestErrorAs(t *testing.T) {
	err := New(ErrCodeInternal, "test error")

	var target *Error
	assert.True(t, err.As(&target))
	assert.Equal(t, err, target)

	// Test with wrong type
	var wrongTarget string
	assert.False(t, err.As(&wrongTarget))
}

// TestStackTrace tests stack trace functionality
func TestStackTrace(t *testing.T) {
	err := New(ErrCodeInternal, "test error")

	stack := err.StackTrace()
	assert.NotEmpty(t, stack)
	assert.Contains(t, stack, "TestStackTrace") // Should contain this function name

	// Wrapped errors should also have stack traces
	wrapped := Wrap(err, ErrCodeInternal, "wrapped")
	wrappedStack := wrapped.StackTrace()
	assert.NotEmpty(t, wrappedStack)
}

// TestWithMetadata tests adding single metadata
func TestWithMetadata(t *testing.T) {
	err := New(ErrCodeInternal, "test error")

	// Add metadata
	errWithMeta := err.WithMetadata("user_id", 123)

	// Original error should be unchanged (immutability)
	assert.Empty(t, err.meta)

	// New error should have metadata
	metadata := errWithMeta.Metadata()
	assert.Equal(t, 123, metadata["user_id"])

	// Add another metadata
	errWithMoreMeta := errWithMeta.WithMetadata("resource", "profile")
	metadata = errWithMoreMeta.Metadata()
	assert.Equal(t, 123, metadata["user_id"])
	assert.Equal(t, "profile", metadata["resource"])
}

// TestWithMetadataMap tests adding multiple metadata
func TestWithMetadataMap(t *testing.T) {
	err := New(ErrCodeInternal, "test error")

	metadata := map[string]any{
		"user_id":     123,
		"resource":    "profile",
		"retry_count": 3,
	}

	errWithMeta := err.WithMetadataMap(metadata)

	// Original error should be unchanged
	assert.Empty(t, err.meta)

	// New error should have all metadata
	resultMeta := errWithMeta.Metadata()
	assert.Equal(t, 123, resultMeta["user_id"])
	assert.Equal(t, "profile", resultMeta["resource"])
	assert.Equal(t, 3, resultMeta["retry_count"])
}

// TestMetadataImmutability tests that metadata is immutable
func TestMetadataImmutability(t *testing.T) {
	err := New(ErrCodeInternal, "test error").
		WithMetadata("key1", "value1")

	metadata := err.Metadata()
	metadata["key1"] = "modified"
	metadata["new_key"] = "new_value"

	// Original error metadata should be unchanged
	originalMeta := err.Metadata()
	assert.Equal(t, "value1", originalMeta["key1"])
	assert.NotContains(t, originalMeta, "new_key")
}

// TestWithContext tests context integration
func TestWithContext(t *testing.T) {
	// Create context with values
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "req-123")
	ctx = context.WithValue(ctx, "user_id", "user-456")

	// Create error and add context
	err := New(ErrCodeInternal, "test error")
	errWithCtx := err.WithContext(ctx)

	// Original error should be unchanged
	assert.Empty(t, err.meta)

	// New error should have context values
	metadata := errWithCtx.Metadata()
	assert.Equal(t, "req-123", metadata["request_id"])
	assert.Equal(t, "user-456", metadata["user_id"])
}

// TestWithContextWithOpenTelemetry tests OpenTelemetry trace extraction
func TestWithContextWithOpenTelemetry(t *testing.T) {
	// Create a mock span context
	traceID := trace.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	spanID := trace.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})

	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

	// Create error and add context
	err := New(ErrCodeInternal, "test error")
	errWithCtx := err.WithContext(ctx)

	// Should extract trace and span IDs
	metadata := errWithCtx.Metadata()
	assert.Equal(t, traceID.String(), metadata["trace_id"])
	assert.Equal(t, spanID.String(), metadata["span_id"])
}

// TestWithSeverity tests severity modification
func TestWithSeverity(t *testing.T) {
	err := New(ErrCodeInternal, "test error") // Default severity: error

	// Change to warning
	errWarning := err.WithSeverity(SeverityWarning)
	assert.Equal(t, SeverityError, err.Severity) // Original unchanged
	assert.Equal(t, SeverityWarning, errWarning.Severity)

	// Change to info
	errInfo := err.WithSeverity(SeverityInfo)
	assert.Equal(t, SeverityInfo, errInfo.Severity)
}

// TestErrorConstructors tests all error constructors
func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name        string
		constructor func() *Error
		wantCode    ErrorCode
		wantMsg     string
	}{
		{
			name:        "not found error",
			constructor: func() *Error { return NotFoundError("user") },
			wantCode:    ErrCodeNotFound,
			wantMsg:     "user not found",
		},
		{
			name:        "validation error",
			constructor: func() *Error { return ValidationError("invalid input") },
			wantCode:    ErrCodeValidation,
			wantMsg:     "invalid input",
		},
		{
			name:        "auth error",
			constructor: func() *Error { return AuthError("unauthorized") },
			wantCode:    ErrCodeUnauthorized,
			wantMsg:     "unauthorized",
		},
		{
			name:        "forbidden error",
			constructor: func() *Error { return ForbiddenError("access denied") },
			wantCode:    ErrCodeForbidden,
			wantMsg:     "access denied",
		},
		{
			name:        "conflict error",
			constructor: func() *Error { return ConflictError("resource conflict") },
			wantCode:    ErrCodeConflict,
			wantMsg:     "resource conflict",
		},
		{
			name:        "timeout error",
			constructor: func() *Error { return TimeoutError("db query", "5s") },
			wantCode:    ErrCodeTimeout,
			wantMsg:     "db query timed out after 5s",
		},
		{
			name:        "rate limit error",
			constructor: func() *Error { return RateLimitError("60s") },
			wantCode:    ErrCodeRateLimited,
			wantMsg:     "rate limit exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor()
			require.NotNil(t, err)
			assert.Equal(t, tt.wantCode, err.Code)
			assert.Equal(t, tt.wantMsg, err.Message)
			assert.Equal(t, SeverityError, err.Severity)
		})
	}
}

// TestInternalError tests internal error constructor
func TestInternalError(t *testing.T) {
	cause := errors.New("database error")
	err := InternalError("failed to fetch user", cause)

	require.NotNil(t, err)
	assert.Equal(t, ErrCodeInternal, err.Code)
	assert.Equal(t, "failed to fetch user", err.Message)

	// Unwrap should return the cause chain
	unwrapped := err.Unwrap()
	require.Error(t, unwrapped)
	assert.Contains(t, unwrapped.Error(), "database error")
}

// TestInternalErrorWithoutCause tests internal error without cause
func TestInternalErrorWithoutCause(t *testing.T) {
	err := InternalError("something went wrong", nil)

	require.NotNil(t, err)
	assert.Equal(t, ErrCodeInternal, err.Code)
	assert.Error(t, err.Unwrap())
}

// TestExternalAPIError tests external API error constructor
func TestExternalAPIError(t *testing.T) {
	cause := errors.New("connection timeout")
	err := ExternalAPIError("payment-gateway", cause)

	require.NotNil(t, err)
	assert.Equal(t, ErrCodeExternalAPI, err.Code)
	assert.Equal(t, "external service payment-gateway error", err.Message)

	// Unwrap should return the cause chain
	unwrapped := err.Unwrap()
	require.Error(t, unwrapped)
	assert.Contains(t, unwrapped.Error(), "connection timeout")

	metadata := err.Metadata()
	assert.Equal(t, "payment-gateway", metadata["service"])
}

// TestErrorInspectionHelpers tests error inspection functions
func TestErrorInspectionHelpers(t *testing.T) {
	tests := []struct {
		name      string
		err       *Error
		checkFunc func(error) bool
		want      bool
	}{
		{
			name:      "is not found",
			err:       NotFoundError("user"),
			checkFunc: IsNotFound,
			want:      true,
		},
		{
			name:      "is not not found",
			err:       ValidationError("invalid"),
			checkFunc: IsNotFound,
			want:      false,
		},
		{
			name:      "is validation",
			err:       ValidationError("invalid"),
			checkFunc: IsValidation,
			want:      true,
		},
		{
			name:      "is auth",
			err:       AuthError("unauthorized"),
			checkFunc: IsAuth,
			want:      true,
		},
		{
			name:      "is forbidden",
			err:       ForbiddenError("access denied"),
			checkFunc: IsForbidden,
			want:      true,
		},
		{
			name:      "is internal",
			err:       InternalError("error", nil),
			checkFunc: IsInternal,
			want:      true,
		},
		{
			name:      "is timeout",
			err:       TimeoutError("op", "5s"),
			checkFunc: IsTimeout,
			want:      true,
		},
		{
			name:      "is rate limited",
			err:       RateLimitError("60s"),
			checkFunc: IsRateLimited,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.checkFunc(tt.err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestGetHelpers tests GetCode, GetStatusCode, GetSeverity, GetMetadata
func TestGetHelpers(t *testing.T) {
	err := NotFoundError("user").
		WithMetadata("user_id", 123).
		WithSeverity(SeverityWarning)

	assert.Equal(t, ErrCodeNotFound, GetCode(err))
	assert.Equal(t, http.StatusNotFound, GetStatusCode(err))
	assert.Equal(t, SeverityWarning, GetSeverity(err))

	metadata := GetMetadata(err)
	assert.Equal(t, 123, metadata["user_id"])

	// Test with standard error
	stdErr := errors.New("standard error")
	assert.Equal(t, ErrCodeInternal, GetCode(stdErr))
	assert.Equal(t, http.StatusInternalServerError, GetStatusCode(stdErr))
	assert.Equal(t, SeverityError, GetSeverity(stdErr))
	assert.Nil(t, GetMetadata(stdErr))
}

// TestHTTPStatusCode tests HTTP status code mapping
func TestHTTPStatusCode(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		wantCode int
	}{
		{ErrCodeNotFound, http.StatusNotFound},
		{ErrCodeValidation, http.StatusBadRequest},
		{ErrCodeUnauthorized, http.StatusUnauthorized},
		{ErrCodeForbidden, http.StatusForbidden},
		{ErrCodeInternal, http.StatusInternalServerError},
		{ErrCodeConflict, http.StatusConflict},
		{ErrCodeTimeout, http.StatusRequestTimeout},
		{ErrCodeRateLimited, http.StatusTooManyRequests},
		{ErrCodeExternalAPI, http.StatusBadGateway},
		{ErrorCode("UNKNOWN"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			assert.Equal(t, tt.wantCode, tt.code.HTTPStatusCode())
		})
	}
}

// TestWriteHTTPResponse tests HTTP response writing
func TestWriteHTTPResponse(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "not found error",
			err:        NotFoundError("user"),
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "validation error",
			err:        ValidationError("invalid input"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "internal error",
			err:        InternalError("error", nil),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
		},
		{
			name:       "standard error",
			err:        errors.New("standard error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteHTTPResponse(w, tt.err)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response HTTPErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.wantCode, response.Error.Code)
			assert.NotEmpty(t, response.Error.Message)
		})
	}
}

// TestWriteHTTPResponseWithDetails tests HTTP response with metadata
func TestWriteHTTPResponseWithDetails(t *testing.T) {
	err := NotFoundError("user").
		WithMetadata("user_id", 123).
		WithMetadata("query", "email = 'test@example.com'")

	w := httptest.NewRecorder()
	WriteHTTPResponse(w, err)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response HTTPErrorResponse
	unmarshalErr := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "NOT_FOUND", response.Error.Code)
	assert.Contains(t, response.Error.Details, "user_id")
	assert.Contains(t, response.Error.Details, "query")
}

// TestWriteHTTPResponseWithTrace tests HTTP response with trace IDs
func TestWriteHTTPResponseWithTrace(t *testing.T) {
	traceID := trace.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	spanID := trace.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})

	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

	err := NotFoundError("user").WithContext(ctx)

	w := httptest.NewRecorder()
	WriteHTTPResponse(w, err)

	var response HTTPErrorResponse
	unmarshalErr := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, traceID.String(), response.Error.TraceID)
	assert.Equal(t, spanID.String(), response.Error.SpanID)
}

// TestErrorChaining tests error chain traversal
func TestErrorChaining(t *testing.T) {
	baseErr := errors.New("database error")
	wrapped1 := Wrap(baseErr, ErrCodeInternal, "repository error")
	wrapped2 := Wrap(wrapped1, ErrCodeInternal, "service error")

	// Should be able to traverse the chain
	require.ErrorIs(t, wrapped2, baseErr)
	require.ErrorIs(t, wrapped2, wrapped1)

	// Should be able to extract our error type
	var appErr *Error
	require.ErrorAs(t, wrapped2, &appErr)
	assert.Equal(t, ErrCodeInternal, appErr.Code)
	assert.Equal(t, "service error", appErr.Message)
}

// TestThreadSafety tests concurrent error operations
func TestThreadSafety(t *testing.T) {
	var wg sync.WaitGroup
	iterations := 100

	// Test concurrent metadata addition
	t.Run("concurrent metadata", func(t *testing.T) {
		baseErr := New(ErrCodeInternal, "test error")

		for i := range iterations {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				_ = baseErr.WithMetadata("goroutine", id)
			}(i)
		}

		wg.Wait()
		// Original error should be unchanged
		assert.Empty(t, baseErr.meta)
	})

	// Test concurrent error creation
	t.Run("concurrent creation", func(t *testing.T) {
		for i := range iterations {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				err := New(ErrCodeInternal, "test error").
					WithMetadata("id", id).
					WithMetadata("goroutine", "test")
				assert.NotNil(t, err)
				metadata := err.Metadata()
				assert.Equal(t, id, metadata["id"])
			}(i)
		}

		wg.Wait()
	})
}

// TestErrorJSONSerialization tests JSON serialization
func TestErrorJSONSerialization(t *testing.T) {
	err := NotFoundError("user").
		WithMetadata("user_id", 123).
		WithMetadata("resource", "profile").
		WithSeverity(SeverityWarning)

	// Marshal to JSON
	data, jsonErr := json.Marshal(err)
	require.NoError(t, jsonErr)

	// Unmarshal and verify
	var result map[string]any
	unmarshalErr := json.Unmarshal(data, &result)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "NOT_FOUND", result["code"])
	assert.Equal(t, "user not found", result["message"])
	assert.Equal(t, "warning", result["severity"])

	details, ok := result["metadata"].(map[string]any)
	assert.True(t, ok)
	assert.InEpsilon(t, float64(123), details["user_id"], 0.0001) // JSON numbers are float64
	assert.Equal(t, "profile", details["resource"])
}

// TestErrorWithEmptyMessage tests error with empty message
func TestErrorWithEmptyMessage(t *testing.T) {
	err := New(ErrCodeInternal, "")

	assert.Equal(t, ErrCodeInternal, err.Code)
	assert.Empty(t, err.Message)
	assert.NotEmpty(t, err.Error()) // Should still produce an error string
}

// TestErrorWithSpecialCharacters tests error with special characters
func TestErrorWithSpecialCharacters(t *testing.T) {
	message := "error with special chars: \n\t\\\"'\\\\"
	err := New(ErrCodeInternal, message)

	assert.Equal(t, message, err.Message)
	assert.Contains(t, err.Error(), message)

	// Should serialize to JSON properly
	data, marshalErr := json.Marshal(err)
	require.NoError(t, marshalErr)
	assert.Contains(t, string(data), "error with special chars")
}

// TestErrorChainWithStandardErrors tests mixing with standard library errors
func TestErrorChainWithStandardErrors(t *testing.T) {
	stdErr := stderrors.New("standard error")
	appErr := Wrap(stdErr, ErrCodeInternal, "wrapped")

	// Should be able to use errors.Is
	require.ErrorIs(t, appErr, stdErr)

	// Should be able to use errors.As for our type
	var target *Error
	require.ErrorAs(t, appErr, &target)
	assert.Equal(t, ErrCodeInternal, target.Code)
}

// TestNilErrorHandling tests handling of nil errors
func TestNilErrorHandling(t *testing.T) {
	// Wrapping nil should return nil
	result := Wrap(nil, ErrCodeInternal, "test")
	assert.Nil(t, result)

	// Get helpers with nil should return defaults
	assert.Equal(t, ErrCodeInternal, GetCode(nil))
	assert.Equal(t, http.StatusInternalServerError, GetStatusCode(nil))
	assert.Equal(t, SeverityError, GetSeverity(nil))
	assert.Nil(t, GetMetadata(nil))

	// Is helpers with nil should return false
	assert.False(t, IsNotFound(nil))
	assert.False(t, IsValidation(nil))
}

// TestErrorMessageFormatting tests error message formatting
func TestErrorMessageFormatting(t *testing.T) {
	tests := []struct {
		name         string
		err          *Error
		wantContains []string
	}{
		{
			name:         "error without cause",
			err:          New(ErrCodeInternal, "something went wrong"),
			wantContains: []string{"INTERNAL_ERROR", "something went wrong"},
		},
		{
			name:         "error with cause",
			err:          Wrap(stderrors.New("database error"), ErrCodeInternal, "fetch failed"),
			wantContains: []string{"INTERNAL_ERROR", "fetch failed", "database error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			for _, want := range tt.wantContains {
				assert.Contains(t, msg, want)
			}
		})
	}
}

// TestIntegrationErrorPropagation tests error propagation across layers
func TestIntegrationErrorPropagation(t *testing.T) {
	// Simulate repository layer
	repoError := func() error {
		return Wrap(errors.New("connection failed"), ErrCodeInternal, "database error")
	}

	// Simulate service layer
	serviceError := func() error {
		err := repoError()
		return Wrap(err, ErrCodeInternal, "service error")
	}

	// Simulate handler layer
	handlerError := func() error {
		err := serviceError()
		return Wrap(err, ErrCodeInternal, "handler error")
	}

	// Get the final error
	err := handlerError()

	// Should be able to traverse the entire chain
	var appErr *Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "handler error", appErr.Message)

	// Should preserve all causes
	require.Error(t, appErr.Unwrap())

	// Should have stack trace
	assert.NotEmpty(t, appErr.StackTrace())
}

// TestIntegrationComplexScenario tests a complex real-world scenario

// TestValidationErrorWithDetails tests validation error with field details
func TestValidationErrorWithDetails(t *testing.T) {
	fields := map[string]string{
		"email":    "invalid email format",
		"password": "must be at least 8 characters",
		"age":      "must be 18 or older",
	}

	err := ValidationErrorWithDetails("validation failed", fields)

	require.NotNil(t, err)
	assert.Equal(t, ErrCodeValidation, err.Code)
	assert.Equal(t, "validation failed", err.Message)
	assert.Equal(t, SeverityError, err.Severity)

	metadata := err.Metadata()
	validationErrors, ok := metadata["validation_errors"].(map[string]string)
	require.True(t, ok)
	assert.Equal(t, fields, validationErrors)
}

// TestWriteHTTPResponseWithStatus tests custom status code responses
func TestWriteHTTPResponseWithStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		wantCode   string
	}{
		{
			name:       "custom 400 status",
			err:        NotFoundError("resource"),
			statusCode: http.StatusBadRequest,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "custom 500 status",
			err:        ValidationError("invalid"),
			statusCode: http.StatusInternalServerError,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "standard error with custom status",
			err:        errors.New("standard error"),
			statusCode: http.StatusBadGateway,
			wantCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteHTTPResponseWithStatus(w, tt.err, tt.statusCode)

			assert.Equal(t, tt.statusCode, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response HTTPErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.wantCode, response.Error.Code)
			assert.NotEmpty(t, response.Error.Message)
		})
	}
}

// TestErrorHandler tests the ErrorHandler convenience function
// TestJSONError tests JSONError convenience function
func TestJSONError(t *testing.T) {
	w := httptest.NewRecorder()
	JSONError(w, ErrCodeNotFound, "user not found", http.StatusNotFound)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response HTTPErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "NOT_FOUND", response.Error.Code)
	assert.Equal(t, "user not found", response.Error.Message)
}

// TestJSONErrorWithDetails tests JSONErrorWithDetails convenience function
func TestJSONErrorWithDetails(t *testing.T) {
	w := httptest.NewRecorder()
	details := map[string]any{
		"user_id": 123,
		"query":   "email = 'test@example.com'",
	}

	JSONErrorWithDetails(w, ErrCodeValidation, "validation failed", details, http.StatusBadRequest)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response HTTPErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "VALIDATION_ERROR", response.Error.Code)
	assert.Equal(t, "validation failed", response.Error.Message)
	// JSON unmarshaling converts numbers to float64
	assert.Equal(t, details["query"], response.Error.Details["query"])
	assert.InEpsilon(t, float64(123), response.Error.Details["user_id"], 0.0001) // int becomes float64
}

// TestErrorString tests String method
// TestUnwrapEdgeCases tests edge cases for Unwrap
func TestUnwrapEdgeCases(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		var err *Error
		assert.NoError(t, err.Unwrap())
	})

	t.Run("error without cause", func(t *testing.T) {
		err := New(ErrCodeInternal, "test")
		// New() creates errors with cockroachdb/errors wrapper, so Unwrap() returns the wrapped error
		unwrapped := err.Unwrap()
		require.Error(t, unwrapped)
		assert.Contains(t, unwrapped.Error(), "test")
	})

	t.Run("wrapped cockroachdb error", func(t *testing.T) {
		baseErr := stderrors.New("base error")
		wrappedErr := cockroachdbErrors.Wrap(baseErr, "wrapped")
		appErr := Wrap(wrappedErr, ErrCodeInternal, "app error")

		unwrapped := appErr.Unwrap()
		require.Error(t, unwrapped)
		// Should eventually unwrap to the original error
		assert.ErrorIs(t, unwrapped, baseErr)
	})

	t.Run("deeply nested errors", func(t *testing.T) {
		err1 := stderrors.New("level 1")
		err2 := cockroachdbErrors.Wrap(err1, "level 2")
		err3 := cockroachdbErrors.Wrap(err2, "level 3")
		appErr := Wrap(err3, ErrCodeInternal, "level 4")

		unwrapped := appErr.Unwrap()
		require.Error(t, unwrapped)
		assert.ErrorIs(t, unwrapped, err1)
	})
}

// TestStackTraceEdgeCases tests edge cases for StackTrace
func TestStackTraceEdgeCases(t *testing.T) {
	t.Run("error without cause", func(t *testing.T) {
		err := New(ErrCodeInternal, "test")
		stack := err.StackTrace()
		assert.NotEmpty(t, stack)
		assert.Contains(t, stack, "TestStackTraceEdgeCases")
	})

	t.Run("wrapped error preserves stack", func(t *testing.T) {
		baseErr := stderrors.New("base")
		wrappedErr := Wrap(baseErr, ErrCodeInternal, "wrapped")

		stack := wrappedErr.StackTrace()
		assert.NotEmpty(t, stack)
		// Should contain stack from both levels
		assert.Contains(t, stack, "wrapped")
		assert.Contains(t, stack, "base")
	})

	t.Run("nil error", func(t *testing.T) {
		var err *Error
		// Should handle nil error gracefully
		assert.NotPanics(t, func() {
			_ = err.StackTrace()
		})
		assert.Empty(t, err.StackTrace())
	})
}

// TestHTTPStatusCodeEdgeCases tests edge cases for HTTP status code mapping
func TestHTTPStatusCodeEdgeCases(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		wantCode int
	}{
		{ErrCodeTokenExpired, http.StatusUnauthorized},
		{ErrCodeInvalidToken, http.StatusUnauthorized},
		{ErrCodeAuthFailed, http.StatusUnauthorized},
		{ErrCodeInvalidInput, http.StatusBadRequest},
		{ErrCodeMissingField, http.StatusBadRequest},
		{ErrCodeInvalidFormat, http.StatusBadRequest},
		{ErrCodeAlreadyExists, http.StatusConflict},
		{ErrCodeResourceExhausted, http.StatusInsufficientStorage},
		{ErrCodeExternalTimeout, http.StatusRequestTimeout},
		{ErrorCode("UNKNOWN_CODE"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			assert.Equal(t, tt.wantCode, tt.code.HTTPStatusCode())
		})
	}
}

// TestErrorIsEdgeCases tests edge cases for Is method
func TestErrorIsEdgeCases(t *testing.T) {
	t.Run("nil target", func(t *testing.T) {
		err := New(ErrCodeInternal, "test")
		assert.False(t, err.Is(nil))
	})

	t.Run("different type", func(t *testing.T) {
		err := New(ErrCodeInternal, "test")
		assert.False(t, err.Is(errors.New("different")))
	})

	t.Run("same code different message", func(t *testing.T) {
		err1 := New(ErrCodeNotFound, "not found 1")
		err2 := New(ErrCodeNotFound, "not found 2")
		assert.True(t, err1.Is(err2))
	})
}

// TestErrorAsEdgeCases tests edge cases for As method
func TestErrorAsEdgeCases(t *testing.T) {
	t.Run("nil target", func(t *testing.T) {
		err := New(ErrCodeInternal, "test")
		assert.False(t, err.As(nil))
	})

	t.Run("wrong pointer type", func(t *testing.T) {
		err := New(ErrCodeInternal, "test")
		var target string
		assert.False(t, err.As(&target))
	})

	t.Run("correct pointer type", func(t *testing.T) {
		err := New(ErrCodeInternal, "test")
		var target *Error
		assert.True(t, err.As(&target))
		assert.Equal(t, err, target)
	})
}

// TestWithMetadataMapEdgeCases tests edge cases for WithMetadataMap
func TestWithMetadataMapEdgeCases(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		err := New(ErrCodeInternal, "test")
		errWithEmpty := err.WithMetadataMap(map[string]any{})

		assert.Empty(t, err.meta) // Original unchanged
		assert.Empty(t, errWithEmpty.meta)
	})

	t.Run("nil map", func(t *testing.T) {
		err := New(ErrCodeInternal, "test")
		errWithNil := err.WithMetadataMap(nil)

		assert.Empty(t, err.meta) // Original unchanged
		assert.Empty(t, errWithNil.meta)
	})

	t.Run("overlapping keys", func(t *testing.T) {
		err := New(ErrCodeInternal, "test").WithMetadata("key1", "original")
		overrides := map[string]any{
			"key1": "overridden",
			"key2": "new",
		}
		errWithOverride := err.WithMetadataMap(overrides)

		metadata := errWithOverride.Metadata()
		assert.Equal(t, "overridden", metadata["key1"]) // Should be overridden
		assert.Equal(t, "new", metadata["key2"])        // Should be added
	})
}

// TestWithContextEdgeCases tests edge cases for WithContext
func TestWithContextEdgeCases(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		err := New(ErrCodeInternal, "test")
		errWithNil := err.WithContext(nil)

		assert.Equal(t, err, errWithNil) // Should return same error
		assert.Empty(t, err.meta)
		assert.Empty(t, errWithNil.meta)
	})

	t.Run("context without span", func(t *testing.T) {
		ctx := context.Background()
		ctx = context.WithValue(ctx, "request_id", "req-123")

		err := New(ErrCodeInternal, "test").WithContext(ctx)
		metadata := err.Metadata()

		assert.Equal(t, "req-123", metadata["request_id"])
		assert.NotContains(t, metadata, "trace_id") // No trace without span
		assert.NotContains(t, metadata, "span_id")
	})

	t.Run("invalid span context", func(t *testing.T) {
		// Create invalid span context (all zeros)
		traceID := trace.TraceID{}
		spanID := trace.SpanID{}

		spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: traceID,
			SpanID:  spanID,
		})

		ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)
		err := New(ErrCodeInternal, "test").WithContext(ctx)
		metadata := err.Metadata()

		// Should not include invalid trace/span IDs
		assert.NotContains(t, metadata, "trace_id")
		assert.NotContains(t, metadata, "span_id")
	})
}

// TestErrorNil tests Error() method with nil receiver
func TestErrorNil(t *testing.T) {
	var err *Error
	assert.Empty(t, err.Error())
}

// TestErrorMessageEdgeCases tests edge cases for Error() method
func TestErrorMessageEdgeCases(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		var err *Error
		assert.Empty(t, err.Error())
	})

	t.Run("error with cause", func(t *testing.T) {
		cause := stderrors.New("cause error")
		err := Wrap(cause, ErrCodeInternal, "wrapped error")
		msg := err.Error()
		assert.Contains(t, msg, "INTERNAL_ERROR")
		assert.Contains(t, msg, "wrapped error")
		assert.Contains(t, msg, "cause error")
	})
}

// TestUnwrapMoreEdgeCases tests additional Unwrap edge cases
func TestUnwrapMoreEdgeCases(t *testing.T) {
	t.Run("unwrap returns cockroachdb wrapper", func(t *testing.T) {
		baseErr := stderrors.New("base")
		wrapped := cockroachdbErrors.Wrap(baseErr, "wrapped")
		appErr := Wrap(wrapped, ErrCodeInternal, "app")

		unwrapped := appErr.Unwrap()
		require.Error(t, unwrapped)
		// Should be able to traverse to base error
		assert.ErrorIs(t, unwrapped, baseErr)
	})

	t.Run("multiple unwrap levels", func(t *testing.T) {
		err1 := stderrors.New("level 1")
		err2 := cockroachdbErrors.Wrap(err1, "level 2")
		err3 := cockroachdbErrors.Wrap(err2, "level 3")
		appErr := Wrap(err3, ErrCodeInternal, "level 4")

		// Should preserve the entire chain
		require.ErrorIs(t, appErr, err1)
		require.ErrorIs(t, appErr, err2)
		assert.ErrorIs(t, appErr, err3)
	})

	t.Run("unwrap with Cause method", func(t *testing.T) {
		// Test errors that have Cause() method instead of Unwrap()
		type causer interface {
			Cause() error
		}

		baseErr := stderrors.New("base")
		wrapped := cockroachdbErrors.Wrap(baseErr, "wrapped")
		appErr := Wrap(wrapped, ErrCodeInternal, "app")

		unwrapped := appErr.Unwrap()
		require.Error(t, unwrapped)

		// Check if it implements causer
		if c, ok := unwrapped.(causer); ok {
			cause := c.Cause()
			assert.Error(t, cause)
		}
	})
}

// TestHTTPStatusCodeAllCodes tests all error codes for HTTP status mapping
func TestHTTPStatusCodeAllCodes(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected int
	}{
		{ErrCodeUnauthorized, http.StatusUnauthorized},
		{ErrCodeForbidden, http.StatusForbidden},
		{ErrCodeNotFound, http.StatusNotFound},
		{ErrCodeValidation, http.StatusBadRequest},
		{ErrCodeInvalidInput, http.StatusBadRequest},
		{ErrCodeMissingField, http.StatusBadRequest},
		{ErrCodeInvalidFormat, http.StatusBadRequest},
		{ErrCodeConflict, http.StatusConflict},
		{ErrCodeAlreadyExists, http.StatusConflict},
		{ErrCodeResourceExhausted, http.StatusInsufficientStorage},
		{ErrCodeInternal, http.StatusInternalServerError},
		{ErrCodeTimeout, http.StatusRequestTimeout},
		{ErrCodeExternalTimeout, http.StatusRequestTimeout},
		{ErrCodeUnavailable, http.StatusServiceUnavailable},
		{ErrCodeRateLimited, http.StatusTooManyRequests},
		{ErrCodeExternalAPI, http.StatusBadGateway},
		{ErrorCode("UNKNOWN"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.HTTPStatusCode())
		})
	}
}

// TestExternalAPIErrorNilCause tests ExternalAPIError with nil cause
func TestExternalAPIErrorNilCause(t *testing.T) {
	err := ExternalAPIError("test-service", nil)

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeExternalAPI, err.Code)
	assert.Equal(t, "external service test-service error", err.Message)
	assert.Error(t, err.Unwrap())
}

// TestIsHelpersWithWrongTypes tests Is* helpers with wrong error types
func TestIsHelpersWithWrongTypes(t *testing.T) {
	stdErr := stderrors.New("standard error")

	assert.False(t, IsNotFound(stdErr))
	assert.False(t, IsValidation(stdErr))
	assert.False(t, IsAuth(stdErr))
	assert.False(t, IsForbidden(stdErr))
	assert.False(t, IsInternal(stdErr))
	assert.False(t, IsTimeout(stdErr))
	assert.False(t, IsRateLimited(stdErr))
}

// TestWriteHTTPResponseNilAndEmpty tests nil and empty error responses
func TestWriteHTTPResponseNilAndEmpty(t *testing.T) {
	t.Run("nil writer", func(t *testing.T) {
		err := NotFoundError("test")
		WriteHTTPResponse(nil, err)
		// Should not panic
	})

	t.Run("nil error", func(t *testing.T) {
		w := httptest.NewRecorder()
		WriteHTTPResponse(w, nil)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Body.Bytes())
	})
}

// TestWriteHTTPResponseWithStatusEdgeCases tests edge cases for WriteHTTPResponseWithStatus
func TestWriteHTTPResponseWithStatusEdgeCases(t *testing.T) {
	t.Run("nil writer", func(t *testing.T) {
		err := NotFoundError("test")
		WriteHTTPResponseWithStatus(nil, err, http.StatusBadRequest)
		// Should not panic
	})

	t.Run("nil error", func(t *testing.T) {
		w := httptest.NewRecorder()
		WriteHTTPResponseWithStatus(w, nil, http.StatusBadRequest)
		// Should return early without writing anything
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Body.Bytes())
	})

	t.Run("encode error", func(t *testing.T) {
		w := httptest.NewRecorder()
		// Create a mock writer that fails on Write
		mockWriter := &mockFailingWriter{ResponseWriter: w}
		err := NotFoundError("test")
		WriteHTTPResponseWithStatus(mockWriter, err, http.StatusOK)
		// Should handle encode error gracefully
	})

	t.Run("json encode error", func(t *testing.T) {
		w := httptest.NewRecorder()
		// Create an error that can't be marshaled (though our errors are always marshalable)
		err := NotFoundError("test")
		WriteHTTPResponseWithStatus(w, err, http.StatusOK)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// mockFailingWriter is a mock writer that fails on Write
type mockFailingWriter struct {
	http.ResponseWriter
}

func (m *mockFailingWriter) Write(b []byte) (int, error) {
	return 0, stderrors.New("write failed")
}

// TestMarshalJSONEdgeCases tests edge cases for JSON marshaling
func TestMarshalJSONEdgeCases(t *testing.T) {
	t.Run("error without metadata", func(t *testing.T) {
		err := New(ErrCodeInternal, "test")
		data, jsonErr := json.Marshal(err)
		require.NoError(t, jsonErr)

		var result map[string]any
		unmarshalErr := json.Unmarshal(data, &result)
		require.NoError(t, unmarshalErr)

		assert.Equal(t, "INTERNAL_ERROR", result["code"])
		assert.Equal(t, "test", result["message"])
		assert.Equal(t, "error", result["severity"])
		assert.NotContains(t, result, "metadata") // Should not include empty metadata
	})

	t.Run("error with complex metadata", func(t *testing.T) {
		err := New(ErrCodeInternal, "test").WithMetadata("complex", map[string]any{
			"nested": map[string]int{
				"value": 42,
			},
			"array": []string{"a", "b", "c"},
		})

		data, jsonErr := json.Marshal(err)
		require.NoError(t, jsonErr)

		var result map[string]any
		unmarshalErr := json.Unmarshal(data, &result)
		require.NoError(t, unmarshalErr)

		metadata, ok := result["metadata"].(map[string]any)
		require.True(t, ok)
		assert.Contains(t, metadata, "complex")
	})
}
