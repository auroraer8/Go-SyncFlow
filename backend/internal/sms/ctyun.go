package sms

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CtyunProvider 天翼云短信实现
type CtyunProvider struct {
	cfg ProviderConfig
}

func NewCtyunProvider(cfg ProviderConfig) *CtyunProvider {
	return &CtyunProvider{cfg: cfg}
}

func (p *CtyunProvider) Name() string { return "天翼云短信" }

func (p *CtyunProvider) SendWithScene(phone, content, scene string) error {
	return p.Send(phone, content)
}

func (p *CtyunProvider) Send(phone, content string) error {
	if p.cfg.AppID == "" || p.cfg.AppSecret == "" {
		return fmt.Errorf("AppID 或 AppSecret 未配置")
	}
	if p.cfg.TemplateCode == "" {
		return fmt.Errorf("模板编码未配置")
	}

	endpoint := p.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://sms-global.ctapi.ctyun.cn/sms/api/v1"
	}

	reqBody := map[string]interface{}{
		"action":       "SendSms",
		"phoneNumber":  phone,
		"signName":     p.cfg.SignName,
		"templateCode": p.cfg.TemplateCode,
		"templateParam": fmt.Sprintf(`{"code":"%s","content":"%s"}`, content, content),
	}
	payload, _ := json.Marshal(reqBody)

	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	mac := hmac.New(sha256.New, []byte(p.cfg.AppSecret))
	mac.Write([]byte(timestamp))
	signature := hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Eop-Authorization", fmt.Sprintf("%s %s %s", p.cfg.AppID, timestamp, signature))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求天翼云短信API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("天翼云短信API返回异常: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.Code != "OK" && result.Code != "" {
		return fmt.Errorf("天翼云短信发送失败: %s - %s", result.Code, result.Message)
	}

	return nil
}
