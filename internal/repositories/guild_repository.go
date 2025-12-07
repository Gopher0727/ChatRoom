package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/Gopher0727/ChatRoom/internal/models"
)

const (
	onlineUserKeyPrefix      = "online_users:guild:"    // Redis ZSET, 分数为过期时间戳
	recentMessagesKeyPrefix  = "recent_messages:guild:" // Redis ZSET, 分数为时间戳，值为消息体 json
	userInboxKeyPrefix       = "inbox:user:"            // Redis LIST, 值是消息体 json
	guildMembersKeyPrefix    = "guild_members:"         // Redis SET, 值是 userID
	guildMessageSeqKeyPrefix = "guild_msg_seq:"         // Redis String (Counter)
	recentMessagesCount      = 100                      // 缓存最近的 100 条消息
)

type GuildRepository struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewGuildRepository(db *gorm.DB, redis *redis.Client) *GuildRepository {
	return &GuildRepository{db: db, redis: redis}
}

// CreateGuild 创建 Guild 并将所有者添加为成员
func (r *GuildRepository) CreateGuild(guild *models.Guild) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(guild).Error; err != nil {
			return err
		}

		member := models.GuildMember{
			GuildID: guild.ID,
			UserID:  guild.OwnerID,
		}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		return nil
	})
}

// GetGuildByID 根据 ID 获取 Guild 信息
func (r *GuildRepository) GetGuildByID(id uint) (*models.Guild, error) {
	var guild models.Guild
	err := r.db.Preload("Owner").First(&guild, id).Error
	return &guild, err
}

// AddMember 向 Guild 添加成员
func (r *GuildRepository) AddMember(guildID, userID uint) error {
	member := models.GuildMember{
		GuildID: guildID,
		UserID:  userID,
	}
	if err := r.db.Create(&member).Error; err != nil {
		return err
	}

	// 更新 Redis 缓存 (如果缓存存在)
	if r.redis != nil {
		key := fmt.Sprintf("%s%d", guildMembersKeyPrefix, guildID)
		// 只有当缓存存在时才添加，防止缓存不一致（如果缓存不存在，下次读取时会自动全量加载）
		if r.redis.Exists(context.Background(), key).Val() > 0 {
			r.redis.SAdd(context.Background(), key, userID)
		}
	}
	return nil
}

// IsMember 检查用户是否是 Guild 成员
func (r *GuildRepository) IsMember(guildID, userID uint) (bool, error) {
	// 尝试从 Redis 缓存检查
	if r.redis != nil {
		key := fmt.Sprintf("%s%d", guildMembersKeyPrefix, guildID)
		ctx := context.Background()

		// 使用 Pipeline 减少 RTT
		pipe := r.redis.Pipeline()
		existsCmd := pipe.Exists(ctx, key)
		isMemberCmd := pipe.SIsMember(ctx, key, userID)
		_, err := pipe.Exec(ctx)

		if err == nil {
			// 如果缓存存在，直接返回缓存结果
			if existsCmd.Val() > 0 {
				return isMemberCmd.Val(), nil
			}
		}
	}

	var count int64
	err := r.db.Model(&models.GuildMember{}).
		Where("guild_id = ? AND user_id = ?", guildID, userID).
		Count(&count).Error
	return count > 0, err
}

// CreateInvite 创建邀请码
func (r *GuildRepository) CreateInvite(invite *models.Invite) error {
	return r.db.Create(invite).Error
}

// GetInviteByCode 根据邀请码获取邀请信息
func (r *GuildRepository) GetInviteByCode(code string) (*models.Invite, error) {
	var invite models.Invite
	err := r.db.Where("code = ?", code).First(&invite).Error
	return &invite, err
}

