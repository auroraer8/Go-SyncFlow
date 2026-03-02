package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"go-syncflow/internal/imclient"
	"go-syncflow/internal/models"
	"go-syncflow/internal/sms"
	"go-syncflow/internal/storage"
)

// ========== 短信发送（dui88 API） ==========

// SMSConfig 短信通道配置
type SMSConfig struct {
	URL          string `json:"url"`
	Sign         string `json:"sign"`
	BodyTemplate string `json:"bodyTemplate"`
}

// getActiveSMSChannel 从数据库获取已启用的短信通道
func getActiveSMSChannel() (*models.NotifyChannel, error) {
	var channel models.NotifyChannel
	smsTypes := []string{
		"sms_aliyun", "sms_tencent", "sms_huawei", "sms_baidu",
		"sms_ctyun", "sms_ronghe", "sms_cmcc", "sms_cmcc_5g",
		"sms_wecom", "sms_dingtalk", "sms_feishu",
		"sms_https", "sms_custom",
	}
	err := storage.DB.Where("channel_type IN ? AND is_active = ?", smsTypes, true).
		First(&channel).Error
	if err != nil {
		return nil, fmt.Errorf("未配置可用的短信通道")
	}
	return &channel, nil
}

// getSMSChannelByScene 根据消息场景获取短信通道（严格遵循消息策略配置，未配置则报错）
func getSMSChannelByScene(scene string) (*models.NotifyChannel, error) {
	channel, err := getChannelByScene(scene, "sms")
	if err != nil {
		return nil, err
	}
	return channel, nil
}

// getChannelByScene 根据消息场景获取指定类型的通道
// channelCategory: "sms" | "dingtalk" | "any"
func getChannelByScene(scene string, channelCategory string) (*models.NotifyChannel, error) {
	// 1. 查找该场景的消息策略
	var policy models.MessagePolicy
	err := storage.DB.Where("scene = ? AND is_active = ?", scene, true).First(&policy).Error
	if err != nil {
		return nil, fmt.Errorf("通知通道未配置")
	}

	// 2. 解析配置的通道 ID 列表
	if policy.ChannelIDs == "" || policy.ChannelIDs == "[]" {
		return nil, fmt.Errorf("通知通道未配置")
	}

	var channelIDs []uint
	if err := json.Unmarshal([]byte(policy.ChannelIDs), &channelIDs); err != nil {
		return nil, fmt.Errorf("通道配置解析失败")
	}

	if len(channelIDs) == 0 {
		return nil, fmt.Errorf("通知通道未配置")
	}

	// 3. 根据类型筛选通道
	var typeFilter []string
	switch channelCategory {
	case "sms":
		typeFilter = []string{
			"sms_aliyun", "sms_tencent", "sms_huawei", "sms_baidu",
			"sms_ctyun", "sms_ronghe", "sms_cmcc", "sms_cmcc_5g",
			"sms_wecom", "sms_dingtalk", "sms_feishu",
			"sms_https", "sms_custom",
		}
	case "dingtalk":
		typeFilter = []string{"dingtalk", "dingtalk_work"}
	case "any":
		typeFilter = nil // 不限类型
	default:
		typeFilter = nil
	}

	var channel models.NotifyChannel
	query := storage.DB.Where("id IN ? AND is_active = ?", channelIDs, true)
	if len(typeFilter) > 0 {
		query = query.Where("channel_type IN ?", typeFilter)
	}
	err = query.First(&channel).Error
	if err != nil {
		if channelCategory == "sms" {
			return nil, fmt.Errorf("短信通道未配置或不可用")
		}
		return nil, fmt.Errorf("通知通道不可用")
	}

	log.Printf("[通知] 场景 %s 使用通道: %s (ID=%d, 类型=%s)", scene, channel.Name, channel.ID, channel.ChannelType)
	return &channel, nil
}

// getChannelsByScene 获取场景配置的所有可用通道
func getChannelsByScene(scene string) ([]models.NotifyChannel, error) {
	var policy models.MessagePolicy
	err := storage.DB.Where("scene = ? AND is_active = ?", scene, true).First(&policy).Error
	if err != nil {
		return nil, fmt.Errorf("通知通道未配置")
	}

	if policy.ChannelIDs == "" || policy.ChannelIDs == "[]" {
		return nil, fmt.Errorf("通知通道未配置")
	}

	var channelIDs []uint
	if err := json.Unmarshal([]byte(policy.ChannelIDs), &channelIDs); err != nil {
		return nil, fmt.Errorf("通道配置解析失败")
	}

	if len(channelIDs) == 0 {
		return nil, fmt.Errorf("通知通道未配置")
	}

	var channels []models.NotifyChannel
	err = storage.DB.Where("id IN ? AND is_active = ?", channelIDs, true).Find(&channels).Error
	if err != nil || len(channels) == 0 {
		return nil, fmt.Errorf("通知通道不可用")
	}

	return channels, nil
}

// getActiveSMSConfig 从数据库获取已启用的短信通道配置（兼容旧调用）
func getActiveSMSConfig() (*SMSConfig, error) {
	channel, err := getActiveSMSChannel()
	if err != nil {
		return nil, err
	}

	var cfg SMSConfig
	if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
		return nil, fmt.Errorf("短信通道配置格式错误: %v", err)
	}

	return &cfg, nil
}

