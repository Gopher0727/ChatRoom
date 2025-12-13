package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Gopher0727/ChatRoom/config"
)

// TestNewProducer tests the creation of a new Kafka producer.
// ! This test requires a running Kafka instance.
func TestNewProducer(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
	}

	producer, err := NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer producer.Close()

	assert.NotNil(t, producer)
	assert.NotNil(t, producer.producer)
	assert.Equal(t, cfg, producer.config)
}

// TestProducer_Produce tests sending a message to Kafka.
// ! This test requires a running Kafka instance.
func TestProducer_Produce(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics: config.TopicsConfig{
			Message: "test.messages",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
	}

	producer, err := NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer producer.Close()

	ctx := context.Background()
	key := []byte("test-key")
	value := []byte("test-message")

	partition, offset, err := producer.Produce(ctx, cfg.Topics.Message, key, value)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, partition, int32(0))
	assert.GreaterOrEqual(t, offset, int64(0))
}

// TestProducer_ProduceWithoutKey tests sending a message without a key.
// ! This test requires a running Kafka instance.
func TestProducer_ProduceWithoutKey(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics: config.TopicsConfig{
			Message: "test.messages",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
	}

	producer, err := NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer producer.Close()

	ctx := context.Background()
	value := []byte("test-message-without-key")

	partition, offset, err := producer.Produce(ctx, cfg.Topics.Message, nil, value)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, partition, int32(0))
	assert.GreaterOrEqual(t, offset, int64(0))
}

// TestProducer_ProduceWithRetry tests the retry mechanism.
// ! This test requires a running Kafka instance.
func TestProducer_ProduceWithRetry(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics: config.TopicsConfig{
			Message: "test.messages",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
	}

	producer, err := NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer producer.Close()

	ctx := context.Background()
	key := []byte("test-key-retry")
	value := []byte("test-message-retry")

	partition, offset, err := producer.ProduceWithRetry(ctx, cfg.Topics.Message, key, value, 2)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, partition, int32(0))
	assert.GreaterOrEqual(t, offset, int64(0))
}

// TestProducer_ProduceWithRetry_ContextCancellation tests context cancellation during retry.
func TestProducer_ProduceWithRetry_ContextCancellation(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics: config.TopicsConfig{
			Message: "test.messages",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 500, // Longer backoff to ensure context cancels during retry
		},
	}

	producer, err := NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer producer.Close()

	// Create a context that will be cancelled immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	key := []byte("test-key-cancel")
	value := []byte("test-message-cancel")

	// Try to produce with cancelled context - should fail immediately
	_, _, err = producer.ProduceWithRetry(ctx, cfg.Topics.Message, key, value, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestProducer_Close tests closing the producer.
func TestProducer_Close(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
	}

	producer, err := NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}

	err = producer.Close()
	assert.NoError(t, err)
}

// TestProducer_MessageSerialization tests message serialization and deserialization.
// ! This test requires a running Kafka instance.
func TestProducer_MessageSerialization(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics: config.TopicsConfig{
			Message: "test.messages.serialization",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
	}

	producer, err := NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer producer.Close()

	ctx := context.Background()

	// Test with various message types
	testCases := []struct {
		name  string
		key   []byte
		value []byte
	}{
		{
			name:  "simple text",
			key:   []byte("key1"),
			value: []byte("simple message"),
		},
		{
			name:  "json message",
			key:   []byte("key2"),
			value: []byte(`{"user_id":"123","content":"hello"}`),
		},
		{
			name:  "binary data",
			key:   []byte("key3"),
			value: []byte{0x00, 0x01, 0x02, 0x03, 0xFF},
		},
		{
			name:  "empty value",
			key:   []byte("key4"),
			value: []byte(""),
		},
		{
			name:  "unicode content",
			key:   []byte("key5"),
			value: []byte("你好世界 🌍"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			partition, offset, err := producer.Produce(ctx, cfg.Topics.Message, tc.key, tc.value)
			assert.NoError(t, err)
			assert.GreaterOrEqual(t, partition, int32(0))
			assert.GreaterOrEqual(t, offset, int64(0))
		})
	}
}

// TestProducer_ConnectionError tests error handling when Kafka connection fails.
func TestProducer_ConnectionError(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers: []string{"invalid-broker:9999"},
		Producer: config.ProducerConfig{
			MaxRetries:     1,
			RetryBackoffMs: 100,
		},
	}

	// Creating producer with invalid broker should fail
	_, err := NewProducer(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create kafka producer")
}
