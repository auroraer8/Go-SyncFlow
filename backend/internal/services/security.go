package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SecurityService 安全服务
type SecurityService struct {
	configCache     map[string]interface{}
	configCacheMu   sync.RWMutex
	configCacheTime time.Time
}

var securityService *SecurityService
var once sync.Once

// GetSecurityService 获取安全服务单例
func GetSecurityService() *SecurityService {
	once.Do(func() {
		securityService = &SecurityService{
			configCache: make(map[string]interface{}),
		}
	})
	return securityService
}

// ========== 配置管理 ==========

// GetConfig 获取安全配置
func (s *SecurityService) GetConfig(key string) (map[string]interface{}, error) {
	s.configCacheMu.RLock()
	if cached, ok := s.configCache[key]; ok && time.Since(s.configCacheTime) < 5*time.Minute {
		s.configCacheMu.RUnlock()
		return cached.(map[string]interface{}), nil
	}
	s.configCacheMu.RUnlock()

	value, err := storage.GetSecurityConfig(key)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return nil, err
	}

	s.configCacheMu.Lock()
	s.configCache[key] = config
	s.configCacheTime = time.Now()
	s.configCacheMu.Unlock()

	return config, nil
}

// ClearConfigCache 清除配置缓存
func (s *SecurityService) ClearConfigCache() {
	s.configCacheMu.Lock()
	s.configCache = make(map[string]interface{})
	s.configCacheMu.Unlock()
}

// ========== 登录安全 ==========

// RecordLoginAttempt 记录登录尝试
func (s *SecurityService) RecordLoginAttempt(username string, userID *uint, ip, userAgent string, success bool, failureReason string) error {
	attempt := models.LoginAttempt{
		Username:      username,
		UserID:        userID,
		IPAddress:     ip,
		UserAgent:     userAgent,
		Success:       success,
		FailureReason: failureReason,
		CreatedAt:     time.Now(),
	}
	return storage.DB.Create(&attempt).Error
}

// CheckAccountLockout 检查账户是否被锁定
func (s *SecurityService) CheckAccountLockout(username string) (bool, time.Time, error) {
	var lockout models.Lockout
	err := storage.DB.Where("lock_type = ? AND target = ? AND is_active = ? AND expires_at > ?",
		models.LockTypeAccount, username, true, time.Now()).
		Order("created_at DESC").First(&lockout).Error

	if err == gorm.ErrRecordNotFound {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, err
	}

	return true, lockout.ExpiresAt, nil
}

// CheckIPLockout 检查IP是否被锁定
func (s *SecurityService) CheckIPLockout(ip string) (bool, time.Time, error) {
	var lockout models.Lockout
	err := storage.DB.Where("lock_type = ? AND target = ? AND is_active = ? AND expires_at > ?",
		models.LockTypeIP, ip, true, time.Now()).
		Order("created_at DESC").First(&lockout).Error

	if err == gorm.ErrRecordNotFound {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, err
	}

	return true, lockout.ExpiresAt, nil
}

// HandleFailedLogin 处理登录失败
func (s *SecurityService) HandleFailedLogin(username, ip string) error {
	config, _ := s.GetConfig("login_security")

	// 更新用户失败次数
	var user models.User
	if err := storage.DB.Where("username = ?", username).First(&user).Error; err == nil {
		user.FailedAttempts++
		storage.DB.Save(&user)

		// 检查账户锁定
		if accountConfig, ok := config["account_lockout"].(map[string]interface{}); ok {
			if enabled, _ := accountConfig["enabled"].(bool); enabled {
				maxAttempts := int(getFloat64(accountConfig, "max_attempts", 5))
				if user.FailedAttempts >= maxAttempts {
					s.LockAccount(username, user.FailedAttempts, user.LockCount+1)
				}
			}
		}
	}

	// 检查IP锁定
	if ipConfig, ok := config["ip_lockout"].(map[string]interface{}); ok {
		if enabled, _ := ipConfig["enabled"].(bool); enabled {
			// 统计该IP最近1小时的失败次数
			var count int64
			storage.DB.Model(&models.LoginAttempt{}).
				Where("ip_address = ? AND success = ? AND created_at > ?", ip, false, time.Now().Add(-1*time.Hour)).
				Count(&count)

			maxAttempts := int64(getFloat64(ipConfig, "max_attempts", 20))
			if count >= maxAttempts {
				s.LockIP(ip, int(count))
			}
		}
	}

	return nil
}

