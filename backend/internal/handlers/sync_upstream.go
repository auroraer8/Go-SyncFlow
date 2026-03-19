package handlers

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	gosync "sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"go-syncflow/internal/imclient"
	"go-syncflow/internal/ldapserver"
	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/services"
	"go-syncflow/internal/storage"
	syncer "go-syncflow/internal/sync"
)

// ========== 上游连接器 CRUD ==========

func ListUpstreamConnectors(c *gin.Context) {
	var list []models.Connector
	storage.DB.Where("direction IN ?", []string{"upstream", "both"}).Order("created_at DESC").Find(&list)
	respondOK(c, list)
}

func CreateUpstreamConnector(c *gin.Context) {
	// 使用 map 接收再手动赋值（因为 models.Connector 的密码字段是 json:"-"，直接 Bind 会丢失密码）
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	var req models.Connector
	// 基本字段
	if v, ok := raw["name"].(string); ok { req.Name = v }
	if v, ok := raw["type"].(string); ok { req.Type = v }
	if v, ok := raw["host"].(string); ok { req.Host = v }
	if v, ok := raw["port"].(float64); ok { req.Port = int(v) }
	if v, ok := raw["useTls"].(bool); ok { req.UseTLS = v }
	if v, ok := raw["baseDn"].(string); ok { req.BaseDN = v }
	if v, ok := raw["bindDn"].(string); ok { req.BindDN = v }
	if v, ok := raw["bindPassword"].(string); ok { req.BindPassword = v }
	if v, ok := raw["upnSuffix"].(string); ok { req.UPNSuffix = v }
	if v, ok := raw["userFilter"].(string); ok { req.UserFilter = v }
	// IM 字段
	if v, ok := raw["imAppId"].(string); ok { req.IMAppID = v }
	if v, ok := raw["imAppSecret"].(string); ok { req.IMAppSecret = v }
	if v, ok := raw["imCorpId"].(string); ok { req.IMCorpID = v }
	if v, ok := raw["imAgentId"].(string); ok { req.IMAgentID = v }
	if v, ok := raw["imBaseUrl"].(string); ok { req.IMBaseURL = v }
	if v, ok := raw["imCallbackUrl"].(string); ok { req.IMCallbackURL = v }
	if v, ok := raw["imAutoRegister"].(bool); ok { req.IMAutoRegister = v }
	if v, ok := raw["imDefaultRoleId"].(float64); ok { req.IMDefaultRoleID = uint(v) }
	if v, ok := raw["imMatchField"].(string); ok { req.IMMatchField = v }
	if v, ok := raw["imUsernameRule"].(string); ok { req.IMUsernameRule = v }
	if v, ok := raw["imSyncInterval"].(float64); ok { req.IMSyncInterval = int(v) }
	if v, ok := raw["imEnableSso"].(bool); ok { req.IMEnableSSO = v }
	if v, ok := raw["imEnableQrLogin"].(bool); ok { req.IMEnableQRLogin = v }
	if v, ok := raw["imSsoPriority"].(float64); ok { req.IMSSOPriority = int(v) }
	if v, ok := raw["imSsoLabel"].(string); ok { req.IMSSOLabel = v }
	// DB 字段
	if v, ok := raw["dbType"].(string); ok { req.DBType = v }
	if v, ok := raw["database"].(string); ok { req.Database = v }
	if v, ok := raw["dbUser"].(string); ok { req.DBUser = v }
	if v, ok := raw["dbPassword"].(string); ok { req.DBPassword = v }
	if v, ok := raw["charset"].(string); ok { req.Charset = v }
	if v, ok := raw["serviceName"].(string); ok { req.ServiceName = v }
	if v, ok := raw["userTable"].(string); ok { req.UserTable = v }
	if v, ok := raw["groupTable"].(string); ok { req.GroupTable = v }
	if v, ok := raw["roleTable"].(string); ok { req.RoleTable = v }
	if v, ok := raw["pwdFormat"].(string); ok { req.PwdFormat = v }
	if v, ok := raw["timeout"].(float64); ok { req.Timeout = int(v) }
	// RADIUS 字段
	if v, ok := raw["radiusServer"].(string); ok { req.RadiusServer = v }
	if v, ok := raw["radiusPort"].(float64); ok { req.RadiusPort = int(v) }
	if v, ok := raw["radiusSecret"].(string); ok { req.RadiusSecret = v }
	if v, ok := raw["radiusNasIp"].(string); ok { req.RadiusNasIP = v }
	if v, ok := raw["radiusTimeout"].(float64); ok { req.RadiusTimeout = int(v) }
	// HTTP API 字段
	if v, ok := raw["httpProtocol"].(string); ok { req.HTTPProtocol = v }
	if v, ok := raw["httpMethod"].(string); ok { req.HTTPMethod = v }
	if v, ok := raw["httpUrl"].(string); ok { req.HTTPURL = v }
	if v, ok := raw["httpUseTls"].(bool); ok { req.HTTPUseTLS = v }
	if v, ok := raw["httpVerifyCert"].(bool); ok { req.HTTPVerifyCert = v }
	if v, ok := raw["httpContentType"].(string); ok { req.HTTPContentType = v }
	if v, ok := raw["httpEncryptMethod"].(string); ok { req.HTTPEncryptMethod = v }
	if v, ok := raw["httpEncryptKey"].(string); ok { req.HTTPEncryptKey = v }
	if v, ok := raw["httpBodyTemplate"].(string); ok { req.HTTPBodyTemplate = v }
	if v, ok := raw["httpSuccessFlag"].(string); ok { req.HTTPSuccessFlag = v }
	if v, ok := raw["httpHeaders"].(string); ok { req.HTTPHeaders = v }
	if v, ok := raw["httpEncoding"].(string); ok { req.HTTPEncoding = v }
	if v, ok := raw["httpTlsVersion"].(string); ok { req.HTTPTLSVersion = v }
	// HTTP API 同步配置
	if v, ok := raw["httpPaginationMode"].(string); ok { req.HTTPPaginationMode = v }
	if v, ok := raw["httpPageParam"].(string); ok { req.HTTPPageParam = v }
	if v, ok := raw["httpPageSizeParam"].(string); ok { req.HTTPPageSizeParam = v }
	if v, ok := raw["httpPageSize"].(float64); ok { req.HTTPPageSize = int(v) }
	if v, ok := raw["httpDataPath"].(string); ok { req.HTTPDataPath = v }
	if v, ok := raw["httpTotalPath"].(string); ok { req.HTTPTotalPath = v }
	// 生成密码开关
	if v, ok := raw["imGeneratePassword"].(bool); ok { req.IMGeneratePassword = v }

	// 强制方向：上游连接器统一设置为 upstream
	req.Direction = "upstream"

	if req.Status == 0 {
		req.Status = 1
	}

	// 使用 Select 明确指定所有字段，确保布尔值 false 和零值能正确保存（不被 GORM 默认值覆盖）
	if err := storage.DB.Select("*").Create(&req).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	middleware.RecordOperationLog(c, "上游连接器", "创建", req.Name+" ("+req.Type+")", "")
	respondOK(c, req)
}

func GetUpstreamConnector(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}
	respondOK(c, conn)
}

