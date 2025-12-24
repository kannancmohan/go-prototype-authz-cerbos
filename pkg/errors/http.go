package errors

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
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
