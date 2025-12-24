package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"

	"github.com/kannancmohan/go-prototype-authz-cerbos/pkg/errors"
)

// zerologLogger implements the Logger interface using zerolog
type zerologLogger struct {
	logger zerolog.Logger
	opts   Options
}

// createZerologLogger creates a new zerolog logger with comprehensive configuration support
func createZerologLogger(opts Options) (Logger, error) {
	// Validate configuration
	if err := validateOptions(opts); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeValidation, "invalid options")
	}

	// Parse log level
	level := zerolog.InfoLevel
	switch opts.Level {
	case "debug":
		level = zerolog.DebugLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	case "fatal":
		level = zerolog.FatalLevel
	}

	// Configure output based on format
	var output io.Writer
	output = opts.Output
	if output == nil {
		output = os.Stdout
	}

	switch opts.Format {
	case "console":
		// Human-readable console output with colors and formatting
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	case "json":
		// Structured JSON output (zerolog's default format)
		// output is already set above
	default:
		// Fallback to JSON format
		// output is already set above
	}

	// Create base logger with timestamp and caller information
	zl := zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Caller().
		Logger()

	// Apply sampling for high-volume scenarios
	if opts.Sampling != nil {
		zl = zl.Sample(&zerolog.BurstSampler{
			Burst:       uint32(opts.Sampling.Initial),                              // Log first N messages
			Period:      0,                                                          // Reset burst counter immediately
			NextSampler: &zerolog.BasicSampler{N: uint32(opts.Sampling.Thereafter)}, // Then every Mth
		})
	}

	return &zerologLogger{logger: zl, opts: opts}, nil
}

// validateOptions validates logger configuration options
func validateOptions(opts Options) error {
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true, "fatal": true}
	if !validLevels[opts.Level] {
		return errors.ValidationError(fmt.Sprintf("invalid log level: %s", opts.Level))
	}

	validFormats := map[string]bool{"json": true, "console": true}
	if !validFormats[opts.Format] {
		return errors.ValidationError(fmt.Sprintf("invalid format: %s", opts.Format))
	}

	if opts.Sampling != nil {
		if opts.Sampling.Initial < 0 || opts.Sampling.Thereafter <= 0 {
			return errors.ValidationError("invalid sampling configuration")
		}
	}

	return nil
}

// isPIIField checks if a field contains personally identifiable information
func (l *zerologLogger) isPIIField(key string) bool {
	// Use custom PII fields from options, or default to common PII fields
	piiFields := l.opts.PIIFields
	if len(piiFields) == 0 {
		// Default PII fields if none specified
		piiFields = []string{"password", "ssn", "credit_card", "email", "phone", "address"}
	}

	return slices.Contains(piiFields, key)
}

// filterPII removes PII fields from log arguments
func (l *zerologLogger) filterPII(args ...any) []any {
	if !l.opts.GDPRFilterPII {
		return args
	}

	filtered := make([]any, 0, len(args))
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			if !l.isPIIField(key) {
				filtered = append(filtered, args[i], args[i+1])
			}
		}
	}
	return filtered
}

// FromContext extracts values from context and returns enriched logger
func (l *zerologLogger) FromContext(ctx context.Context) Logger {
	newLogger := l.logger

	// Extract OpenTelemetry trace/span IDs
	if l.opts.ExtractTraceInfo {
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			newLogger = newLogger.With().
				Str("trace_id", span.SpanContext().TraceID().String()).
				Str("span_id", span.SpanContext().SpanID().String()).
				Logger()
		}
	}

	// Extract configured context keys (e.g., request_id, user_id)
	for _, key := range l.opts.ContextKeys {
		if value := ctx.Value(key); value != nil {
			newLogger = newLogger.With().Interface(key, value).Logger()
		}
	}

	return &zerologLogger{logger: newLogger, opts: l.opts}
}

// Context-aware logging methods with error wrapping support
func (l *zerologLogger) DebugCtx(ctx context.Context, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	l.FromContext(ctx).(*zerologLogger).logger.Debug().Fields(filteredArgs).Msg(msg) //nolint:forcetypeassert // FromContext returns *zerologLogger
}

func (l *zerologLogger) InfoCtx(ctx context.Context, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	l.FromContext(ctx).(*zerologLogger).logger.Info().Fields(filteredArgs).Msg(msg) //nolint:forcetypeassert // FromContext returns *zerologLogger
}

