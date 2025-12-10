package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Gopher0727/ChatRoom/config"
)

func TestNewLogger(t *testing.T) {
	t.Run("creates logger with JSON format and stdout output", func(t *testing.T) {
		cfg := &config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		}

		logger, err := NewLogger(cfg)
		require.NoError(t, err)
		require.NotNil(t, logger)

		// Verify logger can log without errors
		logger.Info("test message")
		err = logger.Sync()
		assert.NoError(t, err)
	})

	t.Run("creates logger with text format", func(t *testing.T) {
		cfg := &config.LoggingConfig{
			Level:  "debug",
			Format: "text",
			Output: "stdout",
		}

		logger, err := NewLogger(cfg)
		require.NoError(t, err)
		require.NotNil(t, logger)

		logger.Debug("test debug message")
		err = logger.Sync()
		assert.NoError(t, err)
	})

	t.Run("creates logger with file output", func(t *testing.T) {
		// Create temporary directory for test logs
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test.log")

		cfg := &config.LoggingConfig{
			Level:    "info",
			Format:   "json",
			Output:   "file",
			FilePath: logFile,
		}

		logger, err := NewLogger(cfg)
		require.NoError(t, err)
		require.NotNil(t, logger)

		// Log a message
		logger.Info("test file message")

		// Close logger to release file handle
		err = logger.Close()
		require.NoError(t, err)

		// Verify file was created and contains the message
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "test file message")
	})

	t.Run("handles different log levels", func(t *testing.T) {
		levels := []string{"debug", "info", "warn", "error"}

		for _, level := range levels {
			cfg := &config.LoggingConfig{
				Level:  level,
				Format: "json",
				Output: "stdout",
			}

			logger, err := NewLogger(cfg)
			require.NoError(t, err, "failed to create logger for level: %s", level)
			require.NotNil(t, logger)

			// Test that logger works
			logger.Info("test message for level: " + level)
			err = logger.Sync()
			assert.NoError(t, err)
		}
	})

	t.Run("defaults to info level for invalid level", func(t *testing.T) {
		cfg := &config.LoggingConfig{
			Level:  "invalid",
			Format: "json",
			Output: "stdout",
		}

		logger, err := NewLogger(cfg)
		require.NoError(t, err)
		require.NotNil(t, logger)
	})
}

func TestNewDevelopmentLogger(t *testing.T) {
	logger, err := NewDevelopmentLogger()
	require.NoError(t, err)
	require.NotNil(t, logger)

	logger.Debug("development debug message")
	logger.Info("development info message")
	err = logger.Sync()
	assert.NoError(t, err)
}

func TestNewProductionLogger(t *testing.T) {
	logger, err := NewProductionLogger()
	require.NoError(t, err)
	require.NotNil(t, logger)

	logger.Info("production info message")
	logger.Error("production error message")
	err = logger.Sync()
	assert.NoError(t, err)
}

func TestWithTraceID(t *testing.T) {
	logger, err := NewDevelopmentLogger()
	require.NoError(t, err)

	traceID := "test-trace-123"
	loggerWithTrace := logger.WithTraceID(traceID)

	require.NotNil(t, loggerWithTrace)
	// Verify it's a different logger instance
	assert.NotEqual(t, logger, loggerWithTrace)

	// Log with trace ID
	loggerWithTrace.Info("message with trace ID")
	err = loggerWithTrace.Sync()
	assert.NoError(t, err)
}

func TestWithContext(t *testing.T) {
	logger, err := NewDevelopmentLogger()
	require.NoError(t, err)

	t.Run("extracts trace ID from context", func(t *testing.T) {
		traceID := "context-trace-456"
		ctx := context.WithValue(context.Background(), TraceIDKey, traceID)

		loggerWithContext := logger.WithContext(ctx)
		require.NotNil(t, loggerWithContext)

		loggerWithContext.Info("message with context trace ID")
		err = loggerWithContext.Sync()
		assert.NoError(t, err)
	})

	t.Run("returns original logger when no trace ID in context", func(t *testing.T) {
		ctx := context.Background()

		loggerWithContext := logger.WithContext(ctx)
		require.NotNil(t, loggerWithContext)

		loggerWithContext.Info("message without trace ID")
		err = loggerWithContext.Sync()
		assert.NoError(t, err)
	})
}

func TestWithFields(t *testing.T) {
	logger, err := NewDevelopmentLogger()
	require.NoError(t, err)

	fields := []zap.Field{
		zap.String("user_id", "user123"),
		zap.String("guild_id", "guild456"),
		zap.Int("count", 42),
	}

	loggerWithFields := logger.WithFields(fields...)
	require.NotNil(t, loggerWithFields)

	loggerWithFields.Info("message with fields")
	err = loggerWithFields.Sync()
	assert.NoError(t, err)
}

