package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/services"
	"go-syncflow/internal/storage"
)

// ========== 双因素认证（2FA）配置 ==========

// TwoFactorConfig 双因素认证配置
type TwoFactorConfig struct {
	Enabled         bool   `json:"enabled"`          // 是否启用 2FA
	AdminPhone      string `json:"admin_phone"`      // 管理员手机号（必须）
	VerifyMethod    string `json:"verify_method"`    // 验证方式: sms
	MessageScene    string `json:"message_scene"`    // 消息场景
	ExemptRoles     []uint `json:"exempt_roles"`     // 豁免的角色 ID 列表
	ExemptWhiteIP   bool   `json:"exempt_white_ip"`  // 白名单 IP 是否豁免
	CodeExpireMins  int    `json:"code_expire_mins"` // 验证码过期时间（分钟）
	MaxRetry        int    `json:"max_retry"`        // 最大重试次数
	LockoutMins     int    `json:"lockout_mins"`     // 锁定时间（分钟）
}

// DefaultTwoFactorConfig 默认 2FA 配置
func DefaultTwoFactorConfig() *TwoFactorConfig {
	return &TwoFactorConfig{
		Enabled:        false,
		VerifyMethod:   "sms",
		MessageScene:   "login_2fa",
		ExemptWhiteIP:  true,
		CodeExpireMins: 5,
		MaxRetry:       5,
		LockoutMins:    30,
	}
}

const TwoFactorConfigKey = "two_factor_auth"

// GetTwoFactorConfig 获取 2FA 配置
func GetTwoFactorConfig() *TwoFactorConfig {
	cfgStr, err := storage.GetSecurityConfig(TwoFactorConfigKey)
	if err != nil {
		return DefaultTwoFactorConfig()
	}

	var cfg TwoFactorConfig
	if err := json.Unmarshal([]byte(cfgStr), &cfg); err != nil {
		return DefaultTwoFactorConfig()
	}

	return &cfg
}

// SetTwoFactorConfig 保存 2FA 配置
func SetTwoFactorConfig(cfg *TwoFactorConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return storage.SetSecurityConfig(TwoFactorConfigKey, string(data), nil)
}

// ========== 登录 2FA 验证流程 ==========

// 2FA 登录会话（用户已通过密码验证，等待 2FA 验证）
var (
	pending2FASessions     = make(map[string]*Pending2FASession)
	pending2FASessionsLock sync.Mutex
)

// Pending2FASession 待验证的 2FA 会话
type Pending2FASession struct {
	UserID      uint
	Username    string
	Phone       string
	ClientIP    string
	UserAgent   string
	Code        string
	ExpiresAt   time.Time
	RetryCount  int
	CreatedAt   time.Time
}

// generatePending2FAToken 生成待验证 2FA 的临时 Token
func generatePending2FAToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// create2FASession 创建 2FA 会话
func create2FASession(userID uint, username, phone, clientIP, userAgent string) (string, string) {
	pending2FASessionsLock.Lock()
	defer pending2FASessionsLock.Unlock()

	// 清理过期会话
	now := time.Now()
	for k, v := range pending2FASessions {
		if now.After(v.ExpiresAt) {
			delete(pending2FASessions, k)
		}
	}

	cfg := GetTwoFactorConfig()
	code := generateVerifyCode()
	token := generatePending2FAToken()

	pending2FASessions[token] = &Pending2FASession{
		UserID:    userID,
		Username:  username,
		Phone:     phone,
		ClientIP:  clientIP,
		UserAgent: userAgent,
		Code:      code,
		ExpiresAt: now.Add(time.Duration(cfg.CodeExpireMins) * time.Minute),
		CreatedAt: now,
	}

	return token, code
}