// HandleSuccessfulLogin 处理登录成功
func (s *SecurityService) HandleSuccessfulLogin(userID uint, ip string) error {
	return storage.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"failed_attempts": 0,
		"last_login_ip":   ip,
		"last_login_at":   time.Now(),
	}).Error
}

// LockAccount 锁定账户
func (s *SecurityService) LockAccount(username string, attemptCount, lockCount int) error {
	config, _ := s.GetConfig("login_security")
	accountConfig, _ := config["account_lockout"].(map[string]interface{})

	// 计算锁定时长（渐进式）
	var durationMinutes int
	progressive, _ := accountConfig["progressive_lockout"].(bool)
	if progressive && lockCount > 1 {
		if lockCount == 2 {
			durationMinutes = int(getFloat64(accountConfig, "second_lockout_minutes", 60))
		} else {
			durationMinutes = int(getFloat64(accountConfig, "third_lockout_minutes", 1440))
		}
	} else {
		durationMinutes = int(getFloat64(accountConfig, "lockout_duration_minutes", 15))
	}

	expiresAt := time.Now().Add(time.Duration(durationMinutes) * time.Minute)

	// 停用之前的锁定记录
	storage.DB.Model(&models.Lockout{}).
		Where("lock_type = ? AND target = ? AND is_active = ?", models.LockTypeAccount, username, true).
		Update("is_active", false)

	// 创建新锁定记录
	lockout := models.Lockout{
		LockType:     models.LockTypeAccount,
		Target:       username,
		Reason:       "连续登录失败次数过多",
		AttemptCount: attemptCount,
		LockCount:    lockCount,
		LockedAt:     time.Now(),
		ExpiresAt:    expiresAt,
		IsActive:     true,
	}
	storage.DB.Create(&lockout)

	// 更新用户锁定信息
	storage.DB.Model(&models.User{}).Where("username = ?", username).Updates(map[string]interface{}{
		"locked_until": expiresAt,
		"lock_count":   lockCount,
	})

	// 记录安全事件
	s.RecordSecurityEvent(models.EventAccountLocked, models.SeverityHigh, "", nil, username,
		"account", username, "账户因连续登录失败被锁定", map[string]interface{}{
			"attempt_count":    attemptCount,
			"lock_count":       lockCount,
			"duration_minutes": durationMinutes,
		})

	return nil
}

// LockIP 锁定IP
func (s *SecurityService) LockIP(ip string, attemptCount int) error {
	config, _ := s.GetConfig("login_security")
	ipConfig, _ := config["ip_lockout"].(map[string]interface{})

	durationHours := int(getFloat64(ipConfig, "lockout_duration_hours", 24))
	expiresAt := time.Now().Add(time.Duration(durationHours) * time.Hour)

	// 停用之前的锁定记录
	storage.DB.Model(&models.Lockout{}).
		Where("lock_type = ? AND target = ? AND is_active = ?", models.LockTypeIP, ip, true).
		Update("is_active", false)

	// 创建新锁定记录
	lockout := models.Lockout{
		LockType:     models.LockTypeIP,
		Target:       ip,
		Reason:       "IP登录失败次数过多",
		AttemptCount: attemptCount,
		LockedAt:     time.Now(),
		ExpiresAt:    expiresAt,
		IsActive:     true,
	}
	storage.DB.Create(&lockout)

	// 记录安全事件
	s.RecordSecurityEvent(models.EventIPBlocked, models.SeverityHigh, ip, nil, "",
		"ip", ip, "IP因登录失败次数过多被锁定", map[string]interface{}{
			"attempt_count":  attemptCount,
			"duration_hours": durationHours,
		})

	return nil
}

