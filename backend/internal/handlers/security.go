package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/services"
	"go-syncflow/internal/storage"

	"github.com/gin-gonic/gin"
)

var securityService = services.GetSecurityService()

// ========== 安全仪表板 ==========

// GetSecurityDashboard 获取安全仪表板数据
func GetSecurityDashboard(c *gin.Context) {
	// 统计数据
	var totalUsers, activeUsers int64
	storage.DB.Model(&models.User{}).Where("is_deleted = 0").Count(&totalUsers)
	storage.DB.Model(&models.User{}).Where("is_deleted = 0 AND status = 1").Count(&activeUsers)

	var activeSessions int64
	storage.DB.Model(&models.Session{}).Where("is_active = ? AND expires_at > ?", true, time.Now()).Count(&activeSessions)

	var blockedIPs int64
	storage.DB.Model(&models.IPBlacklist{}).Where("is_active = ? AND (expires_at IS NULL OR expires_at > ?)", true, time.Now()).Count(&blockedIPs)

	var eventsToday int64
	today := time.Now().Truncate(24 * time.Hour)
	storage.DB.Model(&models.SecurityEvent{}).Where("created_at >= ?", today).Count(&eventsToday)

	var failedLogins24h int64
	storage.DB.Model(&models.LoginAttempt{}).Where("success = ? AND created_at >= ?", false, time.Now().Add(-24*time.Hour)).Count(&failedLogins24h)

	var activeLockouts int64
	storage.DB.Model(&models.Lockout{}).Where("is_active = ? AND expires_at > ?", true, time.Now()).Count(&activeLockouts)

	// 计算安全评分 (简单算法)
	securityScore := 100
	if failedLogins24h > 10 {
		securityScore -= 10
	}
	if blockedIPs > 5 {
		securityScore -= 5
	}
	if activeLockouts > 3 {
		securityScore -= 10
	}
	if securityScore < 0 {
		securityScore = 0
	}

	// 最近事件
	var recentEvents []models.SecurityEvent
	storage.DB.Order("created_at DESC").Limit(10).Find(&recentEvents)

	// 登录趋势 (24小时)
	var loginTrend []map[string]interface{}
	for i := 23; i >= 0; i-- {
		hourStart := time.Now().Add(-time.Duration(i) * time.Hour).Truncate(time.Hour)
		hourEnd := hourStart.Add(time.Hour)

		var success, failed int64
		storage.DB.Model(&models.LoginAttempt{}).Where("success = ? AND created_at >= ? AND created_at < ?", true, hourStart, hourEnd).Count(&success)
		storage.DB.Model(&models.LoginAttempt{}).Where("success = ? AND created_at >= ? AND created_at < ?", false, hourStart, hourEnd).Count(&failed)

		loginTrend = append(loginTrend, map[string]interface{}{
			"hour":    hourStart.Format("15:04"),
			"success": success,
			"failed":  failed,
		})
	}

	// 威胁来源 (按IP统计失败次数)
	var threatSources []map[string]interface{}
	storage.DB.Model(&models.LoginAttempt{}).
		Select("ip_address, COUNT(*) as count").
		Where("success = ? AND created_at >= ?", false, time.Now().Add(-24*time.Hour)).
		Group("ip_address").
		Order("count DESC").
		Limit(10).
		Find(&threatSources)

	respondOK(c, gin.H{
		"overview": gin.H{
			"totalUsers":      totalUsers,
			"activeUsers":     activeUsers,
			"activeSessions":  activeSessions,
			"blockedIPs":      blockedIPs,
			"eventsToday":     eventsToday,
			"failedLogins24h": failedLogins24h,
			"activeLockouts":  activeLockouts,
			"securityScore":   securityScore,
		},
		"recentEvents":  recentEvents,
		"loginTrend":    loginTrend,
		"threatSources": threatSources,
	})
}

// ========== 安全事件 ==========

// GetSecurityEvents 获取安全事件列表
func GetSecurityEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	eventType := c.Query("eventType")
	severity := c.Query("severity")
	sourceIP := c.Query("sourceIp")
	username := c.Query("username")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	query := storage.DB.Model(&models.SecurityEvent{})

	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if sourceIP != "" {
		query = query.Where("source_ip LIKE ?", "%"+sourceIP+"%")
	}
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if startDate != "" {
		query = query.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("created_at <= ?", endDate+" 23:59:59")
	}

	var total int64
	query.Count(&total)

	var events []models.SecurityEvent
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&events)

	respondOK(c, gin.H{
		"list":  events,
		"total": total,
	})
}

// ResolveSecurityEvent 标记事件已处理
func ResolveSecurityEvent(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := c.GetUint("userID")

	now := time.Now()
	err := storage.DB.Model(&models.SecurityEvent{}).Where("id = ?", id).Updates(map[string]interface{}{
		"is_resolved": true,
		"resolved_by": userID,
		"resolved_at": now,
	}).Error

	if err != nil {
		respondError(c, http.StatusInternalServerError, "操作失败")
		return
	}

	respondOK(c, nil)
}

// ========== 登录尝试 ==========

// GetLoginAttempts 获取登录尝试记录
func GetLoginAttempts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	username := c.Query("username")
	ip := c.Query("ip")
	success := c.Query("success")

	query := storage.DB.Model(&models.LoginAttempt{})

	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if ip != "" {
		query = query.Where("ip_address LIKE ?", "%"+ip+"%")
	}
	if success != "" {
		query = query.Where("success = ?", success == "true")
	}

	var total int64
	query.Count(&total)

	var attempts []models.LoginAttempt
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&attempts)

	respondOK(c, gin.H{
		"list":  attempts,
		"total": total,
	})
}

