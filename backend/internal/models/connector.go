package models

import "time"

// Connector 连接器（统一上游+下游）
type Connector struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"size:128;not null" json:"name"`
	Type      string `gorm:"size:32;not null" json:"type"`      // im_dingtalk / im_wechatwork / im_feishu / im_welink / ldap_ad / db_mysql / db_postgresql / db_oracle / db_sqlserver
	Direction string `gorm:"size:16;not null;default:downstream" json:"direction"` // upstream / downstream / both

	// === 通用 ===
	Status  int8   `gorm:"default:1" json:"status"`
	Timeout int    `gorm:"default:5" json:"timeout"`
	Config  string `gorm:"type:text" json:"config"` // 扩展 JSON 配置

	// === LDAP/AD 字段 ===
	Host         string `gorm:"size:255" json:"host"`
	Port         int    `gorm:"default:636" json:"port"`
	BackupHost   string `gorm:"size:255" json:"backupHost"`
	BackupPort   int    `gorm:"default:389" json:"backupPort"`
	UseTLS       bool   `gorm:"default:false" json:"useTls"`
	BaseDN       string `gorm:"size:255" json:"baseDn"`
	BindDN       string `gorm:"size:255" json:"bindDn"`
	BindPassword string `gorm:"size:255" json:"-"`
	UPNSuffix    string `gorm:"size:128" json:"upnSuffix"`
	UserFilter   string `gorm:"size:255" json:"userFilter"`

	// === OpenLDAP 属性映射 ===
	LDAPUserObjectClass  string `gorm:"size:128" json:"ldapUserObjectClass"`  // 用户对象类，如 inetOrgPerson
	LDAPGroupObjectClass string `gorm:"size:128" json:"ldapGroupObjectClass"` // 组对象类，如 groupOfNames
	LDAPLoginAttr        string `gorm:"size:64" json:"ldapLoginAttr"`         // 登录名属性，如 uid
	LDAPNameAttr         string `gorm:"size:64" json:"ldapNameAttr"`          // 姓名属性，如 cn
	LDAPMobileAttr       string `gorm:"size:64" json:"ldapMobileAttr"`        // 手机号属性，如 mobile
	LDAPEmailAttr        string `gorm:"size:64" json:"ldapEmailAttr"`         // 邮箱属性，如 mail
	LDAPMemberAttr       string `gorm:"size:64" json:"ldapMemberAttr"`        // 成员属性，如 member 或 uniqueMember

	// === 数据库字段 ===
	DBType      string `gorm:"size:32" json:"dbType"` // mysql / postgresql / oracle / sqlserver
	Database    string `gorm:"size:128" json:"database"`
	DBUser      string `gorm:"size:128" json:"dbUser"`
	DBPassword  string `gorm:"size:255" json:"-"`
	Charset     string `gorm:"size:32;default:utf8mb4" json:"charset"`
	ServiceName string `gorm:"size:128" json:"serviceName"` // Oracle: Service Name/SID
	UserTable   string `gorm:"size:128" json:"userTable"`
	GroupTable  string `gorm:"size:128" json:"groupTable"`
	RoleTable   string `gorm:"size:128" json:"roleTable"`
	PwdFormat   string `gorm:"size:32;default:sha256" json:"pwdFormat"` // 密码存储格式（17种格式）

	// === 唯一标识字段（用于匹配用户）===
	UniqueField string `gorm:"size:64;default:username" json:"uniqueField"` // username/id/phone/email/employee_id

	// === 数据库字段映射 (JSON) ===
	DBFieldMapping string `gorm:"type:text" json:"dbFieldMapping"` // 自定义字段名映射 JSON

	// === IM 平台通用字段（新增）===
	IMAppID       string `gorm:"size:255" json:"imAppId"`       // AppKey / AppID
	IMAppSecret   string `gorm:"size:255" json:"-"`             // AppSecret (不返回前端)
	IMCorpID      string `gorm:"size:128" json:"imCorpId"`      // CorpID / TenantID
	IMAgentID     string `gorm:"size:128" json:"imAgentId"`     // AgentID (钉钉)
	IMBaseURL     string `gorm:"size:512" json:"imBaseUrl"`     // API 基础地址
	IMCallbackURL string `gorm:"size:512" json:"imCallbackUrl"` // OAuth 回调地址

	// === IM 同步配置 ===
	IMAutoRegister     bool   `gorm:"default:false" json:"imAutoRegister"`                       // 自动创建本地用户
	IMDefaultRoleID    uint   `gorm:"default:0" json:"imDefaultRoleId"`                          // 新用户默认角色
	IMMatchField       string `gorm:"size:32;default:mobile" json:"imMatchField"`                // 匹配字段: mobile/email/userid
	IMUsernameRule     string `gorm:"size:32;default:pinyin" json:"imUsernameRule"`              // 用户名生成规则
	IMSyncInterval     int    `gorm:"default:0" json:"imSyncInterval"`                           // 定时同步间隔(分钟)
	IMGeneratePassword bool   `gorm:"column:im_generate_password;default:true" json:"imGeneratePassword"` // 同步时生成随机密码

	// === IM 免登配置 ===
	IMEnableSSO     bool   `gorm:"column:im_enable_sso;default:false" json:"imEnableSso"`        // 是否启用免登
	IMEnableQRLogin bool   `gorm:"column:im_enable_qr_login;default:false" json:"imEnableQrLogin"` // 是否启用扫码登录（网页端二维码）
	IMSSOPriority   int    `gorm:"column:im_sso_priority;default:0" json:"imSsoPriority"`        // 免登按钮排序
	IMSSOLabel      string `gorm:"column:im_sso_label;size:64" json:"imSsoLabel"`               // 自定义按钮文字

	// === RADIUS 字段 ===
	RadiusServer  string `gorm:"size:255" json:"radiusServer"`           // RADIUS 服务器地址
	RadiusPort    int    `gorm:"default:1812" json:"radiusPort"`         // 认证端口
	RadiusSecret  string `gorm:"size:255" json:"-"`                      // 共享密钥
	RadiusNasIP   string `gorm:"size:64" json:"radiusNasIp"`             // NAS IP
	RadiusTimeout int    `gorm:"default:5" json:"radiusTimeout"`         // 超时时间(秒)

	// === HTTP API 认证字段 ===
	HTTPProtocol      string `gorm:"size:16;default:REST" json:"httpProtocol"`       // REST / SOAP
	HTTPMethod        string `gorm:"size:16;default:POST" json:"httpMethod"`         // POST / GET / PUT
	HTTPEncoding      string `gorm:"size:16;default:UTF-8" json:"httpEncoding"`      // 编码
	HTTPUseTLS        bool   `gorm:"default:true" json:"httpUseTls"`                 // 使用 TLS
	HTTPTLSVersion    string `gorm:"size:16" json:"httpTlsVersion"`                  // TLS1.1/1.2/1.3
	HTTPVerifyCert    bool   `gorm:"default:true" json:"httpVerifyCert"`             // 验证证书
	HTTPURL           string `gorm:"size:512" json:"httpUrl"`                        // 认证接口地址
	HTTPEncryptMethod string `gorm:"size:32" json:"httpEncryptMethod"`               // none/md5/sha256
	HTTPEncryptKey    string `gorm:"size:255" json:"-"`                              // 加密密钥
	HTTPContentType   string `gorm:"size:64;default:application/json" json:"httpContentType"` // Content-Type
	HTTPBodyTemplate  string `gorm:"type:text" json:"httpBodyTemplate"`              // 请求体模板
	HTTPSuccessFlag   string `gorm:"size:255" json:"httpSuccessFlag"`                // 成功标识
	HTTPHeaders       string `gorm:"type:text" json:"httpHeaders"`                   // 自定义请求头 JSON

	// === HTTP API 同步配置（通用化） ===
	HTTPPaginationMode  string `gorm:"size:16;default:page" json:"httpPaginationMode"`    // page(页码) / offset(偏移量) / none(不分页)
	HTTPPageParam       string `gorm:"size:32" json:"httpPageParam"`                      // 页码/偏移量参数名，默认 page 或 firstResult
	HTTPPageSizeParam   string `gorm:"size:32" json:"httpPageSizeParam"`                  // 每页数量参数名，默认 size 或 maxResult
	HTTPPageSize        int    `gorm:"default:200" json:"httpPageSize"`                   // 每页数量
	HTTPDataPath        string `gorm:"size:128" json:"httpDataPath"`                      // 响应中数据列表的 JSON 路径，默认 data.list
	HTTPTotalPath       string `gorm:"size:128" json:"httpTotalPath"`                     // 响应中总数的 JSON 路径，默认 data.total

	// === 连接测试 ===
	LastTestAt  *time.Time `json:"lastTestAt"`
	LastTestOK  bool       `gorm:"default:false" json:"lastTestOk"`
	LastTestMsg string     `gorm:"size:255" json:"lastTestMsg"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// IsIM 判断是否为 IM 平台类型
