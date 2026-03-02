package sms

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Cmcc5GProvider 中国移动5G行业消息（CSP北向接口）实现
// 使用 3.3 单发全网普通消息接口，支持异网短信回落
type Cmcc5GProvider struct {
	cfg ProviderConfig
}

func NewCmcc5GProvider(cfg ProviderConfig) *Cmcc5GProvider {
	return &Cmcc5GProvider{cfg: cfg}
}

func (p *Cmcc5GProvider) Name() string { return "移动5G消息" }

func (p *Cmcc5GProvider) SendWithScene(phone, content, scene string) error {
	return p.Send(phone, content)
}

func (p *Cmcc5GProvider) Send(phone, content string) error {
	if p.cfg.Endpoint == "" {
		return fmt.Errorf("CSP服务器地址未配置")
	}
	if p.cfg.ChatbotURI == "" {
		return fmt.Errorf("Chatbot URI未配置")
	}
	if p.cfg.AppID == "" || p.cfg.Cmcc5GPassword == "" {
		return fmt.Errorf("AppID或开发者密码未配置")
	}

	// 构造 URL
	serverRoot := strings.TrimRight(p.cfg.Endpoint, "/")
	chatbotURI := p.cfg.ChatbotURI
	url := fmt.Sprintf("%s/messaging/single/plain/outbound/%s/requests", serverRoot, chatbotURI)

	// 手机号转 TEL URI 格式
	telURI := formatTelURI(phone)

	// 生成 contributionID
	contributionID := uuid.New().String()

	// 构造 XML 请求体
	xmlBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<msg:outboundMessageRequest xmlns:msg="urn:oma:xml:rest:netapi:messaging:1">
  <destinationAddress>%s</destinationAddress>
  <contentType>text/plain</contentType>
  <bodyText>%s</bodyText>
  <storeSupported>true</storeSupported>
  <shortMessageSupported>true</shortMessageSupported>
  <smsBodyText>%s</smsBodyText>
  <contributionID>%s</contributionID>
  <otherNetSupport>true</otherNetSupport>
  <otherShortMessageSupported>true</otherShortMessageSupported>
  <otherSmsBodyText>%s</otherSmsBodyText>
</msg:outboundMessageRequest>`, telURI, escapeXML(content), escapeXML(content), contributionID, escapeXML(content))

	// 构造鉴权
	dateStr := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
	authorization := p.buildAuthorization(dateStr)

	// 发送请求
	req, err := http.NewRequest("POST", url, strings.NewReader(xmlBody))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Date", dateStr)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求CSP平台失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[移动5G消息] 发送到 %s, 状态: %d, 响应: %s", phone, resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CSP平台返回异常: %d - %s", resp.StatusCode, string(respBody))
	}

	// 检查响应中的 code
	respStr := string(respBody)
	if !strings.Contains(respStr, `"00000"`) && !strings.Contains(respStr, `"code":"00000"`) {
		return fmt.Errorf("CSP平台返回错误: %s", respStr)
	}

	return nil
}

// buildAuthorization 构造 CSP 鉴权 Header
// Authorization = "Basic " + BASE64(appid + ":" + SHA256(token + dateStr))
// token = SHA256(password)
func (p *Cmcc5GProvider) buildAuthorization(dateStr string) string {
	// Step 1: token = SHA256(password)
	token := sha256Hex(p.cfg.Cmcc5GPassword)

	// Step 2: SHA256(token + dateStr)
	signature := sha256Hex(token + dateStr)

	// Step 3: Base64(appid:signature)
	raw := p.cfg.AppID + ":" + signature
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))

	return "Basic " + encoded
}

// sha256Hex 计算 SHA256 并返回小写十六进制字符串
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

// formatTelURI 将手机号转为 TEL URI 格式
func formatTelURI(phone string) string {
	phone = strings.TrimSpace(phone)
	// 去掉已有的前缀
	phone = strings.TrimPrefix(phone, "tel:")
	phone = strings.TrimPrefix(phone, "+86")
	phone = strings.TrimPrefix(phone, "86")
	// 去掉非数字字符
	var digits strings.Builder
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			digits.WriteRune(c)
		}
	}
	return "tel:+86" + digits.String()
}

// escapeXML 转义 XML 特殊字符
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