// UnlockAccount 解锁账户
func (s *SecurityService) UnlockAccount(username string, unlockedBy uint) error {
	now := time.Now()
	storage.DB.Model(&models.Lockout{}).
		Where("lock_type = ? AND target = ? AND is_active = ?", models.LockTypeAccount, username, true).
		Updates(map[string]interface{}{
			"is_active":   false,
			"unlocked_at": now,
			"unlocked_by": unlockedBy,
		})

	storage.DB.Model(&models.User{}).Where("username = ?", username).Updates(map[string]interface{}{
		"locked_until":    nil,
		"failed_attempts": 0,
	})

	s.RecordSecurityEvent(models.EventAccountUnlocked, models.SeverityMedium, "", &unlockedBy, username,
		"account", username, "账户被手动解锁", nil)

	return nil
}

// UnlockIP 解锁IP
func (s *SecurityService) UnlockIP(ip string, unlockedBy uint) error {
	now := time.Now()
	storage.DB.Model(&models.Lockout{}).
		Where("lock_type = ? AND target = ? AND is_active = ?", models.LockTypeIP, ip, true).
		Updates(map[string]interface{}{
			"is_active":   false,
			"unlocked_at": now,
			"unlocked_by": unlockedBy,
		})

	s.RecordSecurityEvent(models.EventIPUnblocked, models.SeverityMedium, ip, &unlockedBy, "",
		"ip", ip, "IP被手动解锁", nil)

	return nil
}

// ========== IP黑白名单 ==========

// CheckIPBlacklist 检查IP是否在黑名单
func (s *SecurityService) CheckIPBlacklist(ip string) (bool, string, error) {
	var blacklist models.IPBlacklist

	// 精确匹配
	err := storage.DB.Where("ip_address = ? AND is_active = ? AND (expires_at IS NULL OR expires_at > ?)",
		ip, true, time.Now()).First(&blacklist).Error

	if err == nil {
		// 更新阻止次数
		storage.DB.Model(&blacklist).Update("blocked_count", blacklist.BlockedCount+1)
		return true, blacklist.Reason, nil
	}

	if err != gorm.ErrRecordNotFound {
		return false, "", err
	}

	// CIDR匹配
	var cidrList []models.IPBlacklist
	storage.DB.Where("ip_type = ? AND is_active = ? AND (expires_at IS NULL OR expires_at > ?)",
		"cidr", true, time.Now()).Find(&cidrList)

	for _, item := range cidrList {
		_, ipNet, err := net.ParseCIDR(item.IPAddress)
		if err != nil {
			continue
		}
		if ipNet.Contains(net.ParseIP(ip)) {
			storage.DB.Model(&item).Update("blocked_count", item.BlockedCount+1)
			return true, item.Reason, nil
		}
	}

	return false, "", nil
}

// CheckIPWhitelist 检查IP是否在白名单
func (s *SecurityService) CheckIPWhitelist(ip string) (bool, error) {
	var whitelist models.IPWhitelist

	// 精确匹配
	err := storage.DB.Where("ip_address = ? AND is_active = ?", ip, true).First(&whitelist).Error
	if err == nil {
		return true, nil
	}

	if err != gorm.ErrRecordNotFound {
		return false, err
	}

	// CIDR匹配
	var cidrList []models.IPWhitelist
	storage.DB.Where("ip_type = ? AND is_active = ?", "cidr", true).Find(&cidrList)

	for _, item := range cidrList {
		_, ipNet, err := net.ParseCIDR(item.IPAddress)
		if err != nil {
			continue
		}
		if ipNet.Contains(net.ParseIP(ip)) {
			return true, nil
		}
	}

	return false, nil
}

// AddToBlacklist 添加IP到黑名单
func (s *SecurityService) AddToBlacklist(ip, reason, source string, expiresAt *time.Time, createdBy *uint) error {
	ipType := "single"
	if strings.Contains(ip, "/") {
		ipType = "cidr"
	}

	blacklist := models.IPBlacklist{
		IPAddress: ip,
		IPType:    ipType,
		Reason:    reason,
		Source:    source,
		ExpiresAt: expiresAt,
		CreatedBy: createdBy,
		IsActive:  true,
	}

	if err := storage.DB.Create(&blacklist).Error; err != nil {
		return err
	}

	var userID *uint
	if createdBy != nil {
		userID = createdBy
	}
	s.RecordSecurityEvent(models.EventIPBlocked, models.SeverityMedium, ip, userID, "",
		"ip", ip, "IP被加入黑名单: "+reason, nil)

	return nil
}