func (c *Connector) IsIM() bool {
	return c.Type == "im_dingtalk" || c.Type == "im_wechatwork" || c.Type == "im_feishu" || c.Type == "im_welink"
}

// IsDatabase 判断是否为数据库类型连接器
func (c *Connector) IsDatabase() bool {
	return c.Type == "database" || c.Type == "mysql" ||
		c.Type == "db_mysql" || c.Type == "db_postgresql" || c.Type == "db_oracle" || c.Type == "db_sqlserver"
}

// IsLDAP 判断是否为 LDAP/AD 类型（含通用 LDAP 和 OpenLDAP）
func (c *Connector) IsLDAP() bool {
	return c.Type == "ldap_ad" || c.Type == "ldap_generic" || c.Type == "ldap_openldap"
}

// IsOpenLDAP 判断是否为 OpenLDAP 类型
func (c *Connector) IsOpenLDAP() bool {
	return c.Type == "ldap_openldap"
}

// IsRADIUS 判断是否为 RADIUS 类型
func (c *Connector) IsRADIUS() bool {
	return c.Type == "radius"
}

// IsHTTPAPI 判断是否为 HTTP API 类型
func (c *Connector) IsHTTPAPI() bool {
	return c.Type == "http_api"
}

// CanAuthenticate 是否支持认证（用于密码代理和 VPN 认证）
func (c *Connector) CanAuthenticate() bool {
	return c.IsLDAP() || c.IsDatabase() || c.IsRADIUS() || c.IsHTTPAPI()
}

