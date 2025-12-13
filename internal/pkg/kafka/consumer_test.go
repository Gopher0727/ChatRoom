package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"

	"github.com/Gopher0727/ChatRoom/config"
)

// TestNewConsumer tests the creation of a new Kafka consumer.
// ! This test requires a running Kafka instance.
func TestNewConsumer(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "test-consumer-group",
		Topics: config.TopicsConfig{
			Message: "test.messages",
			DLQ:     "test.messages.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
	}

	handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		return nil
	}

	consumer, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer consumer.Stop()

	assert.NotNil(t, consumer)
	assert.NotNil(t, consumer.consumerGroup)
	assert.NotNil(t, consumer.dlqProducer)
	assert.Equal(t, cfg, consumer.config)
	assert.Equal(t, []string{cfg.Topics.Message}, consumer.topics)
}

// TestConsumer_StartStop tests starting and stopping the consumer.
// ! This test requires a running Kafka instance.
func TestConsumer_StartStop(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "test-consumer-group-start-stop",
		Topics: config.TopicsConfig{
			Message: "test.messages",
			DLQ:     "test.messages.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
	}

	handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		return nil
	}

	consumer, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}

	ctx := context.Background()
	err = consumer.Start(ctx)
	assert.NoError(t, err)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	err = consumer.Stop()
	assert.NoError(t, err)
}

// TestConsumer_MessageProcessing tests message consumption and processing.
// ! This test requires a running Kafka instance.
func TestConsumer_MessageProcessing(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "test-consumer-group-processing",
		Topics: config.TopicsConfig{
			Message: "test.messages.processing",
			DLQ:     "test.messages.processing.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     2,
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

	// Track processed messages
	var mu sync.Mutex
	processedMessages := make([]string, 0)

	handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		mu.Lock()
		defer mu.Unlock()
		processedMessages = append(processedMessages, string(message.Value))
		return nil
	}

	consumer, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer consumer.Stop()

	ctx := context.Background()
	err = consumer.Start(ctx)
	assert.NoError(t, err)

	// Send test messages
	testMessages := []string{"message1", "message2", "message3"}
	for _, msg := range testMessages {
		_, _, err := producer.Produce(ctx, cfg.Topics.Message, nil, []byte(msg))
		assert.NoError(t, err)
	}

	// Wait for messages to be processed
	time.Sleep(5 * time.Second)

	// Verify messages were processed
	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, len(processedMessages), len(testMessages))
}

// TestConsumer_RetryMechanism tests the retry mechanism for failed messages.
// ! This test requires a running Kafka instance.
func TestConsumer_RetryMechanism(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "test-consumer-group-retry",
		Topics: config.TopicsConfig{
			Message: "test.messages.retry",
			DLQ:     "test.messages.retry.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     2,
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

	// Track retry attempts
	var mu sync.Mutex
	retryAttempts := 0

	handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		mu.Lock()
		defer mu.Unlock()
		retryAttempts++
		// Fail the first 2 attempts, succeed on the 3rd
		if retryAttempts < 3 {
			return errors.New("simulated processing error")
		}
		return nil
	}

	consumer, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer consumer.Stop()

	ctx := context.Background()
	err = consumer.Start(ctx)
	assert.NoError(t, err)

	// Send a test message
	_, _, err = producer.Produce(ctx, cfg.Topics.Message, nil, []byte("retry-test-message"))
	assert.NoError(t, err)

	// Wait for message to be processed with retries
	time.Sleep(2 * time.Second)

	// Verify retries occurred
	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, retryAttempts, 3)
}

// TestConsumer_DLQHandling tests that failed messages are sent to DLQ.
// ! This test requires a running Kafka instance.
func TestConsumer_DLQHandling(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "test-consumer-group-dlq",
		Topics: config.TopicsConfig{
			Message: "test.messages.dlq-test",
			DLQ:     "test.messages.dlq-test.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     1,
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

	// Create a consumer for the DLQ to verify the message was sent there
	var mu sync.Mutex
	dlqMessages := make([]string, 0)

	dlqHandler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		mu.Lock()
		defer mu.Unlock()
		dlqMessages = append(dlqMessages, string(message.Value))
		return nil
	}

	// Use a different consumer group for DLQ verification
	dlqCfg := *cfg
	dlqCfg.ConsumerGroup = "test-consumer-group-dlq-verification"

	dlqConsumer, err := NewConsumer(&dlqCfg, []string{cfg.Topics.DLQ}, dlqHandler)
	if err != nil {
		t.Skipf("Skipping DLQ verification: %v", err)
		return
	}
	defer dlqConsumer.Stop()

	ctx := context.Background()
	err = dlqConsumer.Start(ctx)
	assert.NoError(t, err)

	// Handler that always fails
	handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		return errors.New("simulated permanent failure")
	}

	consumer, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer consumer.Stop()

	err = consumer.Start(ctx)
	assert.NoError(t, err)

	// Send a test message that will fail
	testMessage := []byte("dlq-test-message")
	_, _, err = producer.Produce(ctx, cfg.Topics.Message, nil, testMessage)
	assert.NoError(t, err)

	// Wait for message to be processed and sent to DLQ
	time.Sleep(5 * time.Second)

	// Verify message was sent to DLQ
	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, len(dlqMessages), 1)
}