func UpdateUpstreamConnector(c *gin.Context) {
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

	// 不允许修改类型
	delete(req, "type")
	delete(req, "id")
	delete(req, "createdAt")

	// 处理密码字段：如果传空字符串则保留旧值（从 req 中删除以跳过更新）
	// 如果有实际值，保留在 req 中让 fieldMap 处理映射
	if v, ok := req["imAppSecret"]; ok {
		if s, isStr := v.(string); isStr && s == "" {
			delete(req, "imAppSecret")
		}
		// 非空值保留在 req 中，由 fieldMap 映射
	}
	if v, ok := req["bindPassword"]; ok {
		if s, isStr := v.(string); isStr && s == "" {
			delete(req, "bindPassword")
		}
	}
	if v, ok := req["dbPassword"]; ok {
		if s, isStr := v.(string); isStr && s == "" {
			delete(req, "dbPassword")
		}
	}
	if v, ok := req["radiusSecret"]; ok {
		if s, isStr := v.(string); isStr && s == "" {
			delete(req, "radiusSecret")
		}
	}
	if v, ok := req["httpEncryptKey"]; ok {
		if s, isStr := v.(string); isStr && s == "" {
			delete(req, "httpEncryptKey")
		}
	}

	// 驼峰转蛇形映射（包含密码字段，空值已在上面被删除）
	fieldMap := map[string]string{
		"name": "name", "status": "status", "timeout": "timeout",
		"host": "host", "port": "port", "backupHost": "backup_host", "backupPort": "backup_port",
		"useTls": "use_tls", "baseDn": "base_dn", "bindDn": "bind_dn",
		"upnSuffix": "upn_suffix", "userFilter": "user_filter",
		"dbType": "db_type", "database": "database", "dbUser": "db_user",
		"charset": "charset", "serviceName": "service_name",
		"userTable": "user_table", "groupTable": "group_table", "roleTable": "role_table",
		"pwdFormat": "pwd_format", "config": "config", "uniqueField": "unique_field",
		"imAppId": "im_app_id", "imAppSecret": "im_app_secret",
		"imCorpId": "im_corp_id", "imAgentId": "im_agent_id",
		"imBaseUrl": "im_base_url", "imCallbackUrl": "im_callback_url",
		"imAutoRegister": "im_auto_register", "imDefaultRoleId": "im_default_role_id",
		"imMatchField": "im_match_field", "imUsernameRule": "im_username_rule",
		"imSyncInterval": "im_sync_interval", "imGeneratePassword": "im_generate_password",
		"imEnableSso": "im_enable_sso", "imSsoEnable": "im_enable_sso",
		"imEnableQrLogin": "im_enable_qr_login",
		"imSsoPriority": "im_sso_priority", "imSsoLabel": "im_sso_label",
		"bindPassword": "bind_password", "dbPassword": "db_password",
		// OpenLDAP 属性映射
		"ldapUserObjectClass": "ldap_user_object_class", "ldapGroupObjectClass": "ldap_group_object_class",
		"ldapLoginAttr": "ldap_login_attr", "ldapNameAttr": "ldap_name_attr",
		"ldapMobileAttr": "ldap_mobile_attr", "ldapEmailAttr": "ldap_email_attr",
		"ldapMemberAttr": "ldap_member_attr",
		// RADIUS 字段
		"radiusServer": "radius_server", "radiusPort": "radius_port",
		"radiusSecret": "radius_secret", "radiusNasIp": "radius_nas_ip",
		"radiusTimeout": "radius_timeout",
		// HTTP API 字段
		"httpProtocol": "http_protocol", "httpMethod": "http_method",
		"httpUrl": "http_url", "httpUseTls": "http_use_tls", "httpVerifyCert": "http_verify_cert",
		"httpContentType": "http_content_type", "httpEncryptMethod": "http_encrypt_method",
		"httpEncryptKey": "http_encrypt_key", "httpBodyTemplate": "http_body_template",
		"httpSuccessFlag": "http_success_flag", "httpHeaders": "http_headers",
		"httpEncoding": "http_encoding", "httpTlsVersion": "http_tls_version",
		// HTTP API 同步配置
		"httpPaginationMode": "http_pagination_mode", "httpPageParam": "http_page_param",
		"httpPageSizeParam": "http_page_size_param", "httpPageSize": "http_page_size",
		"httpDataPath": "http_data_path", "httpTotalPath": "http_total_path",
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

	middleware.RecordOperationLog(c, "上游连接器", "更新", conn.Name, "")
	respondOK(c, nil)
}

func DeleteUpstreamConnector(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}

	// 删除关联数据
	storage.DB.Where("connector_id = ?", id).Delete(&models.IMDepartment{})
	storage.DB.Where("connector_id = ?", id).Delete(&models.IMUser{})
	storage.DB.Where("connector_id = ?", id).Delete(&models.SyncRule{})
	storage.DB.Delete(&conn)

	middleware.RecordOperationLog(c, "上游连接器", "删除", conn.Name, "")
	respondOK(c, nil)
}

func TestUpstreamConnector(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}

	now := time.Now()
	var testMsg string
	var testOK bool

	if conn.IsIM() {
		client, err := imclient.NewIMClient(conn)
		if err != nil {
			testMsg = err.Error()
		} else {
			if err := client.TestConnection(); err != nil {
				testMsg = "连接失败: " + err.Error()
			} else {
				testMsg = "连接成功"
				testOK = true
			}
		}
	} else if conn.IsLDAP() {
		var msg string
		var err error
		if conn.IsOpenLDAP() {
			msg, err = syncer.TestOpenLDAPConnection(conn)
		} else {
			msg, err = syncer.TestADConnection(conn)
		}
		if err != nil {
			testMsg = err.Error()
		} else {
			testMsg = msg
			testOK = true
		}
	} else if conn.IsDatabase() {
		msg, err := syncer.TestDBConnection(conn)
		if err != nil {
			testMsg = err.Error()
		} else {
			testMsg = msg
			testOK = true
		}
	} else if conn.IsHTTPAPI() {
		// HTTP API 类型：测试连接可用性
		msg, err := testHTTPAPIConnection(&conn)
		if err != nil {
			testMsg = err.Error()
		} else {
			testMsg = msg
			testOK = true
		}
	} else if conn.IsRADIUS() {
		// RADIUS 类型：暂时只检查配置完整性
		if conn.RadiusServer == "" || conn.RadiusSecret == "" {
			testMsg = "RADIUS 服务器地址或密钥未配置"
		} else {
			testMsg = "配置已保存（RADIUS 需要实际认证才能验证）"
			testOK = true
		}
	} else {
		testMsg = "不支持的连接器类型"
	}

	storage.DB.Model(&conn).Updates(map[string]interface{}{
		"last_test_at":  now,
		"last_test_ok":  testOK,
		"last_test_msg": testMsg,
	})

	respondOK(c, gin.H{"ok": testOK, "message": testMsg})
}

// testHTTPAPIConnection 测试 HTTP API 连接器
func testHTTPAPIConnection(conn *models.Connector) (string, error) {
	if conn.HTTPURL == "" {
		return "", fmt.Errorf("HTTP 认证地址未配置")
	}

	// 使用测试用户名和密码进行测试请求
	testUsername := "__connection_test__"
	testPassword := "__test_password__"

	// 构造请求体
	body := conn.HTTPBodyTemplate
	if body == "" {
		body = `{"username":"{USERNAME}","password":"{PASSWORD}"}`
	}
	body = strings.ReplaceAll(body, "{USERNAME}", testUsername)
	body = strings.ReplaceAll(body, "{PASSWORD}", testPassword)

	// 创建 HTTP 请求
	method := conn.HTTPMethod
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, conn.HTTPURL, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置 Content-Type
	contentType := conn.HTTPContentType
	if contentType == "" {
		contentType = "application/json"
	}
	req.Header.Set("Content-Type", contentType)

	// 添加自定义请求头
	if conn.HTTPHeaders != "" {
		// 尝试解析为 map[string]string 格式
		var headersMap map[string]string
		if err := json.Unmarshal([]byte(conn.HTTPHeaders), &headersMap); err == nil {
			for k, v := range headersMap {
				req.Header.Set(k, v)
			}
		} else {
			// 尝试解析为数组格式 [{"key":"X-App-ID","value":"xxx"}]
			var headersArray []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}
			if err := json.Unmarshal([]byte(conn.HTTPHeaders), &headersArray); err == nil {
				for _, h := range headersArray {
					if h.Key != "" {
						req.Header.Set(h.Key, h.Value)
					}
				}
			}
		}
	}

	// TLS 配置
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !conn.HTTPVerifyCert,
		},
	}

	timeout := conn.Timeout
	if timeout == 0 {
		timeout = 10
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeout) * time.Second,
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接失败: %v", err)
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 能收到响应就算连接正常（测试用的是假数据，非 2xx 状态码不代表连接失败）
	return "连接成功", nil
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ========== 上游连接器 IM 数据查询 ==========

