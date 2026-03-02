package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"go-syncflow/internal/crypto"
	"go-syncflow/internal/dingtalk"
	"go-syncflow/internal/ldapserver"
	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/services"
	"go-syncflow/internal/storage"
	syncer "go-syncflow/internal/sync"
)

// decodeRawPassword 解密前端 RSA 加密的明文密码（用于 AD 同步）
func decodeRawPassword(encrypted string) string {
	if encrypted == "" {
		log.Printf("[密码] RSA解密跳过: 输入为空")
		return ""
	}
	plain, err := services.RSADecrypt(encrypted)
	if err != nil {
		log.Printf("[密码] RSA解密失败: %v (输入长度=%d)", err, len(encrypted))
		return ""
	}
	log.Printf("[密码] RSA解密成功, 明文长度=%d", len(plain))
	return plain
}

// GetRSAPublicKey 返回 RSA 公钥（前端用于加密明文密码）
func GetRSAPublicKey(c *gin.Context) {
	respondOK(c, gin.H{
		"publicKey": services.GetRSAPublicKeyPEM(),
	})
}

// GetRSACryptoConfig 获取 RSA 加密配置（管理员）
func GetRSACryptoConfig(c *gin.Context) {
	cfg := services.GetRSACryptoConfig()
	respondOK(c, cfg)
}

// UpdateRSACryptoConfig 更新 RSA 密钥来源（管理员）
func UpdateRSACryptoConfig(c *gin.Context) {
	var req struct {
		Source     string `json:"source" binding:"required"` // "auto" | "custom" | "https"
		PrivateKey string `json:"privateKey"`                // PEM 格式私钥（custom 模式）
		PublicKey  string `json:"publicKey"`                 // PEM 格式公钥（custom 模式，可选）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	if err := services.SetRSACryptoSource(req.Source, req.PrivateKey, req.PublicKey); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	middleware.RecordOperationLog(c, "安全设置", "更新RSA密钥配置", fmt.Sprintf("来源: %s", req.Source), "")
	respondOK(c, gin.H{"message": "RSA密钥配置已更新"})
}

// hashSHA256 计算字符串的SHA256哈希
func hashSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ========== 登录 CSRF 令牌保护 ==========
// 登录接口必须先获取一次性 CSRF Token，防止脚本/curl 直接调用
var (
	csrfTokenStore  = make(map[string]time.Time) // token -> 创建时间
	csrfTokenMu     sync.Mutex
	csrfTokenMaxAge = 5 * time.Minute // Token 有效期 5 分钟
)

// generateCSRFToken 生成一次性 CSRF Token
func generateCSRFToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	token := hex.EncodeToString(b)

	csrfTokenMu.Lock()
	defer csrfTokenMu.Unlock()

	// 清理过期 token（防止内存泄漏）
	now := time.Now()
	for k, v := range csrfTokenStore {
		if now.Sub(v) > csrfTokenMaxAge {
			delete(csrfTokenStore, k)
		}
	}

	csrfTokenStore[token] = now
	return token
}

// validateCSRFToken 验证并消费一次性 CSRF Token
func validateCSRFToken(token string) bool {
	if token == "" {
		return false
	}

	csrfTokenMu.Lock()
	defer csrfTokenMu.Unlock()

	created, ok := csrfTokenStore[token]
	if !ok {
		return false
	}

	// 使用后立即删除（一次性）
	delete(csrfTokenStore, token)

	// 检查是否过期
	if time.Since(created) > csrfTokenMaxAge {
		return false
	}

	return true
}

// GetLoginCSRFToken 获取登录用的一次性 CSRF Token
// 此接口仅供前端页面在登录前调用
func GetLoginCSRFToken(c *gin.Context) {
	token := generateCSRFToken()
	respondOK(c, gin.H{"csrfToken": token})
}

type loginRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	RawPassword string `json:"rawPassword"` // RSA 加密的明文密码（用于代理认证）
	Encrypted   bool   `json:"_encrypted"`  // 标记密码是否已加密（SHA256）
	CSRFToken   string `json:"_csrf"`       // 一次性 CSRF 令牌
}

func Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	ss := services.GetSecurityService()

	// 1. 检查IP黑名单
	inBlacklist, reason, _ := ss.CheckIPBlacklist(clientIP)
	if inBlacklist {
		ss.RecordLoginAttempt(req.Username, nil, clientIP, userAgent, false, "IP在黑名单中")
		ss.RecordSecurityEvent(models.EventLoginBlocked, models.SeverityHigh, clientIP, nil, req.Username,
			"login", "", "登录被阻止: IP在黑名单中", map[string]interface{}{"reason": reason})
		respondError(c, http.StatusForbidden, "访问被拒绝")
		return
	}

	// 2. 检查IP锁定
	ipLocked, ipExpiresAt, _ := ss.CheckIPLockout(clientIP)
	if ipLocked {
		ss.RecordLoginAttempt(req.Username, nil, clientIP, userAgent, false, "IP被锁定")
		respondError(c, http.StatusForbidden, fmt.Sprintf("IP已被锁定，请在 %s 后重试", ipExpiresAt.Format("15:04:05")))
		return
	}

	// 3. 检查账户锁定
	accountLocked, accountExpiresAt, _ := ss.CheckAccountLockout(req.Username)
	if accountLocked {
		ss.RecordLoginAttempt(req.Username, nil, clientIP, userAgent, false, "账户被锁定")
		respondError(c, http.StatusForbidden, fmt.Sprintf("账户已被锁定，请在 %s 后重试", accountExpiresAt.Format("15:04:05")))
		return
	}

	// 4. 查找用户
	var user models.User
	if err := storage.DB.Where("username = ? AND is_deleted = 0", req.Username).First(&user).Error; err != nil {
		ss.RecordLoginAttempt(req.Username, nil, clientIP, userAgent, false, "用户不存在")
		ss.HandleFailedLogin(req.Username, clientIP)
		middleware.RecordLoginLog(0, req.Username, clientIP, userAgent, false, "用户不存在")
		respondError(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	// 5. 检查用户状态
	if user.Status == 0 {
		ss.RecordLoginAttempt(req.Username, &user.ID, clientIP, userAgent, false, "用户已禁用")
		middleware.RecordLoginLog(user.ID, req.Username, clientIP, userAgent, false, "用户已禁用")
		respondError(c, http.StatusForbidden, "用户已被禁用")
		return
	}

	// 5.5. 检查用户是否已过期
	if user.IsExpired() {
		ss.RecordLoginAttempt(req.Username, &user.ID, clientIP, userAgent, false, "账户已过期")
		middleware.RecordLoginLog(user.ID, req.Username, clientIP, userAgent, false, "账户已过期")
		respondError(c, http.StatusForbidden, fmt.Sprintf("账户已于 %s 过期，请联系管理员", user.ExpiresAt.Format("2006-01-02")))
		return
	}

	// 6. 检查用户级别锁定
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		ss.RecordLoginAttempt(req.Username, &user.ID, clientIP, userAgent, false, "账户被锁定")
		respondError(c, http.StatusForbidden, fmt.Sprintf("账户已被锁定，请在 %s 后重试", user.LockedUntil.Format("15:04:05")))
		return
	}

	// 7. CSRF 令牌校验 — 防止脚本/curl 直接调用登录接口
	if !validateCSRFToken(req.CSRFToken) {
		ss.RecordLoginAttempt(req.Username, &user.ID, clientIP, userAgent, false, "CSRF令牌无效")
		middleware.RecordLoginLog(user.ID, req.Username, clientIP, userAgent, false, "非法登录调用(CSRF)")
		respondError(c, http.StatusForbidden, "登录请求无效，请通过浏览器访问登录页面")
		return
	}

	// 8. 安全策略：禁止明文密码登录，必须由前端 SHA256 哈希后传输
	if !req.Encrypted {
		ss.RecordLoginAttempt(req.Username, &user.ID, clientIP, userAgent, false, "明文密码登录被拒绝")
		middleware.RecordLoginLog(user.ID, req.Username, clientIP, userAgent, false, "明文密码登录被拒绝")
		respondError(c, http.StatusForbidden, "不支持明文密码登录，请使用 AppID/AppKey 方式接入")
		return
	}

	passwordValid := false
	var passwordLearned bool

	log.Printf("[登录调试] 用户 %s, 本地密码长度=%d, rawPassword长度=%d, 代理启用=%v",
		user.Username, len(user.Password), len(req.RawPassword), services.IsPasswordAuthProxyEnabled())

	// 前端发送的是SHA256哈希，密码存储格式：bcrypt(SHA256(原始密码))
	// 支持多种密码格式，包括从 LDAP 同步的 {LDAP}{SSHA}... 格式
	if user.Password != "" {
		// 先尝试标准 bcrypt 验证
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err == nil {
			passwordValid = true
		} else if strings.HasPrefix(user.Password, "{LDAP}") {
			// LDAP 格式密码需要原始密码验证，尝试从 RawPassword 获取
			rawPassword := decodeRawPassword(req.RawPassword)
			if rawPassword != "" && crypto.VerifyPassword(user.Password, rawPassword, false) {
				passwordValid = true
				// 验证成功后，将 LDAP 密码转换为本地格式以便后续快速验证
				// 同时更新 SambaNTPassword 以支持本地 LDAP 服务认证
				if newHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost); err == nil {
					updates := map[string]interface{}{
						"password":          string(newHash),
						"samba_nt_password": ldapserver.ComputeNTHash(rawPassword),
					}
					storage.DB.Model(&user).Updates(updates)
					log.Printf("[登录] 用户 %s 的 LDAP 密码已转换为本地格式（含 SambaNT）", user.Username)
				}
			}
		}
	}

	// 本地密码验证失败，尝试代理认证
	if !passwordValid && services.IsPasswordAuthProxyEnabled() {
		// 解密 RSA 加密的明文密码
		rawPassword := decodeRawPassword(req.RawPassword)
		if rawPassword != "" {
			authenticated, learned, proxyErr := services.ProxyAuthenticateUser(&user, rawPassword)
			if authenticated {
				passwordValid = true
				passwordLearned = learned
				if learned {
					log.Printf("[登录] 用户 %s 通过代理认证成功，密码已学习", user.Username)
				} else {
					log.Printf("[登录] 用户 %s 通过代理认证成功", user.Username)
				}
			} else if proxyErr != nil {
				log.Printf("[登录] 用户 %s 代理认证失败: %v", user.Username, proxyErr)
			}
		} else {
			log.Printf("[登录] 用户 %s 代理认证跳过: rawPassword 为空 (req.RawPassword 长度=%d)", user.Username, len(req.RawPassword))
		}
	}

	if !passwordValid {
		ss.RecordLoginAttempt(req.Username, &user.ID, clientIP, userAgent, false, "密码错误")
		ss.HandleFailedLogin(req.Username, clientIP)
		middleware.RecordLoginLog(user.ID, req.Username, clientIP, userAgent, false, "密码错误")
		respondError(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	// 如果密码是通过代理学习的，记录安全事件并触发同步
	if passwordLearned {
		ss.RecordSecurityEvent(models.EventPasswordChanged, models.SeverityLow, clientIP, &user.ID, req.Username,
			"login", "", "通过代理认证学习密码", nil)

		// 触发密码同步事件（同步到下游 AD 等系统）
		rawPassword := decodeRawPassword(req.RawPassword)
		if rawPassword != "" {
			syncer.DispatchSyncEvent(models.SyncEventPasswordChange, user.ID, rawPassword)
			log.Printf("[密码同步] 已触发用户 %s 密码同步事件", user.Username)
		}
	}

	// 8. 检查是否需要双因素认证
	if Check2FARequired(&user, clientIP) {
		// 需要 2FA，创建待验证会话
		pendingToken, code := create2FASession(user.ID, user.Username, user.Phone, clientIP, userAgent)

		// 发送验证码
		result, err := services.SendVerifyCodeSMS(user.Phone, code, user.Username, user.Nickname)
		if err != nil {
			log.Printf("[2FA] 验证码发送失败, 用户 %s: %v (验证码: %s)", user.Username, err, code)
			respondError(c, http.StatusInternalServerError, "验证码发送失败，请稍后重试")
			return
		}

		// 返回需要 2FA 的响应
		respondOK(c, gin.H{
			"require_2fa":   true,
			"pending_token": pendingToken,
			"channel_type":  result.ChannelType,
			"target":        result.Target,
			"message":       "请输入动态验证码",
		})
		return
	}

	// 9. 不需要 2FA，直接生成 Token
	token, err := middleware.GenerateToken(user.ID, user.Username)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "生成Token失败")
		return
	}

	setAuthCookie(c, token)

	// 10. 记录成功登录
	ss.RecordLoginAttempt(req.Username, &user.ID, clientIP, userAgent, true, "")
	ss.HandleSuccessfulLogin(user.ID, clientIP)
	ss.RecordSecurityEvent(models.EventLoginSuccess, models.SeverityLow, clientIP, &user.ID, req.Username,
		"login", "", "用户登录成功", nil)
	middleware.RecordLoginLog(user.ID, req.Username, clientIP, userAgent, true, "登录成功")

	// 会话记录由 Auth 中间件在首次请求时自动创建

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

