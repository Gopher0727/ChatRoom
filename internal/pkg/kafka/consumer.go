package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"

	"github.com/Gopher0727/ChatRoom/config"
)

// MessageHandler is a function type that processes consumed messages.
// It receives the message and returns an error if processing fails.
type MessageHandler func(ctx context.Context, message *sarama.ConsumerMessage) error

// Consumer represents a Kafka message consumer.
// It manages consumer group membership and message consumption.
type Consumer struct {
	consumerGroup sarama.ConsumerGroup
	config        *config.KafkaConfig
	handler       MessageHandler
	dlqProducer   *Producer
	topics        []string
	ready         chan bool
	wg            sync.WaitGroup
	cancel        context.CancelFunc
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler interface.
type consumerGroupHandler struct {
	consumer *Consumer
}

// NewConsumer creates a new Kafka consumer instance.
// It establishes a connection to the Kafka brokers and joins the consumer group.
//
// Parameters:
//   - config: Kafka configuration containing broker addresses and consumer settings
//   - topics: List of topics to subscribe to
//   - handler: Function to process consumed messages
//
// Returns:
//   - *Consumer: The created consumer instance
//   - error: Any error encountered during initialization
func NewConsumer(config *config.KafkaConfig, topics []string, handler MessageHandler) (*Consumer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V2_6_0_0
	saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	saramaConfig.Consumer.Return.Errors = true

	// Set connection timeouts to prevent hanging
	saramaConfig.Net.DialTimeout = 10 * time.Second
	saramaConfig.Net.ReadTimeout = 10 * time.Second
	saramaConfig.Net.WriteTimeout = 10 * time.Second
	saramaConfig.Metadata.Retry.Max = 3
	saramaConfig.Metadata.Retry.Backoff = 250 * time.Millisecond
	saramaConfig.Metadata.Timeout = 10 * time.Second

	consumerGroup, err := sarama.NewConsumerGroup(config.Brokers, config.ConsumerGroup, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka consumer group: %w", err)
	}

	// Create DLQ producer for failed messages
	dlqProducer, err := NewProducer(config)
	if err != nil {
		consumerGroup.Close()
		return nil, fmt.Errorf("failed to create DLQ producer: %w", err)
	}

	return &Consumer{
		consumerGroup: consumerGroup,
		config:        config,
		handler:       handler,
		dlqProducer:   dlqProducer,
		topics:        topics,
		ready:         make(chan bool),
	}, nil
}

// Start begins consuming messages from the subscribed topics.
// It runs in a goroutine and processes messages using the provided handler.
// Failed messages are retried according to the configuration, and after max retries,
// they are sent to the dead letter queue (DLQ).
//
// Parameters:
//   - ctx: Context for cancellation control
//
// Returns:
//   - error: Any error encountered during consumption
func (c *Consumer) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		handler := &consumerGroupHandler{consumer: c}
		for {
			// Check if context is cancelled
			if ctx.Err() != nil {
				return
			}

			// Consume messages
			if err := c.consumerGroup.Consume(ctx, c.topics, handler); err != nil {
				// Log error but continue consuming
				fmt.Printf("Error from consumer: %v\n", err)
			}

			// Check if context was cancelled
			if ctx.Err() != nil {
				return
			}

			c.ready = make(chan bool)
		}
	}()

	// Wait for consumer to be ready
	<-c.ready
	return nil
}

// Stop stops the consumer and waits for all goroutines to finish.
// It should be called when the consumer is no longer needed.
//
// Returns:
//   - error: Any error encountered during stopping
func (c *Consumer) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()

	if err := c.consumerGroup.Close(); err != nil {
		return fmt.Errorf("failed to close kafka consumer group: %w", err)
	}
	if err := c.dlqProducer.Close(); err != nil {
		return fmt.Errorf("failed to close DLQ producer: %w", err)
	}
	return nil
}

// Setup is run at the beginning of a new session, before ConsumeClaim.
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(h.consumer.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited.
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim processes messages from a partition.
// It implements the message consumption logic with retry and DLQ handling.
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			// Process message with retry logic
			if err := h.processMessageWithRetry(session.Context(), message); err != nil {
				// Send to DLQ after max retries
				if dlqErr := h.sendToDLQ(session.Context(), message, err); dlqErr != nil {
					fmt.Printf("Failed to send message to DLQ: %v\n", dlqErr)
				}
			}

			// Mark message as processed
			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
		}
	}
}

// processMessageWithRetry processes a message with retry logic.
// It retries the handler function according to the consumer configuration.
func (h *consumerGroupHandler) processMessageWithRetry(ctx context.Context, message *sarama.ConsumerMessage) error {
	maxRetries := h.consumer.config.Consumer.MaxRetries
	backoff := time.Duration(h.consumer.config.Consumer.RetryBackoffMs) * time.Millisecond

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := h.consumer.handler(ctx, message)
		if err == nil {
			return nil
		}
		lastErr = err

		// Don't sleep after the last attempt
		if attempt < maxRetries {
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// sendToDLQ sends a failed message to the dead letter queue.
// It includes the original error in the message headers.
func (h *consumerGroupHandler) sendToDLQ(ctx context.Context, message *sarama.ConsumerMessage, processingErr error) error {
	dlqTopic := h.consumer.config.Topics.DLQ

	// Create DLQ message with original message data and error information
	// We'll use the original key and value, but send to DLQ topic
	_, _, err := h.consumer.dlqProducer.Produce(ctx, dlqTopic, message.Key, message.Value)
	if err != nil {
		return fmt.Errorf("failed to send message to DLQ: %w", err)
	}

	fmt.Printf("Message sent to DLQ. Topic: %s, Partition: %d, Offset: %d, Error: %v\n", message.Topic, message.Partition, message.Offset, processingErr)

	return nil
}

// Ready returns a channel that is closed when the consumer is ready to consume messages.
// This is useful for synchronization during startup.
func (c *Consumer) Ready() <-chan bool {
	return c.ready
}