// ========== 锁定管理 ==========

// GetLockouts 获取锁定记录
func GetLockouts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	lockType := c.Query("type")
	activeOnly := c.Query("activeOnly")

	query := storage.DB.Model(&models.Lockout{})

	if lockType != "" {
		query = query.Where("lock_type = ?", lockType)
	}
	if activeOnly == "true" {
		query = query.Where("is_active = ? AND expires_at > ?", true, time.Now())
	}

	var total int64
	query.Count(&total)

	var lockouts []models.Lockout
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&lockouts)

	respondOK(c, gin.H{
		"list":  lockouts,
		"total": total,
	})
}

// UnlockAccount 解锁账户
func UnlockAccount(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := c.GetUint("userID")
	if err := securityService.UnlockAccount(req.Username, userID); err != nil {
		respondError(c, http.StatusInternalServerError, "解锁失败")
		return
	}

	respondOK(c, nil)
}

// UnlockIP 解锁IP
func UnlockIP(c *gin.Context) {
	var req struct {
		IP string `json:"ip" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := c.GetUint("userID")
	if err := securityService.UnlockIP(req.IP, userID); err != nil {
		respondError(c, http.StatusInternalServerError, "解锁失败")
		return
	}

	respondOK(c, nil)
}

// ========== IP黑白名单 ==========

// GetIPBlacklist 获取IP黑名单
func GetIPBlacklist(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	query := storage.DB.Model(&models.IPBlacklist{}).Where("is_active = ?", true)

	var total int64
	query.Count(&total)

	var list []models.IPBlacklist
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)

	respondOK(c, gin.H{
		"list":  list,
		"total": total,
	})
}

// AddIPBlacklist 添加IP到黑名单
func AddIPBlacklist(c *gin.Context) {
	var req struct {
		IPAddress string  `json:"ipAddress" binding:"required"`
		Reason    string  `json:"reason" binding:"required"`
		ExpiresIn *int    `json:"expiresIn"` // 过期时间(小时), nil=永久
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(*req.ExpiresIn) * time.Hour)
		expiresAt = &t
	}

	userID := c.GetUint("userID")
	if err := securityService.AddToBlacklist(req.IPAddress, req.Reason, "manual", expiresAt, &userID); err != nil {
		respondError(c, http.StatusInternalServerError, "添加失败: "+err.Error())
		return
	}

	respondOK(c, nil)
}

// RemoveIPBlacklist 从黑名单移除
func RemoveIPBlacklist(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := c.GetUint("userID")

	if err := securityService.RemoveFromBlacklist(uint(id), userID); err != nil {
		respondError(c, http.StatusInternalServerError, "移除失败")
		return
	}

	respondOK(c, nil)
}

// GetIPWhitelistMode 获取白名单状态（条目数量）
func GetIPWhitelistMode(c *gin.Context) {
	count := middleware.GetIPWhitelistCount()
	respondOK(c, gin.H{
		"enabled": count > 0,
		"count":   count,
	})
}

// GetIPWhitelist 获取IP白名单
func GetIPWhitelist(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	query := storage.DB.Model(&models.IPWhitelist{}).Where("is_active = ?", true)

	var total int64
	query.Count(&total)

	var list []models.IPWhitelist
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)

	respondOK(c, gin.H{
		"list":  list,
		"total": total,
	})
}

// AddIPWhitelist 添加IP到白名单
func AddIPWhitelist(c *gin.Context) {
	var req struct {
		IPAddress   string `json:"ipAddress" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := c.GetUint("userID")
	ipType := "single"
	// 简单 CIDR 检测
	for _, ch := range req.IPAddress {
		if ch == '/' {
			ipType = "cidr"
			break
		}
	}

	whitelist := models.IPWhitelist{
		IPAddress:   req.IPAddress,
		IPType:      ipType,
		Description: req.Description,
		CreatedBy:   &userID,
		IsActive:    true,
	}

	if err := storage.DB.Create(&whitelist).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "添加失败")
		return
	}

	// 立即刷新白名单缓存
	middleware.ForceRefreshIPWhitelist()

	middleware.RecordOperationLog(c, "安全中心", "添加API白名单", req.IPAddress, req.Description)
	respondOK(c, nil)
}

// RemoveIPWhitelist 从白名单移除
func RemoveIPWhitelist(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// 查询被删除的IP用于日志记录
	var wl models.IPWhitelist
	storage.DB.First(&wl, id)

	if err := storage.DB.Delete(&models.IPWhitelist{}, id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "移除失败")
		return
	}

	// 立即刷新白名单缓存
	middleware.ForceRefreshIPWhitelist()

	middleware.RecordOperationLog(c, "安全中心", "移除API白名单", wl.IPAddress, "")
	respondOK(c, nil)
}

