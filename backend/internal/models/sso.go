package models

import "time"

type SSOApp struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"size:128;not null" json:"name"`
	Code        string `gorm:"size:64;not null;uniqueIndex" json:"code"`
	Enabled     bool   `gorm:"default:true" json:"enabled"`
	Description string `gorm:"size:512" json:"description"`

	AccessMode  string `gorm:"size:32;not null;default:all" json:"accessMode"`
	ClaimPolicy string `gorm:"type:text" json:"claimPolicy"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Protocols []SSOAppProtocol `gorm:"foreignKey:AppID;references:ID;constraint:OnDelete:CASCADE" json:"protocols,omitempty"`
}

type SSOAppProtocol struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	AppID    uint   `gorm:"index;not null" json:"appId"`
	Protocol string `gorm:"size:16;not null" json:"protocol"`
	Enabled  bool   `gorm:"default:true" json:"enabled"`
	Config   string `gorm:"type:text" json:"config"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SSOAppMapping struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	AppID    uint   `gorm:"index;not null" json:"appId"`
	Type     string `gorm:"size:16;not null" json:"type"`
	Source   string `gorm:"size:128;not null" json:"source"`
	Target   string `gorm:"size:128;not null" json:"target"`
	Remark   string `gorm:"size:512" json:"remark"`
	Priority int    `gorm:"default:0" json:"priority"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SSOAuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AppID     uint      `gorm:"index" json:"appId"`
	Protocol  string    `gorm:"size:16" json:"protocol"`
	UserID    uint      `gorm:"index" json:"userId"`
	Username  string    `gorm:"size:64" json:"username"`
	Action    string    `gorm:"size:32" json:"action"`
	Success   bool      `gorm:"default:false" json:"success"`
	Message   string    `gorm:"size:512" json:"message"`
	ClientIP  string    `gorm:"size:64" json:"clientIp"`
	RequestID string    `gorm:"size:64" json:"requestId"`
	CreatedAt time.Time `json:"createdAt"`
}
