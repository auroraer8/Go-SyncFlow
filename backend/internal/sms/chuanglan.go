package sms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ChuanglanProvider 创蓝短信实现
// 文档: https://doc.chuanglan.com/
// API地址: https://smssh1.253.com/msg/send/json (或 /msg/v1/send/json)
type ChuanglanProvider struct {
	cfg ProviderConfig
}

func NewChuanglanProvider(cfg ProviderConfig) *ChuanglanProvider {
	return &ChuanglanProvider{cfg: cfg}
}

func (p *ChuanglanProvider) Name() string { return "创蓝短信" }

func (p *ChuanglanProvider) Send(phone, content string) error {
	return p.SendWithScene(phone, content, "")
}

func (p *ChuanglanProvider) SendWithScene(phone, content, scene string) error {
	if p.cfg.Account == "" {
		return fmt.Errorf("创蓝 API 账号未配置")
	}
	if p.cfg.Password == "" {
		return fmt.Errorf("创蓝 API 密码未配置")
	}

	// 短信内容需要携带签名，如：【签名】内容
	msg := content
	if p.cfg.SignName != "" && !strings.HasPrefix(content, "【") {
		msg = fmt.Sprintf("【%s】%s", p.cfg.SignName, content)
	}

	// 构造请求参数
	reqData := map[string]interface{}{
		"account":  p.cfg.Account,
		"password": p.cfg.Password,
		"phone":    phone,
		"msg":      msg,
		"report":   "true",
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return fmt.Errorf("序列化请求参数失败: %v", err)
	}

	// 默认使用 smssh1.253.com，可通过 Endpoint 配置不同区域
	apiURL := "https://smssh1.253.com/msg/send/json"
	if p.cfg.Endpoint != "" {
		apiURL = p.cfg.Endpoint
	}

	// 发送请求
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求创蓝短信API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Code     string `json:"code"`
		MsgId    string `json:"msgId"`
		ErrorMsg string `json:"errorMsg"`
		Time     string `json:"time"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析创蓝响应失败: %s", string(body))
	}

	// code 为 "0" 表示发送成功
	if result.Code != "0" {
		return fmt.Errorf("创蓝短信发送失败: [%s] %s", result.Code, result.ErrorMsg)
	}

	return nil
}