// verify2FASession 验证 2FA 会话
func verify2FASession(token, code string) (*Pending2FASession, bool, string) {
	pending2FASessionsLock.Lock()
	defer pending2FASessionsLock.Unlock()

	session, ok := pending2FASessions[token]
	if !ok {
		return nil, false, "会话已过期，请重新登录"
	}

	if time.Now().After(session.ExpiresAt) {
		delete(pending2FASessions, token)
		return nil, false, "验证码已过期，请重新获取"
	}

	cfg := GetTwoFactorConfig()
	if session.RetryCount >= cfg.MaxRetry {
		delete(pending2FASessions, token)
		return nil, false, "验证码错误次数过多，请重新登录"
	}

	if session.Code != code {
		session.RetryCount++
		remaining := cfg.MaxRetry - session.RetryCount
		return nil, false, fmt.Sprintf("验证码错误（剩余 %d 次）", remaining)
	}

	// 验证成功，删除会话
	delete(pending2FASessions, token)
	return session, true, ""
}

// consume2FASession 消费 2FA 会话（验证成功后调用）
func consume2FASession(token string) {
	pending2FASessionsLock.Lock()
	defer pending2FASessionsLock.Unlock()
	delete(pending2FASessions, token)
}

// resend2FACode 重新发送验证码
func resend2FACode(token string) (string, error) {
	pending2FASessionsLock.Lock()
	defer pending2FASessionsLock.Unlock()

	session, ok := pending2FASessions[token]
	if !ok {
		return "", fmt.Errorf("会话已过期")
	}

	cfg := GetTwoFactorConfig()
	code := generateVerifyCode()
	session.Code = code
	session.ExpiresAt = time.Now().Add(time.Duration(cfg.CodeExpireMins) * time.Minute)

	return code, nil
}

// ========== 2FA API 处理器 ==========

// Get2FAConfig 获取 2FA 配置（管理接口）
func Get2FAConfig(c *gin.Context) {
	cfg := GetTwoFactorConfig()
	respondOK(c, cfg)
}

// Update2FAConfig 更新 2FA 配置（管理接口）
func Update2FAConfig(c *gin.Context) {
	var cfg TwoFactorConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 如果启用 2FA，必须设置管理员手机号
	if cfg.Enabled && cfg.AdminPhone == "" {
		respondError(c, http.StatusBadRequest, "启用双因素认证时必须设置管理员手机号")
		return
	}

	// 检查短信通道是否可用
	if cfg.Enabled && !services.IsSMSChannelAvailable() {
		respondError(c, http.StatusBadRequest, "短信通道未配置，无法启用双因素认证")
		return
	}

	// 设置默认值
	if cfg.CodeExpireMins == 0 {
		cfg.CodeExpireMins = 5
	}
	if cfg.MaxRetry == 0 {
		cfg.MaxRetry = 5
	}
	if cfg.LockoutMins == 0 {
		cfg.LockoutMins = 30
	}

	if err := SetTwoFactorConfig(&cfg); err != nil {
		respondError(c, http.StatusInternalServerError, "保存配置失败")
		return
	}

	middleware.RecordOperationLog(c, "安全设置", "更新双因素认证配置",
		fmt.Sprintf("启用: %v, 管理员手机: %s", cfg.Enabled, maskPhone(cfg.AdminPhone)), "")

	respondOK(c, gin.H{"message": "配置已保存"})
}

// Test2FAConfig 测试 2FA 配置
func Test2FAConfig(c *gin.Context) {
	cfg := GetTwoFactorConfig()

	if cfg.AdminPhone == "" {
		respondError(c, http.StatusBadRequest, "未设置管理员手机号")
		return
	}

	if !services.IsSMSChannelAvailable() {
		respondError(c, http.StatusBadRequest, "短信通道未配置")
		return
	}

	// 发送测试验证码
	code := generateVerifyCode()
	_, err := services.SendVerifyCodeSMS(cfg.AdminPhone, code, "admin", "测试")
	if err != nil {
		respondError(c, http.StatusInternalServerError, "发送失败: "+err.Error())
		return
	}

	respondOK(c, gin.H{"message": "测试短信已发送到 " + maskPhone(cfg.AdminPhone)})
}

