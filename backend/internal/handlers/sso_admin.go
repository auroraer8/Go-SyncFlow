package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

type ssoAppUpsertRequest struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Enabled     *bool  `json:"enabled"`
	Description string `json:"description"`
	AccessMode  string `json:"accessMode"`
	ClaimPolicy string `json:"claimPolicy"`
}

type ssoAppProtocolDTO struct {
	Protocol string `json:"protocol"`
	Enabled  bool   `json:"enabled"`
	Config   string `json:"config"`
}

type ssoAppProtocolsPutRequest struct {
	Protocols []ssoAppProtocolDTO `json:"protocols"`
}

type ssoAppMappingsPutRequest struct {
	Mappings []models.SSOAppMapping `json:"mappings"`
}

func ListSSOApps(c *gin.Context) {
	var apps []models.SSOApp
	var total int64

	query := storage.DB.Model(&models.SSOApp{})
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ?", like, like)
	}
	if enabled := strings.TrimSpace(c.Query("enabled")); enabled != "" {
		if enabled == "true" {
			query = query.Where("enabled = ?", true)
		} else if enabled == "false" {
			query = query.Where("enabled = ?", false)
		}
	}

	query.Count(&total)
	if err := query.Order("id DESC").Preload("Protocols").Find(&apps).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "查询失败")
		return
	}
	respondList(c, apps, total)
}

func CreateSSOApp(c *gin.Context) {
	var req ssoAppUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)
	if req.Name == "" || req.Code == "" {
		respondError(c, http.StatusBadRequest, "name/code 不能为空")
		return
	}

	app := models.SSOApp{
		Name:        req.Name,
		Code:        req.Code,
		Enabled:     true,
		Description: strings.TrimSpace(req.Description),
		AccessMode:  strings.TrimSpace(req.AccessMode),
		ClaimPolicy: req.ClaimPolicy,
	}
	if app.AccessMode == "" {
		app.AccessMode = "all"
	}
	if req.Enabled != nil {
		app.Enabled = *req.Enabled
	}

	if err := storage.DB.Create(&app).Error; err != nil {
		respondError(c, http.StatusBadRequest, "创建失败（code 可能重复）")
		return
	}
	storage.DB.Preload("Protocols").First(&app, app.ID)
	respondOK(c, app)
}

func GetSSOApp(c *gin.Context) {
	id := c.Param("id")
	var app models.SSOApp
	if err := storage.DB.Preload("Protocols").First(&app, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "应用不存在")
		return
	}
	respondOK(c, app)
}

func UpdateSSOApp(c *gin.Context) {
	id := c.Param("id")
	var app models.SSOApp
	if err := storage.DB.First(&app, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "应用不存在")
		return
	}

	var req ssoAppUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	if v := strings.TrimSpace(req.Name); v != "" {
		app.Name = v
	}
	if v := strings.TrimSpace(req.Description); v != "" || req.Description == "" {
		app.Description = strings.TrimSpace(req.Description)
	}
	if v := strings.TrimSpace(req.AccessMode); v != "" {
		app.AccessMode = v
	}
	if req.Enabled != nil {
		app.Enabled = *req.Enabled
	}
	if req.ClaimPolicy != "" || req.ClaimPolicy == "" {
		app.ClaimPolicy = req.ClaimPolicy
	}

	if err := storage.DB.Save(&app).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "更新失败")
		return
	}
	storage.DB.Preload("Protocols").First(&app, app.ID)
	respondOK(c, app)
}

func DeleteSSOApp(c *gin.Context) {
	id := c.Param("id")
	if err := storage.DB.Delete(&models.SSOApp{}, id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "删除失败")
		return
	}
	respondOK(c, gin.H{"deleted": true})
}

func GetSSOAppProtocols(c *gin.Context) {
	appID := c.Param("id")
	var protocols []models.SSOAppProtocol
	if err := storage.DB.Where("app_id = ?", appID).Order("id ASC").Find(&protocols).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "查询失败")
		return
	}
	respondOK(c, gin.H{"protocols": protocols})
}

func PutSSOAppProtocols(c *gin.Context) {
	appID := c.Param("id")
	var app models.SSOApp
	if err := storage.DB.First(&app, appID).Error; err != nil {
		respondError(c, http.StatusNotFound, "应用不存在")
		return
	}

	var req ssoAppProtocolsPutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	tx := storage.DB.Begin()
	if err := tx.Where("app_id = ?", app.ID).Delete(&models.SSOAppProtocol{}).Error; err != nil {
		tx.Rollback()
		respondError(c, http.StatusInternalServerError, "更新失败")
		return
	}

	for _, p := range req.Protocols {
		proto := strings.TrimSpace(p.Protocol)
		if proto == "" {
			continue
		}
		if !p.Enabled {
			continue
		}
		cfg := p.Config
		if proto == "oidc" && strings.TrimSpace(cfg) != "" {
			var m map[string]any
			if err := json.Unmarshal([]byte(cfg), &m); err == nil {
				if v, ok := m["clientSecret"].(string); ok && strings.TrimSpace(v) != "" {
					m["clientSecretHash"] = sha256Hex(strings.TrimSpace(v))
					delete(m, "clientSecret")
				}
				if b, err := json.Marshal(m); err == nil {
					cfg = string(b)
				}
			}
		}
		row := models.SSOAppProtocol{
			AppID:    app.ID,
			Protocol: proto,
			Enabled:  true,
			Config:   cfg,
		}
		if err := tx.Create(&row).Error; err != nil {
			tx.Rollback()
			respondError(c, http.StatusBadRequest, "更新失败")
			return
		}
	}
	if err := tx.Commit().Error; err != nil {
		respondError(c, http.StatusInternalServerError, "更新失败")
		return
	}
	GetSSOAppProtocols(c)
}

func GetSSOAppMappings(c *gin.Context) {
	appID := c.Param("id")
	var mappings []models.SSOAppMapping
	if err := storage.DB.Where("app_id = ?", appID).Order("priority DESC, id ASC").Find(&mappings).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "查询失败")
		return
	}
	respondOK(c, gin.H{"mappings": mappings})
}

func PutSSOAppMappings(c *gin.Context) {
	appID := c.Param("id")
	var app models.SSOApp
	if err := storage.DB.First(&app, appID).Error; err != nil {
		respondError(c, http.StatusNotFound, "应用不存在")
		return
	}

	var req ssoAppMappingsPutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	tx := storage.DB.Begin()
	if err := tx.Where("app_id = ?", app.ID).Delete(&models.SSOAppMapping{}).Error; err != nil {
		tx.Rollback()
		respondError(c, http.StatusInternalServerError, "更新失败")
		return
	}
	for _, m := range req.Mappings {
		row := models.SSOAppMapping{
			AppID:    app.ID,
			Type:     strings.TrimSpace(m.Type),
			Source:   strings.TrimSpace(m.Source),
			Target:   strings.TrimSpace(m.Target),
			Remark:   strings.TrimSpace(m.Remark),
			Priority: m.Priority,
		}
		if row.Type == "" || row.Source == "" || row.Target == "" {
			continue
		}
		if err := tx.Create(&row).Error; err != nil {
			tx.Rollback()
			respondError(c, http.StatusBadRequest, "更新失败")
			return
		}
	}
	if err := tx.Commit().Error; err != nil {
		respondError(c, http.StatusInternalServerError, "更新失败")
		return
	}
	GetSSOAppMappings(c)
}
