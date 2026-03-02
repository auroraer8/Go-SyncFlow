package middleware

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

var (
	jwtSecret     []byte
	jwtSecretOnce sync.Once

	// 用户密码修改后，该用户的 token 最早签发时间（之前签发的全部作废）
	tokenMinIssuedAt   = make(map[uint]time.Time)
	tokenMinIssuedAtMu sync.RWMutex
)

// getJWTSecret 获取JWT密钥（优先使用环境变量，否则使用持久化密钥）
func getJWTSecret() []byte {
	jwtSecretOnce.Do(func() {
		// 1. 优先从环境变量获取
		if secret := os.Getenv("JWT_SECRET"); secret != "" {
			jwtSecret = []byte(secret)
			return
		}

		// 2. 从文件读取持久化密钥
		secretFile := "./data/jwt_secret"
		if data, err := os.ReadFile(secretFile); err == nil && len(data) >= 32 {
			jwtSecret = data
			return
		}

		// 3. 生成新的随机密钥并保存
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			// 降级使用固定密钥（不推荐）
			jwtSecret = []byte("fallback-secret-please-set-env")
			return
		}

		// 保存到文件
		os.MkdirAll("./data", 0755)
		os.WriteFile(secretFile, secret, 0600)
		jwtSecret = secret
	})
	return jwtSecret
}

// GetJWTSecretHex 获取JWT密钥的十六进制表示（用于调试显示，只显示前8位）
func GetJWTSecretHex() string {
	secret := getJWTSecret()
	if len(secret) > 4 {
		return hex.EncodeToString(secret[:4]) + "..."
	}
	return "***"
}

type Claims struct {
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateToken(userID uint, username string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}

func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return getJWTSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		// 支持从 Cookie 读取 token（用于浏览器跳转类协议端点）
		if authHeader == "" {
			if t, err := c.Cookie("gsf_token"); err == nil && t != "" {
				authHeader = "Bearer " + t
			}
		}
		// 支持 query 参数传 token（用于文件下载等场景）
		if authHeader == "" {
			if t := c.Query("token"); t != "" {
				authHeader = "Bearer " + t
			}
		}
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Token格式错误"})
			c.Abort()
			return
		}

		claims, err := ParseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Token无效或已过期"})
			c.Abort()
			return
		}

		// 检查 token 是否因密码修改而被撤销
		if claims.IssuedAt != nil && isTokenRevoked(claims.UserID, claims.IssuedAt.Time) {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "密码已修改，请重新登录"})
			c.Abort()
			return
		}

		tokenStr := parts[1]

		// 检查会话是否被管理员终止，并自动追踪会话
		if storage.DB != nil {
			tokenHash := HashToken(tokenStr)
			var session models.Session
			err := storage.DB.Where("access_token = ?", tokenHash).First(&session).Error
			if err == nil {
				// session 存在，检查是否被终止
				if !session.IsActive {
					c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "会话已被终止，请重新登录"})
					c.Abort()
					return
				}
				// 更新最后活动时间（每5分钟更新一次，避免频繁写库）
				if time.Since(session.LastActivity) > 5*time.Minute {
					storage.DB.Model(&session).Update("last_activity", time.Now())
				}
			} else {
				// session 不存在，自动创建（兼容之前没有 session 的登录）
				newSession := models.Session{
					ID:           generateSessionID(),
					UserID:       claims.UserID,
					AccessToken:  tokenHash,
					RefreshToken: "",
					IPAddress:    c.ClientIP(),
					UserAgent:    c.GetHeader("User-Agent"),
					IsActive:     true,
					LastActivity: time.Now(),
					ExpiresAt:    claims.ExpiresAt.Time,
					CreatedAt:    time.Now(),
				}
				storage.DB.Create(&newSession)
			}
		}

		c.Set("userId", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// HashToken 对 token 取 SHA256 摘要存储，避免明文存储
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// generateSessionID 生成会话ID
func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// InvalidateUserTokens 使某用户在此时间点之前签发的所有 token 失效
func InvalidateUserTokens(userID uint) {
	tokenMinIssuedAtMu.Lock()
	defer tokenMinIssuedAtMu.Unlock()
	tokenMinIssuedAt[userID] = time.Now()
}

// isTokenRevoked 检查 token 是否已被撤销（签发时间早于密码修改时间）
func isTokenRevoked(userID uint, issuedAt time.Time) bool {
	tokenMinIssuedAtMu.RLock()
	defer tokenMinIssuedAtMu.RUnlock()
	if minTime, ok := tokenMinIssuedAt[userID]; ok {
		return issuedAt.Before(minTime)
	}
	return false
}

func GetUserID(c *gin.Context) uint {
	if v, exists := c.Get("userId"); exists {
		return v.(uint)
	}
	return 0
}

func GetUsername(c *gin.Context) string {
	if v, exists := c.Get("username"); exists {
		return v.(string)
	}
	return ""
}
