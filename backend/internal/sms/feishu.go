package sms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FeishuProvider 飞书消息实现
type FeishuProvider struct {
	cfg         ProviderConfig
	accessToken string
	tokenExpire time.Time
}

func NewFeishuProvider(cfg ProviderConfig) *FeishuProvider {
	return &FeishuProvider{cfg: cfg}
}

func (p *FeishuProvider) Name() string { return "飞书" }

func (p *FeishuProvider) SendWithScene(phone, content, scene string) error {
	return p.Send(phone, content)
}

func (p *FeishuProvider) Send(phone, content string) error {
	if p.cfg.FeishuAppID == "" || p.cfg.FeishuAppSecret == "" {
		return fmt.Errorf("FeishuAppId 或 FeishuAppSecret 未配置")
	}

	token, err := p.getTenantAccessToken()
	if err != nil {
		return err
	}

	// phone 在飞书中可以是 open_id 或 user_id
	reqBody := map[string]interface{}{
		"receive_id": phone,
		"msg_type":   "text",
		"content":    fmt.Sprintf(`{"text":"%s"}`, content),
	}
	payload, _ := json.Marshal(reqBody)

	// 使用 open_id 接收
	apiURL := "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=open_id"
	req, _ := http.NewRequest("POST", apiURL, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求飞书API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析飞书响应失败: %s", string(body))
	}

	if result.Code != 0 {
		return fmt.Errorf("飞书消息发送失败: %d - %s", result.Code, result.Msg)
	}

	return nil
}

func (p *FeishuProvider) getTenantAccessToken() (string, error) {
	if p.accessToken != "" && time.Now().Before(p.tokenExpire) {
		return p.accessToken, nil
	}

	reqBody := map[string]string{
		"app_id":     p.cfg.FeishuAppID,
		"app_secret": p.cfg.FeishuAppSecret,
	}
	payload, _ := json.Marshal(reqBody)

	resp, err := http.Post("https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		"application/json; charset=utf-8", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("获取飞书Token失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析飞书Token响应失败: %s", string(body))
	}

	if result.Code != 0 {
		return "", fmt.Errorf("获取飞书Token失败: %d - %s", result.Code, result.Msg)
	}

	p.accessToken = result.TenantAccessToken
	p.tokenExpire = time.Now().Add(time.Duration(result.Expire-60) * time.Second)
	return p.accessToken, nil
}
