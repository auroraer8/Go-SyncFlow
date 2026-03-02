package sms

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AliyunProvider 阿里云短信实现
type AliyunProvider struct {
	cfg ProviderConfig
}

func NewAliyunProvider(cfg ProviderConfig) *AliyunProvider {
	if cfg.RegionID == "" {
		cfg.RegionID = "cn-hangzhou"
	}
	return &AliyunProvider{cfg: cfg}
}

func (p *AliyunProvider) Name() string { return "阿里云短信" }

func (p *AliyunProvider) Send(phone, content string) error {
	return p.SendWithScene(phone, content, "")
}

func (p *AliyunProvider) SendWithScene(phone, content, scene string) error {
	if p.cfg.AccessKeyID == "" || p.cfg.AccessKeySecret == "" {
		return fmt.Errorf("AccessKeyID 或 AccessKeySecret 未配置")
	}
	if p.cfg.SignName == "" {
		return fmt.Errorf("短信签名未配置")
	}
	templateCode := p.cfg.GetTemplateCodeForScene(scene)
	if templateCode == "" {
		return fmt.Errorf("短信模板编码未配置")
	}

	// 构造模板参数 — 只传 code 变量，兼容验证码类模板的严格校验
	templateParam := fmt.Sprintf(`{"code":"%s"}`, content)

	params := map[string]string{
		"AccessKeyId":      p.cfg.AccessKeyID,
		"Action":           "SendSms",
		"Format":           "JSON",
		"PhoneNumbers":     phone,
		"RegionId":         p.cfg.RegionID,
		"SignName":         p.cfg.SignName,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   fmt.Sprintf("%d", time.Now().UnixNano()),
		"SignatureVersion": "1.0",
		"TemplateCode":     templateCode,
		"TemplateParam":    templateParam,
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          "2017-05-25",
	}

	// 计算签名
	signature := p.sign(params)
	params["Signature"] = signature

	// 构造请求
	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}

	apiURL := "https://dysmsapi.aliyuncs.com/?" + query.Encode()
	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("请求阿里云短信API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析阿里云响应失败: %s", string(body))
	}

	if result.Code != "OK" {
		return fmt.Errorf("阿里云短信发送失败: %s - %s", result.Code, result.Message)
	}

	return nil
}

// sign 计算阿里云 API 签名 (HMAC-SHA1)
func (p *AliyunProvider) sign(params map[string]string) string {
	// 按参数名排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 构造待签名字符串
	var canonicalQuery strings.Builder
	for i, k := range keys {
		if i > 0 {
			canonicalQuery.WriteString("&")
		}
		canonicalQuery.WriteString(specialURLEncode(k))
		canonicalQuery.WriteString("=")
		canonicalQuery.WriteString(specialURLEncode(params[k]))
	}

	stringToSign := "GET&" + specialURLEncode("/") + "&" + specialURLEncode(canonicalQuery.String())

	// HMAC-SHA1
	mac := hmac.New(sha1.New, []byte(p.cfg.AccessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// specialURLEncode 阿里云特殊 URL 编码
func specialURLEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}
