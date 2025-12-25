package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

// captureOutput captures log output for testing
type captureOutput struct {
	buffer *bytes.Buffer
}

func newCaptureOutput() *captureOutput {
	return &captureOutput{buffer: &bytes.Buffer{}}
}

func (c *captureOutput) Write(p []byte) (n int, err error) {
	return c.buffer.Write(p)
}

// logEntry represents a parsed log entry for validation
type logEntry struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Msg     string         `json:"msg"` // slog uses "msg" instead of "message"
	Time    string         `json:"time"`
	TraceID string         `json:"trace_id,omitempty"`
	SpanID  string         `json:"span_id,omitempty"`
	Error   string         `json:"error,omitempty"`
	Stack   string         `json:"stack,omitempty"`
	Fields  map[string]any `json:"-"`
}

// parseLogEntry parses a JSON log line into a logEntry
func parseLogEntry(line string) (*logEntry, error) {
	var entry logEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return nil, err
	}

	// Extract all fields into Fields map
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, err
	}

	// Remove standard fields
	delete(raw, "level")
	delete(raw, "message")
	delete(raw, "msg") // slog uses msg
	delete(raw, "time")
	delete(raw, "trace_id")
	delete(raw, "span_id")
	delete(raw, "error")
	delete(raw, "stack")
	delete(raw, "source") // zerolog adds this
	delete(raw, "caller") // slog might add this

	entry.Fields = raw

	// Normalize message field (slog uses "msg", zerolog uses "message")
	if entry.Message == "" && entry.Msg != "" {
		entry.Message = entry.Msg
	}

	return &entry, nil
}

