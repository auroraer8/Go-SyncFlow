package dbdata

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"reflect"

	"go-syncflow/internal/models"
	"go-syncflow/internal/services"
	"go-syncflow/internal/storage"
	"go-syncflow/internal/sync"

	"golang.org/x/crypto/bcrypt"
)

// SyncFlow 本地用户认证
// 使用 Go-SyncFlow 的本地用户系统进行 VPN 认证

func init() {
	authRegistry["syncflow"] = reflect.TypeOf(AuthSyncFlow{})
}

type AuthSyncFlow struct{}

func (a AuthSyncFlow) checkData(authData map[string]interface{}) error {
	return nil
}

func (a AuthSyncFlow) checkUser(name, pwd string, g *Group, ext map[string]interface{}) error {
	// 1. 从 Go-SyncFlow 用户表查找用户
	var user models.User
	if err := storage.DB.Where("username = ? AND is_deleted = 0", name).First(&user).Error; err != nil {
		return fmt.Errorf("%s 用户不存在或用户已停用", name)
	}

	// 2. 检查用户状态
	if user.Status != 1 {
		return fmt.Errorf("%s 用户已停用", name)
	}

	// 3. 检查使用范围（用户/部门权限）
	if !a.checkUserScope(user, g) {
		return fmt.Errorf("%s 无权限使用此 VPN 用户组", name)
	}

	// 4. 验证密码
	// Go-SyncFlow 密码存储格式: bcrypt(SHA256(原始密码))
	// VPN 客户端发送的是明文密码，需要先进行 SHA256 哈希
	passwordHash := hashSHA256(pwd)

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(passwordHash)); err != nil {
		// 本地密码验证失败，尝试密码代理认证
		if services.IsPasswordAuthProxyEnabled() {
			log.Printf("[VPN-密码代理] 本地密码验证失败，尝试代理认证: %s", name)
			authenticated, learned, proxyErr := services.ProxyAuthenticateUser(&user, pwd)
			if authenticated {
				log.Printf("[VPN-密码代理] 用户 %s 代理认证成功, 密码已学习: %v", name, learned)
				// 如果密码学习成功，触发同步事件
				if learned {
					sync.DispatchSyncEvent(models.SyncEventPasswordChange, user.ID, pwd)
					log.Printf("[VPN-密码同步] 已触发用户 %s 密码同步事件", name)
				}
				return nil
			}
			log.Printf("[VPN-密码代理] 用户 %s 代理认证失败: %v", name, proxyErr)
		}
		return fmt.Errorf("%s 密码错误", name)
	}

	return nil
}

// hashSHA256 计算字符串的 SHA256 哈希（与 Go-SyncFlow 登录一致）
func hashSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// checkUserScope 检查用户是否在允许的使用范围内
func (a AuthSyncFlow) checkUserScope(user models.User, g *Group) bool {
	// 如果没有设置使用范围，则允许所有用户
	if len(g.AllowedUserIDs) == 0 && len(g.AllowedGroupIDs) == 0 {
		return true
	}

	// 检查用户 ID 是否在允许列表中
	for _, uid := range g.AllowedUserIDs {
		if uid == user.ID {
			return true
		}
	}

	// 检查用户所属部门是否在允许列表中（包含下级部门）
	if len(g.AllowedGroupIDs) > 0 && user.GroupID > 0 {
		// 获取允许的部门及其所有下级部门ID
		allowedGroupIDs := getAllChildGroupIDs(g.AllowedGroupIDs)

		// 检查用户的部门是否在允许列表中
		for _, gid := range allowedGroupIDs {
			if gid == user.GroupID {
				return true
			}
		}
	}

	return false
}

// getAllChildGroupIDs 获取指定部门ID列表及其所有下级部门的ID
func getAllChildGroupIDs(parentIDs []uint) []uint {
	if len(parentIDs) == 0 {
		return nil
	}

	// 使用 map 去重
	resultMap := make(map[uint]bool)
	for _, id := range parentIDs {
		resultMap[id] = true
	}

	// 递归获取所有下级部门
	currentLevel := parentIDs
	for len(currentLevel) > 0 {
		var childGroups []models.UserGroup
		storage.DB.Where("parent_id IN ?", currentLevel).Find(&childGroups)

		if len(childGroups) == 0 {
			break
		}

		currentLevel = make([]uint, 0, len(childGroups))
		for _, g := range childGroups {
			if !resultMap[g.ID] {
				resultMap[g.ID] = true
				currentLevel = append(currentLevel, g.ID)
			}
		}
	}

	// 转换为切片
	result := make([]uint, 0, len(resultMap))
	for id := range resultMap {
		result = append(result, id)
	}
	return result
}

// GetSyncFlowUserPhone 获取 SyncFlow 用户的手机号
func GetSyncFlowUserPhone(username string) string {
	var user models.User
	if err := storage.DB.Where("username = ? AND is_deleted = 0", username).First(&user).Error; err != nil {
		return ""
	}
	return user.Phone
}

// IsSyncFlowUserSmsDisabled 检查 SyncFlow 用户是否禁用短信验证
// 对于 SyncFlow 用户，默认不禁用短信验证
func IsSyncFlowUserSmsDisabled(username string) bool {
	// 可以根据需要添加用户级别的短信禁用逻辑
	return false
}

// ValidateSyncFlowUserExists 验证 SyncFlow 用户是否存在
func ValidateSyncFlowUserExists(username string) bool {
	var count int64
	storage.DB.Model(&models.User{}).Where("username = ? AND is_deleted = 0 AND status = 1", username).Count(&count)
	return count > 0
}

// GetSyncFlowUserNickname 获取 SyncFlow 用户的昵称
func GetSyncFlowUserNickname(username string) string {
	var user models.User
	if err := storage.DB.Where("username = ? AND is_deleted = 0", username).First(&user).Error; err != nil {
		return ""
	}
	return user.Nickname
}