// CheckIP 检查IP状态
func CheckIP(c *gin.Context) {
	var req struct {
		IP string `json:"ip" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	inBlacklist, reason, _ := securityService.CheckIPBlacklist(req.IP)
	inWhitelist, _ := securityService.CheckIPWhitelist(req.IP)
	isLocked, expiresAt, _ := securityService.CheckIPLockout(req.IP)

	respondOK(c, gin.H{
		"ip":          req.IP,
		"inBlacklist": inBlacklist,
		"blacklistReason": reason,
		"inWhitelist": inWhitelist,
		"isLocked":    isLocked,
		"lockExpiresAt": expiresAt,
	})
}

// ========== 会话管理 ==========

// GetAllSessions 获取所有活跃会话
func GetAllSessions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	query := storage.DB.Model(&models.Session{}).Where("is_active = ? AND expires_at > ?", true, time.Now())

	var total int64
	query.Count(&total)

	var sessions []models.Session
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&sessions)

	// 关联用户信息
	type SessionWithUser struct {
		models.Session
		Username string `json:"username"`
		Nickname string `json:"nickname"`
	}

	var result []SessionWithUser
	for _, s := range sessions {
		var user models.User
		storage.DB.Select("username, nickname").First(&user, s.UserID)
		result = append(result, SessionWithUser{
			Session:  s,
			Username: user.Username,
			Nickname: user.Nickname,
		})
	}

	respondOK(c, gin.H{
		"list":  result,
		"total": total,
	})
}

// GetMySessions 获取当前用户会话
func GetMySessions(c *gin.Context) {
	userID := c.GetUint("userID")
	sessions, _ := securityService.GetUserSessions(userID)
	respondOK(c, sessions)
}

// TerminateSession 终止会话
func TerminateSession(c *gin.Context) {
	sessionID := c.Param("id")
	userID := c.GetUint("userID")

	if err := securityService.TerminateSession(sessionID, &userID); err != nil {
		respondError(c, http.StatusInternalServerError, "终止失败")
		return
	}

	respondOK(c, nil)
}

// TerminateUserSessions 终止用户所有会话
func TerminateUserSessions(c *gin.Context) {
	targetUserID, _ := strconv.ParseUint(c.Param("userId"), 10, 32)
	userID := c.GetUint("userID")

	storage.DB.Model(&models.Session{}).Where("user_id = ? AND is_active = ?", targetUserID, true).
		Update("is_active", false)

	securityService.RecordSecurityEvent(models.EventSessionTerminated, models.SeverityMedium, "",
		&userID, "", "user", strconv.FormatUint(targetUserID, 10), "用户所有会话被终止", nil)

	respondOK(c, nil)
}

// ========== 安全配置 ==========

// GetSecurityConfigs 获取所有安全配置
func GetSecurityConfigs(c *gin.Context) {
	var configs []models.SecurityConfig
	storage.DB.Find(&configs)

	result := make(map[string]interface{})
	for _, cfg := range configs {
		var value interface{}
		json.Unmarshal([]byte(cfg.ConfigValue), &value)
		result[cfg.ConfigKey] = value
	}

	respondOK(c, result)
}

// GetSecurityConfig 获取指定安全配置
func GetSecurityConfig(c *gin.Context) {
	key := c.Param("key")
	config, err := securityService.GetConfig(key)
	if err != nil {
		respondError(c, http.StatusNotFound, "配置不存在")
		return
	}
	respondOK(c, config)
}

// UpdateSecurityConfig 更新安全配置
func UpdateSecurityConfig(c *gin.Context) {
	key := c.Param("key")

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	value, _ := json.Marshal(req)
	userID := c.GetUint("userID")

	if err := storage.SetSecurityConfig(key, string(value), &userID); err != nil {
		respondError(c, http.StatusInternalServerError, "保存失败")
		return
	}

	securityService.ClearConfigCache()

	// 记录配置修改事件
	securityService.RecordSecurityEvent(models.EventConfigChanged, models.SeverityMedium, c.ClientIP(),
		&userID, "", "security_config", key, "安全配置已修改: "+key, nil)

	respondOK(c, nil)
}

// ========== 告警配置 ==========

// GetNotifyChannels 获取通知渠道
func GetNotifyChannels(c *gin.Context) {
	var channels []models.NotifyChannel
	storage.DB.Find(&channels)

	// Config 字段有 json:"-" 标签，需要手动构造返回数据
	type ChannelResp struct {
		ID          uint                   `json:"id"`
		Name        string                 `json:"name"`
		ChannelType string                 `json:"channelType"`
		Config      map[string]interface{} `json:"config"`
		IsActive    bool                   `json:"isActive"`
		TestResult  string                 `json:"testResult"`
		TestedAt    interface{}            `json:"testedAt"`
		CreatedAt   interface{}            `json:"createdAt"`
		UpdatedAt   interface{}            `json:"updatedAt"`
	}

	result := make([]ChannelResp, 0, len(channels))
	for _, ch := range channels {
		var cfg map[string]interface{}
		json.Unmarshal([]byte(ch.Config), &cfg)
		resp := ChannelResp{
			ID:          ch.ID,
			Name:        ch.Name,
			ChannelType: ch.ChannelType,
			Config:      cfg,
			IsActive:    ch.IsActive,
			TestResult:  ch.TestResult,
			TestedAt:    ch.TestedAt,
			CreatedAt:   ch.CreatedAt,
			UpdatedAt:   ch.UpdatedAt,
		}
		result = append(result, resp)
	}
	respondOK(c, result)
}

// CreateNotifyChannel 创建通知渠道
func CreateNotifyChannel(c *gin.Context) {
	var req struct {
		Name        string                 `json:"name" binding:"required"`
		ChannelType string                 `json:"channelType" binding:"required"`
		Config      map[string]interface{} `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	configJSON, _ := json.Marshal(req.Config)
	channel := models.NotifyChannel{
		Name:        req.Name,
		ChannelType: req.ChannelType,
		Config:      string(configJSON),
		IsActive:    true,
	}

	if err := storage.DB.Create(&channel).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	respondOK(c, channel)
}

// UpdateNotifyChannel 更新通知渠道
func UpdateNotifyChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		Name        string                 `json:"name"`
		ChannelType string                 `json:"channelType"`
		Config      map[string]interface{} `json:"config"`
		IsActive    *bool                  `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	updates := make(map[string]interface{})
	var selectFields []string
	if req.Name != "" {
		updates["name"] = req.Name
		selectFields = append(selectFields, "name")
	}
	if req.ChannelType != "" {
		updates["channel_type"] = req.ChannelType
		selectFields = append(selectFields, "channel_type")
	}
	if req.Config != nil {
		configJSON, _ := json.Marshal(req.Config)
		updates["config"] = string(configJSON)
		selectFields = append(selectFields, "config")
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
		selectFields = append(selectFields, "is_active")
	}

	if len(selectFields) > 0 {
		if err := storage.DB.Model(&models.NotifyChannel{}).Where("id = ?", id).Select(selectFields).Updates(updates).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "更新失败")
			return
		}
	}

	respondOK(c, nil)
}

// DeleteNotifyChannel 删除通知渠道
func DeleteNotifyChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	if err := storage.DB.Delete(&models.NotifyChannel{}, id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "删除失败")
		return
	}

	respondOK(c, nil)
}

// TestNotifyChannel 测试通知渠道
func TestNotifyChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var channel models.NotifyChannel
	if err := storage.DB.First(&channel, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "通道不存在")
		return
	}

	// 接收可选的收件人和消息内容
	var req struct {
		Recipient string `json:"recipient"` // 手机号 或 钉钉UserID
		Message   string `json:"message"`   // 自定义消息内容
	}
	c.ShouldBindJSON(&req)

	if req.Message == "" {
		req.Message = "【测试】这是一条通道测试消息，收到请忽略。"
	}

	var testErr error

	switch {
	// 所有 SMS 类型通道（阿里云、腾讯云、华为云、百度云、天翼云、融合云信、移动云MAS、企业微信短信、钉钉短信、飞书、HTTPS自定义）
	case strings.HasPrefix(channel.ChannelType, "sms_"):
		phone := req.Recipient
		if phone == "" {
			var cfg struct {
				Phones string `json:"phones"`
			}
			json.Unmarshal([]byte(channel.Config), &cfg)
			if cfg.Phones != "" {
				phones := strings.Split(cfg.Phones, ",")
				phone = strings.TrimSpace(phones[0])
			}
		}
		if phone == "" {
			respondError(c, http.StatusBadRequest, "请指定接收手机号/用户ID")
			return
		}
		testErr = services.TestSMSChannelWithMessage(channel.ChannelType, channel.Config, phone, req.Message)

	case channel.ChannelType == "email":
		testErr = services.TestEmailChannel(channel.Config, req.Recipient, req.Message)

	case channel.ChannelType == "webhook":
		testErr = services.TestWebhookChannel(channel.Config, req.Message)

	case channel.ChannelType == "dingtalk_work":
		userID := req.Recipient
		if userID == "" {
			var cfg struct {
				TestUserID string `json:"testUserID"`
			}
			json.Unmarshal([]byte(channel.Config), &cfg)
			userID = cfg.TestUserID
		}
		if userID == "" {
			var admin models.User
			if err := storage.DB.Where("username = 'admin' AND is_deleted = 0 AND dingtalk_uid != ''").First(&admin).Error; err == nil {
				userID = admin.DingTalkUID
			}
		}
		if userID == "" {
			respondError(c, http.StatusBadRequest, "请指定接收人的钉钉UserID")
			return
		}
		testErr = services.TestDingTalkChannelWithMessage(userID, req.Message)

	default:
		respondError(c, http.StatusBadRequest, "不支持的通道类型: "+channel.ChannelType)
		return
	}

	now := time.Now()
	if testErr != nil {
		storage.DB.Model(&channel).Updates(map[string]interface{}{
			"test_result": "失败: " + testErr.Error(),
			"tested_at":   now,
		})
		// 记录测试失败日志
		middleware.RecordOperationLog(c, "消息通知", "测试发送",
			channel.Name+" ("+getChannelTypeName(channel.ChannelType)+")",
			"结果: 失败 - "+testErr.Error()+", 接收人: "+req.Recipient)
		respondError(c, http.StatusInternalServerError, "测试失败: "+testErr.Error())
		return
	}

	storage.DB.Model(&channel).Updates(map[string]interface{}{
		"test_result": "成功",
		"tested_at":   now,
	})
	// 记录测试成功日志
	middleware.RecordOperationLog(c, "消息通知", "测试发送",
		channel.Name+" ("+getChannelTypeName(channel.ChannelType)+")",
		"结果: 发送成功, 接收人: "+req.Recipient)
	respondOK(c, gin.H{"message": "测试发送成功"})
}

// ========== 消息模板管理 ==========

// GetMessageTemplates 获取所有消息模板
func GetMessageTemplates(c *gin.Context) {
	var templates []models.MessageTemplate
	storage.DB.Order("is_builtin DESC, id ASC").Find(&templates)
	respondOK(c, templates)
}

// GetMessageTemplate 获取单个消息模板
func GetMessageTemplate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var tpl models.MessageTemplate
	if err := storage.DB.First(&tpl, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "模板不存在")
		return
	}
	respondOK(c, tpl)
}

// CreateMessageTemplate 创建消息模板
func CreateMessageTemplate(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		Scene     string `json:"scene" binding:"required"`
		Content   string `json:"content" binding:"required"`
		Variables string `json:"variables"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 检查 scene 是否重复
	var existing models.MessageTemplate
	if storage.DB.Where("scene = ?", req.Scene).First(&existing).Error == nil {
		respondError(c, http.StatusConflict, "场景标识已存在")
		return
	}

	tpl := models.MessageTemplate{
		Name:      req.Name,
		Scene:     req.Scene,
		Content:   req.Content,
		Variables: req.Variables,
		IsBuiltin: false,
		IsActive:  true,
	}
	if err := storage.DB.Create(&tpl).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}
	respondOK(c, tpl)
}