// CreateMessage 创建消息
func (r *GuildRepository) CreateMessage(msg *models.Message) error {
	// 生成 SequenceID (如果 Redis 可用)
	if r.redis != nil {
		key := fmt.Sprintf("%s%d", guildMessageSeqKeyPrefix, msg.GuildID)

		// 确保 SequenceID 已初始化
		if err := r.ensureSequenceID(context.Background(), msg.GuildID, key); err != nil {
			// 记录错误但继续执行，Redis 会从 0 开始或使用现有值
			fmt.Printf("确保 sequence id 存在时出错: %v\n", err)
		}

		seq, err := r.redis.Incr(context.Background(), key).Result()
		if err != nil {
			fmt.Printf("生成 sequence id 失败: %v\n", err)
		} else {
			msg.SequenceID = seq
		}
	}

	// 保存到 PostgreSQL
	if err := r.db.Create(msg).Error; err != nil {
		return err
	}

	// 缓存到 Redis
	if r.redis != nil {
		go func() {
			ctx := context.Background()
			msgJSON, err := json.Marshal(msg)
			if err != nil {
				return
			}

			key := fmt.Sprintf("%s%d", recentMessagesKeyPrefix, msg.GuildID)
			member := redis.Z{
				Score:  float64(msg.SequenceID),
				Member: msgJSON,
			}
			pipe := r.redis.Pipeline()
			pipe.ZAdd(ctx, key, member)
			// 保留最近 1000 条
			pipe.ZRemRangeByRank(ctx, key, 0, -(recentMessagesCount + 1))
			pipe.Expire(ctx, key, 7*24*time.Hour)
			_, _ = pipe.Exec(ctx)
		}()
	}

	return nil
}

// GetGuildMessages 获取 Guild 的历史消息，按创建时间倒序排列，支持分页，并预加载发送者信息
func (r *GuildRepository) GetGuildMessages(guildID uint, limit, offset int) ([]models.Message, error) {
	// 仅当从头开始拉取且数量在缓存范围内时，才尝试从 Redis 读取
	if r.redis != nil && offset == 0 && limit <= recentMessagesCount {
		key := fmt.Sprintf("%s%d", recentMessagesKeyPrefix, guildID)
		// 从 ZSET 中按分数（时间戳）倒序获取消息
		results, err := r.redis.ZRevRange(context.Background(), key, 0, int64(limit-1)).Result()
		if err == nil && len(results) > 0 {
			var messages []models.Message
			for _, msgJSON := range results {
				var msg models.Message
				if json.Unmarshal([]byte(msgJSON), &msg) == nil {
					messages = append(messages, msg)
				}
			}
			// 缓存中的消息已经是最新优先，直接返回
			return messages, nil
		}
	}

	var messages []models.Message
	err := r.db.Where("guild_id = ?", guildID).
		Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Preload("Sender"). // 预加载发送者信息
		Find(&messages).Error
	return messages, err
}

// GetMessagesAfterSequence 获取指定 SequenceID 之后的消息 (增量同步)
func (r *GuildRepository) GetMessagesAfterSequence(guildID uint, afterSeq int64, limit int) ([]models.Message, error) {
	// 尝试从 Redis 获取 (利用 SequenceID 作为 Score)
	if r.redis != nil {
		key := fmt.Sprintf("%s%d", recentMessagesKeyPrefix, guildID)
		// ZRangeByScore: (afterSeq, +inf]
		op := redis.ZRangeBy{
			Min:    fmt.Sprintf("(%d", afterSeq), // "(" 表示开区间，即 > afterSeq
			Max:    "+inf",
			Offset: 0,
			Count:  int64(limit),
		}
		results, err := r.redis.ZRangeByScore(context.Background(), key, &op).Result()

		if err == nil && len(results) > 0 {
			// 检查 Redis 里的数据是否满足连续性要求
			// 获取 Redis 中最小的 Score (即最早的消息 Seq)
			minScoreData, errMin := r.redis.ZRangeWithScores(context.Background(), key, 0, 0).Result()
			if errMin == nil && len(minScoreData) > 0 {
				minSeq := int64(minScoreData[0].Score)
				// 如果请求的 afterSeq >= minSeq - 1，说明 Redis 里的数据是完整的后续部分
				if afterSeq >= minSeq-1 {
					var messages []models.Message
					for _, msgJSON := range results {
						var msg models.Message
						if json.Unmarshal([]byte(msgJSON), &msg) == nil {
							messages = append(messages, msg)
						}
					}
					return messages, nil
				}
			}
		}
	}

	var messages []models.Message
	// 增量拉取按 SequenceID 正序排列 (旧 -> 新)
	err := r.db.Where("guild_id = ? AND sequence_id > ?", guildID, afterSeq).
		Order("sequence_id asc").
		Limit(limit).
		Preload("Sender").
		Find(&messages).Error
	return messages, err
}

