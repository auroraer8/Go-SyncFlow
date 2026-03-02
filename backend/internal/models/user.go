package models

import "time"

type User struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	Username            string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Password            string     `gorm:"size:255;not null" json:"-"`
	Nickname            string     `gorm:"size:64" json:"nickname"`
	Phone               string     `gorm:"size:32" json:"phone"`
	Email               string     `gorm:"size:128" json:"email"`
	Avatar              string     `gorm:"size:512" json:"avatar"`
	Status              int8       `gorm:"default:1" json:"status"`
	IsDeleted           int8       `gorm:"default:0" json:"-"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	ExpiresAt           *time.Time `json:"expiresAt"`                              // 账户到期时间，nil表示永不过期
	Roles               []Role     `gorm:"many2many:user_roles" json:"roles,omitempty"`
	// 用户来源
	Source              string     `gorm:"size:16;default:local" json:"source"`    // local / dingtalk
	DingTalkUID         string     `gorm:"size:128;index" json:"dingtalkUid"`      // 钉钉UserID
	IMUserID            string     `gorm:"size:128" json:"imUserId"`               // 通用IM UserID（企微/飞书等）
	DepartmentID        int64      `json:"departmentId"`                           // 所属部门钉钉ID
	DepartmentName      string     `gorm:"size:128" json:"departmentName"`         // 部门名称
	DepartmentPath      string     `gorm:"size:512" json:"departmentPath"`         // 部门完整路径，如：职能/行政/行政
	JobTitle            string     `gorm:"size:64" json:"jobTitle"`                // 职位
	GroupID             uint       `gorm:"index;default:0" json:"groupId"`         // 本地分组ID
	SambaNTPassword     string     `gorm:"size:64" json:"-"`                       // Samba NT密码哈希
	// 安全相关字段
	PasswordChangedAt   *time.Time `json:"passwordChangedAt"`
	PasswordHistory     string     `gorm:"type:text" json:"-"`
	FailedAttempts      int        `gorm:"default:0" json:"-"`
	LockedUntil         *time.Time `json:"lockedUntil"`
	LockCount           int        `gorm:"default:0" json:"-"`
	MFAEnabled          bool       `gorm:"default:false" json:"mfaEnabled"`
	MFASecret           string     `gorm:"size:128" json:"-"`
	LastLoginIP         string     `gorm:"size:45" json:"lastLoginIp"`
	LastLoginAt         *time.Time `json:"lastLoginAt"`
	ForcePasswordChange bool       `gorm:"default:false" json:"forcePasswordChange"`
}

// IsExpired 检查用户是否已过期
func (u *User) IsExpired() bool {
	if u.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*u.ExpiresAt)
}

type Role struct {
	ID           uint         `gorm:"primaryKey" json:"id"`
	Name         string       `gorm:"size:64;not null" json:"name"`
	Code         string       `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Description  string       `gorm:"size:255" json:"description"`
	Status       int8         `gorm:"default:1" json:"status"`
	SidebarMode  string       `gorm:"size:16;default:auto" json:"sidebarMode"`  // auto=自动 | visible=始终显示 | hidden=始终隐藏
	LandingPage  string       `gorm:"size:255" json:"landingPage"`              // 自定义登录后首页路径，空则自动检测
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`
	Permissions  []Permission `gorm:"many2many:role_permissions" json:"permissions,omitempty"`
}

type Permission struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	ParentID uint   `gorm:"default:0" json:"parentId"`
	Name     string `gorm:"size:64;not null" json:"name"`
	Code     string `gorm:"size:128;uniqueIndex;not null" json:"code"`
	Type     string `gorm:"size:32" json:"type"`
	Path     string `gorm:"size:255" json:"path"`
	Icon     string `gorm:"size:64" json:"icon"`
	Sort     int    `gorm:"default:0" json:"sort"`
}

// UserGroup 本地用户分组
type UserGroup struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Name           string    `gorm:"size:128;not null" json:"name"`
	ParentID       uint      `gorm:"index;default:0" json:"parentId"`
	Order          int       `gorm:"default:0" json:"order"`
	DingTalkDeptID int64     `gorm:"index;default:0" json:"dingtalkDeptId"` // 关联的钉钉部门ID，0表示本地创建
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type UserRole struct {
	ID     uint `gorm:"primaryKey"`
	UserID uint `gorm:"index"`
	RoleID uint `gorm:"index"`
}

type RolePermission struct {
	ID           uint `gorm:"primaryKey"`
	RoleID       uint `gorm:"index"`
	PermissionID uint `gorm:"index"`
}

// RoleAutoAssignRule 角色自动分配规则
type RoleAutoAssignRule struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RoleID    uint      `gorm:"index;not null" json:"roleId"`
	RuleType  string    `gorm:"size:32;not null" json:"ruleType"` // group / job_title
	RuleValue string    `gorm:"size:255;not null" json:"ruleValue"` // groupId 或 职位名称
	CreatedAt time.Time `json:"createdAt"`
}