// UpdateMessageTemplate 更新消息模板
func UpdateMessageTemplate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var tpl models.MessageTemplate
	if err := storage.DB.First(&tpl, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "模板不存在")
		return
	}

	var req struct {
		Name      *string `json:"name"`
		Content   *string `json:"content"`
		Variables *string `json:"variables"`
		IsActive  *bool   `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	updates := make(map[string]interface{})
	var selectFields []string
	if req.Name != nil {
		updates["name"] = *req.Name
		selectFields = append(selectFields, "name")
	}
	if req.Content != nil {
		updates["content"] = *req.Content
		selectFields = append(selectFields, "content")
	}
	if req.Variables != nil {
		updates["variables"] = *req.Variables
		selectFields = append(selectFields, "variables")
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
		selectFields = append(selectFields, "is_active")
	}

	if len(selectFields) > 0 {
		if err := storage.DB.Model(&tpl).Select(selectFields).Updates(updates).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "更新失败")
			return
		}
	}
	respondOK(c, nil)
}

// DeleteMessageTemplate 删除消息模板
func DeleteMessageTemplate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var tpl models.MessageTemplate
	if err := storage.DB.First(&tpl, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "模板不存在")
		return
	}

	if tpl.IsBuiltin {
		respondError(c, http.StatusForbidden, "内置模板不可删除")
		return
	}

	if err := storage.DB.Delete(&tpl).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "删除失败")
		return
	}
	respondOK(c, nil)
}

// GetAlertRules 获取告警规则
func GetAlertRules(c *gin.Context) {
	var rules []models.AlertRule
	storage.DB.Find(&rules)

	// 解析 JSON 字段方便前端使用
	type RuleView struct {
		models.AlertRule
		EventTypeList  []string `json:"eventTypes"`
		ChannelIDList  []uint   `json:"channelIds"`
		RoleIDList     []uint   `json:"notifyRoleIds"`
		UserIDList     []uint   `json:"notifyUserIds"`
	}
	var views []RuleView
	for _, r := range rules {
		v := RuleView{AlertRule: r}
		json.Unmarshal([]byte(r.EventTypes), &v.EventTypeList)
		json.Unmarshal([]byte(r.NotifyChannels), &v.ChannelIDList)
		json.Unmarshal([]byte(r.NotifyRoleIDs), &v.RoleIDList)
		json.Unmarshal([]byte(r.NotifyUserIDs), &v.UserIDList)
		views = append(views, v)
	}
	respondOK(c, views)
}

// CreateAlertRule 创建告警规则
func CreateAlertRule(c *gin.Context) {
	var req struct {
		AlertType         string   `json:"alertType"`
		Name              string   `json:"name" binding:"required"`
		EventTypes        []string `json:"eventTypes" binding:"required"`
		SeverityThreshold string   `json:"severityThreshold"`
		NotifyChannels    []uint   `json:"notifyChannels" binding:"required"`
		NotifyTarget      string   `json:"notifyTarget"`
		NotifyRoleIDs     []uint   `json:"notifyRoleIds"`
		NotifyUserIDs     []uint   `json:"notifyUserIds"`
		TemplateID        uint     `json:"templateId"`
		CooldownMinutes   int      `json:"cooldownMinutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	eventTypesJSON, _ := json.Marshal(req.EventTypes)
	channelsJSON, _ := json.Marshal(req.NotifyChannels)
	roleIDsJSON, _ := json.Marshal(req.NotifyRoleIDs)
	userIDsJSON, _ := json.Marshal(req.NotifyUserIDs)

	alertType := req.AlertType
	if alertType == "" {
		alertType = "admin"
	}
	notifyTarget := req.NotifyTarget
	if notifyTarget == "" {
		if alertType == "employee" {
			notifyTarget = "event_user"
		} else {
			notifyTarget = "channel"
		}
	}

	rule := models.AlertRule{
		AlertType:         alertType,
		Name:              req.Name,
		EventTypes:        string(eventTypesJSON),
		SeverityThreshold: req.SeverityThreshold,
		NotifyChannels:    string(channelsJSON),
		NotifyTarget:      notifyTarget,
		NotifyRoleIDs:     string(roleIDsJSON),
		NotifyUserIDs:     string(userIDsJSON),
		TemplateID:        req.TemplateID,
		CooldownMinutes:   req.CooldownMinutes,
		IsActive:          true,
	}

	if err := storage.DB.Create(&rule).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	respondOK(c, rule)
}