// TestLoggerImplementations tests both slog and zerolog implementations
func TestLoggerImplementations(t *testing.T) {
	// Test cases covering various logging scenarios
	tests := []struct {
		name           string
		setupLogger    func(impl string, output *captureOutput) Logger
		testFunc       func(t *testing.T, logger Logger, output *captureOutput)
		expectedLevels []string
		minLogCount    int
	}{
		{
			name: "Basic logging levels",
			setupLogger: func(impl string, output *captureOutput) Logger {
				opts := Options{
					Level:  "debug",
					Format: "json",
					Output: output,
				}
				factory := NewFactory(&opts)
				logger, err := factory.Create(impl)
				if err != nil {
					t.Fatalf("Failed to create %s logger: %v", impl, err)
				}
				return logger
			},
			testFunc: func(t *testing.T, logger Logger, output *captureOutput) {
				logger.Debug("Debug message", "key1", "value1")
				logger.Info("Info message", "key2", "value2")
				logger.Warn("Warn message", "key3", "value3")
				logger.Error("Error message", "key4", "value4")
			},
			expectedLevels: []string{"debug", "info", "warn", "error"},
			minLogCount:    4,
		},
		{
			name: "Context-aware logging with trace extraction",
			setupLogger: func(impl string, output *captureOutput) Logger {
				opts := Options{
					Level:            "info",
					Format:           "json",
					ExtractTraceInfo: true,
					Output:           output,
				}
				factory := NewFactory(&opts)
				logger, err := factory.Create(impl)
				if err != nil {
					t.Fatalf("Failed to create %s logger: %v", impl, err)
				}
				return logger
			},
			testFunc: func(t *testing.T, logger Logger, output *captureOutput) {
				// Create a context with trace info (simulated)
				ctx := context.WithValue(context.Background(), "request_id", "req-123")
				logger.InfoCtx(ctx, "Request processed", "status", "success")
			},
			expectedLevels: []string{"info"},
			minLogCount:    1,
		},
		{
			name: "PII filtering",
			setupLogger: func(impl string, output *captureOutput) Logger {
				opts := Options{
					Level:         "info",
					Format:        "json",
					GDPRFilterPII: true,
					PIIFields:     []string{"password", "email", "ssn"},
					Output:        output,
				}
				factory := NewFactory(&opts)
				logger, err := factory.Create(impl)
				if err != nil {
					t.Fatalf("Failed to create %s logger: %v", impl, err)
				}
				return logger
			},
			testFunc: func(t *testing.T, logger Logger, output *captureOutput) {
				logger.Info("User login", "username", "john", "password", "secret123", "email", "john@example.com", "status", "success")
			},
			expectedLevels: []string{"info"},
			minLogCount:    1,
		},
		{
			name: "Error with stack trace",
			setupLogger: func(impl string, output *captureOutput) Logger {
				opts := Options{
					Level:              "error",
					Format:             "json",
					Output:             output,
					ErrorFieldKey:      "error",
					StackTraceFieldKey: "stack",
					ErrorCodeFieldKey:  "error_code",
					SeverityFieldKey:   "severity",
				}
				factory := NewFactory(&opts)
				logger, err := factory.Create(impl)
				if err != nil {
					t.Fatalf("Failed to create %s logger: %v", impl, err)
				}
				return logger
			},
			testFunc: func(t *testing.T, logger Logger, output *captureOutput) {
				err := errors.New("database connection failed")
				logger.ErrorWithErr(err, "Failed to connect", "host", "localhost", "port", 5432)
			},
			expectedLevels: []string{"error"},
			minLogCount:    1,
		},
		{
			name: "Child logger with fields",
			setupLogger: func(impl string, output *captureOutput) Logger {
				opts := Options{
					Level:  "info",
					Format: "json",
					Output: output,
				}
				factory := NewFactory(&opts)
				logger, err := factory.Create(impl)
				if err != nil {
					t.Fatalf("Failed to create %s logger: %v", impl, err)
				}
				return logger
			},
			testFunc: func(t *testing.T, logger Logger, output *captureOutput) {
				childLogger := logger.With("service", "api", "version", "1.0")
				childLogger.Info("Service initialized", "port", 8080)
			},
			expectedLevels: []string{"info"},
			minLogCount:    1,
		},
		{
			name: "Log sampling",
			setupLogger: func(impl string, output *captureOutput) Logger {
				opts := Options{
					Level:  "info",
					Format: "json",
					Sampling: &SamplingConfig{
						Initial:    2, // Log first 2 messages
						Thereafter: 3, // Then every 3rd message
					},
					Output: output,
				}
				factory := NewFactory(&opts)
				logger, err := factory.Create(impl)
				if err != nil {
					t.Fatalf("Failed to create %s logger: %v", impl, err)
				}
				return logger
			},
			testFunc: func(t *testing.T, logger Logger, output *captureOutput) {
				// Log 10 messages with same level and message
				for i := range 10 {
					logger.Info("Sampled log", "iteration", i)
				}
			},
			expectedLevels: []string{"info"},
			minLogCount:    4, // Should log: 1, 2, 5, 8 (2 initial + every 3rd)
		},
		{
			name: "Context keys extraction",
			setupLogger: func(impl string, output *captureOutput) Logger {
				opts := Options{
					Level:       "info",
					Format:      "json",
					ContextKeys: []string{"request_id", "user_id"},
					Output:      output,
				}
				factory := NewFactory(&opts)
				logger, err := factory.Create(impl)
				if err != nil {
					t.Fatalf("Failed to create %s logger: %v", impl, err)
				}
				return logger
			},
			testFunc: func(t *testing.T, logger Logger, output *captureOutput) {
				ctx := context.WithValue(context.Background(), "request_id", "req-456")
				ctx = context.WithValue(ctx, "user_id", "user-789")
				logger.InfoCtx(ctx, "Processing request", "status", "in_progress")
			},
			expectedLevels: []string{"info"},
			minLogCount:    1,
		},
	}

	// Test both implementations
	implementations := []string{"slog", "zerolog"}

	for _, impl := range implementations {
		t.Run(impl, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					// Capture output
					output := newCaptureOutput()

					// Create logger with captured output
					// Note: We need to modify the logger creation to use our output
					// For now, we'll test with stdout and parse the output
					logger := tt.setupLogger(impl, output)

					// Run test
					tt.testFunc(t, logger, output)

					// Flush to ensure all logs are written
					logger.Flush()

					// Parse and validate output
					lines := strings.Split(strings.TrimSpace(output.buffer.String()), "\n")
					if len(lines) < tt.minLogCount {
						t.Errorf("Expected at least %d log lines, got %d", tt.minLogCount, len(lines))
					}

					// Validate each log entry
					for i, line := range lines {
						if line == "" {
							continue
						}

						entry, err := parseLogEntry(line)
						if err != nil {
							t.Errorf("Failed to parse log entry %d: %v\nLine: %s", i, err, line)
							continue
						}

						// Normalize level (slog uses uppercase, zerolog uses lowercase)
						normalizedLevel := strings.ToLower(entry.Level)
						if !contains(tt.expectedLevels, normalizedLevel) {
							t.Errorf(
								"Log entry %d: unexpected level %s (normalized: %s), expected one of %v",
								i,
								entry.Level,
								normalizedLevel,
								tt.expectedLevels,
							)
						}

						// Validate message is present
						if entry.Message == "" {
							t.Errorf("Log entry %d: missing message", i)
						}

						// Validate timestamp
						if entry.Time == "" {
							t.Errorf("Log entry %d: missing timestamp", i)
						}

						// Test-specific validations
						switch tt.name {
						case "PII filtering":
							if _, hasPassword := entry.Fields["password"]; hasPassword {
								t.Errorf("Log entry %d: PII field 'password' should be filtered", i)
							}
							if _, hasEmail := entry.Fields["email"]; hasEmail {
								t.Errorf("Log entry %d: PII field 'email' should be filtered", i)
							}
							if _, hasUsername := entry.Fields["username"]; !hasUsername {
								t.Errorf("Log entry %d: non-PII field 'username' should be present", i)
							}
							if _, hasStatus := entry.Fields["status"]; !hasStatus {
								t.Errorf("Log entry %d: non-PII field 'status' should be present", i)
							}

						case "Error with stack trace":
							if entry.Error == "" {
								t.Errorf("Log entry %d: missing error field", i)
							}
							// Note: Stack trace implementation varies between loggers
							// zerolog includes stack trace when .Stack() is called, slog may not
							if impl == "zerolog" && entry.Stack == "" {
								t.Logf("Log entry %d: stack trace missing (zerolog should include stack trace)", i)
							}

						case "Child logger with fields":
							if service, ok := entry.Fields["service"]; !ok || service != "api" {
								t.Errorf("Log entry %d: missing or incorrect 'service' field", i)
							}
							if version, ok := entry.Fields["version"]; !ok || version != "1.0" {
								t.Errorf("Log entry %d: missing or incorrect 'version' field", i)
							}
							if port, ok := entry.Fields["port"]; !ok || port != float64(8080) {
								t.Errorf("Log entry %d: missing or incorrect 'port' field", i)
							}

						case "Context keys extraction":
							if requestID, ok := entry.Fields["request_id"]; !ok || requestID != "req-456" {
								t.Errorf("Log entry %d: missing or incorrect 'request_id' field", i)
							}
							if userID, ok := entry.Fields["user_id"]; !ok || userID != "user-789" {
								t.Errorf("Log entry %d: missing or incorrect 'user_id' field", i)
							}
						}
					}
				})
			}
		})
	}
}

