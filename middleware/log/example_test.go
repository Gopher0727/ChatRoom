package logger_test

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/Gopher0727/ChatRoom/config"
	logger "github.com/Gopher0727/ChatRoom/middleware/log"
)

// Example_basicUsage demonstrates basic logger usage
func Example_basicUsage() {
	// Create logger from configuration
	cfg := &config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}

	log, err := logger.NewLogger(cfg)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	// Log messages at different levels
	log.Debug("This is a debug message")
	log.Info("Application started")
	log.Warn("This is a warning")
	log.Error("An error occurred", zap.Error(fmt.Errorf("example error")))
}

// Example_withTraceID demonstrates trace ID usage
func Example_withTraceID() {
	log, _ := logger.NewDevelopmentLogger()
	defer log.Sync()

	// Generate a new trace ID
	traceID := logger.NewTraceID()

	// Create logger with trace ID
	logWithTrace := log.WithTraceID(traceID)
	logWithTrace.Info("Processing request")
	logWithTrace.Info("Request completed")
}

// Example_contextAware demonstrates context-aware logging
func Example_contextAware() {
	log, _ := logger.NewDevelopmentLogger()
	defer log.Sync()

	// Create context with trace ID
	ctx := logger.WithTraceID(context.Background(), "trace-123")

	// Log with context - trace ID is automatically included
	log.InfoContext(ctx, "User logged in",
		zap.String("user_id", "user123"),
		zap.String("ip", "192.168.1.1"))

	log.InfoContext(ctx, "User action performed",
		zap.String("action", "create_guild"))
}

// Example_structuredFields demonstrates structured logging
func Example_structuredFields() {
	log, _ := logger.NewDevelopmentLogger()
	defer log.Sync()

	// Log with structured fields
	log.Info("Message sent",
		zap.String("message_id", "msg123"),
		zap.String("user_id", "user456"),
		zap.String("guild_id", "guild789"),
		zap.Int64("seq_id", 12345),
		zap.Duration("latency", 50))
}

// Example_persistentFields demonstrates creating a logger with persistent fields
func Example_persistentFields() {
	log, _ := logger.NewDevelopmentLogger()
	defer log.Sync()

	// Create a logger with persistent fields for a specific user
	userLog := log.WithFields(
		zap.String("user_id", "user123"),
		zap.String("session_id", "session456"))

	// All subsequent logs will include these fields
	userLog.Info("User action: login")
	userLog.Info("User action: create guild")
	userLog.Info("User action: send message")
}

// Example_httpMiddleware demonstrates logger usage in HTTP middleware
func Example_httpMiddleware() {
	log, _ := logger.NewDevelopmentLogger()
	defer log.Sync()

	// Simulate HTTP request handling
	ctx := context.Background()

	// Generate trace ID for the request
	traceID := logger.NewTraceID()
	ctx = logger.WithTraceID(ctx, traceID)

	// Log request start
	log.InfoContext(ctx, "Request received",
		zap.String("method", "POST"),
		zap.String("path", "/api/v1/messages"))

	// Process request...

	// Log request completion
	log.InfoContext(ctx, "Request completed",
		zap.Int("status", 200),
		zap.Duration("latency", 45))
}

// Example_errorHandling demonstrates error logging
func Example_errorHandling() {
	log, _ := logger.NewDevelopmentLogger()
	defer log.Sync()

	ctx := logger.WithTraceID(context.Background(), "trace-456")

	// Simulate an error
	err := fmt.Errorf("database connection failed")

	// Log error with context
	log.ErrorContext(ctx, "Failed to save message",
		zap.Error(err),
		zap.String("user_id", "user123"),
		zap.String("guild_id", "guild456"),
		zap.String("operation", "insert"))
}

// Example_serviceLayer demonstrates logger usage in service layer
func Example_serviceLayer() {
	log, _ := logger.NewDevelopmentLogger()
	defer log.Sync()

	// Create a service-specific logger
	authLog := log.WithFields(zap.String("service", "auth"))

	ctx := logger.WithTraceID(context.Background(), "trace-789")

	// Log service operations
	authLog.InfoContext(ctx, "User registration started",
		zap.String("username", "newuser"))

	// Simulate validation
	authLog.DebugContext(ctx, "Validating user input")

	// Simulate success
	authLog.InfoContext(ctx, "User registered successfully",
		zap.String("user_id", "user123"))
}
