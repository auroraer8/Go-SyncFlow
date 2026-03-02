package sms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WecomProvider 企业微信消息实现
type WecomProvider struct {
	cfg         ProviderConfig
	accessToken string
	tokenExpire time.Time
}

func NewWecomProvider(cfg ProviderConfig) *WecomProvider {
	return &WecomProvider{cfg: cfg}
}

func (p *WecomProvider) Name() string { return "企业微信" }

func (p *WecomProvider) SendWithScene(phone, content, scene string) error {
	return p.Send(phone, content)
}

func (p *WecomProvider) Send(phone, content string) error {
	if p.cfg.CorpID == "" || p.cfg.CorpSecret == "" {
		return fmt.Errorf("CorpID 或 CorpSecret 未配置")
	}
	if p.cfg.AgentID == "" {
		return fmt.Errorf("AgentID 未配置")
	}

	token, err := p.getAccessToken()
	if err != nil {
		return err
	}

	// 发送应用消息（文本消息）
	// 企业微信不支持直接通过手机号发送，需要用 UserID
	// 这里 phone 可以作为 UserID 使用
	reqBody := map[string]interface{}{
		"touser":  phone,
		"msgtype": "text",
		"agentid": p.cfg.AgentID,
		"text": map[string]string{
			"content": content,
		},
	}
	payload, _ := json.Marshal(reqBody)

	apiURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)
	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("请求企业微信API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析企业微信响应失败: %s", string(body))
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("企业微信消息发送失败: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

func (p *WecomProvider) getAccessToken() (string, error) {
	if p.accessToken != "" && time.Now().Before(p.tokenExpire) {
		return p.accessToken, nil
	}

	apiURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		p.cfg.CorpID, p.cfg.CorpSecret)
	resp, err := http.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("获取企业微信Token失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析企业微信Token响应失败: %s", string(body))
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("获取企业微信Token失败: %d - %s", result.ErrCode, result.ErrMsg)
	}

	p.accessToken = result.AccessToken
	p.tokenExpire = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return p.accessToken, nil
}