// ensureSequenceID 确保 Redis 中的 SequenceID 已根据数据库中的最大值初始化
func (r *GuildRepository) ensureSequenceID(ctx context.Context, guildID uint, key string) error {
	// 检查 Key 是否存在
	exists, err := r.redis.Exists(ctx, key).Result()
	if err != nil {
		return err
	}

	if exists > 0 {
		return nil
	}

	// Key 不存在，查询数据库最大 SequenceID
	var maxSeq int64
	err = r.db.Model(&models.Message{}).
		Where("guild_id = ?", guildID).
		Select("COALESCE(MAX(sequence_id), 0)").
		Scan(&maxSeq).Error
	if err != nil {
		return err
	}

	// 使用 SetNX 初始化，防止并发覆盖
	_, err = r.redis.SetNX(ctx, key, maxSeq, 0).Result()
	return err
}

// GetUserGuildIDs 获取用户加入的所有 Guild ID
func (r *GuildRepository) GetUserGuildIDs(userID uint) ([]uint, error) {
	var guildIDs []uint
	err := r.db.Model(&models.GuildMember{}).
		Where("user_id = ?", userID).
		Pluck("guild_id", &guildIDs).Error
	return guildIDs, err
}

// GetGuildsByUserID 获取用户加入的所有 Guild 详情
func (r *GuildRepository) GetGuildsByUserID(userID uint) ([]models.Guild, error) {
	var guilds []models.Guild
	// 联表查询：通过 members 表连接获取 guilds
	// 注意：GuildMember 模型的 TableName 是 "members"
	err := r.db.Joins("JOIN members ON members.guild_id = guilds.id").
		Where("members.user_id = ?", userID).
		Find(&guilds).Error
	return guilds, err
}

// GetGuildMemberIDs 获取 Guild 的所有成员 ID
func (r *GuildRepository) GetGuildMemberIDs(guildID uint) ([]uint, error) {
	// 尝试从 Redis 获取
	if r.redis != nil {
		key := fmt.Sprintf("%s%d", guildMembersKeyPrefix, guildID)
		members, err := r.redis.SMembers(context.Background(), key).Result()
		if err == nil && len(members) > 0 {
			userIDs := make([]uint, len(members))
			for i, member := range members {
				id, _ := strconv.ParseUint(member, 10, 64)
				userIDs[i] = uint(id)
			}
			return userIDs, nil
		}
	}

	// 从数据库获取
	var userIDs []uint
	err := r.db.Model(&models.GuildMember{}).
		Where("guild_id = ?", guildID).
		Pluck("user_id", &userIDs).Error
	if err != nil {
		return nil, err
	}

	// 回填 Redis
	if r.redis != nil && len(userIDs) > 0 {
		key := fmt.Sprintf("%s%d", guildMembersKeyPrefix, guildID)
		ctx := context.Background()
		pipe := r.redis.Pipeline()

		members := make([]any, len(userIDs))
		for i, id := range userIDs {
			members[i] = id
		}

		pipe.SAdd(ctx, key, members...)
		pipe.Expire(ctx, key, 24*time.Hour) // 设置 24 小时过期，避免永久占用
		pipe.Exec(ctx)
	}

	return userIDs, err
}

// SetUserOnline 标记用户在指定 Guild 在线
func (r *GuildRepository) SetUserOnline(guildID, userID uint) {
	if r.redis == nil {
		return
	}

	key := fmt.Sprintf("%s%d", onlineUserKeyPrefix, guildID)
	// Score = 当前时间戳 + TTL (例如 5 分钟)
	expireAt := float64(time.Now().Add(5 * time.Minute).Unix())
	r.redis.ZAdd(context.Background(), key, redis.Z{Score: expireAt, Member: userID})
}

// SetUserOffline 标记用户在指定 Guild 离线
func (r *GuildRepository) SetUserOffline(guildID, userID uint) {
	if r.redis == nil {
		return
	}

	key := fmt.Sprintf("%s%d", onlineUserKeyPrefix, guildID)
	r.redis.ZRem(context.Background(), key, userID)
}

// GetOnlineUserIDsInGuild 获取 Guild 中的所有在线用户 ID
func (r *GuildRepository) GetOnlineUserIDsInGuild(guildID uint) ([]uint, error) {
	if r.redis == nil {
		return []uint{}, nil
	}

	key := fmt.Sprintf("%s%d", onlineUserKeyPrefix, guildID)
	ctx := context.Background()

	// 清理已过期的用户 (Score < 当前时间)
	now := float64(time.Now().Unix())
	if err := r.redis.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%f", now)).Err(); err != nil {
		fmt.Printf("清理过期在线用户失败: %v\n", err)
	}

	// 获取剩余的在线用户
	members, err := r.redis.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	userIDs := make([]uint, len(members))
	for i, member := range members {
		id, _ := strconv.ParseUint(member, 10, 64)
		userIDs[i] = uint(id)
	}
	return userIDs, nil
}

