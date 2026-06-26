package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kannancmohan/go-prototype-authz-cerbos/pkg/logger"
)

func main() {
	// Create logger factory with default configuration
	options := logger.DefaultOptions()
	factory := logger.NewFactory(&options)

	// Create a zerolog logger instance
	log, err := factory.Create("slog")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	// Ensure logs are flushed when main exits (even on panic or early return)
	defer func() {
		if err := log.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush logs: %v\n", err)
		}
	}()

	// Create a context for the application startup
	ctx := context.Background()

	// Log application startup with structured logging
	log.InfoCtx(ctx, "Authorization service starting",
		"service", "authz-cerbos",
		"version", "1.0.0",
		"environment", "development")

	//nolint:gocritic // Keep as demo examples
	// // Example of using the logger for different log levels
	// log.Debug("Debug information", "debug_flag", true)
	// log.Warn("This is a warning message", "component", "main")

	//nolint:gocritic // Keep as demo examples
	// // Example of creating a child logger with additional fields
	// appLog := log.With("service_type", "authorization", "framework", "cerbos")
	// appLog.Info("Service initialization complete", "status", "ready")

	fmt.Println("Authorization service started successfully!")
}
