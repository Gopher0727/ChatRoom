package services

import (
	"context"
	"errors"
	"fmt"

	redis "github.com/redis/go-redis/v9"

	"github.com/Gopher0727/ChatRoom/internal/models"
	"github.com/Gopher0727/ChatRoom/internal/repositories"
)

// MessageService 消息服务
type MessageService struct {
	messageRepo *repositories.MessageRepository
	groupRepo   *repositories.GroupRepository
	redisClient *redis.Client
}

// NewMessageService 创建消息服务实例
func NewMessageService(messageRepo *repositories.MessageRepository, groupRepo *repositories.GroupRepository, redisClient *redis.Client) *MessageService {
	return &MessageService{
		messageRepo: messageRepo,
		groupRepo:   groupRepo,
		redisClient: redisClient,
	}
}

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	GroupID uint   `json:"group_id" binding:"required"`
	Content string `json:"content" binding:"required"`
	MsgType string `json:"msg_type"`
}

// MessageDTO 消息数据传输对象
type MessageDTO struct {
	ID         int64    `json:"id"`
	GroupID    uint     `json:"group_id"`
	SenderID   uint     `json:"sender_id"`
	Sender     *UserDTO `json:"sender"`
	Content    string   `json:"content"`
	MsgType    string   `json:"msg_type"`
	SequenceID int64    `json:"sequence_id"`
	CreatedAt  string   `json:"created_at"`
}

// SendMessage 发送消息
func (s *MessageService) SendMessage(senderID uint, req *SendMessageRequest) (*MessageDTO, error) {
	// 验证消息内容
	if len(req.Content) == 0 || len(req.Content) > 5000 {
		return nil, errors.New("message content invalid")
	}

	// 设置默认消息类型
	if req.MsgType == "" {
		req.MsgType = "text"
	}

	// 检查用户是否是群组成员
	member, err := s.messageRepo.GetGroupMember(req.GroupID, senderID)
	if err != nil {
		return nil, errors.New("not a member of this group")
	}
	if member == nil {
		return nil, errors.New("not a member of this group")
	}

	// 获取下一个序列号
	ctx := context.Background()
	seqKey := fmt.Sprintf("group:%d:seq", req.GroupID)
	sequenceID, err := s.redisClient.Incr(ctx, seqKey).Result()
	if err != nil {
		return nil, err
	}

	// 创建消息
	message := &models.Message{
		GroupID:    req.GroupID,
		SenderID:   senderID,
		Content:    req.Content,
		MsgType:    req.MsgType,
		SequenceID: sequenceID,
	}

	if err := s.messageRepo.Create(message); err != nil {
		return nil, err
	}

	// 缓存消息到Redis（保留最近100条）
	msgCacheKey := fmt.Sprintf("group:%d:messages", req.GroupID)
	msgJSON := fmt.Sprintf(`{"id":%d,"sender_id":%d,"content":"%s","msg_type":"%s","sequence_id":%d,"created_at":"%s"}`,
		message.ID, message.SenderID, message.Content, message.MsgType, message.SequenceID, message.CreatedAt.Format("2006-01-02 15:04:05"))

	s.redisClient.RPush(ctx, msgCacheKey, msgJSON)
	s.redisClient.LTrim(ctx, msgCacheKey, -100, -1) // 只保留最近100条

	return &MessageDTO{
		ID:         message.ID,
		GroupID:    message.GroupID,
		SenderID:   message.SenderID,
		Content:    message.Content,
		MsgType:    message.MsgType,
		SequenceID: message.SequenceID,
		CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// GetGroupMessages 获取群组消息列表
func (s *MessageService) GetGroupMessages(groupID uint, page, pageSize int) ([]MessageDTO, int64, error) {
	offset := (page - 1) * pageSize
	messages, total, err := s.messageRepo.GetGroupMessages(groupID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}

	var messageDTOs []MessageDTO
	for _, msg := range messages {
		messageDTOs = append(messageDTOs, MessageDTO{
			ID:         msg.ID,
			GroupID:    msg.GroupID,
			SenderID:   msg.SenderID,
			Content:    msg.Content,
			MsgType:    msg.MsgType,
			SequenceID: msg.SequenceID,
			CreatedAt:  msg.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return messageDTOs, total, nil
}

// GetMessagesBySequence 根据序列号范围获取消息
func (s *MessageService) GetMessagesBySequence(groupID uint, startSeq, endSeq int64) ([]MessageDTO, error) {
	messages, err := s.messageRepo.GetGroupMessagesBySequence(groupID, startSeq, endSeq)
	if err != nil {
		return nil, err
	}

	var messageDTOs []MessageDTO
	for _, msg := range messages {
		messageDTOs = append(messageDTOs, MessageDTO{
			ID:         msg.ID,
			GroupID:    msg.GroupID,
			SenderID:   msg.SenderID,
			Content:    msg.Content,
			MsgType:    msg.MsgType,
			SequenceID: msg.SequenceID,
			CreatedAt:  msg.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return messageDTOs, nil
}

// MarkAsRead 标记消息已读
func (s *MessageService) MarkAsRead(groupID uint, userID uint, msgID int64) error {
	return s.messageRepo.UpdateMemberLastReadMsg(groupID, userID, msgID)
}

// GetUnreadCount 获取未读消息数
func (s *MessageService) GetUnreadCount(groupID uint, userID uint) (int64, error) {
	member, err := s.messageRepo.GetGroupMember(groupID, userID)
	if err != nil {
		return 0, err
	}

	latestSeq, err := s.messageRepo.GetLatestSequence(groupID)
	if err != nil {
		return 0, err
	}

	unreadCount := latestSeq - member.LastReadMsgID
	if unreadCount < 0 {
		return 0, nil
	}

	return unreadCount, nil
}

// GetGroupLatestSequence 获取群组最新序列号
func (s *MessageService) GetGroupLatestSequence(groupID uint) (int64, error) {
	return s.messageRepo.GetLatestSequence(groupID)
}
