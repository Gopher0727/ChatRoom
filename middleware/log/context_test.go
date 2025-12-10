package logger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithTraceIDContext(t *testing.T) {
	t.Run("adds provided trace ID to context", func(t *testing.T) {
		ctx := context.Background()
		traceID := "test-trace-123"

		newCtx := WithTraceID(ctx, traceID)
		require.NotNil(t, newCtx)

		extractedTraceID := GetTraceID(newCtx)
		assert.Equal(t, traceID, extractedTraceID)
	})

	t.Run("generates new trace ID when empty string provided", func(t *testing.T) {
		ctx := context.Background()

		newCtx := WithTraceID(ctx, "")
		require.NotNil(t, newCtx)

		extractedTraceID := GetTraceID(newCtx)
		assert.NotEmpty(t, extractedTraceID)
		// Verify it's a valid UUID format (36 characters with hyphens)
		assert.Len(t, extractedTraceID, 36)
	})

	t.Run("preserves other context values", func(t *testing.T) {
		type testKey string
		key := testKey("test-key")
		value := "test-value"

		ctx := context.WithValue(context.Background(), key, value)
		traceID := "trace-456"

		newCtx := WithTraceID(ctx, traceID)
		require.NotNil(t, newCtx)

		// Verify trace ID is present
		assert.Equal(t, traceID, GetTraceID(newCtx))

		// Verify original value is preserved
		extractedValue, ok := newCtx.Value(key).(string)
		require.True(t, ok)
		assert.Equal(t, value, extractedValue)
	})
}

func TestGetTraceID(t *testing.T) {
	t.Run("returns trace ID from context", func(t *testing.T) {
		traceID := "test-trace-789"
		ctx := context.WithValue(context.Background(), TraceIDKey, traceID)

		extractedTraceID := GetTraceID(ctx)
		assert.Equal(t, traceID, extractedTraceID)
	})

	t.Run("returns empty string when no trace ID in context", func(t *testing.T) {
		ctx := context.Background()

		extractedTraceID := GetTraceID(ctx)
		assert.Empty(t, extractedTraceID)
	})

	t.Run("returns empty string when trace ID is wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), TraceIDKey, 12345)

		extractedTraceID := GetTraceID(ctx)
		assert.Empty(t, extractedTraceID)
	})
}

func TestNewTraceID(t *testing.T) {
	t.Run("generates valid UUID", func(t *testing.T) {
		traceID := NewTraceID()

		assert.NotEmpty(t, traceID)
		// UUID v4 format: 8-4-4-4-12 characters
		assert.Len(t, traceID, 36)
		assert.Contains(t, traceID, "-")
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		traceID1 := NewTraceID()
		traceID2 := NewTraceID()

		assert.NotEqual(t, traceID1, traceID2)
	})

	t.Run("generates multiple unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)
		count := 100

		for i := 0; i < count; i++ {
			id := NewTraceID()
			assert.NotEmpty(t, id)
			assert.False(t, ids[id], "duplicate ID generated: %s", id)
			ids[id] = true
		}

		assert.Len(t, ids, count)
	})
}

func TestTraceIDPropagation(t *testing.T) {
	t.Run("trace ID propagates through context chain", func(t *testing.T) {
		// Create initial context with trace ID
		traceID := "propagation-test-123"
		ctx1 := WithTraceID(context.Background(), traceID)

		// Create child context
		type testKey string
		ctx2 := context.WithValue(ctx1, testKey("key"), "value")

		// Verify trace ID is still accessible
		extractedTraceID := GetTraceID(ctx2)
		assert.Equal(t, traceID, extractedTraceID)
	})

	t.Run("can override trace ID in child context", func(t *testing.T) {
		// Create initial context with trace ID
		traceID1 := "trace-1"
		ctx1 := WithTraceID(context.Background(), traceID1)

		// Override with new trace ID
		traceID2 := "trace-2"
		ctx2 := WithTraceID(ctx1, traceID2)

		// Verify new trace ID
		extractedTraceID := GetTraceID(ctx2)
		assert.Equal(t, traceID2, extractedTraceID)

		// Verify original context still has old trace ID
		originalTraceID := GetTraceID(ctx1)
		assert.Equal(t, traceID1, originalTraceID)
	})
}
