package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
	"go-syncflow/internal/vpn/dbdata"
)

// ========== API 调用日志 ==========

// ListAPIAccessLogs 查询 API 调用日志
func ListAPIAccessLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	authType := c.Query("authType")
	appID := c.Query("appId")
	caller := c.Query("caller")
	method := c.Query("method")
	statusCode := c.Query("statusCode")
	ip := c.Query("ip")
	path := c.Query("path")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	if page < 1 {
		page = 1
	}

	query := storage.DB.Model(&models.APIAccessLog{})
	if authType != "" {
		query = query.Where("auth_type = ?", authType)
	}
	if appID != "" {
		query = query.Where("app_id = ?", appID)
	}
	if caller != "" {
		query = query.Where("(app_name LIKE ? OR username LIKE ? OR app_id LIKE ?)", "%"+caller+"%", "%"+caller+"%", "%"+caller+"%")
	}
	if method != "" {
		query = query.Where("method = ?", method)
	}
	if statusCode != "" {
		sc, _ := strconv.Atoi(statusCode)
		query = query.Where("status_code = ?", sc)
	}
	if ip != "" {
		query = query.Where("ip LIKE ?", "%"+ip+"%")
	}
	if path != "" {
		query = query.Where("path LIKE ?", "%"+path+"%")
	}
	if startDate != "" {
		query = query.Where("created_at >= ?", startDate+" 00:00:00")
	}
	if endDate != "" {
		query = query.Where("created_at <= ?", endDate+" 23:59:59")
	}

	var total int64
	query.Count(&total)

	var logs []models.APIAccessLog
	query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&logs)

	respondOK(c, gin.H{"list": logs, "total": total})
}

// GetAPIAccessLogStats 获取 API 调用统计
func GetAPIAccessLogStats(c *gin.Context) {
	today := time.Now().Format("2006-01-02")

	// 今日总调用数
	var todayTotal int64
	storage.DB.Model(&models.APIAccessLog{}).Where("created_at >= ?", today+" 00:00:00").Count(&todayTotal)

	// 今日成功数
	var todaySuccess int64
	storage.DB.Model(&models.APIAccessLog{}).Where("created_at >= ? AND status_code < 400", today+" 00:00:00").Count(&todaySuccess)

	// 平均响应时间
	var avgDuration float64
	storage.DB.Model(&models.APIAccessLog{}).Where("created_at >= ?", today+" 00:00:00").Select("COALESCE(AVG(duration),0)").Scan(&avgDuration)

	// Top5 AppID
	type appStat struct {
		AppID string `json:"appId"`
		Count int64  `json:"count"`
	}
	var topApps []appStat
	storage.DB.Model(&models.APIAccessLog{}).
		Select("app_id, COUNT(*) as count").
		Where("created_at >= ? AND app_id != ''", today+" 00:00:00").
		Group("app_id").
		Order("count DESC").
		Limit(5).
		Find(&topApps)

	successRate := float64(0)
	if todayTotal > 0 {
		successRate = float64(todaySuccess) / float64(todayTotal) * 100
	}

	respondOK(c, gin.H{
		"todayTotal":   todayTotal,
		"successRate":  fmt.Sprintf("%.1f%%", successRate),
		"avgDuration":  fmt.Sprintf("%.0fms", avgDuration),
		"topApps":      topApps,
	})
}

// ========== API 调用日志记录中间件 ==========

