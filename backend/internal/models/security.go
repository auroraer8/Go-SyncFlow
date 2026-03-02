package models

import "time"

// LoginAttempt 登录尝试记录
type LoginAttempt struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Username          string    `gorm:"size:64;index;not null" json:"username"`
	UserID            *uint     `gorm:"index" json:"userId"`
	IPAddress         string    `gorm:"size:45;index;not null" json:"ipAddress"`
	UserAgent         string    `gorm:"size:512" json:"userAgent"`
	Success           bool      `gorm:"not null" json:"success"`
	FailureReason     string    `gorm:"size:128" json:"failureReason"`
	Location          string    `gorm:"size:128" json:"location"`
	DeviceFingerprint string    `gorm:"size:128" json:"deviceFingerprint"`
	CreatedAt         time.Time `gorm:"index" json:"createdAt"`
}

// Lockout 锁定记录
type Lockout struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	LockType     string     `gorm:"size:16;not null" json:"lockType"` // account | ip
	Target       string     `gorm:"size:128;index;not null" json:"target"`
	Reason       string     `gorm:"size:255;not null" json:"reason"`
	AttemptCount int        `gorm:"default:0" json:"attemptCount"`
	LockCount    int        `gorm:"default:1" json:"lockCount"` // 第几次锁定
	LockedAt     time.Time  `json:"lockedAt"`
	ExpiresAt    time.Time  `gorm:"index" json:"expiresAt"`
	UnlockedAt   *time.Time `json:"unlockedAt"`
	UnlockedBy   *uint      `json:"unlockedBy"`
	IsActive     bool       `gorm:"default:true;index" json:"isActive"`
}

// IPBlacklist IP黑名单
type IPBlacklist struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	IPAddress    string     `gorm:"size:45;uniqueIndex;not null" json:"ipAddress"`
	IPType       string     `gorm:"size:16;default:single" json:"ipType"` // single | range | cidr
	Reason       string     `gorm:"size:255;not null" json:"reason"`
	Source       string     `gorm:"size:32;default:manual" json:"source"` // manual | auto | threat_intel
	ThreatScore  int        `gorm:"default:0" json:"threatScore"`
	BlockedCount int        `gorm:"default:0" json:"blockedCount"`
	ExpiresAt    *time.Time `json:"expiresAt"` // NULL = 永久
	CreatedBy    *uint      `json:"createdBy"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	IsActive     bool       `gorm:"default:true;index" json:"isActive"`
}

// IPWhitelist IP白名单
type IPWhitelist struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	IPAddress   string    `gorm:"size:45;uniqueIndex;not null" json:"ipAddress"`
	IPType      string    `gorm:"size:16;default:single" json:"ipType"`
	Description string    `gorm:"size:255" json:"description"`
	CreatedBy   *uint     `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	IsActive    bool      `gorm:"default:true;index" json:"isActive"`
}

// SecurityEvent 安全事件
type SecurityEvent struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	EventType   string     `gorm:"size:32;index;not null" json:"eventType"`
	Severity    string     `gorm:"size:16;index;not null" json:"severity"` // low | medium | high | critical
	SourceIP    string     `gorm:"size:45;index" json:"sourceIp"`
	UserID      *uint      `gorm:"index" json:"userId"`
	Username    string     `gorm:"size:64" json:"username"`
	TargetType  string     `gorm:"size:32" json:"targetType"`
	TargetID    string     `gorm:"size:64" json:"targetId"`
	Description string     `gorm:"size:512;not null" json:"description"`
	Details     string     `gorm:"type:text" json:"details"` // JSON
	Location    string     `gorm:"size:128" json:"location"`
	UserAgent   string     `gorm:"size:512" json:"userAgent"`
	IsResolved  bool       `gorm:"default:false" json:"isResolved"`
	ResolvedBy  *uint      `json:"resolvedBy"`
	ResolvedAt  *time.Time `json:"resolvedAt"`
	CreatedAt   time.Time  `gorm:"index" json:"createdAt"`
}

