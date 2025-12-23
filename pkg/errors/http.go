package errors

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"

	"github.com/kannancmohan/go-prototype-authz-cerbos/pkg/logger"
)

// HTTPErrorResponse represents the JSON structure for HTTP error responses
type HTTPErrorResponse struct {
	Error HTTPErrorDetail `json:"error"`
}

// HTTPErrorDetail contains the detailed error information
type HTTPErrorDetail struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	TraceID   string         `json:"trace_id,omitempty"`
	SpanID    string         `json:"span_id,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
}

// extractMetadata extracts trace/span/request IDs from error metadata
func extractMetadata(appErr *Error) (traceID, spanID, requestID string) {
	if val, ok := appErr.meta["trace_id"].(string); ok {
		traceID = val
	}
	if val, ok := appErr.meta["span_id"].(string); ok {
		spanID = val
	}
	if val, ok := appErr.meta["request_id"].(string); ok {
		requestID = val
	}

	return traceID, spanID, requestID
}

// handlePanicRecovery handles panic recovery and converts it to an error response
func handlePanicRecovery(rec any, w http.ResponseWriter, r *http.Request, log logger.Logger) {
	// Convert panic to error
	var err error
	switch val := rec.(type) {
	case error:
		err = val
	case string:
		err = InternalError(val, nil)
	default:
		err = InternalError(fmt.Sprintf("%v", val), nil)
	}

	// Add request context to the error
	var appErr *Error
	if errors.As(err, &appErr) {
		err = appErr.WithContext(r.Context())
	}

	// Log the error
	if errors.As(err, &appErr) {
		if log != nil {
			_ = appErr.LogCtx(r.Context(), log)
		} else {
			_ = appErr.Log(nil)
		}
	}

	// Write error response
	WriteHTTPResponse(w, err)
}

// WriteHTTPResponse writes an error as a JSON HTTP response with the appropriate status code
func WriteHTTPResponse(w http.ResponseWriter, err error) {
	if w == nil || err == nil {
		return
	}

	// Determine status code and error details
	statusCode := http.StatusInternalServerError
	var errorDetail HTTPErrorDetail

	// Check if it's our structured error
	var appErr *Error
	if errors.As(err, &appErr) {
		statusCode = appErr.Code.HTTPStatusCode()
		traceID, spanID, requestID := extractMetadata(appErr)
		errorDetail = HTTPErrorDetail{
			Code:      string(appErr.Code),
			Message:   appErr.Message,
			Details:   appErr.meta,
			TraceID:   traceID,
			SpanID:    spanID,
			RequestID: requestID,
		}
	} else {
		// For standard errors, use internal error code
		errorDetail = HTTPErrorDetail{
			Code:    string(ErrCodeInternal),
			Message: err.Error(),
			Details: nil,
		}
	}

	// Create response
	response := HTTPErrorResponse{
		Error: errorDetail,
	}

	// Set headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Encode and write JSON response
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		// If we can't encode the error response, write a simple text response
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, `{"error":{"code":"INTERNAL_ERROR","message":"failed to encode error response"}}`)
		// Note: Ignoring write error in fallback path - already handling an error condition
	}
}

// WriteHTTPResponseWithStatus writes an error as a JSON HTTP response with a custom status code
func WriteHTTPResponseWithStatus(w http.ResponseWriter, err error, statusCode int) {
	if w == nil || err == nil {
		return
	}

	var errorDetail HTTPErrorDetail

	// Check if it's our structured error
	var appErr *Error
	if errors.As(err, &appErr) {
		traceID, spanID, requestID := extractMetadata(appErr)
		errorDetail = HTTPErrorDetail{
			Code:      string(appErr.Code),
			Message:   appErr.Message,
			Details:   appErr.meta,
			TraceID:   traceID,
			SpanID:    spanID,
			RequestID: requestID,
		}
	} else {
		// For standard errors
		errorDetail = HTTPErrorDetail{
			Code:    string(ErrCodeInternal),
			Message: err.Error(),
			Details: nil,
		}
	}

	// Create response
	response := HTTPErrorResponse{
		Error: errorDetail,
	}

	// Set headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Encode and write JSON response
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		// If we can't encode the error response, write a simple text response
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, `{"error":{"code":"INTERNAL_ERROR","message":"failed to encode error response"}}`)
		// Note: Ignoring write error in fallback path - already handling an error condition
	}
}

// HTTPMiddleware creates an HTTP middleware that automatically handles errors
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a custom response writer to capture status code
		crw := &captureResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Process the request
		next.ServeHTTP(crw, r)

		// Note: If the status code indicates an error and no body was written,
		// you might want to add default error handling here if needed
		// For now, we just capture the status code for observability
	})
}

// RecoveryMiddleware creates an HTTP middleware that recovers from panics and converts them to errors
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				handlePanicRecovery(rec, w, r, nil)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// getErrorForStatusCode returns an appropriate error for the given HTTP status code
func getErrorForStatusCode(statusCode int) error {
	switch statusCode {
	case http.StatusNotFound:
		return NotFoundError("resource")
	case http.StatusUnauthorized:
		return AuthError("unauthorized")
	case http.StatusForbidden:
		return ForbiddenError("forbidden")
	case http.StatusBadRequest:
		return ValidationError("bad request")
	default:
		return InternalError(fmt.Sprintf("HTTP %d", statusCode), nil)
	}
}

// CombinedMiddleware creates a combined middleware with error recovery and handling
func CombinedMiddleware(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a custom response writer to capture status code and body
			crw := &captureResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			//nolint:contextcheck // defer function correctly accesses context via r.Context()
			defer func() {
				if rec := recover(); rec != nil {
					handlePanicRecovery(rec, w, r, log)
					return
				}

				// If no panic occurred but we have an error status with no body,
				// we might want to add default error handling
				if crw.statusCode >= 400 && crw.bodyWritten == 0 {
					err := getErrorForStatusCode(crw.statusCode)

					// Add request context
					var appErr *Error
					if errors.As(err, &appErr) {
						err = appErr.WithContext(r.Context())
					}

					// Log and write response
					if errors.As(err, &appErr) {
						_ = appErr.LogCtx(r.Context(), log)
					}
					WriteHTTPResponse(w, err)
				}
			}()

			next.ServeHTTP(crw, r)
		})
	}
}

// captureResponseWriter wraps http.ResponseWriter to capture status code and body size
type captureResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	bodyWritten int
}

// WriteHeader captures the status code
func (crw *captureResponseWriter) WriteHeader(statusCode int) {
	crw.statusCode = statusCode
	crw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the body size
func (crw *captureResponseWriter) Write(b []byte) (int, error) {
	size, err := crw.ResponseWriter.Write(b)
	crw.bodyWritten += size
	if err != nil {
		return size, fmt.Errorf("response write failed after %d bytes: %w", size, err)
	}

	return size, nil
}

// ErrorHandler is a convenience function to handle errors in HTTP handlers
func ErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	// Add request context to the error
	var appErr *Error
	if errors.As(err, &appErr) {
		err = appErr.WithContext(r.Context())
	}

	// Log the error
	if errors.As(err, &appErr) {
		_ = appErr.LogCtx(r.Context(), nil) // Logger will be injected
	}

	// Write error response
	WriteHTTPResponse(w, err)
}

// JSONError creates a properly formatted JSON error response
func JSONError(w http.ResponseWriter, code ErrorCode, message string, statusCode int) {
	err := New(code, message)
	WriteHTTPResponseWithStatus(w, err, statusCode)
}

// JSONErrorWithDetails creates a JSON error response with additional details
func JSONErrorWithDetails(w http.ResponseWriter, code ErrorCode, message string, details map[string]any, statusCode int) {
	err := New(code, message).WithMetadataMap(details)
	WriteHTTPResponseWithStatus(w, err, statusCode)
}