func (l *zerologLogger) WarnCtx(ctx context.Context, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	l.FromContext(ctx).(*zerologLogger).logger.Warn().Fields(filteredArgs).Msg(msg) //nolint:forcetypeassert // FromContext returns *zerologLogger
}

func (l *zerologLogger) ErrorCtx(ctx context.Context, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	l.FromContext(ctx).(*zerologLogger).logger.Error().Fields(filteredArgs).Msg(msg) //nolint:forcetypeassert // FromContext returns *zerologLogger
}

func (l *zerologLogger) FatalCtx(ctx context.Context, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	l.FromContext(ctx).(*zerologLogger).logger.Fatal().Fields(filteredArgs).Msg(msg) //nolint:forcetypeassert // FromContext returns *zerologLogger
}

// ErrorCtxWithErr logs an error with stack trace and additional fields (context-aware)
func (l *zerologLogger) ErrorCtxWithErr(ctx context.Context, err error, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	logEvent := l.FromContext(ctx).(*zerologLogger).logger.Error().Err(err).Stack() //nolint:forcetypeassert // FromContext returns *zerologLogger
	// If it's a structured error, add metadata, code, and severity
	if appErr, ok := err.(*errors.Error); ok {
		logEvent = logEvent.Str("error_code", string(appErr.Code)).
			Str("severity", string(appErr.Severity))
		for k, v := range appErr.Metadata() {
			logEvent = logEvent.Interface(k, v)
		}
	}
	logEvent.Fields(filteredArgs).Msg(msg)
}

// FatalCtxWithErr logs a fatal error with stack trace and additional fields (context-aware)
func (l *zerologLogger) FatalCtxWithErr(ctx context.Context, err error, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	logEvent := l.FromContext(ctx).(*zerologLogger).logger.Fatal().Err(err).Stack() //nolint:forcetypeassert // FromContext returns *zerologLogger
	// If it's a structured error, add metadata, code, and severity
	if appErr, ok := err.(*errors.Error); ok {
		logEvent = logEvent.Str("error_code", string(appErr.Code)).
			Str("severity", string(appErr.Severity))
		for k, v := range appErr.Metadata() {
			logEvent = logEvent.Interface(k, v)
		}
	}
	logEvent.Fields(filteredArgs).Msg(msg)
}

// Standard logging methods (context-free)
func (l *zerologLogger) Debug(msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	l.logger.Debug().Fields(filteredArgs).Msg(msg)
}

func (l *zerologLogger) Info(msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	l.logger.Info().Fields(filteredArgs).Msg(msg)
}

func (l *zerologLogger) Warn(msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	l.logger.Warn().Fields(filteredArgs).Msg(msg)
}

func (l *zerologLogger) Error(msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	l.logger.Error().Fields(filteredArgs).Msg(msg)
}

func (l *zerologLogger) Fatal(msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	l.logger.Fatal().Fields(filteredArgs).Msg(msg)
}

// ErrorWithErr logs an error with stack trace and additional fields (context-free)
func (l *zerologLogger) ErrorWithErr(err error, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	logEvent := l.logger.Error().Err(err).Stack()
	// If it's a structured error, add metadata, code, and severity
	if appErr, ok := err.(*errors.Error); ok {
		logEvent = logEvent.Str("error_code", string(appErr.Code)).
			Str("severity", string(appErr.Severity))
		for k, v := range appErr.Metadata() {
			logEvent = logEvent.Interface(k, v)
		}
	}
	logEvent.Fields(filteredArgs).Msg(msg)
}

// FatalWithErr logs a fatal error with stack trace and additional fields (context-free)
func (l *zerologLogger) FatalWithErr(err error, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	logEvent := l.logger.Fatal().Err(err).Stack()
	// If it's a structured error, add metadata, code, and severity
	if appErr, ok := err.(*errors.Error); ok {
		logEvent = logEvent.Str("error_code", string(appErr.Code)).
			Str("severity", string(appErr.Severity))
		for k, v := range appErr.Metadata() {
			logEvent = logEvent.Interface(k, v)
		}
	}
	logEvent.Fields(filteredArgs).Msg(msg)
}

// With creates child logger with additional fields
func (l *zerologLogger) With(args ...any) Logger {
	filteredArgs := l.filterPII(args...)
	return &zerologLogger{
		logger: l.logger.With().Fields(filteredArgs).Logger(),
		opts:   l.opts,
	}
}

// Flush ensures buffered logs are written (zerolog has no buffer)
func (l *zerologLogger) Flush() error {
	return nil // Zerolog doesn't buffer
}