// UpdateAlertRule 更新告警规则
func UpdateAlertRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		AlertType         *string  `json:"alertType"`
		Name              string   `json:"name"`
		EventTypes        []string `json:"eventTypes"`
		SeverityThreshold string   `json:"severityThreshold"`
		NotifyChannels    []uint   `json:"notifyChannels"`
		NotifyTarget      *string  `json:"notifyTarget"`
		NotifyRoleIDs     []uint   `json:"notifyRoleIds"`
		NotifyUserIDs     []uint   `json:"notifyUserIds"`
		TemplateID        *uint    `json:"templateId"`
		CooldownMinutes   *int     `json:"cooldownMinutes"`
		IsActive          *bool    `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	updates := make(map[string]interface{})
	var selectFields []string
	if req.AlertType != nil {
		updates["alert_type"] = *req.AlertType
		selectFields = append(selectFields, "alert_type")
	}
	if req.Name != "" {
		updates["name"] = req.Name
		selectFields = append(selectFields, "name")
	}
	if req.EventTypes != nil {
		eventTypesJSON, _ := json.Marshal(req.EventTypes)
		updates["event_types"] = string(eventTypesJSON)
		selectFields = append(selectFields, "event_types")
	}
	if req.SeverityThreshold != "" {
		updates["severity_threshold"] = req.SeverityThreshold
		selectFields = append(selectFields, "severity_threshold")
	}
	if req.NotifyChannels != nil {
		channelsJSON, _ := json.Marshal(req.NotifyChannels)
		updates["notify_channels"] = string(channelsJSON)
		selectFields = append(selectFields, "notify_channels")
	}
	if req.NotifyTarget != nil {
		updates["notify_target"] = *req.NotifyTarget
		selectFields = append(selectFields, "notify_target")
	}
	if req.NotifyRoleIDs != nil {
		roleIDsJSON, _ := json.Marshal(req.NotifyRoleIDs)
		updates["notify_role_i_ds"] = string(roleIDsJSON)
		selectFields = append(selectFields, "notify_role_i_ds")
	}
	if req.NotifyUserIDs != nil {
		userIDsJSON, _ := json.Marshal(req.NotifyUserIDs)
		updates["notify_user_i_ds"] = string(userIDsJSON)
		selectFields = append(selectFields, "notify_user_i_ds")
	}
	if req.TemplateID != nil {
		updates["template_id"] = *req.TemplateID
		selectFields = append(selectFields, "template_id")
	}
	if req.CooldownMinutes != nil {
		updates["cooldown_minutes"] = *req.CooldownMinutes
		selectFields = append(selectFields, "cooldown_minutes")
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
		selectFields = append(selectFields, "is_active")
	}

	if len(selectFields) > 0 {
		if err := storage.DB.Model(&models.AlertRule{}).Where("id = ?", id).Select(selectFields).Updates(updates).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "更新失败")
			return
		}
	}

	respondOK(c, nil)
}

// DeleteAlertRule 删除告警规则
func DeleteAlertRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	if err := storage.DB.Delete(&models.AlertRule{}, id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "删除失败")
		return
	}

	respondOK(c, nil)
}

// GetAlertLogs 获取告警日志
func GetAlertLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	var total int64
	storage.DB.Model(&models.AlertLog{}).Count(&total)

	var logs []models.AlertLog
	storage.DB.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

	respondOK(c, gin.H{
		"list":  logs,
		"total": total,
	})
}

// ========== 消息策略管理 ==========

// ResolveAllowedChannelTypes 根据消息策略解析指定场景允许的通知渠道类型
// groupId: 用户所属群组ID, roleIds: 用户拥有的角色ID列表（可选）
func ResolveAllowedChannelTypes(scene string, groupId uint, roleIds ...uint) []string {
	var policies []models.MessagePolicy
	storage.DB.Where("scene = ? AND is_active = ?", scene, true).Order("priority desc, id asc").Find(&policies)

	var matched *models.MessagePolicy

	// 优先匹配群组/角色专属策略
	for i := range policies {
		if policies[i].TargetType != "group" {
			continue
		}
		// 群组匹配
		if groupId > 0 {
			var gids []uint
			json.Unmarshal([]byte(policies[i].TargetGroupIDs), &gids)
			for _, gid := range gids {
				if gid == groupId {
					matched = &policies[i]
					break
				}
			}
		}
		// 角色匹配
		if matched == nil && len(roleIds) > 0 {
			var rids []uint
			json.Unmarshal([]byte(policies[i].TargetRoleIDs), &rids)
			for _, rid := range rids {
				for _, userRid := range roleIds {
					if rid == userRid {
						matched = &policies[i]
						break
					}
				}
				if matched != nil {
					break
				}
			}
		}
		if matched != nil {
			break
		}
	}

	// 回退到默认策略
	if matched == nil {
		for i := range policies {
			if policies[i].TargetType == "all" || policies[i].TargetType == "" {
				matched = &policies[i]
				break
			}
		}
	}

	if matched == nil {
		return nil
	}

	var ids []uint
	json.Unmarshal([]byte(matched.ChannelIDs), &ids)
	if len(ids) == 0 {
		return nil
	}

	var channels []models.NotifyChannel
	storage.DB.Where("id IN ?", ids).Find(&channels)

	typeSet := make(map[string]bool)
	var types []string
	for _, ch := range channels {
		ct := normalizeChannelType(ch.ChannelType)
		if !typeSet[ct] {
			typeSet[ct] = true
			types = append(types, ct)
		}
	}
	return types
}

// GetPolicyByScene 获取指定场景的消息策略（任何已登录用户可调用）
// 支持 groupId 和 userId 参数，优先匹配群组/角色策略
func GetPolicyByScene(c *gin.Context) {
	scene := c.Query("scene")
	if scene == "" {
		respondError(c, http.StatusBadRequest, "缺少scene参数")
		return
	}

	groupIdStr := c.Query("groupId")
	groupId := uint(0)
	if groupIdStr != "" {
		if v, err := strconv.ParseUint(groupIdStr, 10, 32); err == nil {
			groupId = uint(v)
		}
	}

	// 获取用户角色 ID（通过 userId 参数）
	var userRoleIDs []uint
	userIdStr := c.Query("userId")
	if userIdStr != "" {
		if uid, err := strconv.ParseUint(userIdStr, 10, 32); err == nil {
			var userRoles []models.UserRole
			storage.DB.Where("user_id = ?", uid).Find(&userRoles)
			for _, ur := range userRoles {
				userRoleIDs = append(userRoleIDs, ur.RoleID)
			}
		}
	}

	var policies []models.MessagePolicy
	storage.DB.Where("scene = ? AND is_active = ?", scene, true).Order("priority desc, id asc").Find(&policies)

	var matched *models.MessagePolicy

	for i := range policies {
		if policies[i].TargetType != "group" {
			continue
		}
		// 群组匹配
		if groupId > 0 {
			var gids []uint
			json.Unmarshal([]byte(policies[i].TargetGroupIDs), &gids)
			for _, gid := range gids {
				if gid == groupId {
					matched = &policies[i]
					break
				}
			}
		}
		// 角色匹配
		if matched == nil && len(userRoleIDs) > 0 {
			var rids []uint
			json.Unmarshal([]byte(policies[i].TargetRoleIDs), &rids)
			for _, rid := range rids {
				for _, urid := range userRoleIDs {
					if rid == urid {
						matched = &policies[i]
						break
					}
				}
				if matched != nil {
					break
				}
			}
		}
		if matched != nil {
			break
		}
	}

	// 回退到默认策略（targetType=all）
	if matched == nil {
		for i := range policies {
			if policies[i].TargetType == "all" || policies[i].TargetType == "" {
				matched = &policies[i]
				break
			}
		}
	}

	if matched == nil {
		respondOK(c, gin.H{"scene": scene, "channelIdList": []uint{}, "channelNames": []string{}, "isActive": false})
		return
	}

	var ids []uint
	json.Unmarshal([]byte(matched.ChannelIDs), &ids)

	// 获取渠道名称和类型
	var channels []models.NotifyChannel
	channelNames := []string{}
	channelTypes := []string{}
	if len(ids) > 0 {
		storage.DB.Where("id IN ?", ids).Find(&channels)
		for _, ch := range channels {
			channelNames = append(channelNames, ch.Name)
			// 将 channelType 映射为 sendPasswordResetNotification 可用的通知类型
			ct := normalizeChannelType(ch.ChannelType)
			channelTypes = append(channelTypes, ct)
		}
	}

	respondOK(c, gin.H{
		"scene":         matched.Scene,
		"channelIdList": ids,
		"channelNames":  channelNames,
		"channelTypes":  channelTypes,
		"isActive":      matched.IsActive,
		"targetType":    matched.TargetType,
	})
}

// GetMessagePolicies 获取所有消息策略
func GetMessagePolicies(c *gin.Context) {
	var policies []models.MessagePolicy
	storage.DB.Order("priority desc, scene asc, id asc").Find(&policies)

	type policyResp struct {
		models.MessagePolicy
		ChannelIDList []uint `json:"channelIdList"`
		GroupIDList   []uint `json:"groupIdList"`
		RoleIDList   []uint `json:"roleIdList"`
	}
	resp := make([]policyResp, 0, len(policies))
	for _, p := range policies {
		var ids []uint
		json.Unmarshal([]byte(p.ChannelIDs), &ids)
		var gids []uint
		json.Unmarshal([]byte(p.TargetGroupIDs), &gids)
		var rids []uint
		json.Unmarshal([]byte(p.TargetRoleIDs), &rids)
		resp = append(resp, policyResp{MessagePolicy: p, ChannelIDList: ids, GroupIDList: gids, RoleIDList: rids})
	}

	respondOK(c, resp)
}

// UpsertMessagePolicy 创建或更新消息策略
func UpsertMessagePolicy(c *gin.Context) {
	var req struct {
		Scene      string `json:"scene" binding:"required"`
		SceneName  string `json:"sceneName"`
		ChannelIDs []uint `json:"channelIds"`
		IsActive   bool   `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	idsJSON, _ := json.Marshal(req.ChannelIDs)

	var existing models.MessagePolicy
	if storage.DB.Where("scene = ? AND (target_type = 'all' OR target_type = '' OR target_type IS NULL)", req.Scene).First(&existing).Error == nil {
		// 更新默认策略
		storage.DB.Model(&existing).Updates(map[string]interface{}{
			"scene_name":  req.SceneName,
			"channel_ids": string(idsJSON),
			"is_active":   req.IsActive,
			"target_type": "all",
		})
	} else {
		// 创建默认策略
		policy := models.MessagePolicy{
			Scene:      req.Scene,
			SceneName:  req.SceneName,
			ChannelIDs: string(idsJSON),
			TargetType: "all",
			IsActive:   req.IsActive,
		}
		storage.DB.Create(&policy)
	}

	respondOK(c, nil)
}

