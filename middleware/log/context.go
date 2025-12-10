package logger

import (
	"context"

	"github.com/google/uuid"
)

// WithTraceID adds a trace ID to the context.
// If no trace ID is provided, a new UUID is generated.
//
// Parameters:
//   - ctx: The parent context
//   - traceID: The trace ID to add (optional, will generate if empty)
//
// Returns:
//   - context.Context: A new context with the trace ID
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		traceID = uuid.New().String()
	}
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// GetTraceID extracts the trace ID from the context.
// Returns an empty string if no trace ID is found.
//
// Parameters:
//   - ctx: The context to extract the trace ID from
//
// Returns:
//   - string: The trace ID if found, empty string otherwise
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// NewTraceID generates a new trace ID using UUID v4.
//
// Returns:
//   - string: A new UUID trace ID
func NewTraceID() string {
	return uuid.New().String()
}
