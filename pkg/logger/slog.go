package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/kannancmohan/go-prototype-authz-cerbos/pkg/errors"
)

// slogLogger implements the Logger interface using slog
type slogLogger struct {
	logger *slog.Logger
	opts   Options
}

// createSlogLogger creates a new slog logger with comprehensive configuration support
func createSlogLogger(opts *Options) (Logger, error) { //nolint:maintidx // factory function with necessary complexity
	// Validate configuration
	if err := validateOptions(opts); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeValidation, "invalid options")
	}

	// Parse log level
	var level slog.Level
	switch opts.Level {
	case LevelDebug:
		level = slog.LevelDebug
	case LevelInfo:
		level = slog.LevelInfo
	case LevelWarn:
		level = slog.LevelWarn
	case LevelError:
		level = slog.LevelError
	case LevelFatal:
		level = slog.LevelError // slog doesn't have fatal level, map to error
	default:
		level = slog.LevelInfo
	}

	// Configure output based on format
	var output io.Writer
	var handler slog.Handler

	// Use custom output if provided, otherwise default to os.Stdout
	output = opts.Output
	if output == nil {
		output = os.Stdout
	}

	// Determine time format
	timeFormat := opts.TimeFormat
	if timeFormat == "" {
		timeFormat = DefaultTimeFormat
	}

	switch opts.Format {
	case "console":
		// Human-readable console output
		handler = slog.NewTextHandler(output, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				// Customize time format
				if a.Key == slog.TimeKey {
					return slog.Attr{
						Key:   a.Key,
						Value: slog.StringValue(time.Now().Format(timeFormat)),
					}
				}

				return a
			},
		})
	case "json":
		// Structured JSON output
		handler = slog.NewJSONHandler(output, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		})
	default:
		// Fallback to JSON format
		handler = slog.NewJSONHandler(output, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		})
	}

	// Apply sampling for high-volume scenarios
	if opts.Sampling != nil {
		handler = &samplingHandler{
			handler:    handler,
			counts:     make(map[string]int),
			initial:    opts.Sampling.Initial,
			thereafter: opts.Sampling.Thereafter,
		}
	}

	// Create base logger
	logger := slog.New(handler)

	return &slogLogger{logger: logger, opts: *opts}, nil
}

// isPIIField checks if a field contains personally identifiable information
func (l *slogLogger) isPIIField(key string) bool {
	// Use only explicitly configured PII fields
	return slices.Contains(l.opts.PIIFields, key)
}

// filterPII removes PII fields from log arguments
func (l *slogLogger) filterPII(args ...any) []any {
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

// convertArgsToAttrs converts key-value pairs to slog attributes
func convertArgsToAttrs(args ...any) []slog.Attr {
	if len(args)%argsPerAttr != 0 {
		// If odd number of args, treat the last one as a message
		return []slog.Attr{slog.String("message", fmt.Sprintf("%v", args[len(args)-1]))}
	}

	attrs := make([]slog.Attr, 0, len(args)/argsPerAttr)
	for i := 0; i < len(args); i += argsPerAttr {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			value := args[i+1]
			attrs = append(attrs, slog.Any(key, value))
		}
	}

	return attrs
}

// FromContext extracts values from context and returns enriched logger
func (l *slogLogger) FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return l
	}

	newLogger := l.logger

	// Extract OpenTelemetry trace/span IDs
	if l.opts.ExtractTraceInfo {
		span := trace.SpanFromContext(ctx)
		if span.SpanContext().IsValid() {
			traceKey := l.opts.TraceIDFieldKey
			spanKey := l.opts.SpanIDFieldKey
			// Only include trace/span IDs if field keys are explicitly set
			hasTrace := traceKey != ""
			hasSpan := spanKey != ""
			switch {
			case hasTrace && hasSpan:
				newLogger = newLogger.With(
					traceKey, span.SpanContext().TraceID().String(),
					spanKey, span.SpanContext().SpanID().String(),
				)
			case hasTrace:
				newLogger = newLogger.With(traceKey, span.SpanContext().TraceID().String())
			case hasSpan:
				newLogger = newLogger.With(spanKey, span.SpanContext().SpanID().String())
			}
		}
	}

	// Extract configured context keys (e.g., request_id, user_id)
	for _, key := range l.opts.ContextKeys {
		if value := ctx.Value(key); value != nil {
			newLogger = newLogger.With(key, value)
		}
	}

	return &slogLogger{logger: newLogger, opts: l.opts}
}