func GetUpstreamDepartments(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var depts []models.IMDepartment
	storage.DB.Where("connector_id = ?", id).Order("sort_order asc, id asc").Find(&depts)
	respondOK(c, depts)
}

func GetUpstreamUsers(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	deptID := c.Query("deptId")
	keyword := c.Query("keyword")

	query := storage.DB.Model(&models.IMUser{}).Where("connector_id = ?", id)
	if deptID != "" {
		query = query.Where("department_id = ?", deptID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR mobile LIKE ? OR email LIKE ?", like, like, like)
	}

	var total int64
	query.Count(&total)

	var users []models.IMUser
	query.Offset((page - 1) * pageSize).Limit(pageSize).Order("id desc").Find(&users)

	respondList(c, users, total)
}

// ========== 上游同步规则 CRUD ==========

func ListUpstreamRules(c *gin.Context) {
	var list []models.SyncRule
	storage.DB.Preload("Connector").Where("direction = ?", "upstream").Order("created_at DESC").Find(&list)
	respondOK(c, list)
}

func CreateUpstreamRule(c *gin.Context) {
	var req struct {
		Name                 string   `json:"name" binding:"required"`
		ConnectorID          uint     `json:"connectorId" binding:"required"`
		EnableSchedule       bool     `json:"enableSchedule"`
		ScheduleType         string   `json:"scheduleType"`
		ScheduleTime         string   `json:"scheduleTime"`         // JSON数组 或 单个时间
		ScheduleTimes        []string `json:"scheduleTimes"`        // 前端发来的时间数组
		ScheduleInterval     int      `json:"scheduleInterval"`
		CronExpr             string   `json:"cronExpr"`
		EnableEvent          bool     `json:"enableEvent"`
		Events               []string `json:"events"`
		SyncUsers            bool     `json:"syncUsers"`
		SyncGroups           bool     `json:"syncGroups"`
		AutoCreateUser       bool     `json:"autoCreateUser"`
		AutoDisableUser      bool     `json:"autoDisableUser"`
		AutoSyncGroups       bool     `json:"autoSyncGroups"`
		PreservePassword     bool     `json:"preservePassword"`
		EnableChangeDetect   bool     `json:"enableChangeDetect"`
		ChangeDetectInterval int      `json:"changeDetectInterval"`
		ChangeDetectField    string   `json:"changeDetectField"`
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

	// 处理定时时间：优先使用 scheduleTimes 数组
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

	rule := models.SyncRule{
		Name:                 req.Name,
		ConnectorID:          req.ConnectorID,
		Direction:            "upstream",
		EnableSchedule:       req.EnableSchedule,
		ScheduleType:         schedType,
		ScheduleTime:         scheduleTime,
		ScheduleInterval:     req.ScheduleInterval,
		CronExpr:             req.CronExpr,
		EnableEvent:          req.EnableEvent,
		Events:               eventsJSON,
		SyncUsers:            req.SyncUsers,
		SyncGroups:           req.SyncGroups,
		AutoCreateUser:       req.AutoCreateUser,
		AutoDisableUser:      req.AutoDisableUser,
		AutoSyncGroups:       req.AutoSyncGroups,
		PreservePassword:     req.PreservePassword,
		EnableChangeDetect:   req.EnableChangeDetect,
		ChangeDetectInterval: req.ChangeDetectInterval,
		ChangeDetectField:    req.ChangeDetectField,
		Status:               1,
	}

	if err := storage.DB.Create(&rule).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	// 查找连接器类型，创建默认属性映射
	var conn models.Connector
	if storage.DB.First(&conn, rule.ConnectorID).Error == nil {
		createDefaultUpstreamMappings(rule.ID, conn.Type)
	}

	// 创建后立即启动定时器（无需重启服务）
	if rule.EnableSchedule {
		startUpstreamScheduler(rule)
	}

	middleware.RecordOperationLog(c, "上游同步规则", "创建", rule.Name, "")
	respondOK(c, rule)
}

func GetUpstreamRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var rule models.SyncRule
	if err := storage.DB.Preload("Connector").First(&rule, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步规则不存在")
		return
	}
	respondOK(c, rule)
}

func UpdateUpstreamRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var rule models.SyncRule
	if err := storage.DB.First(&rule, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步规则不存在")
		return
	}

	var req map[string]interface{}
	c.ShouldBindJSON(&req)

	fieldMap := map[string]string{
		"name": "name", "enableSchedule": "enable_schedule",
		"scheduleType": "schedule_type", "scheduleTime": "schedule_time",
		"scheduleInterval": "schedule_interval",
		"cronExpr": "cron_expr", "enableEvent": "enable_event",
		"syncUsers": "sync_users", "syncGroups": "sync_groups",
		"autoCreateUser": "auto_create_user", "autoDisableUser": "auto_disable_user",
		"autoSyncGroups": "auto_sync_groups", "preservePassword": "preserve_password",
		"enableChangeDetect": "enable_change_detect",
		"changeDetectInterval": "change_detect_interval",
		"changeDetectField": "change_detect_field",
		"status": "status",
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
			// 前端发来的时间数组，转为JSON存储
			if arr, ok := v.([]interface{}); ok {
				b, _ := json.Marshal(arr)
				updates["schedule_time"] = string(b)
				selectFields = append(selectFields, "schedule_time")
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

	// 重新加载规则并重启定时器（无需重启服务）
	var updated models.SyncRule
	storage.DB.First(&updated, id)
	StopUpstreamScheduler(uint(id))
	if updated.EnableSchedule && updated.Status == 1 {
		startUpstreamScheduler(updated)
	}

	middleware.RecordOperationLog(c, "上游同步规则", "更新", rule.Name, "")
	respondOK(c, nil)
}

func DeleteUpstreamRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var rule models.SyncRule
	if err := storage.DB.First(&rule, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步规则不存在")
		return
	}

	StopUpstreamScheduler(uint(id))
	storage.DB.Delete(&rule)
	middleware.RecordOperationLog(c, "上游同步规则", "删除", rule.Name, "")
	respondOK(c, nil)
}

// TriggerUpstreamSync 手动触发上游同步
func TriggerUpstreamSync(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var rule models.SyncRule
	if err := storage.DB.Preload("Connector").First(&rule, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步规则不存在")
		return
	}

	go executeUpstreamSync(rule, "manual")

	middleware.RecordOperationLog(c, "上游同步", "手动触发", rule.Name, "")
	respondOK(c, gin.H{"message": "上游同步已触发"})
}

// ========== 上游同步核心逻辑 ==========

func executeUpstreamSync(rule models.SyncRule, triggerType string) {
	start := time.Now()
	var conn models.Connector
	if err := storage.DB.First(&conn, rule.ConnectorID).Error; err != nil {
		logUpstreamSync(rule.ID, conn.ID, triggerType, "failed", "连接器不存在", 0, time.Since(start).Milliseconds())
		return
	}

	var result syncer.UpstreamSyncResult

	log.Printf("[UpstreamSync] 连接器=%s, type=%s, IsIM=%v, IsLDAP=%v, IsDB=%v, IsHTTPAPI=%v",
		conn.Name, conn.Type, conn.IsIM(), conn.IsLDAP(), conn.IsDatabase(), conn.IsHTTPAPI())

	switch {
	case conn.IsIM():
		result = executeIMUpstreamSync(conn, rule)
	case conn.IsLDAP():
		result = syncer.SyncUsersFromLDAP(conn, rule)
	case conn.IsDatabase():
		result = syncer.SyncUsersFromDatabase(conn, rule)
	case conn.IsHTTPAPI():
		result = syncer.SyncUsersFromHTTPAPI(conn, rule)
	default:
		result = syncer.UpstreamSyncResult{Error: "上游同步暂不支持此连接器类型: " + conn.Type}
	}
	log.Printf("[UpstreamSync] 同步结果: total=%d, created=%d, updated=%d, error=%s",
		result.UsersTotal, result.UsersCreated, result.UsersUpdated, result.Error)

	result.Duration = time.Since(start).String()

	status := "success"
	if result.Error != "" {
		status = "failed"
	} else if result.UsersCreated+result.UsersUpdated+result.UsersDisabled == 0 {
		status = "success"
	}

	msg := fmt.Sprintf("部门:%d 新增:%d 更新:%d 禁用:%d",
		result.DepartmentsSynced, result.UsersCreated, result.UsersUpdated, result.UsersDisabled)
	if result.Error != "" {
		msg = result.Error
	}

	affected := result.UsersCreated + result.UsersUpdated
	logUpstreamSync(rule.ID, conn.ID, triggerType, status, msg, affected, time.Since(start).Milliseconds())

	// 更新规则状态
	now := time.Now()
	storage.DB.Model(&rule).Updates(map[string]interface{}{
		"last_sync_at":      now,
		"last_sync_status":  status,
		"last_sync_message": msg,
		"sync_count":        rule.SyncCount + 1,
	})
}

func executeIMUpstreamSync(conn models.Connector, rule models.SyncRule) syncer.UpstreamSyncResult {
	result := syncer.UpstreamSyncResult{}

	client, err := imclient.NewIMClient(conn)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// 1. 拉取部门
	if rule.AutoSyncGroups || rule.SyncGroups {
		depts, err := client.GetAllDepartments()
		if err != nil {
			result.Error = "拉取部门失败: " + err.Error()
			return result
		}
		result.DepartmentsSynced = syncIMDepartments(conn, depts)
	}

	// 2. 拉取用户
	depts, _ := client.GetAllDepartments()
	allUsers := make(map[string]imclient.IMUserInfo) // 按 UserID 去重
	for _, dept := range depts {
		users, err := client.GetDepartmentUsers(dept.DeptID)
		if err != nil {
			log.Printf("[上游同步] 获取部门 %s 用户失败: %v", dept.Name, err)
			continue
		}
		for _, u := range users {
			u.DeptName = dept.Name
			allUsers[u.UserID] = u
		}
	}

	result.UsersTotal = len(allUsers)

	// 3. 更新 IM 缓存表
	syncIMUsers(conn, allUsers)

	// 4. 同步到本地用户
	processedUIDs := make(map[string]bool)
	for _, imUser := range allUsers {
		processedUIDs[imUser.UserID] = true
		detail := syncIMUserToLocal(conn, rule, imUser)
		result.Details = append(result.Details, detail)

		switch detail.Action {
		case "created":
			result.UsersCreated++
		case "updated":
			result.UsersUpdated++
		}
	}

	// 5. 检查本地用户是否在 IM 端已删除（自动禁用）
	// 安全检查：只有当本次成功获取到用户时才执行禁用逻辑
	// 避免因 API 调用失败导致所有用户被错误禁用
	if rule.AutoDisableUser && len(processedUIDs) > 0 {
		result.UsersDisabled = disableRemovedIMUsers(conn, processedUIDs)
	}

	// 6. 检查本地群组是否在 IM 端已删除（自动删除/归档）
	// 同样需要安全检查：只有成功获取到部门时才执行
	if rule.AutoSyncGroups && len(depts) > 0 {
		result.GroupsDeleted = removeDeletedIMGroups(conn, depts)
	}

	return result
}

func syncIMDepartments(conn models.Connector, depts []imclient.IMDeptInfo) int {
	count := 0
	for _, d := range depts {
		var existing models.IMDepartment
		err := storage.DB.Where("connector_id = ? AND remote_dept_id = ?", conn.ID, d.DeptID).First(&existing).Error
		if err != nil {
			// 创建
			storage.DB.Create(&models.IMDepartment{
				ConnectorID:  conn.ID,
				PlatformType: conn.Type,
				RemoteDeptID: d.DeptID,
				Name:         d.Name,
				ParentDeptID: d.ParentID,
				SortOrder:    d.Order,
			})
		} else {
			// 更新
			storage.DB.Model(&existing).Updates(map[string]interface{}{
				"name":           d.Name,
				"parent_dept_id": d.ParentID,
				"sort_order":     d.Order,
			})
		}

		// 同步到本地群组
		syncDeptToLocalGroup(conn, d)
		count++
	}
	return count
}

func syncDeptToLocalGroup(conn models.Connector, dept imclient.IMDeptInfo) {
	if dept.DeptID == "0" || dept.DeptID == "1" || dept.Name == "" || dept.Name == "根部门" {
		return // 跳过虚拟根部门，不创建本地群组
	}

	// 将 IM 远程部门 ID 转为 int64 用于关联 UserGroup.DingTalkDeptID
	remoteDeptID, _ := strconv.ParseInt(dept.DeptID, 10, 64)

	// 优先按 DingTalkDeptID（IM 部门 ID）查找已关联的本地群组，避免同名部门冲突
	var group models.UserGroup
	found := false
	if remoteDeptID > 0 {
		found = storage.DB.Where("ding_talk_dept_id = ?", remoteDeptID).First(&group).Error == nil
	}
	// 降级：按名称查找（兼容未关联的旧数据）
	if !found {
		found = storage.DB.Where("name = ? AND ding_talk_dept_id = 0", dept.Name).First(&group).Error == nil
	}

	// 记录旧值，用于检测变化并同步到下游
	oldName := group.Name
	oldParentID := group.ParentID

	// 查找父群组：优先按父部门的 IM ID 查找
	var parentID uint
	if dept.ParentID != "" && dept.ParentID != "0" && dept.ParentID != "1" {
		parentRemoteID, _ := strconv.ParseInt(dept.ParentID, 10, 64)
		if parentRemoteID > 0 {
			var parentGroup models.UserGroup
			if storage.DB.Where("ding_talk_dept_id = ?", parentRemoteID).First(&parentGroup).Error == nil {
				parentID = parentGroup.ID
			}
		}
		if parentID == 0 {
			var parentDept models.IMDepartment
			if storage.DB.Where("connector_id = ? AND remote_dept_id = ?", conn.ID, dept.ParentID).First(&parentDept).Error == nil {
				var parentGroup models.UserGroup
				parentDeptRemoteID, _ := strconv.ParseInt(parentDept.RemoteDeptID, 10, 64)
				if parentDeptRemoteID > 0 {
					if storage.DB.Where("ding_talk_dept_id = ?", parentDeptRemoteID).First(&parentGroup).Error == nil {
						parentID = parentGroup.ID
					}
				}
			}
		}
	}

	if found {
		// 更新已有群组
		updates := map[string]interface{}{"name": dept.Name, "parent_id": parentID}
		if remoteDeptID > 0 && group.DingTalkDeptID == 0 {
			updates["ding_talk_dept_id"] = remoteDeptID
		}
		storage.DB.Model(&group).Updates(updates)

		// 如果名称或父级变化，同步到下游
		if oldName != dept.Name || oldParentID != parentID {
			// 重新加载更新后的数据
			storage.DB.First(&group, group.ID)
			go syncer.SyncGroupToDownstream(group, "group_update", oldName, oldParentID)
			log.Printf("[上游同步] 群组已更新并同步到下游: %s (id=%d)", group.Name, group.ID)
		}
	} else {
		// 创建新群组
		group = models.UserGroup{
			Name:           dept.Name,
			ParentID:       parentID,
			Order:          dept.Order,
			DingTalkDeptID: remoteDeptID,
		}
		storage.DB.Create(&group)

		// 同步创建到下游
		go syncer.SyncGroupToDownstream(group, "group_create", "", 0)
		log.Printf("[上游同步] 群组已创建并同步到下游: %s (id=%d)", group.Name, group.ID)
	}
}

func syncIMUsers(conn models.Connector, allUsers map[string]imclient.IMUserInfo) {
	for _, u := range allUsers {
		var existing models.IMUser
		err := storage.DB.Where("connector_id = ? AND remote_user_id = ?", conn.ID, u.UserID).First(&existing).Error
		if err != nil {
			storage.DB.Create(&models.IMUser{
				ConnectorID:    conn.ID,
				PlatformType:   conn.Type,
				RemoteUserID:   u.UserID,
				Name:           u.Name,
				Mobile:         u.Mobile,
				Email:          u.Email,
				Avatar:         u.Avatar,
				JobTitle:       u.JobTitle,
				DepartmentID:   u.DeptID,
				DepartmentName: u.DeptName,
				Active:         u.Active,
			})
		} else {
			storage.DB.Model(&existing).Updates(map[string]interface{}{
				"name":            u.Name,
				"mobile":          u.Mobile,
				"email":           u.Email,
				"avatar":          u.Avatar,
				"job_title":       u.JobTitle,
				"department_id":   u.DeptID,
				"department_name": u.DeptName,
				"active":          u.Active,
			})
		}
	}
}

func syncIMUserToLocal(conn models.Connector, rule models.SyncRule, imUser imclient.IMUserInfo) syncer.UpstreamDetail {
	detail := syncer.UpstreamDetail{
		RemoteUID:  imUser.UserID,
		RemoteName: imUser.Name,
		Department: imUser.DeptName,
	}

	// 查找本地用户：优先已关联，再按前端配置的匹配字段
	var localUser models.User
	found := false

	// 第1层：按 IM 平台用户ID 匹配（已关联的用户一定能匹配到）
	if imUser.UserID != "" {
		found = storage.DB.Where("ding_talk_uid = ? AND is_deleted = 0", imUser.UserID).First(&localUser).Error == nil
	}

	// 第2层：按 IM 缓存表中的 local_user_id 匹配
	if !found {
		var imRecord models.IMUser
		if storage.DB.Where("connector_id = ? AND remote_user_id = ? AND local_user_id > 0", conn.ID, imUser.UserID).First(&imRecord).Error == nil {
			found = storage.DB.Where("id = ? AND is_deleted = 0", imRecord.LocalUserID).First(&localUser).Error == nil
		}
	}

	// 第3层：严格按前端配置的匹配字段查找（唯一匹配规则）
	if !found {
		matchField := conn.IMMatchField
		if matchField == "" {
			matchField = "mobile"
		}
		switch matchField {
		case "mobile":
			if imUser.Mobile != "" {
				found = storage.DB.Where("phone = ? AND is_deleted = 0", imUser.Mobile).First(&localUser).Error == nil
			}
		case "email":
			if imUser.Email != "" && strings.Contains(imUser.Email, "@") && strings.Index(imUser.Email, "@") > 0 {
				found = storage.DB.Where("email = ? AND is_deleted = 0", imUser.Email).First(&localUser).Error == nil
			}
		case "userid":
			// 已在第1层处理
		}
	}

	if !found && rule.AutoCreateUser {
		// 创建本地用户
		var hashedPwd string
		var rawPassword string
		var sambaNTHash string

		// 根据连接器配置决定是否生成随机密码
		if conn.IMGeneratePassword {
			rawPassword = generateRandomPassword()
			hashedPwd, _ = hashPasswordForUpstream(rawPassword)
			sambaNTHash = ldapserver.ComputeNTHash(rawPassword)
		}

		username := imclient.GenerateUsername(conn.IMUsernameRule, &imUser)

		// 查找部门对应的群组：优先按 IM 部门 ID 匹配，避免同名部门冲突
		var groupID uint
		if imUser.DeptID != "" {
			// 通过 IM 部门缓存表获取远程部门 ID -> 本地群组
			remoteDeptID, _ := strconv.ParseInt(imUser.DeptID, 10, 64)
			if remoteDeptID > 0 {
				var g models.UserGroup
				if storage.DB.Where("ding_talk_dept_id = ?", remoteDeptID).First(&g).Error == nil {
					groupID = g.ID
				}
			}
		}
		// 降级：按部门名称匹配（兼容旧数据）
		if groupID == 0 && imUser.DeptName != "" {
			var g models.UserGroup
			if storage.DB.Where("name = ?", imUser.DeptName).First(&g).Error == nil {
				groupID = g.ID
			}
		}

		// 无效邮箱不存入本地用户（@前必须有内容）
		userEmail := imUser.Email
		if userEmail != "" && (strings.Index(userEmail, "@") <= 0) {
			userEmail = ""
		}
		newUser := models.User{
			Username:        username,
			Password:        hashedPwd,
			SambaNTPassword: sambaNTHash,
			Nickname:        imUser.Name,
			Phone:           imUser.Mobile,
			Email:           userEmail,
			Avatar:          imUser.Avatar,
			JobTitle:        imUser.JobTitle,
			DepartmentName:  imUser.DeptName,
			GroupID:         groupID,
			Status:          1,
			Source:          conn.Type,
			DingTalkUID:     imUser.UserID,
		}
		// 使用 Select("*") 确保所有字段（包括 Status=1）都被正确保存
		if err := storage.DB.Select("*").Create(&newUser).Error; err != nil {
			detail.Action = "failed"
			detail.Message = "创建用户失败: " + err.Error()
			return detail
		}

		detail.LocalUser = username
		detail.Action = "created"

		// 分配默认角色
		if conn.IMDefaultRoleID > 0 {
			storage.DB.Create(&models.UserRole{UserID: newUser.ID, RoleID: conn.IMDefaultRoleID})
		}
		// 默认分配"普通用户"角色
		var normalRole models.Role
		if storage.DB.Where("code = ?", "user").First(&normalRole).Error == nil {
			var existUR models.UserRole
			if storage.DB.Where("user_id = ? AND role_id = ?", newUser.ID, normalRole.ID).First(&existUR).Error != nil {
				storage.DB.Create(&models.UserRole{UserID: newUser.ID, RoleID: normalRole.ID})
			}
		}

		// 更新 IM 用户关联
		storage.DB.Model(&models.IMUser{}).Where("connector_id = ? AND remote_user_id = ?", conn.ID, imUser.UserID).Update("local_user_id", newUser.ID)

		// 触发下游同步
		syncer.DispatchSyncEvent("user_create", newUser.ID, rawPassword)

		// 发送账号通知
		go sendAccountCreatedNotification(newUser, rawPassword)

	} else if found {
		// 更新已有用户（保留密码）
		updates := map[string]interface{}{
			"nickname":        imUser.Name,
			"avatar":          imUser.Avatar,
			"job_title":       imUser.JobTitle,
			"department_name": imUser.DeptName,
		}
		// 如果用户在 IM 端存在，确保状态为启用（恢复被错误禁用的用户）
		if localUser.Status == 0 {
			updates["status"] = int8(1)
			log.Printf("[上游同步] 用户 %s 在 IM 端存在，恢复启用状态", localUser.Username)
		}
		if imUser.Mobile != "" && localUser.Phone == "" {
			updates["phone"] = imUser.Mobile
		}
		if imUser.Email != "" && localUser.Email == "" {
			updates["email"] = imUser.Email
		}
		if localUser.Source == "" || localUser.Source == "local" {
			updates["source"] = conn.Type
		}
		if localUser.DingTalkUID == "" {
			updates["ding_talk_uid"] = imUser.UserID
		}

		// 更新群组：优先按 IM 部门 ID 精确匹配，避免同名部门冲突
		if imUser.DeptID != "" {
			remoteDeptID, _ := strconv.ParseInt(imUser.DeptID, 10, 64)
			if remoteDeptID > 0 {
				var g models.UserGroup
				if storage.DB.Where("ding_talk_dept_id = ?", remoteDeptID).First(&g).Error == nil {
					updates["group_id"] = g.ID
				}
			}
		}
		if updates["group_id"] == nil && imUser.DeptName != "" {
			var g models.UserGroup
			if storage.DB.Where("name = ? AND parent_id = 0", imUser.DeptName).First(&g).Error == nil {
				updates["group_id"] = g.ID
			}
		}

		storage.DB.Model(&localUser).Updates(updates)

		// 更新 IM 用户关联
		storage.DB.Model(&models.IMUser{}).Where("connector_id = ? AND remote_user_id = ?", conn.ID, imUser.UserID).Update("local_user_id", localUser.ID)

		detail.LocalUser = localUser.Username
		detail.Action = "updated"

		// 触发下游更新
		syncer.DispatchSyncEvent("user_update", localUser.ID, "")
	} else {
		detail.Action = "skipped"
		detail.Message = "未找到匹配用户且未启用自动创建"
	}

	return detail
}

func disableRemovedIMUsers(conn models.Connector, activeUIDs map[string]bool) int {
	// 查找所有已关联本地用户的 IM 用户
	var imUsers []models.IMUser
	storage.DB.Where("connector_id = ? AND local_user_id > 0", conn.ID).Find(&imUsers)

	disabled := 0
	for _, imu := range imUsers {
		if !activeUIDs[imu.RemoteUserID] {
			// IM 端已删除，禁用本地用户
			storage.DB.Model(&models.User{}).Where("id = ? AND status = 1", imu.LocalUserID).Update("status", 0)
			syncer.DispatchSyncEvent("user_disable", imu.LocalUserID, "")
			disabled++
		}
	}
	return disabled
}

// removeDeletedIMGroups 删除 IM 端已不存在的本地群组，并同步到下游
func removeDeletedIMGroups(conn models.Connector, activeDepts []imclient.IMDeptInfo) int {
	// 构建活跃部门 ID 集合
	activeDeptIDs := make(map[int64]bool)
	for _, d := range activeDepts {
		remoteDeptID, _ := strconv.ParseInt(d.DeptID, 10, 64)
		if remoteDeptID > 0 {
			activeDeptIDs[remoteDeptID] = true
		}
	}

	// 查找所有由该 IM 连接器同步创建的本地群组（DingTalkDeptID > 0）
	var localGroups []models.UserGroup
	storage.DB.Where("ding_talk_dept_id > 0").Find(&localGroups)

	deleted := 0
	// 按层级从深到浅排序删除（先删子级再删父级）
	// 计算每个群组的层级深度
	type groupWithLevel struct {
		group models.UserGroup
		level int
	}
	var groupsToDelete []groupWithLevel

	for _, g := range localGroups {
		if !activeDeptIDs[g.DingTalkDeptID] {
			// 该群组在 IM 端已不存在
			// 检查是否还有用户属于此群组
			var userCount int64
			storage.DB.Model(&models.User{}).Where("group_id = ? AND is_deleted = 0", g.ID).Count(&userCount)
			if userCount > 0 {
				// 有用户还在此群组，将用户移动到父级群组
				storage.DB.Model(&models.User{}).Where("group_id = ?", g.ID).Update("group_id", g.ParentID)
				log.Printf("[上游同步] 群组 %s 即将删除，已将 %d 个用户移动到父级群组", g.Name, userCount)

				// 触发这些用户的下游同步（更新 OU 位置）
				var usersToSync []models.User
				storage.DB.Where("group_id = ?", g.ParentID).Find(&usersToSync)
				for _, u := range usersToSync {
					syncer.DispatchSyncEvent("user_update", u.ID, "")
				}
			}

			// 计算层级深度
			level := 0
			parentID := g.ParentID
			for parentID > 0 {
				level++
				var parent models.UserGroup
				if storage.DB.First(&parent, parentID).Error != nil {
					break
				}
				parentID = parent.ParentID
			}
			groupsToDelete = append(groupsToDelete, groupWithLevel{group: g, level: level})
		}
	}

	// 按层级降序排序（深的先删）
	for i := 0; i < len(groupsToDelete); i++ {
		for j := i + 1; j < len(groupsToDelete); j++ {
			if groupsToDelete[j].level > groupsToDelete[i].level {
				groupsToDelete[i], groupsToDelete[j] = groupsToDelete[j], groupsToDelete[i]
			}
		}
	}

	// 执行删除
	for _, gwl := range groupsToDelete {
		g := gwl.group

		// 检查是否还有子群组
		var childCount int64
		storage.DB.Model(&models.UserGroup{}).Where("parent_id = ?", g.ID).Count(&childCount)
		if childCount > 0 {
			// 将子群组移动到父级
			storage.DB.Model(&models.UserGroup{}).Where("parent_id = ?", g.ID).Update("parent_id", g.ParentID)
			log.Printf("[上游同步] 群组 %s 即将删除，已将 %d 个子群组移动到父级", g.Name, childCount)
		}

		// 同步删除到下游（先同步再删本地）
		syncer.SyncGroupToDownstream(g, "group_delete", "", 0)

		// 删除本地群组
		storage.DB.Delete(&g)
		log.Printf("[上游同步] 群组已删除: %s (DingTalkDeptID=%d)", g.Name, g.DingTalkDeptID)
		deleted++
	}

	return deleted
}

func logUpstreamSync(ruleID, connID uint, triggerType, status, message string, affected int, duration int64) {
	log.Printf("[上游同步] ruleID=%d trigger=%s status=%s msg=%s", ruleID, triggerType, status, message)
	storage.DB.Create(&models.SyncLog{
		SyncRuleID:  ruleID,
		ConnectorID: connID,
		Direction:   "upstream",
		TriggerType: triggerType,
		Status:      status,
		Message:     message,
		AffectedCount: affected,
		Duration:    duration,
	})
}

// ========== 连接器类型列表 ==========

func GetConnectorTypes(c *gin.Context) {
	respondOK(c, models.ConnectorTypeOptions)
}

// ========== 辅助函数 ==========

func generateRandomPassword() string {
	const chars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$"
	b := make([]byte, 12)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[n.Int64()]
	}
	return string(b)
}

