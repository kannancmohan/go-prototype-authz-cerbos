# Task: Create Hybrid Error Package (cockroachdb/errors wrapper)

## Overview
Create a production-ready error package that wraps `github.com/cockroachdb/errors` to provide a clean, domain-specific API while leveraging battle-tested error handling capabilities (stack traces, error chains, performance optimizations).

## Architecture
Similar to [`pkg/logger`](pkg/logger/logger.go) wrapping `zerolog`, we'll wrap `cockroachdb/errors` with our own interface to maintain control and provide consistent patterns across services.

## Tasks and Subtasks

### Phase 1: Setup and Core Wrapper

- [ ] **Initialize Package and Dependencies**
  - [ ] Add `github.com/cockroachdb/errors` to [`go.mod`](go.mod)
  - [ ] Create [`pkg/errors/errors.go`](pkg/errors/errors.go) with core types
  - [ ] Define `Error` struct with code, message, severity, metadata
  - [ ] Define `ErrorCode` type and constants
  - [ ] Define `Severity` type (error, warning, info)

- [ ] **Implement Core Wrapper Functions**
  - [ ] `New(code ErrorCode, message string) *Error` - create new error with stack trace
  - [ ] `Wrap(err error, code ErrorCode, message string) *Error` - wrap existing error
  - [ ] `Error() string` - implement error interface
  - [ ] `Unwrap() error` - support error chain traversal
  - [ ] `Is(target error) bool` - support errors.Is() checks
  - [ ] `As(target any) bool` - support errors.As() checks

- [ ] **Implement Stack Trace Integration**
  - [ ] Use `errors.WithStack()` internally for automatic stack capture
  - [ ] Add `StackTrace() string` method for formatted stack output
  - [ ] Implement stack trace formatting options
  - [ ] Add configuration for stack trace depth

### Phase 2: Structured Metadata and Context

- [ ] **Implement Metadata System**
  - [ ] Create `Metadata` type (map[string]any with type safety)
  - [ ] Add `WithMetadata(key string, value any) *Error` method
  - [ ] Add `WithMetadataMap(metadata map[string]any) *Error` method
  - [ ] Implement `Metadata() map[string]any` getter
  - [ ] Add metadata serialization for JSON responses

- [ ] **Add Context Support**
  - [ ] Implement `WithContext(ctx context.Context) *Error` for trace/span IDs
  - [ ] Extract OpenTelemetry trace/span IDs automatically
  - [ ] Add request ID and user ID extraction from context
  - [ ] Store context values in error metadata

### Phase 3: Error Categorization and Helpers

- [ ] **Define Error Domains and Codes**
  - [ ] Create error code constants in [`pkg/errors/codes.go`](pkg/errors/codes.go)
  - [ ] Define domains: auth, validation, database, external, internal
  - [ ] Map error codes to HTTP status codes
  - [ ] Add error severity classification

- [ ] **Implement Common Error Constructors**
  - [ ] `NotFound(resource string) *Error` - 404 errors
  - [ ] `ValidationError(message string, fields map[string]string) *Error` - 400 validation errors
  - [ ] `AuthError(message string) *Error` - 401 authentication errors
  - [ ] `ForbiddenError(message string) *Error` - 403 forbidden errors
  - [ ] `InternalError(message string, cause error) *Error` - 500 internal errors
  - [ ] `ConflictError(message string) *Error` - 409 conflict errors
  - [ ] `TimeoutError(operation string, duration time.Duration) *Error` - 408 timeout errors
  - [ ] `RateLimitError(retryAfter time.Duration) *Error` - 429 rate limit errors

- [ ] **Create Error Inspection Helpers**
  - [ ] `IsNotFound(err error) bool` - check if error is not found
  - [ ] `IsValidation(err error) bool` - check if error is validation error
  - [ ] `IsAuth(err error) bool` - check if error is authentication error
  - [ ] `IsForbidden(err error) bool` - check if error is forbidden
  - [ ] `IsInternal(err error) bool` - check if error is internal
  - [ ] `GetCode(err error) ErrorCode` - extract error code
  - [ ] `GetStatusCode(err error) int` - get HTTP status code
  - [ ] `GetSeverity(err error) Severity` - get error severity

### Phase 4: Logger Integration

- [ ] **Integrate with Existing Logger**
  - [ ] Create `Log(logger logger.Logger) *Error` method for fluent logging
  - [ ] Implement automatic error logging with stack traces
  - [ ] Add error metadata to log fields automatically
  - [ ] Support both context-aware and standard logging
  - [ ] Map error severity to appropriate log levels

- [ ] **Create Logger Helper Functions**
  - [ ] `LogError(logger logger.Logger, err error, msg string, args ...any)` - log error with context
  - [ ] `LogErrorCtx(ctx context.Context, logger logger.Logger, err error, msg string, args ...any)` - context-aware error logging
  - [ ] Add automatic error code and metadata extraction for logs

