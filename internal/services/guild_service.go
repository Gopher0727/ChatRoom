package services

import (
	"errors"
	"log"
	"time"

	"github.com/Gopher0727/ChatRoom/internal/models"
	"github.com/Gopher0727/ChatRoom/internal/repositories"
	"github.com/Gopher0727/ChatRoom/pkg/utils"
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
	UserRepo  *repositories.UserRepository
}

func NewGuildService(guildRepo *repositories.GuildRepository, userRepo *repositories.UserRepository) *GuildService {
	return &GuildService{
		GuildRepo: guildRepo,
		UserRepo:  userRepo,
	}
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

// CreateGuild 接收 OwnerID 和请求参数，构建 Guild 模型，调用 Repository 持久化，并返回创建后的 Guild 信息
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

// CreateInvite 检查请求用户是否为 Guild 成员，生成唯一邀请码，设置过期时间，并持久化到数据库
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

// JoinGuild 验证邀请码有效性（存在且未过期），检查用户是否已是成员，若通过则将用户添加到 Guild 成员列表
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
	NodeID  string `json:"node_id"`  // 新增：一致性哈希命中的节点ID
}

type MessageResponse struct {
	ID           int64     `json:"id"`
	GuildID      uint      `json:"guild_id"`
	SenderID     uint      `json:"sender_id"`
	SenderName   string    `json:"sender_name"`
	SenderAvatar string    `json:"sender_avatar"`
	Content      string    `json:"content"`
	MsgType      string    `json:"msg_type"`
	SequenceID   int64     `json:"sequence_id"`
	CreatedAt    time.Time `json:"created_at"`
	NodeID       string    `json:"node_id"` // 新增：命中节点ID
}

// 填充发送者信息
func (s *GuildService) fillSenderInfo(resp *MessageResponse) {
	user, err := s.UserRepo.GetByID(resp.SenderID)
	if err == nil && user != nil {
		resp.SenderName = user.Nickname
		if resp.SenderName == "" {
			resp.SenderName = user.UserName
		}
		resp.SenderAvatar = user.AvatarURL
	} else {
		resp.SenderName = "Unknown"
	}
}

// 批量填充发送者信息
func (s *GuildService) populateMessageSenders(msgs []models.Message) ([]MessageResponse, error) {
	// 收集需要查询用户信息的 SenderID (针对 Redis 缓存命中或 Preload 失败的情况)
	senderIDs := make([]uint, 0)
	senderIDMap := make(map[uint]bool)
	for _, m := range msgs {
		if m.Sender == nil && !senderIDMap[m.SenderID] {
			senderIDs = append(senderIDs, m.SenderID)
			senderIDMap[m.SenderID] = true
		}
	}

	// 批量获取用户信息
	var usersMap map[uint]*models.User
	if len(senderIDs) > 0 {
		usersMap, _ = s.UserRepo.GetByIDs(senderIDs)
	}

	var resp []MessageResponse
	for _, m := range msgs {
		r := MessageResponse{
			ID:         m.ID,
			GuildID:    m.GuildID,
			SenderID:   m.SenderID,
			Content:    m.Content,
			MsgType:    m.MsgType,
			SequenceID: m.SequenceID,
			CreatedAt:  m.CreatedAt,
		}

		// 优化：如果 GORM Preload 已经加载了 Sender，直接使用
		if m.Sender != nil {
			r.SenderName = m.Sender.Nickname
			if r.SenderName == "" {
				r.SenderName = m.Sender.UserName
			}
			r.SenderAvatar = m.Sender.AvatarURL
		} else if user, ok := usersMap[m.SenderID]; ok && user != nil {
			// 否则查批量获取的结果
			r.SenderName = user.Nickname
			if r.SenderName == "" {
				r.SenderName = user.UserName
			}
			r.SenderAvatar = user.AvatarURL
		} else {
			r.SenderName = "Unknown"
		}

		resp = append(resp, r)
	}
	return resp, nil
}

// SendMessage 检查发送者是否为 Guild 成员，构建消息模型（默认为文本类型），调用 Repository 保存消息
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

	// 获取所有成员和在线成员，计算出离线成员
	allMembers, err := s.GuildRepo.GetGuildMemberIDs(guildID)
	if err != nil {
		log.Printf("Error getting guild members: %v", err)
	}

	onlineMembers, err := s.GuildRepo.GetOnlineUserIDsInGuild(guildID)
	if err != nil {
		log.Printf("Error getting online members: %v", err)
	}

	onlineSet := make(map[uint]bool)
	for _, id := range onlineMembers {
		onlineSet[id] = true
	}

	// 将消息存入离线成员的收件箱
	for _, memberID := range allMembers {
		// 不给自己存离线消息，只给离线的成员存
		if !onlineSet[memberID] && memberID != userID {
			if err := s.GuildRepo.PushToInbox(memberID, msg); err != nil {
				log.Printf("Error pushing to inbox for user %d: %v", memberID, err)
			}
		}
	}

	resp := &MessageResponse{
		ID:         msg.ID,
		GuildID:    msg.GuildID,
		SenderID:   msg.SenderID,
		Content:    msg.Content,
		MsgType:    msg.MsgType,
		SequenceID: msg.SequenceID,
		CreatedAt:  msg.CreatedAt,
		NodeID:     req.NodeID, // 透传节点ID
	}
	s.fillSenderInfo(resp)

	return resp, nil
}

// GetOfflineMessages 获取用户的离线消息
func (s *GuildService) GetOfflineMessages(userID uint) ([]MessageResponse, error) {
	msgs, err := s.GuildRepo.GetAndClearInbox(userID)
	if err != nil {
		return nil, err
	}
	return s.populateMessageSenders(msgs)
}

// RefreshUserOnlineStatus 刷新用户在指定 Guilds 的在线状态
func (s *GuildService) RefreshUserOnlineStatus(userID uint, guildIDs []uint) {
	for _, guildID := range guildIDs {
		s.GuildRepo.RefreshUserOnline(guildID, userID)
	}
}

// GetMessages 检查请求用户是否为 Guild 成员，处理分页参数（limit, offset），调用 Repository 查询消息记录并转换为响应格式
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

	return s.populateMessageSenders(msgs)
}

// GetMessagesAfterSequence 获取指定 SequenceID 之后的消息 (增量同步)
func (s *GuildService) GetMessagesAfterSequence(userID, guildID uint, afterSeq int64, limit int) ([]MessageResponse, error) {
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

	msgs, err := s.GuildRepo.GetMessagesAfterSequence(guildID, afterSeq, limit)
	if err != nil {
		return nil, err
	}

	return s.populateMessageSenders(msgs)
}

// GetUserGuildIDs 获取用户加入的所有 Guild ID
func (s *GuildService) GetUserGuildIDs(userID uint) ([]uint, error) {
	return s.GuildRepo.GetUserGuildIDs(userID)
}