// hashPasswordForUpstream 上游同步专用密码哈希
func hashPasswordForUpstream(password string) (string, error) {
	return storage.HashPasswordForStorage(password)
}

// hashPasswordBcrypt 直接 bcrypt 哈希
func hashPasswordBcrypt(password string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h)
}

// ========== 上游定时同步调度器 ==========

var upstreamDone = make(map[uint]chan bool)
var upstreamMu gosync.Mutex

// StartUpstreamSchedulers 启动所有上游定时同步
func StartUpstreamSchedulers() {
	var rules []models.SyncRule
	storage.DB.Where("direction = ? AND status = ? AND enable_schedule = ?", "upstream", 1, true).Find(&rules)
	for _, rule := range rules {
		startUpstreamScheduler(rule)
	}

	// 同时检查没有规则但有定时间隔的连接器，直接同步
	var connectors []models.Connector
	storage.DB.Where("direction IN ? AND status = 1 AND im_sync_interval > 0", []string{"upstream", "both"}).Find(&connectors)
	for _, conn := range connectors {
		if conn.IsIM() {
			var ruleCount int64
			storage.DB.Model(&models.SyncRule{}).Where("connector_id = ? AND direction = ?", conn.ID, "upstream").Count(&ruleCount)
			if ruleCount == 0 {
				rule := models.SyncRule{
					Name:             conn.Name + " 自动同步",
					ConnectorID:      conn.ID,
					Direction:        "upstream",
					EnableSchedule:   true,
					ScheduleType:     "interval",
					ScheduleInterval: conn.IMSyncInterval,
					SyncUsers:        true,
					SyncGroups:       true,
					AutoCreateUser:   conn.IMAutoRegister,
					AutoDisableUser:  true,
					AutoSyncGroups:   true,
					PreservePassword: true,
					Status:           1,
				}
				storage.DB.Create(&rule)
				startUpstreamScheduler(rule)
			}
		}
	}

	// 启动变更检测调度器
	startChangeDetectSchedulers()

	log.Printf("[上游调度] 已启动定时任务")
}

