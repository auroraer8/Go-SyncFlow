package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/dingtalk"
	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
	syncer "go-syncflow/internal/sync"
)

// ========== 同步器 CRUD ==========

func ListSynchronizers(c *gin.Context) {
	var list []models.Synchronizer
	storage.DB.Preload("Connector").Order("created_at DESC").Find(&list)
	respondOK(c, list)
}

func GetSynchronizer(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var s models.Synchronizer
	if err := storage.DB.Preload("Connector").First(&s, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步器不存在")
		return
	}

	// 同时获取属性映射
	var mappings []models.SyncAttributeMapping
	storage.DB.Where("synchronizer_id = ?", id).Order("object_type, priority").Find(&mappings)

	respondOK(c, gin.H{
		"synchronizer": s,
		"mappings":     mappings,
	})
}

func CreateSynchronizer(c *gin.Context) {
	var req struct {
		Name               string   `json:"name" binding:"required"`
		ConnectorID        uint     `json:"connectorId" binding:"required"`
		Direction          string   `json:"direction"`
		SourceType         string   `json:"sourceType"`
		TargetContainer    string   `json:"targetContainer"`
		RoleGroupContainer string   `json:"roleGroupContainer"`
		EnableSchedule     bool     `json:"enableSchedule"`
		ScheduleTime       string   `json:"scheduleTime"`
		CronExpr           string   `json:"cronExpr"`
		EnableEvent        bool     `json:"enableEvent"`
		Events             []string `json:"events"`
		SyncUsers          bool     `json:"syncUsers"`
		SyncGroups         bool     `json:"syncGroups"`
		SyncRoles          bool     `json:"syncRoles"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// 检查连接器存在
	var conn models.Connector
	if err := storage.DB.First(&conn, req.ConnectorID).Error; err != nil {
		respondError(c, http.StatusBadRequest, "连接器不存在")
		return
	}

	if req.Direction == "" {
		req.Direction = "push"
	}
	if req.SourceType == "" {
		req.SourceType = "local"
	}

	eventsJSON := "[]"
	if len(req.Events) > 0 {
		b, _ := json.Marshal(req.Events)
		eventsJSON = string(b)
	}

	s := models.Synchronizer{
		Name:               req.Name,
		ConnectorID:        req.ConnectorID,
		Direction:          req.Direction,
		SourceType:         req.SourceType,
		TargetContainer:    req.TargetContainer,
		RoleGroupContainer: req.RoleGroupContainer,
		EnableSchedule:  req.EnableSchedule,
		ScheduleTime:    req.ScheduleTime,
		CronExpr:        req.CronExpr,
		EnableEvent:     req.EnableEvent,
		Events:          eventsJSON,
		SyncUsers:       req.SyncUsers,
		SyncGroups:      req.SyncGroups,
		SyncRoles:       req.SyncRoles,
		Status:          1,
	}

	if err := storage.DB.Create(&s).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	// 创建默认属性映射
	createDefaultMappings(s.ID, conn.Type)

	middleware.RecordOperationLog(c, "同步器管理", "创建同步器", strconv.FormatUint(uint64(s.ID), 10), s.Name)
	respondOK(c, s)
}

func UpdateSynchronizer(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var s models.Synchronizer
	if err := storage.DB.First(&s, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步器不存在")
		return
	}

	var req struct {
		Name               *string  `json:"name"`
		ConnectorID        *uint    `json:"connectorId"`
		TargetContainer    *string  `json:"targetContainer"`
		RoleGroupContainer *string  `json:"roleGroupContainer"`
		EnableSchedule     *bool    `json:"enableSchedule"`
		ScheduleTime       *string  `json:"scheduleTime"`
		CronExpr           *string  `json:"cronExpr"`
		EnableEvent        *bool    `json:"enableEvent"`
		Events             []string `json:"events"`
		SyncUsers          *bool    `json:"syncUsers"`
		SyncGroups         *bool    `json:"syncGroups"`
		SyncRoles          *bool    `json:"syncRoles"`
		PreventPwdChange   *bool    `json:"preventPwdChange"`
		Status             *int8    `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	updates := make(map[string]interface{})
	var selectFields []string
	if req.Name != nil {
		updates["name"] = *req.Name
		selectFields = append(selectFields, "name")
	}
	if req.ConnectorID != nil {
		updates["connector_id"] = *req.ConnectorID
		selectFields = append(selectFields, "connector_id")
	}
	if req.TargetContainer != nil {
		updates["target_container"] = *req.TargetContainer
		selectFields = append(selectFields, "target_container")
	}
	if req.RoleGroupContainer != nil {
		updates["role_group_container"] = *req.RoleGroupContainer
		selectFields = append(selectFields, "role_group_container")
	}
	if req.EnableSchedule != nil {
		updates["enable_schedule"] = *req.EnableSchedule
		selectFields = append(selectFields, "enable_schedule")
	}
	if req.ScheduleTime != nil {
		updates["schedule_time"] = *req.ScheduleTime
		selectFields = append(selectFields, "schedule_time")
	}
	if req.CronExpr != nil {
		updates["cron_expr"] = *req.CronExpr
		selectFields = append(selectFields, "cron_expr")
	}
	if req.EnableEvent != nil {
		updates["enable_event"] = *req.EnableEvent
		selectFields = append(selectFields, "enable_event")
	}
	if req.Events != nil {
		b, _ := json.Marshal(req.Events)
		updates["events"] = string(b)
		selectFields = append(selectFields, "events")
	}
	if req.SyncUsers != nil {
		updates["sync_users"] = *req.SyncUsers
		selectFields = append(selectFields, "sync_users")
	}
	if req.SyncGroups != nil {
		updates["sync_groups"] = *req.SyncGroups
		selectFields = append(selectFields, "sync_groups")
	}
	if req.SyncRoles != nil {
		updates["sync_roles"] = *req.SyncRoles
		selectFields = append(selectFields, "sync_roles")
	}
	if req.PreventPwdChange != nil {
		updates["prevent_pwd_change"] = *req.PreventPwdChange
		selectFields = append(selectFields, "prevent_pwd_change")
	}
	if req.Status != nil {
		updates["status"] = *req.Status
		selectFields = append(selectFields, "status")
	}

	if len(selectFields) > 0 {
		storage.DB.Model(&s).Select(selectFields).Updates(updates)
	}
	middleware.RecordOperationLog(c, "同步器管理", "更新同步器", strconv.FormatUint(id, 10), s.Name)
	respondOK(c, nil)
}

func DeleteSynchronizer(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var s models.Synchronizer
	if err := storage.DB.First(&s, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步器不存在")
		return
	}

	// 删除关联的属性映射
	storage.DB.Where("synchronizer_id = ?", id).Delete(&models.SyncAttributeMapping{})
	// 删除同步日志
	storage.DB.Where("synchronizer_id = ?", id).Delete(&models.SyncLog{})
	// 删除同步器
	storage.DB.Delete(&s)

	middleware.RecordOperationLog(c, "同步器管理", "删除同步器", strconv.FormatUint(id, 10), s.Name)
	respondOK(c, nil)
}

// TriggerSync 手动触发同步
func TriggerSync(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var s models.Synchronizer
	if err := storage.DB.First(&s, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步器不存在")
		return
	}

	// 异步执行
	go func() {
		syncer.ExecuteFullSync(s, "manual")
	}()

	middleware.RecordOperationLog(c, "同步器管理", "手动触发同步", strconv.FormatUint(id, 10), s.Name)
	respondOK(c, gin.H{"message": "同步已触发，请查看同步日志"})
}

// ========== 属性映射 CRUD ==========

func ListMappings(c *gin.Context) {
	syncID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var mappings []models.SyncAttributeMapping
	storage.DB.Where("synchronizer_id = ?", syncID).Order("object_type, priority").Find(&mappings)
	respondOK(c, mappings)
}

func CreateMapping(c *gin.Context) {
	syncID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req models.SyncAttributeMapping
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	req.SynchronizerID = uint(syncID)
	if req.MappingType == "" {
		req.MappingType = "mapping"
	}
	req.IsEnabled = true

	storage.DB.Create(&req)
	respondOK(c, req)
}

func UpdateMapping(c *gin.Context) {
	mid, _ := strconv.ParseUint(c.Param("mid"), 10, 32)

	var m models.SyncAttributeMapping
	if err := storage.DB.First(&m, mid).Error; err != nil {
		respondError(c, http.StatusNotFound, "映射不存在")
		return
	}

	var req map[string]interface{}
	c.ShouldBindJSON(&req)

	updates := make(map[string]interface{})
	var selectFields []string
	allowedFields := []string{"sourceAttribute", "targetAttribute", "mappingType", "transformRule", "priority", "isEnabled"}
	fieldMap := map[string]string{
		"sourceAttribute": "source_attribute",
		"targetAttribute": "target_attribute",
		"mappingType":     "mapping_type",
		"transformRule":   "transform_rule",
		"priority":        "priority",
		"isEnabled":       "is_enabled",
	}
	for _, f := range allowedFields {
		if v, ok := req[f]; ok {
			dbField := fieldMap[f]
			updates[dbField] = v
			selectFields = append(selectFields, dbField)
		}
	}

	if len(selectFields) > 0 {
		storage.DB.Model(&m).Select(selectFields).Updates(updates)
	}
	respondOK(c, nil)
}

func DeleteMapping(c *gin.Context) {
	mid, _ := strconv.ParseUint(c.Param("mid"), 10, 32)
	storage.DB.Delete(&models.SyncAttributeMapping{}, mid)
	respondOK(c, nil)
}

func BatchUpdateMappings(c *gin.Context) {
	syncID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		Mappings []models.SyncAttributeMapping `json:"mappings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 删除旧映射
	storage.DB.Where("synchronizer_id = ?", syncID).Delete(&models.SyncAttributeMapping{})

	// 创建新映射
	for i := range req.Mappings {
		req.Mappings[i].ID = 0
		req.Mappings[i].SynchronizerID = uint(syncID)
		if req.Mappings[i].MappingType == "" {
			req.Mappings[i].MappingType = "mapping"
		}
	}
	if len(req.Mappings) > 0 {
		storage.DB.Select("*").Create(&req.Mappings)
	}

	respondOK(c, gin.H{"count": len(req.Mappings)})
}

// ========== 同步日志 ==========

func GetSyncLogs(c *gin.Context) {
	syncID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	var total int64
	var logs []models.SyncLog

	query := storage.DB.Model(&models.SyncLog{}).Where("synchronizer_id = ?", syncID)
	query.Count(&total)
	query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&logs)

	respondOK(c, gin.H{
		"list":  logs,
		"total": total,
	})
}

// GetSyncEvents 获取所有支持的同步事件
func GetSyncEvents(c *gin.Context) {
	respondOK(c, models.AllSyncEvents)
}

// GetSourceFields 获取本地源字段定义
func GetSourceFields(c *gin.Context) {
	objectType := c.DefaultQuery("objectType", "user")

	type fieldDef struct {
		Key   string `json:"key"`
		Label string `json:"label"`
		Type  string `json:"type"`
	}

	switch objectType {
	case "user":
		respondOK(c, []fieldDef{
			{"id", "用户ID", "uint"},
			{"username", "用户名", "string"},
			{"password_raw", "密码(原文)", "string"},
			{"password_hash", "密码(bcrypt哈希)", "string"},
			{"samba_nt_password", "Samba NT密码", "string"},
			{"nickname", "姓名/昵称", "string"},
			{"phone", "手机号", "string"},
			{"email", "邮箱", "string"},
			{"avatar", "头像URL", "string"},
			{"status", "状态(1=启用,0=禁用)", "int"},
			{"source", "用户来源", "string"},
			{"group_id", "分组ID", "uint"},
			{"group_name", "分组名称", "string"},
			{"department_name", "部门名称", "string"},
			{"job_title", "职位", "string"},
			{"roles", "角色代码(逗号分隔)", "string"},
			{"role_names", "角色名称(逗号分隔)", "string"},
			{"dingtalk_uid", "钉钉UserID", "string"},
			{"created_at", "创建时间", "time"},
			{"updated_at", "更新时间", "time"},
			{"password_changed_at", "密码修改时间", "time"},
			{"last_login_at", "最后登录时间", "time"},
			{"last_login_ip", "最后登录IP", "string"},
			{"mfa_enabled", "MFA状态", "bool"},
		})
	case "group":
		respondOK(c, []fieldDef{
			{"id", "分组ID", "uint"},
			{"name", "分组名称", "string"},
			{"parent_id", "上级分组ID", "uint"},
			{"parent_name", "上级分组名称", "string"},
			{"full_path", "完整路径", "string"},
			{"order", "排序", "int"},
			{"member_count", "成员数量", "int"},
			{"created_at", "创建时间", "time"},
			{"updated_at", "更新时间", "time"},
		})
	case "role":
		respondOK(c, []fieldDef{
			{"id", "角色ID", "uint"},
			{"name", "角色名称", "string"},
			{"code", "角色代码", "string"},
			{"description", "描述", "string"},
			{"status", "状态", "int"},
			{"permissions", "权限代码(逗号分隔)", "string"},
			{"member_count", "成员数量", "int"},
			{"created_at", "创建时间", "time"},
			{"updated_at", "更新时间", "time"},
		})
	default:
		respondOK(c, []fieldDef{})
	}
}

// GetTargetFields 获取目标字段定义
func GetTargetFields(c *gin.Context) {
	connID, _ := strconv.ParseUint(c.Query("connectorId"), 10, 32)
	objectType := c.DefaultQuery("objectType", "user")

	var conn models.Connector
	if err := storage.DB.First(&conn, connID).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}

	type fieldDef struct {
		Key   string `json:"key"`
		Label string `json:"label"`
		Type  string `json:"type"`
	}

	if conn.Type == "ldap_ad" {
		switch objectType {
		case "user":
			respondOK(c, []fieldDef{
				{"sAMAccountName", "登录名(2000前)", "string"},
				{"userPrincipalName", "用户主体名(UPN)", "string"},
				{"cn", "通用名(CN)", "string"},
				{"givenName", "名", "string"},
				{"sn", "姓", "string"},
				{"displayName", "显示名称", "string"},
				{"initials", "缩写", "string"},
				{"description", "描述", "string"},
				{"mail", "邮箱", "string"},
				{"telephoneNumber", "办公电话", "string"},
				{"mobile", "手机号", "string"},
				{"homePhone", "家庭电话", "string"},
				{"pager", "传呼号", "string"},
				{"facsimileTelephoneNumber", "传真", "string"},
				{"ipPhone", "IP电话", "string"},
				{"wWWHomePage", "网页", "string"},
				{"streetAddress", "街道地址", "string"},
				{"postOfficeBox", "邮政信箱", "string"},
				{"l", "城市", "string"},
				{"st", "省/州", "string"},
				{"postalCode", "邮编", "string"},
				{"co", "国家/地区", "string"},
				{"c", "国家代码(2位)", "string"},
				{"countryCode", "国家数字代码", "int"},
				{"title", "职位", "string"},
				{"department", "部门", "string"},
				{"company", "公司", "string"},
				{"manager", "上级经理(DN)", "string"},
				{"physicalDeliveryOfficeName", "办公室", "string"},
				{"employeeID", "员工编号", "string"},
				{"employeeNumber", "员工号", "string"},
				{"employeeType", "员工类型", "string"},
				{"division", "分部", "string"},
				{"unicodePwd", "密码(需LDAPS)", "binary"},
				{"userAccountControl", "账户控制标志", "int"},
				{"accountExpires", "账户过期时间", "largeInt"},
				{"pwdLastSet", "密码最后修改", "largeInt"},
				{"profilePath", "配置文件路径", "string"},
				{"scriptPath", "登录脚本", "string"},
				{"homeDirectory", "主目录路径", "string"},
				{"homeDrive", "主驱动器", "string"},
				{"primaryGroupID", "主组ID", "int"},
				{"personalTitle", "个人头衔", "string"},
			})
		case "group":
			respondOK(c, []fieldDef{
				{"ou", "OU名称", "string"},
				{"cn", "通用名(CN)", "string"},
				{"description", "描述", "string"},
				{"displayName", "显示名称", "string"},
				{"managedBy", "管理者(DN)", "string"},
				{"l", "城市", "string"},
				{"st", "省/州", "string"},
				{"c", "国家代码", "string"},
				{"postalCode", "邮编", "string"},
				{"streetAddress", "街道地址", "string"},
			})
		case "role":
			respondOK(c, []fieldDef{
				{"cn", "组名(CN)", "string"},
				{"sAMAccountName", "组登录名", "string"},
				{"description", "描述", "string"},
				{"mail", "邮箱", "string"},
				{"managedBy", "管理者(DN)", "string"},
				{"groupType", "组类型", "int"},
				{"info", "备注", "string"},
			})
		}
	} else if conn.Type == "mysql" {
		// 动态获取目标表字段
		tableName := ""
		switch objectType {
		case "user":
			tableName = conn.UserTable
		case "group":
			tableName = conn.GroupTable
		case "role":
			tableName = conn.RoleTable
		}

		if tableName == "" {
			respondOK(c, []fieldDef{})
			return
		}

		columns, err := syncer.DiscoverMySQLColumns(conn, tableName)
		if err != nil {
			respondOK(c, []fieldDef{})
			return
		}

		fields := make([]fieldDef, 0, len(columns))
		for _, col := range columns {
			label := col["name"]
			if col["comment"] != "" {
				label = fmt.Sprintf("%s (%s)", col["name"], col["comment"])
			}
			fields = append(fields, fieldDef{
				Key:   col["name"],
				Label: label,
				Type:  col["type"],
			})
		}
		respondOK(c, fields)
	}
}

// ========== 创建默认映射 ==========

func createDefaultMappings(syncID uint, connType string) {
	var mappings []models.SyncAttributeMapping

	if connType == "ldap_ad" {
		mappings = []models.SyncAttributeMapping{
			{SynchronizerID: syncID, ObjectType: "group", SourceAttribute: "name", TargetAttribute: "ou", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "group", SourceAttribute: "name", TargetAttribute: "description", MappingType: "mapping", Priority: 2, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "role", SourceAttribute: "name", TargetAttribute: "cn", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "role", SourceAttribute: "code", TargetAttribute: "description", MappingType: "mapping", Priority: 2, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "username", TargetAttribute: "sAMAccountName", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "username", TargetAttribute: "userPrincipalName", MappingType: "transform", TransformRule: "append:@domain", Priority: 2, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "cn", MappingType: "mapping", Priority: 3, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "displayName", MappingType: "mapping", Priority: 4, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "email", TargetAttribute: "mail", MappingType: "mapping", Priority: 5, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "phone", TargetAttribute: "mobile", MappingType: "mapping", Priority: 6, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "password_raw", TargetAttribute: "unicodePwd", MappingType: "transform", TransformRule: "password_to_unicode", Priority: 7, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "status", TargetAttribute: "userAccountControl", MappingType: "transform", TransformRule: "status_to_uac", Priority: 8, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "description", MappingType: "mapping", Priority: 9, IsEnabled: true},
		}
	} else if connType == "mysql" {
		mappings = []models.SyncAttributeMapping{
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "username", TargetAttribute: "username", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "password_hash", TargetAttribute: "password", MappingType: "mapping", Priority: 2, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "display_name", MappingType: "mapping", Priority: 3, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "email", TargetAttribute: "email", MappingType: "mapping", Priority: 4, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "phone", TargetAttribute: "phone", MappingType: "mapping", Priority: 5, IsEnabled: true},
			{SynchronizerID: syncID, ObjectType: "user", SourceAttribute: "status", TargetAttribute: "status", MappingType: "mapping", Priority: 6, IsEnabled: true},
		}
	}

	if len(mappings) > 0 {
		storage.DB.Create(&mappings)
	}
}

// TriggerAllSync 触发所有同步任务（钉钉 + 所有同步器）
func TriggerAllSync(c *gin.Context) {
	triggered := []string{}

	// 1. 触发钉钉同步（如果已启用）
	client := dingtalk.GetClient()
	if client.IsEnabled() {
		go func() {
			if result, err := ExecuteDingTalkSync("manual"); err != nil {
				log.Printf("[全局同步] 钉钉同步失败: %v", err)
			} else {
				log.Printf("[全局同步] 钉钉同步完成: 部门=%d 新增=%d 更新=%d",
					result.DepartmentsSynced, result.UsersCreated, result.UsersUpdated)
			}
		}()
		triggered = append(triggered, "钉钉同步")
	}

	// 2. 触发所有启用的同步器
	var syncs []models.Synchronizer
	storage.DB.Where("status = ?", 1).Find(&syncs)
	for _, s := range syncs {
		syncCopy := s
		go func() {
			syncer.ExecuteFullSync(syncCopy, "manual")
		}()
		triggered = append(triggered, syncCopy.Name)
	}

	middleware.RecordOperationLog(c, "全局同步", "触发全部同步任务", fmt.Sprintf("共%d项", len(triggered)), strings.Join(triggered, ", "))
	respondOK(c, gin.H{
		"message":   fmt.Sprintf("已触发 %d 个同步任务", len(triggered)),
		"triggered": triggered,
	})
}
