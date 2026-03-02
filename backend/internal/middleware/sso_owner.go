package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

func SSOAppPermissionOrOwnerMiddleware(appIDParam string, capability string, codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
			c.Abort()
			return
		}

		if hasAnyPermission(userID, codes...) {
			c.Next()
			return
		}

		appID, _ := strconv.Atoi(c.Param(appIDParam))
		if appID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
			c.Abort()
			return
		}

		if isSSOAppOwner(userID, uint(appID), capability) {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "无权限访问"})
		c.Abort()
	}
}

func SSOEntitlementPermissionOrOwnerMiddleware(entitlementIDParam string, capability string, codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
			c.Abort()
			return
		}

		if hasAnyPermission(userID, codes...) {
			c.Next()
			return
		}

		eid, _ := strconv.Atoi(c.Param(entitlementIDParam))
		if eid <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
			c.Abort()
			return
		}

		var ent models.SSOAppEntitlement
		if err := storage.DB.First(&ent, eid).Error; err != nil || ent.AppID == 0 {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "资源不存在"})
			c.Abort()
			return
		}

		if isSSOAppOwner(userID, ent.AppID, capability) {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "无权限访问"})
		c.Abort()
	}
}

func hasAnyPermission(userID uint, codes ...string) bool {
	if len(codes) == 0 {
		return false
	}
	perms := GetUserPermissions(userID)
	if len(perms) == 0 {
		return false
	}
	set := map[string]struct{}{}
	for _, p := range perms {
		set[p] = struct{}{}
	}
	for _, code := range codes {
		if _, ok := set[code]; ok {
			return true
		}
	}
	return false
}

func isSSOAppOwner(userID uint, appID uint, capability string) bool {
	roleIDs := getUserRoleIDs(userID)
	caps := []string{"all"}
	if capability != "" {
		caps = append([]string{capability}, caps...)
	}

	var count int64
	storage.DB.Model(&models.SSOAppOwner{}).
		Where("app_id = ? AND capability IN ? AND ((subject_type = ? AND subject_id = ?) OR (subject_type = ? AND subject_id IN ?))",
			appID, caps, "user", userID, "role", roleIDs).
		Count(&count)
	return count > 0
}

func getUserRoleIDs(userID uint) []uint {
	var urs []models.UserRole
	storage.DB.Where("user_id = ?", userID).Find(&urs)
	if len(urs) == 0 {
		return []uint{0}
	}
	var ids []uint
	for _, ur := range urs {
		ids = append(ids, ur.RoleID)
	}
	return ids
}
