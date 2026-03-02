package sms

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// YunpianProvider 云片短信实现
// 文档: https://www.yunpian.com/official/document/sms/zh_cn/domestic_single_send
type YunpianProvider struct {
	cfg ProviderConfig
}

func NewYunpianProvider(cfg ProviderConfig) *YunpianProvider {
	return &YunpianProvider{cfg: cfg}
}

func (p *YunpianProvider) Name() string { return "云片短信" }

func (p *YunpianProvider) Send(phone, content string) error {
	return p.SendWithScene(phone, content, "")
}

func (p *YunpianProvider) SendWithScene(phone, content, scene string) error {
	if p.cfg.APIKey == "" {
		return fmt.Errorf("云片 API Key 未配置")
	}
	if p.cfg.SignName == "" {
		return fmt.Errorf("云片签名未配置")
	}

	// 云片短信需要在内容前携带签名，如：【云片网】您的验证码是1234
	text := content
	if !strings.HasPrefix(content, "【") {
		text = fmt.Sprintf("【%s】%s", p.cfg.SignName, content)
	}

	// 构造请求参数
	data := url.Values{}
	data.Set("apikey", p.cfg.APIKey)
	data.Set("mobile", phone)
	data.Set("text", text)

	// 发送请求
	req, err := http.NewRequest("POST", "https://sms.yunpian.com/v2/sms/single_send.json", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	req.Header.Set("Accept", "application/json;charset=utf-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求云片短信API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Code   int    `json:"code"`
		Msg    string `json:"msg"`
		Count  int    `json:"count"`
		Fee    float64 `json:"fee"`
		Mobile string `json:"mobile"`
		Sid    int64  `json:"sid"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析云片响应失败: %s", string(body))
	}

	// code 为 0 表示发送成功
	if result.Code != 0 {
		return fmt.Errorf("云片短信发送失败: [%d] %s", result.Code, result.Msg)
	}

	return nil
}
