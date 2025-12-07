package consumer

import (
	"context"
	"encoding/json"
	"log"

	"github.com/IBM/sarama"

	"github.com/Gopher0727/ChatRoom/internal/services"
	"github.com/Gopher0727/ChatRoom/pkg/ws"
)

type MessageConsumer struct {
	guildService *services.GuildService
	hub          *ws.Hub
}

func NewMessageConsumer(guildService *services.GuildService, hub *ws.Hub) *MessageConsumer {
	return &MessageConsumer{
		guildService: guildService,
		hub:          hub,
	}
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *MessageConsumer) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (consumer *MessageConsumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *MessageConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		log.Printf("消费消息: value = %s, timestamp = %v, topic = %s", string(message.Value), message.Timestamp, message.Topic)

		var req struct {
			UserID  uint                         `json:"user_id"`
			GuildID uint                         `json:"guild_id"`
			Content *services.SendMessageRequest `json:"content"`
		}

		if err := json.Unmarshal(message.Value, &req); err != nil {
			log.Printf("反序列化消息失败: %v", err)
			session.MarkMessage(message, "")
			continue
		}

		// 调用 Service 保存消息
		resp, err := consumer.guildService.SendMessage(req.UserID, req.GuildID, req.Content)
		if err != nil {
			log.Printf("保存来自 Kafka 的消息失败: %v", err)
			// 可以在这里决定是否重试，或者记录到死信队列
			// 暂时标记为已消费，避免死循环
			session.MarkMessage(message, "")
			continue
		}

		// 广播消息
		consumer.hub.BroadcastToGuild(req.GuildID, resp)

		session.MarkMessage(message, "")
	}
	return nil
}

func StartConsumer(brokers []string, groupID string, topic string, consumer *MessageConsumer) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	client, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		log.Fatalf("创建消费者组客户端失败: %v", err)
	}

	ctx := context.Background()
	go func() {
		for {
			if err := client.Consume(ctx, []string{topic}, consumer); err != nil {
				log.Printf("消费者错误: %v", err)
			}
			// check if context was cancelled, signaling that the consumer should stop
			if ctx.Err() != nil {
				return
			}
		}
	}()
}
