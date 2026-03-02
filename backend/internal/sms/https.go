package sms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPSProvider HTTPS自定义短信通道实现
type HTTPSProvider struct {
	cfg ProviderConfig
}

func NewHTTPSProvider(cfg ProviderConfig) *HTTPSProvider {
	return &HTTPSProvider{cfg: cfg}
}

func (p *HTTPSProvider) Name() string { return "HTTPS自定义" }

func (p *HTTPSProvider) SendWithScene(phone, content, scene string) error {
	return p.Send(phone, content)
}

func (p *HTTPSProvider) Send(phone, content string) error {
	if p.cfg.URL == "" {
		return fmt.Errorf("请求URL未配置")
	}

	// JSON 转义
	jsonEscape := func(s string) string {
		b, _ := json.Marshal(s)
		return string(b[1 : len(b)-1])
	}

	body := p.cfg.BodyTemplate
	if body == "" {
		body = fmt.Sprintf(`{"sign":"%s","mobile":"%s","content":"%s"}`,
			jsonEscape(p.cfg.Sign), jsonEscape(phone), jsonEscape(content))
	} else {
		body = strings.ReplaceAll(body, "{{phone}}", jsonEscape(phone))
		body = strings.ReplaceAll(body, "{{mobile}}", jsonEscape(phone))
		body = strings.ReplaceAll(body, "{{message}}", jsonEscape(content))
		body = strings.ReplaceAll(body, "{{content}}", jsonEscape(content))
		body = strings.ReplaceAll(body, "{{time}}", jsonEscape(time.Now().Format("2006-01-02 15:04:05")))
		body = strings.ReplaceAll(body, "{{sign}}", jsonEscape(p.cfg.Sign))
	}

	method := p.cfg.Method
	if method == "" {
		method = "POST"
	}
	contentType := p.cfg.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(method, p.cfg.URL, bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", contentType)

	// 自定义 headers
	if p.cfg.Headers != nil {
		switch h := p.cfg.Headers.(type) {
		case string:
			if h != "" {
				var headers map[string]string
				if err := json.Unmarshal([]byte(h), &headers); err == nil {
					for k, v := range headers {
						req.Header.Set(k, v)
					}
				}
			}
		case map[string]interface{}:
			for k, v := range h {
				if vs, ok := v.(string); ok {
					req.Header.Set(k, vs)
				}
			}
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("[短信-HTTPS] 请求URL: %s\n", p.cfg.URL)
	fmt.Printf("[短信-HTTPS] 请求体: %s\n", body)
	fmt.Printf("[短信-HTTPS] 响应: status=%d, body=%s\n", resp.StatusCode, string(respBody))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API返回异常: %d - %s", resp.StatusCode, string(respBody))
	}

	// 检查响应 body 中的 success 字段（部分网关 HTTP 200 但 body 返回失败）
	var result map[string]interface{}
	if json.Unmarshal(respBody, &result) == nil {
		if success, ok := result["success"]; ok {
			if s, isBool := success.(bool); isBool && !s {
				errMsg := ""
				if em, ok := result["errorMsg"]; ok && em != nil {
					errMsg = fmt.Sprintf("%v", em)
				}
				return fmt.Errorf("网关返回失败: %s", errMsg)
			}
		}
	}

	return nil
}