// parseScheduleTimes 解析 ScheduleTime 字段 (支持JSON数组和逗号分隔)
func parseScheduleTimes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	// 尝试 JSON 数组
	var times []string
	if err := json.Unmarshal([]byte(raw), &times); err == nil {
		return times
	}
	// 尝试逗号分隔
	if strings.Contains(raw, ",") {
		parts := strings.Split(raw, ",")
		for _, p := range parts {
			t := strings.TrimSpace(p)
			if t != "" {
				times = append(times, t)
			}
		}
		return times
	}
	// 单个时间
	return []string{raw}
}

func startUpstreamScheduler(rule models.SyncRule) {
	upstreamMu.Lock()
	defer upstreamMu.Unlock()

	// 先停止旧的
	if done, ok := upstreamDone[rule.ID]; ok {
		close(done)
		delete(upstreamDone, rule.ID)
	}

	done := make(chan bool)
	upstreamDone[rule.ID] = done

	schedType := rule.ScheduleType
	if schedType == "" {
		// 兼容旧数据：如果有时间点就用 times，否则 interval
		if rule.ScheduleTime != "" {
			schedType = "times"
		} else {
			schedType = "interval"
		}
	}

	if schedType == "times" {
		// 定点时间模式
		times := parseScheduleTimes(rule.ScheduleTime)
		if len(times) == 0 {
			return
		}
		go func() {
			// 每30秒检查一次是否到达指定时间
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			lastTriggered := ""
			for {
				select {
				case <-done:
					return
				case now := <-ticker.C:
					nowStr := now.Format("15:04")
					for _, t := range times {
						t = strings.TrimSpace(t)
						if t == nowStr && lastTriggered != nowStr {
							lastTriggered = nowStr
							log.Printf("[上游定时同步] 定点触发规则: %s (时间: %s)", rule.Name, t)
							executeUpstreamSync(rule, "schedule")
							break
						}
					}
					// 每分钟切换后重置
					if nowStr != lastTriggered {
						lastTriggered = ""
					}
				}
			}
		}()
	} else {
		// 间隔模式
		interval := rule.ScheduleInterval
		if interval <= 0 {
			// 兼容旧数据：从连接器获取
			var conn models.Connector
			if storage.DB.First(&conn, rule.ConnectorID).Error == nil {
				interval = conn.IMSyncInterval
			}
		}
		if interval <= 0 {
			interval = 60 // 默认60分钟
		}
		go func() {
			ticker := time.NewTicker(time.Duration(interval) * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					log.Printf("[上游定时同步] 间隔触发规则: %s (每%d分钟)", rule.Name, interval)
					executeUpstreamSync(rule, "schedule")
				}
			}
		}()
	}
}