// IsUpstream 是否支持上游
func (c *Connector) IsUpstream() bool {
	return c.Direction == "upstream" || c.Direction == "both"
}

// IsDownstream 是否支持下游
func (c *Connector) IsDownstream() bool {
	return c.Direction == "downstream" || c.Direction == "both"
}

// SupportsSSO 是否支持 SSO 免登
func (c *Connector) SupportsSSO() bool {
	return c.Type == "im_dingtalk" || c.Type == "im_wechatwork" || c.Type == "im_feishu"
}

// IMPlatformName 获取 IM 平台中文名
func (c *Connector) IMPlatformName() string {
	switch c.Type {
	case "im_dingtalk":
		return "钉钉"
	case "im_wechatwork":
		return "企业微信"
	case "im_feishu":
		return "飞书"
	case "im_welink":
		return "WeLink"
	default:
		return ""
	}
}

// EffectiveDBType 获取实际的数据库类型（兼容旧数据）
func (c *Connector) EffectiveDBType() string {
	switch c.Type {
	case "db_mysql":
		return "mysql"
	case "db_postgresql":
		return "postgresql"
	case "db_oracle":
		return "oracle"
	case "db_sqlserver":
		return "sqlserver"
	case "mysql":
		return "mysql"
	}
	if c.DBType != "" {
		return c.DBType
	}
	return "mysql"
}

// ConnectorTypeName 连接器类型的中文名称
func (c *Connector) ConnectorTypeName() string {
	switch c.Type {
	case "im_dingtalk":
		return "钉钉"
	case "im_wechatwork":
		return "企业微信"
	case "im_feishu":
		return "飞书"
	case "im_welink":
		return "WeLink"
	case "ldap_ad":
		return "LDAP/AD"
	case "ldap_generic":
		return "LDAP通用"
	case "ldap_openldap":
		return "OpenLDAP"
	case "db_mysql":
		return "MySQL"
	case "db_postgresql":
		return "PostgreSQL"
	case "db_oracle":
		return "Oracle"
	case "db_sqlserver":
		return "SQL Server"
	case "radius":
		return "RADIUS"
	case "http_api":
		return "HTTP API"
	default:
		return c.Type
	}
}

