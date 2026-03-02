package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

func PermissionMiddleware(requiredCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
			c.Abort()
			return
		}

		// 查询用户角色
		var userRoles []models.UserRole
		storage.DB.Where("user_id = ?", userID).Find(&userRoles)

		if len(userRoles) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "无权限访问"})
			c.Abort()
			return
		}

		// 查询角色权限
		var roleIDs []uint
		for _, ur := range userRoles {
			roleIDs = append(roleIDs, ur.RoleID)
		}

		var rolePerms []models.RolePermission
		storage.DB.Where("role_id IN ?", roleIDs).Find(&rolePerms)

		var permIDs []uint
		for _, rp := range rolePerms {
			permIDs = append(permIDs, rp.PermissionID)
		}

		// 查询是否有所需权限
		var perm models.Permission
		if err := storage.DB.Where("id IN ? AND code = ?", permIDs, requiredCode).First(&perm).Error; err != nil {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "无权限访问"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// PermissionAnyMiddleware 任意一个权限满足即可通过
func PermissionAnyMiddleware(codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
			c.Abort()
			return
		}

		var userRoles []models.UserRole
		storage.DB.Where("user_id = ?", userID).Find(&userRoles)
		if len(userRoles) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "无权限访问"})
			c.Abort()
			return
		}

		var roleIDs []uint
		for _, ur := range userRoles {
			roleIDs = append(roleIDs, ur.RoleID)
		}

		var rolePerms []models.RolePermission
		storage.DB.Where("role_id IN ?", roleIDs).Find(&rolePerms)

		var permIDs []uint
		for _, rp := range rolePerms {
			permIDs = append(permIDs, rp.PermissionID)
		}

		// 查询是否有任一所需权限
		var count int64
		storage.DB.Model(&models.Permission{}).Where("id IN ? AND code IN ?", permIDs, codes).Count(&count)
		if count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "无权限访问"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CheckPermission 检查当前请求用户是否拥有指定权限（不中断请求，仅返回布尔值）
func CheckPermission(c *gin.Context, code string) bool {
	userID := GetUserID(c)
	if userID == 0 {
		return false
	}
	perms := GetUserPermissions(userID)
	for _, p := range perms {
		if p == code {
			return true
		}
	}
	return false
}

func GetUserPermissions(userID uint) []string {
	var userRoles []models.UserRole
	storage.DB.Where("user_id = ?", userID).Find(&userRoles)

	if len(userRoles) == 0 {
		return nil
	}

	var roleIDs []uint
	for _, ur := range userRoles {
		roleIDs = append(roleIDs, ur.RoleID)
	}

	var rolePerms []models.RolePermission
	storage.DB.Where("role_id IN ?", roleIDs).Find(&rolePerms)

	var permIDs []uint
	for _, rp := range rolePerms {
		permIDs = append(permIDs, rp.PermissionID)
	}

	var perms []models.Permission
	storage.DB.Where("id IN ?", permIDs).Find(&perms)

	var codes []string
	for _, p := range perms {
		codes = append(codes, p.Code)
	}
	return codes
}