func Logout(c *gin.Context) {
	// 登出时终止当前会话
	userID := middleware.GetUserID(c)
	if userID > 0 {
		token := c.GetHeader("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
		// 使用 token hash 查找并终止匹配的会话
		tokenHash := middleware.HashToken(token)
		storage.DB.Model(&models.Session{}).
			Where("user_id = ? AND access_token = ? AND is_active = ?", userID, tokenHash, true).
			Update("is_active", false)
	}
	clearAuthCookie(c)
	respondOK(c, nil)
}

func setAuthCookie(c *gin.Context, token string) {
	secure := false
	if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil {
		secure = true
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "gsf_token",
		Value:    token,
		Path:     "/",
		MaxAge:   24 * 3600,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAuthCookie(c *gin.Context) {
	secure := false
	if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil {
		secure = true
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "gsf_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func GetUserInfo(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var user models.User
	if err := storage.DB.Preload("Roles").First(&user, userID).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}

	permissions := middleware.GetUserPermissions(userID)

	// ---- 合并所有角色的布局配置 ----
	// sidebarMode: 最宽松原则 visible > auto > hidden
	//   只要任何一个角色设了 visible，就显示侧栏
	//   全部 hidden 时才隐藏，否则 auto
	// landingPage: 收集所有角色配置的候选页，按优先级选取
	//   super_admin > 功能性角色(非user) > 默认角色(user)
	//   在同一优先级内验证用户是否有权访问该页面
	sidebarMode := "auto"
	hasVisible := false
	allHidden := true
	var superAdminLanding string
	var functionalLandings []string
	var defaultLandings []string
	for _, role := range user.Roles {
		// sidebarMode 合并
		if role.SidebarMode == "visible" {
			hasVisible = true
		}
		if role.SidebarMode != "hidden" {
			allHidden = false
		}
		// landingPage 收集（按角色类型分组）
		if role.LandingPage != "" {
			if role.Code == "super_admin" {
				superAdminLanding = role.LandingPage
			} else if role.Code == "user" {
				defaultLandings = append(defaultLandings, role.LandingPage)
			} else {
				functionalLandings = append(functionalLandings, role.LandingPage)
			}
		}
	}
	if hasVisible {
		sidebarMode = "visible"
	} else if allHidden && len(user.Roles) > 0 {
		sidebarMode = "hidden"
	}

	// landingPage: 按 super_admin > 功能性角色 > 默认角色 的优先级
	// 选第一个用户有权限访问的页面
	landingPage := ""
	// 页面路径 -> 所需权限的映射表
	pagePermMap := map[string]string{
		"/admin":                     "settings:system",
		"/admin/users/local":         "user:list",
		"/admin/users/connectors":    "settings:system",
		"/admin/users/synchronizers": "settings:system",
		"/admin/roles":               "role:list",
		"/admin/sso/apps":            "sso:apps",
		"/admin/sso/perms":           "sso:perms",
		"/admin/sso/mappings":        "sso:mappings",
		"/admin/sso/integration":     "sso:integration",
		"/admin/sso/logs":            "sso:logs",
		"/admin/logs/system":         "log:login",
		"/admin/logs/login":          "log:login",
		"/admin/logs/operation":      "log:operation",
		"/admin/logs/sync":           "log:operation",
		"/admin/notify/channels":     "settings:system",
		"/admin/notify/templates":    "settings:system",
		"/admin/notify/rules":        "settings:system",
		"/admin/settings":            "settings:system",
		"/admin/security":            "settings:system",
		"/admin/apikeys":             "settings:system",
		"/admin/profile":             "", // 个人中心无权限要求
	}
	permSet := make(map[string]bool)
	for _, p := range permissions {
		permSet[p] = true
	}
	canAccess := func(path string) bool {
		needed, exists := pagePermMap[path]
		if !exists {
			return true // 未知页面默认允许（前端会再验证）
		}
		if needed == "" {
			return true // 无权限要求
		}
		return permSet[needed]
	}
	// 按优先级尝试
	if superAdminLanding != "" && canAccess(superAdminLanding) {
		landingPage = superAdminLanding
	}
	if landingPage == "" {
		for _, lp := range functionalLandings {
			if canAccess(lp) {
				landingPage = lp
				break
			}
		}
	}
	if landingPage == "" {
		for _, lp := range defaultLandings {
			if canAccess(lp) {
				landingPage = lp
				break
			}
		}
	}

	respondOK(c, gin.H{
		"id":          user.ID,
		"username":    user.Username,
		"nickname":    user.Nickname,
		"phone":       user.Phone,
		"email":       user.Email,
		"avatar":      user.Avatar,
		"roles":       user.Roles,
		"permissions": permissions,
		"layoutConfig": gin.H{
			"sidebarMode": sidebarMode,
			"landingPage": landingPage,
		},
	})
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
	Encrypted   bool   `json:"_encrypted"`
	RawPwd      string `json:"_rawPwd"`
}

func ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := middleware.GetUserID(c)
	var user models.User
	if err := storage.DB.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		respondError(c, http.StatusBadRequest, "原密码错误")
		return
	}

	ss := services.GetSecurityService()

	// 如果前端未加密（原文密码），才在后端执行密码策略校验
	if !req.Encrypted {
		valid, errors := ss.ValidatePassword(req.NewPassword)
		if !valid {
			respondError(c, http.StatusBadRequest, errors[0])
			return
		}
	}

	// 检查密码历史
	canUse, err := ss.CheckPasswordHistory(userID, req.NewPassword)
	if err != nil || !canUse {
		respondError(c, http.StatusBadRequest, "不能使用最近使用过的密码")
		return
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	now := time.Now()
	rawPwd := decodeRawPassword(req.RawPwd)
	updates := map[string]interface{}{
		"password":              string(hashed),
		"force_password_change": false,
		"password_changed_at":   now,
	}
	if rawPwd != "" {
		updates["samba_nt_password"] = ldapserver.ComputeNTHash(rawPwd)
	}
	storage.DB.Model(&user).Updates(updates)

	// 更新密码历史
	ss.UpdatePasswordHistory(userID, string(hashed))

	// 使该用户所有已签发的 token 失效
	middleware.InvalidateUserTokens(userID)

	// 记录安全事件
	ss.RecordSecurityEvent(models.EventPasswordChanged, models.SeverityMedium, c.ClientIP(), &userID, user.Username,
		"user", fmt.Sprintf("%d", userID), "用户修改了密码", nil)

	middleware.RecordOperationLog(c, "用户", "修改密码", "", "")
	syncer.DispatchSyncEvent(models.SyncEventPasswordChange, userID, rawPwd)
	respondOK(c, nil)
}

// ========== 个人中心 ==========

// GetProfile 获取当前用户个人资料
func GetProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var user models.User
	if err := storage.DB.Preload("Roles").First(&user, userID).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}

	// 根据消息策略确定可用的验证方式（合并所有密码相关场景）+ 检查通道实际可用性
	userRoleIDs := make([]uint, 0, len(user.Roles))
	for _, r := range user.Roles {
		userRoleIDs = append(userRoleIDs, r.ID)
	}
	verifyMethodSet := make(map[string]bool)
	for _, scene := range []string{"password_reset", "verify_code", "password_reset_notify"} {
		for _, t := range ResolveAllowedChannelTypes(scene, user.GroupID, userRoleIDs...) {
			verifyMethodSet[t] = true
		}
	}
	var allowedVerifyMethods []string
	for t := range verifyMethodSet {
		// 过滤掉实际不可用的通道
		switch t {
		case "dingtalk":
			if !services.IsIMConnectorAvailable("im_dingtalk") {
				continue
			}
		case "feishu":
			if !services.IsIMConnectorAvailable("im_feishu") {
				continue
			}
		case "wecom":
			if !services.IsIMConnectorAvailable("im_wechatwork") {
				continue
			}
		case "sms":
			if !services.IsSMSChannelAvailable() {
				continue
			}
		}
		allowedVerifyMethods = append(allowedVerifyMethods, t)
	}

	respondOK(c, gin.H{
		"id":                   user.ID,
		"username":             user.Username,
		"nickname":             user.Nickname,
		"phone":                user.Phone,
		"email":                user.Email,
		"avatar":               user.Avatar,
		"source":               user.Source,
		"dingtalkUid":          user.DingTalkUID,
		"departmentName":       user.DepartmentName,
		"jobTitle":             user.JobTitle,
		"roles":                user.Roles,
		"createdAt":            user.CreatedAt,
		"lastLoginAt":          user.LastLoginAt,
		"lastLoginIp":          user.LastLoginIP,
		"passwordChangedAt":    user.PasswordChangedAt,
		"allowedVerifyMethods": allowedVerifyMethods, // 消息策略允许的通知渠道类型
	})
}