// SyncRule 同步规则（替代旧 Synchronizer）
type SyncRule struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	ConnectorID uint      `gorm:"not null;index" json:"connectorId"`
	Connector   Connector `gorm:"foreignKey:ConnectorID" json:"connector,omitempty"`
	Direction   string    `gorm:"size:16;not null" json:"direction"` // upstream / downstream

	// === 触发方式 ===
	EnableSchedule bool   `gorm:"default:false" json:"enableSchedule"`
	ScheduleType   string `gorm:"size:16;default:times" json:"scheduleType"` // times=定点时间, interval=间隔
	ScheduleTime   string `gorm:"size:512" json:"scheduleTime"`              // JSON: ["10:30","14:00","18:00"] 或旧格式单时间
	ScheduleInterval int  `gorm:"default:0" json:"scheduleInterval"`         // 间隔(分钟)，scheduleType=interval时使用
	CronExpr       string `gorm:"size:64" json:"cronExpr"`
	EnableEvent    bool   `gorm:"default:true" json:"enableEvent"`
	Events         string `gorm:"type:text" json:"events"` // JSON: ["user_create","user_update",...]
	// === 变更检测 ===
	EnableChangeDetect   bool   `gorm:"default:false" json:"enableChangeDetect"`       // 启用变更检测
	ChangeDetectInterval int    `gorm:"default:60" json:"changeDetectInterval"`         // 变更检测间隔(秒)
	ChangeDetectField    string `gorm:"size:64" json:"changeDetectField"`               // 变更检测字段 (如 updated_at)
	LastChangeDetectAt   *time.Time `json:"lastChangeDetectAt"`                         // 上次变更检测时间

	// === 同步范围 ===
	SyncUsers  bool `gorm:"default:true" json:"syncUsers"`
	SyncGroups bool `gorm:"default:false" json:"syncGroups"`
	SyncRoles  bool `gorm:"default:false" json:"syncRoles"`

	// === 上游专用 ===
	AutoCreateUser   bool `gorm:"default:true" json:"autoCreateUser"`    // 自动创建本地用户
	AutoDisableUser  bool `gorm:"default:true" json:"autoDisableUser"`   // 源端删除时禁用本地用户
	AutoSyncGroups   bool `gorm:"default:true" json:"autoSyncGroups"`    // 自动同步部门为本地群组
	PreservePassword bool `gorm:"default:true" json:"preservePassword"`  // 已有密码不覆盖

	// === 下游专用 ===
	SourceType          string `gorm:"size:32;default:local" json:"sourceType"`
	TargetContainer     string `gorm:"size:255" json:"targetContainer"` // AD: OU DN
	RoleGroupContainer  string `gorm:"size:255" json:"roleGroupContainer"` // 安全组存放的 OU 名称，为空则放在 targetContainer 下
	PreventPwdChange    bool   `gorm:"default:false" json:"preventPwdChange"`

	// === 状态 ===
	Status          int8       `gorm:"default:1" json:"status"`
	LastSyncAt      *time.Time `json:"lastSyncAt"`
	LastSyncStatus  string     `gorm:"size:32" json:"lastSyncStatus"`
	LastSyncMessage string     `gorm:"type:text" json:"lastSyncMessage"`
	SyncCount       int        `gorm:"default:0" json:"syncCount"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// Synchronizer 保持旧模型兼容（用于数据迁移）
type Synchronizer struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	Name                string     `gorm:"size:64;not null" json:"name"`
	ConnectorID         uint       `gorm:"not null;index" json:"connectorId"`
	Connector           Connector  `gorm:"foreignKey:ConnectorID" json:"connector,omitempty"`
	Direction           string     `gorm:"size:16;default:push" json:"direction"`
	SourceType          string     `gorm:"size:32;default:local" json:"sourceType"`
	TargetContainer     string     `gorm:"size:255" json:"targetContainer"`
	RoleGroupContainer  string     `gorm:"size:255" json:"roleGroupContainer"`
	EnableSchedule      bool       `gorm:"default:false" json:"enableSchedule"`
	ScheduleTime        string     `gorm:"size:32" json:"scheduleTime"`
	CronExpr            string     `gorm:"size:64" json:"cronExpr"`
	EnableEvent         bool       `gorm:"default:true" json:"enableEvent"`
	Events              string     `gorm:"type:text" json:"events"`
	SyncUsers           bool       `gorm:"default:true" json:"syncUsers"`
	SyncGroups          bool       `gorm:"default:false" json:"syncGroups"`
	SyncRoles           bool       `gorm:"default:false" json:"syncRoles"`
	PreventPwdChange    bool       `gorm:"default:false" json:"preventPwdChange"`
	Status           int8       `gorm:"default:1" json:"status"`
	LastSyncAt       *time.Time `json:"lastSyncAt"`
	LastSyncStatus   string     `gorm:"size:32" json:"lastSyncStatus"`
	LastSyncMessage  string     `gorm:"type:text" json:"lastSyncMessage"`
	SyncCount        int        `gorm:"default:0" json:"syncCount"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// SyncAttributeMapping 属性映射