// RemoveFromBlacklist 从黑名单移除
func (s *SecurityService) RemoveFromBlacklist(id uint, removedBy uint) error {
	var blacklist models.IPBlacklist
	if err := storage.DB.First(&blacklist, id).Error; err != nil {
		return err
	}

	if err := storage.DB.Delete(&blacklist).Error; err != nil {
		return err
	}

	s.RecordSecurityEvent(models.EventIPUnblocked, models.SeverityLow, blacklist.IPAddress, &removedBy, "",
		"ip", blacklist.IPAddress, "IP从黑名单移除", nil)

	return nil
}

// ========== 安全事件 ==========

// RecordSecurityEvent 记录安全事件
func (s *SecurityService) RecordSecurityEvent(eventType, severity, sourceIP string, userID *uint, username,
	targetType, targetID, description string, details map[string]interface{}) error {

	detailsJSON := ""
	if details != nil {
		data, _ := json.Marshal(details)
		detailsJSON = string(data)
	}

	event := models.SecurityEvent{
		EventType:   eventType,
		Severity:    severity,
		SourceIP:    sourceIP,
		UserID:      userID,
		Username:    username,
		TargetType:  targetType,
		TargetID:    targetID,
		Description: description,
		Details:     detailsJSON,
		CreatedAt:   time.Now(),
	}

	if err := storage.DB.Create(&event).Error; err != nil {
		return err
	}

	// 异步触发告警规则匹配
	go s.triggerAlertRules(&event)

	return nil
}