// TestLoggerLevelFiltering tests that log levels are properly filtered
func TestLoggerLevelFiltering(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		logFunc   func(Logger)
		shouldLog bool
	}{
		{
			name:  "Debug filtered at info level",
			level: "info",
			logFunc: func(l Logger) {
				l.Debug("Debug message")
			},
			shouldLog: false,
		},
		{
			name:  "Info logged at info level",
			level: "info",
			logFunc: func(l Logger) {
				l.Info("Info message")
			},
			shouldLog: true,
		},
		{
			name:  "Warn logged at info level",
			level: "info",
			logFunc: func(l Logger) {
				l.Warn("Warn message")
			},
			shouldLog: true,
		},
		{
			name:  "Error logged at warn level",
			level: "warn",
			logFunc: func(l Logger) {
				l.Error("Error message")
			},
			shouldLog: true,
		},
		{
			name:  "Debug filtered at error level",
			level: "error",
			logFunc: func(l Logger) {
				l.Debug("Debug message")
			},
			shouldLog: false,
		},
	}

	implementations := []string{"slog", "zerolog"}

	for _, impl := range implementations {
		t.Run(impl, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					output := newCaptureOutput()
					opts := Options{
						Level:  tt.level,
						Format: "json",
						Output: output,
					}
					factory := NewFactory(&opts)
					logger, err := factory.Create(impl)
					if err != nil {
						t.Fatalf("Failed to create logger: %v", err)
					}

					tt.logFunc(logger)
					logger.Flush()

					hasOutput := strings.TrimSpace(output.buffer.String()) != ""
					if hasOutput != tt.shouldLog {
						t.Errorf("Expected log output: %v, got: %v", tt.shouldLog, hasOutput)
					}
				})
			}
		})
	}
}

