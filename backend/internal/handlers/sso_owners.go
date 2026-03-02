package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

type putSSOAppOwnersReq struct {
	Owners []struct {
		SubjectType string `json:"subjectType"`
		SubjectID   uint   `json:"subjectId"`
		Capability  string `json:"capability"`
	} `json:"owners"`
}

func ListSSOAppOwners(c *gin.Context) {
	appID, _ := strconv.Atoi(c.Param("id"))
	if appID <= 0 {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	var owners []models.SSOAppOwner
	if err := storage.DB.Where("app_id = ?", appID).Order("id ASC").Find(&owners).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "查询失败")
		return
	}
	respondOK(c, gin.H{"list": owners})
}

func PutSSOAppOwners(c *gin.Context) {
	appID, _ := strconv.Atoi(c.Param("id"))
	if appID <= 0 {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	var req putSSOAppOwnersReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	now := time.Now()
	var rows []models.SSOAppOwner
	for _, o := range req.Owners {
		if o.SubjectType != "user" && o.SubjectType != "role" {
			continue
		}
		cap := o.Capability
		if cap == "" {
			cap = "all"
		}
		rows = append(rows, models.SSOAppOwner{
			AppID:       uint(appID),
			SubjectType: o.SubjectType,
			SubjectID:   o.SubjectID,
			Capability:  cap,
			CreatedAt:   now,
		})
	}

	tx := storage.DB.Begin()
	if err := tx.Where("app_id = ?", appID).Delete(&models.SSOAppOwner{}).Error; err != nil {
		tx.Rollback()
		respondError(c, http.StatusInternalServerError, "保存失败")
		return
	}
	if len(rows) > 0 {
		if err := tx.Create(&rows).Error; err != nil {
			tx.Rollback()
			respondError(c, http.StatusInternalServerError, "保存失败")
			return
		}
	}
	tx.Commit()

	respondOK(c, gin.H{"count": len(rows)})
}

func ListMySSOApps(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		respondError(c, http.StatusUnauthorized, "未登录")
		return
	}

	roleIDs := getUserRoleIDs(userID)
	q := storage.DB.Model(&models.SSOApp{}).Distinct("sso_apps.*").
		Joins("JOIN sso_app_owners o ON o.app_id = sso_apps.id").
		Where("(o.subject_type = ? AND o.subject_id = ?) OR (o.subject_type = ? AND o.subject_id IN ?)", "user", userID, "role", roleIDs)

	var list []models.SSOApp
	if err := q.Order("id DESC").Find(&list).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "查询失败")
		return
	}

	for i := range list {
		var protocols []models.SSOAppProtocol
		storage.DB.Where("app_id = ?", list[i].ID).Find(&protocols)
		list[i].Protocols = protocols
	}

	respondOK(c, gin.H{"list": list})
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
