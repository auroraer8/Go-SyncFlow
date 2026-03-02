package models

import "time"

type SSOOAuthCode struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	CodeHash            string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	AppID               uint       `gorm:"index;not null" json:"appId"`
	ClientID            string     `gorm:"size:128;not null" json:"clientId"`
	UserID              uint       `gorm:"index;not null" json:"userId"`
	RedirectURI         string     `gorm:"size:512;not null" json:"redirectUri"`
	Scope               string     `gorm:"size:512" json:"scope"`
	State               string     `gorm:"size:256" json:"state"`
	Nonce               string     `gorm:"size:256" json:"nonce"`
	CodeChallenge       string     `gorm:"size:256" json:"codeChallenge"`
	CodeChallengeMethod string     `gorm:"size:16" json:"codeChallengeMethod"`
	ExpiresAt           time.Time  `gorm:"index" json:"expiresAt"`
	UsedAt              *time.Time `json:"usedAt"`
	CreatedAt           time.Time  `json:"createdAt"`
}
