package services

import (
	"errors"
	"time"

	"github.com/Gopher0727/ChatRoom/internal/models"
	"github.com/Gopher0727/ChatRoom/internal/repositories"
	"github.com/Gopher0727/ChatRoom/internal/utils"
)

var (
	ErrGuildNotFound  = errors.New("服务器不存在")
	ErrInviteNotFound = errors.New("邀请码不存在")
	ErrInviteExpired  = errors.New("邀请码已过期")
	ErrUserNotMember  = errors.New("用户不是该服务器成员")
	ErrAlreadyMember  = errors.New("用户已经是该服务器成员")
)

type GuildService struct {
	GuildRepo *repositories.GuildRepository
}

func NewGuildService(guildRepo *repositories.GuildRepository) *GuildService {
	return &GuildService{GuildRepo: guildRepo}
}

type CreateGuildRequest struct {
	Topic string `json:"topic" binding:"required"`
}

type CreateGuildResponse struct {
	ID        uint      `json:"id"`
	Topic     string    `json:"topic"`
	OwnerID   uint      `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateGuild 创建一个新的 Guild
// 实现逻辑：接收 OwnerID 和请求参数，构建 Guild 模型，调用 Repository 持久化，并返回创建后的 Guild 信息
func (s *GuildService) CreateGuild(ownerID uint, req *CreateGuildRequest) (*CreateGuildResponse, error) {
	guild := &models.Guild{
		OwnerID: ownerID,
		Topic:   req.Topic,
	}

	if err := s.GuildRepo.CreateGuild(guild); err != nil {
		return nil, err
	}

	return &CreateGuildResponse{
		ID:        guild.ID,
		Topic:     guild.Topic,
		OwnerID:   guild.OwnerID,
		CreatedAt: guild.CreatedAt,
	}, nil
}

type CreateInviteResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateInvite 为指定 Guild 创建邀请码
// 实现逻辑：检查请求用户是否为 Guild 成员，生成唯一邀请码，设置过期时间，并持久化到数据库
func (s *GuildService) CreateInvite(userID, guildID uint) (*CreateInviteResponse, error) {
	// 检查用户是否是成员（通常只有成员才能邀请，或者只有管理员，这里简化为成员）
	isMember, err := s.GuildRepo.IsMember(guildID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrUserNotMember
	}

	code := utils.GenerateInviteCode()
	expiresAt := time.Now().Add(24 * time.Hour) // 默认24小时过期

	invite := &models.Invite{
		GuildID:   guildID,
		Code:      code,
		CreatorID: userID,
		ExpiresAt: expiresAt,
	}

	if err := s.GuildRepo.CreateInvite(invite); err != nil {
		return nil, err
	}

	return &CreateInviteResponse{
		Code:      code,
		ExpiresAt: expiresAt,
	}, nil
}

// JoinGuild 用户通过邀请码加入 Guild
// 实现逻辑：验证邀请码有效性（存在且未过期），检查用户是否已是成员，若通过则将用户添加到 Guild 成员列表
func (s *GuildService) JoinGuild(userID uint, code string) error {
	invite, err := s.GuildRepo.GetInviteByCode(code)
	if err != nil {
		return ErrInviteNotFound
	}

	if invite.ExpiresAt.Before(time.Now()) {
		return ErrInviteExpired
	}

	// 检查是否已经是成员
	isMember, err := s.GuildRepo.IsMember(invite.GuildID, userID)
	if err != nil {
		return err
	}
	if isMember {
		return ErrAlreadyMember
	}

	return s.GuildRepo.AddMember(invite.GuildID, userID)
}

type SendMessageRequest struct {
	Content string `json:"content" binding:"required"`
	MsgType string `json:"msg_type"` // 可选，默认为 text
}

type MessageResponse struct {
	ID        int64     `json:"id"`
	SenderID  uint      `json:"sender_id"`
	Content   string    `json:"content"`
	MsgType   string    `json:"msg_type"`
	CreatedAt time.Time `json:"created_at"`
	// SenderName string `json:"sender_name,omitempty"` // 如果需要返回发送者名字
}

// SendMessage 发送消息到指定 Guild
// 实现逻辑：检查发送者是否为 Guild 成员，构建消息模型（默认为文本类型），调用 Repository 保存消息
func (s *GuildService) SendMessage(userID, guildID uint, req *SendMessageRequest) (*MessageResponse, error) {
	// 检查成员资格
	isMember, err := s.GuildRepo.IsMember(guildID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrUserNotMember
	}

	msgType := "text"
	if req.MsgType != "" {
		msgType = req.MsgType
	}

	msg := &models.Message{
		GuildID:  guildID,
		SenderID: userID,
		Content:  req.Content,
		MsgType:  msgType,
		// SequenceID: 0, // 暂时不处理
	}

	if err := s.GuildRepo.CreateMessage(msg); err != nil {
		return nil, err
	}

	return &MessageResponse{
		ID:        msg.ID,
		SenderID:  msg.SenderID,
		Content:   msg.Content,
		MsgType:   msg.MsgType,
		CreatedAt: msg.CreatedAt,
	}, nil
}

// GetMessages 获取指定 Guild 的消息列表
// 实现逻辑：检查请求用户是否为 Guild 成员，处理分页参数（limit, offset），调用 Repository 查询消息记录并转换为响应格式
func (s *GuildService) GetMessages(userID, guildID uint, limit, offset int) ([]MessageResponse, error) {
	// 检查成员资格
	isMember, err := s.GuildRepo.IsMember(guildID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrUserNotMember
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	msgs, err := s.GuildRepo.GetGuildMessages(guildID, limit, offset)
	if err != nil {
		return nil, err
	}

	var resp []MessageResponse
	for _, m := range msgs {
		resp = append(resp, MessageResponse{
			ID:        m.ID,
			SenderID:  m.SenderID,
			Content:   m.Content,
			MsgType:   m.MsgType,
			CreatedAt: m.CreatedAt,
		})
	}
	return resp, nil
}

// GetUserGuildIDs 获取用户加入的所有 Guild ID
func (s *GuildService) GetUserGuildIDs(userID uint) ([]uint, error) {
	return s.GuildRepo.GetUserGuildIDs(userID)
}
