/**
  @author: Hanhai
  @desc: JWT认证相关功能模块，提供token生成、解析和密码哈希等功能
**/

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flowsilicon/internal/logger"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	// 默认的密钥
	secretKey = []byte("flowsilicon_default_secret_key")
)

// GenerateToken 生成简单的认证Token
// 格式: timestamp.expiration.signature
// timestamp: 当前时间戳
// expiration: 过期时间（分钟）
// signature: HMAC-SHA256(timestamp.expiration, secretKey)
func GenerateToken(expirationMinutes int) (string, error) {
	now := time.Now().Unix()

	// 确保有一个最小的过期时间（1分钟）
	if expirationMinutes <= 0 {
		expirationMinutes = 1
	}

	expiration := now + int64(expirationMinutes*60)

	data := fmt.Sprintf("%d.%d", now, expiration)

	// 计算签名
	h := hmac.New(sha256.New, secretKey)
	h.Write([]byte(data))
	signature := hex.EncodeToString(h.Sum(nil))

	// 生成完整token
	token := fmt.Sprintf("%s.%s", data, signature)
	return token, nil
}

// ParseToken 解析令牌
func ParseToken(tokenString string) (bool, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return false, errors.New("invalid token format")
	}

	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false, err
	}

	expiration, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return false, err
	}

	// 检查token是否过期
	now := time.Now().Unix()
	if now > expiration {
		return false, errors.New("token expired")
	}

	// 验证签名
	data := fmt.Sprintf("%d.%d", timestamp, expiration)
	h := hmac.New(sha256.New, secretKey)
	h.Write([]byte(data))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	if parts[2] != expectedSignature {
		return false, errors.New("invalid signature")
	}

	return true, nil
}

// HashPassword 使用SHA256对密码进行哈希
func HashPassword(password string) string {
	if password == "" {
		return ""
	}

	// 计算SHA256哈希
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// VerifyPassword 验证密码
func VerifyPassword(inputPassword, storedPassword string) bool {
	// 如果存储的密码为空，则不需要验证
	if storedPassword == "" {
		return true
	}

	// 对输入密码进行哈希处理
	hashedInput := HashPassword(inputPassword)

	// 对比哈希值
	return hashedInput == storedPassword
}

// GenerateCookie 生成包含Token的Cookie值
func GenerateCookie(expirationMinutes int) (string, error) {
	// 生成令牌
	tokenString, err := GenerateToken(expirationMinutes)
	if err != nil {
		logger.Error("生成令牌失败: %v", err)
		return "", err
	}

	// 简单编码
	cookieValue := base64.StdEncoding.EncodeToString([]byte(tokenString))
	return cookieValue, nil
}

// ParseCookie 解析Cookie中的Token
func ParseCookie(cookieValue string) (bool, error) {
	if cookieValue == "" {
		return false, errors.New("空Cookie值")
	}

	// 解码Cookie值
	tokenBytes, err := base64.StdEncoding.DecodeString(cookieValue)
	if err != nil {
		logger.Error("解码Cookie值失败: %v", err)
		return false, err
	}

	// 解析令牌
	return ParseToken(string(tokenBytes))
}
