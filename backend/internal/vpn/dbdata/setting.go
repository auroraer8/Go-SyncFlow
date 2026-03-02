package dbdata

import (
	"encoding/json"
	"reflect"

	"xorm.io/xorm"
)

type SettingInstall struct {
	Installed bool `json:"installed"`
}

type SettingSmtp struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	From       string `json:"from"`
	Encryption string `json:"encryption"`
}

type SettingAuditLog struct {
	AuditInterval int    `json:"audit_interval"`
	LifeDay       int    `json:"life_day"`
	ClearTime     string `json:"clear_time"`
}

type SettingOther struct {
	LinkAddr    string `json:"link_addr"`
	Banner      string `json:"banner"`
	Homecode    int    `json:"homecode"`
	Homeindex   string `json:"homeindex"`
	AccountMail string `json:"account_mail"`
}

// 短信渠道配置
type SettingSms struct {
	Enabled  bool   `json:"enabled"`  // 全局开关
	Provider string `json:"provider"` // aliyun/tencent/huawei/baidu/ctyun/cmcc/wecom/dingtalk/feishu/https

	// ===== 阿里云短信 =====
	AliyunAccessKeyId     string `json:"aliyun_access_key_id"`
	AliyunAccessKeySecret string `json:"aliyun_access_key_secret"`
	AliyunSignName        string `json:"aliyun_sign_name"`
	AliyunTemplateCode    string `json:"aliyun_template_code"`
	AliyunRegionId        string `json:"aliyun_region_id"` // 默认 cn-hangzhou

	// ===== 腾讯云短信 =====
	TencentSecretId   string `json:"tencent_secret_id"`
	TencentSecretKey  string `json:"tencent_secret_key"`
	TencentAppId      string `json:"tencent_app_id"`
	TencentSignName   string `json:"tencent_sign_name"`
	TencentTemplateId string `json:"tencent_template_id"`

	// ===== 华为云短信 =====
	HuaweiAppKey     string `json:"huawei_app_key"`
	HuaweiAppSecret  string `json:"huawei_app_secret"`
	HuaweiChannel    string `json:"huawei_channel"`     // 签名通道号
	HuaweiTemplateId string `json:"huawei_template_id"` // 模板ID
	HuaweiSignName   string `json:"huawei_sign_name"`   // 签名名称
	HuaweiEndpoint   string `json:"huawei_endpoint"`    // API端点

	// ===== 百度云短信 =====
	BaiduAccessKeyId     string `json:"baidu_access_key_id"`
	BaiduAccessKeySecret string `json:"baidu_access_key_secret"`
	BaiduInvokeId        string `json:"baidu_invoke_id"`     // 业务ID
	BaiduSignName        string `json:"baidu_sign_name"`     // 签名ID
	BaiduTemplateCode    string `json:"baidu_template_code"` // 模板编码
	BaiduEndpoint        string `json:"baidu_endpoint"`      // API端点

	// ===== 天翼云短信 =====
	CtyunAppId        string `json:"ctyun_app_id"`
	CtyunAppSecret    string `json:"ctyun_app_secret"`
	CtyunSignName     string `json:"ctyun_sign_name"`
	CtyunTemplateCode string `json:"ctyun_template_code"`
	CtyunEndpoint     string `json:"ctyun_endpoint"`

	// ===== 移动云MAS =====
	CmccEcId       string `json:"cmcc_ec_id"`       // 企业代码
	CmccApiKey     string `json:"cmcc_api_key"`     // API Key
	CmccSecretKey  string `json:"cmcc_secret_key"`  // Secret Key
	CmccSignName   string `json:"cmcc_sign_name"`   // 签名名称
	CmccTemplateId string `json:"cmcc_template_id"` // 模板ID
	CmccEndpoint   string `json:"cmcc_endpoint"`    // API端点

	// ===== 企业微信 =====
	WecomCorpId     string `json:"wecom_corp_id"`     // 企业ID
	WecomCorpSecret string `json:"wecom_corp_secret"` // 应用Secret
	WecomAgentId    string `json:"wecom_agent_id"`    // 应用AgentID

	// ===== 钉钉 =====
	DingtalkAppKey    string `json:"dingtalk_app_key"`
	DingtalkAppSecret string `json:"dingtalk_app_secret"`
	DingtalkAgentId   string `json:"dingtalk_agent_id"`

	// ===== 飞书 =====
	FeishuAppId     string `json:"feishu_app_id"`
	FeishuAppSecret string `json:"feishu_app_secret"`

	// ===== 自定义HTTPS API =====
	CustomUrl         string `json:"custom_url"`          // API URL
	CustomMethod      string `json:"custom_method"`       // GET/POST
	CustomContentType string `json:"custom_content_type"` // Content-Type
	CustomHeaders     string `json:"custom_headers"`      // JSON格式的请求头
	CustomBody        string `json:"custom_body"`         // 请求体模板，支持 {{phone}} {{code}} {{message}} 变量

	// ===== 验证码设置 =====
	CodeLength int `json:"code_length"` // 验证码长度，默认6
	CodeExpire int `json:"code_expire"` // 验证码过期时间(秒)，默认300
}

