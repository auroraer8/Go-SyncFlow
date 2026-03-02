package services

import (
	"log"
	"time"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
	syncer "go-syncflow/internal/sync"
)

// StartUserExpiryScheduler 启动用户过期检查调度器
// 每小时检查一次，自动禁用过期用户并触发下游同步
func StartUserExpiryScheduler() {
	go func() {
		// 启动时先执行一次
		time.Sleep(30 * time.Second) // 等待数据库初始化完成
		checkAndDisableExpiredUsers()

		// 每小时执行一次
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			checkAndDisableExpiredUsers()
		}
	}()
	log.Println("[用户过期] 定时检查调度器已启动（每小时执行）")
}

// checkAndDisableExpiredUsers 检查并禁用过期用户
func checkAndDisableExpiredUsers() {
	now := time.Now()
	
	// 查找所有已过期但仍启用的用户
	var expiredUsers []models.User
	storage.DB.Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ?", 1, now).
		Preload("Roles").
		Find(&expiredUsers)

	if len(expiredUsers) == 0 {
		return
	}

	log.Printf("[用户过期] 发现 %d 个过期用户需要禁用", len(expiredUsers))

	for _, user := range expiredUsers {
		// 禁用用户
		if err := storage.DB.Model(&user).Update("status", 0).Error; err != nil {
			log.Printf("[用户过期] 禁用用户失败: %s, error: %v", user.Username, err)
			continue
		}

		log.Printf("[用户过期] 已禁用过期用户: %s (过期时间: %s)", 
			user.Username, user.ExpiresAt.Format("2006-01-02 15:04:05"))

		// 触发下游同步事件（禁用事件）
		syncer.DispatchSyncEvent(models.SyncEventUserDisable, user.ID, "")

		// 记录操作日志
		storage.DB.Create(&models.OperationLog{
			UserID:   0, // 系统自动操作
			Username: "system",
			Module:   "用户管理",
			Action:   "自动禁用",
			Target:   user.Username,
			Content:  "账户已过期自动禁用",
			IP:       "127.0.0.1",
		})
	}

	log.Printf("[用户过期] 完成过期用户处理，共禁用 %d 人", len(expiredUsers))
}

// CheckUserExpiry 手动触发一次过期检查（可通过API调用）
func CheckUserExpiry() (int, error) {
	now := time.Now()
	
	var expiredUsers []models.User
	storage.DB.Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ?", 1, now).
		Preload("Roles").
		Find(&expiredUsers)

	if len(expiredUsers) == 0 {
		return 0, nil
	}

	count := 0
	for _, user := range expiredUsers {
		if err := storage.DB.Model(&user).Update("status", 0).Error; err != nil {
			log.Printf("[用户过期] 禁用用户失败: %s, error: %v", user.Username, err)
			continue
		}

		syncer.DispatchSyncEvent(models.SyncEventUserDisable, user.ID, "")

		storage.DB.Create(&models.OperationLog{
			UserID:   0,
			Username: "system",
			Module:   "用户管理",
			Action:   "自动禁用",
			Target:   user.Username,
			Content:  "账户已过期自动禁用",
			IP:       "127.0.0.1",
		})

		count++
	}

	return count, nil
}