// APIAccessLogMiddleware 记录 API 调用日志的中间件
func APIAccessLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 只记录 /api/open 路径的调用
		if !strings.HasPrefix(c.Request.URL.Path, "/api/open") {
			c.Next()
			return
		}

		// 读取请求体并缓存，以供后续生成概要使用
		if c.Request.Body != nil && c.Request.Method != "GET" {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				c.Set("requestBody", string(bodyBytes))
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		c.Next()

		// 异步写入日志
		go func() {
			duration := int(time.Since(start).Milliseconds())

			authType := "jwt"
			appID := ""
			appName := ""
			var apiKeyID uint
			var userID uint
			username := ""

			if v, exists := c.Get("apiKeyId"); exists {
				authType = "apikey"
				apiKeyID, _ = v.(uint)
			}
			if v, exists := c.Get("apiKeyAppId"); exists {
				appID, _ = v.(string)
			}
			if v, exists := c.Get("apiKeyName"); exists {
				appName, _ = v.(string)
			}
			if v, exists := c.Get("userID"); exists {
				userID, _ = v.(uint)
			}
			if v, exists := c.Get("username"); exists {
				username, _ = v.(string)
			}
			if username == "" && appID != "" {
				username = appID
			}
			
			// 如果没有 appName，尝试从数据库获取
			if appName == "" && apiKeyID > 0 {
				var apiKey models.APIKey
				if err := storage.DB.Select("name").First(&apiKey, apiKeyID).Error; err == nil {
					appName = apiKey.Name
				}
			}

			// 脱敏请求体
			reqBody := ""
			if body, exists := c.Get("requestBody"); exists {
				reqBody = sanitizeBody(body.(string))
			}

			// 脱敏查询参数
			query := sanitizeQuery(c.Request.URL.RawQuery)

			// 生成请求概要
			summary := generateAPISummary(c.Request.Method, c.Request.URL.Path, query, reqBody)

			errMsg := ""
			if c.Writer.Status() >= 400 {
				if v, exists := c.Get("errorMessage"); exists {
					errMsg, _ = v.(string)
				}
			}

			storage.DB.Create(&models.APIAccessLog{
				AuthType:     authType,
				AppID:        appID,
				AppName:      appName,
				APIKeyID:     apiKeyID,
				UserID:       userID,
				Username:     username,
				Method:       c.Request.Method,
				Path:         c.Request.URL.Path,
				Query:        query,
				RequestBody:  reqBody,
				Summary:      summary,
				StatusCode:   c.Writer.Status(),
				ResponseSize: c.Writer.Size(),
				IP:           c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
				Duration:     duration,
				ErrorMessage: errMsg,
			})
		}()
	}
}

// generateAPISummary 生成 API 请求概要
func generateAPISummary(method, path, query, body string) string {
	// 根据路径和请求内容生成概要
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 {
		return ""
	}
	
	// /api/open/xxx
	resource := ""
	if len(parts) >= 3 {
		resource = parts[2] // users, groups, roles, etc.
	}
	
	action := ""
	target := ""
	
	// 特殊路径处理
	if resource == "auth" && len(parts) >= 4 {
		subAction := parts[3]
		actionNames := map[string]string{
			"authenticate": "认证登录",
			"token":        "获取令牌",
			"refresh":      "刷新令牌",
			"logout":       "退出登录",
		}
		if name, ok := actionNames[subAction]; ok {
			target = extractTargetFromBody(body, resource)
			if target != "" {
				return name + ": " + target
			}
			return name
		}
	}

	switch method {
	case "GET":
		action = "查询"
		if len(parts) >= 4 {
			target = parts[3]
			action = "获取"
		} else if query != "" {
			if strings.Contains(query, "keyword=") {
				action = "搜索"
			}
		}
	case "POST":
		action = "创建"
		target = extractTargetFromBody(body, resource)
	case "PUT":
		action = "更新"
		if len(parts) >= 4 {
			target = parts[3]
		}
		if target == "" {
			target = extractTargetFromBody(body, resource)
		}
	case "DELETE":
		action = "删除"
		if len(parts) >= 4 {
			target = parts[3]
		}
	}
	
	// 处理 /api/open/sync/xxx 路径
	if resource == "sync" && len(parts) >= 4 {
		syncResource := parts[3]
		syncResNames := map[string]string{
			"users": "用户同步数据", "groups": "分组同步数据", "roles": "角色同步数据",
			"upstream": "上游同步", "downstream": "下游同步",
		}
		name := syncResNames[syncResource]
		if name == "" {
			name = syncResource
		}
		if len(parts) >= 5 {
			return fmt.Sprintf("获取%s: %s", name, parts[4])
		}
		return fmt.Sprintf("%s%s", action, name)
	}

	// 映射资源名称
	resourceNames := map[string]string{
		"users":        "用户",
		"groups":       "分组",
		"roles":        "角色",
		"auth":         "认证",
		"connectors":   "连接器",
		"apikeys":      "API密钥",
		"permissions":  "权限",
		"notify":       "通知",
		"settings":     "设置",
		"logs":         "日志",
		"vpn":          "VPN",
	}
	resourceName := resourceNames[resource]
	if resourceName == "" {
		resourceName = resource
	}
	
	if target != "" {
		return fmt.Sprintf("%s%s: %s", action, resourceName, target)
	}
	return fmt.Sprintf("%s%s", action, resourceName)
}

