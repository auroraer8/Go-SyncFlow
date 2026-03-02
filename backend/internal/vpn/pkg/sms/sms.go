package sms

import (
	"crypto/rand"
	"math/big"
	"sync"
	"time"
)

// SmsProvider 短信发送接口
type SmsProvider interface {
	Send(phone, code string) error
}

// codeInfo 验证码信息
type codeInfo struct {
	Code      string
	ExpireAt  time.Time
	CreatedAt time.Time
}

var (
	smsCodeCache = make(map[string]codeInfo)
	smsCacheMux  = sync.Mutex{}
)

func init() {
	go cleanExpiredCodes()
}

// cleanExpiredCodes 定时清理过期验证码
func cleanExpiredCodes() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		smsCacheMux.Lock()
		now := time.Now()
		for phone, info := range smsCodeCache {
			if now.After(info.ExpireAt) {
				delete(smsCodeCache, phone)
			}
		}
		smsCacheMux.Unlock()
	}
}

// GenerateCode 生成随机验证码
func GenerateCode(length int) string {
	if length <= 0 {
		length = 6
	}
	digits := "0123456789"
	code := make([]byte, length)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		code[i] = digits[n.Int64()]
	}
	return string(code)
}

// SaveCode 保存验证码
func SaveCode(phone, code string, expireSeconds int) {
	smsCacheMux.Lock()
	defer smsCacheMux.Unlock()
	smsCodeCache[phone] = codeInfo{
		Code:      code,
		ExpireAt:  time.Now().Add(time.Duration(expireSeconds) * time.Second),
		CreatedAt: time.Now(),
	}
}

// VerifyCode 验证码校验
func VerifyCode(phone, code string) bool {
	smsCacheMux.Lock()
	defer smsCacheMux.Unlock()
	info, ok := smsCodeCache[phone]
	if !ok {
		return false
	}
	if time.Now().After(info.ExpireAt) {
		delete(smsCodeCache, phone)
		return false
	}
	if info.Code != code {
		return false
	}
	delete(smsCodeCache, phone)
	return true
}

// CanResend 检查是否可以重新发送（防止频繁发送）
func CanResend(phone string, intervalSeconds int) bool {
	smsCacheMux.Lock()
	defer smsCacheMux.Unlock()
	info, ok := smsCodeCache[phone]
	if !ok {
		return true
	}
	return time.Now().After(info.CreatedAt.Add(time.Duration(intervalSeconds) * time.Second))
}

// GetRemainingSeconds 获取验证码剩余有效秒数
func GetRemainingSeconds(phone string) int {
	smsCacheMux.Lock()
	defer smsCacheMux.Unlock()
	info, ok := smsCodeCache[phone]
	if !ok {
		return 0
	}
	remaining := int(info.ExpireAt.Sub(time.Now()).Seconds())
	if remaining < 0 {
		return 0
	}
	return remaining
}
