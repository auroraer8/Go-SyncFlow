package sms

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"go-syncflow/internal/vpn/dbdata"
)

// CustomProvider 自定义HTTP API短信发送器
type CustomProvider struct {
	config *dbdata.SettingSms
}

// NewCustomProvider 创建自定义短信发送器
func NewCustomProvider(config *dbdata.SettingSms) *CustomProvider {
	return &CustomProvider{config: config}
}

// Send 发送短信
func (p *CustomProvider) Send(phone, code string) error {
	if p.config.CustomUrl == "" {
		return errors.New("自定义API URL未配置")
	}
	url := replaceVariables(p.config.CustomUrl, phone, code)
	body := replaceVariables(p.config.CustomBody, phone, code)
	req, err := http.NewRequest(p.config.CustomMethod, url, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	if p.config.CustomHeaders != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(p.config.CustomHeaders), &headers); err == nil {
			for k, v := range headers {
				req.Header.Set(k, replaceVariables(v, phone, code))
			}
		}
	}
	if req.Header.Get("Content-Type") == "" && p.config.CustomMethod == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return errors.New("短信发送失败: " + string(respBody))
	}
	return nil
}

// replaceVariables 替换模板变量
func replaceVariables(template, phone, code string) string {
	result := strings.ReplaceAll(template, "{phone}", phone)
	result = strings.ReplaceAll(result, "{code}", code)
	return result
}
