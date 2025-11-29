package repositories

import (
	"gorm.io/gorm"

	"github.com/Gopher0727/ChatRoom/internal/models"
)

// MessageRepository 消息仓储
type MessageRepository struct {
	db *gorm.DB
}

// NewMessageRepository 创建消息仓储实例
func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Create 创建消息
func (r *MessageRepository) Create(message *models.Message) error {
	return r.db.Create(message).Error
}

// GetByID 根据ID获取消息
func (r *MessageRepository) GetByID(id int64) (*models.Message, error) {
	var message models.Message
	if err := r.db.Preload("Sender").First(&message, id).Error; err != nil {
		return nil, err
	}
	return &message, nil
}

// GetGroupMessages 获取群组消息列表
func (r *MessageRepository) GetGroupMessages(groupID uint, limit, offset int) ([]models.Message, int64, error) {
	var messages []models.Message
	var total int64

	err := r.db.Where("group_id = ?", groupID).Model(&models.Message{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = r.db.Where("group_id = ?", groupID).
		Preload("Sender").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages).Error
	return messages, total, err
}

// GetGroupMessagesBySequence 根据序列号获取消息
func (r *MessageRepository) GetGroupMessagesBySequence(groupID uint, startSeq, endSeq int64) ([]models.Message, error) {
	var messages []models.Message
	err := r.db.Where("group_id = ? AND sequence_id >= ? AND sequence_id <= ?", groupID, startSeq, endSeq).
		Preload("Sender").
		Order("sequence_id ASC").
		Find(&messages).Error
	return messages, err
}

// GetLatestSequence 获取群组的最新序列号
func (r *MessageRepository) GetLatestSequence(groupID uint) (int64, error) {
	var maxSeq int64
	err := r.db.Model(&models.Message{}).
		Where("group_id = ?", groupID).
		Select("COALESCE(MAX(sequence_id), 0)").
		Row().
		Scan(&maxSeq)
	return maxSeq, err
}

// Delete 删除消息
func (r *MessageRepository) Delete(id int64) error {
	return r.db.Delete(&models.Message{}, id).Error
}

// AddGroupMember 添加群组成员
func (r *MessageRepository) AddGroupMember(member *models.GroupMember) error {
	return r.db.Create(member).Error
}

// RemoveGroupMember 移除群组成员
func (r *MessageRepository) RemoveGroupMember(groupID, userID uint) error {
	return r.db.Where("group_id = ? AND user_id = ?", groupID, userID).Delete(&models.GroupMember{}).Error
}

// GetGroupMember 获取群组成员信息
func (r *MessageRepository) GetGroupMember(groupID, userID uint) (*models.GroupMember, error) {
	var member models.GroupMember
	if err := r.db.Where("group_id = ? AND user_id = ?", groupID, userID).First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

// UpdateMemberLastReadMsg 更新成员最后读取消息ID
func (r *MessageRepository) UpdateMemberLastReadMsg(groupID, userID uint, msgID int64) error {
	return r.db.Model(&models.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Update("last_read_msg_id", msgID).Error
}