// Context-aware logging methods with error wrapping support
func (l *slogLogger) DebugCtx(ctx context.Context, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	attrs := convertArgsToAttrs(filteredArgs...)
	l.FromContext(ctx).(*slogLogger).logger.DebugContext(ctx, msg, attrsToArgs(attrs)...) //nolint:forcetypeassert // FromContext returns *slogLogger
}

func (l *slogLogger) InfoCtx(ctx context.Context, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	attrs := convertArgsToAttrs(filteredArgs...)
	l.FromContext(ctx).(*slogLogger).logger.InfoContext(ctx, msg, attrsToArgs(attrs)...) //nolint:forcetypeassert // FromContext returns *slogLogger
}

func (l *slogLogger) WarnCtx(ctx context.Context, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	attrs := convertArgsToAttrs(filteredArgs...)
	l.FromContext(ctx).(*slogLogger).logger.WarnContext(ctx, msg, attrsToArgs(attrs)...) //nolint:forcetypeassert // FromContext returns *slogLogger
}

func (l *slogLogger) ErrorCtx(ctx context.Context, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	attrs := convertArgsToAttrs(filteredArgs...)
	l.FromContext(ctx).(*slogLogger).logger.ErrorContext(ctx, msg, attrsToArgs(attrs)...) //nolint:forcetypeassert // FromContext returns *slogLogger
}

func (l *slogLogger) FatalCtx(ctx context.Context, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	attrs := convertArgsToAttrs(filteredArgs...)
	l.FromContext(ctx).(*slogLogger).logger.ErrorContext(ctx, msg, attrsToArgs(attrs)...) //nolint:forcetypeassert // FromContext returns *slogLogger
	// slog doesn't have fatal, so we exit after logging
	os.Exit(1)
}

// logErrorInternal handles common error logging logic for both context and non-context scenarios.
func (l *slogLogger) logErrorInternal(ctx context.Context, err error, msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	attrs := convertArgsToAttrs(filteredArgs...)
	var logger *slog.Logger
	if ctx != nil {
		logger = l.FromContext(ctx).(*slogLogger).logger //nolint:forcetypeassert // FromContext returns *slogLogger
	} else {
		logger = l.logger
	}
	stack := fmt.Sprintf("%+v", err)
	var appErr *errors.Error
	if e, ok := errors.AsError(err); ok {
		appErr = e
		stack = appErr.StackTrace()
	}
	// Determine field keys, only include fields whose keys are explicitly set
	extraFields := []any{}
	if key := l.opts.ErrorFieldKey; key != "" {
		extraFields = append(extraFields, key, err)
	}
	if key := l.opts.StackTraceFieldKey; key != "" {
		extraFields = append(extraFields, key, stack)
	}
	if appErr != nil {
		if key := l.opts.ErrorCodeFieldKey; key != "" {
			extraFields = append(extraFields, key, appErr.Code)
		}
		if key := l.opts.SeverityFieldKey; key != "" {
			extraFields = append(extraFields, key, appErr.Severity)
		}
		for k, v := range appErr.Metadata() {
			extraFields = append(extraFields, k, v)
		}
	}
	if ctx != nil {
		logger.ErrorContext(ctx, msg, append(extraFields, attrsToArgs(attrs)...)...)
	} else {
		logger.Error(msg, append(extraFields, attrsToArgs(attrs)...)...)
	}
}

// ErrorCtxWithErr logs an error with stack trace and additional fields (context-aware)
func (l *slogLogger) ErrorCtxWithErr(ctx context.Context, err error, msg string, args ...any) {
	l.logErrorInternal(ctx, err, msg, args...)
}

