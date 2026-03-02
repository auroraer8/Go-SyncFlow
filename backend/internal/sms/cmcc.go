package sms

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CmccProvider 移动云MAS短信实现
type CmccProvider struct {
	cfg ProviderConfig
}

func NewCmccProvider(cfg ProviderConfig) *CmccProvider {
	return &CmccProvider{cfg: cfg}
}

func (p *CmccProvider) Name() string { return "移动云MAS" }

func (p *CmccProvider) SendWithScene(phone, content, scene string) error {
	return p.Send(phone, content)
}

func (p *CmccProvider) Send(phone, content string) error {
	if p.cfg.ECID == "" || p.cfg.APIKey == "" || p.cfg.SecretKeyM == "" {
		return fmt.Errorf("ECID/APIKey/SecretKey 未配置")
	}

	endpoint := p.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://mas.10086.cn/api/v1/sms/send"
	}

	smsContent := content
	if p.cfg.SignName != "" {
		smsContent = "【" + p.cfg.SignName + "】" + content
	}

	reqBody := map[string]interface{}{
		"ecId":      p.cfg.ECID,
		"mobiles":   phone,
		"content":   smsContent,
		"sign":      p.cfg.SignName,
		"addSerial": "",
	}
	if p.cfg.TemplateID != "" {
		reqBody["templateId"] = p.cfg.TemplateID
		reqBody["params"] = []string{content}
	}
	payload, _ := json.Marshal(reqBody)

	timestamp := time.Now().Format("20060102150405")
	mac := hmac.New(sha256.New, []byte(p.cfg.SecretKeyM))
	mac.Write([]byte(p.cfg.ECID + p.cfg.APIKey + timestamp))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-EC-ID", p.cfg.ECID)
	req.Header.Set("X-API-KEY", p.cfg.APIKey)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Sign", signature)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求移动云MAS API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("移动云MAS API返回异常: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success bool   `json:"success"`
		Rspcod  string `json:"rspcod"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err == nil && !result.Success {
		return fmt.Errorf("移动云MAS发送失败: %s - %s", result.Rspcod, result.Msg)
	}

	return nil
}
