package kafka

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/Gopher0727/ChatRoom/config"
)

// Feature: distributed-chat-system, Property 36: Kafka 消费重试和死信队列
// Validates: Requirements 12.2
// TestProperty_KafkaConsumerRetryAndDLQ tests that for any Kafka message that fails to process,
// the Consumer should retry consumption according to configuration, and after reaching max retries,
// the message should be moved to the dead letter queue (DLQ).
func TestProperty_KafkaConsumerRetryAndDLQ(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "test-consumer-group-property-retry-dlq",
		Topics: config.TopicsConfig{
			Message: "test.messages.property.retry",
			DLQ:     "test.messages.property.retry.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     2, // Will try 3 times total (initial + 2 retries)
			RetryBackoffMs: 50,
		},
	}

	// Create producer to send test messages
	producer, err := NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer producer.Close()

	// Start DLQ consumer once before the rapid tests
	var dlqMu sync.Mutex
	dlqMessages := make(map[string]bool)

	dlqHandler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		dlqMu.Lock()
		defer dlqMu.Unlock()
		dlqMessages[string(message.Value)] = true
		return nil
	}

	dlqConsumer, err := NewConsumer(cfg, []string{cfg.Topics.DLQ}, dlqHandler)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer dlqConsumer.Stop()

	ctx := context.Background()
	err = dlqConsumer.Start(ctx)
	if err != nil {
		t.Skipf("Failed to start DLQ consumer: %v", err)
		return
	}

	rapid.Check(t, func(t *rapid.T) {
		// Generate random message content (alphanumeric only to avoid special characters)
		messageContent := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(t, "messageContent")

		// Track retry attempts for this specific message
		var mu sync.Mutex
		retryCount := 0
		expectedRetries := cfg.Consumer.MaxRetries + 1 // Initial attempt + retries

		// Handler that always fails to trigger DLQ
		handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
			mu.Lock()
			defer mu.Unlock()
			retryCount++
			return errors.New("simulated processing failure")
		}

		// Create consumer
		consumer, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
		if err != nil {
			t.Skipf("Kafka not available: %v", err)
			return
		}
		defer consumer.Stop()

		err = consumer.Start(ctx)
		if err != nil {
			t.Fatalf("Failed to start consumer: %v", err)
		}

		// Send test message
		_, _, err = producer.Produce(ctx, cfg.Topics.Message, nil, []byte(messageContent))
		if err != nil {
			t.Fatalf("Failed to produce message: %v", err)
		}

		// Wait for message to be processed with retries and sent to DLQ
		time.Sleep(2 * time.Second)

		// Verify retry count
		mu.Lock()
		actualRetries := retryCount
		mu.Unlock()

		// Property: The handler should be called exactly (MaxRetries + 1) times
		assert.Equal(t, expectedRetries, actualRetries,
			"Handler should be called %d times (1 initial + %d retries), but was called %d times",
			expectedRetries, cfg.Consumer.MaxRetries, actualRetries)

		// Wait a bit more for DLQ message to be consumed
		time.Sleep(1 * time.Second)

		// Property: After max retries, the message should be in the DLQ
		dlqMu.Lock()
		receivedInDLQ := dlqMessages[messageContent]
		dlqMu.Unlock()

		assert.True(t, receivedInDLQ, "Message '%s' should be sent to DLQ after max retries", messageContent)
	})
}

