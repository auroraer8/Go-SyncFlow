package sms

import (
	"fmt"
)

// Provider 短信服务商统一接口
type Provider interface {
	// Send 发送短信
	// phone: 手机号, content: 短信内容（已渲染模板）
	Send(phone, content string) error
	// SendWithScene 按场景发送短信（可使用场景对应的模板ID）
	SendWithScene(phone, content, scene string) error
	// Name 提供商名称（用于日志）
	Name() string
}

// ProviderConfig 各厂商通用配置字段
type ProviderConfig struct {
	// 阿里云
	AccessKeyID     string `json:"accessKeyId,omitempty"`
	AccessKeySecret string `json:"accessKeySecret,omitempty"`
	SignName        string `json:"signName,omitempty"`
	TemplateCode    string `json:"templateCode,omitempty"`
	RegionID        string `json:"regionId,omitempty"`

	// 腾讯云
	SecretID    string `json:"secretId,omitempty"`
	SecretKey   string `json:"secretKey,omitempty"`
	SdkAppID    string `json:"sdkAppId,omitempty"`
	TemplateID  string `json:"templateId,omitempty"`

	// 华为云
	AppKey      string `json:"appKey,omitempty"`
	AppSecret   string `json:"appSecret,omitempty"`
	Channel     string `json:"channel,omitempty"` // 通道号
	Sender      string `json:"sender,omitempty"`  // 短信签名通道号

	// 百度云
	InvokeID    string `json:"invokeId,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`

	// 天翼云
	AppID       string `json:"appId,omitempty"`
	Token       string `json:"token,omitempty"`

	// 融合云信
	Account     string `json:"account,omitempty"`
	Password    string `json:"password,omitempty"`

	// 移动云MAS
	ECID        string `json:"ecId,omitempty"` // 企业代码
	APIKey      string `json:"apiKey,omitempty"`
	SecretKeyM  string `json:"secretKeyM,omitempty"` // 区别于腾讯的 secretKey

	// 企业微信
	CorpID      string `json:"corpId,omitempty"`
	CorpSecret  string `json:"corpSecret,omitempty"`
	AgentID     string `json:"agentId,omitempty"`

	// 钉钉
	DingAppKey    string `json:"dingAppKey,omitempty"`
	DingAppSecret string `json:"dingAppSecret,omitempty"`
	DingAgentID   string `json:"dingAgentId,omitempty"`

	// 飞书
	FeishuAppID     string `json:"feishuAppId,omitempty"`
	FeishuAppSecret string `json:"feishuAppSecret,omitempty"`

	// 中国移动5G行业消息（CSP）
	ChatbotURI     string `json:"chatbotUri,omitempty"`     // Chatbot SIP URI
	Cmcc5GPassword string `json:"cmcc5gPassword,omitempty"` // 开发者密码

	// 云片短信
	YunpianAPIKey string `json:"yunpianApiKey,omitempty"` // 云片 API Key

	// 创蓝短信
	ChuanglanAccount  string `json:"chuanglanAccount,omitempty"`  // 创蓝 API 账号
	ChuanglanPassword string `json:"chuanglanPassword,omitempty"` // 创蓝 API 密码

	// HTTPS 自定义
	URL          string `json:"url,omitempty"`
	Method       string `json:"method,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	BodyTemplate string `json:"bodyTemplate,omitempty"`
	Sign         string `json:"sign,omitempty"`
	Headers      interface{} `json:"headers,omitempty"` // JSON 格式的自定义 header（可以是字符串或数组）

	// 场景模板映射（不同场景使用不同模板ID）
	TemplateCodeMap map[string]string `json:"templateCodeMap,omitempty"` // scene -> templateCode
	TemplateIDMap   map[string]string `json:"templateIdMap,omitempty"`   // scene -> templateId

	// 通用
	Phones       string `json:"phones,omitempty"` // 测试手机号（配置界面用）
}

// GetTemplateCodeForScene 根据场景获取模板编码，未配置时返回默认
func (c *ProviderConfig) GetTemplateCodeForScene(scene string) string {
	if c.TemplateCodeMap != nil && scene != "" {
		if code, ok := c.TemplateCodeMap[scene]; ok && code != "" {
			return code
		}
	}
	return c.TemplateCode
}

// GetTemplateIDForScene 根据场景获取模板ID，未配置时返回默认
func (c *ProviderConfig) GetTemplateIDForScene(scene string) string {
	if c.TemplateIDMap != nil && scene != "" {
		if id, ok := c.TemplateIDMap[scene]; ok && id != "" {
			return id
		}
	}
	return c.TemplateID
}

// NewProvider 根据通道类型创建对应的 SMS Provider
func NewProvider(channelType string, cfg ProviderConfig) (Provider, error) {
	switch channelType {
	case "sms_aliyun":
		return NewAliyunProvider(cfg), nil
	case "sms_tencent":
		return NewTencentProvider(cfg), nil
	case "sms_huawei":
		return NewHuaweiProvider(cfg), nil
	case "sms_baidu":
		return NewBaiduProvider(cfg), nil
	case "sms_ctyun":
		return NewCtyunProvider(cfg), nil
	case "sms_ronghe":
		return NewRongheProvider(cfg), nil
	case "sms_cmcc":
		return NewCmccProvider(cfg), nil
	case "sms_wecom":
		return NewWecomProvider(cfg), nil
	case "sms_dingtalk":
		return NewDingTalkProvider(cfg), nil
	case "sms_feishu":
		return NewFeishuProvider(cfg), nil
	case "sms_cmcc_5g":
		return NewCmcc5GProvider(cfg), nil
	case "sms_yunpian":
		return NewYunpianProvider(cfg), nil
	case "sms_chuanglan":
		return NewChuanglanProvider(cfg), nil
	case "sms_https", "sms_custom":
		return NewHTTPSProvider(cfg), nil
	default:
		return nil, fmt.Errorf("不支持的短信通道类型: %s", channelType)
	}
}

// IsSMSChannel 判断通道类型是否为短信类
func IsSMSChannel(channelType string) bool {
	switch channelType {
	case "sms_aliyun", "sms_tencent", "sms_huawei", "sms_baidu",
		"sms_ctyun", "sms_ronghe", "sms_cmcc", "sms_cmcc_5g",
		"sms_wecom", "sms_dingtalk", "sms_feishu",
		"sms_yunpian", "sms_chuanglan",
		"sms_https", "sms_custom":
		return true
	default:
		return false
	}
}
