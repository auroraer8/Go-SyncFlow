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
	"strconv"
	"strings"
	"time"
)

// TencentProvider 腾讯云短信实现
type TencentProvider struct {
	cfg ProviderConfig
}

func NewTencentProvider(cfg ProviderConfig) *TencentProvider {
	return &TencentProvider{cfg: cfg}
}

func (p *TencentProvider) Name() string { return "腾讯云短信" }

func (p *TencentProvider) Send(phone, content string) error {
	return p.SendWithScene(phone, content, "")
}

func (p *TencentProvider) SendWithScene(phone, content, scene string) error {
	if p.cfg.SecretID == "" || p.cfg.SecretKey == "" {
		return fmt.Errorf("SecretId 或 SecretKey 未配置")
	}
	if p.cfg.SdkAppID == "" {
		return fmt.Errorf("SdkAppId 未配置")
	}
	templateID := p.cfg.GetTemplateIDForScene(scene)
	if templateID == "" {
		return fmt.Errorf("TemplateId 未配置")
	}

	// 腾讯云手机号需要加 +86 前缀
	if !strings.HasPrefix(phone, "+") {
		phone = "+86" + phone
	}

	// 构造请求体
	reqBody := map[string]interface{}{
		"SmsSdkAppId":  p.cfg.SdkAppID,
		"SignName":     p.cfg.SignName,
		"TemplateId":   templateID,
		"PhoneNumberSet": []string{phone},
		"TemplateParamSet": []string{content},
	}
	payload, _ := json.Marshal(reqBody)

	// TC3-HMAC-SHA256 签名
	host := "sms.tencentcloudapi.com"
	service := "sms"
	action := "SendSms"
	version := "2021-01-11"
	timestamp := time.Now().Unix()
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")

	// 步骤1: 拼接规范请求串
	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	canonicalHeaders := "content-type:application/json; charset=utf-8\nhost:" + host + "\nx-tc-action:" + strings.ToLower(action) + "\n"
	signedHeaders := "content-type;host;x-tc-action"

	hashedPayload := sha256hex(payload)
	canonicalRequest := httpRequestMethod + "\n" + canonicalURI + "\n" + canonicalQueryString + "\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + hashedPayload

	// 步骤2: 拼接待签名字符串
	algorithm := "TC3-HMAC-SHA256"
	credentialScope := date + "/" + service + "/tc3_request"
	stringToSign := algorithm + "\n" + strconv.FormatInt(timestamp, 10) + "\n" + credentialScope + "\n" + sha256hex([]byte(canonicalRequest))

	// 步骤3: 计算签名
	secretDate := hmacSHA256([]byte("TC3"+p.cfg.SecretKey), date)
	secretService := hmacSHA256(secretDate, service)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	// 步骤4: 构造 Authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, p.cfg.SecretID, credentialScope, signedHeaders, signature)

	// 发送请求
	req, _ := http.NewRequest("POST", "https://"+host, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Region", "ap-guangzhou")
	req.Header.Set("Authorization", authorization)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求腾讯云短信API失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Response struct {
			SendStatusSet []struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"SendStatusSet"`
			Error struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析腾讯云响应失败: %s", string(body))
	}

	if result.Response.Error.Code != "" {
		return fmt.Errorf("腾讯云短信API错误: %s - %s", result.Response.Error.Code, result.Response.Error.Message)
	}

	if len(result.Response.SendStatusSet) > 0 && result.Response.SendStatusSet[0].Code != "Ok" {
		return fmt.Errorf("腾讯云短信发送失败: %s - %s",
			result.Response.SendStatusSet[0].Code,
			result.Response.SendStatusSet[0].Message)
	}

	return nil
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