// UpdateProfile 更新个人资料（仅允许修改昵称、邮箱、头像）
func UpdateProfile(c *gin.Context) {
	var req struct {
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
		Avatar   string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := middleware.GetUserID(c)
	updates := map[string]interface{}{}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Avatar != "" {
		updates["avatar"] = req.Avatar
	}
	if len(updates) == 0 {
		respondError(c, http.StatusBadRequest, "无更新内容")
		return
	}

	if err := storage.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "更新失败")
		return
	}

	middleware.RecordOperationLog(c, "个人中心", "更新个人资料", "", "")
	respondOK(c, nil)
}

// ChangePasswordMultiMethod 多方式修改密码（原密码 / 钉钉验证码 / 短信验证码）
func ChangePasswordMultiMethod(c *gin.Context) {
	var req struct {
		Method      string `json:"method" binding:"required"`      // "password" / "dingtalk" / "sms"
		OldPassword string `json:"oldPassword"`                    // method=password 时必填
		Code        string `json:"code"`                           // method=dingtalk/sms 时必填
		NewPassword string `json:"newPassword" binding:"required"` // 新密码
		Encrypted   bool   `json:"_encrypted"`                     // 前端是否已SHA256哈希
		RawPwd      string `json:"_rawPwd"`                        // Base64编码的明文密码(AD同步用)
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := middleware.GetUserID(c)
	var user models.User
	if err := storage.DB.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}

	// 根据验证方式进行身份验证
	switch req.Method {
	case "password":
		if req.OldPassword == "" {
			respondError(c, http.StatusBadRequest, "请输入原密码")
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
			respondError(c, http.StatusBadRequest, "原密码错误")
			return
		}

	case "dingtalk":
		if req.Code == "" {
			respondError(c, http.StatusBadRequest, "请输入钉钉验证码")
			return
		}
		// 验证钉钉工作通知验证码
		if !verifyDingTalkCode(userID, req.Code) {
			respondError(c, http.StatusBadRequest, "钉钉验证码错误或已过期")
			return
		}

	case "sms":
		if req.Code == "" {
			respondError(c, http.StatusBadRequest, "请输入短信验证码")
			return
		}
		// 验证短信验证码
		if !verifySmsCode(userID, req.Code) {
			respondError(c, http.StatusBadRequest, "短信验证码错误或已过期")
			return
		}

	default:
		respondError(c, http.StatusBadRequest, "不支持的验证方式")
		return
	}

	ss := services.GetSecurityService()

	// 如果前端未加密（原文密码），才在后端执行密码策略校验
	// 前端已 SHA256 哈希时，策略校验在前端完成，后端跳过
	if !req.Encrypted {
		valid, policyErrors := ss.ValidatePassword(req.NewPassword)
		if !valid {
			respondError(c, http.StatusBadRequest, policyErrors[0])
			return
		}
	}

	// 检查密码历史
	canUse, err := ss.CheckPasswordHistory(userID, req.NewPassword)
	if err != nil || !canUse {
		respondError(c, http.StatusBadRequest, "不能使用最近使用过的密码")
		return
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	now := time.Now()
	rawPwd := decodeRawPassword(req.RawPwd)
	updates := map[string]interface{}{
		"password":              string(hashed),
		"force_password_change": false,
		"password_changed_at":   now,
	}
	if rawPwd != "" {
		updates["samba_nt_password"] = ldapserver.ComputeNTHash(rawPwd)
	}
	storage.DB.Model(&user).Updates(updates)

	ss.UpdatePasswordHistory(userID, string(hashed))

	// 使该用户所有已签发的 token 失效，强制所有会话重新登录
	middleware.InvalidateUserTokens(userID)

	ss.RecordSecurityEvent(models.EventPasswordChanged, models.SeverityMedium, c.ClientIP(), &userID, user.Username,
		"user", fmt.Sprintf("%d", userID), fmt.Sprintf("用户通过 %s 方式修改了密码", req.Method), nil)

	middleware.RecordOperationLog(c, "个人中心", "修改密码", fmt.Sprintf("方式: %s", req.Method), "")
	syncer.DispatchSyncEvent(models.SyncEventPasswordChange, userID, rawPwd)
	respondOK(c, nil)
}

// SendVerifyCode 发送验证码（钉钉/短信）
func SendVerifyCode(c *gin.Context) {
	var req struct {
		Method string `json:"method" binding:"required"` // "dingtalk" / "sms"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := middleware.GetUserID(c)
	var user models.User
	if err := storage.DB.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}

	switch req.Method {
	case "dingtalk":
		if user.DingTalkUID == "" {
			respondError(c, http.StatusBadRequest, "该用户未绑定钉钉，无法发送钉钉验证码")
			return
		}
		code := generateVerifyCode()
		storeVerifyCode(userID, "dingtalk", code)
		if err := services.SendVerifyCodeDingTalk(user.DingTalkUID, user.Username, user.Nickname, code); err != nil {
			log.Printf("[验证码] 钉钉发送失败, 用户 %s: %v (验证码: %s)", user.Username, err, code)
			respondError(c, http.StatusInternalServerError, "验证码发送失败: "+err.Error())
			return
		}
		respondOK(c, gin.H{"message": "验证码已发送到钉钉工作通知"})

	case "sms":
		if user.Phone == "" {
			respondError(c, http.StatusBadRequest, "该用户未绑定手机号，无法发送短信验证码")
			return
		}
		code := generateVerifyCode()
		storeVerifyCode(userID, "sms", code)
		_, err := services.SendVerifyCodeSMS(user.Phone, code, user.Username, user.Nickname)
		if err != nil {
			log.Printf("[验证码] 短信发送失败, 用户 %s: %v (验证码: %s)", user.Username, err, code)
			respondError(c, http.StatusInternalServerError, "验证码发送失败: "+err.Error())
			return
		}
		respondOK(c, gin.H{"message": "验证码已发送到手机"})

	default:
		respondError(c, http.StatusBadRequest, "不支持的验证方式")
	}
}

// ========== 忘记密码（无需登录） ==========

// --- 验证码发送频率追踪 ---
var (
	codeSendTracker     = make(map[string]*codeSendRecord)
	codeSendTrackerLock = &sync.Mutex{}
	// 忘记密码验证码错误次数追踪
	forgotFailTracker     = make(map[string]*forgotFailRecord)
	forgotFailTrackerLock = &sync.Mutex{}
)

type codeSendRecord struct {
	LastSent  time.Time
	HourCount int
	HourReset time.Time
	DayCount  int
	DayReset  time.Time
}

type forgotFailRecord struct {
	FailCount int
	LockedAt  time.Time
}

// checkCodeSendLimit 检查验证码发送频率（60秒冷却 / 每小时3次 / 每天10次）
// 返回: allowed bool, reason string
func checkCodeSendLimit(username string) (bool, string) {
	codeSendTrackerLock.Lock()
	defer codeSendTrackerLock.Unlock()

	now := time.Now()
	rec, exists := codeSendTracker[username]
	if !exists {
		codeSendTracker[username] = &codeSendRecord{
			LastSent:  now,
			HourCount: 1,
			HourReset: now.Add(time.Hour),
			DayCount:  1,
			DayReset:  now.Add(24 * time.Hour),
		}
		return true, ""
	}

	// 60秒冷却
	if now.Sub(rec.LastSent) < 60*time.Second {
		remaining := 60 - int(now.Sub(rec.LastSent).Seconds())
		return false, fmt.Sprintf("请在%d秒后再试", remaining)
	}

	// 小时计数器重置
	if now.After(rec.HourReset) {
		rec.HourCount = 0
		rec.HourReset = now.Add(time.Hour)
	}

	// 日计数器重置
	if now.After(rec.DayReset) {
		rec.DayCount = 0
		rec.DayReset = now.Add(24 * time.Hour)
	}

	if rec.HourCount >= 3 {
		return false, "每小时最多请求3次验证码，请稍后再试"
	}
	if rec.DayCount >= 10 {
		return false, "今日验证码请求已达上限（10次），请明天再试"
	}

	rec.LastSent = now
	rec.HourCount++
	rec.DayCount++
	return true, ""
}

// getCodeSendCount 获取当前发送计数（用于判断是否超限需通知）
func getCodeSendCount(username string) (hourCount, dayCount int) {
	codeSendTrackerLock.Lock()
	defer codeSendTrackerLock.Unlock()
	if rec, ok := codeSendTracker[username]; ok {
		return rec.HourCount, rec.DayCount
	}
	return 0, 0
}

// checkForgotLock 检查忘记密码功能是否被锁定（验证码错误过多）
func checkForgotLock(username string) (bool, int) {
	forgotFailTrackerLock.Lock()
	defer forgotFailTrackerLock.Unlock()

	rec, exists := forgotFailTracker[username]
	if !exists {
		return false, 0
	}

	// 30分钟后自动解锁
	if rec.FailCount >= 5 && time.Since(rec.LockedAt) < 30*time.Minute {
		remaining := 30 - int(time.Since(rec.LockedAt).Minutes())
		return true, remaining
	}

	// 过期则重置
	if rec.FailCount >= 5 {
		rec.FailCount = 0
	}

	return false, 0
}

// recordForgotFail 记录验证码错误
func recordForgotFail(username string) int {
	forgotFailTrackerLock.Lock()
	defer forgotFailTrackerLock.Unlock()

	rec, exists := forgotFailTracker[username]
	if !exists {
		forgotFailTracker[username] = &forgotFailRecord{FailCount: 1}
		return 1
	}

	// 如果之前已锁定且已过期，重新计数
	if rec.FailCount >= 5 && time.Since(rec.LockedAt) >= 30*time.Minute {
		rec.FailCount = 1
		rec.LockedAt = time.Time{}
		return 1
	}

	rec.FailCount++
	if rec.FailCount >= 5 {
		rec.LockedAt = time.Now()
	}
	return rec.FailCount
}

// resetForgotFail 重置验证码错误计数
func resetForgotFail(username string) {
	forgotFailTrackerLock.Lock()
	defer forgotFailTrackerLock.Unlock()
	delete(forgotFailTracker, username)
}

// sendAbuseNotification 记录验证码滥用安全事件
// 安全事件会被自动记录到数据库，告警通知由「告警规则」系统统一处理
// （RecordSecurityEvent 内部会异步触发 triggerAlertRules 匹配告警规则）
func sendAbuseNotification(user models.User, clientIP string, hourCount, dayCount int) {
	ss := services.GetSecurityService()

	// 记录安全事件 → 自动触发告警规则匹配
	ss.RecordSecurityEvent(models.EventSuspiciousActivity, models.SeverityHigh, clientIP, &user.ID, user.Username,
		"forgot_password", fmt.Sprintf("%d", user.ID),
		fmt.Sprintf("验证码请求频率异常（小时: %d次, 日: %d次），IP: %s", hourCount, dayCount, clientIP),
		map[string]interface{}{"hourCount": hourCount, "dayCount": dayCount, "ip": clientIP})
}

// forgotPasswordIPCheck 忘记密码 IP 黑名单检查
func forgotPasswordIPCheck(c *gin.Context) bool {
	ss := services.GetSecurityService()
	blocked, reason, _ := ss.CheckIPBlacklist(c.ClientIP())
	if blocked {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "您的IP已被封禁: " + reason,
		})
		return true
	}
	return false
}

