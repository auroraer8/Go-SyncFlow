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

func ListRoles(c *gin.Context) {
	var roles []models.Role
	storage.DB.Order("id asc").Find(&roles)
	respondOK(c, roles)
}

type createRoleRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Description string `json:"description"`
}

func CreateRole(c *gin.Context) {
	var req createRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	var count int64
	storage.DB.Model(&models.Role{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		respondError(c, http.StatusBadRequest, "角色编码已存在")
		return
	}

	role := models.Role{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Status:      1,
	}

	if err := storage.DB.Create(&role).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	middleware.RecordOperationLog(c, "角色管理", "新增角色", req.Name, "")
	
	// 同步角色到下游
	go sync.SyncRoleToDownstream(role, "role_create")
	
	// 广播角色创建事件
	BroadcastRoleChange("created", role.ID, role.Name)
	
	respondOK(c, role)
}

func GetRole(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var role models.Role
	if err := storage.DB.Preload("Permissions").First(&role, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "角色不存在")
		return
	}

	respondOK(c, role)
}

func UpdateRole(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		SidebarMode string `json:"sidebarMode"`
		LandingPage string `json:"landingPage"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	var role models.Role
	if err := storage.DB.First(&role, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "角色不存在")
		return
	}

	updates := map[string]interface{}{
		"name":        req.Name,
		"description": req.Description,
	}
	// 布局配置（仅当传了值时更新）
	if req.SidebarMode != "" {
		updates["sidebar_mode"] = req.SidebarMode
	}
	updates["landing_page"] = req.LandingPage

	storage.DB.Model(&role).Updates(updates)
	
	// 重新加载角色数据用于下游同步
	storage.DB.First(&role, id)

	middleware.RecordOperationLog(c, "角色管理", "编辑角色", req.Name, "")
	
	// 同步角色到下游
	go sync.SyncRoleToDownstream(role, "role_update")
	
	// 广播角色更新事件
	BroadcastRoleChange("updated", uint(id), req.Name)
	
	respondOK(c, nil)
}

func DeleteRole(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var role models.Role
	if err := storage.DB.First(&role, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "角色不存在")
		return
	}

	if role.Code == "super_admin" || role.Code == "user" {
		respondError(c, http.StatusBadRequest, "内置角色不可删除")
		return
	}

	// 检查是否有用户使用
	var count int64
	storage.DB.Model(&models.UserRole{}).Where("role_id = ?", id).Count(&count)
	if count > 0 {
		respondError(c, http.StatusBadRequest, "该角色正在被使用，无法删除")
		return
	}

	// 先同步删除到下游（需要在本地删除前执行）
	sync.SyncRoleToDownstream(role, "role_delete")
	
	roleName := role.Name
	storage.DB.Delete(&models.Role{}, id)
	storage.DB.Where("role_id = ?", id).Delete(&models.RolePermission{})

	middleware.RecordOperationLog(c, "角色管理", "删除角色", strconv.FormatUint(id, 10), "")
	
	// 广播角色删除事件
	BroadcastRoleChange("deleted", uint(id), roleName)
	
	respondOK(c, nil)
}

func GetRolePermissions(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var rolePerms []models.RolePermission
	storage.DB.Where("role_id = ?", id).Find(&rolePerms)

	var permIDs []uint
	for _, rp := range rolePerms {
		permIDs = append(permIDs, rp.PermissionID)
	}

	respondOK(c, permIDs)
}

type updatePermissionsRequest struct {
	PermissionIDs []uint `json:"permissionIds"`
}

func UpdateRolePermissions(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req updatePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 删除旧权限
	storage.DB.Where("role_id = ?", id).Delete(&models.RolePermission{})

	// 添加新权限
	for _, permID := range req.PermissionIDs {
		storage.DB.Create(&models.RolePermission{RoleID: uint(id), PermissionID: permID})
	}

	middleware.RecordOperationLog(c, "角色管理", "分配权限", strconv.FormatUint(id, 10), "")
	respondOK(c, nil)
}

// ========== 角色自动分配规则 ==========

func GetRoleAutoAssignRules(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var rules []models.RoleAutoAssignRule
	storage.DB.Where("role_id = ?", id).Order("id asc").Find(&rules)

	respondOK(c, rules)
}

func UpdateRoleAutoAssignRules(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		Rules []struct {
			RuleType  string `json:"ruleType"`
			RuleValue string `json:"ruleValue"`
		} `json:"rules"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 删除旧规则
	storage.DB.Where("role_id = ?", id).Delete(&models.RoleAutoAssignRule{})

	// 添加新规则
	for _, r := range req.Rules {
		if r.RuleType != "" && r.RuleValue != "" {
			storage.DB.Create(&models.RoleAutoAssignRule{
				RoleID:    uint(id),
				RuleType:  r.RuleType,
				RuleValue: r.RuleValue,
			})
		}
	}

	middleware.RecordOperationLog(c, "角色管理", "更新自动分配规则", strconv.FormatUint(id, 10), "")
	respondOK(c, nil)
}

// ApplyAutoAssignRules 立即执行角色自动分配规则
func ApplyAutoAssignRules(c *gin.Context) {
	result := applyAutoAssignRulesWithResult()

	middleware.RecordOperationLog(c, "角色管理", "立即执行自动分配规则",
		fmt.Sprintf("匹配:%d 新分配:%d 跳过:%d", result.TotalMatched, result.TotalAssigned, result.TotalSkipped), "")
	respondOK(c, result)
}

func GetPermissionTree(c *gin.Context) {
	var permissions []models.Permission
	storage.DB.Order("sort asc").Find(&permissions)

	tree := buildPermissionTree(permissions, 0)
	respondOK(c, tree)
}

type permissionNode struct {
	models.Permission
	Children []permissionNode `json:"children,omitempty"`
}

func buildPermissionTree(permissions []models.Permission, parentID uint) []permissionNode {
	var nodes []permissionNode
	for _, p := range permissions {
		if p.ParentID == parentID {
			node := permissionNode{Permission: p}
			node.Children = buildPermissionTree(permissions, p.ID)
			nodes = append(nodes, node)
		}
	}
	return nodes
}