// RefreshUserOnline 刷新用户的在线状态 (心跳)
func (r *GuildRepository) RefreshUserOnline(guildID, userID uint) {
	if r.redis == nil {
		return
	}

	key := fmt.Sprintf("%s%d", onlineUserKeyPrefix, guildID)
	expireAt := float64(time.Now().Add(5 * time.Minute).Unix())
	// ZAdd 如果成员已存在，会更新 Score
	r.redis.ZAdd(context.Background(), key, redis.Z{Score: expireAt, Member: userID})
}

// PushToInbox 将消息推送到用户的收件箱
func (r *GuildRepository) PushToInbox(userID uint, msg *models.Message) error {
	if r.redis == nil {
		return nil
	}

	key := fmt.Sprintf("%s%d", userInboxKeyPrefix, userID)
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	// 使用 LPUSH，让新消息在列表头部
	return r.redis.LPush(context.Background(), key, msgJSON).Err()
}

// BatchPushToInbox 批量将消息推送到多个用户的收件箱
func (r *GuildRepository) BatchPushToInbox(userIDs []uint, msg *models.Message) error {
	if r.redis == nil || len(userIDs) == 0 {
		return nil
	}

	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	ctx := context.Background()
	pipe := r.redis.Pipeline()
	for _, userID := range userIDs {
		key := fmt.Sprintf("%s%d", userInboxKeyPrefix, userID)
		pipe.LPush(ctx, key, msgJSON)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// GetAndClearInbox 原子地获取并清空用户的所有收件箱消息
func (r *GuildRepository) GetAndClearInbox(userID uint) ([]models.Message, error) {
	if r.redis == nil {
		return nil, nil
	}

	key := fmt.Sprintf("%s%d", userInboxKeyPrefix, userID)
	ctx := context.Background()

	// 使用管道原子化 "获取全部" 和 "删除" 操作
	pipe := r.redis.Pipeline()
	listCmd := pipe.LRange(ctx, key, 0, -1)
	pipe.Del(ctx, key)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	results, err := listCmd.Result()
	if err != nil {
		return nil, err
	}

	var messages []models.Message
	for _, msgJSON := range results {
		var msg models.Message
		if json.Unmarshal([]byte(msgJSON), &msg) == nil {
			messages = append(messages, msg)
		}
	}
	// LRange 获取的是从头到尾 (新->旧)，反转切片得到时间正序 (旧->新)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

// UpdateLastRead 更新用户在某群组的最后读取消息ID
func (r *GuildRepository) UpdateLastRead(guildID, userID uint, msgID int64) error {
	return r.db.Model(&models.GuildMember{}).
		Where("guild_id = ? AND user_id = ?", guildID, userID).
		Update("last_read_msg_id", msgID).Error
}

// GetMaxSequenceID 获取群组当前最大消息 SequenceID
func (r *GuildRepository) GetMaxSequenceID(guildID uint) (int64, error) {
	if r.redis != nil {
		key := fmt.Sprintf("%s%d", guildMessageSeqKeyPrefix, guildID)
		val, err := r.redis.Get(context.Background(), key).Int64()
		if err == nil {
			return val, nil
		}
	}
	// Fallback to DB
	var maxSeq int64
	err := r.db.Model(&models.Message{}).
		Where("guild_id = ?", guildID).
		Select("COALESCE(MAX(sequence_id), 0)").
		Scan(&maxSeq).Error
	return maxSeq, err
}

type GuildMemberResult struct {
	ID            uint      `json:"id"`
	Topic         string    `json:"topic"`
	OwnerID       uint      `json:"owner_id"`
	CreatedAt     time.Time `json:"created_at"`
	LastReadMsgID int64     `json:"last_read_msg_id"`
}

// GetGuildsAndMemberInfoByUserID 获取用户加入的所有 Guild 详情及成员信息
func (r *GuildRepository) GetGuildsAndMemberInfoByUserID(userID uint) ([]GuildMemberResult, error) {
	var results []GuildMemberResult
	err := r.db.Table("guilds").
		Joins("JOIN members ON members.guild_id = guilds.id").
		Where("members.user_id = ?", userID).
		Select("guilds.id, guilds.topic, guilds.owner_id, guilds.created_at, members.last_read_msg_id").
		Scan(&results).Error
	return results, err
}