// Feature: distributed-chat-system, Property 36: Kafka 消费重试和死信队列 (successful retry)
// Validates: Requirements 12.2
// TestProperty_KafkaConsumerSuccessfulRetry tests that for any message that fails initially
// but succeeds on retry, the message should NOT be sent to DLQ.
func TestProperty_KafkaConsumerSuccessfulRetry(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "test-consumer-group-property-successful-retry",
		Topics: config.TopicsConfig{
			Message: "test.messages.property.successful-retry",
			DLQ:     "test.messages.property.successful-retry.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 50,
		},
	}

	// Create producer to send test messages
	producer, err := NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer producer.Close()

	// Start DLQ consumer once before the rapid tests
	var dlqMu sync.Mutex
	dlqMessages := make(map[string]bool)

	dlqHandler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		dlqMu.Lock()
		defer dlqMu.Unlock()
		dlqMessages[string(message.Value)] = true
		return nil
	}

	dlqConsumer, err := NewConsumer(cfg, []string{cfg.Topics.DLQ}, dlqHandler)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer dlqConsumer.Stop()

	ctx := context.Background()
	err = dlqConsumer.Start(ctx)
	if err != nil {
		t.Skipf("Failed to start DLQ consumer: %v", err)
		return
	}

	rapid.Check(t, func(t *rapid.T) {
		// Generate random message content (alphanumeric only)
		messageContent := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(t, "messageContent")

		// Generate random number of failures before success (less than max retries)
		failuresBeforeSuccess := rapid.IntRange(1, int(cfg.Consumer.MaxRetries)).Draw(t, "failuresBeforeSuccess")

		// Track retry attempts
		var mu sync.Mutex
		attemptCount := 0

		// Handler that fails N times then succeeds
		handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
			mu.Lock()
			defer mu.Unlock()
			attemptCount++
			if attemptCount <= failuresBeforeSuccess {
				return errors.New("simulated temporary failure")
			}
			return nil
		}

		// Create consumer
		consumer, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
		if err != nil {
			t.Skipf("Kafka not available: %v", err)
			return
		}
		defer consumer.Stop()

		err = consumer.Start(ctx)
		if err != nil {
			t.Fatalf("Failed to start consumer: %v", err)
		}

		// Send test message
		_, _, err = producer.Produce(ctx, cfg.Topics.Message, nil, []byte(messageContent))
		if err != nil {
			t.Fatalf("Failed to produce message: %v", err)
		}

		// Wait for message to be processed
		time.Sleep(2 * time.Second)

		// Verify the message was eventually processed successfully
		mu.Lock()
		finalAttemptCount := attemptCount
		mu.Unlock()

		// Property: The handler should be called at least (failuresBeforeSuccess + 1) times
		assert.GreaterOrEqual(t, finalAttemptCount, failuresBeforeSuccess+1,
			"Handler should be called at least %d times", failuresBeforeSuccess+1)

		// Wait a bit more to ensure no DLQ message
		time.Sleep(1 * time.Second)

		// Property: Message should NOT be in DLQ if it eventually succeeded
		dlqMu.Lock()
		receivedInDLQ := dlqMessages[messageContent]
		dlqMu.Unlock()

		assert.False(t, receivedInDLQ, "Message '%s' should NOT be sent to DLQ if processing eventually succeeds", messageContent)
	})
}

// Feature: distributed-chat-system, Property 36: Kafka 消费重试和死信队列 (exponential backoff)
// Validates: Requirements 12.2
// TestProperty_KafkaConsumerExponentialBackoff tests that retry intervals follow exponential backoff.
func TestProperty_KafkaConsumerExponentialBackoff(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "test-consumer-group-property-backoff",
		Topics: config.TopicsConfig{
			Message: "test.messages.property.backoff",
			DLQ:     "test.messages.property.backoff.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100, // 100ms base backoff
		},
	}

	// Create producer to send test messages
	producer, err := NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer producer.Close()

	rapid.Check(t, func(t *rapid.T) {
		// Generate random message content (alphanumeric only)
		messageContent := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(t, "messageContent")

		// Track retry timestamps
		var mu sync.Mutex
		attemptTimestamps := make([]time.Time, 0)

		// Handler that always fails
		handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
			mu.Lock()
			defer mu.Unlock()
			attemptTimestamps = append(attemptTimestamps, time.Now())
			return errors.New("simulated failure for backoff test")
		}

		// Create consumer
		consumer, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
		if err != nil {
			t.Skipf("Kafka not available: %v", err)
			return
		}
		defer consumer.Stop()

		ctx := context.Background()
		err = consumer.Start(ctx)
		if err != nil {
			t.Fatalf("Failed to start consumer: %v", err)
		}

		// Send test message
		_, _, err = producer.Produce(ctx, cfg.Topics.Message, nil, []byte(messageContent))
		if err != nil {
			t.Fatalf("Failed to produce message: %v", err)
		}

		// Wait for all retries to complete
		time.Sleep(3 * time.Second)

		mu.Lock()
		timestamps := make([]time.Time, len(attemptTimestamps))
		copy(timestamps, attemptTimestamps)
		mu.Unlock()

		// Property: Each retry interval should be approximately double the previous one
		if len(timestamps) >= 2 {
			baseBackoff := time.Duration(cfg.Consumer.RetryBackoffMs) * time.Millisecond

			for i := 1; i < len(timestamps); i++ {
				interval := timestamps[i].Sub(timestamps[i-1])
				expectedBackoff := baseBackoff * time.Duration(1<<uint(i-1)) // 2^(i-1) * baseBackoff

				// Allow 50% tolerance for timing variations
				minExpected := expectedBackoff / 2
				maxExpected := expectedBackoff * 2

				assert.GreaterOrEqual(t, interval, minExpected,
					"Retry interval %d should be at least %v (got %v)", i, minExpected, interval)
				assert.LessOrEqual(t, interval, maxExpected,
					"Retry interval %d should be at most %v (got %v)", i, maxExpected, interval)
			}
		}
	})
}