// FatalCtxWithErr logs a fatal error with stack trace and additional fields (context-aware)
func (l *slogLogger) FatalCtxWithErr(ctx context.Context, err error, msg string, args ...any) {
	l.logErrorInternal(ctx, err, msg, args...)
	// slog doesn't have fatal, so we exit after logging
	os.Exit(1)
}

// Standard logging methods (context-free)
func (l *slogLogger) Debug(msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	attrs := convertArgsToAttrs(filteredArgs...)
	l.logger.Debug(msg, attrsToArgs(attrs)...)
}

func (l *slogLogger) Info(msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	attrs := convertArgsToAttrs(filteredArgs...)
	l.logger.Info(msg, attrsToArgs(attrs)...)
}

func (l *slogLogger) Warn(msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	attrs := convertArgsToAttrs(filteredArgs...)
	l.logger.Warn(msg, attrsToArgs(attrs)...)
}

func (l *slogLogger) Error(msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	attrs := convertArgsToAttrs(filteredArgs...)
	l.logger.Error(msg, attrsToArgs(attrs)...)
}

func (l *slogLogger) Fatal(msg string, args ...any) {
	filteredArgs := l.filterPII(args...)
	attrs := convertArgsToAttrs(filteredArgs...)
	l.logger.Error(msg, attrsToArgs(attrs)...)
	// slog doesn't have fatal, so we exit after logging
	os.Exit(1)
}

// ErrorWithErr logs an error with stack trace and additional fields (context-free)
func (l *slogLogger) ErrorWithErr(err error, msg string, args ...any) {
	l.logErrorInternal(nil, err, msg, args...) //nolint:staticcheck // nil context is intentional for non-context logging
}

// FatalWithErr logs a fatal error with stack trace and additional fields (context-free)
func (l *slogLogger) FatalWithErr(err error, msg string, args ...any) {
	l.logErrorInternal(nil, err, msg, args...) //nolint:staticcheck // nil context is intentional for non-context logging
	// slog doesn't have fatal, so we exit after logging
	os.Exit(1)
}

// With creates child logger with additional fields
func (l *slogLogger) With(args ...any) Logger {
	filteredArgs := l.filterPII(args...)
	attrs := convertArgsToAttrs(filteredArgs...)
	return &slogLogger{
		logger: l.logger.With(attrsToArgs(attrs)...),
		opts:   l.opts,
	}
}

// Flush ensures buffered logs are written (slog has no buffer)
func (l *slogLogger) Flush() error {
	return nil // slog doesn't buffer
}

// Helper function to convert attrs to args for slog
func attrsToArgs(attrs []slog.Attr) []any {
	args := make([]any, 0, len(attrs)*argsPerAttr)
	for i := range attrs {
		attr := &attrs[i]
		args = append(args, attr.Key, attr.Value.Any())
	}

	return args
}

// samplingHandler implements sampling for slog by wrapping another handler
type samplingHandler struct {
	handler    slog.Handler
	mu         sync.Mutex
	counts     map[string]int
	initial    int
	thereafter int
}

// Enabled returns true if the handler is enabled for the given level
func (h *samplingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle processes the log record with sampling
func (h *samplingHandler) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic
	key := fmt.Sprintf("%s:%s", r.Level, r.Message)
	h.mu.Lock()
	count := h.counts[key]
	count++
	h.counts[key] = count
	h.mu.Unlock()

	if count <= h.initial || (count-h.initial)%h.thereafter == 0 {
		if err := h.handler.Handle(ctx, r); err != nil {
			return errors.Wrap(err, errors.ErrCodeInternal, "sampling handler failed")
		}

		return nil
	}

	return nil
}

// WithAttrs returns a new handler with additional attributes
func (h *samplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &samplingHandler{
		handler:    h.handler.WithAttrs(attrs),
		counts:     h.counts, // share counts map
		initial:    h.initial,
		thereafter: h.thereafter,
	}
}

// WithGroup returns a new handler with a group name
func (h *samplingHandler) WithGroup(name string) slog.Handler {
	return &samplingHandler{
		handler:    h.handler.WithGroup(name),
		counts:     h.counts, // share counts map
		initial:    h.initial,
		thereafter: h.thereafter,
	}
}
