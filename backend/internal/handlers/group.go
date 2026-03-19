package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
	"go-syncflow/internal/sync"
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

	// 异步同步到下游
	go sync.SyncGroupToDownstream(group, "group_create", "", 0)
	
	// 广播群组创建事件
	BroadcastGroupChange("created", group.ID, group.Name)

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

	// 记录旧值，用于同步到下游
	oldName := group.Name
	oldParentID := group.ParentID

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
	needSync := false
	if req.Name != "" && req.Name != group.Name {
		updates["name"] = req.Name
		needSync = true
	}
	if req.ParentID != nil {
		// 不能将自己设为自己的子级
		if *req.ParentID == group.ID {
			respondError(c, http.StatusBadRequest, "不能将分组设为自身的子级")
			return
		}
		if *req.ParentID != group.ParentID {
			updates["parent_id"] = *req.ParentID
			needSync = true
		}
	}
	if req.Order != nil {
		updates["order"] = *req.Order
	}

	if len(updates) > 0 {
		storage.DB.Model(&group).Updates(updates)
		// 重新加载更新后的数据
		storage.DB.First(&group, id)
	}

	middleware.RecordOperationLog(c, "用户分组", "更新分组", group.Name, "")

	// 如果名称或父级变化，同步到下游
	if needSync {
		go sync.SyncGroupToDownstream(group, "group_update", oldName, oldParentID)
	}
	
	// 广播群组更新事件
	BroadcastGroupChange("updated", group.ID, group.Name)

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

	// 收集要删除的群组（用于同步到下游）
	var groupsToDelete []models.UserGroup
	if recursive {
		groupsToDelete = collectGroupsRecursive(uint(id))
	} else {
		groupsToDelete = []models.UserGroup{group}
	}

	// 收集受影响的用户（用于触发下游同步）
	var affectedUserIDs []uint
	for _, g := range groupsToDelete {
		var userIDs []uint
		storage.DB.Model(&models.User{}).Where("group_id = ?", g.ID).Pluck("id", &userIDs)
		affectedUserIDs = append(affectedUserIDs, userIDs...)
	}

	if recursive {
		deleted := deleteGroupRecursiveWithParentMove(uint(id), group.ParentID)
		middleware.RecordOperationLog(c, "用户分组", "递归删除分组", group.Name, fmt.Sprintf("共删除 %d 个分组", deleted))
	} else {
		// 将该分组下的用户移动到父级分组（而不是设为0）
		storage.DB.Model(&models.User{}).Where("group_id = ?", id).Update("group_id", group.ParentID)
		storage.DB.Delete(&group)
		middleware.RecordOperationLog(c, "用户分组", "删除分组", group.Name, "")
	}

	// 异步同步删除到下游（从叶子节点向上删除）+ 触发用户更新同步
	go func() {
		// 1. 先触发受影响用户的下游同步（因为他们的 GroupID 变了）
		for _, userID := range affectedUserIDs {
			sync.DispatchSyncEvent(models.SyncEventUserUpdate, userID, "")
		}
		// 2. 反向遍历删除 OU（先删子级再删父级）
		for i := len(groupsToDelete) - 1; i >= 0; i-- {
			sync.SyncGroupToDownstream(groupsToDelete[i], "group_delete", "", 0)
		}
	}()
	
	// 广播群组删除事件
	for _, g := range groupsToDelete {
		BroadcastGroupChange("deleted", g.ID, g.Name)
	}
	// 广播用户变更事件
	for _, userID := range affectedUserIDs {
		BroadcastUserChange("updated", userID, "")
	}

	respondOK(c, nil)
}

// collectGroupsRecursive 递归收集所有要删除的群组（按层级顺序，父级在前）
func collectGroupsRecursive(parentID uint) []models.UserGroup {
	var result []models.UserGroup
	var group models.UserGroup
	if storage.DB.First(&group, parentID).Error == nil {
		result = append(result, group)
	}

	var children []models.UserGroup
	storage.DB.Where("parent_id = ?", parentID).Find(&children)
	for _, child := range children {
		result = append(result, collectGroupsRecursive(child.ID)...)
	}
	return result
}

// deleteGroupRecursive 递归删除分组及所有子分组，返回删除的分组总数
// 注意：此函数已废弃，请使用 deleteGroupRecursiveWithParentMove
func deleteGroupRecursive(parentID uint) int {
	return deleteGroupRecursiveWithParentMove(parentID, 0)
}

// deleteGroupRecursiveWithParentMove 递归删除分组，并将用户移动到指定的父级分组
func deleteGroupRecursiveWithParentMove(groupID uint, moveToParentID uint) int {
	var group models.UserGroup
	if storage.DB.First(&group, groupID).Error != nil {
		return 0
	}

	var children []models.UserGroup
	storage.DB.Where("parent_id = ?", groupID).Find(&children)

	count := 0
	for _, child := range children {
		// 子分组的用户移动到当前分组的父级
		count += deleteGroupRecursiveWithParentMove(child.ID, moveToParentID)
	}

	// 将该分组下的用户移动到父级分组
	storage.DB.Model(&models.User{}).Where("group_id = ?", groupID).Update("group_id", moveToParentID)
	storage.DB.Where("id = ?", groupID).Delete(&models.UserGroup{})
	count++
	return count
}
