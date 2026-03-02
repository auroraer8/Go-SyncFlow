package sms

import (
	"errors"
	"regexp"

	"go-syncflow/internal/vpn/dbdata"
)

// NewProvider 根据配置创建短信发送器
func NewProvider(config *dbdata.SettingSms) (SmsProvider, error) {
	switch config.Provider {
	case "aliyun":
		return NewAliyunProvider(config), nil
	case "tencent":
		return NewTencentProvider(config), nil
	case "custom":
		return NewCustomProvider(config), nil
	default:
		return nil, errors.New("unknown sms provider")
	}
}

// SendCode 发送验证码
func SendCode(phone string, config *dbdata.SettingSms) (string, error) {
	if !config.Enabled {
		return "", errors.New("短信功能未启用")
	}
	if !validatePhone(phone) {
		return "", errors.New("手机号格式不正确")
	}
	if !CanResend(phone, 60) {
		return "", errors.New("发送过于频繁，请稍后再试")
	}
	codeLength := config.CodeLength
	if codeLength <= 0 {
		codeLength = 6
	}
	code := GenerateCode(codeLength)
	provider, err := NewProvider(config)
	if err != nil {
		return "", err
	}
	err = provider.Send(phone, code)
	if err != nil {
		return "", err
	}
	expireSeconds := config.CodeExpire
	if expireSeconds <= 0 {
		expireSeconds = 300
	}
	SaveCode(phone, code, expireSeconds)
	return code, nil
}

// validatePhone 简单验证手机号格式
func validatePhone(phone string) bool {
	re := regexp.MustCompile(`^1[3-9]\d{9}$`)
	return re.MatchString(phone)
}