type SyncAttributeMapping struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	SynchronizerID  uint   `gorm:"not null;index" json:"synchronizerId"`  // 兼容旧数据
	SyncRuleID      uint   `gorm:"index" json:"syncRuleId"`              // 新字段
	ObjectType      string `gorm:"size:16;not null" json:"objectType"`
	SourceAttribute string `gorm:"size:64;not null" json:"sourceAttribute"`
	TargetAttribute string `gorm:"size:64;not null" json:"targetAttribute"`
	MappingType     string `gorm:"size:16;default:mapping" json:"mappingType"`
	TransformRule   string `gorm:"size:512" json:"transformRule"`
	Priority        int    `gorm:"default:0" json:"priority"`
	IsEnabled       bool   `gorm:"default:true" json:"isEnabled"`
	// 密码格式转换（仅对 password 字段生效）
	PasswordFormat string `gorm:"size:32" json:"passwordFormat"` // 17种格式：bcrypt_sha256, sha256, nt_hash 等
}

// SyncLog 同步日志
type SyncLog struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	SynchronizerID uint      `gorm:"index" json:"synchronizerId"`   // 兼容旧数据
	SyncRuleID     uint      `gorm:"index" json:"syncRuleId"`       // 新字段
	ConnectorID    uint      `gorm:"index" json:"connectorId"`      // 新字段：直接关联连接器
	Direction      string    `gorm:"size:16" json:"direction"`       // upstream / downstream
	TriggerType    string    `gorm:"size:16" json:"triggerType"`
	TriggerEvent   string    `gorm:"size:64" json:"triggerEvent"`
	UserID         uint      `gorm:"index" json:"userId"`
	Username       string    `gorm:"size:64" json:"username"`
	Status         string    `gorm:"size:16" json:"status"`
	Message        string    `gorm:"type:text" json:"message"`
	Detail         string    `gorm:"type:text" json:"detail"`
	AffectedCount  int       `gorm:"default:0" json:"affectedCount"`
	Duration       int64     `json:"duration"`
	CreatedAt      time.Time `json:"createdAt"`
}

// IMDepartment IM 平台部门缓存（通用化）
type IMDepartment struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ConnectorID  uint      `gorm:"not null;index" json:"connectorId"`
	PlatformType string    `gorm:"size:32;not null" json:"platformType"` // im_dingtalk / im_wechatwork / im_feishu / im_welink
	RemoteDeptID string    `gorm:"size:128;not null" json:"remoteDeptId"`
	Name         string    `gorm:"size:128" json:"name"`
	ParentDeptID string    `gorm:"size:128" json:"parentDeptId"`
	SortOrder    int       `json:"sortOrder"`
	MemberCount  int       `json:"memberCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// IMUser IM 平台用户缓存（通用化）
type IMUser struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ConnectorID    uint      `gorm:"not null;index" json:"connectorId"`
	PlatformType   string    `gorm:"size:32;not null" json:"platformType"`
	RemoteUserID   string    `gorm:"size:128;not null" json:"remoteUserId"`
	Name           string    `gorm:"size:64" json:"name"`
	Mobile         string    `gorm:"size:32" json:"mobile"`
	Email          string    `gorm:"size:128" json:"email"`
	Avatar         string    `gorm:"size:512" json:"avatar"`
	JobTitle       string    `gorm:"size:64" json:"jobTitle"`
	DepartmentID   string    `gorm:"size:128" json:"departmentId"`
	DepartmentName string    `gorm:"size:128" json:"departmentName"`
	Active         bool      `gorm:"default:true" json:"active"`
	LocalUserID    uint      `gorm:"index" json:"localUserId"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// DingTalkDepartment 钉钉部门（保留兼容旧数据）
