// Package logger provides a comprehensive logging interface with context-aware capabilities,
// structured logging support, and GDPR compliance features. It supports multiple logger
// implementations through a factory pattern and includes advanced features like log sampling,
// context value extraction, and distributed tracing integration.
//
// Basic usage:
//
//	import "github.com/yourproject/pkg/logger"
//
//	// Create a factory with configuration
//	factory := logger.NewFactory(logger.Options{
//	    Level:  "info",
//	    Format: "json",
//	})
//
//	// Create a logger instance
//	log, err := factory.Create("zerolog")
//	if err != nil {
//	    panic(err)
//	}
//
//	// Use context-aware logging
//	ctx := context.WithValue(context.Background(), "request_id", "123")
//	log.InfoCtx(ctx, "Application started", "port", 8080)
//
//	// Use standard logging
//	log.Info("Application started", "port", 8080)
//
//	// Create child logger with fields
//	childLog := log.With("service", "api", "version", "1.0")
//	childLog.Info("Service initialized")
package logger

import (
	"context"
	"io"

	"github.com/kannancmohan/go-prototype-authz-cerbos/pkg/errors"
)

// Level represents log severity levels
type Level int

// Log levels defined as constants.
const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// String constants for log levels (used in configuration and parsing)
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelFatal = "fatal"
)

// Default values for logger configuration
const (
	// DefaultTimeFormat is the default time format used in logs
	DefaultTimeFormat = "2006-01-02T15:04:05Z07:00" // time.RFC3339
	// DefaultErrorFieldKey is the default key for error field
	DefaultErrorFieldKey = "error"
	// DefaultStackTraceFieldKey is the default key for stack trace field
	DefaultStackTraceFieldKey = "stack"
	// DefaultErrorCodeFieldKey is the default key for error code field
	DefaultErrorCodeFieldKey = "error_code"
	// DefaultSeverityFieldKey is the default key for severity field
	DefaultSeverityFieldKey = "severity"
	// DefaultTraceIDFieldKey is the default key for trace ID field
	DefaultTraceIDFieldKey = "trace_id"
	// DefaultSpanIDFieldKey is the default key for span ID field
	DefaultSpanIDFieldKey = "span_id"
)

// DefaultPIIFields are the default personally identifiable information fields
var DefaultPIIFields = []string{ //nolint:gochecknoglobals // global defaults are acceptable for configuration
	"password",
	"ssn",
	"credit_card",
	"email",
	"phone",
	"address",
}

// argsPerAttr defines the number of arguments per attribute in key-value logging.
// Each attribute consists of a key and a value, hence 2.
const argsPerAttr = 2

// slogCallerSkipFrames defines the number of stack frames to skip for slog caller resolution.
const slogCallerSkipFrames = 5

// zerologCallerSkipFrames defines the number of stack frames to skip for zerolog caller resolution.
const zerologCallerSkipFrames = 3

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return LevelDebug
	case InfoLevel:
		return LevelInfo
	case WarnLevel:
		return LevelWarn
	case ErrorLevel:
		return LevelError
	case FatalLevel:
		return LevelFatal
	default:
		return "unknown"
	}
}

// Logger defines the core interface for context-aware logging.
// It provides both context-aware and standard logging methods,
// supports structured logging with key-value pairs, and allows
// creating child loggers with additional fields.
type Logger interface {
	// Extract values from context and return enriched logger
	FromContext(ctx context.Context) Logger

	// Context-aware logging methods
	DebugCtx(ctx context.Context, msg string, args ...any)
	InfoCtx(ctx context.Context, msg string, args ...any)
	WarnCtx(ctx context.Context, msg string, args ...any)
	ErrorCtx(ctx context.Context, msg string, args ...any)
	FatalCtx(ctx context.Context, msg string, args ...any)

	// Context-aware logging methods with explicit error handling
	ErrorCtxWithErr(ctx context.Context, err error, msg string, args ...any)
	FatalCtxWithErr(ctx context.Context, err error, msg string, args ...any)

	// Standard logging methods
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Fatal(msg string, args ...any)

	// Standard logging methods with explicit error handling
	ErrorWithErr(err error, msg string, args ...any)
	FatalWithErr(err error, msg string, args ...any)

	// Create child logger with fields
	With(args ...any) Logger

	// Flush buffered logs
	Flush() error
}