func StopUpstreamScheduler(ruleID uint) {
	upstreamMu.Lock()
	defer upstreamMu.Unlock()
	if done, ok := upstreamDone[ruleID]; ok {
		close(done)
		delete(upstreamDone, ruleID)
	}
}

// ========== 变更检测调度器 ==========

var changeDetectDone = make(map[uint]chan bool)

func startChangeDetectSchedulers() {
	var rules []models.SyncRule
	storage.DB.Where("direction = ? AND status = ? AND enable_change_detect = ?", "upstream", 1, true).Find(&rules)
	for _, rule := range rules {
		startChangeDetector(rule)
	}
}

func startChangeDetector(rule models.SyncRule) {
	upstreamMu.Lock()
	defer upstreamMu.Unlock()

	cdKey := rule.ID + 100000 // 避免和常规调度冲突
	if done, ok := changeDetectDone[cdKey]; ok {
		close(done)
		delete(changeDetectDone, cdKey)
	}

	interval := rule.ChangeDetectInterval
	if interval <= 0 {
		interval = 60
	}

	done := make(chan bool)
	changeDetectDone[cdKey] = done

	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				detectUpstreamChanges(rule)
			}
		}
	}()
}

func detectUpstreamChanges(rule models.SyncRule) {
	var conn models.Connector
	if err := storage.DB.First(&conn, rule.ConnectorID).Error; err != nil {
		return
	}

	if !conn.IsDatabase() {
		return // 变更检测仅支持数据库类型
	}

	changeField := rule.ChangeDetectField
	if changeField == "" {
		changeField = "updated_at"
	}

	var lastDetect time.Time
	if rule.LastChangeDetectAt != nil {
		lastDetect = *rule.LastChangeDetectAt
	} else {
		lastDetect = time.Now().Add(-24 * time.Hour) // 首次检测最近24小时
	}

	// 查询数据库中变更的记录数
	changedCount, err := queryDBChangedCount(conn, rule, changeField, lastDetect)
	if err != nil {
		log.Printf("[变更检测] 规则 %s 查询失败: %v", rule.Name, err)
		return
	}

	if changedCount > 0 {
		log.Printf("[变更检测] 规则 %s 检测到 %d 条变更，触发同步", rule.Name, changedCount)
		executeUpstreamSync(rule, "change_detect")
	}

	// 更新检测时间
	now := time.Now()
	storage.DB.Model(&rule).Update("last_change_detect_at", now)
}

