package vpn

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/vpn/admin"
)

// RegisterRoutes 注册 VPN 相关路由（JWT认证）
func RegisterRoutes(auth *gin.RouterGroup) {
	// 注册所有 VPN 管理路由（包括服务控制和 admin API）
	RegisterAdminRoutes(auth)
}

// RegisterOpenAPIRoutes 注册 VPN Open API 路由（AppID/AppKey认证）
func RegisterOpenAPIRoutes(openAPI *gin.RouterGroup) {
	vpn := openAPI.Group("/vpn")

	// ========== 服务控制 ==========
	vpn.GET("/service/status", GetServiceStatus)
	vpn.POST("/service/start", StartService)
	vpn.POST("/service/stop", StopService)
	vpn.POST("/service/restart", RestartService)

	// ========== 仪表盘 ==========
	vpn.GET("/dashboard", handleDashboard)

	// ========== 设置 ==========
	vpn.GET("/settings", handleGetSettings)
	vpn.PUT("/settings", handleUpdateSettings)

	// ========== 用户组管理 ==========
	vpn.GET("/groups", handleGroupList)
	vpn.POST("/groups", handleGroupCreate)
	vpn.GET("/groups/:id", handleGroupDetail)
	vpn.PUT("/groups/:id", handleGroupUpdate)
	vpn.DELETE("/groups/:id", handleGroupDelete)

	// ========== VPN 用户管理 ==========
	vpn.GET("/users", wrapHandler(admin.UserList))
	vpn.GET("/users/:id", handleUserDetail)
	vpn.PUT("/users/:id", handleUserUpdate)
	vpn.POST("/users/:id/reset-pin", handleUserResetPin)

	// ========== 在线用户 ==========
	vpn.GET("/online", handleOnlineUsers)
	vpn.GET("/online/stats", handleOnlineStats)
	vpn.POST("/online/:token/kick", handleKickUser)

	// ========== IP 映射 ==========
	vpn.GET("/ip-maps", wrapHandler(admin.UserIpMapList))
	vpn.POST("/ip-maps", handleIPMapCreate)
	vpn.PUT("/ip-maps/:id", handleIPMapUpdate)
	vpn.DELETE("/ip-maps/:id", handleIPMapDelete)

	// ========== 访问策略 ==========
	vpn.GET("/policies", wrapHandler(admin.PolicyList))
	vpn.GET("/policies/detail", wrapHandler(admin.PolicyDetail))
	vpn.POST("/policies/set", wrapHandler(admin.PolicySet))
	vpn.POST("/policies/del", wrapHandler(admin.PolicyDel))

	// ========== 审计日志 ==========
	vpn.GET("/audit", handleAuditList)
	vpn.GET("/user-logs", handleUserActLogList)
}

// GetServiceStatus 获取服务状态
func GetServiceStatus(c *gin.Context) {
	status := Status()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// StartService 启动服务
func StartService(c *gin.Context) {
	if err := Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "VPN 服务已启动",
	})
}

// StopService 停止服务
func StopService(c *gin.Context) {
	if err := Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "VPN 服务已停止",
	})
}

// RestartService 重启服务
func RestartService(c *gin.Context) {
	if err := Restart(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "VPN 服务已重启",
	})
}
