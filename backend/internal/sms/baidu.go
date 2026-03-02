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
	"strings"
	"time"
)

// BaiduProvider 百度云短信实现
type BaiduProvider struct {
	cfg ProviderConfig
}

func NewBaiduProvider(cfg ProviderConfig) *BaiduProvider {
	return &BaiduProvider{cfg: cfg}
}

func (p *BaiduProvider) Name() string { return "百度云短信" }

func (p *BaiduProvider) SendWithScene(phone, content, scene string) error {
	return p.Send(phone, content)
}

func (p *BaiduProvider) Send(phone, content string) error {
	if p.cfg.AccessKeyID == "" || p.cfg.AccessKeySecret == "" {
		return fmt.Errorf("AccessKeyID 或 AccessKeySecret 未配置")
	}
	if p.cfg.SignName == "" || p.cfg.TemplateCode == "" {
		return fmt.Errorf("签名或模板未配置")
	}
	if p.cfg.InvokeID == "" {
		return fmt.Errorf("InvokeId 未配置")
	}

	endpoint := p.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://smsv3.bj.baidubce.com"
	}

	reqBody := map[string]interface{}{
		"invokeId":     p.cfg.InvokeID,
		"phoneNumber":  phone,
		"signatureId":  p.cfg.SignName,
		"templateCode": p.cfg.TemplateCode,
		"contentVar":   map[string]string{"content": content, "code": content},
	}
	payload, _ := json.Marshal(reqBody)

	apiURL := endpoint + "/api/v3/sendsms"
	now := time.Now().UTC()
	timestamp := now.Format("2006-01-02T15:04:05Z")

	req, _ := http.NewRequest("POST", apiURL, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://"))
	req.Header.Set("x-bce-date", timestamp)

	// BCE 签名
	authStr := p.bceSign(req, timestamp)
	req.Header.Set("Authorization", authStr)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求百度云短信API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("百度云短信API返回异常: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.Code != "" && result.Code != "1000" {
		return fmt.Errorf("百度云短信发送失败: %s - %s", result.Code, result.Message)
	}

	return nil
}

func (p *BaiduProvider) bceSign(req *http.Request, timestamp string) string {
	// 简化 BCE 签名
	signingKey := bceHmacSHA256(p.cfg.AccessKeySecret, "bce-auth-v1/"+p.cfg.AccessKeyID+"/"+timestamp+"/1800")
	canonicalRequest := req.Method + "\n" + req.URL.Path + "\n\nhost:" + req.Host
	signature := hex.EncodeToString(bceHmacSHA256Raw(signingKey, canonicalRequest))
	return fmt.Sprintf("bce-auth-v1/%s/%s/1800/host/%s", p.cfg.AccessKeyID, timestamp, signature)
}

func bceHmacSHA256(key, data string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func bceHmacSHA256Raw(key, data string) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