// ========== 登录流程中的 2FA 处理 ==========

// Check2FARequired 检查用户是否需要 2FA
func Check2FARequired(user *models.User, clientIP string) bool {
	cfg := GetTwoFactorConfig()

	// 未启用
	if !cfg.Enabled {
		return false
	}

	// 白名单 IP 豁免
	if cfg.ExemptWhiteIP {
		var whiteCount int64
		storage.DB.Model(&models.IPWhitelist{}).Where("ip_address = ? AND is_active = ?", clientIP, true).Count(&whiteCount)
		if whiteCount > 0 {
			return false
		}
	}

	// 豁免角色
	if len(cfg.ExemptRoles) > 0 {
		var userRoles []models.UserRole
		storage.DB.Where("user_id = ?", user.ID).Find(&userRoles)
		for _, ur := range userRoles {
			for _, exemptRole := range cfg.ExemptRoles {
				if ur.RoleID == exemptRole {
					return false
				}
			}
		}
	}

	// 用户必须有手机号
	if user.Phone == "" {
		return false
	}

	return true
}

// Send2FACode 发送 2FA 验证码（登录第一步成功后调用）
func Send2FACode(c *gin.Context) {
	var req struct {
		PendingToken string `json:"pending_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	pending2FASessionsLock.Lock()
	session, ok := pending2FASessions[req.PendingToken]
	pending2FASessionsLock.Unlock()

	if !ok {
		respondError(c, http.StatusBadRequest, "会话已过期，请重新登录")
		return
	}

	// 重新生成验证码
	code, err := resend2FACode(req.PendingToken)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// 发送验证码
	result, err := services.SendVerifyCodeSMS(session.Phone, code, session.Username, "")
	if err != nil {
		log.Printf("[2FA] 验证码发送失败, 用户 %s: %v", session.Username, err)
		respondError(c, http.StatusInternalServerError, "验证码发送失败")
		return
	}
	_ = result // 用于重发时不需要返回通道信息

	respondOK(c, gin.H{
		"message":      "验证码已发送",
		"masked_phone": maskPhone(session.Phone),
	})
}

// Verify2FACode 验证 2FA 验证码（登录第二步）
func Verify2FACode(c *gin.Context) {
	var req struct {
		PendingToken string `json:"pending_token" binding:"required"`
		Code         string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	session, success, errMsg := verify2FASession(req.PendingToken, req.Code)
	if !success {
		respondError(c, http.StatusBadRequest, errMsg)
		return
	}

	// 验证成功，生成真正的登录 Token
	token, err := middleware.GenerateToken(session.UserID, session.Username)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "生成Token失败")
		return
	}

	setAuthCookie(c, token)

	// 记录登录成功
	ss := services.GetSecurityService()
	ss.RecordLoginAttempt(session.Username, &session.UserID, session.ClientIP, session.UserAgent, true, "")
	ss.HandleSuccessfulLogin(session.UserID, session.ClientIP)
	ss.RecordSecurityEvent(models.EventLoginSuccess, models.SeverityLow, session.ClientIP, &session.UserID, session.Username,
		"login", "", "用户通过双因素认证登录成功", nil)
	middleware.RecordLoginLog(session.UserID, session.Username, session.ClientIP, session.UserAgent, true, "登录成功(2FA)")

	// 获取用户信息
	var user models.User
	storage.DB.First(&user, session.UserID)

	respondOK(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":                  user.ID,
			"username":            user.Username,
			"nickname":            user.Nickname,
			"avatar":              user.Avatar,
			"forcePasswordChange": user.ForcePasswordChange,
		},
	})
}

// ========== 初始化 2FA 默认配置 ==========

// Init2FAConfig 初始化 2FA 配置
func Init2FAConfig() error {
	_, err := storage.GetSecurityConfig(TwoFactorConfigKey)
	if err != nil {
		// 不存在，创建默认配置
		cfg := DefaultTwoFactorConfig()
		return SetTwoFactorConfig(cfg)
	}
	return nil
}