// triggerAlertRules 匹配告警规则并发送通知
func (s *SecurityService) triggerAlertRules(event *models.SecurityEvent) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[告警引擎] panic recovered: %v", r)
		}
	}()

	// 获取所有活跃的告警规则
	var rules []models.AlertRule
	storage.DB.Where("is_active = ?", true).Find(&rules)

	severityOrder := map[string]int{"low": 1, "medium": 2, "high": 3, "critical": 4}

	for _, rule := range rules {
		// 1. 检查事件类型是否匹配
		var eventTypes []string
		json.Unmarshal([]byte(rule.EventTypes), &eventTypes)
		if len(eventTypes) > 0 {
			matched := false
			for _, et := range eventTypes {
				if et == event.EventType {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 2. 检查严重级别
		eventLevel := severityOrder[event.Severity]
		thresholdLevel := severityOrder[rule.SeverityThreshold]
		if eventLevel < thresholdLevel {
			continue
		}

		// 3. 检查冷却时间
		if rule.LastTriggeredAt != nil {
			cooldown := time.Duration(rule.CooldownMinutes) * time.Minute
			if time.Since(*rule.LastTriggeredAt) < cooldown {
				continue
			}
		}

		// 4. 匹配成功，发送告警
		s.sendAlertNotification(&rule, event)

		// 5. 更新规则的最后触发时间
		now := time.Now()
		storage.DB.Model(&rule).Update("last_triggered_at", now)
	}
}

// sendAlertNotification 通过配置的通道发送告警通知
func (s *SecurityService) sendAlertNotification(rule *models.AlertRule, event *models.SecurityEvent) {
	// 获取通知渠道
	var channelIDs []uint
	json.Unmarshal([]byte(rule.NotifyChannels), &channelIDs)

	if len(channelIDs) == 0 {
		return
	}

	// 获取消息模板内容
	content := s.buildAlertContent(rule, event)

	// 遍历渠道发送
	for _, chID := range channelIDs {
		var channel models.NotifyChannel
		if err := storage.DB.First(&channel, chID).Error; err != nil {
			continue
		}
		if !channel.IsActive {
			continue
		}

		status := "sent"
		errMsg := ""

		switch channel.ChannelType {
		case "webhook":
			if err := SendWebhookAlert(channel.Config, content, event); err != nil {
				status = "failed"
				errMsg = err.Error()
				log.Printf("[告警引擎] webhook 发送失败: %v", err)
			}
		case "email":
			if err := s.sendEmailAlert(channel, rule, event, content); err != nil {
				status = "failed"
				errMsg = err.Error()
				log.Printf("[告警引擎] email 发送失败: %v", err)
			}
		default:
			// 短信等其他类型，获取目标用户发送
			if err := s.sendSMSAlert(channel, rule, event, content); err != nil {
				status = "failed"
				errMsg = err.Error()
				log.Printf("[告警引擎] sms 发送失败: %v", err)
			}
		}

		// 记录告警日志
		ruleID := rule.ID
		eventID := event.ID
		sentAt := time.Now()
		alertLog := models.AlertLog{
			RuleID:       &ruleID,
			EventID:      &eventID,
			ChannelID:    &channel.ID,
			ChannelType:  channel.ChannelType,
			Recipient:    channel.Name,
			Content:      content,
			Status:       status,
			ErrorMessage: errMsg,
			SentAt:       &sentAt,
		}
		storage.DB.Create(&alertLog)
	}
}

// buildAlertContent 构建告警消息内容
func (s *SecurityService) buildAlertContent(rule *models.AlertRule, event *models.SecurityEvent) string {
	// 如果配置了模板，使用模板
	if rule.TemplateID > 0 {
		var tpl models.MessageTemplate
		if err := storage.DB.First(&tpl, rule.TemplateID).Error; err == nil {
			content := tpl.Content
			content = strings.Replace(content, "{{event_type}}", event.EventType, -1)
			content = strings.Replace(content, "{{severity}}", event.Severity, -1)
			content = strings.Replace(content, "{{source_ip}}", event.SourceIP, -1)
			content = strings.Replace(content, "{{username}}", event.Username, -1)
			content = strings.Replace(content, "{{description}}", event.Description, -1)
			content = strings.Replace(content, "{{time}}", event.CreatedAt.Format("2006-01-02 15:04:05"), -1)
			content = strings.Replace(content, "{{rule_name}}", rule.Name, -1)
			return content
		}
	}

	// 默认模板
	severityNames := map[string]string{"low": "低", "medium": "中", "high": "高", "critical": "严重"}
	eventNames := map[string]string{
		"login_failed": "登录失败", "login_blocked": "登录阻止", "login_success": "登录成功",
		"account_locked": "账户锁定", "account_unlocked": "账户解锁",
		"ip_blocked": "IP封禁", "ip_unblocked": "IP解封",
		"password_changed": "密码修改", "config_changed": "配置变更",
		"session_terminated": "会话终止", "suspicious_activity": "可疑活动",
	}

	eventName := eventNames[event.EventType]
	if eventName == "" {
		eventName = event.EventType
	}
	severityName := severityNames[event.Severity]
	if severityName == "" {
		severityName = event.Severity
	}

	return fmt.Sprintf("【安全告警】%s\n规则: %s\n事件: %s\n级别: %s\n用户: %s\nIP: %s\n描述: %s\n时间: %s",
		rule.Name, rule.Name, eventName, severityName,
		event.Username, event.SourceIP, event.Description,
		event.CreatedAt.Format("2006-01-02 15:04:05"))
}

// sendEmailAlert 发送邮件告警
func (s *SecurityService) sendEmailAlert(channel models.NotifyChannel, rule *models.AlertRule, event *models.SecurityEvent, content string) error {
	users := s.getTargetUsers(rule, event)
	var emails []string
	for _, u := range users {
		if u.Email != "" {
			emails = append(emails, u.Email)
		}
	}
	if len(emails) == 0 {
		return fmt.Errorf("no target emails found")
	}

	var config map[string]interface{}
	json.Unmarshal([]byte(channel.Config), &config)

	for _, email := range emails {
		cfg := EmailConfig{
			SMTPHost:     getStr(config, "smtp_host"),
			SMTPPort:     int(getFloat64(config, "smtp_port", 587)),
			SMTPUser:     getStr(config, "smtp_user"),
			SMTPPassword: getStr(config, "smtp_password"),
			SMTPTLS:      getStr(config, "smtp_tls"),
			From:         getStr(config, "from"),
			FromName:     getStr(config, "from_name"),
		}
		subject := fmt.Sprintf("安全告警: %s", rule.Name)
		if err := SendEmail(cfg, email, subject, content); err != nil {
			log.Printf("[告警引擎] 发送邮件到 %s 失败: %v", email, err)
		}
	}
	return nil
}

// sendSMSAlert 发送短信告警
func (s *SecurityService) sendSMSAlert(channel models.NotifyChannel, rule *models.AlertRule, event *models.SecurityEvent, content string) error {
	users := s.getTargetUsers(rule, event)
	var phones []string
	for _, u := range users {
		if u.Phone != "" {
			phones = append(phones, u.Phone)
		}
	}
	if len(phones) == 0 {
		return fmt.Errorf("no target phones found")
	}

	for _, phone := range phones {
		if err := SendSMS(phone, content); err != nil {
			log.Printf("[告警引擎] 发送短信到 %s 失败: %v", phone, err)
		}
	}
	return nil
}

// getTargetUsers 获取告警规则的通知目标用户
func (s *SecurityService) getTargetUsers(rule *models.AlertRule, event *models.SecurityEvent) []models.User {
	var users []models.User

	switch rule.NotifyTarget {
	case "event_user":
		// 员工告警：通知触发事件的用户本人
		if event.UserID != nil {
			var user models.User
			if err := storage.DB.Where("id = ? AND status = ?", *event.UserID, "active").First(&user).Error; err == nil {
				users = append(users, user)
			}
		}
	case "role":
		// 管理员告警：通知指定角色的用户
		var roleIDs []uint
		json.Unmarshal([]byte(rule.NotifyRoleIDs), &roleIDs)
		if len(roleIDs) > 0 {
			storage.DB.Distinct("users.*").
				Joins("JOIN user_roles ON user_roles.user_id = users.id").
				Where("user_roles.role_id IN ? AND users.status = ?", roleIDs, "active").
				Find(&users)
		}
	case "user":
		// 管理员告警：通知指定用户
		var userIDs []uint
		json.Unmarshal([]byte(rule.NotifyUserIDs), &userIDs)
		if len(userIDs) > 0 {
			storage.DB.Where("id IN ? AND status = ?", userIDs, "active").Find(&users)
		}
	default:
		// channel 模式：不需要查用户，直接通过渠道发送（如 webhook 到钉钉群）
	}

	return users
}

// ========== 密码策略 ==========

// ValidatePassword 验证密码是否符合策略
func (s *SecurityService) ValidatePassword(password string) (bool, []string) {
	var errors []string

	config, err := s.GetConfig("password_policy")
	if err != nil {
		return true, nil // 如果获取配置失败，跳过验证
	}

	minLength := int(getFloat64(config, "min_length", 8))
	maxLength := int(getFloat64(config, "max_length", 128))

	if len(password) < minLength {
		errors = append(errors, "密码长度不能少于"+string(rune('0'+minLength))+"位")
	}
	if len(password) > maxLength {
		errors = append(errors, "密码长度不能超过"+string(rune('0'+maxLength))+"位")
	}

	if requireUpper, _ := config["require_uppercase"].(bool); requireUpper {
		if !strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			errors = append(errors, "密码必须包含大写字母")
		}
	}

	if requireLower, _ := config["require_lowercase"].(bool); requireLower {
		if !strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz") {
			errors = append(errors, "密码必须包含小写字母")
		}
	}

	if requireNumber, _ := config["require_number"].(bool); requireNumber {
		if !strings.ContainsAny(password, "0123456789") {
			errors = append(errors, "密码必须包含数字")
		}
	}

	if requireSpecial, _ := config["require_special"].(bool); requireSpecial {
		specialChars, _ := config["special_chars"].(string)
		if specialChars == "" {
			specialChars = "!@#$%^&*()_+-=[]{}|;:,.<>?"
		}
		if !strings.ContainsAny(password, specialChars) {
			errors = append(errors, "密码必须包含特殊字符")
		}
	}

	// 弱密码检查
	if weakCheck, _ := config["weak_password_check"].(bool); weakCheck {
		weakPasswords := []string{"password", "123456", "12345678", "admin", "qwerty", "password123", "admin123"}
		lowerPwd := strings.ToLower(password)
		for _, weak := range weakPasswords {
			if lowerPwd == weak {
				errors = append(errors, "密码过于简单，请使用更复杂的密码")
				break
			}
		}
	}

	return len(errors) == 0, errors
}

