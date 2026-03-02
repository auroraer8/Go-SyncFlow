package sms

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RongheProvider 融合云信短信实现
type RongheProvider struct {
	cfg ProviderConfig
}

func NewRongheProvider(cfg ProviderConfig) *RongheProvider {
	return &RongheProvider{cfg: cfg}
}

func (p *RongheProvider) Name() string { return "融合云信" }

func (p *RongheProvider) SendWithScene(phone, content, scene string) error {
	return p.Send(phone, content)
}

func (p *RongheProvider) Send(phone, content string) error {
	if p.cfg.Account == "" || p.cfg.Password == "" {
		return fmt.Errorf("Account 或 Password 未配置")
	}

	endpoint := p.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.mix2.zthysms.com/v2/sendSms"
	}

	// 融合云信使用 MD5 签名
	timestamp := time.Now().Format("20060102150405")
	hash := md5.Sum([]byte(p.cfg.Account + p.cfg.Password + timestamp))
	sign := hex.EncodeToString(hash[:])

	smsContent := content
	if p.cfg.SignName != "" {
		smsContent = "【" + p.cfg.SignName + "】" + content
	}

	form := url.Values{}
	form.Set("account", p.cfg.Account)
	form.Set("password", sign)
	form.Set("mobile", phone)
	form.Set("content", smsContent)
	form.Set("time", timestamp)

	resp, err := http.Post(endpoint, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("请求融合云信API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析融合云信响应失败: %s", string(body))
	}

	if result.Code != 200 {
		return fmt.Errorf("融合云信发送失败: %d - %s", result.Code, result.Msg)
	}

	return nil
}
