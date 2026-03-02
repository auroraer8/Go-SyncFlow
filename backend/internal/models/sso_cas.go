package models

import "time"

type SSOCasTicket struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	TicketHash string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	AppID      uint       `gorm:"index;not null" json:"appId"`
	Service    string     `gorm:"size:512;not null" json:"service"`
	UserID     uint       `gorm:"index;not null" json:"userId"`
	Username   string     `gorm:"size:64;not null" json:"username"`
	ExpiresAt  time.Time  `gorm:"index;not null" json:"expiresAt"`
	UsedAt     *time.Time `json:"usedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}