// CheckPasswordHistory 检查密码历史
func (s *SecurityService) CheckPasswordHistory(userID uint, newPassword string) (bool, error) {
	config, _ := s.GetConfig("password_policy")
	historyCount := int(getFloat64(config, "history_count", 5))

	if historyCount <= 0 {
		return true, nil
	}

	var user models.User
	if err := storage.DB.First(&user, userID).Error; err != nil {
		return true, nil
	}

	if user.PasswordHistory == "" {
		return true, nil
	}

	var history []string
	if err := json.Unmarshal([]byte(user.PasswordHistory), &history); err != nil {
		return true, nil
	}

	// 检查最近N次密码
	checkCount := historyCount
	if len(history) < checkCount {
		checkCount = len(history)
	}

	for i := len(history) - checkCount; i < len(history); i++ {
		if bcrypt.CompareHashAndPassword([]byte(history[i]), []byte(newPassword)) == nil {
			return false, errors.New("不能使用最近使用过的密码")
		}
	}

	return true, nil
}

// UpdatePasswordHistory 更新密码历史
func (s *SecurityService) UpdatePasswordHistory(userID uint, passwordHash string) error {
	config, _ := s.GetConfig("password_policy")
	historyCount := int(getFloat64(config, "history_count", 5))

	var user models.User
	if err := storage.DB.First(&user, userID).Error; err != nil {
		return err
	}

	var history []string
	if user.PasswordHistory != "" {
		json.Unmarshal([]byte(user.PasswordHistory), &history)
	}

	history = append(history, passwordHash)

	// 只保留最近N次
	if len(history) > historyCount {
		history = history[len(history)-historyCount:]
	}

	historyJSON, _ := json.Marshal(history)
	now := time.Now()

	return storage.DB.Model(&user).Updates(map[string]interface{}{
		"password_history":    string(historyJSON),
		"password_changed_at": now,
	}).Error
}

