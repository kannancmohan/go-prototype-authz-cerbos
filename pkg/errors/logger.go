package errors

import (
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/kannancmohan/go-prototype-authz-cerbos/pkg/logger"
)

// prepareLogFields prepares log fields from error metadata
func (e *Error) prepareLogFields() []any {
	//nolint:mnd // *2 because each metadata entry becomes 2 slice elements (key, value) - standard pattern
	fields := make([]any, 0, len(e.meta)*2)
	for k, v := range e.meta {
		fields = append(fields, k, v)
	}
	fields = append(fields, "error_code", e.Code, "severity", e.Severity)

	return fields
}

// Log logs the error using the provided logger and returns the error for method chaining
func (e *Error) Log(log logger.Logger) *Error {
	if log == nil {
		return e
	}

	fields := e.prepareLogFields()
	e.logWithoutContext(log, fields)

	return e
}

// LogCtx logs the error with context using the provided logger and returns the error for method chaining
func (e *Error) LogCtx(ctx any, log logger.Logger) *Error {
	if log == nil {
		return e
	}

	fields := e.prepareLogFields()
	e.logWithContext(ctx, log, fields)

	return e
}

// LogError logs an error with automatic metadata extraction
func LogError(log logger.Logger, err error, msg string, args ...any) {
	if log == nil {
		return
	}

	// Check if it's our structured error
	var appErr *Error
	if errors.As(err, &appErr) {
		_ = appErr.Log(log)

		return
	}

	// For standard errors, just log normally
	log.ErrorWithErr(err, msg, args...)
}

// LogErrorCtx logs an error with context and automatic metadata extraction
func LogErrorCtx(ctx any, log logger.Logger, err error, msg string, args ...any) {
	if log == nil {
		return
	}

	// Check if it's our structured error
	var appErr *Error
	if errors.As(err, &appErr) {
		_ = appErr.LogCtx(ctx, log)
		return
	}

	// For standard errors, just log normally
	if ctx != nil {
		if ctxValue, ok := ctx.(interface{ ErrorCtx(error, string, ...any) }); ok {
			ctxValue.ErrorCtx(err, msg, args...)
		} else {
			log.ErrorWithErr(err, msg, args...)
		}
	} else {
		log.ErrorWithErr(err, msg, args...)
	}
}

// LogErrorFields logs an error with additional fields extracted from metadata
func LogErrorFields(log logger.Logger, err error, msg string, fields map[string]any) {
	if log == nil {
		return
	}

	// Check if it's our structured error
	var appErr *Error
	if errors.As(err, &appErr) {
		_ = appErr.Log(log)
		return
	}

	// For standard errors, convert fields map to args slice and log
	//nolint:mnd // *2 because each field entry becomes 2 slice elements (key, value) - standard pattern
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	log.ErrorWithErr(err, msg, args...)
}

// ToLoggerFields converts error metadata to logger fields
func (e *Error) ToLoggerFields() []any {
	if e == nil {
		return nil
	}

	//nolint:mnd // *2+4: *2 for metadata key-value pairs, +4 for error_code and severity (2 elements each)
	fields := make([]any, 0, len(e.meta)*2+4)
	for k, v := range e.meta {
		fields = append(fields, k, v)
	}
	fields = append(fields, "error_code", e.Code, "severity", e.Severity)

	return fields
}

// String returns a string representation of the error for logging
func (e *Error) String() string {
	if e == nil {
		return ""
	}

	base := fmt.Sprintf("[%s] %s", e.Code, e.Message)
	if len(e.meta) > 0 {
		base += fmt.Sprintf(" (metadata: %v)", e.meta)
	}

	return base
}

// logWithContext logs the error with context-aware methods if available
func (e *Error) logWithContext(ctx any, log logger.Logger, fields []any) {
	if ctx == nil {
		e.logWithoutContext(log, fields)
		return
	}

	switch e.Severity {
	case SeverityError:
		if ctxValue, ok := ctx.(interface{ ErrorCtx(error, string, ...any) }); ok {
			ctxValue.ErrorCtx(e.cause, e.Message, fields...)
		} else {
			log.ErrorWithErr(e.cause, e.Message, fields...)
		}
	case SeverityWarning:
		if ctxValue, ok := ctx.(interface{ WarnCtx(string, ...any) }); ok {
			ctxValue.WarnCtx(e.Message, fields...)
		} else {
			log.Warn(e.Message, fields...)
		}
	case SeverityInfo:
		if ctxValue, ok := ctx.(interface{ InfoCtx(string, ...any) }); ok {
			ctxValue.InfoCtx(e.Message, fields...)
		} else {
			log.Info(e.Message, fields...)
		}
	default:
		log.ErrorWithErr(e.cause, e.Message, fields...)
	}
}

// logWithoutContext logs the error without context
func (e *Error) logWithoutContext(log logger.Logger, fields []any) {
	switch e.Severity {
	case SeverityError:
		log.ErrorWithErr(e.cause, e.Message, fields...)
	case SeverityWarning:
		log.Warn(e.Message, fields...)
	case SeverityInfo:
		log.Info(e.Message, fields...)
	default:
		log.ErrorWithErr(e.cause, e.Message, fields...)
	}
}