// extractTargetFromBody 从请求体中提取目标对象
func extractTargetFromBody(body, resource string) string {
	if body == "" {
		return ""
	}
	
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return ""
	}
	
	// 根据资源类型提取不同字段
	switch resource {
	case "users":
		if username, ok := data["username"].(string); ok {
			return username
		}
	case "groups":
		if name, ok := data["name"].(string); ok {
			return name
		}
	case "roles":
		if name, ok := data["name"].(string); ok {
			return name
		}
	case "auth":
		if username, ok := data["username"].(string); ok {
			return username
		}
		if account, ok := data["account"].(string); ok {
			return account
		}
	}
	
	// 通用：尝试 name 或 username
	if name, ok := data["name"].(string); ok {
		return name
	}
	if username, ok := data["username"].(string); ok {
		return username
	}
	
	return ""
}

// sanitizeBody 脱敏请求体
func sanitizeBody(body string) string {
	if len(body) > 2048 {
		body = body[:2048] + "...(truncated)"
	}

	// 替换敏感字段
	sensitiveFields := regexp.MustCompile(`(?i)"(password|secret|appKey|appSecret|token|bindPassword|dbPassword)":\s*"[^"]*"`)
	body = sensitiveFields.ReplaceAllString(body, `"$1":"***"`)

	return body
}

// sanitizeQuery 脱敏查询参数
func sanitizeQuery(query string) string {
	if query == "" {
		return ""
	}
	sensitiveParams := regexp.MustCompile(`(?i)(token|secret|password|key)=[^&]*`)
	return sensitiveParams.ReplaceAllString(query, "$1=***")
}

// ========== 日志保留设置 ==========

// LogRetentionConfig 日志保留配置
type LogRetentionConfig struct {
	LoginLogDays      int    `json:"loginLogDays"`
	OperationLogDays  int    `json:"operationLogDays"`
	SyncLogDays       int    `json:"syncLogDays"`
	APIAccessLogDays  int    `json:"apiAccessLogDays"`
	SecurityEventDays int    `json:"securityEventDays"`
	AlertLogDays      int    `json:"alertLogDays"`
	LoginAttemptDays  int    `json:"loginAttemptDays"`
	VPNLogDays        int    `json:"vpnLogDays"`        // VPN 访问日志保留天数
	VPNAuditLogDays   int    `json:"vpnAuditLogDays"`   // VPN 审计日志保留天数
	AutoCleanEnabled  bool   `json:"autoCleanEnabled"`
	CleanTime         string `json:"cleanTime"`
}

func defaultLogRetentionConfig() LogRetentionConfig {
	return LogRetentionConfig{
		LoginLogDays:      7,
		OperationLogDays:  15,
		SyncLogDays:       7,
		APIAccessLogDays:  7,
		SecurityEventDays: 30,
		AlertLogDays:      7,
		LoginAttemptDays:  30,
		VPNLogDays:        30,
		VPNAuditLogDays:   30,
		AutoCleanEnabled:  true,
		CleanTime:         "23:00",
	}
}

// GetLogRetention 获取日志保留配置
func GetLogRetention(c *gin.Context) {
	cfg := getLogRetentionConfig()
	respondOK(c, cfg)
}

// UpdateLogRetention 更新日志保留配置
func UpdateLogRetention(c *gin.Context) {
	var cfg LogRetentionConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	data, _ := json.Marshal(cfg)
	storage.SetConfig("log_retention", string(data))

	// 同步 VPN 日志保留配置到 VPN 模块
	syncVPNLogRetention(cfg)

	// 重新配置清理调度器
	RestartLogCleanupScheduler()

	middleware.RecordOperationLog(c, "日志管理", "更新日志保留配置", "", "")
	respondOK(c, nil)
}

// syncVPNLogRetention 同步 VPN 日志保留配置到 VPN 模块
func syncVPNLogRetention(cfg LogRetentionConfig) {
	// VPN 模块使用 LifeDay 作为日志保留天数
	// 取 VPN 活动日志和审计日志中较大的值作为 VPN 模块的保留天数
	vpnLifeDay := cfg.VPNLogDays
	if cfg.VPNAuditLogDays > vpnLifeDay {
		vpnLifeDay = cfg.VPNAuditLogDays
	}

	vpnAuditCfg := dbdata.SettingAuditLog{
		LifeDay:   vpnLifeDay,
		ClearTime: cfg.CleanTime,
	}
	if err := dbdata.SettingSet(vpnAuditCfg); err != nil {
		log.Printf("[日志管理] 同步 VPN 日志保留配置失败: %v", err)
	} else {
		log.Printf("[日志管理] 已同步 VPN 日志保留配置: %d 天", vpnLifeDay)
	}
}

