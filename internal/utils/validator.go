package utils

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword 使用 bcrypt 对密码进行哈希
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// ValidateUserName 验证用户名格式（3-20个字符，字母数字下划线）
func ValidateUserName(username string) bool {
	if len(username) < 3 || len(username) > 20 {
		return false
	}
	pattern := `^[a-zA-Z0-9_]+$`
	re := regexp.MustCompile(pattern)
	return re.MatchString(username)
}

// CheckPassword 验证密码
func CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// ValidatePassword 验证密码强度（至少8个字符）
func ValidatePassword(password string) bool {
	return len(password) >= 8
}

// ValidateEmail 验证邮箱格式
func ValidateEmail(email string) bool {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	re := regexp.MustCompile(pattern)
	return re.MatchString(email)
}

// GenerateInviteCode 生成邀请码（6位随机字符串）
func GenerateInviteCode() string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d%d", time.Now().Unix(), time.Now().Nanosecond())))
	code := fmt.Sprintf("%x", hash)
	return strings.ToUpper(code[:6])
}