type DingTalkDepartment struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	DeptID      int64     `gorm:"uniqueIndex" json:"deptId"`
	Name        string    `gorm:"size:128" json:"name"`
	ParentID    int64     `gorm:"index" json:"parentId"`
	Order       int64     `json:"order"`
	MemberCount int       `json:"memberCount"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// DingTalkUser 钉钉用户（保留兼容旧数据）
type DingTalkUser struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	DingTalkUID    string    `gorm:"size:128;uniqueIndex" json:"dingtalkUid"`
	Name           string    `gorm:"size:64" json:"name"`
	Mobile         string    `gorm:"size:32" json:"mobile"`
	Email          string    `gorm:"size:128" json:"email"`
	Avatar         string    `gorm:"size:512" json:"avatar"`
	JobTitle       string    `gorm:"size:64" json:"jobTitle"`
	DepartmentID   int64     `gorm:"index" json:"departmentId"`
	DepartmentName string    `gorm:"size:128" json:"departmentName"`
	Active         bool      `gorm:"default:true" json:"active"`
	LocalUserID    uint      `gorm:"index;default:0" json:"localUserId"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// SyncEvent 同步事件常量
const (
	SyncEventUserCreate     = "user_create"
	SyncEventUserUpdate     = "user_update"
	SyncEventUserDelete     = "user_delete"
	SyncEventUserEnable     = "user_enable"
	SyncEventUserDisable    = "user_disable"
	SyncEventPasswordChange = "password_change"
	SyncEventRoleChange     = "role_change"
	SyncEventGroupChange    = "group_change"
)

// AllSyncEvents 所有支持的同步事件
var AllSyncEvents = []struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}{
	{SyncEventUserCreate, "添加用户"},
	{SyncEventUserUpdate, "更新用户"},
	{SyncEventUserDelete, "删除用户"},
	{SyncEventUserEnable, "启用用户"},
	{SyncEventUserDisable, "禁用用户"},
	{SyncEventPasswordChange, "密码修改"},
	{SyncEventRoleChange, "角色变更"},
	{SyncEventGroupChange, "分组变更"},
}

// ConnectorTypeOptions 所有可用的连接器类型
var ConnectorTypeOptions = []struct {
	Type       string `json:"type"`
	Label      string `json:"label"`
	Category   string `json:"category"`   // im / ldap / database / radius / http
	Upstream   bool   `json:"upstream"`
	Downstream bool   `json:"downstream"`
	SSO        bool   `json:"sso"`
	Auth       bool   `json:"auth"`       // 是否支持认证（用于密码代理/VPN）
}{
	{"im_dingtalk", "钉钉 DingTalk", "im", true, false, true, false},
	{"im_wechatwork", "企业微信 WeChatWork", "im", true, false, true, false},
	{"im_feishu", "飞书 FeiShu", "im", true, false, true, false},
	{"im_welink", "WeLink", "im", true, false, false, false},
	{"ldap_ad", "LDAP / Active Directory", "ldap", true, true, false, true},
	{"ldap_openldap", "OpenLDAP", "ldap", true, true, false, true},
	{"ldap_generic", "LDAP 通用", "ldap", false, true, false, true},
	{"db_mysql", "MySQL", "database", true, true, false, true},
	{"db_postgresql", "PostgreSQL", "database", true, true, false, true},
	{"db_oracle", "Oracle", "database", true, true, false, true},
	{"db_sqlserver", "SQL Server", "database", true, true, false, true},
	{"radius", "RADIUS 服务器", "radius", true, false, false, true},
	{"http_api", "HTTP API 认证", "http", true, false, false, true},
}