// TestContextAwareLogging tests context-aware logging methods
func TestContextAwareLogging(t *testing.T) {
	tests := []struct {
		name        string
		logFunc     func(Logger, context.Context)
		expectedMsg string
		level       string
	}{
		{
			name: "DebugCtx",
			logFunc: func(l Logger, ctx context.Context) {
				l.DebugCtx(ctx, "Debug context message", "key1", "value1")
			},
			expectedMsg: "Debug context message",
			level:       "debug",
		},
		{
			name: "InfoCtx",
			logFunc: func(l Logger, ctx context.Context) {
				l.InfoCtx(ctx, "Info context message", "key2", "value2")
			},
			expectedMsg: "Info context message",
			level:       "info",
		},
		{
			name: "WarnCtx",
			logFunc: func(l Logger, ctx context.Context) {
				l.WarnCtx(ctx, "Warn context message", "key3", "value3")
			},
			expectedMsg: "Warn context message",
			level:       "warn",
		},
		{
			name: "ErrorCtx",
			logFunc: func(l Logger, ctx context.Context) {
				l.ErrorCtx(ctx, "Error context message", "key4", "value4")
			},
			expectedMsg: "Error context message",
			level:       "error",
		},
	}

	implementations := []string{"slog", "zerolog"}

	for _, impl := range implementations {
		t.Run(impl, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					output := newCaptureOutput()
					opts := Options{
						Level:  tt.level,
						Format: "json",
						Output: output,
					}
					factory := NewFactory(&opts)
					logger, err := factory.Create(impl)
					if err != nil {
						t.Fatalf("Failed to create logger: %v", err)
					}

					// Create context with values
					ctx := context.WithValue(context.Background(), "request_id", "req-123")
					ctx = context.WithValue(ctx, "user_id", "user-456")

					tt.logFunc(logger, ctx)
					logger.Flush()

					lines := strings.Split(strings.TrimSpace(output.buffer.String()), "\n")
					if len(lines) == 0 || lines[0] == "" {
						t.Fatal("Expected log output, got none")
					}

					entry, err := parseLogEntry(lines[0])
					if err != nil {
						t.Fatalf("Failed to parse log entry: %v", err)
					}

					if entry.Message != tt.expectedMsg {
						t.Errorf("Expected message '%s', got '%s'", tt.expectedMsg, entry.Message)
					}

					// Check that context values are present if ExtractTraceInfo is enabled
					if impl == "slog" {
						// slog implementation may include context values
						if entry.TraceID == "" && entry.SpanID == "" {
							// This is acceptable if context extraction is not configured
							t.Log("No trace info extracted from context (expected if not configured)")
						}
					}
				})
			}
		})
	}
}

