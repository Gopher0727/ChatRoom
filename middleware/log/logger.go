package logger

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/Gopher0727/ChatRoom/config"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// TraceIDKey is the context key for trace ID
	TraceIDKey contextKey = "trace_id"
)

// Logger wraps zap.Logger with additional functionality
type Logger struct {
	*zap.Logger
	file *os.File // Keep reference to file for proper cleanup
}

// NewLogger creates a new logger instance based on the provided configuration.
// It supports different log levels (DEBUG, INFO, WARN, ERROR, FATAL),
// formats (JSON, text), and outputs (stdout, file).
//
// Parameters:
//   - cfg: LoggingConfig containing level, format, output, and file path settings
//
// Returns:
//   - *Logger: A configured logger instance
//   - error: Any error encountered during logger creation
func NewLogger(cfg *config.LoggingConfig) (*Logger, error) {
	// Parse log level
	level, err := parseLogLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create encoder based on format
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Create writer sync based on output
	var writeSyncer zapcore.WriteSyncer
	var file *os.File
	if cfg.Output == "file" {
		var err error
		file, err = os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		writeSyncer = zapcore.AddSync(file)
	} else {
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	// Create core
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// Create logger with caller and stacktrace
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &Logger{Logger: zapLogger, file: file}, nil
}

// NewDevelopmentLogger creates a logger suitable for development.
// It uses console encoding and debug level by default.
func NewDevelopmentLogger() (*Logger, error) {
	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	return &Logger{Logger: zapLogger}, nil
}

// NewProductionLogger creates a logger suitable for production.
// It uses JSON encoding and info level by default.
func NewProductionLogger() (*Logger, error) {
	zapLogger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return &Logger{Logger: zapLogger}, nil
}

// WithTraceID returns a new logger with the trace ID field added.
// This is useful for distributed tracing and correlating logs across services.
//
// Parameters:
//   - traceID: The trace ID to add to all log entries
//
// Returns:
//   - *Logger: A new logger instance with the trace ID field
func (l *Logger) WithTraceID(traceID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(zap.String("trace_id", traceID)),
	}
}

// WithContext extracts the trace ID from the context and returns a logger with it.
// If no trace ID is found in the context, returns the original logger.
//
// Parameters:
//   - ctx: The context potentially containing a trace ID
//
// Returns:
//   - *Logger: A logger instance with trace ID if found, otherwise the original logger
func (l *Logger) WithContext(ctx context.Context) *Logger {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		return l.WithTraceID(traceID)
	}
	return l
}

// WithFields returns a new logger with the provided fields added.
// This is useful for adding structured context to log entries.
//
// Parameters:
//   - fields: Variable number of zap.Field to add to the logger
//
// Returns:
//   - *Logger: A new logger instance with the additional fields
func (l *Logger) WithFields(fields ...zap.Field) *Logger {
	return &Logger{
		Logger: l.Logger.With(fields...),
	}
}

// DebugContext logs a debug message with trace ID from context if available
func (l *Logger) DebugContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.WithContext(ctx).Debug(msg, fields...)
}

// InfoContext logs an info message with trace ID from context if available
func (l *Logger) InfoContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.WithContext(ctx).Info(msg, fields...)
}

// WarnContext logs a warning message with trace ID from context if available
func (l *Logger) WarnContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.WithContext(ctx).Warn(msg, fields...)
}

// ErrorContext logs an error message with trace ID from context if available
func (l *Logger) ErrorContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.WithContext(ctx).Error(msg, fields...)
}

// FatalContext logs a fatal message with trace ID from context if available and exits
func (l *Logger) FatalContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.WithContext(ctx).Fatal(msg, fields...)
}

// parseLogLevel converts a string log level to zapcore.Level
func parseLogLevel(level string) (zapcore.Level, error) {
	switch level {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, nil
	}
}

// Sync flushes any buffered log entries.
// Applications should call Sync before exiting.
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// Close closes the logger and any associated file handles.
// This should be called when the logger is no longer needed.
func (l *Logger) Close() error {
	if err := l.Sync(); err != nil {
		return err
	}
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
