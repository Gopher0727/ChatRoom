package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/Gopher0727/ChatRoom/internal/model"
	pb "github.com/Gopher0727/ChatRoom/internal/pkg/proto"
	"github.com/Gopher0727/ChatRoom/internal/pkg/redis"
	"github.com/Gopher0727/ChatRoom/internal/repository"
	"github.com/Gopher0727/ChatRoom/utils/snowflake"
)

var (
	ErrMessageNotFound       = errors.New("message not found")
	ErrInvalidMessageContent = errors.New("invalid message content")
	ErrUserNotInGuild        = errors.New("user is not a member of this guild")
)

// SendMessageRequest represents a request to send a message
type SendMessageRequest struct {
	UserID  string `json:"user_id" binding:"required"`
	GuildID string `json:"guild_id" binding:"required"`
	Content string `json:"content" binding:"required,max=2000"`
}

// GetMessagesRequest represents a request to retrieve messages
type GetMessagesRequest struct {
	GuildID   string `json:"guild_id" binding:"required"`
	LastSeqID int64  `json:"last_seq_id"`
	Limit     int    `json:"limit"`
}

// IMessageService defines the interface for message operations
type IMessageService interface {
	SendMessage(ctx context.Context, userID, guildID, content string) (*model.Message, error)
	GetMessages(ctx context.Context, guildID string, lastSeqID int64, limit int) ([]*model.Message, bool, error)
	BatchGetMessages(ctx context.Context, messageIDs []string) ([]*model.Message, error)
}

// MessageService implements the MessageService interface
type MessageService struct {
	messageRepo  repository.IMessageRepository
	guildService IGuildService
	snowflakeGen *snowflake.Generator
	redisClient  redis.RedisClient
}

// NewMessageService creates a new MessageService instance
func NewMessageService(
	messageRepo repository.IMessageRepository,
	guildService IGuildService,
	snowflakeGen *snowflake.Generator,
	redisClient redis.RedisClient,
) IMessageService {
	return &MessageService{
		messageRepo:  messageRepo,
		guildService: guildService,
		snowflakeGen: snowflakeGen,
		redisClient:  redisClient,
	}
}

// SendMessage sends a message to a guild
// It generates a Snowflake ID, obtains a Seq ID from Redis,
// and writes to both PostgreSQL and Redis Pub/Sub in parallel
func (s *MessageService) SendMessage(ctx context.Context, userID, guildID, content string) (*model.Message, error) {
	// Validate message content
	if len(content) == 0 {
		return nil, ErrInvalidMessageContent
	}
	if len(content) > 2000 {
		return nil, ErrInvalidMessageContent
	}

	// Verify user is a member of the guild
	isMember, err := s.guildService.IsMember(ctx, userID, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to check guild membership: %w", err)
	}
	if !isMember {
		return nil, ErrUserNotInGuild
	}

	// Generate Snowflake ID for the message
	snowflakeID, err := s.snowflakeGen.NextID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate snowflake ID: %w", err)
	}
	messageID := strconv.FormatInt(snowflakeID, 10)

	// Get Seq ID from Redis (atomic increment)
	seqID, err := s.redisClient.GenerateSeqID(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate seq ID: %w", err)
	}

	// Create message object
	message := &model.Message{
		ID:      messageID,
		UserID:  userID,
		GuildID: guildID,
		Content: content,
		SeqID:   seqID,
	}

	// Parallel write to PostgreSQL and Redis Pub/Sub
	var wg sync.WaitGroup
	var dbErr, pubsubErr error

	wg.Add(2)

	// Write to PostgreSQL
	go func() {
		defer wg.Done()
		dbErr = s.messageRepo.Create(ctx, message)
	}()

	// Publish to Redis Pub/Sub
	go func() {
		defer wg.Done()
		pubsubErr = s.publishMessage(ctx, message)
	}()

	wg.Wait()

	// Check for errors
	if dbErr != nil {
		return nil, fmt.Errorf("failed to save message to database: %w", dbErr)
	}
	if pubsubErr != nil {
		// Log the error but don't fail the request since the message is already in the database
		// TODO: In production, this should trigger an alert
		fmt.Printf("WARNING: failed to publish message to Redis Pub/Sub: %v\n", pubsubErr)
	}

	return message, nil
}

// GetMessages retrieves messages for a guild with optional filtering by sequence ID
// Supports incremental message queries and pagination
func (s *MessageService) GetMessages(ctx context.Context, guildID string, lastSeqID int64, limit int) ([]*model.Message, bool, error) {
	// Set default limit if not provided
	if limit <= 0 {
		limit = 50 // Default page size
	}
	if limit > 100 {
		limit = 100 // Maximum page size
	}

	// Query messages from database (fetch one extra to check if there are more)
	messages, err := s.messageRepo.FindByGuild(ctx, guildID, lastSeqID, limit+1)
	if err != nil {
		return nil, false, fmt.Errorf("failed to retrieve messages: %w", err)
	}

	// Check if there are more messages
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}

	return messages, hasMore, nil
}

// BatchGetMessages retrieves multiple messages by their IDs
func (s *MessageService) BatchGetMessages(ctx context.Context, messageIDs []string) ([]*model.Message, error) {
	if len(messageIDs) == 0 {
		return []*model.Message{}, nil
	}

	messages := make([]*model.Message, 0, len(messageIDs))
	for _, id := range messageIDs {
		message, err := s.messageRepo.FindByID(ctx, id)
		if err != nil {
			// Skip messages that don't exist
			continue
		}
		messages = append(messages, message)
	}

	return messages, nil
}

// publishMessage publishes a message to Redis Pub/Sub
// The message is serialized using Protobuf and published to a guild-specific channel
func (s *MessageService) publishMessage(ctx context.Context, message *model.Message) error {
	// Convert to Protobuf message
	pbMessage := &pb.WSMessage{
		MessageId: message.ID,
		UserId:    message.UserID,
		GuildId:   message.GuildID,
		Content:   message.Content,
		SeqId:     message.SeqID,
		Timestamp: time.Now().UnixMilli(),
		Type:      pb.MessageType_TEXT,
	}

	// Serialize to bytes
	data, err := proto.Marshal(pbMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish to guild-specific channel
	channel := fmt.Sprintf("guild:%s", message.GuildID)
	if err := s.redisClient.Publish(ctx, channel, data); err != nil {
		return fmt.Errorf("failed to publish to channel %s: %w", channel, err)
	}

	return nil
}
