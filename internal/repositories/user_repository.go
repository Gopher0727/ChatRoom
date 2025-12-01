package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/Gopher0727/ChatRoom/internal/models"
)

const (
	userCacheKeyPrefix = "user:info:" // Redis String, 值是 user JSON
	userCacheTTL       = 1 * time.Hour
)

type UserRepository struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewUserRepository(db *gorm.DB, redis *redis.Client) *UserRepository {
	return &UserRepository{db: db, redis: redis}
}

// Create 创建用户
func (r *UserRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

// GetByID 根据 ID 获取用户 (带缓存)
func (r *UserRepository) GetByID(id uint) (*models.User, error) {
	// 尝试从 Redis 获取
	if r.redis != nil {
		key := fmt.Sprintf("%s%d", userCacheKeyPrefix, id)
		val, err := r.redis.Get(context.Background(), key).Result()
		if err == nil {
			var user models.User
			if json.Unmarshal([]byte(val), &user) == nil {
				return &user, nil
			}
		}
	}

	// 从数据库获取
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}

	// 回填 Redis
	if r.redis != nil {
		key := fmt.Sprintf("%s%d", userCacheKeyPrefix, id)
		if data, err := json.Marshal(&user); err == nil {
			r.redis.Set(context.Background(), key, data, userCacheTTL)
		}
	}

	return &user, nil
}

// GetByUserName 根据用户名获取用户
func (r *UserRepository) GetByUserName(username string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByEmail 根据邮箱获取用户
func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// ExistsByUserName 检查用户名是否存在
func (r *UserRepository) ExistsByUserName(username string) (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

// ExistsByEmail 检查邮箱是否存在
func (r *UserRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

// Update 更新用户 (同时清除缓存)
func (r *UserRepository) Update(user *models.User) error {
	if err := r.db.Save(user).Error; err != nil {
		return err
	}

	if r.redis != nil {
		key := fmt.Sprintf("%s%d", userCacheKeyPrefix, user.ID)
		r.redis.Del(context.Background(), key)
	}
	return nil
}

// UpdateStatus 更新用户状态 (同时清除缓存)
func (r *UserRepository) UpdateStatus(id uint, status string) error {
	if err := r.db.Model(&models.User{}).Where("id = ?", id).Update("status", status).Error; err != nil {
		return err
	}

	if r.redis != nil {
		key := fmt.Sprintf("%s%d", userCacheKeyPrefix, id)
		r.redis.Del(context.Background(), key)
	}
	return nil
}

// Delete 删除用户 (同时清除缓存)
func (r *UserRepository) Delete(id uint) error {
	if err := r.db.Delete(&models.User{}, id).Error; err != nil {
		return err
	}

	if r.redis != nil {
		key := fmt.Sprintf("%s%d", userCacheKeyPrefix, id)
		r.redis.Del(context.Background(), key)
	}
	return nil
}

// List 获取用户列表
func (r *UserRepository) List(limit, offset int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	err := r.db.Model(&models.User{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = r.db.Limit(limit).Offset(offset).Find(&users).Error
	return users, total, err
}

// GetByIDs 批量获取用户信息 (带缓存)
func (r *UserRepository) GetByIDs(ids []uint) (map[uint]*models.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	result := make(map[uint]*models.User)
	var missingIDs []uint

	// 尝试从 Redis 批量获取
	if r.redis != nil {
		keys := make([]string, len(ids))
		for i, id := range ids {
			keys[i] = fmt.Sprintf("%s%d", userCacheKeyPrefix, id)
		}

		// MGet 一次性获取所有 key
		vals, err := r.redis.MGet(context.Background(), keys...).Result()
		if err == nil {
			for i, val := range vals {
				if valStr, ok := val.(string); ok {
					var user models.User
					if json.Unmarshal([]byte(valStr), &user) == nil {
						result[ids[i]] = &user
					} else {
						missingIDs = append(missingIDs, ids[i])
					}
				} else {
					missingIDs = append(missingIDs, ids[i])
				}
			}
		} else {
			missingIDs = ids // Redis 失败，全部查 DB
		}
	} else {
		missingIDs = ids
	}

	// 从数据库获取缺失的用户
	if len(missingIDs) > 0 {
		var users []models.User
		if err := r.db.Where("id IN ?", missingIDs).Find(&users).Error; err != nil {
			return result, err // 返回已获取的部分
		}

		// 回填 Redis 并更新结果集
		if r.redis != nil {
			ctx := context.Background()
			pipe := r.redis.Pipeline()
			for _, user := range users {
				u := user // 避免闭包问题
				result[u.ID] = &u

				key := fmt.Sprintf("%s%d", userCacheKeyPrefix, u.ID)
				if data, err := json.Marshal(&u); err == nil {
					pipe.Set(ctx, key, data, userCacheTTL)
				}
			}
			pipe.Exec(ctx)
		} else {
			for _, user := range users {
				u := user
				result[u.ID] = &u
			}
		}
	}

	return result, nil
}