// SendSMS 发送短信（统一入口，自动路由到对应提供商）
func SendSMS(phone, content string) error {
	channel, err := getActiveSMSChannel()
	if err != nil {
		return err
	}
	return sendSMSViaChannel(channel, phone, content)
}

// SendSMSByScene 根据消息场景发送短信（遵循消息策略配置的通道）
func SendSMSByScene(scene, phone, content string) error {
	channel, err := getSMSChannelByScene(scene)
	if err != nil {
		return err
	}
	return sendSMSViaChannel(channel, phone, content)
}

// sendSMSViaChannel 通过指定通道发送短信
func sendSMSViaChannel(channel *models.NotifyChannel, phone, content string) error {
	// 使用新 Provider 系统
	if sms.IsSMSChannel(channel.ChannelType) {
		var cfg sms.ProviderConfig
		if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
			return fmt.Errorf("短信通道配置格式错误: %v", err)
		}
		provider, err := sms.NewProvider(channel.ChannelType, cfg)
		if err != nil {
			return err
		}
		log.Printf("[短信] 使用 %s 发送到 %s", provider.Name(), phone)
		if err := provider.Send(phone, content); err != nil {
			log.Printf("[短信] %s 发送失败: %v", provider.Name(), err)
			return err
		}
		log.Printf("[短信] %s 发送成功 -> %s", provider.Name(), phone)
		return nil
	}

	// 后备：旧的 HTTPS 发送逻辑
	return sendSMSLegacy(phone, content)
}