// ForgotPasswordCheck 查询用户可用的验证方式（防枚举）
func ForgotPasswordCheck(c *gin.Context) {
	if forgotPasswordIPCheck(c) {
		return
	}

	var req struct {
		Username string `json:"username" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 支持用户名、手机号、邮箱查找用户
	var user models.User
	input := strings.TrimSpace(req.Username)
	err := storage.DB.Where(
		"(username = ? OR phone = ? OR email = ?) AND is_deleted = 0",
		input, input, input,
	).First(&user).Error
	if err != nil {
		// 防枚举：用户不存在时返回统一格式（空 methods），不暴露"用户不存在"
		respondOK(c, gin.H{
			"nickname": "",
			"methods":  []gin.H{},
			"message":  "如果该用户存在，可用的验证方式将显示在下方",
		})
		return
	}

	// 加载用户角色
	storage.DB.Preload("Roles").First(&user, user.ID)
	fpRoleIDs := make([]uint, 0, len(user.Roles))
	for _, r := range user.Roles {
		fpRoleIDs = append(fpRoleIDs, r.ID)
	}

	// 根据消息策略过滤可用的验证方式
	allowedSet := make(map[string]bool)
	hasAnyPolicy := false
	for _, scene := range []string{"password_reset", "verify_code", "password_reset_notify"} {
		types := ResolveAllowedChannelTypes(scene, user.GroupID, fpRoleIDs...)
		if len(types) > 0 {
			hasAnyPolicy = true
			for _, t := range types {
				allowedSet[t] = true
			}
		}
	}

	methods := []gin.H{}
	// 显示验证方式前，同时检查：(1) 用户绑定信息 (2) 消息策略 (3) 通道实际可用性
	if user.DingTalkUID != "" && (!hasAnyPolicy || allowedSet["dingtalk"]) && services.IsIMConnectorAvailable("im_dingtalk") {
		methods = append(methods, gin.H{"key": "dingtalk", "name": "钉钉验证码", "hint": "发送至钉钉工作通知"})
	}
	if user.Phone != "" && (!hasAnyPolicy || allowedSet["sms"]) && services.IsSMSChannelAvailable() {
		methods = append(methods, gin.H{"key": "sms", "name": "短信验证码", "hint": "发送至 " + maskPhone(user.Phone)})
	}

	respondOK(c, gin.H{
		"nickname": user.Nickname,
		"username": user.Username,
		"methods":  methods,
	})
}

// ForgotPasswordSendCode 忘记密码 - 发送验证码（带频率限制和通知）
func ForgotPasswordSendCode(c *gin.Context) {
	if forgotPasswordIPCheck(c) {
		return
	}

	var req struct {
		Username string `json:"username" binding:"required"`
		Method   string `json:"method" binding:"required"` // "dingtalk" / "sms"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 支持用户名、手机号、邮箱查找用户
	input := strings.TrimSpace(req.Username)
	var user models.User
	if err := storage.DB.Where(
		"(username = ? OR phone = ? OR email = ?) AND is_deleted = 0",
		input, input, input,
	).First(&user).Error; err != nil {
		// 防枚举：统一返回
		respondOK(c, gin.H{"message": "如果账号信息正确，验证码将会发送"})
		return
	}

	if user.Status == 0 {
		respondOK(c, gin.H{"message": "如果账号信息正确，验证码将会发送"})
		return
	}

	// 频率检查（使用真实用户名）
	allowed, reason := checkCodeSendLimit(user.Username)
	if !allowed {
		// 检查是否需要发送滥用通知
		hourCount, dayCount := getCodeSendCount(req.Username)
		if hourCount >= 3 || dayCount >= 10 {
			sendAbuseNotification(user, c.ClientIP(), hourCount, dayCount)
		}
		respondError(c, http.StatusTooManyRequests, reason)
		return
	}

	// 发送完成后检查是否接近上限，提前通知
	hourCount, dayCount := getCodeSendCount(req.Username)
	if hourCount >= 3 || dayCount >= 10 {
		sendAbuseNotification(user, c.ClientIP(), hourCount, dayCount)
	}

	switch req.Method {
	case "dingtalk":
		if user.DingTalkUID == "" {
			respondError(c, http.StatusBadRequest, "该用户未绑定钉钉，无法使用此方式")
			return
		}
		code := generateVerifyCode()
		storeVerifyCode(user.ID, "forgot_dingtalk", code)
		if err := services.SendVerifyCodeDingTalk(user.DingTalkUID, user.Username, user.Nickname, code); err != nil {
			log.Printf("[忘记密码] 钉钉验证码发送失败, 用户 %s: %v (验证码: %s)", user.Username, err, code)
			respondError(c, http.StatusInternalServerError, "验证码发送失败: "+err.Error())
			return
		}
		respondOK(c, gin.H{
			"message":  "验证码已发送到钉钉工作通知",
			"maskedId": maskDingTalkUID(user.DingTalkUID),
		})

	case "sms":
		if user.Phone == "" {
			respondError(c, http.StatusBadRequest, "该用户未绑定手机号，无法使用此方式")
			return
		}
		code := generateVerifyCode()
		storeVerifyCode(user.ID, "forgot_sms", code)
		_, err := services.SendVerifyCodeSMS(user.Phone, code, user.Username, user.Nickname)
		if err != nil {
			log.Printf("[忘记密码] 短信验证码发送失败, 用户 %s: %v (验证码: %s)", user.Username, err, code)
			respondError(c, http.StatusInternalServerError, "验证码发送失败: "+err.Error())
			return
		}
		respondOK(c, gin.H{
			"message":     "验证码已发送到手机",
			"maskedPhone": maskPhone(user.Phone),
		})

	default:
		respondError(c, http.StatusBadRequest, "不支持的验证方式")
	}
}

// ForgotPasswordReset 忘记密码 - 重置密码（带错误锁定）
func ForgotPasswordReset(c *gin.Context) {
	if forgotPasswordIPCheck(c) {
		return
	}

	var req struct {
		Username    string `json:"username" binding:"required"`
		Method      string `json:"method" binding:"required"`
		Code        string `json:"code" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required"`
		Encrypted   bool   `json:"_encrypted"`
		RawPwd      string `json:"_rawPwd"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 检查忘记密码功能是否被锁定
	locked, remaining := checkForgotLock(req.Username)
	if locked {
		respondError(c, http.StatusTooManyRequests,
			fmt.Sprintf("验证码错误次数过多，忘记密码功能已锁定，请%d分钟后再试", remaining))
		return
	}

	// 支持用户名、手机号、邮箱查找用户
	resetInput := strings.TrimSpace(req.Username)
	var user models.User
	if err := storage.DB.Where(
		"(username = ? OR phone = ? OR email = ?) AND is_deleted = 0",
		resetInput, resetInput, resetInput,
	).First(&user).Error; err != nil {
		respondError(c, http.StatusBadRequest, "验证码错误或已过期")
		return
	}

	// 验证码校验
	codeMethod := "forgot_" + req.Method
	if !checkVerifyCode(user.ID, codeMethod, req.Code) {
		failCount := recordForgotFail(req.Username)
		ss := services.GetSecurityService()
		if failCount >= 5 {
			ss.RecordSecurityEvent(models.EventSuspiciousActivity, models.SeverityHigh, c.ClientIP(), &user.ID, user.Username,
				"forgot_password", fmt.Sprintf("%d", user.ID),
				fmt.Sprintf("验证码连续错误%d次，忘记密码功能已锁定30分钟", failCount), nil)
		}
		respondError(c, http.StatusBadRequest,
			fmt.Sprintf("验证码错误或已过期（已错误%d/5次）", failCount))
		return
	}

	// 验证通过，重置错误计数
	resetForgotFail(req.Username)

	// 密码策略（仅原文时校验）
	if !req.Encrypted {
		ss := services.GetSecurityService()
		valid, policyErrors := ss.ValidatePassword(req.NewPassword)
		if !valid {
			respondError(c, http.StatusBadRequest, policyErrors[0])
			return
		}
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	now := time.Now()
	rawPwd := decodeRawPassword(req.RawPwd)
	updates := map[string]interface{}{
		"password":              string(hashed),
		"force_password_change": false,
		"password_changed_at":   now,
	}
	if rawPwd != "" {
		updates["samba_nt_password"] = ldapserver.ComputeNTHash(rawPwd)
	}
	storage.DB.Model(&user).Updates(updates)

	middleware.InvalidateUserTokens(user.ID)

	ss := services.GetSecurityService()
	ss.UpdatePasswordHistory(user.ID, string(hashed))
	ss.RecordSecurityEvent(models.EventPasswordChanged, models.SeverityMedium, c.ClientIP(), &user.ID, user.Username,
		"user", fmt.Sprintf("%d", user.ID), fmt.Sprintf("用户通过忘记密码（%s）重置了密码", req.Method), nil)

	syncer.DispatchSyncEvent(models.SyncEventPasswordChange, user.ID, rawPwd)
	respondOK(c, gin.H{"message": "密码重置成功，请使用新密码登录"})
}

func maskPhone(phone string) string {
	if len(phone) >= 7 {
		return phone[:3] + "****" + phone[len(phone)-4:]
	}
	return phone
}

func maskDingTalkUID(uid string) string {
	if len(uid) > 4 {
		return uid[:2] + "***" + uid[len(uid)-2:]
	}
	return "***"
}

// ===== 验证码存储与校验（内存实现，后续可替换为 Redis） =====

var (
	verifyCodeStore     = make(map[string]verifyCodeEntry)
	verifyCodeStoreLock = &sync.Mutex{}
)

type verifyCodeEntry struct {
	Code      string
	ExpiresAt time.Time
}

func generateVerifyCode() string {
	code := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	return code
}

func storeVerifyCode(userID uint, method string, code string) {
	key := fmt.Sprintf("%d:%s", userID, method)
	verifyCodeStoreLock.Lock()
	defer verifyCodeStoreLock.Unlock()
	verifyCodeStore[key] = verifyCodeEntry{
		Code:      code,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
}

func verifyDingTalkCode(userID uint, code string) bool {
	return checkVerifyCode(userID, "dingtalk", code)
}

func verifySmsCode(userID uint, code string) bool {
	return checkVerifyCode(userID, "sms", code)
}

func checkVerifyCode(userID uint, method string, code string) bool {
	key := fmt.Sprintf("%d:%s", userID, method)
	verifyCodeStoreLock.Lock()
	defer verifyCodeStoreLock.Unlock()

	entry, ok := verifyCodeStore[key]
	if !ok {
		return false
	}

	if time.Now().After(entry.ExpiresAt) {
		delete(verifyCodeStore, key)
		return false
	}

	if entry.Code != code {
		return false
	}

	// 验证通过后删除，一次性使用
	delete(verifyCodeStore, key)
	return true
}

// ========== 钉钉免登 ==========

type dingTalkLoginRequest struct {
	AuthCode string `json:"authCode" binding:"required"`
}

// DingTalkLogin 钉钉免登接口
func DingTalkLogin(c *gin.Context) {
	var req dingTalkLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	ss := services.GetSecurityService()

	// 1. 检查钉钉免登是否启用（优先检查新连接器 SSO 开关）
	var ssoConn models.Connector
	hasSSOConn := storage.DB.Where("type = ? AND im_enable_sso = ? AND status = 1", "im_dingtalk", true).
		First(&ssoConn).Error == nil

	// 如果存在新的连接器配置但 SSO 已关闭，拒绝免登
	var anyDTConn int64
	storage.DB.Model(&models.Connector{}).Where("type = ? AND status = 1", "im_dingtalk").Count(&anyDTConn)
	if anyDTConn > 0 && !hasSSOConn {
		respondError(c, http.StatusForbidden, "钉钉免登已关闭")
		return
	}

	// 兼容旧配置：如果没有新连接器，检查旧系统设置
	client := dingtalk.GetClient()
	if anyDTConn == 0 && !client.IsEnabled() {
		respondError(c, http.StatusForbidden, "钉钉免登未启用")
		return
	}

	// 2. 获取钉钉配置
	cfg, err := client.GetConfig()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取钉钉配置失败")
		return
	}

	// 3. 通过authCode获取钉钉用户信息
	ddUser, err := client.GetUserInfoByAuthCode(req.AuthCode)
	if err != nil {
		ss.RecordSecurityEvent(models.EventLoginFailed, models.SeverityMedium, clientIP, nil, "",
			"dingtalk", "", "钉钉免登失败: "+err.Error(), nil)
		respondError(c, http.StatusUnauthorized, "钉钉认证失败: "+err.Error())
		return
	}

	// 4. 根据配置的匹配字段查找系统用户
	var user models.User
	var found bool

	switch cfg.MatchField {
	case "mobile":
		if ddUser.Mobile != "" {
			err = storage.DB.Where("phone = ? AND is_deleted = 0", ddUser.Mobile).First(&user).Error
			found = err == nil
		}
	case "email":
		if ddUser.Email != "" {
			err = storage.DB.Where("email = ? AND is_deleted = 0", ddUser.Email).First(&user).Error
			found = err == nil
		}
	case "userid":
		// 使用钉钉UserID作为用户名
		err = storage.DB.Where("username = ? AND is_deleted = 0", ddUser.UserID).First(&user).Error
		found = err == nil
	default:
		// 默认使用手机号
		if ddUser.Mobile != "" {
			err = storage.DB.Where("phone = ? AND is_deleted = 0", ddUser.Mobile).First(&user).Error
			found = err == nil
		}
	}

	// 5. 如果用户不存在，检查是否启用自动注册
	if !found {
		if cfg.AutoRegister && cfg.DefaultRoleID > 0 {
			// 自动创建用户
			// 生成随机密码
			userIDSuffix := ddUser.UserID
			if len(userIDSuffix) > 6 {
				userIDSuffix = userIDSuffix[:6]
			}
			randomPwd := fmt.Sprintf("dd_%d_%s", time.Now().UnixNano(), userIDSuffix)
			hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(randomPwd), bcrypt.DefaultCost)

			user = models.User{
				Username:        ddUser.UserID,
				Password:        string(hashedPwd),
				SambaNTPassword: ldapserver.ComputeNTHash(randomPwd),
				Nickname:        ddUser.Name,
				Phone:           ddUser.Mobile,
				Email:           ddUser.Email,
				Avatar:          ddUser.Avatar,
				Status:          1,
			}

			if err := storage.DB.Create(&user).Error; err != nil {
				respondError(c, http.StatusInternalServerError, "创建用户失败")
				return
			}

			// 分配默认角色
			var role models.Role
			if err := storage.DB.First(&role, cfg.DefaultRoleID).Error; err == nil {
				storage.DB.Model(&user).Association("Roles").Append(&role)
			}

			ss.RecordSecurityEvent(models.EventLoginSuccess, models.SeverityLow, clientIP, &user.ID, user.Username,
				"dingtalk", "", "钉钉免登自动注册用户", map[string]interface{}{
					"ddUserId": ddUser.UserID,
					"ddName":   ddUser.Name,
				})
		} else {
			ss.RecordSecurityEvent(models.EventLoginFailed, models.SeverityMedium, clientIP, nil, ddUser.UserID,
				"dingtalk", "", "钉钉免登失败: 用户未绑定", map[string]interface{}{
					"ddUserId": ddUser.UserID,
					"ddName":   ddUser.Name,
					"ddMobile": ddUser.Mobile,
				})
			respondError(c, http.StatusUnauthorized, "该钉钉用户未绑定系统账号")
			return
		}
	}

	// 6. 检查用户状态
	if user.Status == 0 {
		ss.RecordLoginAttempt(user.Username, &user.ID, clientIP, userAgent, false, "用户已禁用")
		respondError(c, http.StatusForbidden, "用户已被禁用")
		return
	}

	// 7. 生成Token
	token, err := middleware.GenerateToken(user.ID, user.Username)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "生成Token失败")
		return
	}

	// 8. 记录登录成功
	ss.RecordLoginAttempt(user.Username, &user.ID, clientIP, userAgent, true, "")
	ss.HandleSuccessfulLogin(user.ID, clientIP)
	ss.RecordSecurityEvent(models.EventLoginSuccess, models.SeverityLow, clientIP, &user.ID, user.Username,
		"dingtalk", "", "钉钉免登成功", map[string]interface{}{
			"ddUserId": ddUser.UserID,
			"ddName":   ddUser.Name,
		})
	middleware.RecordLoginLog(user.ID, user.Username, clientIP, userAgent, true, "钉钉免登")

	// 会话记录由 Auth 中间件在首次请求时自动创建

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
