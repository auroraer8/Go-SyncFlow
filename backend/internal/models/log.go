package models

import "time"

type LoginLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"userId"`
	Username  string    `gorm:"size:64" json:"username"`
	IP        string    `gorm:"size:64" json:"ip"`
	UserAgent string    `gorm:"size:512" json:"userAgent"`
	Status    int8      `json:"status"` // 1成功 0失败
	Message   string    `gorm:"size:255" json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type OperationLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"userId"`
	Username  string    `gorm:"size:64" json:"username"`
	Module    string    `gorm:"size:64" json:"module"`
	Action    string    `gorm:"size:64" json:"action"`
	Target    string    `gorm:"size:255" json:"target"`
	Content   string    `gorm:"type:text" json:"content"`
	IP        string    `gorm:"size:64" json:"ip"`
	CreatedAt time.Time `json:"createdAt"`
}

// APIAccessLog API 调用日志
type APIAccessLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	AuthType     string    `gorm:"size:16;index" json:"authType"` // apikey / jwt
	AppID        string    `gorm:"size:64;index" json:"appId"`
	AppName      string    `gorm:"size:128" json:"appName"`       // 调用者名称
	APIKeyID     uint      `json:"apiKeyId"`
	UserID       uint      `json:"userId"`
	Username     string    `gorm:"size:64" json:"username"`
	Method       string    `gorm:"size:8" json:"method"`
	Path         string    `gorm:"size:512;index" json:"path"`
	Query        string    `gorm:"size:1024" json:"query"`
	RequestBody  string    `gorm:"type:text" json:"requestBody"`
	Summary      string    `gorm:"size:256" json:"summary"` // 请求概要（操作对象等）
	StatusCode   int       `json:"statusCode"`
	ResponseSize int       `json:"responseSize"`
	IP           string    `gorm:"size:45;index" json:"ip"`
	UserAgent    string    `gorm:"size:512" json:"userAgent"`
	Duration     int       `json:"duration"` // 耗时(ms)
	ErrorMessage string    `gorm:"size:512" json:"errorMessage"`
	CreatedAt    time.Time `gorm:"index" json:"createdAt"`
}