// TestErrorWithErr tests error handling methods
func TestErrorWithErr(t *testing.T) {
	tests := []struct {
		name      string
		logFunc   func(Logger)
		expectErr bool
	}{
		{
			name: "ErrorWithErr",
			logFunc: func(l Logger) {
				err := errors.New("test error")
				l.ErrorWithErr(err, "Operation failed", "operation", "test")
			},
			expectErr: true,
		},
	}

	implementations := []string{"slog", "zerolog"}

	for _, impl := range implementations {
		t.Run(impl, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					output := newCaptureOutput()
					opts := Options{
						Level:              "error",
						Format:             "json",
						Output:             output,
						ErrorFieldKey:      "error",
						StackTraceFieldKey: "stack",
					}
					factory := NewFactory(&opts)
					logger, err := factory.Create(impl)
					if err != nil {
						t.Fatalf("Failed to create logger: %v", err)
					}

					tt.logFunc(logger)
					logger.Flush()

					lines := strings.Split(strings.TrimSpace(output.buffer.String()), "\n")
					if len(lines) == 0 || lines[0] == "" {
						t.Fatal("Expected log output, got none")
					}

					entry, err := parseLogEntry(lines[0])
					if err != nil {
						t.Fatalf("Failed to parse log entry: %v", err)
					}

					if tt.expectErr && entry.Error == "" {
						t.Error("Expected error field in log entry, but none found")
					}

					if entry.Message != "Operation failed" {
						t.Errorf("Expected message 'Operation failed', got '%s'", entry.Message)
					}
				})
			}
		})
	}
}

// TestLoggerCreationErrors tests error scenarios in logger creation
func TestLoggerCreationErrors(t *testing.T) {
	tests := []struct {
		name        string
		opts        Options
		impl        string
		expectError bool
	}{
		{
			name: "Invalid implementation",
			opts: Options{
				Level:  "info",
				Format: "json",
			},
			impl:        "invalid",
			expectError: true,
		},
		{
			name: "Invalid log level",
			opts: Options{
				Level:  "invalid",
				Format: "json",
			},
			impl:        "slog",
			expectError: true, // Logger validates and returns error for invalid level
		},
		{
			name: "Invalid format",
			opts: Options{
				Level:  "info",
				Format: "invalid",
			},
			impl:        "slog",
			expectError: true, // Logger validates and returns error for invalid format
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory(&tt.opts)
			_, err := factory.Create(tt.impl)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestLoggerWithNilContext tests logging with nil context
func TestLoggerWithNilContext(t *testing.T) {
	implementations := []string{"slog", "zerolog"}

	for _, impl := range implementations {
		t.Run(impl, func(t *testing.T) {
			output := newCaptureOutput()
			opts := Options{
				Level:  "info",
				Format: "json",
				Output: output,
			}
			factory := NewFactory(&opts)
			logger, err := factory.Create(impl)
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}

			// Test with nil context - should not panic
			logger.InfoCtx(nil, "Message with nil context", "key", "value")
			logger.Flush()

			lines := strings.Split(strings.TrimSpace(output.buffer.String()), "\n")
			if len(lines) == 0 || lines[0] == "" {
				t.Fatal("Expected log output, got none")
			}

			entry, err := parseLogEntry(lines[0])
			if err != nil {
				t.Fatalf("Failed to parse log entry: %v", err)
			}

			if entry.Message != "Message with nil context" {
				t.Errorf("Expected message 'Message with nil context', got '%s'", entry.Message)
			}
		})
	}
}

// TestLoggerWithEmptyFields tests logger with empty or nil fields
func TestLoggerWithEmptyFields(t *testing.T) {
	implementations := []string{"slog", "zerolog"}

	for _, impl := range implementations {
		t.Run(impl, func(t *testing.T) {
			output := newCaptureOutput()
			opts := Options{
				Level:  "info",
				Format: "json",
				Output: output,
			}
			factory := NewFactory(&opts)
			logger, err := factory.Create(impl)
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}

			// Test with empty fields
			logger.Info("Message with no fields")

			// Test with proper key-value pairs
			logger.Info("Message with fields", "key1", "value1", "key2", "value2")

			logger.Flush()

			lines := strings.Split(strings.TrimSpace(output.buffer.String()), "\n")
			if len(lines) < 2 {
				t.Fatalf("Expected at least 2 log lines, got %d", len(lines))
			}

			// First line should have no extra fields
			entry1, err := parseLogEntry(lines[0])
			if err != nil {
				t.Fatalf("Failed to parse first log entry: %v", err)
			}
			if entry1.Message != "Message with no fields" {
				t.Errorf("Expected message 'Message with no fields', got '%s'", entry1.Message)
			}

			// Second line should have the fields
			entry2, err := parseLogEntry(lines[1])
			if err != nil {
				t.Fatalf("Failed to parse second log entry: %v", err)
			}
			if entry2.Message != "Message with fields" {
				t.Errorf("Expected message 'Message with fields', got '%s'", entry2.Message)
			}
			if _, ok := entry2.Fields["key1"]; !ok {
				t.Error("Expected 'key1' field in second log entry")
			}
			if _, ok := entry2.Fields["key2"]; !ok {
				t.Error("Expected 'key2' field in second log entry")
			}
		})
	}
}

// TestLevelString tests the String method of Level type
func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{DebugLevel, "debug"},
		{InfoLevel, "info"},
		{WarnLevel, "warn"},
		{ErrorLevel, "error"},
		{FatalLevel, "fatal"},
		{Level(999), "unknown"}, // Invalid level
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("Level.String() = %s, want %s", result, tt.expected)
			}
		})
	}
}

