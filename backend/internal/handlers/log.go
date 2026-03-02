package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

func ListLoginLogs(c *gin.Context) {
	pageIndex, _ := strconv.Atoi(c.DefaultQuery("pageIndex", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	username := c.Query("username")
	status := c.Query("status")

	var logs []models.LoginLog
	var total int64

	ip := c.Query("ip")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	query := storage.DB.Model(&models.LoginLog{})
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if status != "" {
		s, _ := strconv.Atoi(status)
		query = query.Where("status = ?", s)
	}
	if ip != "" {
		query = query.Where("ip LIKE ?", "%"+ip+"%")
	}
	if startDate != "" {
		query = query.Where("created_at >= ?", startDate+" 00:00:00")
	}
	if endDate != "" {
		query = query.Where("created_at <= ?", endDate+" 23:59:59")
	}

	query.Count(&total)
	query.Offset(pageIndex * pageSize).Limit(pageSize).Order("id desc").Find(&logs)

	respondList(c, logs, total)
}

func ListOperationLogs(c *gin.Context) {
	pageIndex, _ := strconv.Atoi(c.DefaultQuery("pageIndex", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	username := c.Query("username")
	module := c.Query("module")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	var logs []models.OperationLog
	var total int64

	query := storage.DB.Model(&models.OperationLog{})
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if module != "" {
		query = query.Where("module = ?", module)
	}
	if startDate != "" {
		query = query.Where("created_at >= ?", startDate+" 00:00:00")
	}
	if endDate != "" {
		query = query.Where("created_at <= ?", endDate+" 23:59:59")
	}

	query.Count(&total)
	query.Offset(pageIndex * pageSize).Limit(pageSize).Order("id desc").Find(&logs)

	respondList(c, logs, total)
}

// ListAllSyncLogs 查询所有同步器的同步日志
func ListAllSyncLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	status := c.Query("status")
	event := c.Query("event")
	username := c.Query("username")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	var total int64
	var logs []models.SyncLog

	direction := c.Query("direction")

	triggerType := c.Query("triggerType")

	query := storage.DB.Model(&models.SyncLog{})
	if direction != "" {
		query = query.Where("direction = ?", direction)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if event != "" {
		query = query.Where("trigger_event = ?", event)
	}
	if triggerType != "" {
		query = query.Where("trigger_type = ?", triggerType)
	}
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	keyword := c.Query("keyword")
	if keyword != "" {
		query = query.Where("message LIKE ?", "%"+keyword+"%")
	}
	if startDate != "" {
		query = query.Where("created_at >= ?", startDate+" 00:00:00")
	}
	if endDate != "" {
		query = query.Where("created_at <= ?", endDate+" 23:59:59")
	}

	query.Count(&total)
	query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&logs)

	type SyncLogWithSource struct {
		models.SyncLog
		ConnectorName string `json:"connectorName"`
	}

	connIDs := make(map[uint]bool)
	for _, l := range logs {
		if l.ConnectorID > 0 {
			connIDs[l.ConnectorID] = true
		}
	}

	connNames := make(map[uint]string)
	if len(connIDs) > 0 {
		ids := make([]uint, 0, len(connIDs))
		for id := range connIDs {
			ids = append(ids, id)
		}
		var conns []models.Connector
		storage.DB.Select("id, name").Where("id IN ?", ids).Find(&conns)
		for _, c := range conns {
			connNames[c.ID] = c.Name
		}
	}

	result := make([]SyncLogWithSource, len(logs))
	for i, l := range logs {
		result[i] = SyncLogWithSource{
			SyncLog:       l,
			ConnectorName: connNames[l.ConnectorID],
		}
	}

	respondOK(c, gin.H{"list": result, "total": total})
}

// ========== 日志统计 ==========

// LoginLogStats 登录日志统计
func LoginLogStats(c *gin.Context) {
	today := time.Now().Format("2006-01-02")
	
	var todayTotal int64
	storage.DB.Model(&models.LoginLog{}).Where("DATE(created_at) = ?", today).Count(&todayTotal)
	
	var todaySuccess int64
	storage.DB.Model(&models.LoginLog{}).Where("DATE(created_at) = ? AND status = 1", today).Count(&todaySuccess)
	
	var todayFailed int64
	storage.DB.Model(&models.LoginLog{}).Where("DATE(created_at) = ? AND status = 0", today).Count(&todayFailed)
	
	var uniqueUsers int64
	storage.DB.Model(&models.LoginLog{}).Where("DATE(created_at) = ? AND status = 1", today).Distinct("username").Count(&uniqueUsers)
	
	respondOK(c, gin.H{
		"todayTotal":   todayTotal,
		"todaySuccess": todaySuccess,
		"todayFailed":  todayFailed,
		"uniqueUsers":  uniqueUsers,
	})
}

// OperationLogStats 操作日志统计
func OperationLogStats(c *gin.Context) {
	today := time.Now().Format("2006-01-02")
	
	var todayTotal int64
	storage.DB.Model(&models.OperationLog{}).Where("DATE(created_at) = ?", today).Count(&todayTotal)
	
	var createCount int64
	storage.DB.Model(&models.OperationLog{}).Where("DATE(created_at) = ? AND (action LIKE ? OR action LIKE ?)", today, "%新增%", "%创建%").Count(&createCount)
	
	var updateCount int64
	storage.DB.Model(&models.OperationLog{}).Where("DATE(created_at) = ? AND (action LIKE ? OR action LIKE ? OR action LIKE ?)", today, "%更新%", "%编辑%", "%修改%").Count(&updateCount)
	
	var deleteCount int64
	storage.DB.Model(&models.OperationLog{}).Where("DATE(created_at) = ? AND action LIKE ?", today, "%删除%").Count(&deleteCount)
	
	respondOK(c, gin.H{
		"todayTotal":  todayTotal,
		"createCount": createCount,
		"updateCount": updateCount,
		"deleteCount": deleteCount,
	})
}

// SystemLogStats 系统日志（登录+操作）统计
func SystemLogStats(c *gin.Context) {
	today := time.Now().Format("2006-01-02")
	
	var loginCount int64
	storage.DB.Model(&models.LoginLog{}).Where("DATE(created_at) = ?", today).Count(&loginCount)
	
	var operationCount int64
	storage.DB.Model(&models.OperationLog{}).Where("DATE(created_at) = ?", today).Count(&operationCount)
	
	var failedCount int64
	storage.DB.Model(&models.LoginLog{}).Where("DATE(created_at) = ? AND status = 0", today).Count(&failedCount)
	
	todayTotal := loginCount + operationCount
	
	respondOK(c, gin.H{
		"todayTotal":      todayTotal,
		"loginCount":      loginCount,
		"operationCount":  operationCount,
		"failedCount":     failedCount,
	})
}

// ========== 日志导出 (CSV/Excel) ==========

func escapeCSV(s string) string {
	s = strings.ReplaceAll(s, "\"", "\"\"")
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + s + "\""
	}
	return s
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// ExportLoginLogs 导出登录日志
func ExportLoginLogs(c *gin.Context) {
	var logs []models.LoginLog
	query := storage.DB.Model(&models.LoginLog{})
	username := c.Query("username")
	status := c.Query("status")
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if status != "" {
		s, _ := strconv.Atoi(status)
		query = query.Where("status = ?", s)
	}
	query.Order("id desc").Limit(10000).Find(&logs)

	// BOM + CSV
	var b strings.Builder
	b.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel
	b.WriteString("ID,用户名,IP地址,状态,备注,时间\n")
	for _, l := range logs {
		statusText := "登录失败"
		if l.Status == 1 {
			statusText = "登录成功"
		}
		b.WriteString(fmt.Sprintf("%d,%s,%s,%s,%s,%s\n",
			l.ID, escapeCSV(l.Username), escapeCSV(l.IP),
			statusText, escapeCSV(l.Message), fmtTime(l.CreatedAt)))
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=login_logs.csv")
	c.String(http.StatusOK, b.String())
}

// ExportOperationLogs 导出操作日志
func ExportOperationLogs(c *gin.Context) {
	var logs []models.OperationLog
	query := storage.DB.Model(&models.OperationLog{})
	username := c.Query("username")
	module := c.Query("module")
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if module != "" {
		query = query.Where("module = ?", module)
	}
	query.Order("id desc").Limit(10000).Find(&logs)

	var b strings.Builder
	b.WriteString("\xEF\xBB\xBF")
	b.WriteString("ID,用户名,模块,操作,目标,详情,IP地址,时间\n")
	for _, l := range logs {
		b.WriteString(fmt.Sprintf("%d,%s,%s,%s,%s,%s,%s,%s\n",
			l.ID, escapeCSV(l.Username), escapeCSV(l.Module),
			escapeCSV(l.Action), escapeCSV(l.Target), escapeCSV(l.Content),
			escapeCSV(l.IP), fmtTime(l.CreatedAt)))
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=operation_logs.csv")
	c.String(http.StatusOK, b.String())
}

// ExportSyncLogs 导出同步日志
func ExportSyncLogs(c *gin.Context) {
	var logs []models.SyncLog
	query := storage.DB.Model(&models.SyncLog{})
	status := c.Query("status")
	event := c.Query("event")
	username := c.Query("username")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if event != "" {
		query = query.Where("trigger_event = ?", event)
	}
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	query.Order("created_at DESC").Limit(10000).Find(&logs)

	var b strings.Builder
	b.WriteString("\xEF\xBB\xBF")
	b.WriteString("ID,时间,触发方式,事件,用户,状态,概要,详情,耗时(ms)\n")
	for _, l := range logs {
		b.WriteString(fmt.Sprintf("%d,%s,%s,%s,%s,%s,%s,%s,%d\n",
			l.ID, fmtTime(l.CreatedAt), escapeCSV(l.TriggerType),
			escapeCSV(l.TriggerEvent), escapeCSV(l.Username),
			escapeCSV(l.Status), escapeCSV(l.Message),
			escapeCSV(l.Detail), l.Duration))
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=sync_logs.csv")
	c.String(http.StatusOK, b.String())
}

// ========== 系统日志（登录+操作合并） ==========

type SystemLogEntry struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Username  string    `json:"username"`
	LogType   string    `json:"logType"`   // login / operation
	Summary   string    `json:"summary"`
	IP        string    `json:"ip"`
	Status    string    `json:"status"`
}

func ListSystemLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	logType := c.Query("type")       // login / operation / 空=全部
	statusFilter := c.Query("status") // success / failed
	keyword := c.Query("keyword")
	summaryKw := c.Query("summary")
	ipKw := c.Query("ip")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * size

	// 构建 WHERE 条件（登录日志）
	var loginWhereParts []string
	var loginArgs []interface{}
	if keyword != "" {
		loginWhereParts = append(loginWhereParts, "username LIKE ?")
		loginArgs = append(loginArgs, "%"+keyword+"%")
	}
	if summaryKw != "" {
		loginWhereParts = append(loginWhereParts, "COALESCE(message,'') LIKE ?")
		loginArgs = append(loginArgs, "%"+summaryKw+"%")
	}
	if ipKw != "" {
		loginWhereParts = append(loginWhereParts, "ip LIKE ?")
		loginArgs = append(loginArgs, "%"+ipKw+"%")
	}
	if startDate != "" {
		loginWhereParts = append(loginWhereParts, "created_at >= ?")
		loginArgs = append(loginArgs, startDate+" 00:00:00")
	}
	if endDate != "" {
		loginWhereParts = append(loginWhereParts, "created_at <= ?")
		loginArgs = append(loginArgs, endDate+" 23:59:59")
	}
	if statusFilter == "success" {
		loginWhereParts = append(loginWhereParts, "status = 1")
	} else if statusFilter == "failed" {
		loginWhereParts = append(loginWhereParts, "status = 0")
	}
	loginWhereClause := ""
	if len(loginWhereParts) > 0 {
		loginWhereClause = " WHERE " + strings.Join(loginWhereParts, " AND ")
	}

	// 构建 WHERE 条件（操作日志）
	var opWhereParts []string
	var opArgs []interface{}
	if keyword != "" {
		opWhereParts = append(opWhereParts, "username LIKE ?")
		opArgs = append(opArgs, "%"+keyword+"%")
	}
	if summaryKw != "" {
		opWhereParts = append(opWhereParts, "(module || ' ' || action || ' ' || COALESCE(target,'')) LIKE ?")
		opArgs = append(opArgs, "%"+summaryKw+"%")
	}
	if ipKw != "" {
		opWhereParts = append(opWhereParts, "ip LIKE ?")
		opArgs = append(opArgs, "%"+ipKw+"%")
	}
	if startDate != "" {
		opWhereParts = append(opWhereParts, "created_at >= ?")
		opArgs = append(opArgs, startDate+" 00:00:00")
	}
	if endDate != "" {
		opWhereParts = append(opWhereParts, "created_at <= ?")
		opArgs = append(opArgs, endDate+" 23:59:59")
	}
	opWhereClause := ""
	if len(opWhereParts) > 0 {
		opWhereClause = " WHERE " + strings.Join(opWhereParts, " AND ")
	}

	// UNION 查询：登录日志 + 操作日志
	loginSelect := fmt.Sprintf(
		`SELECT id, created_at, username, 'login' as log_type,
		 CASE WHEN status=1 THEN '登录成功' ELSE '登录失败: ' || COALESCE(message,'') END as summary,
		 ip, CASE WHEN status=1 THEN 'success' ELSE 'failed' END as status
		 FROM login_logs%s`, loginWhereClause)

	opSelect := fmt.Sprintf(
		`SELECT id, created_at, username, 'operation' as log_type,
		 module || ' - ' || action || ': ' || COALESCE(target,'') as summary,
		 ip, 'success' as status
		 FROM operation_logs%s`, opWhereClause)

	var unionQuery string
	var countArgs []interface{}
	
	// 根据状态筛选决定查询哪些日志
	includeOp := true
	if statusFilter == "failed" {
		// 失败只有登录日志有
		includeOp = false
	}
	
	switch logType {
	case "login":
		unionQuery = loginSelect
		countArgs = append(countArgs, loginArgs...)
	case "operation":
		if statusFilter == "failed" {
			// 操作日志没有失败状态，返回空
			respondOK(c, gin.H{"list": []SystemLogEntry{}, "total": 0})
			return
		}
		unionQuery = opSelect
		countArgs = append(countArgs, opArgs...)
	default:
		if includeOp {
			unionQuery = loginSelect + " UNION ALL " + opSelect
			countArgs = append(countArgs, loginArgs...)
			countArgs = append(countArgs, opArgs...)
		} else {
			unionQuery = loginSelect
			countArgs = append(countArgs, loginArgs...)
		}
	}

	// 计算总数
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS t", unionQuery)
	var total int64
	storage.DB.Raw(countSQL, countArgs...).Scan(&total)

	// 查询数据
	dataSQL := fmt.Sprintf("SELECT * FROM (%s) AS t ORDER BY created_at DESC LIMIT ? OFFSET ?", unionQuery)
	dataArgs := make([]interface{}, 0)
	dataArgs = append(dataArgs, countArgs...)
	dataArgs = append(dataArgs, size, offset)

	var logs []SystemLogEntry
	storage.DB.Raw(dataSQL, dataArgs...).Scan(&logs)

	respondOK(c, gin.H{"list": logs, "total": total})
}