// CleanLogsNow 立即执行日志清理
func CleanLogsNow(c *gin.Context) {
	result := cleanExpiredLogs()
	middleware.RecordOperationLog(c, "日志管理", "手动清理日志", "", result)
	respondOK(c, gin.H{"message": result})
}

// GetLogRetentionStats 获取各类日志统计
func GetLogRetentionStats(c *gin.Context) {
	type logStat struct {
		Type     string `json:"type"`
		Label    string `json:"label"`
		Count    int64  `json:"count"`
		SizeMB   string `json:"sizeMb"`
	}

	tables := []struct {
		model interface{}
		label string
		typ   string
	}{
		{&models.LoginLog{}, "登录日志", "loginLog"},
		{&models.OperationLog{}, "操作日志", "operationLog"},
		{&models.SyncLog{}, "同步日志", "syncLog"},
		{&models.APIAccessLog{}, "API调用日志", "apiAccessLog"},
		{&models.SecurityEvent{}, "安全事件", "securityEvent"},
		{&models.AlertLog{}, "告警日志", "alertLog"},
		{&models.LoginAttempt{}, "登录尝试", "loginAttempt"},
	}

	stats := make([]logStat, 0, len(tables)+2)
	for _, t := range tables {
		var count int64
		storage.DB.Model(t.model).Count(&count)

		// 估算磁盘占用（按平均行大小估算）
		avgRowSize := 300.0 // bytes
		sizeMB := float64(count) * avgRowSize / 1024 / 1024

		stats = append(stats, logStat{
			Type:   t.typ,
			Label:  t.label,
			Count:  count,
			SizeMB: fmt.Sprintf("%.1f", sizeMB),
		})
	}

	// 添加 VPN 日志统计
	vpnLogCount, _ := dbdata.CountUserActLog()
	vpnAuditCount, _ := dbdata.CountAccessAudit()
	stats = append(stats, logStat{
		Type:   "vpnLog",
		Label:  "VPN活动日志",
		Count:  vpnLogCount,
		SizeMB: fmt.Sprintf("%.1f", float64(vpnLogCount)*300.0/1024/1024),
	})
	stats = append(stats, logStat{
		Type:   "vpnAuditLog",
		Label:  "VPN审计日志",
		Count:  vpnAuditCount,
		SizeMB: fmt.Sprintf("%.1f", float64(vpnAuditCount)*300.0/1024/1024),
	})

	respondOK(c, stats)
}

func getLogRetentionConfig() LogRetentionConfig {
	cfg := defaultLogRetentionConfig()
	value, err := storage.GetConfig("log_retention")
	if err == nil && value != "" {
		json.Unmarshal([]byte(value), &cfg)
	}
	return cfg
}

// ========== 自动清理引擎 ==========

var logCleanupTicker *time.Ticker
var logCleanupDone chan bool

// StartLogCleanupScheduler 启动日志清理调度器
func StartLogCleanupScheduler() {
	cfg := getLogRetentionConfig()
	if !cfg.AutoCleanEnabled {
		return
	}

	// 解析清理时间
	cleanTime := cfg.CleanTime
	if cleanTime == "" {
		cleanTime = "03:00"
	}

	// 计算到下一个清理时间的间隔
	now := time.Now()
	parts := strings.SplitN(cleanTime, ":", 2)
	hour, _ := strconv.Atoi(parts[0])
	minute := 0
	if len(parts) > 1 {
		minute, _ = strconv.Atoi(parts[1])
	}

	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if next.Before(now) {
		next = next.Add(24 * time.Hour)
	}

	delay := next.Sub(now)
	log.Printf("[日志清理] 下次清理时间: %s (%.0f分钟后)", next.Format("2006-01-02 15:04"), delay.Minutes())

	// 先延迟到清理时间，然后每24小时执行一次
	go func() {
		time.Sleep(delay)
		log.Println("[日志清理] 执行定时清理...")
		cleanExpiredLogs()

		logCleanupTicker = time.NewTicker(24 * time.Hour)
		logCleanupDone = make(chan bool)

		for {
			select {
			case <-logCleanupDone:
				return
			case <-logCleanupTicker.C:
				log.Println("[日志清理] 执行定时清理...")
				cleanExpiredLogs()
			}
		}
	}()
}

// RestartLogCleanupScheduler 重启日志清理调度器
func RestartLogCleanupScheduler() {
	if logCleanupTicker != nil {
		logCleanupTicker.Stop()
	}
	if logCleanupDone != nil {
		close(logCleanupDone)
		logCleanupDone = nil
	}
	StartLogCleanupScheduler()
}

