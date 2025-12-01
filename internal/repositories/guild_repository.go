package repositories

import (
	"gorm.io/gorm"

	"github.com/Gopher0727/ChatRoom/internal/models"
)

type GuildRepository struct {
	db *gorm.DB
}

func NewGuildRepository(db *gorm.DB) *GuildRepository {
	return &GuildRepository{db: db}
}

// CreateGuild 创建 Guild 并将所有者添加为成员
// 实现逻辑：开启事务，创建 Guild 记录，然后将 OwnerID 对应的用户添加到关联表 members 中
func (r *GuildRepository) CreateGuild(guild *models.Guild) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(guild).Error; err != nil {
			return err
		}
		// 将 Owner 加入成员列表
		if err := tx.Model(guild).Association("Members").Append(&models.User{ID: guild.OwnerID}); err != nil {
			return err
		}
		return nil
	})
}

// GetGuildByID 根据 ID 获取 Guild 信息
// 实现逻辑：查询 guilds 表，并预加载 Owner 字段信息
func (r *GuildRepository) GetGuildByID(id uint) (*models.Guild, error) {
	var guild models.Guild
	err := r.db.Preload("Owner").First(&guild, id).Error
	return &guild, err
}

// AddMember 向 Guild 添加成员
// 实现逻辑：通过 GORM 的 Association 方法，向 members 中间表添加记录
func (r *GuildRepository) AddMember(guildID, userID uint) error {
	// 使用 Association 添加成员
	// 需要先构造一个只有 ID 的 Guild 和 User 对象
	guild := models.Guild{ID: guildID}
	user := models.User{ID: userID}
	return r.db.Model(&guild).Association("Members").Append(&user)
}

// IsMember 检查用户是否是 Guild 成员
// 实现逻辑：查询 members 中间表，检查是否存在指定的 guild_id 和 user_id 组合
func (r *GuildRepository) IsMember(guildID, userID uint) (bool, error) {
	var count int64
	// 检查关联表中是否存在记录
	// GORM 的多对多关联表通常命名为 `members` (在 Guild 模型中定义)
	// 这里的 SQL 可能依赖具体的表名，更安全的方式是查询关联
	err := r.db.Table("members").Where("guild_id = ? AND user_id = ?", guildID, userID).Count(&count).Error
	return count > 0, err
}

// CreateInvite 创建邀请码
// 实现逻辑：向 invites 表插入一条新记录
func (r *GuildRepository) CreateInvite(invite *models.Invite) error {
	return r.db.Create(invite).Error
}

// GetInviteByCode 根据邀请码获取邀请信息
// 实现逻辑：查询 invites 表，查找匹配 code 的记录
func (r *GuildRepository) GetInviteByCode(code string) (*models.Invite, error) {
	var invite models.Invite
	err := r.db.Where("code = ?", code).First(&invite).Error
	return &invite, err
}

// CreateMessage 创建消息
// 实现逻辑：向 messages 表插入一条新记录
func (r *GuildRepository) CreateMessage(msg *models.Message) error {
	return r.db.Create(msg).Error
}

// GetGuildMessages 获取 Guild 的历史消息
// 实现逻辑：查询 messages 表，按创建时间倒序排列，支持分页，并预加载发送者信息
func (r *GuildRepository) GetGuildMessages(guildID uint, limit, offset int) ([]models.Message, error) {
	var messages []models.Message
	err := r.db.Where("guild_id = ?", guildID).
		Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Preload("Sender"). // 预加载发送者信息
		Find(&messages).Error
	return messages, err
}
