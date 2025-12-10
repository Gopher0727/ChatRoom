package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"

	"github.com/Gopher0727/ChatRoom/config"
)

// Producer represents a Kafka message producer.
// It manages the connection to Kafka brokers and provides methods to send messages.
type Producer struct {
	producer sarama.SyncProducer
	config   *config.KafkaConfig
}

// NewProducer creates a new Kafka producer instance.
// It establishes a connection to the Kafka brokers specified in the configuration.
//
// Parameters:
//   - config: Kafka configuration containing broker addresses and producer settings
//
// Returns:
//   - *Producer: The created producer instance
//   - error: Any error encountered during initialization
func NewProducer(config *config.KafkaConfig) (*Producer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Retry.Max = config.Producer.MaxRetries
	saramaConfig.Producer.Retry.Backoff = time.Duration(config.Producer.RetryBackoffMs) * time.Millisecond
	saramaConfig.Producer.Compression = sarama.CompressionSnappy
	saramaConfig.Producer.Idempotent = true
	saramaConfig.Net.MaxOpenRequests = 1

	// Set connection timeouts to prevent hanging
	saramaConfig.Net.DialTimeout = 10 * time.Second
	saramaConfig.Net.ReadTimeout = 10 * time.Second
	saramaConfig.Net.WriteTimeout = 10 * time.Second
	saramaConfig.Metadata.Retry.Max = 3
	saramaConfig.Metadata.Retry.Backoff = 250 * time.Millisecond
	saramaConfig.Metadata.Timeout = 10 * time.Second

	producer, err := sarama.NewSyncProducer(config.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}
	return &Producer{
		producer: producer,
		config:   config,
	}, nil
}

// Produce sends a message to the specified Kafka topic.
// It handles retries automatically based on the producer configuration.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - topic: The Kafka topic to send the message to
//   - key: Optional message key for partitioning (can be nil)
//   - value: The message payload as bytes
//
// Returns:
//   - partition: The partition the message was sent to
//   - offset: The offset of the message in the partition
//   - error: Any error encountered during sending
func (p *Producer) Produce(ctx context.Context, topic string, key []byte, value []byte) (partition int32, offset int64, err error) {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(value),
	}
	if key != nil {
		msg.Key = sarama.ByteEncoder(key)
	}

	partition, offset, err = p.producer.SendMessage(msg)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to send message to topic %s: %w", topic, err)
	}
	return partition, offset, nil
}

// ProduceWithRetry sends a message to Kafka with custom retry logic.
// This method provides additional retry capabilities beyond the producer's built-in retries.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - topic: The Kafka topic to send the message to
//   - key: Optional message key for partitioning (can be nil)
//   - value: The message payload as bytes
//   - maxRetries: Maximum number of retry attempts
//
// Returns:
//   - partition: The partition the message was sent to
//   - offset: The offset of the message in the partition
//   - error: Any error encountered during sending
func (p *Producer) ProduceWithRetry(ctx context.Context, topic string, key []byte, value []byte, maxRetries int) (partition int32, offset int64, err error) {
	var lastErr error
	backoff := time.Duration(p.config.Producer.RetryBackoffMs) * time.Millisecond
	for attempt := 0; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		default:
		}

		partition, offset, err = p.Produce(ctx, topic, key, value)
		if err == nil {
			return partition, offset, nil
		}
		lastErr = err

		if attempt < maxRetries {
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}
	}
	return 0, 0, fmt.Errorf("failed to send message after %d attempts: %w", maxRetries, lastErr)
}

// Close closes the Kafka producer and releases all resources.
// It should be called when the producer is no longer needed.
//
// Returns:
//   - error: Any error encountered during closing
func (p *Producer) Close() error {
	if p.producer != nil {
		if err := p.producer.Close(); err != nil {
			return fmt.Errorf("failed to close kafka producer: %w", err)
		}
	}
	return nil
}
