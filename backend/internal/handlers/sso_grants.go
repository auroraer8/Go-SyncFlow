package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

type ssoGrantDTO struct {
	SubjectType    string `json:"subjectType"`
	SubjectID      uint   `json:"subjectId"`
	Enabled        bool   `json:"enabled"`
	EntitlementIDs []uint `json:"entitlementIds"`
}

type ssoGrantPutRequest struct {
	Grants []ssoGrantDTO `json:"grants"`
}

func ListSSOAppGrants(c *gin.Context) {
	appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || appID == 0 {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	var grants []models.SSOAppGrant
	if err := storage.DB.Where("app_id = ?", appID).Order("id DESC").Find(&grants).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "查询失败")
		return
	}
	var grantIDs []uint
	for _, g := range grants {
		grantIDs = append(grantIDs, g.ID)
	}
	itemsByGrant := map[uint][]uint{}
	if len(grantIDs) > 0 {
		var items []models.SSOAppGrantItem
		_ = storage.DB.Where("grant_id IN ?", grantIDs).Find(&items).Error
		for _, it := range items {
			itemsByGrant[it.GrantID] = append(itemsByGrant[it.GrantID], it.EntitlementID)
		}
	}
	var list []gin.H
	for _, g := range grants {
		list = append(list, gin.H{
			"id":             g.ID,
			"subjectType":    g.SubjectType,
			"subjectId":      g.SubjectID,
			"enabled":        g.Enabled,
			"entitlementIds": itemsByGrant[g.ID],
		})
	}
	respondOK(c, gin.H{"list": list})
}

func PutSSOAppGrants(c *gin.Context) {
	appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || appID == 0 {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	var req ssoGrantPutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	tx := storage.DB.Begin()
	var old []models.SSOAppGrant
	_ = tx.Where("app_id = ?", appID).Find(&old).Error
	var oldIDs []uint
	for _, g := range old {
		oldIDs = append(oldIDs, g.ID)
	}
	if len(oldIDs) > 0 {
		_ = tx.Where("grant_id IN ?", oldIDs).Delete(&models.SSOAppGrantItem{}).Error
	}
	if err := tx.Where("app_id = ?", appID).Delete(&models.SSOAppGrant{}).Error; err != nil {
		tx.Rollback()
		respondError(c, http.StatusBadRequest, "更新失败")
		return
	}

	for _, g := range req.Grants {
		st := strings.TrimSpace(g.SubjectType)
		if st == "" || g.SubjectID == 0 {
			continue
		}
		row := models.SSOAppGrant{
			AppID:       uint(appID),
			SubjectType: st,
			SubjectID:   g.SubjectID,
			Enabled:     g.Enabled,
		}
		if err := tx.Create(&row).Error; err != nil {
			tx.Rollback()
			respondError(c, http.StatusBadRequest, "更新失败")
			return
		}
		for _, eid := range g.EntitlementIDs {
			if eid == 0 {
				continue
			}
			it := models.SSOAppGrantItem{GrantID: row.ID, EntitlementID: eid}
			if err := tx.Create(&it).Error; err != nil {
				tx.Rollback()
				respondError(c, http.StatusBadRequest, "更新失败")
				return
			}
		}
	}
	tx.Commit()
	respondOK(c, nil)
}
