package services

import (
	"fmt"

	"go-syncflow/internal/crypto"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

// 密码格式常量 - 重新导出
const (
	PwdFormatBcryptSHA256 = crypto.PwdFormatBcryptSHA256
	PwdFormatBcrypt       = crypto.PwdFormatBcrypt
	PwdFormatArgon2id     = crypto.PwdFormatArgon2id
	PwdFormatSHA256       = crypto.PwdFormatSHA256
	PwdFormatSHA512       = crypto.PwdFormatSHA512
	PwdFormatSHA1         = crypto.PwdFormatSHA1
	PwdFormatSHA384       = crypto.PwdFormatSHA384
	PwdFormatSHA3_256     = crypto.PwdFormatSHA3_256
	PwdFormatSHA3_512     = crypto.PwdFormatSHA3_512
	PwdFormatMD5          = crypto.PwdFormatMD5
	PwdFormatSSHA         = crypto.PwdFormatSSHA
	PwdFormatSSHA256      = crypto.PwdFormatSSHA256
	PwdFormatNTHash       = crypto.PwdFormatNTHash
	PwdFormatSCRAMSHA256  = crypto.PwdFormatSCRAMSHA256
	PwdFormatPBKDF2       = crypto.PwdFormatPBKDF2
	PwdFormatDjango       = crypto.PwdFormatDjango
	PwdFormatPlaintext    = crypto.PwdFormatPlaintext
)

// PasswordFormatInfo 密码格式信息 - 重新导出
type PasswordFormatInfo = crypto.PasswordFormatInfo

// AllPasswordFormats 所有支持的密码格式 - 重新导出
var AllPasswordFormats = crypto.AllPasswordFormats

// HashPassword 根据格式哈希密码 - 委托给 crypto 包
func HashPassword(password, format string) (string, error) {
	return crypto.HashPassword(password, format)
}

// GetDefaultPasswordFormat 获取连接器类型的默认密码格式 - 委托给 crypto 包
func GetDefaultPasswordFormat(connectorType, dbType string) string {
	return crypto.GetDefaultPasswordFormat(connectorType, dbType)
}

// GetPasswordFormatLabel 获取密码格式的显示标签 - 委托给 crypto 包
func GetPasswordFormatLabel(format string) string {
	return crypto.GetPasswordFormatLabel(format)
}

// _dummy 使用 fmt 包
var _ = fmt.Sprintf

// GetSystemPasswordPolicy 获取系统密码存储策略
func GetSystemPasswordPolicy() string {
	if sysConfig, err := GetSystemConfigValue("password_storage_policy"); err == nil && sysConfig != "" {
		return sysConfig
	}
	return PwdFormatBcryptSHA256
}

// SetSystemPasswordPolicy 设置系统密码存储策略
func SetSystemPasswordPolicy(policy string) error {
	valid := false
	for _, f := range AllPasswordFormats {
		if f.Value == policy {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid password policy: %s", policy)
	}
	return SaveSystemConfigValue("password_storage_policy", policy)
}

// GetSystemConfigValue 获取系统配置值
func GetSystemConfigValue(key string) (string, error) {
	var sysConfig models.SystemConfig
	if err := storage.DB.Where("key = ?", key).First(&sysConfig).Error; err != nil {
		return "", err
	}
	return sysConfig.Value, nil
}

// SaveSystemConfigValue 保存系统配置值
func SaveSystemConfigValue(key, value string) error {
	var sysConfig models.SystemConfig
	if err := storage.DB.Where("key = ?", key).First(&sysConfig).Error; err != nil {
		sysConfig = models.SystemConfig{
			Key:   key,
			Value: value,
		}
		return storage.DB.Create(&sysConfig).Error
	}
	sysConfig.Value = value
	return storage.DB.Save(&sysConfig).Error
}