func StructName(data interface{}) string {
	ref := reflect.ValueOf(data)
	s := &ref
	if s.Kind() == reflect.Ptr {
		e := s.Elem()
		s = &e
	}
	name := s.Type().Name()
	return name
}

func SettingSessAdd(sess *xorm.Session, data interface{}) error {
	name := StructName(data)
	v, _ := json.Marshal(data)
	s := &Setting{Name: name, Data: v}
	_, err := sess.InsertOne(s)
	return err
}

func SettingSet(data interface{}) error {
	name := StructName(data)
	v, _ := json.Marshal(data)
	s := &Setting{Data: v}
	err := Update("name", name, s)
	return err
}

func SettingGet(data interface{}) error {
	name := StructName(data)
	s := &Setting{}
	err := One("name", name, s)
	if err != nil {
		return err
	}
	err = json.Unmarshal(s.Data, data)
	return err
}

func SettingGetAuditLog() (SettingAuditLog, error) {
	data := SettingAuditLog{}
	err := SettingGet(&data)
	if err == nil {
		return data, err
	}
	if !CheckErrNotFound(err) {
		return data, err
	}
	sess := xdb.NewSession()
	defer sess.Close()
	auditLog := SettingGetAuditLogDefault()
	err = SettingSessAdd(sess, auditLog)
	if err != nil {
		return data, err
	}
	return auditLog, nil
}

func SettingGetAuditLogDefault() SettingAuditLog {
	auditLog := SettingAuditLog{
		LifeDay:   0,
		ClearTime: "05:00",
	}
	return auditLog
}

func SettingGetSms() (SettingSms, error) {
	data := SettingSms{}
	err := SettingGet(&data)
	if err == nil {
		return data, err
	}
	if !CheckErrNotFound(err) {
		return data, err
	}
	sess := xdb.NewSession()
	defer sess.Close()
	sms := SettingGetSmsDefault()
	err = SettingSessAdd(sess, sms)
	if err != nil {
		return data, err
	}
	return sms, nil
}

func SettingGetSmsDefault() SettingSms {
	sms := SettingSms{
		Enabled:      false,
		Provider:     "custom",
		CodeLength:   6,
		CodeExpire:   300,
		CustomMethod: "POST",
	}
	return sms
}

// SettingAutoStart 自动启动配置
type SettingAutoStart struct {
	Enabled bool `json:"enabled"`
}

// SettingGetAutoStart 获取自动启动配置
func SettingGetAutoStart() (SettingAutoStart, error) {
	data := SettingAutoStart{}
	err := SettingGet(&data)
	if err == nil {
		return data, nil
	}
	if !CheckErrNotFound(err) {
		return data, err
	}
	// 默认启用自动启动
	return SettingAutoStart{Enabled: true}, nil
}

// SettingSetAutoStart 设置自动启动配置
func SettingSetAutoStart(enabled bool) error {
	data := SettingAutoStart{Enabled: enabled}
	return SettingSet(&data)
}
