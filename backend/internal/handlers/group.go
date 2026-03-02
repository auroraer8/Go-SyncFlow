package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

// ListUserGroups 获取所有分组
func ListUserGroups(c *gin.Context) {
	var groups []models.UserGroup
	if err := storage.DB.Order("\"order\" asc, id asc").Find(&groups).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "查询分组失败")
		return
	}

	// 统计每个分组的用户数
	type GroupCount struct {
		GroupID uint `gorm:"column:group_id"`
		Count   int  `gorm:"column:cnt"`
	}
	var counts []GroupCount
	storage.DB.Model(&models.User{}).
		Select("group_id, count(*) as cnt").
		Where("is_deleted = 0 AND group_id > 0").
		Group("group_id").
		Find(&counts)

	countMap := make(map[uint]int)
	for _, gc := range counts {
		countMap[gc.GroupID] = gc.Count
	}

	// 未分组用户数
	var ungroupedCount int64
	storage.DB.Model(&models.User{}).Where("is_deleted = 0 AND group_id = 0").Count(&ungroupedCount)

	type GroupResp struct {
		models.UserGroup
		MemberCount int `json:"memberCount"`
	}

	result := make([]GroupResp, 0, len(groups))
	for _, g := range groups {
		result = append(result, GroupResp{
			UserGroup:   g,
			MemberCount: countMap[g.ID],
		})
	}

	respondOK(c, gin.H{
		"groups":         result,
		"ungroupedCount": ungroupedCount,
	})
}

// CreateUserGroup 创建分组
func CreateUserGroup(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		ParentID uint   `json:"parentId"`
		Order    int    `json:"order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请输入分组名称")
		return
	}

	// 如果有父级，检查是否存在
	if req.ParentID > 0 {
		var parent models.UserGroup
		if storage.DB.First(&parent, req.ParentID).Error != nil {
			respondError(c, http.StatusBadRequest, "上级分组不存在")
			return
		}
	} else {
		// 没有选择上级分组时，自动挂到根分组下面
		var rootGroup models.UserGroup
		if storage.DB.Where("parent_id = 0 OR parent_id IS NULL").Order("id asc").First(&rootGroup).Error == nil {
			req.ParentID = rootGroup.ID
		}
	}

	group := models.UserGroup{
		Name:     req.Name,
		ParentID: req.ParentID,
		Order:    req.Order,
	}

	if err := storage.DB.Create(&group).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建分组失败")
		return
	}

	middleware.RecordOperationLog(c, "用户分组", "创建分组", group.Name, "")
	respondOK(c, group)
}

// UpdateUserGroup 更新分组
func UpdateUserGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	var group models.UserGroup
	if err := storage.DB.First(&group, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "分组不存在")
		return
	}

	var req struct {
		Name     string `json:"name"`
		ParentID *uint  `json:"parentId"`
		Order    *int   `json:"order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.ParentID != nil {
		// 不能将自己设为自己的子级
		if *req.ParentID == group.ID {
			respondError(c, http.StatusBadRequest, "不能将分组设为自身的子级")
			return
		}
		updates["parent_id"] = *req.ParentID
	}
	if req.Order != nil {
		updates["order"] = *req.Order
	}

	if len(updates) > 0 {
		storage.DB.Model(&group).Updates(updates)
	}

	middleware.RecordOperationLog(c, "用户分组", "更新分组", group.Name, "")
	respondOK(c, group)
}

// DeleteUserGroup 删除分组
func DeleteUserGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	var group models.UserGroup
	if err := storage.DB.First(&group, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "分组不存在")
		return
	}

	recursive := c.Query("recursive") == "true"

	var childCount int64
	storage.DB.Model(&models.UserGroup{}).Where("parent_id = ?", id).Count(&childCount)
	if childCount > 0 && !recursive {
		respondError(c, http.StatusBadRequest, "该分组下有子分组，请先删除子分组")
		return
	}

	if recursive {
		deleted := deleteGroupRecursive(uint(id))
		middleware.RecordOperationLog(c, "用户分组", "递归删除分组", group.Name, fmt.Sprintf("共删除 %d 个分组", deleted))
	} else {
		storage.DB.Model(&models.User{}).Where("group_id = ?", id).Update("group_id", 0)
		storage.DB.Delete(&group)
		middleware.RecordOperationLog(c, "用户分组", "删除分组", group.Name, "")
	}

	respondOK(c, nil)
}

// deleteGroupRecursive 递归删除分组及所有子分组，返回删除的分组总数
func deleteGroupRecursive(parentID uint) int {
	var children []models.UserGroup
	storage.DB.Where("parent_id = ?", parentID).Find(&children)

	count := 0
	for _, child := range children {
		count += deleteGroupRecursive(child.ID)
	}

	storage.DB.Model(&models.User{}).Where("group_id = ?", parentID).Update("group_id", 0)
	storage.DB.Where("id = ?", parentID).Delete(&models.UserGroup{})
	count++
	return count
}
