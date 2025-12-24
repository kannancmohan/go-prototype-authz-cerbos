# Task: Move logging logic from pkg/errors/logger.go into pkg/logger implementations

## Overview
Remove `pkg/errors/logger.go` and integrate structured error logging directly into `pkg/logger/slog.go` and `pkg/logger/zerolog.go`. This will eliminate the dependency of `pkg/errors` on `pkg/logger` and centralize logging logic.

## Tasks

### 1. Analyze current usage of logging functions
- [ ] Identify all call sites of `Log`, `LogCtx`, `LogError`, `LogErrorCtx`, `LogErrorFields` in the codebase (already done).
- [ ] Determine which functions are used only in tests vs production code.
- [ ] Decide on migration strategy for each call site (replace with direct logger calls or keep compatibility).

### 2. Update pkg/logger to import pkg/errors
- [ ] Add import `github.com/kannancmohan/go-prototype-authz-cerbos/pkg/errors` to `pkg/logger/slog.go`.
- [ ] Add same import to `pkg/logger/zerolog.go`.
- [ ] Ensure the import does not cause cyclic dependencies (it shouldn't).

### 3. Enhance logger methods to detect *errors.Error
- [ ] In `pkg/logger/slog.go`, modify `ErrorWithErr` and `ErrorCtxWithErr` to check if `err` is of type `*errors.Error`.
- [ ] If yes, extract metadata via `err.Metadata()`, error code via `err.Code`, severity via `err.Severity`.
- [ ] Include these as additional log fields (e.g., `error_code`, `severity`, plus each metadata key-value pair).
- [ ] Ensure stack trace is still logged (already via `fmt.Sprintf("%+v", err)`).
- [ ] Apply the same enhancement to `FatalWithErr` and `FatalCtxWithErr`.
- [ ] Also consider enhancing `Error` and `ErrorCtx` (without explicit error param) if they receive an error as part of args? (Not needed.)

- [ ] In `pkg/logger/zerolog.go`, make similar modifications to `ErrorWithErr`, `ErrorCtxWithErr`, `FatalWithErr`, `FatalCtxWithErr`.
- [ ] Use zerolog's `Fields` method to add metadata fields.
- [ ] Ensure severity mapping (errors severity vs log level) is consistent (maybe log at appropriate level? Currently logger level is independent; we can keep logging at error level but include severity field).

### 4. Remove pkg/errors/logger.go
- [ ] Delete the file `pkg/errors/logger.go`.
- [ ] Remove its import from `pkg/errors` (the import line `github.com/kannancmohan/go-prototype-authz-cerbos/pkg/logger`).
- [ ] Update `pkg/errors/errors.go` documentation to reflect that logging is now handled by the logger package.

### 5. Update call sites in production code
- [ ] In `pkg/errors/http.go`, replace calls to `appErr.Log(...)` and `appErr.LogCtx(...)` with direct logger calls using the enhanced logger methods.
- [ ] Ensure the logger instance is available (likely via context or injected). Adjust accordingly.
- [ ] Update any other non‑test files that use the removed functions.

### 6. Update tests
- [ ] In `pkg/errors/errors_test.go`, update tests that rely on `Log`, `LogCtx`, `LogError`, `LogErrorCtx`, `LogErrorFields`.
- [ ] Replace with appropriate logger calls, or keep the tests but mock the logger to verify logging behavior.
- [ ] Ensure tests still pass after changes.

### 7. Verify no regression
- [ ] Run `go test ./pkg/errors` to ensure all tests pass.
- [ ] Run `go test ./pkg/logger` to ensure logger tests pass.
- [ ] Run any integration tests that involve logging.

### 8. Clean up and finalize
- [ ] Remove any unused helper functions that were only used by the deleted logging logic (e.g., `prepareLogFields`, `logWithContext`, `logWithoutContext`, `ToLoggerFields`).
- [ ] Check if `String` method on `*Error` is still needed (it's used for logging but may be kept). Decide whether to keep or remove.
- [ ] Update any documentation comments in `pkg/errors` and `pkg/logger` to reflect the new behavior.

## Subtask Dependencies
- Task 2 (import) must be done before Task 3.
- Task 3 must be done before Task 5 and Task 6.
- Task 4 can be done after Task 5 and Task 6 are updated (or simultaneously).
- Task 7 should be the final verification.

## Notes
- Keep backward compatibility: The logger interface remains unchanged; only internal behavior changes.
- Ensure that logging of standard errors (non‑*errors.Error) works exactly as before.
- Consider performance impact of type assertions; it's negligible.

Each task can be picked up independently by an AI agent, but they should be executed in the order listed to avoid breaking the build.