// ========== 会话管理 ==========

// CreateSession 创建会话
func (s *SecurityService) CreateSession(userID uint, accessToken, refreshToken, ip, userAgent string) (*models.Session, error) {
	config, _ := s.GetConfig("session")

	accessTTL := int(getFloat64(config, "access_token_ttl_minutes", 60))
	refreshTTL := int(getFloat64(config, "refresh_token_ttl_days", 7))
	maxSessions := int(getFloat64(config, "max_concurrent_sessions", 5))
	singleMode, _ := config["single_session_mode"].(bool)

	// 单会话模式：终止其他会话
	if singleMode {
		storage.DB.Model(&models.Session{}).Where("user_id = ? AND is_active = ?", userID, true).
			Update("is_active", false)
	} else {
		// 检查并发会话数
		var count int64
		storage.DB.Model(&models.Session{}).Where("user_id = ? AND is_active = ?", userID, true).Count(&count)
		if int(count) >= maxSessions {
			// 终止最旧的会话
			var oldestSession models.Session
			storage.DB.Where("user_id = ? AND is_active = ?", userID, true).
				Order("created_at ASC").First(&oldestSession)
			storage.DB.Model(&oldestSession).Update("is_active", false)
		}
	}

	session := models.Session{
		ID:           generateSessionID(),
		UserID:       userID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		IPAddress:    ip,
		UserAgent:    userAgent,
		IsActive:     true,
		LastActivity: time.Now(),
		ExpiresAt:    time.Now().Add(time.Duration(refreshTTL) * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	_ = accessTTL // 用于token生成

	if err := storage.DB.Create(&session).Error; err != nil {
		return nil, err
	}

	return &session, nil
}

// TerminateSession 终止会话
func (s *SecurityService) TerminateSession(sessionID string, terminatedBy *uint) error {
	var session models.Session
	if err := storage.DB.First(&session, "id = ?", sessionID).Error; err != nil {
		return err
	}

	if err := storage.DB.Model(&session).Update("is_active", false).Error; err != nil {
		return err
	}

	s.RecordSecurityEvent(models.EventSessionTerminated, models.SeverityLow, session.IPAddress,
		terminatedBy, "", "session", sessionID, "会话被终止", nil)

	return nil
}

// GetUserSessions 获取用户会话列表
func (s *SecurityService) GetUserSessions(userID uint) ([]models.Session, error) {
	var sessions []models.Session
	err := storage.DB.Where("user_id = ? AND is_active = ?", userID, true).
		Order("created_at DESC").Find(&sessions).Error
	return sessions, err
}

// Helper functions
func getFloat64(m map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return defaultVal
}

func generateSessionID() string {
	return time.Now().Format("20060102150405") + randomString(16)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
