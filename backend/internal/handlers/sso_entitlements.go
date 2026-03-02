package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

type ssoEntitlementUpsertRequest struct {
	Type        string `json:"type"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

func ListSSOAppEntitlements(c *gin.Context) {
	appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || appID == 0 {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	var list []models.SSOAppEntitlement
	if err := storage.DB.Where("app_id = ?", appID).Order("id DESC").Find(&list).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "查询失败")
		return
	}
	respondOK(c, gin.H{"list": list})
}

func CreateSSOAppEntitlement(c *gin.Context) {
	appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || appID == 0 {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	var req ssoEntitlementUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	req.Type = strings.TrimSpace(req.Type)
	req.Key = strings.TrimSpace(req.Key)
	req.Value = strings.TrimSpace(req.Value)
	if req.Type == "" || req.Value == "" {
		respondError(c, http.StatusBadRequest, "type/value 不能为空")
		return
	}
	row := models.SSOAppEntitlement{
		AppID:       uint(appID),
		Type:        req.Type,
		Key:         req.Key,
		Value:       req.Value,
		Description: strings.TrimSpace(req.Description),
		Enabled:     req.Enabled,
	}
	if err := storage.DB.Create(&row).Error; err != nil {
		respondError(c, http.StatusBadRequest, "创建失败")
		return
	}
	respondOK(c, row)
}

func UpdateSSOEntitlement(c *gin.Context) {
	entID, err := strconv.ParseUint(c.Param("eid"), 10, 64)
	if err != nil || entID == 0 {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	var req ssoEntitlementUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	var row models.SSOAppEntitlement
	if err := storage.DB.First(&row, entID).Error; err != nil {
		respondError(c, http.StatusNotFound, "不存在")
		return
	}
	req.Type = strings.TrimSpace(req.Type)
	req.Key = strings.TrimSpace(req.Key)
	req.Value = strings.TrimSpace(req.Value)
	if req.Type == "" || req.Value == "" {
		respondError(c, http.StatusBadRequest, "type/value 不能为空")
		return
	}
	row.Type = req.Type
	row.Key = req.Key
	row.Value = req.Value
	row.Description = strings.TrimSpace(req.Description)
	row.Enabled = req.Enabled
	if err := storage.DB.Save(&row).Error; err != nil {
		respondError(c, http.StatusBadRequest, "更新失败")
		return
	}
	respondOK(c, row)
}

func DeleteSSOEntitlement(c *gin.Context) {
	entID, err := strconv.ParseUint(c.Param("eid"), 10, 64)
	if err != nil || entID == 0 {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	storage.DB.Where("entitlement_id = ?", entID).Delete(&models.SSOAppGrantItem{})
	if err := storage.DB.Delete(&models.SSOAppEntitlement{}, entID).Error; err != nil {
		respondError(c, http.StatusBadRequest, "删除失败")
		return
	}
	respondOK(c, nil)
}