// Options configures the logger with various settings including
// log level, output format, context keys for extraction, and
// GDPR compliance features.
type Options struct {
	Level              string          // "debug", "info", "warn", "error"
	Format             string          // "json", "console"
	ContextKeys        []string        // Keys to extract from context
	ExtractTraceInfo   bool            // Auto-extract OTel trace/span IDs
	Sampling           *SamplingConfig // Log sampling for high volume
	GDPRFilterPII      bool            // Filter PII fields for GDPR compliance
	PIIFields          []string        // Custom PII fields to filter (e.g., "password", "ssn", "email")
	DefaultPIIFields   []string        // Default PII fields used when PIIFields is empty (optional)
	TimeFormat         string          // Time format for logs (default: time.RFC3339)
	ConsoleNoColor     bool            // Disable colors in console output (default: false)
	ErrorFieldKey      string          // Field key for error (default: "error")
	StackTraceFieldKey string          // Field key for stack trace (default: "stack")
	ErrorCodeFieldKey  string          // Field key for error code (default: "error_code")
	SeverityFieldKey   string          // Field key for severity (default: "severity")
	TraceIDFieldKey    string          // Field key for trace ID (default: "trace_id")
	SpanIDFieldKey     string          // Field key for span ID (default: "span_id")
	Output             io.Writer       // Custom output writer (defaults to os.Stdout if nil)
}

// DefaultOptions returns an Options struct populated with default values.
func DefaultOptions() Options {
	return Options{
		Level:              LevelInfo,
		Format:             "json",
		ContextKeys:        nil,
		ExtractTraceInfo:   false,
		Sampling:           nil,
		GDPRFilterPII:      false,
		PIIFields:          nil,
		DefaultPIIFields:   DefaultPIIFields,
		TimeFormat:         DefaultTimeFormat,
		ConsoleNoColor:     false,
		ErrorFieldKey:      DefaultErrorFieldKey,
		StackTraceFieldKey: DefaultStackTraceFieldKey,
		ErrorCodeFieldKey:  DefaultErrorCodeFieldKey,
		SeverityFieldKey:   DefaultSeverityFieldKey,
		TraceIDFieldKey:    DefaultTraceIDFieldKey,
		SpanIDFieldKey:     DefaultSpanIDFieldKey,
		Output:             nil,
	}
}

// SamplingConfig prevents log spam in high-volume scenarios by
// logging the first N messages of each type, then every Mth message.
type SamplingConfig struct {
	Initial    int // Log first N messages of each type
	Thereafter int // Then log every Mth message
}

// Factory creates logger instances with different implementations.
// It is immutable and safe for concurrent use.
type Factory struct {
	opts *Options
}

// NewFactory creates a new logger factory with the specified options.
func NewFactory(opts *Options) *Factory {
	return &Factory{opts: opts}
}

// Create creates a new logger instance with the specified implementation.
// Supported implementations: "slog", "zerolog", "zap"
func (f *Factory) Create(impl string) (Logger, error) {
	switch impl {
	case "slog":
		return createSlogLogger(f.opts)
	case "zerolog":
		return createZerologLogger(f.opts)
	case "zap":
		return newZapLogger(f.opts), nil
	default:
		return nil, errors.New(errors.ErrCodeInvalidInput, "unknown implementation: "+impl)
	}
}

// newZapLogger creates a new zap-based logger (placeholder for future implementation)
func newZapLogger(opts *Options) Logger {
	// TODO: Implement zap logger
	return nil
}
