package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

func ListSSOAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		respondError(c, http.StatusUnauthorized, "未登录")
		return
	}

	query := storage.DB.Model(&models.SSOAuditLog{})

	if !middleware.CheckPermission(c, "sso:logs") {
		appIDStr := strings.TrimSpace(c.Query("appId"))
		if appIDStr == "" {
			respondError(c, http.StatusForbidden, "无权限访问")
			return
		}
		appID, err := strconv.Atoi(appIDStr)
		if err != nil || appID <= 0 {
			respondError(c, http.StatusBadRequest, "参数错误")
			return
		}
		roleIDs := getUserRoleIDsForSSOLog(userID)
		var count int64
		storage.DB.Model(&models.SSOAppOwner{}).
			Where("app_id = ? AND ((subject_type = ? AND subject_id = ?) OR (subject_type = ? AND subject_id IN ?))",
				appID, "user", userID, "role", roleIDs).
			Count(&count)
		if count == 0 {
			respondError(c, http.StatusForbidden, "无权限访问")
			return
		}
	}

	if appID := strings.TrimSpace(c.Query("appId")); appID != "" {
		query = query.Where("app_id = ?", appID)
	}
	if protocol := strings.TrimSpace(c.Query("protocol")); protocol != "" {
		query = query.Where("protocol = ?", protocol)
	}
	if userID := strings.TrimSpace(c.Query("userId")); userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR action LIKE ? OR message LIKE ?", like, like, like)
	}
	if from := strings.TrimSpace(c.Query("from")); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			query = query.Where("created_at <= ?", t)
		}
	}

	var total int64
	query.Count(&total)

	var list []models.SSOAuditLog
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "查询失败")
		return
	}
	respondList(c, list, total)
}

func getUserRoleIDsForSSOLog(userID uint) []uint {
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

func GetSSOIntegrationOverview(c *gin.Context) {
	issuer := issuerURL(c)
	_, _, oidcKid := getOIDCSigningKey()
	_, certDER := getSAMLKeypair()

	respondOK(c, gin.H{
		"issuer": issuer,
		"oidc": gin.H{
			"discovery": issuer + "/.well-known/openid-configuration",
			"jwks":      issuer + "/oidc/jwks",
			"authorize": issuer + "/oauth/authorize",
			"token":     issuer + "/oauth/token",
			"userinfo":  issuer + "/oidc/userinfo",
			"kid":       oidcKid,
		},
		"cas": gin.H{
			"login":           issuer + "/cas/login",
			"serviceValidate": issuer + "/cas/serviceValidate",
			"logout":          issuer + "/cas/logout",
		},
		"saml": gin.H{
			"metadataPattern": issuer + "/saml/{appCode}/metadata",
			"ssoPattern":      issuer + "/saml/{appCode}/sso",
			"certBytes":       len(certDER),
		},
	})
}