// TestConsumer_MessageDeserialization tests message deserialization.
// ! This test requires a running Kafka instance.
func TestConsumer_MessageDeserialization(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "test-consumer-group-deserialization",
		Topics: config.TopicsConfig{
			Message: "test.messages.deserialization",
			DLQ:     "test.messages.deserialization.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     2,
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

	// Track received messages
	var mu sync.Mutex
	receivedMessages := make(map[string]string)

	handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		mu.Lock()
		defer mu.Unlock()
		receivedMessages[string(message.Key)] = string(message.Value)
		return nil
	}

	consumer, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer consumer.Stop()

	ctx := context.Background()
	err = consumer.Start(ctx)
	assert.NoError(t, err)

	// Send test messages with different formats
	testMessages := map[string]string{
		"text":    "simple text message",
		"json":    `{"user":"alice","msg":"hello"}`,
		"unicode": "你好世界 🌍",
		"empty":   "",
	}

	for key, value := range testMessages {
		_, _, err := producer.Produce(ctx, cfg.Topics.Message, []byte(key), []byte(value))
		assert.NoError(t, err)
	}

	// Wait for messages to be processed
	time.Sleep(2 * time.Second)

	// Verify all messages were received and deserialized correctly
	mu.Lock()
	defer mu.Unlock()
	for key, expectedValue := range testMessages {
		actualValue, exists := receivedMessages[key]
		assert.True(t, exists, "Message with key '%s' should be received", key)
		assert.Equal(t, expectedValue, actualValue, "Message value should match for key '%s'", key)
	}
}

// TestConsumer_ConsumerGroupManagement tests consumer group behavior.
// ! This test requires a running Kafka instance.
func TestConsumer_ConsumerGroupManagement(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "test-consumer-group-management",
		Topics: config.TopicsConfig{
			Message: "test.messages.consumer-group",
			DLQ:     "test.messages.consumer-group.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     2,
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

	// Create two consumers in the same consumer group
	var mu1, mu2 sync.Mutex
	consumer1Messages := make([]string, 0)
	consumer2Messages := make([]string, 0)

	handler1 := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		mu1.Lock()
		defer mu1.Unlock()
		consumer1Messages = append(consumer1Messages, string(message.Value))
		return nil
	}

	handler2 := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		mu2.Lock()
		defer mu2.Unlock()
		consumer2Messages = append(consumer2Messages, string(message.Value))
		return nil
	}

	consumer1, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler1)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer consumer1.Stop()

	consumer2, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler2)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}
	defer consumer2.Stop()

	ctx := context.Background()
	err = consumer1.Start(ctx)
	assert.NoError(t, err)

	err = consumer2.Start(ctx)
	assert.NoError(t, err)

	// Send multiple messages
	messageCount := 10
	for i := 0; i < messageCount; i++ {
		msg := fmt.Sprintf("message-%d", i)
		_, _, err := producer.Produce(ctx, cfg.Topics.Message, nil, []byte(msg))
		assert.NoError(t, err)
	}

	// Wait for messages to be distributed
	time.Sleep(3 * time.Second)

	// Verify messages were distributed between consumers
	mu1.Lock()
	count1 := len(consumer1Messages)
	mu1.Unlock()

	mu2.Lock()
	count2 := len(consumer2Messages)
	mu2.Unlock()

	totalReceived := count1 + count2
	assert.GreaterOrEqual(t, totalReceived, messageCount,
		"Total messages received should be at least %d (got %d)", messageCount, totalReceived)

	// Both consumers should have received some messages (load balancing)
	// ! This might not always be true due to partition assignment, but typically should be
	t.Logf("Consumer 1 received %d messages, Consumer 2 received %d messages", count1, count2)
}

// TestConsumer_ConnectionError tests error handling when Kafka connection fails.
func TestConsumer_ConnectionError(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"invalid-broker:9999"},
		ConsumerGroup: "test-consumer-group-error",
		Topics: config.TopicsConfig{
			Message: "test.messages",
			DLQ:     "test.messages.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     1,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     1,
			RetryBackoffMs: 50,
		},
	}

	handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		return nil
	}

	// Creating consumer with invalid broker should fail
	_, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create kafka consumer group")
}

// TestConsumer_ContextCancellation tests consumer behavior when context is cancelled.
// ! This test requires a running Kafka instance.
func TestConsumer_ContextCancellation(t *testing.T) {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "test-consumer-group-cancellation",
		Topics: config.TopicsConfig{
			Message: "test.messages.cancellation",
			DLQ:     "test.messages.cancellation.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     2,
			RetryBackoffMs: 50,
		},
	}

	handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		// Simulate slow processing
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	consumer, err := NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	err = consumer.Start(ctx)
	assert.NoError(t, err)

	// Cancel context after a short time
	time.Sleep(500 * time.Millisecond)
	cancel()

	// Stop consumer
	err = consumer.Stop()
	assert.NoError(t, err)
}