### Phase 5: HTTP Response Helpers

- [ ] **Implement HTTP Error Response Generation**
  - [ ] Create `WriteHTTPResponse(w http.ResponseWriter, err error)` function
  - [ ] Implement JSON error response format:
    ```json
    {
      "error": {
        "code": "NOT_FOUND",
        "message": "Resource not found",
        "details": { ... },
        "trace_id": "..."
      }
    }
    ```
  - [ ] Add proper HTTP status code setting
  - [ ] Include trace/span IDs when available
  - [ ] Support error details serialization

- [ ] **Create HTTP Middleware**
  - [ ] Implement error recovery middleware
  - [ ] Add automatic error response generation
  - [ ] Implement error logging in middleware
  - [ ] Add request context to errors automatically

### Phase 6: Testing and Quality Assurance

- [ ] **Write Unit Tests**
  - [ ] Test error creation and wrapping
  - [ ] Test stack trace capture and formatting
  - [ ] Test metadata handling and serialization
  - [ ] Test error inspection helpers
  - [ ] Test logger integration
  - [ ] Test HTTP response generation
  - [ ] Test error chain traversal
  - [ ] Test thread safety

- [ ] **Write Integration Tests**
  - [ ] Test end-to-end error flow (creation → logging → HTTP response)
  - [ ] Test OpenTelemetry integration
  - [ ] Test error propagation across service boundaries
  - [ ] Test error serialization/deserialization

- [ ] **Performance Testing**
  - [ ] Benchmark error creation overhead vs. raw cockroachdb/errors
  - [ ] Test memory allocation patterns
  - [ ] Profile error handling in high-load scenarios
  - [ ] Ensure wrapper overhead is minimal (<5%)

### Phase 7: Documentation and Examples

- [ ] **Write Package Documentation**
  - [ ] Create comprehensive package-level docs
  - [ ] Document all public types and functions
  - [ ] Add usage examples for common scenarios
  - [ ] Document error handling best practices

- [ ] **Create Usage Examples**
  - [ ] Basic error creation and handling
  - [ ] Error wrapping with metadata
  - [ ] HTTP error response example
  - [ ] Logger integration example
  - [ ] OpenTelemetry integration example
  - [ ] Error inspection and type checking
  - [ ] Migration from old error patterns

- [ ] **Create Developer Guide**
  - [ ] When to use which error constructor
  - [ ] Error logging best practices
  - [ ] Error response formatting guidelines
  - [ ] Troubleshooting common issues

### Phase 8: Migration and Rollout

- [ ] **Create Migration Utilities**
  - [ ] Add `FromStandardError(err error) *Error` converter
  - [ ] Create migration guide for existing services
  - [ ] Add backward compatibility helpers
  - [ ] Create codemod scripts for common patterns

- [ ] **Update Existing Services**
  - [ ] Migrate [`cmd/main.go`](cmd/main.go) to use new error package
  - [ ] Update error handling in HTTP handlers
  - [ ] Update error logging calls
  - [ ] Test migration in development environment

## Key Design Decisions

1. **Wrapper Pattern**: Similar to logger package, wrap cockroachdb/errors to maintain control and provide consistent API
2. **Immutable Errors**: All error modifications return new error instances for thread safety
3. **Fluent API**: Chainable methods for adding metadata and context
4. **Zero Config Stack Traces**: Automatic stack capture without configuration
5. **Logger Integration**: Seamless integration with existing logger package
6. **OpenTelemetry Ready**: Built-in support for trace/span correlation

## Dependencies

```go
require (
    github.com/cockroachdb/errors v1.11.1
    go.opentelemetry.io/otel v1.21.0 // for trace context
)
```

## Success Criteria

- [ ] Wrapper overhead is minimal (<5% performance impact)
- [ ] All errors include stack traces automatically
- [ ] Seamless integration with existing logger package
- [ ] Clean, domain-specific API surface
- [ ] Comprehensive test coverage (>90%)
- [ ] Clear migration path from existing error patterns
- [ ] Production-ready with proper error categorization
- [ ] OpenTelemetry integration working
- [ ] HTTP response helpers functional
- [ ] Documentation complete with examples

## Estimated Timeline

- **Phase 1 (Setup & Core):** 1-2 days
- **Phase 2 (Metadata & Context):** 1-2 days
- **Phase 3 (Categorization):** 2-3 days
- **Phase 4 (Logger Integration):** 1-2 days
- **Phase 5 (HTTP Helpers):** 1-2 days
- **Phase 6 (Testing):** 2-3 days
- **Phase 7 (Documentation):** 1-2 days
- **Phase 8 (Migration):** 2-3 days

**Total Estimated Time:** 2-3 weeks (vs. 3-4 weeks for from-scratch implementation)