func TestContextLoggingMethods(t *testing.T) {
	logger, err := NewDevelopmentLogger()
	require.NoError(t, err)

	traceID := "test-trace-789"
	ctx := context.WithValue(context.Background(), TraceIDKey, traceID)

	t.Run("DebugContext", func(t *testing.T) {
		logger.DebugContext(ctx, "debug message with context",
			zap.String("key", "value"))
	})

	t.Run("InfoContext", func(t *testing.T) {
		logger.InfoContext(ctx, "info message with context",
			zap.String("key", "value"))
	})

	t.Run("WarnContext", func(t *testing.T) {
		logger.WarnContext(ctx, "warn message with context",
			zap.String("key", "value"))
	})

	t.Run("ErrorContext", func(t *testing.T) {
		logger.ErrorContext(ctx, "error message with context",
			zap.String("key", "value"))
	})

	err = logger.Sync()
	assert.NoError(t, err)
}

func TestLogLevels(t *testing.T) {
	// Create a temporary file to capture logs
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "level_test.log")

	cfg := &config.LoggingConfig{
		Level:    "warn",
		Format:   "json",
		Output:   "file",
		FilePath: logFile,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	// Log at different levels
	logger.Debug("debug message - should not appear")
	logger.Info("info message - should not appear")
	logger.Warn("warn message - should appear")
	logger.Error("error message - should appear")

	// Close logger to release file handle
	err = logger.Close()
	require.NoError(t, err)

	// Read log file
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	logContent := string(content)

	// Verify only warn and error messages appear
	assert.NotContains(t, logContent, "debug message")
	assert.NotContains(t, logContent, "info message")
	assert.Contains(t, logContent, "warn message")
	assert.Contains(t, logContent, "error message")
}

func TestJSONFormat(t *testing.T) {
	// Create a buffer to capture logs
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "json_test.log")

	cfg := &config.LoggingConfig{
		Level:    "info",
		Format:   "json",
		Output:   "file",
		FilePath: logFile,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	// Log a structured message
	logger.Info("test json message",
		zap.String("user_id", "user123"),
		zap.Int("count", 42),
		zap.Bool("active", true),
	)

	// Close logger to release file handle
	err = logger.Close()
	require.NoError(t, err)

	// Read and parse JSON log
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Parse JSON
	var logEntry map[string]any
	err = json.Unmarshal(bytes.TrimSpace(content), &logEntry)
	require.NoError(t, err)

	// Verify JSON structure
	assert.Equal(t, "info", logEntry["level"])
	assert.Equal(t, "test json message", logEntry["message"])
	assert.Equal(t, "user123", logEntry["user_id"])
	assert.Equal(t, float64(42), logEntry["count"])
	assert.Equal(t, true, logEntry["active"])
	assert.NotEmpty(t, logEntry["timestamp"])
}

func TestTraceIDInLogs(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "trace_test.log")

	cfg := &config.LoggingConfig{
		Level:    "info",
		Format:   "json",
		Output:   "file",
		FilePath: logFile,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	traceID := "trace-abc-123"
	ctx := context.WithValue(context.Background(), TraceIDKey, traceID)

	logger.InfoContext(ctx, "message with trace ID")

	// Close logger to release file handle
	err = logger.Close()
	require.NoError(t, err)

	// Read and verify trace ID in log
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var logEntry map[string]any
	err = json.Unmarshal(bytes.TrimSpace(content), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, traceID, logEntry["trace_id"])
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"debug", "debug"},
		{"info", "info"},
		{"warn", "warn"},
		{"error", "error"},
		{"fatal", "fatal"},
		{"invalid", "info"}, // defaults to info
		{"", "info"},        // defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := parseLogLevel(tt.input)
			require.NoError(t, err)

			expectedLevel, _ := parseLogLevel(tt.expected)
			assert.Equal(t, expectedLevel, level)
		})
	}
}

func TestLoggerClose(t *testing.T) {
	t.Run("closes file logger properly", func(t *testing.T) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "close_test.log")

		cfg := &config.LoggingConfig{
			Level:    "info",
			Format:   "json",
			Output:   "file",
			FilePath: logFile,
		}

		logger, err := NewLogger(cfg)
		require.NoError(t, err)

		logger.Info("test message before close")

		// Close logger
		err = logger.Close()
		assert.NoError(t, err)

		// Verify file exists and contains message
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "test message before close")
	})

	t.Run("closes stdout logger without error", func(t *testing.T) {
		cfg := &config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		}

		logger, err := NewLogger(cfg)
		require.NoError(t, err)

		logger.Info("test message")

		// Close should not error even for stdout
		err = logger.Close()
		assert.NoError(t, err)
	})
}