func queryDBChangedCount(conn models.Connector, rule models.SyncRule, changeField string, since time.Time) (int, error) {
	db, err := syncer.OpenDBConnection(conn)
	if err != nil {
		return 0, fmt.Errorf("连接数据库失败: %v", err)
	}
	defer db.Close()

	table := conn.UserTable
	if table == "" {
		table = "users"
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s > ?", table, changeField)
	var count int
	err = db.QueryRow(query, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("查询变更记录失败: %v", err)
	}
	return count, nil
}

// ========== 上游同步规则的属性映射 ==========

func ListUpstreamRuleMappings(c *gin.Context) {
	ruleID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var mappings []models.SyncAttributeMapping
	storage.DB.Where("sync_rule_id = ?", ruleID).Order("object_type, priority").Find(&mappings)
	respondOK(c, mappings)
}

func BatchUpdateUpstreamRuleMappings(c *gin.Context) {
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

	// 从映射中提取 username 转换规则，同步回连接器
	var rule models.SyncRule
	if storage.DB.First(&rule, ruleID).Error == nil {
		for _, m := range req.Mappings {
			if m.TargetAttribute == "username" && m.MappingType == "transform" && m.IsEnabled && m.TransformRule != "" {
				storage.DB.Model(&models.Connector{}).Where("id = ?", rule.ConnectorID).
					Update("im_username_rule", m.TransformRule)
				break
			}
		}
	}
	if len(req.Mappings) > 0 {
		storage.DB.Select("*").Create(&req.Mappings)
	}

	middleware.RecordOperationLog(c, "上游属性映射", "批量更新", strconv.FormatUint(ruleID, 10), fmt.Sprintf("%d条", len(req.Mappings)))
	respondOK(c, gin.H{"count": len(req.Mappings)})
}

func ResetUpstreamRuleMappings(c *gin.Context) {
	ruleID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var rule models.SyncRule
	if err := storage.DB.Preload("Connector").First(&rule, ruleID).Error; err != nil {
		respondError(c, http.StatusNotFound, "同步规则不存在")
		return
	}

	// 删除旧映射
	storage.DB.Where("sync_rule_id = ?", ruleID).Delete(&models.SyncAttributeMapping{})

	// 根据连接器类型重建默认映射
	connType := ""
	if rule.Connector.ID > 0 {
		connType = rule.Connector.Type
	} else {
		var conn models.Connector
		if storage.DB.First(&conn, rule.ConnectorID).Error == nil {
			connType = conn.Type
		}
	}
	createDefaultUpstreamMappings(uint(ruleID), connType)

	var mappings []models.SyncAttributeMapping
	storage.DB.Where("sync_rule_id = ?", ruleID).Order("object_type, priority").Find(&mappings)
	respondOK(c, mappings)
}

// createDefaultUpstreamMappings 根据上游连接器类型创建默认属性映射
func createDefaultUpstreamMappings(ruleID uint, connType string) {
	// 获取连接器的用户名生成规则
	usernameRule := "pinyin"
	var rule models.SyncRule
	if storage.DB.First(&rule, ruleID).Error == nil {
		var conn models.Connector
		if storage.DB.First(&conn, rule.ConnectorID).Error == nil {
			if conn.IMUsernameRule != "" {
				usernameRule = conn.IMUsernameRule
			}
		}
	}

	var mappings []models.SyncAttributeMapping

	if strings.HasPrefix(connType, "im_") {
		// IM平台 → 本地用户（包含 username 转换规则）
		mappings = []models.SyncAttributeMapping{
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "name", TargetAttribute: "username", MappingType: "transform", TransformRule: usernameRule, Priority: 0, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "name", TargetAttribute: "nickname", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "mobile", TargetAttribute: "phone", MappingType: "mapping", Priority: 2, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "email", TargetAttribute: "email", MappingType: "mapping", Priority: 3, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "userid", TargetAttribute: "im_user_id", MappingType: "mapping", Priority: 4, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "avatar", TargetAttribute: "avatar", MappingType: "mapping", Priority: 5, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "title", TargetAttribute: "position", MappingType: "mapping", Priority: 6, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "group", SourceAttribute: "name", TargetAttribute: "name", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "group", SourceAttribute: "deptId", TargetAttribute: "remote_dept_id", MappingType: "mapping", Priority: 2, IsEnabled: true},
		}
	} else if connType == "ldap_ad" {
		// AD → 本地用户
		mappings = []models.SyncAttributeMapping{
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "sAMAccountName", TargetAttribute: "username", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "cn", TargetAttribute: "nickname", MappingType: "mapping", Priority: 2, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "displayName", TargetAttribute: "nickname", MappingType: "mapping", Priority: 3, IsEnabled: false},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "mail", TargetAttribute: "email", MappingType: "mapping", Priority: 4, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "mobile", TargetAttribute: "phone", MappingType: "mapping", Priority: 5, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "telephoneNumber", TargetAttribute: "phone", MappingType: "mapping", Priority: 6, IsEnabled: false},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "title", TargetAttribute: "job_title", MappingType: "mapping", Priority: 7, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "department", TargetAttribute: "department_name", MappingType: "mapping", Priority: 8, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "group", SourceAttribute: "ou", TargetAttribute: "name", MappingType: "mapping", Priority: 1, IsEnabled: true},
		}
	} else if connType == "ldap_openldap" || connType == "ldap_generic" {
		// OpenLDAP → 本地用户
		mappings = []models.SyncAttributeMapping{
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "uid", TargetAttribute: "username", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "cn", TargetAttribute: "nickname", MappingType: "mapping", Priority: 2, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "displayName", TargetAttribute: "nickname", MappingType: "mapping", Priority: 3, IsEnabled: false},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "mail", TargetAttribute: "email", MappingType: "mapping", Priority: 4, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "mobile", TargetAttribute: "phone", MappingType: "mapping", Priority: 5, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "telephoneNumber", TargetAttribute: "phone", MappingType: "mapping", Priority: 6, IsEnabled: false},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "title", TargetAttribute: "job_title", MappingType: "mapping", Priority: 7, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "departmentNumber", TargetAttribute: "department_name", MappingType: "mapping", Priority: 8, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "userPassword", TargetAttribute: "ldap_password", MappingType: "mapping", Priority: 9, IsEnabled: false},
			{SyncRuleID: ruleID, ObjectType: "group", SourceAttribute: "ou", TargetAttribute: "name", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "group", SourceAttribute: "cn", TargetAttribute: "name", MappingType: "mapping", Priority: 2, IsEnabled: false},
		}
	} else if strings.HasPrefix(connType, "db_") {
		// 数据库 → 本地用户
		mappings = []models.SyncAttributeMapping{
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "username", TargetAttribute: "username", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "name", TargetAttribute: "nickname", MappingType: "mapping", Priority: 2, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "nickname", MappingType: "mapping", Priority: 3, IsEnabled: false},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "display_name", TargetAttribute: "nickname", MappingType: "mapping", Priority: 4, IsEnabled: false},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "email", TargetAttribute: "email", MappingType: "mapping", Priority: 5, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "phone", TargetAttribute: "phone", MappingType: "mapping", Priority: 6, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "mobile", TargetAttribute: "phone", MappingType: "mapping", Priority: 7, IsEnabled: false},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "job_title", TargetAttribute: "job_title", MappingType: "mapping", Priority: 8, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "department", TargetAttribute: "department_name", MappingType: "mapping", Priority: 9, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "status", TargetAttribute: "status", MappingType: "mapping", Priority: 10, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "password", TargetAttribute: "password_hash", MappingType: "mapping", Priority: 11, IsEnabled: false},
		}
	} else if connType == "http_api" {
		// HTTP API → 本地用户
		mappings = []models.SyncAttributeMapping{
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "username", TargetAttribute: "username", MappingType: "mapping", Priority: 1, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "name", TargetAttribute: "nickname", MappingType: "mapping", Priority: 2, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "nickname", MappingType: "mapping", Priority: 3, IsEnabled: false},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "email", TargetAttribute: "email", MappingType: "mapping", Priority: 4, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "phone", TargetAttribute: "phone", MappingType: "mapping", Priority: 5, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "mobile", TargetAttribute: "phone", MappingType: "mapping", Priority: 6, IsEnabled: false},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "status", TargetAttribute: "status", MappingType: "mapping", Priority: 7, IsEnabled: true},
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "password", TargetAttribute: "password_hash", MappingType: "mapping", Priority: 8, IsEnabled: false},
		}
	} else if connType == "radius" {
		// RADIUS → 本地用户（通常只用于认证，不用于同步）
		mappings = []models.SyncAttributeMapping{
			{SyncRuleID: ruleID, ObjectType: "user", SourceAttribute: "User-Name", TargetAttribute: "username", MappingType: "mapping", Priority: 1, IsEnabled: true},
		}
	}

	if len(mappings) > 0 {
		storage.DB.Create(&mappings)
	}
}