// sendSMSLegacy 旧版 HTTPS 短信发送（仅兼容用途）
func sendSMSLegacy(phone, content string) error {
	cfg, err := getActiveSMSConfig()
	if err != nil {
		return err
	}

	jsonEscape := func(s string) string {
		b, _ := json.Marshal(s)
		return string(b[1 : len(b)-1])
	}

	body := cfg.BodyTemplate
	if body == "" {
		body = fmt.Sprintf(`{"sign":"%s","mobile":"%s","content":"%s"}`,
			jsonEscape(cfg.Sign), jsonEscape(phone), jsonEscape(content))
	} else {
		body = strings.ReplaceAll(body, "{{phone}}", jsonEscape(phone))
		body = strings.ReplaceAll(body, "{{mobile}}", jsonEscape(phone))
		body = strings.ReplaceAll(body, "{{message}}", jsonEscape(content))
		body = strings.ReplaceAll(body, "{{content}}", jsonEscape(content))
		body = strings.ReplaceAll(body, "{{time}}", jsonEscape(time.Now().Format("2006-01-02 15:04:05")))
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", cfg.URL, bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("创建短信请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送短信请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[短信] 发送到 %s, 状态: %d, 响应: %s", phone, resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("短信API返回异常状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ========== IM 工作消息发送（统一从连接器读取配置） ==========

// sendIMMessage 统一 IM 消息发送（从 connectors 表读取配置）
// platformType: "im_dingtalk" / "im_feishu" / "im_wechatwork"
// userIMID: 用户在IM平台的ID，为空时通过 mobile 查询
// mobile: 用户手机号，当 userIMID 为空时作为 fallback 查询IM用户ID
func sendIMMessage(platformType string, userIMID string, mobile string, content string) error {
	platformName := map[string]string{
		"im_dingtalk":   "钉钉",
		"im_feishu":     "飞书",
		"im_wechatwork": "企业微信",
	}[platformType]
	if platformName == "" {
		platformName = platformType
	}

	var conn models.Connector
	if err := storage.DB.Where("type = ? AND status = 1", platformType).
		First(&conn).Error; err != nil {
		return fmt.Errorf("%s连接器未配置或未启用，请在上游连接器中添加", platformName)
	}
	client, err := imclient.NewIMClient(conn)
	if err != nil {
		return fmt.Errorf("创建IM客户端失败: %v", err)
	}

	if userIMID == "" && mobile != "" {
		log.Printf("[IM消息] %s: 用户无IM ID，尝试通过手机号 %s 查询", platformName, mobile)
		uid, err := client.GetUserIDByMobile(mobile)
		if err != nil {
			return fmt.Errorf("通过手机号查询%s用户失败: %v", platformName, err)
		}
		userIMID = uid
		log.Printf("[IM消息] %s: 手机号 %s → 用户ID %s", platformName, mobile, userIMID)
	}

	if userIMID == "" {
		return fmt.Errorf("无法确定%s接收用户（无IM ID且无手机号）", platformName)
	}

	return client.SendMessage(userIMID, content)
}

// IsIMConnectorAvailable 检查指定 IM 平台是否有已启用的连接器
func IsIMConnectorAvailable(platformType string) bool {
	var count int64
	storage.DB.Model(&models.Connector{}).Where("type = ? AND status = 1", platformType).Count(&count)
	return count > 0
}

// IsSMSChannelAvailable 检查是否有已启用的短信通道
func IsSMSChannelAvailable() bool {
	_, err := getActiveSMSChannel()
	return err == nil
}

// SendDingTalkWorkMessage 发送钉钉工作消息（从连接器读取配置）
func SendDingTalkWorkMessage(dingtalkUID, content string) error {
	return sendIMMessage("im_dingtalk", dingtalkUID, "", content)
}

// ========== 通用通知发送（支持所有渠道类型） ==========

// NotifyResult 单渠道发送结果
type NotifyResult struct {
	Channel string
	Success bool
	Message string
}

// SendNotificationByChannels 根据渠道类型列表向用户发送通知
// user: 目标用户, subject: 邮件主题, content: 消息内容, channelTypes: 渠道类型列表（sms/dingtalk/email/webhook 等）
func SendNotificationByChannels(user models.User, subject, content string, channelTypes []string) []NotifyResult {
	var results []NotifyResult

	for _, ch := range channelTypes {
		normalized := normalizeChannelTypeForSend(ch)

		switch normalized {
		case "sms":
			if user.Phone == "" {
				results = append(results, NotifyResult{Channel: "短信", Success: false, Message: "无手机号"})
				continue
			}
			if err := SendSMS(user.Phone, content); err != nil {
				results = append(results, NotifyResult{Channel: "短信", Success: false, Message: err.Error()})
			} else {
				results = append(results, NotifyResult{Channel: "短信", Success: true, Message: "已发送"})
			}

		case "dingtalk":
			imUID := user.DingTalkUID
			if imUID == "" && user.Phone == "" {
				results = append(results, NotifyResult{Channel: "钉钉", Success: false, Message: "未绑定钉钉且无手机号"})
				continue
			}
			if err := sendIMMessage("im_dingtalk", imUID, user.Phone, content); err != nil {
				results = append(results, NotifyResult{Channel: "钉钉", Success: false, Message: err.Error()})
			} else {
				results = append(results, NotifyResult{Channel: "钉钉", Success: true, Message: "已发送"})
			}

		case "feishu":
			feishuUID := user.IMUserID
			if feishuUID == "" {
				feishuUID = user.DingTalkUID
			}
			if feishuUID == "" && user.Phone == "" {
				results = append(results, NotifyResult{Channel: "飞书", Success: false, Message: "未绑定飞书且无手机号"})
				continue
			}
			if err := sendIMMessage("im_feishu", feishuUID, user.Phone, content); err != nil {
				results = append(results, NotifyResult{Channel: "飞书", Success: false, Message: err.Error()})
			} else {
				results = append(results, NotifyResult{Channel: "飞书", Success: true, Message: "已发送"})
			}

		case "wecom":
			wecomUID := user.IMUserID
			if wecomUID == "" {
				wecomUID = user.DingTalkUID
			}
			if wecomUID == "" && user.Phone == "" {
				results = append(results, NotifyResult{Channel: "企业微信", Success: false, Message: "未绑定企业微信且无手机号"})
				continue
			}
			if err := sendIMMessage("im_wechatwork", wecomUID, user.Phone, content); err != nil {
				results = append(results, NotifyResult{Channel: "企业微信", Success: false, Message: err.Error()})
			} else {
				results = append(results, NotifyResult{Channel: "企业微信", Success: true, Message: "已发送"})
			}

		case "email":
			if user.Email == "" {
				results = append(results, NotifyResult{Channel: "邮件", Success: false, Message: "无邮箱地址"})
				continue
			}
			// 从数据库获取已启用的邮件通道配置
			var emailChannel models.NotifyChannel
			if err := storage.DB.Where("channel_type = ? AND is_active = ?", "email", true).First(&emailChannel).Error; err != nil {
				results = append(results, NotifyResult{Channel: "邮件", Success: false, Message: "未配置可用的邮件通道"})
				continue
			}
			var emailCfg EmailConfig
			if err := json.Unmarshal([]byte(emailChannel.Config), &emailCfg); err != nil {
				results = append(results, NotifyResult{Channel: "邮件", Success: false, Message: "邮件通道配置解析失败"})
				continue
			}
			if subject == "" {
				subject = "统一身份认证平台通知"
			}
			if err := SendEmail(emailCfg, user.Email, subject, content); err != nil {
				results = append(results, NotifyResult{Channel: "邮件", Success: false, Message: err.Error()})
			} else {
				results = append(results, NotifyResult{Channel: "邮件", Success: true, Message: "已发送"})
			}

		case "webhook":
			// 从数据库获取已启用的 Webhook 通道配置
			var whChannel models.NotifyChannel
			if err := storage.DB.Where("channel_type = ? AND is_active = ?", "webhook", true).First(&whChannel).Error; err != nil {
				results = append(results, NotifyResult{Channel: "Webhook", Success: false, Message: "未配置可用的Webhook通道"})
				continue
			}
			var whCfg WebhookConfig
			if err := json.Unmarshal([]byte(whChannel.Config), &whCfg); err != nil {
				results = append(results, NotifyResult{Channel: "Webhook", Success: false, Message: "Webhook配置解析失败"})
				continue
			}
			// Webhook body 模板中额外替换用户信息
			if whCfg.BodyTemplate != "" {
				whCfg.BodyTemplate = strings.ReplaceAll(whCfg.BodyTemplate, "{{username}}", user.Username)
				whCfg.BodyTemplate = strings.ReplaceAll(whCfg.BodyTemplate, "{{nickname}}", user.Nickname)
				whCfg.BodyTemplate = strings.ReplaceAll(whCfg.BodyTemplate, "{{email}}", user.Email)
				whCfg.BodyTemplate = strings.ReplaceAll(whCfg.BodyTemplate, "{{phone}}", user.Phone)
			}
			if err := SendWebhook(whCfg, content); err != nil {
				results = append(results, NotifyResult{Channel: "Webhook", Success: false, Message: err.Error()})
			} else {
				results = append(results, NotifyResult{Channel: "Webhook", Success: true, Message: "已发送"})
			}

		default:
			results = append(results, NotifyResult{Channel: ch, Success: false, Message: "不支持的通道类型"})
		}
	}

	// 记录通知发送日志到操作日志表
	logNotifyResults(user, subject, results)

	return results
}

// logNotifyResults 记录通知发送结果到操作日志
func logNotifyResults(user models.User, subject string, results []NotifyResult) {
	for _, r := range results {
		target := user.Username
		if user.Nickname != "" {
			target = user.Nickname + "(" + user.Username + ")"
		}
		content := fmt.Sprintf("渠道: %s, 场景: %s", r.Channel, subject)
		if r.Success {
			content += ", 结果: 发送成功"
		} else {
			content += ", 结果: 发送失败 - " + r.Message
		}
		if user.Phone != "" && (r.Channel == "短信" || r.Channel == "移动5G消息") {
			content += ", 手机号: " + user.Phone
		}

		opLog := models.OperationLog{
			UserID:   0, // 系统操作
			Username: "system",
			Module:   "消息通知",
			Action:   "发送通知",
			Target:   target,
			Content:  content,
		}
		storage.DB.Create(&opLog)
	}
}

// FormatNotifyResults 格式化通知结果为字符串
func FormatNotifyResults(results []NotifyResult) string {
	parts := make([]string, 0, len(results))
	for _, r := range results {
		if r.Success {
			parts = append(parts, r.Channel+": "+r.Message)
		} else {
			parts = append(parts, r.Channel+": 失败-"+r.Message)
		}
	}
	return strings.Join(parts, "; ")
}

// normalizeChannelTypeForSend 标准化通道类型用于发送
func normalizeChannelTypeForSend(ch string) string {
	if strings.HasPrefix(ch, "sms") {
		return "sms"
	}
	if strings.HasPrefix(ch, "dingtalk") {
		return "dingtalk"
	}
	if strings.HasPrefix(ch, "feishu") {
		return "feishu"
	}
	if strings.HasPrefix(ch, "wecom") {
		return "wecom"
	}
	return ch
}

// ========== 高级封装：验证码发送 ==========

// getVerifyCodeContent 获取验证码消息内容（优先使用消息模板）
func getVerifyCodeContent(code, username, nickname string) string {
	var tpl models.MessageTemplate
	if storage.DB.Where("scene = ?", "verify_code").First(&tpl).Error != nil {
		log.Printf("[通知] 消息模板 verify_code 不存在，请在消息模板管理中创建")
		return ""
	}
	content := tpl.Content
	content = strings.ReplaceAll(content, "{{code}}", code)
	content = strings.ReplaceAll(content, "{{username}}", username)
	content = strings.ReplaceAll(content, "{{nickname}}", nickname)
	content = strings.ReplaceAll(content, "{{name}}", nickname)
	content = strings.ReplaceAll(content, "{{expire}}", "5")
	content = strings.ReplaceAll(content, "{{time}}", "5分钟")
	content = strings.ReplaceAll(content, "{{app_name}}", "统一身份认证平台")
	return content
}

// VerifyCodeResult 验证码发送结果
type VerifyCodeResult struct {
	ChannelType string // sms, dingtalk, feishu, email
	Target      string // 脱敏后的目标，如 131****7555 或 钉钉工作通知
}

// SendVerifyCodeSMS 发送短信验证码（遵循 verify_code 场景的消息策略配置）
// 支持短信通道、钉钉通道、飞书通道，根据配置自动选择
// 返回发送结果（通道类型和目标）
func SendVerifyCodeSMS(phone, code, username, nickname string) (*VerifyCodeResult, error) {
	content := getVerifyCodeContent(code, username, nickname)
	if content == "" {
		return nil, fmt.Errorf("消息模板未配置")
	}

	// 获取场景配置的所有通道
	channels, err := getChannelsByScene("verify_code")
	if err != nil {
		log.Printf("[验证码] 获取通道失败: %v", err)
		return nil, err
	}

	// 遍历通道，尝试发送
	var lastErr error
	for _, channel := range channels {
		channelType := channel.ChannelType

		// 短信类型通道
		if strings.HasPrefix(channelType, "sms_") {
			if phone == "" {
				log.Printf("[验证码] 跳过短信通道 %s: 用户无手机号", channel.Name)
				lastErr = fmt.Errorf("用户未绑定手机号")
				continue
			}
			err := sendSMSViaChannel(&channel, phone, content)
			if err != nil {
				log.Printf("[验证码-短信] %s 发送失败: %v", channel.Name, err)
				lastErr = fmt.Errorf("短信发送失败")
				continue
			}
			log.Printf("[验证码-短信] %s 发送成功 -> %s", channel.Name, phone)
			return &VerifyCodeResult{
				ChannelType: "sms",
				Target:      maskPhone(phone),
			}, nil
		}

		// 钉钉类型通道
		if strings.HasPrefix(channelType, "dingtalk") {
			var user models.User
			if err := storage.DB.Where("username = ?", username).First(&user).Error; err != nil {
				log.Printf("[验证码] 跳过钉钉通道: 用户不存在")
				lastErr = fmt.Errorf("用户不存在")
				continue
			}
			if user.DingTalkUID == "" && user.Phone == "" {
				log.Printf("[验证码] 跳过钉钉通道 %s: 用户未绑定钉钉且无手机号", channel.Name)
				lastErr = fmt.Errorf("用户未绑定钉钉且无手机号")
				continue
			}
			err := sendIMMessage("im_dingtalk", user.DingTalkUID, user.Phone, content)
			if err != nil {
				log.Printf("[验证码-钉钉] %s 发送失败: %v", channel.Name, err)
				lastErr = fmt.Errorf("钉钉消息发送失败")
				continue
			}
			log.Printf("[验证码-钉钉] %s 发送成功 -> 用户 %s", channel.Name, username)
			return &VerifyCodeResult{
				ChannelType: "dingtalk",
				Target:      "钉钉工作通知",
			}, nil
		}

		// 飞书类型通道
		if strings.HasPrefix(channelType, "feishu") {
			var user models.User
			if err := storage.DB.Where("username = ?", username).First(&user).Error; err != nil {
				log.Printf("[验证码] 跳过飞书通道: 用户不存在")
				lastErr = fmt.Errorf("用户不存在")
				continue
			}
			feishuUserID := user.IMUserID
			if feishuUserID == "" {
				feishuUserID = user.DingTalkUID
			}
			if feishuUserID == "" && user.Phone == "" {
				log.Printf("[验证码] 跳过飞书通道 %s: 用户未绑定飞书且无手机号", channel.Name)
				lastErr = fmt.Errorf("用户未绑定飞书且无手机号")
				continue
			}
			err := sendIMMessage("im_feishu", feishuUserID, user.Phone, content)
			if err != nil {
				log.Printf("[验证码-飞书] %s 发送失败: %v", channel.Name, err)
				lastErr = fmt.Errorf("飞书消息发送失败")
				continue
			}
			log.Printf("[验证码-飞书] %s 发送成功 -> 用户 %s", channel.Name, username)
			return &VerifyCodeResult{
				ChannelType: "feishu",
				Target:      "飞书消息",
			}, nil
		}

		// 企业微信类型通道
		if strings.HasPrefix(channelType, "wecom") {
			var user models.User
			if err := storage.DB.Where("username = ?", username).First(&user).Error; err != nil {
				log.Printf("[验证码] 跳过企业微信通道: 用户不存在")
				lastErr = fmt.Errorf("用户不存在")
				continue
			}
			wecomUserID := user.IMUserID
			if wecomUserID == "" {
				wecomUserID = user.DingTalkUID
			}
			if wecomUserID == "" && user.Phone == "" {
				log.Printf("[验证码] 跳过企业微信通道 %s: 用户未绑定企微且无手机号", channel.Name)
				lastErr = fmt.Errorf("用户未绑定企业微信且无手机号")
				continue
			}
			err := sendIMMessage("im_wechatwork", wecomUserID, user.Phone, content)
			if err != nil {
				log.Printf("[验证码-企业微信] %s 发送失败: %v", channel.Name, err)
				lastErr = fmt.Errorf("企业微信消息发送失败")
				continue
			}
			log.Printf("[验证码-企业微信] %s 发送成功 -> 用户 %s", channel.Name, username)
			return &VerifyCodeResult{
				ChannelType: "wecom",
				Target:      "企业微信消息",
			}, nil
		}

		// 邮件类型通道（暂未实现验证码场景）
		if channelType == "email" {
			log.Printf("[验证码] 跳过邮件通道 %s: 暂未支持", channel.Name)
			lastErr = fmt.Errorf("邮件通道暂未支持验证码")
			continue
		}

		log.Printf("[验证码] 跳过不支持的通道类型: %s", channelType)
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("无可用通知通道")
}

// maskPhone 手机号脱敏 131****7555
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// maskEmail 邮箱脱敏 xx****@domain.com
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}
	name := parts[0]
	if len(name) <= 2 {
		return name + "****@" + parts[1]
	}
	return name[:2] + "****@" + parts[1]
}

// SendVerifyCodeDingTalk 发送钉钉验证码
func SendVerifyCodeDingTalk(dingtalkUID, username, nickname, code string) error {
	content := getVerifyCodeContent(code, username, nickname)
	err := SendDingTalkWorkMessage(dingtalkUID, content)
	if err != nil {
		log.Printf("[验证码-钉钉] 发送失败 -> 用户 %s: %v", username, err)
		return err
	}
	log.Printf("[验证码-钉钉] 发送成功 -> 用户 %s", username)
	return nil
}

// SendSecurityAlertSMS 发送安全告警短信
func SendSecurityAlertSMS(phone, message string) {
	if phone == "" {
		return
	}
	if err := SendSMS(phone, message); err != nil {
		log.Printf("[安全通知-短信] 发送失败 -> %s: %v", phone, err)
	} else {
		log.Printf("[安全通知-短信] 发送成功 -> %s", phone)
	}
}

// SendSecurityAlertDingTalk 发送安全告警钉钉消息（从连接器读取配置）
func SendSecurityAlertDingTalk(dingtalkUID, username, message string) {
	if dingtalkUID == "" {
		return
	}
	if err := sendIMMessage("im_dingtalk", dingtalkUID, "", message); err != nil {
		log.Printf("[安全通知-钉钉] 发送失败 -> %s: %v", username, err)
	} else {
		log.Printf("[安全通知-钉钉] 发送成功 -> %s", username)
	}
}

// ========== 通道测试 ==========

// TestSMSChannelWithMessage 测试短信通道（自定义消息内容）— 支持所有 SMS Provider
func TestSMSChannelWithMessage(channelType, channelConfig string, testPhone string, message string) error {
	if testPhone == "" {
		return fmt.Errorf("测试手机号未配置")
	}

	// 优先使用新 Provider 系统
	if sms.IsSMSChannel(channelType) {
		var cfg sms.ProviderConfig
		if err := json.Unmarshal([]byte(channelConfig), &cfg); err != nil {
			return fmt.Errorf("配置解析失败: %v", err)
		}
		provider, err := sms.NewProvider(channelType, cfg)
		if err != nil {
			return err
		}
		return provider.Send(testPhone, message)
	}

	// 后备旧逻辑
	var cfg SMSConfig
	if err := json.Unmarshal([]byte(channelConfig), &cfg); err != nil {
		return fmt.Errorf("配置解析失败: %v", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("请求地址未配置")
	}

	body := cfg.BodyTemplate
	if body == "" {
		body = fmt.Sprintf(`{"sign":"%s","mobile":"%s","content":"%s"}`, cfg.Sign, testPhone, message)
	} else {
		body = strings.ReplaceAll(body, "{{phone}}", testPhone)
		body = strings.ReplaceAll(body, "{{mobile}}", testPhone)
		body = strings.ReplaceAll(body, "{{message}}", message)
		body = strings.ReplaceAll(body, "{{content}}", message)
		body = strings.ReplaceAll(body, "{{time}}", time.Now().Format("2006-01-02 15:04:05"))
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", cfg.URL, bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API返回异常: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// TestIMChannelWithMessage 测试 IM 工作消息通道（自定义消息内容，从连接器读取配置）
func TestIMChannelWithMessage(platformType string, userID string, message string) error {
	return sendIMMessage(platformType, userID, "", message)
}

// TestDingTalkChannelWithMessage 测试钉钉工作消息（兼容旧调用）
func TestDingTalkChannelWithMessage(userID string, message string) error {
	return TestIMChannelWithMessage("im_dingtalk", userID, message)
}

// TestSMSChannel 测试短信通道（发送一条测试短信）
func TestSMSChannel(channelType, channelConfig string, testPhone string) error {
	content := "【测试】这是一条通道测试短信，收到请忽略。"
	return TestSMSChannelWithMessage(channelType, channelConfig, testPhone, content)
}

// TestDingTalkChannel 测试钉钉工作消息通道（从连接器读取配置）
func TestDingTalkChannel(testUserID string) error {
	if testUserID == "" {
		var admin models.User
		if err := storage.DB.Where("username = 'admin' AND is_deleted = 0 AND ding_talk_uid != ''").First(&admin).Error; err != nil {
			return fmt.Errorf("请指定测试用户的钉钉UserID")
		}
		testUserID = admin.DingTalkUID
	}
	return sendIMMessage("im_dingtalk", testUserID, "", "【测试】这是一条钉钉工作消息通道测试，收到请忽略。")
}

// ========== 默认变量定义 ==========

var defaultVarsJSON string

func init() {
	vars := []map[string]string{
		{"key": "username", "desc": "用户名", "example": "zhangsan"},
		{"key": "nickname", "desc": "用户昵称", "example": "张三"},
		{"key": "name", "desc": "姓名（真实姓名）", "example": "张三"},
		{"key": "code", "desc": "验证码", "example": "283746"},
		{"key": "time", "desc": "当前时间", "example": "2026-02-08 11:30:00"},
		{"key": "ip", "desc": "来源IP地址", "example": "192.168.1.100"},
		{"key": "app_name", "desc": "系统名称", "example": "统一身份认证平台"},
	}
	data, _ := json.Marshal(vars)
	defaultVarsJSON = string(data)
}

// InitDefaultTemplates 初始化默认消息模板
func InitDefaultTemplates() {
	templates := []models.MessageTemplate{
		{
			Name:      "验证码通知",
			Scene:     "verify_code",
			Content:   "【{{app_name}}】您的验证码是 {{code}}，5分钟内有效，请勿泄露给他人。",
			Variables: defaultVarsJSON,
			IsBuiltin: true,
			IsActive:  true,
		},
		{
			Name:      "密码重置通知",
			Scene:     "password_reset",
			Content:   "【{{app_name}}】您正在重置密码，验证码为 {{code}}，5分钟内有效。如非本人操作，请忽略此消息。",
			Variables: defaultVarsJSON,
			IsBuiltin: true,
			IsActive:  true,
		},
		{
			Name:      "安全告警通知（员工）",
			Scene:     "security_alert",
			Content:   "【安全提醒】您的账号（{{username}}）正在被频繁请求验证码重置密码。如非本人操作，请立即联系管理员。来源IP: {{ip}}",
			Variables: defaultVarsJSON,
			IsBuiltin: true,
			IsActive:  true,
		},
		{
			Name:      "管理员安全告警",
			Scene:     "admin_alert",
			Content:   "【安全警告】用户 {{username}} 的验证码请求触发频率上限，来源IP: {{ip}}，请核查是否为恶意攻击。",
			Variables: defaultVarsJSON,
			IsBuiltin: true,
			IsActive:  true,
		},
		{
			Name:      "密码被重置通知",
			Scene:     "password_reset_notify",
			Content:   "【{{app_name}}】您的账号（{{username}}）密码已被管理员重置，新密码为：{{password}}，请尽快登录并修改密码。如非本人操作请立即联系管理员。",
			Variables: `[{"key":"username","desc":"用户名","example":"zhangsan"},{"key":"nickname","desc":"用户昵称","example":"张三"},{"key":"name","desc":"姓名","example":"张三"},{"key":"password","desc":"新密码","example":"Abc@1234"},{"key":"time","desc":"当前时间","example":"2026-02-08 12:00:00"},{"key":"app_name","desc":"系统名称","example":"统一身份认证平台"}]`,
			IsBuiltin: true,
			IsActive:  true,
		},
		{
			Name:      "账号开通通知",
			Scene:     "account_created",
			Content:   "【{{app_name}}】您的账号已开通，用户名：{{username}}，初始密码：{{password}}。请尽快登录并修改密码。如有疑问请联系管理员。",
			Variables: `[{"key":"username","desc":"用户名","example":"zhangsan"},{"key":"nickname","desc":"用户昵称","example":"张三"},{"key":"name","desc":"姓名","example":"张三"},{"key":"password","desc":"初始密码","example":"Abc@1234"},{"key":"department","desc":"部门","example":"技术部"},{"key":"time","desc":"当前时间","example":"2026-02-10 12:00:00"},{"key":"app_name","desc":"系统名称","example":"统一身份认证平台"}]`,
			IsBuiltin: true,
			IsActive:  true,
		},
		{
			Name:      "测试消息",
			Scene:     "test",
			Content:   "【测试】这是一条来自 {{app_name}} 的测试消息，发送时间: {{time}}，收到请忽略。",
			Variables: defaultVarsJSON,
			IsBuiltin: true,
			IsActive:  true,
		},
	}

	for _, tpl := range templates {
		var existing models.MessageTemplate
		if storage.DB.Where("scene = ?", tpl.Scene).First(&existing).Error == nil {
			continue
		}
		if err := storage.DB.Create(&tpl).Error; err != nil {
			log.Printf("[通知] 创建默认模板 %s 失败: %v", tpl.Name, err)
		} else {
			log.Printf("[通知] 已创建默认模板: %s", tpl.Name)
		}
	}
}

// ========== 默认通道初始化 ==========

// InitDefaultChannels 通知渠道初始化（不创建任何默认通道，由管理员在界面中配置）
func InitDefaultChannels() {
	// 通知渠道完全由管理员在「通知管理 → 通知渠道」中手动配置
	// 打包时数据库不包含在内，新部署默认无通知渠道
}

// ========== 邮件发送 ==========

// EmailConfig 邮件通道配置
type EmailConfig struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUser     string `json:"smtp_user"`
	SMTPPassword string `json:"smtp_password"`
	SMTPTLS      string `json:"smtp_tls"` // ssl / starttls / none
	From         string `json:"from"`
	FromName     string `json:"from_name"`
	Recipients   string `json:"recipients"`
}

// SendEmail 发送邮件
func SendEmail(cfg EmailConfig, to, subject, body string) error {
	if cfg.SMTPHost == "" || cfg.From == "" {
		return fmt.Errorf("SMTP配置不完整")
	}

	// 构建邮件内容
	fromHeader := cfg.From
	if cfg.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", cfg.FromName, cfg.From)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		fromHeader, to, subject, body)

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)

	switch cfg.SMTPTLS {
	case "ssl":
		// SSL/TLS 直连 (端口 465)
		tlsConfig := &tls.Config{ServerName: cfg.SMTPHost}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("TLS连接失败: %v", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, cfg.SMTPHost)
		if err != nil {
			return fmt.Errorf("创建SMTP客户端失败: %v", err)
		}
		defer client.Close()

		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP认证失败: %v", err)
		}
		if err := client.Mail(cfg.From); err != nil {
			return fmt.Errorf("设置发件人失败: %v", err)
		}
		for _, rcpt := range strings.Split(to, ",") {
			rcpt = strings.TrimSpace(rcpt)
			if rcpt != "" {
				if err := client.Rcpt(rcpt); err != nil {
					return fmt.Errorf("设置收件人失败(%s): %v", rcpt, err)
				}
			}
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("发送Data命令失败: %v", err)
		}
		if _, err := w.Write([]byte(msg)); err != nil {
			return fmt.Errorf("写入邮件内容失败: %v", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("关闭Data失败: %v", err)
		}
		return client.Quit()

	case "starttls":
		// STARTTLS (端口 587)
		return smtp.SendMail(addr, auth, cfg.From, strings.Split(to, ","), []byte(msg))

	default:
		// 无加密 (端口 25)
		return smtp.SendMail(addr, auth, cfg.From, strings.Split(to, ","), []byte(msg))
	}
}

// TestEmailChannel 测试邮件通道
func TestEmailChannel(channelConfig string, recipient string, message string) error {
	var cfg EmailConfig
	if err := json.Unmarshal([]byte(channelConfig), &cfg); err != nil {
		return fmt.Errorf("配置解析失败: %v", err)
	}
	if recipient == "" {
		recipient = cfg.Recipients
	}
	if recipient == "" {
		return fmt.Errorf("请指定收件人邮箱")
	}
	subject := "【测试】通道测试邮件"
	if message == "" {
		message = "这是一封来自统一身份认证平台的测试邮件，收到请忽略。"
	}
	return SendEmail(cfg, recipient, subject, message)
}

// ========== Webhook 发送 ==========

// WebhookConfig Webhook通道配置
type WebhookConfig struct {
	URL           string `json:"url"`
	Method        string `json:"method"`
	BodyTemplate  string `json:"bodyTemplate"`
	SignType      string `json:"signType"`      // none / hmac_sha256 / token
	SignSecret    string `json:"signSecret"`     // HMAC密钥
	SignHeader    string `json:"signHeader"`     // 签名放在哪个header
	TokenValue    string `json:"tokenValue"`     // Token值
	TokenPosition string `json:"tokenPosition"`  // header / query
	TokenHeader   string `json:"tokenHeader"`    // Token header名称
}

// SendWebhook 发送Webhook
func SendWebhook(cfg WebhookConfig, message string) error {
	if cfg.URL == "" {
		return fmt.Errorf("Webhook URL未配置")
	}

	method := cfg.Method
	if method == "" {
		method = "POST"
	}

	body := cfg.BodyTemplate
	if body == "" {
		payload := map[string]string{
			"message": message,
			"time":    time.Now().Format("2006-01-02 15:04:05"),
		}
		data, _ := json.Marshal(payload)
		body = string(data)
	} else {
		body = strings.ReplaceAll(body, "{{message}}", message)
		body = strings.ReplaceAll(body, "{{time}}", time.Now().Format("2006-01-02 15:04:05"))
	}

	targetURL := cfg.URL
	// Token 放在 URL 参数
	if cfg.SignType == "token" && cfg.TokenPosition == "query" {
		sep := "?"
		if strings.Contains(targetURL, "?") {
			sep = "&"
		}
		targetURL = targetURL + sep + "token=" + cfg.TokenValue
	}

	req, err := http.NewRequest(method, targetURL, bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// HMAC-SHA256 签名
	if cfg.SignType == "hmac_sha256" && cfg.SignSecret != "" {
		mac := hmac.New(sha256.New, []byte(cfg.SignSecret))
		mac.Write([]byte(body))
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		header := cfg.SignHeader
		if header == "" {
			header = "X-Signature-256"
		}
		req.Header.Set(header, signature)
	}

	// Token 放在 Header
	if cfg.SignType == "token" && cfg.TokenPosition == "header" {
		header := cfg.TokenHeader
		if header == "" {
			header = "Authorization"
		}
		req.Header.Set(header, cfg.TokenValue)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Webhook返回异常: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// TestWebhookChannel 测试Webhook通道
func TestWebhookChannel(channelConfig string, message string) error {
	var cfg WebhookConfig
	if err := json.Unmarshal([]byte(channelConfig), &cfg); err != nil {
		return fmt.Errorf("配置解析失败: %v", err)
	}
	if message == "" {
		message = "【测试】这是一条Webhook测试消息，收到请忽略。"
	}
	return SendWebhook(cfg, message)
}

// SendWebhookAlert 发送安全告警到 Webhook（支持钉钉群机器人格式）
func SendWebhookAlert(channelConfig string, content string, event *models.SecurityEvent) error {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(channelConfig), &config); err != nil {
		return fmt.Errorf("配置解析失败: %v", err)
	}

	url := getStr(config, "url")
	if url == "" {
		return fmt.Errorf("webhook URL 为空")
	}

	// 检测是否是钉钉机器人 Webhook
	isDingTalk := strings.Contains(url, "oapi.dingtalk.com/robot")

	var body []byte
	if isDingTalk {
		// 钉钉机器人消息格式（Markdown）
		severityEmoji := map[string]string{"low": "ℹ️", "medium": "⚠️", "high": "🔴", "critical": "🚨"}
		emoji := severityEmoji[event.Severity]
		if emoji == "" {
			emoji = "⚠️"
		}

		markdown := fmt.Sprintf("### %s 安全告警\n\n"+
			"- **事件**: %s\n"+
			"- **级别**: %s\n"+
			"- **用户**: %s\n"+
			"- **IP**: %s\n"+
			"- **描述**: %s\n"+
			"- **时间**: %s\n",
			emoji, event.EventType, event.Severity,
			event.Username, event.SourceIP, event.Description,
			event.CreatedAt.Format("2006-01-02 15:04:05"))

		msg := map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": fmt.Sprintf("安全告警: %s", event.EventType),
				"text":  markdown,
			},
		}
		body, _ = json.Marshal(msg)
	} else {
		// 通用 Webhook
		cfg := WebhookConfig{
			URL:    url,
			Method: getStr(config, "method"),
		}
		if cfg.Method == "" {
			cfg.Method = "POST"
		}
		cfg.SignType = getStr(config, "signType")
		cfg.SignSecret = getStr(config, "signSecret")
		cfg.SignHeader = getStr(config, "signHeader")
		cfg.TokenValue = getStr(config, "tokenValue")
		cfg.TokenPosition = getStr(config, "tokenPosition")
		cfg.TokenHeader = getStr(config, "tokenHeader")
		return SendWebhook(cfg, content)
	}

	// 钉钉机器人：处理加签
	signType := getStr(config, "signType")
	if signType == "hmac_sha256" {
		secret := getStr(config, "signSecret")
		if secret != "" {
			timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
			stringToSign := timestamp + "\n" + secret
			h := hmacSha256([]byte(secret), []byte(stringToSign))
			sign := base64Encode(h)
			if strings.Contains(url, "?") {
				url += "&timestamp=" + timestamp + "&sign=" + sign
			} else {
				url += "?timestamp=" + timestamp + "&sign=" + sign
			}
		}
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook 请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回异常: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// hmacSha256 计算 HMAC-SHA256
func hmacSha256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// base64Encode Base64 编码
func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// getStr 从 map 获取字符串
func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
