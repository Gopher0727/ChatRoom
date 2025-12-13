package kafka

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/IBM/sarama"

	"github.com/Gopher0727/ChatRoom/config"
	"github.com/Gopher0727/ChatRoom/internal/pkg/kafka"
)

// ExampleProducer demonstrates how to create and use a Kafka producer.
func ExampleProducer() {
	// Create Kafka configuration
	cfg := &config.KafkaConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics: config.TopicsConfig{
			Message: "chat.messages",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
	}

	// Create producer
	producer, err := kafka.NewProducer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	// Send a message
	ctx := context.Background()
	key := []byte("user-123")
	value := []byte(`{"user_id":"user-123","guild_id":"guild-456","content":"Hello, World!"}`)

	partition, offset, err := producer.Produce(ctx, cfg.Topics.Message, key, value)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Message sent to partition %d at offset %d\n", partition, offset)
}

// ExampleProducer_withRetry demonstrates how to send a message with custom retry logic.
func ExampleProducer_withRetry() {
	cfg := &config.KafkaConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics: config.TopicsConfig{
			Message: "chat.messages",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
	}

	producer, err := kafka.NewProducer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	ctx := context.Background()
	key := []byte("user-123")
	value := []byte(`{"content":"Important message"}`)

	// Send with custom retry count
	partition, offset, err := producer.ProduceWithRetry(ctx, cfg.Topics.Message, key, value, 5)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Message sent to partition %d at offset %d\n", partition, offset)
}

// ExampleConsumer demonstrates how to create and use a Kafka consumer.
func ExampleConsumer() {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "chat-consumer",
		Topics: config.TopicsConfig{
			Message: "chat.messages",
			DLQ:     "chat.messages.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 1000,
		},
	}

	// Define message handler
	handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		fmt.Printf("Received message: %s\n", string(message.Value))
		// Process the message here
		return nil
	}

	// Create consumer
	consumer, err := kafka.NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Stop()

	// Start consuming
	ctx := context.Background()
	if err := consumer.Start(ctx); err != nil {
		log.Fatal(err)
	}

	// Wait for consumer to be ready
	<-consumer.Ready()
	fmt.Println("Consumer is ready and processing messages")

	// Keep running for a while
	time.Sleep(10 * time.Second)
}

// ExampleConsumer_withErrorHandling demonstrates consumer with error handling and DLQ.
func ExampleConsumer_withErrorHandling() {
	cfg := &config.KafkaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		ConsumerGroup: "chat-consumer-with-dlq",
		Topics: config.TopicsConfig{
			Message: "chat.messages",
			DLQ:     "chat.messages.dlq",
		},
		Producer: config.ProducerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 100,
		},
		Consumer: config.ConsumerConfig{
			MaxRetries:     3,
			RetryBackoffMs: 1000,
		},
	}

	// Handler that may fail
	handler := func(ctx context.Context, message *sarama.ConsumerMessage) error {
		// Simulate processing
		fmt.Printf("Processing message: %s\n", string(message.Value))

		// Simulate occasional errors
		// In real code, this would be actual business logic
		if string(message.Value) == "bad-message" {
			return fmt.Errorf("failed to process message")
		}

		return nil
	}

	consumer, err := kafka.NewConsumer(cfg, []string{cfg.Topics.Message}, handler)
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Stop()

	ctx := context.Background()
	if err := consumer.Start(ctx); err != nil {
		log.Fatal(err)
	}

	<-consumer.Ready()
	fmt.Println("Consumer is ready with error handling")

	// Messages that fail after max retries will be sent to DLQ
	time.Sleep(10 * time.Second)
}
