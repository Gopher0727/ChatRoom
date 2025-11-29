package repositories

import (
	"gorm.io/gorm"

	"github.com/Gopher0727/ChatRoom/internal/models"
)

// GroupRepository 群组仓储
type GroupRepository struct {
	db *gorm.DB
}

// NewGroupRepository 创建群组仓储实例
func NewGroupRepository(db *gorm.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

// Create 创建群组
func (r *GroupRepository) Create(group *models.Group) error {
	return r.db.Create(group).Error
}

// GetByID 根据ID获取群组
func (r *GroupRepository) GetByID(id uint) (*models.Group, error) {
	var group models.Group
	if err := r.db.Preload("Owner").First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// GetByInviteCode 根据邀请码获取群组
func (r *GroupRepository) GetByInviteCode(code string) (*models.Group, error) {
	var group models.Group
	if err := r.db.Where("invite_code = ?", code).First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// Update 更新群组
func (r *GroupRepository) Update(group *models.Group) error {
	return r.db.Save(group).Error
}

// Delete 删除群组
func (r *GroupRepository) Delete(id uint) error {
	return r.db.Delete(&models.Group{}, id).Error
}

// GetUserGroups 获取用户所在的所有群组
func (r *GroupRepository) GetUserGroups(userID uint, limit, offset int) ([]models.Group, int64, error) {
	var groups []models.Group
	var total int64

	query := r.db.Joins("JOIN group_members ON groups.id = group_members.group_id").
		Where("group_members.user_id = ?", userID).
		Preload("Owner")

	err := query.Model(&models.Group{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Limit(limit).Offset(offset).Find(&groups).Error
	return groups, total, err
}

// GetGroupMembers 获取群组成员
func (r *GroupRepository) GetGroupMembers(groupID uint, limit, offset int) ([]models.GroupMember, int64, error) {
	var members []models.GroupMember
	var total int64

	err := r.db.Where("group_id = ?", groupID).Model(&models.GroupMember{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = r.db.Where("group_id = ?", groupID).Preload("User").Limit(limit).Offset(offset).Find(&members).Error
	return members, total, err
}

// IncrementMemberCount 增加群组成员数
func (r *GroupRepository) IncrementMemberCount(groupID uint) error {
	return r.db.Model(&models.Group{}).Where("id = ?", groupID).Update("member_count", gorm.Expr("member_count + 1")).Error
}

// DecrementMemberCount 减少群组成员数
func (r *GroupRepository) DecrementMemberCount(groupID uint) error {
	return r.db.Model(&models.Group{}).Where("id = ?", groupID).Update("member_count", gorm.Expr("member_count - 1")).Error
}
