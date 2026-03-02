package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

type ssoTemplateDTO struct {
	Protocol string `json:"protocol"`
	Enabled  bool   `json:"enabled"`
	Template string `json:"template"`
}

type ssoTemplatePutRequest struct {
	Templates []ssoTemplateDTO `json:"templates"`
}

func ListSSOAppTemplates(c *gin.Context) {
	appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || appID == 0 {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	var list []models.SSOAppTemplate
	if err := storage.DB.Where("app_id = ?", appID).Order("id DESC").Find(&list).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "查询失败")
		return
	}
	respondOK(c, gin.H{"list": list})
}

func PutSSOAppTemplates(c *gin.Context) {
	appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || appID == 0 {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	var req ssoTemplatePutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	tx := storage.DB.Begin()
	if err := tx.Where("app_id = ?", appID).Delete(&models.SSOAppTemplate{}).Error; err != nil {
		tx.Rollback()
		respondError(c, http.StatusBadRequest, "更新失败")
		return
	}
	for _, t := range req.Templates {
		proto := strings.TrimSpace(t.Protocol)
		if proto == "" {
			continue
		}
		row := models.SSOAppTemplate{
			AppID:    uint(appID),
			Protocol: proto,
			Enabled:  t.Enabled,
			Template: t.Template,
		}
		if err := tx.Create(&row).Error; err != nil {
			tx.Rollback()
			respondError(c, http.StatusBadRequest, "更新失败")
			return
		}
	}
	tx.Commit()
	respondOK(c, nil)
}