func TestMultipleFieldsAndTraceID(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "multi_fields_test.log")

	cfg := &config.LoggingConfig{
		Level:    "info",
		Format:   "json",
		Output:   "file",
		FilePath: logFile,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	// Add trace ID and multiple fields
	traceID := "trace-multi-123"
	ctx := context.WithValue(context.Background(), TraceIDKey, traceID)

	logger.InfoContext(ctx, "complex log entry",
		zap.String("user_id", "user456"),
		zap.String("guild_id", "guild789"),
		zap.Int("message_count", 10),
		zap.Bool("is_admin", true),
		zap.Float64("latency_ms", 45.67),
	)

	err = logger.Close()
	require.NoError(t, err)

	// Read and verify all fields
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var logEntry map[string]any
	err = json.Unmarshal(bytes.TrimSpace(content), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "info", logEntry["level"])
	assert.Equal(t, "complex log entry", logEntry["message"])
	assert.Equal(t, traceID, logEntry["trace_id"])
	assert.Equal(t, "user456", logEntry["user_id"])
	assert.Equal(t, "guild789", logEntry["guild_id"])
	assert.Equal(t, float64(10), logEntry["message_count"])
	assert.Equal(t, true, logEntry["is_admin"])
	assert.Equal(t, 45.67, logEntry["latency_ms"])
}

func TestLoggerWithFieldsChaining(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "chaining_test.log")

	cfg := &config.LoggingConfig{
		Level:    "info",
		Format:   "json",
		Output:   "file",
		FilePath: logFile,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	// Chain multiple WithFields calls
	logger1 := logger.WithFields(zap.String("service", "gateway"))
	logger2 := logger1.WithFields(zap.String("component", "websocket"))
	logger3 := logger2.WithTraceID("trace-chain-456")

	logger3.Info("chained logger message")

	err = logger.Close()
	require.NoError(t, err)

	// Read and verify all fields are present
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var logEntry map[string]any
	err = json.Unmarshal(bytes.TrimSpace(content), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "gateway", logEntry["service"])
	assert.Equal(t, "websocket", logEntry["component"])
	assert.Equal(t, "trace-chain-456", logEntry["trace_id"])
}

func TestConsoleFormatOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "console_test.log")

	cfg := &config.LoggingConfig{
		Level:    "info",
		Format:   "text",
		Output:   "file",
		FilePath: logFile,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	logger.Info("console format message",
		zap.String("key1", "value1"),
		zap.Int("key2", 42),
	)

	err = logger.Close()
	require.NoError(t, err)

	// Read and verify console format (not JSON)
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	logContent := string(content)
	assert.Contains(t, logContent, "console format message")
	assert.Contains(t, logContent, "key1")
	assert.Contains(t, logContent, "value1")
	assert.Contains(t, logContent, "key2")

	// Verify it's NOT JSON format
	var jsonTest map[string]any
	err = json.Unmarshal(bytes.TrimSpace(content), &jsonTest)
	assert.Error(t, err, "console format should not be valid JSON")
}

func TestLogLevelFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "filtering_test.log")

	cfg := &config.LoggingConfig{
		Level:    "error",
		Format:   "json",
		Output:   "file",
		FilePath: logFile,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	// Log at different levels
	logger.Debug("debug - should not appear")
	logger.Info("info - should not appear")
	logger.Warn("warn - should not appear")
	logger.Error("error - should appear")

	err = logger.Close()
	require.NoError(t, err)

	// Read log file
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	logContent := string(content)

	// Verify only error level appears
	assert.NotContains(t, logContent, "debug - should not appear")
	assert.NotContains(t, logContent, "info - should not appear")
	assert.NotContains(t, logContent, "warn - should not appear")
	assert.Contains(t, logContent, "error - should appear")
}

func TestContextLoggingWithoutTraceID(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "no_trace_test.log")

	cfg := &config.LoggingConfig{
		Level:    "info",
		Format:   "json",
		Output:   "file",
		FilePath: logFile,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	// Use context without trace ID
	ctx := context.Background()
	logger.InfoContext(ctx, "message without trace ID",
		zap.String("user_id", "user123"),
	)

	err = logger.Close()
	require.NoError(t, err)

	// Read and verify no trace_id field
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var logEntry map[string]any
	err = json.Unmarshal(bytes.TrimSpace(content), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "message without trace ID", logEntry["message"])
	assert.Equal(t, "user123", logEntry["user_id"])
	// trace_id should not be present
	_, hasTraceID := logEntry["trace_id"]
	assert.False(t, hasTraceID, "trace_id should not be present when not in context")
}
