package sms

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HuaweiProvider 华为云短信实现
type HuaweiProvider struct {
	cfg ProviderConfig
}

func NewHuaweiProvider(cfg ProviderConfig) *HuaweiProvider {
	return &HuaweiProvider{cfg: cfg}
}

func (p *HuaweiProvider) Name() string { return "华为云短信" }

func (p *HuaweiProvider) SendWithScene(phone, content, scene string) error {
	return p.Send(phone, content)
}

func (p *HuaweiProvider) Send(phone, content string) error {
	if p.cfg.AppKey == "" || p.cfg.AppSecret == "" {
		return fmt.Errorf("AppKey 或 AppSecret 未配置")
	}
	if p.cfg.Channel == "" {
		return fmt.Errorf("通道号未配置")
	}
	if p.cfg.TemplateID == "" {
		return fmt.Errorf("模板ID未配置")
	}

	endpoint := p.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://smsapi.cn-north-4.myhuaweicloud.com:443/sms/batchSendSms/v1"
	}

	if !strings.HasPrefix(phone, "+") {
		phone = "+86" + phone
	}

	// WSSE 认证
	nonce := fmt.Sprintf("%x", time.Now().UnixNano())
	created := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	digest := sha256hex([]byte(nonce + created + p.cfg.AppSecret))

	wsseHeader := fmt.Sprintf(`UsernameToken Username="%s", PasswordDigest="%s", Nonce="%s", Created="%s"`,
		p.cfg.AppKey, digest, nonce, created)

	form := url.Values{}
	form.Set("from", p.cfg.Channel)
	form.Set("to", phone)
	form.Set("templateId", p.cfg.TemplateID)
	form.Set("templateParas", fmt.Sprintf(`["%s"]`, content))
	if p.cfg.SignName != "" {
		form.Set("signature", p.cfg.SignName)
	}

	req, _ := http.NewRequest("POST", endpoint, bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", `WSSE realm="SDP",profile="UsernameToken",type="Appkey"`)
	req.Header.Set("X-WSSE", wsseHeader)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求华为云短信API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析华为云响应失败: %s", string(body))
	}

	if result.Code != "000000" {
		return fmt.Errorf("华为云短信发送失败: %s - %s", result.Code, result.Description)
	}

	return nil
}

func huaweiSha256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
