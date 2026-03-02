package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	gosync "sync"
	"time"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
	syncer "go-syncflow/internal/sync"
)

// ========== 下游连接器 CRUD ==========

func ListDownstreamConnectors(c *gin.Context) {
	var list []models.Connector
	storage.DB.Where("direction IN ?", []string{"downstream", "both"}).Order("created_at DESC").Find(&list)
	respondOK(c, list)
}

func CreateDownstreamConnector(c *gin.Context) {
	// 使用自定义 struct 接收（因为 models.Connector 的密码字段是 json:"-"，直接 Bind 会丢失密码）
	var req struct {
		Name         string `json:"name" binding:"required"`
		Type         string `json:"type" binding:"required"`
		Host         string `json:"host"`
		Port         int    `json:"port"`
		UseTLS       bool   `json:"useTls"`
		BaseDN       string `json:"baseDn"`
		BindDN       string `json:"bindDn"`
		BindPassword string `json:"bindPassword"`
		UPNSuffix    string `json:"upnSuffix"`
		DBType       string `json:"dbType"`
		Database     string `json:"database"`
		DBUser       string `json:"dbUser"`
		DBPassword   string `json:"dbPassword"`
		Charset      string `json:"charset"`
		ServiceName  string `json:"serviceName"`
		UserTable    string `json:"userTable"`
		GroupTable   string `json:"groupTable"`
		RoleTable    string `json:"roleTable"`
		PwdFormat    string `json:"pwdFormat"`
		Timeout      int    `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	conn := models.Connector{
		Name: req.Name, Type: req.Type, Direction: "downstream", Status: 1,
		Host: req.Host, Port: req.Port, UseTLS: req.UseTLS,
		BaseDN: req.BaseDN, BindDN: req.BindDN, BindPassword: req.BindPassword,
		UPNSuffix: req.UPNSuffix,
		DBType: req.DBType, Database: req.Database, DBUser: req.DBUser, DBPassword: req.DBPassword,
		Charset: req.Charset, ServiceName: req.ServiceName,
		UserTable: req.UserTable, GroupTable: req.GroupTable, RoleTable: req.RoleTable,
		PwdFormat: req.PwdFormat, Timeout: req.Timeout,
	}
	if conn.Timeout == 0 {
		conn.Timeout = 5
	}

	if err := storage.DB.Create(&conn).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	middleware.RecordOperationLog(c, "下游连接器", "创建", conn.Name+" ("+conn.Type+")", "")
	respondOK(c, conn)
}

func GetDownstreamConnector(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}
	respondOK(c, conn)
}

func UpdateDownstreamConnector(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	delete(req, "type")
	delete(req, "id")
	delete(req, "createdAt")

	// 密码空字符串保留旧值
	for _, pwdField := range []string{"bindPassword", "dbPassword"} {
		if v, ok := req[pwdField]; ok {
			if s, isStr := v.(string); isStr && s == "" {
				delete(req, pwdField)
			}
		}
	}

	fieldMap := map[string]string{
		"name": "name", "status": "status", "timeout": "timeout",
		"host": "host", "port": "port", "backupHost": "backup_host", "backupPort": "backup_port",
		"useTls": "use_tls", "baseDn": "base_dn", "bindDn": "bind_dn",
		"bindPassword": "bind_password", "upnSuffix": "upn_suffix", "userFilter": "user_filter",
		"dbType": "db_type", "database": "database", "dbUser": "db_user",
		"dbPassword": "db_password", "charset": "charset", "serviceName": "service_name",
		"userTable": "user_table", "groupTable": "group_table", "roleTable": "role_table",
		"pwdFormat": "pwd_format", "config": "config",
	}
	updates := make(map[string]interface{})
	var selectFields []string
	for k, v := range req {
		if dbField, ok := fieldMap[k]; ok {
			updates[dbField] = v
			selectFields = append(selectFields, dbField)
		}
	}

	if len(updates) > 0 {
		// 使用 Select 明确指定要更新的字段，确保布尔值 false 和零值也能被正确更新
		storage.DB.Model(&conn).Select(selectFields).Updates(updates)
	}

	middleware.RecordOperationLog(c, "下游连接器", "更新", conn.Name, "")
	respondOK(c, nil)
}

func DeleteDownstreamConnector(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}

	// 删除关联同步规则和日志
	storage.DB.Where("connector_id = ?", id).Delete(&models.SyncRule{})
	storage.DB.Delete(&conn)

	middleware.RecordOperationLog(c, "下游连接器", "删除", conn.Name, "")
	respondOK(c, nil)
}

func TestDownstreamConnector(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}

	TestUpstreamConnector(c) // 复用测试逻辑
}

// ========== 下游同步规则 CRUD ==========

func ListDownstreamRules(c *gin.Context) {
	var list []models.SyncRule
	storage.DB.Preload("Connector").Where("direction = ?", "downstream").Order("created_at DESC").Find(&list)
	respondOK(c, list)
}

func CreateDownstreamRule(c *gin.Context) {
	var req struct {
		Name               string   `json:"name" binding:"required"`
		ConnectorID        uint     `json:"connectorId" binding:"required"`
		TargetContainer    string   `json:"targetContainer"`
		RoleGroupContainer string   `json:"roleGroupContainer"`
		EnableSchedule     bool     `json:"enableSchedule"`
		ScheduleType       string   `json:"scheduleType"`
		ScheduleTime       string   `json:"scheduleTime"`
		ScheduleTimes      []string `json:"scheduleTimes"`
		ScheduleInterval   int      `json:"scheduleInterval"`
		CronExpr           string   `json:"cronExpr"`
		EnableEvent        bool     `json:"enableEvent"`
		Events             []string `json:"events"`
		SyncUsers          bool     `json:"syncUsers"`
		SyncGroups         bool     `json:"syncGroups"`
		SyncRoles          bool     `json:"syncRoles"`
		PreventPwdChange   bool     `json:"preventPwdChange"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	eventsJSON := "[]"
	if len(req.Events) > 0 {
		b, _ := json.Marshal(req.Events)
		eventsJSON = string(b)
	}

	// 处理定时时间
	scheduleTime := req.ScheduleTime
	if len(req.ScheduleTimes) > 0 {
		b, _ := json.Marshal(req.ScheduleTimes)
		scheduleTime = string(b)
	}
	schedType := req.ScheduleType
	if schedType == "" {
		if len(req.ScheduleTimes) > 0 || scheduleTime != "" {
			schedType = "times"
		} else {
			schedType = "interval"
		}
	}

	// AD 连接器默认将安全组放到 Users 容器下
	roleGroupContainer := req.RoleGroupContainer
	if roleGroupContainer == "" {
		var conn models.Connector
		if storage.DB.First(&conn, req.ConnectorID).Error == nil && conn.Type == "ldap_ad" {
			roleGroupContainer = "Users"
		}
	}

	rule := models.SyncRule{
		Name:               req.Name,
		ConnectorID:        req.ConnectorID,
		Direction:          "downstream",
		SourceType:         "local",
		TargetContainer:    req.TargetContainer,
		RoleGroupContainer: roleGroupContainer,
		EnableSchedule:   req.EnableSchedule,
		ScheduleType:     schedType,
		ScheduleTime:     scheduleTime,
		ScheduleInterval: req.ScheduleInterval,
		CronExpr:         req.CronExpr,
		EnableEvent:      req.EnableEvent,
		Events:           eventsJSON,
		SyncUsers:        req.SyncUsers,
		SyncGroups:       req.SyncGroups,
		SyncRoles:        req.SyncRoles,
		PreventPwdChange: req.PreventPwdChange,
		Status:           1,
	}

	if err := storage.DB.Create(&rule).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	// 创建默认映射
	var conn models.Connector
	if storage.DB.First(&conn, req.ConnectorID).Error == nil {
		createDefaultSyncRuleMappings(rule.ID, conn.Type)
	}

	// 创建后立即启动定时器
	if rule.EnableSchedule {
		startDownstreamScheduler(rule)
	}

	middleware.RecordOperationLog(c, "下游同步规则", "创建", rule.Name, "")
	respondOK(c, rule)
}

func GetDownstreamRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var rule models.SyncRule
	if err := storage.DB.Preload("Connector").First(&rule, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步规则不存在")
		return
	}

	var mappings []models.SyncAttributeMapping
	storage.DB.Where("sync_rule_id = ?", id).Order("object_type, priority").Find(&mappings)

	respondOK(c, gin.H{
		"rule":     rule,
		"mappings": mappings,
	})
}

func UpdateDownstreamRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var rule models.SyncRule
	if err := storage.DB.First(&rule, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步规则不存在")
		return
	}

	var req map[string]interface{}
	c.ShouldBindJSON(&req)

	fieldMap := map[string]string{
		"name": "name", "targetContainer": "target_container",
		"roleGroupContainer": "role_group_container",
		"enableSchedule": "enable_schedule", "scheduleType": "schedule_type",
		"scheduleTime": "schedule_time", "scheduleInterval": "schedule_interval",
		"cronExpr": "cron_expr", "enableEvent": "enable_event",
		"syncUsers": "sync_users", "syncGroups": "sync_groups", "syncRoles": "sync_roles",
		"preventPwdChange": "prevent_pwd_change", "status": "status",
	}
	updates := make(map[string]interface{})
	var selectFields []string
	for k, v := range req {
		if k == "events" {
			if arr, ok := v.([]interface{}); ok {
				b, _ := json.Marshal(arr)
				updates["events"] = string(b)
				selectFields = append(selectFields, "events")
			}
		} else if k == "scheduleTimes" {
			if arr, ok := v.([]interface{}); ok {
				b, _ := json.Marshal(arr)
				updates["schedule_time"] = string(b)
				updates["schedule_type"] = "times"
				selectFields = append(selectFields, "schedule_time", "schedule_type")
			}
		} else if dbField, ok := fieldMap[k]; ok {
			updates[dbField] = v
			selectFields = append(selectFields, dbField)
		}
	}

	if len(updates) > 0 {
		// 使用 Select 明确指定要更新的字段，确保布尔值 false 和零值也能被正确更新
		storage.DB.Model(&rule).Select(selectFields).Updates(updates)
	}

	// 重新加载规则并重启定时器
	var updated models.SyncRule
	storage.DB.First(&updated, id)
	stopDownstreamScheduler(uint(id))
	if updated.EnableSchedule && updated.Status == 1 {
		startDownstreamScheduler(updated)
	}

	middleware.RecordOperationLog(c, "下游同步规则", "更新", rule.Name, "")
	respondOK(c, nil)
}

func DeleteDownstreamRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var rule models.SyncRule
	if err := storage.DB.First(&rule, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步规则不存在")
		return
	}

	stopDownstreamScheduler(uint(id))
	storage.DB.Where("sync_rule_id = ?", id).Delete(&models.SyncAttributeMapping{})
	storage.DB.Delete(&rule)

	middleware.RecordOperationLog(c, "下游同步规则", "删除", rule.Name, "")
	respondOK(c, nil)
}

func TriggerDownstreamSync(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var rule models.SyncRule
	if err := storage.DB.First(&rule, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步规则不存在")
		return
	}

	go syncer.ExecuteFullSyncRule(rule, "manual")

	middleware.RecordOperationLog(c, "下游同步", "手动触发", rule.Name, "")
	respondOK(c, gin.H{"message": "下游同步已触发"})
}

// ========== 下游同步规则的属性映射 ==========

func ListDownstreamRuleMappings(c *gin.Context) {
	ruleID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var mappings []models.SyncAttributeMapping
	storage.DB.Where("sync_rule_id = ?", ruleID).Order("object_type, priority").Find(&mappings)
	respondOK(c, mappings)
}

func BatchUpdateDownstreamRuleMappings(c *gin.Context) {
	ruleID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		Mappings []models.SyncAttributeMapping `json:"mappings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	storage.DB.Where("sync_rule_id = ?", ruleID).Delete(&models.SyncAttributeMapping{})

	for i := range req.Mappings {
		req.Mappings[i].ID = 0
		req.Mappings[i].SyncRuleID = uint(ruleID)
		if req.Mappings[i].MappingType == "" {
			req.Mappings[i].MappingType = "mapping"
		}
	}
	if len(req.Mappings) > 0 {
		storage.DB.Select("*").Create(&req.Mappings)
	}

	respondOK(c, gin.H{"count": len(req.Mappings)})
}

// createDefaultSyncRuleMappings 为新下游同步规则创建默认映射
func createDefaultSyncRuleMappings(ruleID uint, connType string) {
	var mappings []models.SyncAttributeMapping

	switch connType {
	case "ldap_ad":
		mappings = []models.SyncAttributeMapping{
			{SyncRuleID: ruleID, ObjectType: "group", SourceAttribute: "name", TargetAttribute: "ou", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "group", SourceAttribute: "name", TargetAttribute: "description", MappingType: "mapping", Priority: 2, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "role", SourceAttribute: "name", TargetAttribute: "cn", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "role", SourceAttribute: "code", TargetAttribute: "description", MappingType: "mapping", Priority: 2, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "username", TargetAttribute: "sAMAccountName", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "username", TargetAttribute: "userPrincipalName", MappingType: "transform", TransformRule: "append:@domain", Priority: 2, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "cn", MappingType: "mapping", Priority: 3, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "displayName", MappingType: "mapping", Priority: 4, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "email", TargetAttribute: "mail", MappingType: "mapping", Priority: 5, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "phone", TargetAttribute: "mobile", MappingType: "mapping", Priority: 6, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "password_raw", TargetAttribute: "unicodePwd", MappingType: "transform", TransformRule: "password_to_unicode", Priority: 7, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "status", TargetAttribute: "userAccountControl", MappingType: "transform", TransformRule: "status_to_uac", Priority: 8, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "description", MappingType: "mapping", Priority: 9, IsEnabled: true},
		}
	case "ldap_generic":
		mappings = []models.SyncAttributeMapping{
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "username", TargetAttribute: "uid", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "cn", MappingType: "mapping", Priority: 2, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "displayName", MappingType: "mapping", Priority: 3, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "sn", MappingType: "mapping", Priority: 4, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "email", TargetAttribute: "mail", MappingType: "mapping", Priority: 5, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "phone", TargetAttribute: "telephoneNumber", MappingType: "mapping", Priority: 6, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "password_hash", TargetAttribute: "userPassword", MappingType: "mapping", Priority: 7, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "group", SourceAttribute: "name", TargetAttribute: "ou", MappingType: "mapping", Priority: 1, IsEnabled: true},
		}
	default:
		// 数据库类型
		mappings = []models.SyncAttributeMapping{
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "username", TargetAttribute: "username", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "password_hash", TargetAttribute: "password", MappingType: "mapping", Priority: 2, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "display_name", MappingType: "mapping", Priority: 3, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "email", TargetAttribute: "email", MappingType: "mapping", Priority: 4, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "phone", TargetAttribute: "phone", MappingType: "mapping", Priority: 5, IsEnabled: true},
		}
	}

	if len(mappings) > 0 {
		storage.DB.Create(&mappings)
	}
}

// TestDBConnection 导出测试函数
func TestDBConnection(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}

	msg, err := syncer.TestDBConnection(conn)
	if err != nil {
		respondOK(c, gin.H{"ok": false, "message": err.Error()})
		return
	}
	respondOK(c, gin.H{"ok": true, "message": msg})
}

// DiscoverDownstreamColumns 查询数据库列
func DiscoverDownstreamColumns(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}

	tableName := c.Query("table")
	if tableName == "" {
		respondError(c, http.StatusBadRequest, "缺少 table 参数")
		return
	}

	columns, err := syncer.DiscoverMySQLColumns(conn, tableName)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, columns)
}

// TestDBConnectionExists 检查 syncer 中是否导出了 TestDBConnection
var _ = fmt.Sprint // 确保 import

// ========== 下游定时同步调度器 ==========

var (
	downstreamMu   gosync.Mutex
	downstreamDone = make(map[uint]chan bool)
)

// StartDownstreamSchedulers 启动所有下游定时同步调度器（服务启动时调用）
func StartDownstreamSchedulers() {
	var rules []models.SyncRule
	storage.DB.Where("direction = ? AND status = ? AND enable_schedule = ?", "downstream", 1, true).Find(&rules)
	for _, rule := range rules {
		startDownstreamScheduler(rule)
	}
	if len(rules) > 0 {
		log.Printf("[下游调度] 已启动 %d 个定时任务", len(rules))
	}
}

func startDownstreamScheduler(rule models.SyncRule) {
	downstreamMu.Lock()
	defer downstreamMu.Unlock()

	if done, ok := downstreamDone[rule.ID]; ok {
		close(done)
		delete(downstreamDone, rule.ID)
	}

	done := make(chan bool)
	downstreamDone[rule.ID] = done

	schedType := rule.ScheduleType
	if schedType == "" {
		schedType = "interval"
	}

	if schedType == "times" {
		times := parseDownstreamScheduleTimes(rule.ScheduleTime)
		if len(times) == 0 { return }
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			lastTriggered := ""
			for {
				select {
				case <-done: return
				case now := <-ticker.C:
					nowStr := now.Format("15:04")
					for _, t := range times {
						t = strings.TrimSpace(t)
						if t == nowStr && lastTriggered != nowStr {
							lastTriggered = nowStr
							log.Printf("[下游定时同步] 定点触发规则: %s (时间: %s)", rule.Name, t)
							go syncer.ExecuteFullSyncRule(rule, "schedule")
							break
						}
					}
					if nowStr != lastTriggered { lastTriggered = "" }
				}
			}
		}()
	} else {
		interval := rule.ScheduleInterval
		if interval <= 0 { interval = 60 }
		go func() {
			ticker := time.NewTicker(time.Duration(interval) * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-done: return
				case <-ticker.C:
					log.Printf("[下游定时同步] 间隔触发规则: %s (每%d分钟)", rule.Name, interval)
					syncer.ExecuteFullSyncRule(rule, "schedule")
				}
			}
		}()
	}
}

func stopDownstreamScheduler(ruleID uint) {
	downstreamMu.Lock()
	defer downstreamMu.Unlock()
	if done, ok := downstreamDone[ruleID]; ok {
		close(done)
		delete(downstreamDone, ruleID)
	}
}

func parseDownstreamScheduleTimes(raw string) []string {
	if raw == "" { return nil }
	var times []string
	if err := json.Unmarshal([]byte(raw), &times); err != nil {
		return []string{strings.TrimSpace(raw)}
	}
	return times
}