// BatchUpdateMessagePolicies 批量更新默认消息策略（targetType=all）
func BatchUpdateMessagePolicies(c *gin.Context) {
	var req []struct {
		Scene      string `json:"scene"`
		SceneName  string `json:"sceneName"`
		ChannelIDs []uint `json:"channelIds"`
		IsActive   bool   `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	for _, item := range req {
		idsJSON, _ := json.Marshal(item.ChannelIDs)
		var existing models.MessagePolicy
		// 查找默认策略（targetType=all 或空）
		if storage.DB.Where("scene = ? AND (target_type = 'all' OR target_type = '' OR target_type IS NULL)", item.Scene).First(&existing).Error == nil {
			storage.DB.Model(&existing).Updates(map[string]interface{}{
				"scene_name":   item.SceneName,
				"channel_ids":  string(idsJSON),
				"is_active":    item.IsActive,
				"target_type":  "all",
			})
		} else {
			storage.DB.Create(&models.MessagePolicy{
				Scene:      item.Scene,
				SceneName:  item.SceneName,
				ChannelIDs: string(idsJSON),
				TargetType: "all",
				IsActive:   item.IsActive,
			})
		}
	}

	respondOK(c, nil)
}

// ========== 群组消息策略管理 ==========

// CreateGroupPolicy 创建群组/角色消息策略
func CreateGroupPolicy(c *gin.Context) {
	var req struct {
		Scene          string `json:"scene" binding:"required"`
		SceneName      string `json:"sceneName"`
		ChannelIDs     []uint `json:"channelIds"`
		TargetGroupIDs []uint `json:"targetGroupIds"`
		TargetRoleIDs  []uint `json:"targetRoleIds"`
		IsActive       bool   `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if len(req.TargetGroupIDs) == 0 && len(req.TargetRoleIDs) == 0 {
		respondError(c, http.StatusBadRequest, "请至少选择一个适用群组或角色")
		return
	}

	idsJSON, _ := json.Marshal(req.ChannelIDs)
	gidsJSON, _ := json.Marshal(req.TargetGroupIDs)
	ridsJSON, _ := json.Marshal(req.TargetRoleIDs)

	policy := models.MessagePolicy{
		Scene:          req.Scene,
		SceneName:      req.SceneName,
		ChannelIDs:     string(idsJSON),
		TargetType:     "group",
		TargetGroupIDs: string(gidsJSON),
		TargetRoleIDs:  string(ridsJSON),
		Priority:       10,
		IsActive:       req.IsActive,
	}
	if err := storage.DB.Create(&policy).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	respondOK(c, policy)
}

// UpdateGroupPolicy 更新群组/角色消息策略
func UpdateGroupPolicy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		Scene          string `json:"scene"`
		SceneName      string `json:"sceneName"`
		ChannelIDs     []uint `json:"channelIds"`
		TargetGroupIDs []uint `json:"targetGroupIds"`
		TargetRoleIDs  []uint `json:"targetRoleIds"`
		IsActive       bool   `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	var policy models.MessagePolicy
	if err := storage.DB.First(&policy, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "策略不存在")
		return
	}

	idsJSON, _ := json.Marshal(req.ChannelIDs)
	gidsJSON, _ := json.Marshal(req.TargetGroupIDs)
	ridsJSON, _ := json.Marshal(req.TargetRoleIDs)

	storage.DB.Model(&policy).Updates(map[string]interface{}{
		"scene":            req.Scene,
		"scene_name":       req.SceneName,
		"channel_ids":      string(idsJSON),
		"target_group_ids": string(gidsJSON),
		"target_role_ids":  string(ridsJSON),
		"is_active":        req.IsActive,
	})

	respondOK(c, nil)
}

// DeleteGroupPolicy 删除群组消息策略
func DeleteGroupPolicy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var policy models.MessagePolicy
	if err := storage.DB.First(&policy, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "策略不存在")
		return
	}

	if policy.TargetType != "group" {
		respondError(c, http.StatusBadRequest, "只能删除群组策略")
		return
	}

	storage.DB.Delete(&policy)
	respondOK(c, nil)
}

// normalizeChannelType 将数据库中的通道类型映射为通知发送函数识别的标准类型
// getChannelTypeName 获取通道类型中文名称
func getChannelTypeName(channelType string) string {
	names := map[string]string{
		"sms_aliyun": "阿里云短信", "sms_tencent": "腾讯云短信", "sms_huawei": "华为云短信",
		"sms_baidu": "百度云短信", "sms_ctyun": "天翼云短信", "sms_ronghe": "融合云信",
		"sms_cmcc": "移动云MAS", "sms_cmcc_5g": "移动5G消息",
		"sms_wecom": "企微短信", "sms_dingtalk": "钉钉短信", "sms_feishu": "飞书短信",
		"sms_https": "HTTPS短信", "sms_custom": "自定义短信",
		"email": "邮件", "webhook": "Webhook", "dingtalk_work": "钉钉工作消息",
	}
	if name, ok := names[channelType]; ok {
		return name
	}
	return channelType
}

func normalizeChannelType(channelType string) string {
	if strings.HasPrefix(channelType, "sms") {
		return "sms"
	}
	if strings.HasPrefix(channelType, "dingtalk") {
		return "dingtalk"
	}
	if strings.HasPrefix(channelType, "feishu") {
		return "feishu"
	}
	if strings.HasPrefix(channelType, "wecom") {
		return "wecom"
	}
	// email, webhook 等保持原样
	return channelType
}
