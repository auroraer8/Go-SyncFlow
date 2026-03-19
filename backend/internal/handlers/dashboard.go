package handlers

import (
	"net/http"
	"sync"
	"time"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"

	"github.com/gin-gonic/gin"
)

// DashboardStats 仪表盘统计数据
type DashboardStats struct {
	Users      UserStats      `json:"users"`
	Connectors ConnectorStats `json:"connectors"`
	Roles      RoleStats      `json:"roles"`
	SSOApps    SSOStats       `json:"ssoApps"`
	Sync       SyncTaskStats  `json:"sync"`
	Timestamp  int64          `json:"timestamp"`
}

type UserStats struct {
	Total    int64 `json:"total"`
	Active   int64 `json:"active"`
	Disabled int64 `json:"disabled"`
	Today    int64 `json:"today"`
}

type ConnectorStats struct {
	Total      int64 `json:"total"`
	Upstream   int64 `json:"upstream"`
	Downstream int64 `json:"downstream"`
	Online     int64 `json:"online"`
}

type RoleStats struct {
	Total       int64 `json:"total"`
	Permissions int64 `json:"permissions"`
	Assignments int64 `json:"assignments"`
}

type SSOStats struct {
	Total   int64 `json:"total"`
	Enabled int64 `json:"enabled"`
}

type SyncTaskStats struct {
	Total    int64  `json:"total"`
	Enabled  int64  `json:"enabled"`
	LastSync string `json:"lastSync"`
}

// 缓存相关
var (
	dashboardCache     *DashboardStats
	dashboardCacheMu   sync.RWMutex
	dashboardCacheTime time.Time
	dashboardCacheTTL  = 10 * time.Second // 缓存10秒
)

// GetDashboardStats 获取仪表盘统计数据
func GetDashboardStats(c *gin.Context) {
	// 检查缓存
	dashboardCacheMu.RLock()
	if dashboardCache != nil && time.Since(dashboardCacheTime) < dashboardCacheTTL {
		stats := *dashboardCache
		dashboardCacheMu.RUnlock()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    stats,
			"cached":  true,
		})
		return
	}
	dashboardCacheMu.RUnlock()

	// 重新计算统计数据
	stats := DashboardStats{
		Timestamp: time.Now().Unix(),
	}

	// 使用 goroutine 并行查询提高性能
	var wg sync.WaitGroup
	wg.Add(5)

	// 用户统计
	go func() {
		defer wg.Done()
		storage.DB.Model(&models.User{}).Where("is_deleted = 0").Count(&stats.Users.Total)
		storage.DB.Model(&models.User{}).Where("is_deleted = 0 AND status = 1").Count(&stats.Users.Active)
		storage.DB.Model(&models.User{}).Where("is_deleted = 0 AND status = 0").Count(&stats.Users.Disabled)
		// 今日新增
		today := time.Now().Format("2006-01-02")
		storage.DB.Model(&models.User{}).Where("is_deleted = 0 AND DATE(created_at) = ?", today).Count(&stats.Users.Today)
	}()

	// 连接器统计
	go func() {
		defer wg.Done()
		storage.DB.Model(&models.Connector{}).Where("status = 1").Count(&stats.Connectors.Online)
		storage.DB.Model(&models.Connector{}).Count(&stats.Connectors.Total)
		storage.DB.Model(&models.Connector{}).Where("direction = 'upstream' OR direction = 'both'").Count(&stats.Connectors.Upstream)
		storage.DB.Model(&models.Connector{}).Where("direction = 'downstream' OR direction = 'both'").Count(&stats.Connectors.Downstream)
	}()

	// 角色统计
	go func() {
		defer wg.Done()
		storage.DB.Model(&models.Role{}).Count(&stats.Roles.Total)
		storage.DB.Model(&models.RolePermission{}).Count(&stats.Roles.Permissions)
		storage.DB.Model(&models.UserRole{}).Count(&stats.Roles.Assignments)
	}()

	// SSO 应用统计
	go func() {
		defer wg.Done()
		storage.DB.Model(&models.SSOApp{}).Count(&stats.SSOApps.Total)
		storage.DB.Model(&models.SSOApp{}).Where("enabled = ?", true).Count(&stats.SSOApps.Enabled)
	}()

	// 同步任务统计
	go func() {
		defer wg.Done()
		storage.DB.Model(&models.Synchronizer{}).Count(&stats.Sync.Total)
		storage.DB.Model(&models.Synchronizer{}).Where("status = ?", 1).Count(&stats.Sync.Enabled)
		// 获取最近同步时间
		var lastLog models.SyncLog
		if storage.DB.Order("created_at DESC").First(&lastLog).Error == nil {
			stats.Sync.LastSync = lastLog.CreatedAt.Format("01-02 15:04")
		} else {
			stats.Sync.LastSync = "-"
		}
	}()

	wg.Wait()

	// 更新缓存
	dashboardCacheMu.Lock()
	dashboardCache = &stats
	dashboardCacheTime = time.Now()
	dashboardCacheMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
		"cached":  false,
	})
}

// InvalidateDashboardCache 使仪表盘缓存失效（供其他模块调用）
func InvalidateDashboardCache() {
	dashboardCacheMu.Lock()
	dashboardCache = nil
	dashboardCacheMu.Unlock()
}
