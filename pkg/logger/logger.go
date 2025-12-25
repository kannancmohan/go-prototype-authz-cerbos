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

// argsPerAttr defines the number of arguments per attribute in key-value logging.
// Each attribute consists of a key and a value, hence 2.
const argsPerAttr = 2

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
	Level            string          // "debug", "info", "warn", "error"
	Format           string          // "json", "console"
	ContextKeys      []string        // Keys to extract from context
	ExtractTraceInfo bool            // Auto-extract OTel trace/span IDs
	Sampling         *SamplingConfig // Log sampling for high volume
	GDPRFilterPII    bool            // Filter PII fields for GDPR compliance
	PIIFields        []string        // Custom PII fields to filter (e.g., "password", "ssn", "email")
	Output           io.Writer       // Custom output writer (defaults to os.Stdout if nil)
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