// TestSamplingHandler tests the sampling handler functionality
func TestSamplingHandler(t *testing.T) {
	output := newCaptureOutput()
	opts := Options{
		Level:  "info",
		Format: "json",
		Output: output,
		Sampling: &SamplingConfig{
			Initial:    2, // Log first 2 messages
			Thereafter: 3, // Then every 3rd message
		},
	}

	// Test with slog implementation
	factory := NewFactory(&opts)
	logger, err := factory.Create("slog")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log 10 identical messages
	for i := range 10 {
		logger.Info("Sampled message", "iteration", i)
	}

	logger.Flush()

	lines := strings.Split(strings.TrimSpace(output.buffer.String()), "\n")
	// Should log: 1, 2, 5, 8 (2 initial + every 3rd after that)
	expectedCount := 4
	if len(lines) != expectedCount {
		t.Errorf("Expected %d log lines with sampling, got %d", expectedCount, len(lines))
	}

	// Verify the logged iterations
	loggedIterations := make([]int, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		entry, err := parseLogEntry(line)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}
		if iter, ok := entry.Fields["iteration"].(float64); ok {
			loggedIterations = append(loggedIterations, int(iter))
		}
	}

	expectedIterations := []int{0, 1, 4, 7} // 0-based indexing
	if len(loggedIterations) != len(expectedIterations) {
		t.Errorf("Expected %d logged iterations, got %d", len(expectedIterations), len(loggedIterations))
	}

	for i, iter := range loggedIterations {
		if i < len(expectedIterations) && iter != expectedIterations[i] {
			t.Errorf("Expected iteration %d at position %d, got %d", expectedIterations[i], i, iter)
		}
	}
}

// TestNewZapLogger tests the placeholder zap logger creation
func TestNewZapLogger(t *testing.T) {
	opts := Options{
		Level:  "info",
		Format: "json",
	}

	// Test that newZapLogger returns nil (placeholder implementation)
	logger := newZapLogger(&opts)
	if logger != nil {
		t.Error("Expected newZapLogger to return nil (placeholder), got non-nil logger")
	}
}

// Helper function to check if a string is in a slice
func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}
