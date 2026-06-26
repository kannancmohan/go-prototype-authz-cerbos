package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
func createZerologLogger(opts *Options) (Logger, error) { //nolint:maintidx // factory function with necessary complexity
	// Validate configuration
	if err := validateOptions(opts); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeValidation, "invalid options")
	}

	// // Configure caller to use relative paths
	// zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
	// 	cwd, err := os.Getwd()
	// 	if err != nil {
	// 		return fmt.Sprintf("%s:%d", file, line)
	// 	}
	// 	rel, err := filepath.Rel(cwd, file)
	// 	if err != nil {
	// 		return fmt.Sprintf("%s:%d", file, line)
	// 	}
	// 	return fmt.Sprintf("%s:%d", rel, line)
	// }
	// Configure caller to use relative paths
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		// 1. Get the current working directory
		cwd, err := os.Getwd()
		if err != nil {
			// SECURE FALLBACK: If we can't get CWD, only show the filename
			return fmt.Sprintf("%s:%d", filepath.Base(file), line)
		}

		// 2. Calculate the relative path
		rel, err := filepath.Rel(cwd, file)
		if err != nil {
			// SECURE FALLBACK: If calculation fails, only show the filename
			return fmt.Sprintf("%s:%d", filepath.Base(file), line)
		}

		// 3. Optional: Strip "../" prefixes if you ran the binary from a subfolder
		// (e.g. if you are in 'pkg' and logging from 'cmd', Rel returns '../../cmd/main.go')
		// If you want to force clean 'cmd/main.go' style logs regardless of run directory:
		if strings.HasPrefix(rel, "../") {
			return fmt.Sprintf("%s:%d", filepath.Base(file), line)
		}

		return fmt.Sprintf("%s:%d", rel, line)
	}

	// Parse log level
	level := zerolog.InfoLevel
	switch opts.Level {
	case LevelDebug:
		level = zerolog.DebugLevel
	case LevelWarn:
		level = zerolog.WarnLevel
	case LevelError:
		level = zerolog.ErrorLevel
	case LevelFatal:
		level = zerolog.FatalLevel
	}

	// Configure output based on format
	var output io.Writer
	output = opts.Output
	if output == nil {
		output = os.Stdout
	}

	// Determine time format and console color settings
	timeFormat := opts.TimeFormat
	if timeFormat == "" {
		timeFormat = DefaultTimeFormat
	}
	noColor := opts.ConsoleNoColor

	switch opts.Format {
	case "console":
		// Human-readable console output with colors and formatting
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: timeFormat,
			NoColor:    noColor,
		}
	case "json":
		// Structured JSON output (zerolog's default format)
		// output is already set above
	default:
		// Fallback to JSON format
		// output is already set above
	}

	// Create base logger with timestamp and caller information (skip 3 frames to point to application code)
	zl := zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		CallerWithSkipFrameCount(zerologCallerSkipFrames).
		Logger()

	// Apply sampling for high-volume scenarios
	if opts.Sampling != nil {
		zl = zl.Sample(&zerolog.BurstSampler{
			Burst:       uint32(opts.Sampling.Initial),                              //nolint:gosec // validation ensures non‑negative and within safe bounds
			Period:      0,                                                          // Reset burst counter immediately
			NextSampler: &zerolog.BasicSampler{N: uint32(opts.Sampling.Thereafter)}, //nolint:gosec // validation ensures positive and within safe bounds
		})
	}

	return &zerologLogger{logger: zl, opts: *opts}, nil
}

// validateOptions validates logger configuration options
func validateOptions(opts *Options) error {
	validLevels := map[string]bool{LevelDebug: true, LevelInfo: true, LevelWarn: true, LevelError: true, LevelFatal: true}
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
	// Use only explicitly configured PII fields
	return slices.Contains(l.opts.PIIFields, key)
}

// filterPII removes PII fields from log arguments
func (l *zerologLogger) filterPII(args ...any) []any {
	if !l.opts.GDPRFilterPII {
		return args
	}

	filtered := make([]any, 0, len(args))
	for i := 0; i < len(args); i += argsPerAttr {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			if !l.isPIIField(key) {
				filtered = append(filtered, args[i], args[i+1])
			}
		}
	}

	return filtered
}

// enrichLogEvent adds structured error metadata to a log event if the error is a structured error.
func (l *zerologLogger) enrichLogEvent(logEvent *zerolog.Event, err error) *zerolog.Event {
	if appErr, ok := errors.AsError(err); ok {
		if key := l.opts.ErrorCodeFieldKey; key != "" {
			logEvent = logEvent.Str(key, string(appErr.Code))
		}
		if key := l.opts.SeverityFieldKey; key != "" {
			logEvent = logEvent.Str(key, string(appErr.Severity))
		}
		for k, v := range appErr.Metadata() {
			logEvent = logEvent.Interface(k, v)
		}
	}

	return logEvent
}

// addTraceSpanFields adds OpenTelemetry trace/span IDs to the logger if configured.
func addTraceSpanFields(logger *zerolog.Logger, ctx context.Context, opts *Options) zerolog.Logger {
	if !opts.ExtractTraceInfo {
		return *logger
	}
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return *logger
	}
	traceKey := opts.TraceIDFieldKey
	spanKey := opts.SpanIDFieldKey
	if traceKey == "" && spanKey == "" {
		return *logger
	}
	builder := logger.With()
	if traceKey != "" {
		builder = builder.Str(traceKey, span.SpanContext().TraceID().String())
	}
	if spanKey != "" {
		builder = builder.Str(spanKey, span.SpanContext().SpanID().String())
	}

	return builder.Logger()
}

// FromContext extracts values from context and returns enriched logger
func (l *zerologLogger) FromContext(ctx context.Context) Logger {
	newLogger := addTraceSpanFields(&l.logger, ctx, &l.opts)

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
	logEvent = l.enrichLogEvent(logEvent, err)
	logEvent.Fields(filteredArgs).Msg(msg)
}

// FatalCtxWithErr logs a fatal error with stack trace and additional fields (context-aware)
func (l *zerologLogger) FatalCtxWithErr(ctx context.Context, err error, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	logEvent := l.FromContext(ctx).(*zerologLogger).logger.Fatal().Err(err).Stack() //nolint:forcetypeassert // FromContext returns *zerologLogger
	logEvent = l.enrichLogEvent(logEvent, err)
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
	logEvent = l.enrichLogEvent(logEvent, err)
	logEvent.Fields(filteredArgs).Msg(msg)
}

// FatalWithErr logs a fatal error with stack trace and additional fields (context-free)
func (l *zerologLogger) FatalWithErr(err error, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	logEvent := l.logger.Fatal().Err(err).Stack()
	logEvent = l.enrichLogEvent(logEvent, err)
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
