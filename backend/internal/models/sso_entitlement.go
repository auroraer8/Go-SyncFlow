package models

import "time"

type SSOAppEntitlement struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AppID       uint      `gorm:"index;not null" json:"appId"`
	Type        string    `gorm:"size:16;not null" json:"type"`
	Key         string    `gorm:"size:128" json:"key"`
	Value       string    `gorm:"size:512;not null" json:"value"`
	Description string    `gorm:"size:512" json:"description"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type SSOAppGrant struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AppID       uint      `gorm:"index;not null" json:"appId"`
	SubjectType string    `gorm:"size:16;not null" json:"subjectType"`
	SubjectID   uint      `gorm:"index;not null" json:"subjectId"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type SSOAppGrantItem struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	GrantID       uint      `gorm:"index;not null" json:"grantId"`
	EntitlementID uint      `gorm:"index;not null" json:"entitlementId"`
	CreatedAt     time.Time `json:"createdAt"`
}

type SSOAppTemplate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AppID     uint      `gorm:"index;not null" json:"appId"`
	Protocol  string    `gorm:"size:16;not null" json:"protocol"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	Template  string    `gorm:"type:text" json:"template"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