// Session 会话管理
type Session struct {
	ID           string    `gorm:"primaryKey;size:64" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"userId"`
	AccessToken  string    `gorm:"size:512;index;not null" json:"-"`
	RefreshToken string    `gorm:"size:512;index;not null" json:"-"`
	IPAddress    string    `gorm:"size:45;not null" json:"ipAddress"`
	UserAgent    string    `gorm:"size:512" json:"userAgent"`
	DeviceInfo   string    `gorm:"size:512" json:"deviceInfo"` // JSON
	Location     string    `gorm:"size:128" json:"location"`
	IsActive     bool      `gorm:"default:true;index" json:"isActive"`
	LastActivity time.Time `json:"lastActivity"`
	ExpiresAt    time.Time `json:"expiresAt"`
	CreatedAt    time.Time `json:"createdAt"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ConfigKey   string    `gorm:"size:64;uniqueIndex;not null" json:"configKey"`
	ConfigValue string    `gorm:"type:text;not null" json:"configValue"` // JSON
	Description string    `gorm:"size:255" json:"description"`
	UpdatedBy   *uint     `json:"updatedBy"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// NotifyChannel 通知渠道
type NotifyChannel struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"size:64;not null" json:"name"`
	ChannelType string     `gorm:"size:32;not null" json:"channelType"` // email | webhook | sms_aliyun | sms_tencent | sms_custom
	Config      string     `gorm:"type:text;not null" json:"-"`         // 加密JSON
	IsActive    bool       `gorm:"default:true" json:"isActive"`
	TestResult  string     `gorm:"size:255" json:"testResult"`
	TestedAt    *time.Time `json:"testedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	Name              string     `gorm:"size:64;not null" json:"name"`
	AlertType         string     `gorm:"size:16;default:admin" json:"alertType"`   // admin=管理员告警 | employee=员工告警
	EventTypes        string     `gorm:"type:text;not null" json:"eventTypes"`     // JSON数组
	SeverityThreshold string     `gorm:"size:16;default:medium" json:"severityThreshold"`
	Conditions        string     `gorm:"type:text" json:"conditions"`              // JSON
	NotifyChannels    string     `gorm:"type:text;not null" json:"notifyChannels"` // JSON数组 通知渠道ID
	NotifyTarget      string     `gorm:"size:16;default:channel" json:"notifyTarget"` // channel=渠道直发 | role=指定角色 | user=指定用户 | event_user=事件触发者
	NotifyRoleIDs     string     `gorm:"type:text" json:"notifyRoleIds"`           // JSON数组 角色ID（管理员告警+role模式）
	NotifyUserIDs     string     `gorm:"type:text" json:"notifyUserIds"`           // JSON数组 用户ID（管理员告警+user模式）
	TemplateID        uint       `gorm:"default:0" json:"templateId"`              // 关联消息模板ID，0=使用默认模板
	CooldownMinutes   int        `gorm:"default:5" json:"cooldownMinutes"`
	IsActive          bool       `gorm:"default:true" json:"isActive"`
	LastTriggeredAt   *time.Time `json:"lastTriggeredAt"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// AlertLog 告警日志
type AlertLog struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	RuleID       *uint      `gorm:"index" json:"ruleId"`
	EventID      *uint      `gorm:"index" json:"eventId"`
	ChannelID    *uint      `json:"channelId"`
	ChannelType  string     `gorm:"size:32;not null" json:"channelType"`
	Recipient    string     `gorm:"size:255;not null" json:"recipient"`
	Content      string     `gorm:"type:text;not null" json:"content"`
	Status       string     `gorm:"size:16;default:pending" json:"status"` // pending | sent | failed
	ErrorMessage string     `gorm:"size:512" json:"errorMessage"`
	SentAt       *time.Time `json:"sentAt"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// MessageTemplate 消息模板
type MessageTemplate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	Scene     string    `gorm:"size:64;uniqueIndex;not null" json:"scene"` // verify_code | password_reset | security_alert | admin_alert | test | custom_*
	Content   string    `gorm:"type:text;not null" json:"content"`
	Variables string    `gorm:"type:text" json:"variables"` // JSON: [{key, desc, example}]
	IsBuiltin bool      `gorm:"default:false" json:"isBuiltin"`
	IsActive  bool      `gorm:"default:true" json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MessagePolicy 消息策略（场景→通道路由）
type MessagePolicy struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Scene          string    `gorm:"size:64;not null;index" json:"scene"` // verify_code | password_reset_notify | security_alert | admin_alert | test
	SceneName      string    `gorm:"size:64;not null" json:"sceneName"`   // 中文名称
	ChannelIDs     string    `gorm:"type:text" json:"channelIds"`         // JSON数组 [1,2,3]
	TargetType     string    `gorm:"size:16;default:all" json:"targetType"` // all=所有用户 | group=指定群组/角色
	TargetGroupIDs string    `gorm:"type:text" json:"targetGroupIds"`     // JSON数组 群组ID
	TargetRoleIDs  string    `gorm:"type:text" json:"targetRoleIds"`      // JSON数组 角色ID
	Priority       int       `gorm:"default:0" json:"priority"`           // 优先级，数字越大越优先（群组/角色策略 > 默认策略）
	IsActive       bool      `gorm:"default:true" json:"isActive"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// AdminAlertConfig 管理员告警配置（存储在 system_configs 表中，key="admin_alert_config"）
// 管理员定义：拥有指定角色的用户被视为管理员，默认为 admin 角色（roleId=1）
// 通知方式：通过 MessagePolicy 的 admin_alert 场景配置通道（支持 webhook 到钉钉群）

// 安全事件类型常量
const (
	EventLoginSuccess       = "login_success"
	EventLoginFailed        = "login_failed"
	EventLoginBlocked       = "login_blocked"
	EventAccountLocked      = "account_locked"
	EventAccountUnlocked    = "account_unlocked"
	EventPasswordChanged    = "password_changed"
	EventPasswordReset      = "password_reset"
	EventSuspiciousActivity = "suspicious_activity"
	EventPermissionChanged  = "permission_changed"
	EventRoleChanged        = "role_changed"
	EventUserCreated        = "user_created"
	EventUserDeleted        = "user_deleted"
	EventConfigChanged      = "config_changed"
	EventIPBlocked          = "ip_blocked"
	EventIPUnblocked        = "ip_unblocked"
	EventSessionTerminated  = "session_terminated"
	EventBruteForceDetected = "brute_force_detected"
	EventAnomalyDetected    = "anomaly_detected"
	EventGeoBlocked         = "geo_blocked"
)

// 严重级别常量
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// 锁定类型常量
const (
	LockTypeAccount = "account"
	LockTypeIP      = "ip"
)

// APIKey 开放接口密钥
type APIKey struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	AppID       string     `gorm:"size:64;uniqueIndex;not null" json:"appId"`
	AppKey      string     `gorm:"size:128;not null" json:"-"`            // 存储SHA256哈希
	AppKeyHint  string     `gorm:"size:16" json:"appKeyHint"`            // 显示后4位
	Name        string     `gorm:"size:128;not null" json:"name"`        // 名称/备注
	Description string     `gorm:"size:512" json:"description"`          // 详细描述
	Permissions string     `gorm:"type:text" json:"permissions"`         // JSON数组：允许的权限范围 ["user:list","user:create"...]，空=全部
	IPWhitelist string     `gorm:"type:text" json:"ipWhitelist"`         // JSON数组：IP白名单，空=不限制
	IPBlacklist string     `gorm:"type:text" json:"ipBlacklist"`         // JSON数组：IP黑名单
	RateLimit   int        `gorm:"default:60" json:"rateLimit"`          // 每分钟请求上限
	IsActive    bool       `gorm:"default:true;index" json:"isActive"`
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	LastUsedIP  string     `gorm:"size:45" json:"lastUsedIp"`
	UsageCount  int64      `gorm:"default:0" json:"usageCount"`
	ExpiresAt   *time.Time `json:"expiresAt"`                            // 过期时间，NULL=永不过期
	CreatedBy   uint       `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}
