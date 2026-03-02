package sms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DingTalkSMSProvider 钉钉工作消息实现
type DingTalkSMSProvider struct {
	cfg         ProviderConfig
	accessToken string
	tokenExpire time.Time
}

func NewDingTalkProvider(cfg ProviderConfig) *DingTalkSMSProvider {
	return &DingTalkSMSProvider{cfg: cfg}
}

func (p *DingTalkSMSProvider) Name() string { return "钉钉" }

func (p *DingTalkSMSProvider) SendWithScene(phone, content, scene string) error {
	return p.Send(phone, content)
}

func (p *DingTalkSMSProvider) Send(phone, content string) error {
	if p.cfg.DingAppKey == "" || p.cfg.DingAppSecret == "" {
		return fmt.Errorf("DingAppKey 或 DingAppSecret 未配置")
	}

	token, err := p.getAccessToken()
	if err != nil {
		return err
	}

	// phone 在钉钉中作为 userid
	reqBody := map[string]interface{}{
		"agent_id":    p.cfg.DingAgentID,
		"userid_list": phone,
		"msg": map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": content,
			},
		},
	}
	payload, _ := json.Marshal(reqBody)

	apiURL := fmt.Sprintf("https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2?access_token=%s", token)
	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("请求钉钉API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析钉钉响应失败: %s", string(body))
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("钉钉消息发送失败: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

func (p *DingTalkSMSProvider) getAccessToken() (string, error) {
	if p.accessToken != "" && time.Now().Before(p.tokenExpire) {
		return p.accessToken, nil
	}

	apiURL := fmt.Sprintf("https://oapi.dingtalk.com/gettoken?appkey=%s&appsecret=%s",
		p.cfg.DingAppKey, p.cfg.DingAppSecret)
	resp, err := http.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("获取钉钉Token失败: %v", err)
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
		return "", fmt.Errorf("解析钉钉Token响应失败: %s", string(body))
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("获取钉钉Token失败: %d - %s", result.ErrCode, result.ErrMsg)
	}

	p.accessToken = result.AccessToken
	p.tokenExpire = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return p.accessToken, nil
}
