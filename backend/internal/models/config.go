package models

import "time"

type SystemConfig struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Key         string    `gorm:"size:128;uniqueIndex;not null" json:"key"`
	Value       string    `gorm:"type:text" json:"value"`
	Description string    `gorm:"size:255" json:"description"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// 界面配置结构
type UIConfig struct {
	BrowserTitle    string `json:"browserTitle"`
	LoginTitle      string `json:"loginTitle"`
	Logo            string `json:"logo"`
	FooterShortName string `json:"footerShortName"`
	FooterCompany   string `json:"footerCompany"`
	FooterICP       string `json:"footerICP"`
	FooterText      string `json:"footerText"`
	// 登录页配置
	LoginBrandName  string `json:"loginBrandName"`  // 登录页左上角品牌名称
	LoginSubtitle   string `json:"loginSubtitle"`   // 登录页副标题（欢迎使用xxx）
	// 主题配置
	AdminTheme string `json:"adminTheme"` // 管理后台主题: default/blue/gray/dark
	LoginTheme string `json:"loginTheme"` // 登录页主题: light/dark/blue
}

// 钉钉配置结构
type DingTalkConfig struct {
	Enabled       bool   `json:"enabled"`       // 是否启用钉钉
	AppKey        string `json:"appKey"`         // 企业应用AppKey
	AppSecret     string `json:"appSecret"`      // 企业应用AppSecret
	AgentID       string `json:"agentId"`        // 应用AgentId
	CorpID        string `json:"corpId"`         // 企业CorpId
	MatchField    string `json:"matchField"`     // 匹配字段: mobile/email/userid
	AutoRegister  bool   `json:"autoRegister"`   // 是否自动注册用户
	DefaultRoleID uint   `json:"defaultRoleId"`  // 自动注册时的默认角色ID
	// 同步相关
	UsernameField   string `json:"usernameField"`   // 用户名生成策略: email_prefix/email/dingtalk_userid/mobile/pinyin
	SyncInterval    int    `json:"syncInterval"`    // 同步间隔（分钟），0为不自动同步
	LastSyncAt      string `json:"lastSyncAt"`      // 最后同步时间
	LastSyncStatus  string `json:"lastSyncStatus"`  // 最后同步状态: success/failed
	LastSyncMessage string `json:"lastSyncMessage"` // 最后同步信息
}

// LDAP配置结构
type LDAPConfig struct {
	Enabled          bool   `json:"enabled"`          // 是否启用 LDAP 服务
	Port             int    `json:"port"`             // LDAP 端口，默认 389
	UseTLS           bool   `json:"useTLS"`           // 是否启用 LDAPS
	TLSPort          int    `json:"tlsPort"`          // LDAPS 端口，默认 636
	TLSCertFile      string `json:"tlsCertFile"`      // TLS 证书路径（空则复用 HTTPS 证书）
	TLSKeyFile       string `json:"tlsKeyFile"`       // TLS 私钥路径
	Domain           string `json:"domain"`           // 域名，如 example.com
	BaseDN           string `json:"baseDN"`           // Base DN，如 dc=example,dc=com
	ManagerDN        string `json:"managerDN"`        // 管理员 Bind DN (cn=Manager,baseDN)
	ManagerPassword  string `json:"managerPassword"`  // 管理员 Bind 密码
	ReadonlyDN       string `json:"readonlyDN"`       // 只读账号 Bind DN (cn=readonly,baseDN)
	ReadonlyPassword string `json:"readonlyPassword"` // 只读账号 Bind 密码
	SambaEnabled     bool   `json:"sambaEnabled"`     // 是否启用 Samba 属性
	SambaSID         string `json:"sambaSID"`         // Samba 域 SID
	// 兼容旧配置字段（已弃用，保留用于自动迁移）
	AdminDN       string `json:"adminDN,omitempty"`       // 已弃用
	AdminPassword string `json:"adminPassword,omitempty"` // 已弃用
}

// HTTPS配置结构
type HTTPSConfig struct {
	Enabled     bool   `json:"enabled"`
	Port        string `json:"port"`
	CertFile    string `json:"certFile"`
	KeyFile     string `json:"keyFile"`
	Domain      string `json:"domain"`
	CertExpiry  string `json:"certExpiry"`
	CertSubject string `json:"certSubject"`
}
