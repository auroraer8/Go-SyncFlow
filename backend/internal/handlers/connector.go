package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/imclient"
	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
	"go-syncflow/internal/sync"
)

// ========== 连接器 CRUD ==========

func ListConnectors(c *gin.Context) {
	var list []models.Connector
	storage.DB.Order("created_at DESC").Find(&list)
	// 返回时不暴露密码，但标记是否已设置
	type connResp struct {
		models.Connector
		HasPassword bool `json:"hasPassword"`
	}
	resp := make([]connResp, 0, len(list))
	for _, c := range list {
		resp = append(resp, connResp{
			Connector:   c,
			HasPassword: c.BindPassword != "" || c.DBPassword != "",
		})
	}
	respondOK(c, resp)
}

func GetConnector(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}
	type connResp struct {
		models.Connector
		HasPassword bool `json:"hasPassword"`
	}
	respondOK(c, connResp{
		Connector:   conn,
		HasPassword: conn.BindPassword != "" || conn.DBPassword != "",
	})
}

func CreateConnector(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		Type         string `json:"type" binding:"required"` // ldap_ad / database
		Host         string `json:"host" binding:"required"`
		Port         int    `json:"port"`
		BackupHost   string `json:"backupHost"`
		BackupPort   int    `json:"backupPort"`
		UseTLS       bool   `json:"useTls"`
		BaseDN       string `json:"baseDn"`
		BindDN       string `json:"bindDn"`
		BindPassword string `json:"bindPassword"`
		Database     string `json:"database"`
		DBUser       string `json:"dbUser"`
		DBPassword   string `json:"dbPassword"`
		DBType       string `json:"dbType"`       // mysql/postgresql/oracle/sqlserver
		ServiceName  string `json:"serviceName"`   // Oracle ServiceName
		Charset      string `json:"charset"`
		UserTable    string `json:"userTable"`
		GroupTable   string `json:"groupTable"`
		RoleTable    string `json:"roleTable"`
		PwdFormat    string `json:"pwdFormat"`
		Timeout      int    `json:"timeout"`
		UPNSuffix    string `json:"upnSuffix"`
		UserFilter   string `json:"userFilter"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// 兼容旧类型 "mysql"，自动转换为 "database" + dbType
	if req.Type == "mysql" {
		req.Type = "database"
		if req.DBType == "" {
			req.DBType = "mysql"
		}
	}

	// 根据数据库类型设置默认端口
	if req.Port == 0 {
		if req.Type == "ldap_ad" {
			if req.UseTLS {
				req.Port = 636
			} else {
				req.Port = 389
			}
		} else {
			switch req.DBType {
			case "postgresql":
				req.Port = 5432
			case "oracle":
				req.Port = 1521
			case "sqlserver":
				req.Port = 1433
			default:
				req.Port = 3306
			}
		}
	}
	if req.Timeout == 0 {
		req.Timeout = 5
	}
	if req.Charset == "" && (req.DBType == "mysql" || req.DBType == "") {
		req.Charset = "utf8mb4"
	}
	if req.PwdFormat == "" {
		req.PwdFormat = "bcrypt"
	}

	conn := models.Connector{
		Name:         req.Name,
		Type:         req.Type,
		Host:         req.Host,
		Port:         req.Port,
		BackupHost:   req.BackupHost,
		BackupPort:   req.BackupPort,
		UseTLS:       req.UseTLS,
		BaseDN:       req.BaseDN,
		BindDN:       req.BindDN,
		BindPassword: req.BindPassword,
		Database:     req.Database,
		DBUser:       req.DBUser,
		DBPassword:   req.DBPassword,
		DBType:       req.DBType,
		ServiceName:  req.ServiceName,
		Charset:      req.Charset,
		UserTable:    req.UserTable,
		GroupTable:   req.GroupTable,
		RoleTable:    req.RoleTable,
		PwdFormat:    req.PwdFormat,
		Timeout:      req.Timeout,
		UPNSuffix:    req.UPNSuffix,
		UserFilter:   req.UserFilter,
		Status:       1,
	}

	if err := storage.DB.Create(&conn).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	middleware.RecordOperationLog(c, "连接器管理", "创建连接器", strconv.FormatUint(uint64(conn.ID), 10), conn.Name+" ("+conn.Type+")")
	respondOK(c, conn)
}

func UpdateConnector(c *gin.Context) {
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

	// 如果密码字段为空字符串则不更新
	if v, ok := req["bindPassword"]; ok && v == "" {
		delete(req, "bindPassword")
	}
	if v, ok := req["dbPassword"]; ok && v == "" {
		delete(req, "dbPassword")
	}

	// 兼容旧类型
	if t, ok := req["type"]; ok && t == "mysql" {
		req["type"] = "database"
		if _, hasDBType := req["dbType"]; !hasDBType {
			req["dbType"] = "mysql"
		}
	}

	// 转换字段名
	updates := make(map[string]interface{})
	var selectFields []string
	fieldMap := map[string]string{
		"name": "name", "type": "type", "host": "host", "port": "port",
		"backupHost": "backup_host", "backupPort": "backup_port", "useTls": "use_tls",
		"baseDn": "base_dn", "bindDn": "bind_dn", "bindPassword": "bind_password",
		"database": "database", "dbUser": "db_user", "dbPassword": "db_password",
		"dbType": "db_type", "serviceName": "service_name",
		"charset": "charset", "userTable": "user_table", "groupTable": "group_table",
		"roleTable": "role_table", "pwdFormat": "pwd_format", "timeout": "timeout",
		"upnSuffix": "upn_suffix", "userFilter": "user_filter", "status": "status",
	}
	for k, v := range req {
		if dbField, ok := fieldMap[k]; ok {
			updates[dbField] = v
			selectFields = append(selectFields, dbField)
		}
	}

	if len(selectFields) > 0 {
		storage.DB.Model(&conn).Select(selectFields).Updates(updates)
	}
	middleware.RecordOperationLog(c, "连接器管理", "更新连接器", strconv.FormatUint(id, 10), conn.Name)
	respondOK(c, nil)
}

func DeleteConnector(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// 检查是否有同步器引用
	var syncCount int64
	storage.DB.Model(&models.Synchronizer{}).Where("connector_id = ?", id).Count(&syncCount)
	if syncCount > 0 {
		respondError(c, http.StatusBadRequest, "该连接器正被同步器使用，无法删除")
		return
	}

	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}

	storage.DB.Delete(&conn)
	middleware.RecordOperationLog(c, "连接器管理", "删除连接器", strconv.FormatUint(id, 10), conn.Name)
	respondOK(c, nil)
}

func TestConnector(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}

	now := time.Now()
	var msg string
	var testErr error

	switch {
	case conn.Type == "ldap_ad":
		msg, testErr = sync.TestADConnection(conn)
	case conn.Type == "mysql" || conn.IsDatabase():
		msg, testErr = sync.TestDBConnection(conn)
	case conn.IsIM():
		// 测试 IM 连接器（飞书、企微、钉钉等）
		client, err := imclient.NewIMClient(conn)
		if err != nil {
			testErr = err
		} else {
			testErr = client.TestConnection()
			if testErr == nil {
				// 尝试获取部门数量
				depts, _ := client.GetAllDepartments()
				msg = fmt.Sprintf("连接成功，检测到 %d 个部门", len(depts))
			}
		}
	default:
		respondError(c, http.StatusBadRequest, "不支持的连接器类型")
		return
	}

	if testErr != nil {
		storage.DB.Model(&conn).Updates(map[string]interface{}{
			"last_test_at":  now,
			"last_test_ok":  false,
			"last_test_msg": testErr.Error(),
		})
		respondError(c, http.StatusOK, testErr.Error())
		return
	}

	storage.DB.Model(&conn).Updates(map[string]interface{}{
		"last_test_at":  now,
		"last_test_ok":  true,
		"last_test_msg": msg,
	})
	respondOK(c, gin.H{"message": msg})
}

// DiscoverColumns MySQL字段发现
func DiscoverColumns(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	tableName := c.Query("table")
	if tableName == "" {
		respondError(c, http.StatusBadRequest, "请指定表名")
		return
	}

	var conn models.Connector
	if err := storage.DB.First(&conn, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}

	if conn.Type != "mysql" && !conn.IsDatabase() {
		respondError(c, http.StatusBadRequest, "仅数据库连接器支持字段发现")
		return
	}

	columns, err := sync.DiscoverDBColumns(conn, tableName)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, columns)
}
