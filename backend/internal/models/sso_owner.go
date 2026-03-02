package models

import "time"

type SSOAppOwner struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AppID       uint      `gorm:"index;not null" json:"appId"`
	SubjectType string    `gorm:"size:16;index;not null" json:"subjectType"`
	SubjectID   uint      `gorm:"index;not null" json:"subjectId"`
	Capability  string    `gorm:"size:32;index;not null" json:"capability"`
	CreatedAt   time.Time `json:"createdAt"`
}
