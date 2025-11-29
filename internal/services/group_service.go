package services

import (
	"errors"

	"github.com/Gopher0727/ChatRoom/internal/models"
	"github.com/Gopher0727/ChatRoom/internal/repositories"
	"github.com/Gopher0727/ChatRoom/pkg/utils"
)

// GroupService 群组服务
type GroupService struct {
	groupRepo   *repositories.GroupRepository
	messageRepo *repositories.MessageRepository
}

// NewGroupService 创建群组服务实例
func NewGroupService(groupRepo *repositories.GroupRepository, messageRepo *repositories.MessageRepository) *GroupService {
	return &GroupService{
		groupRepo:   groupRepo,
		messageRepo: messageRepo,
	}
}

// CreateGroupRequest 创建群组请求
type CreateGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	MaxMembers  int    `json:"max_members"`
}

// GroupDTO 群组数据传输对象
type GroupDTO struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatar_url"`
	OwnerID     uint   `json:"owner_id"`
	InviteCode  string `json:"invite_code"`
	MaxMembers  int    `json:"max_members"`
	MemberCount int    `json:"member_count"`
	CreatedAt   string `json:"created_at"`
}

// GroupMemberDTO 群组成员数据传输对象
type GroupMemberDTO struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
	JoinedAt string `json:"joined_at"`
}

// CreateGroup 创建群组
func (s *GroupService) CreateGroup(ownerID uint, req *CreateGroupRequest) (*GroupDTO, error) {
	// 验证群组名
	if len(req.Name) < 1 || len(req.Name) > 50 {
		return nil, errors.New("group name length invalid")
	}

	// 设置默认值
	if req.MaxMembers <= 0 {
		req.MaxMembers = 10000
	}

	// 生成邀请码
	inviteCode := utils.GenerateInviteCode()

	// 创建群组
	group := &models.Group{
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     ownerID,
		InviteCode:  inviteCode,
		MaxMembers:  req.MaxMembers,
		MemberCount: 1,
		Status:      "active",
	}

	if err := s.groupRepo.Create(group); err != nil {
		return nil, err
	}

	// 添加创建者作为管理员成员
	member := &models.GroupMember{
		GroupID: group.ID,
		UserID:  ownerID,
		Role:    "admin",
	}
	if err := s.messageRepo.AddGroupMember(member); err != nil {
		return nil, err
	}

	return &GroupDTO{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		OwnerID:     group.OwnerID,
		InviteCode:  group.InviteCode,
		MaxMembers:  group.MaxMembers,
		MemberCount: group.MemberCount,
		CreatedAt:   group.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// GetGroupByInviteCode 通过邀请码获取群组
func (s *GroupService) GetGroupByInviteCode(code string) (*GroupDTO, error) {
	group, err := s.groupRepo.GetByInviteCode(code)
	if err != nil {
		return nil, errors.New("group not found")
	}

	return &GroupDTO{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		OwnerID:     group.OwnerID,
		InviteCode:  group.InviteCode,
		MaxMembers:  group.MaxMembers,
		MemberCount: group.MemberCount,
		CreatedAt:   group.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// JoinGroup 用户加入群组
func (s *GroupService) JoinGroup(userID uint, groupID uint) error {
	// 检查群组是否存在
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return errors.New("group not found")
	}

	// 检查人数限制
	if group.MemberCount >= group.MaxMembers {
		return errors.New("group is full")
	}

	// 检查用户是否已经是群组成员
	if _, err := s.messageRepo.GetGroupMember(groupID, userID); err == nil {
		return errors.New("already a member of this group")
	}

	// 添加成员
	member := &models.GroupMember{
		GroupID: groupID,
		UserID:  userID,
		Role:    "member",
	}

	if err := s.messageRepo.AddGroupMember(member); err != nil {
		return err
	}

	// 增加群组成员数
	return s.groupRepo.IncrementMemberCount(groupID)
}

// LeaveGroup 用户离开群组
func (s *GroupService) LeaveGroup(userID uint, groupID uint) error {
	// 检查用户是否是群组成员
	member, err := s.messageRepo.GetGroupMember(groupID, userID)
	if err != nil {
		return errors.New("not a member of this group")
	}

	// 不允许群主离开（应该转移所有权或删除群组）
	if member.Role == "admin" {
		group, _ := s.groupRepo.GetByID(groupID)
		if group.OwnerID == userID {
			return errors.New("group owner cannot leave group")
		}
	}

	// 移除成员
	if err := s.messageRepo.RemoveGroupMember(groupID, userID); err != nil {
		return err
	}

	// 减少群组成员数
	return s.groupRepo.DecrementMemberCount(groupID)
}

// GetUserGroups 获取用户所在的所有群组
func (s *GroupService) GetUserGroups(userID uint, page, pageSize int) ([]GroupDTO, int64, error) {
	offset := (page - 1) * pageSize
	groups, total, err := s.groupRepo.GetUserGroups(userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}

	var groupDTOs []GroupDTO
	for _, group := range groups {
		groupDTOs = append(groupDTOs, GroupDTO{
			ID:          group.ID,
			Name:        group.Name,
			Description: group.Description,
			OwnerID:     group.OwnerID,
			InviteCode:  group.InviteCode,
			MaxMembers:  group.MaxMembers,
			MemberCount: group.MemberCount,
			CreatedAt:   group.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return groupDTOs, total, nil
}

// GetGroupMembers 获取群组成员列表
func (s *GroupService) GetGroupMembers(groupID uint, page, pageSize int) ([]GroupMemberDTO, int64, error) {
	offset := (page - 1) * pageSize
	members, total, err := s.groupRepo.GetGroupMembers(groupID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}

	var memberDTOs []GroupMemberDTO
	for _, member := range members {
		memberDTOs = append(memberDTOs, GroupMemberDTO{
			ID:       member.User.ID,
			Username: member.User.Username,
			Nickname: member.User.Nickname,
			Role:     member.Role,
			JoinedAt: member.JoinedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return memberDTOs, total, nil
}

// GetGroupDetail 获取群组详情
func (s *GroupService) GetGroupDetail(groupID uint) (*GroupDTO, error) {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return nil, errors.New("group not found")
	}

	return &GroupDTO{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		OwnerID:     group.OwnerID,
		InviteCode:  group.InviteCode,
		MaxMembers:  group.MaxMembers,
		MemberCount: group.MemberCount,
		CreatedAt:   group.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}