// ========== 全局同步触发 ==========

// TriggerAllSyncNew 触发所有同步（上游+下游）
func TriggerAllSyncNew(c *gin.Context) {
	triggered := []string{}

	// 1. 触发所有上游同步规则
	var upRules []models.SyncRule
	storage.DB.Where("direction = ? AND status = 1", "upstream").Find(&upRules)
	log.Printf("[TriggerAll] 查到 %d 条上游规则", len(upRules))
	fmt.Printf("[TriggerAll-FMT] 查到 %d 条上游规则\n", len(upRules))
	for _, rule := range upRules {
		log.Printf("[TriggerAll] 上游规则: id=%d, name=%s, connector_id=%d", rule.ID, rule.Name, rule.ConnectorID)
		r := rule
		go executeUpstreamSync(r, "manual")
		triggered = append(triggered, "[上游] "+r.Name)
	}

	// 2. 触发所有下游同步规则
	var downRules []models.SyncRule
	storage.DB.Where("direction = ? AND status = 1", "downstream").Find(&downRules)
	for _, rule := range downRules {
		r := rule
		go syncer.ExecuteFullSyncRule(r, "manual")
		triggered = append(triggered, "[下游] "+r.Name)
	}

	// 3. 兼容旧同步器
	var oldSyncs []models.Synchronizer
	storage.DB.Where("status = ?", 1).Find(&oldSyncs)
	for _, s := range oldSyncs {
		sc := s
		go syncer.ExecuteFullSync(sc, "manual")
		triggered = append(triggered, "[旧] "+sc.Name)
	}

	middleware.RecordOperationLog(c, "全局同步", "触发全部",
		fmt.Sprintf("共%d项", len(triggered)), strings.Join(triggered, ", "))
	respondOK(c, gin.H{
		"message":   fmt.Sprintf("已触发 %d 个同步任务", len(triggered)),
		"triggered": triggered,
	})
}

// ========== 发送账号通知（复用自 dingtalk handler） ==========

func sendUpstreamAccountNotification(user models.User, rawPassword string) {
	sendAccountCreatedNotification(user, rawPassword)
}

// ========== 辅助：使用 services 包发送 ==========
var _ = services.SendNotificationByChannels // 确保 import