// cleanExpiredLogs 清理过期日志
func cleanExpiredLogs() string {
	cfg := getLogRetentionConfig()
	now := time.Now()

	type cleanTarget struct {
		tableName string
		label     string
		days      int
	}

	targets := []cleanTarget{
		{"login_logs", "登录日志", cfg.LoginLogDays},
		{"operation_logs", "操作日志", cfg.OperationLogDays},
		{"sync_logs", "同步日志", cfg.SyncLogDays},
		{"api_access_logs", "API调用日志", cfg.APIAccessLogDays},
		{"security_events", "安全事件", cfg.SecurityEventDays},
		{"alert_logs", "告警日志", cfg.AlertLogDays},
		{"login_attempts", "登录尝试", cfg.LoginAttemptDays},
	}

	var results []string
	totalDeleted := int64(0)

	for _, t := range targets {
		if t.days <= 0 {
			continue
		}
		cutoff := now.AddDate(0, 0, -t.days)
		result := storage.DB.Exec("DELETE FROM "+t.tableName+" WHERE created_at < ?", cutoff)
		deleted := result.RowsAffected
		if deleted > 0 {
			log.Printf("[日志清理] %s: 删除 %d 条 (保留 %d 天)", t.label, deleted, t.days)
			results = append(results, fmt.Sprintf("%s删除%d条", t.label, deleted))
			totalDeleted += deleted
		}
	}

	// 清理 VPN 日志（使用 VPN 模块的清理函数）
	vpnResults := cleanVPNLogs(cfg)
	if vpnResults != "" {
		results = append(results, vpnResults)
	}

	if len(results) == 0 {
		return "没有需要清理的日志"
	}
	return fmt.Sprintf("共清理 %d 条: %s", totalDeleted, strings.Join(results, ", "))
}

// cleanVPNLogs 清理 VPN 日志
func cleanVPNLogs(cfg LogRetentionConfig) string {
	var results []string

	// 清理 VPN 用户活动日志
	if cfg.VPNLogDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -cfg.VPNLogDays).Format("2006-01-02 15:04:05")
		affected, err := dbdata.UserActLogIns.ClearUserActLog(cutoff)
		if err != nil {
			log.Printf("[日志清理] VPN活动日志清理失败: %v", err)
		} else if affected > 0 {
			log.Printf("[日志清理] VPN活动日志: 删除 %d 条 (保留 %d 天)", affected, cfg.VPNLogDays)
			results = append(results, fmt.Sprintf("VPN活动日志删除%d条", affected))
		}
	}

	// 清理 VPN 审计日志
	if cfg.VPNAuditLogDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -cfg.VPNAuditLogDays).Format("2006-01-02 15:04:05")
		affected, err := dbdata.ClearAccessAudit(cutoff)
		if err != nil {
			log.Printf("[日志清理] VPN审计日志清理失败: %v", err)
		} else if affected > 0 {
			log.Printf("[日志清理] VPN审计日志: 删除 %d 条 (保留 %d 天)", affected, cfg.VPNAuditLogDays)
			results = append(results, fmt.Sprintf("VPN审计日志删除%d条", affected))
		}
	}

	return strings.Join(results, ", ")
}

// ExportAPIAccessLogs 导出API调用日志
func ExportAPIAccessLogs(c *gin.Context) {
	var logs []models.APIAccessLog
	query := storage.DB.Model(&models.APIAccessLog{})

	authType := c.Query("authType")
	appID := c.Query("appId")
	if authType != "" {
		query = query.Where("auth_type = ?", authType)
	}
	if appID != "" {
		query = query.Where("app_id = ?", appID)
	}

	query.Order("created_at DESC").Limit(10000).Find(&logs)

	var b strings.Builder
	b.WriteString("\xEF\xBB\xBF") // UTF-8 BOM
	b.WriteString("ID,时间,认证方式,调用者,方法,路径,状态码,耗时(ms),IP,错误\n")
	for _, l := range logs {
		b.WriteString(fmt.Sprintf("%d,%s,%s,%s,%s,%s,%d,%d,%s,%s\n",
			l.ID,
			l.CreatedAt.Format("2006-01-02 15:04:05"),
			l.AuthType,
			escapeCSVField(l.Username),
			l.Method,
			escapeCSVField(l.Path),
			l.StatusCode,
			l.Duration,
			l.IP,
			escapeCSVField(l.ErrorMessage),
		))
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=api_access_logs.csv")
	c.String(http.StatusOK, b.String())
}

func escapeCSVField(s string) string {
	s = strings.ReplaceAll(s, "\"", "\"\"")
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + s + "\""
	}
	return s
}
