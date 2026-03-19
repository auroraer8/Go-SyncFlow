package handlers

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/services"
	"go-syncflow/internal/storage"
	syncer "go-syncflow/internal/sync"
)

// ========== 辅助函数 ==========

// generateAppID 生成 AppID（格式：ak_前缀 + 16位随机hex）
func generateAppID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "ak_" + hex.EncodeToString(b)
}

// generateAppKey 生成 AppKey（32位随机hex）
func generateAppKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// hashAppKey 对 AppKey 做 SHA256 哈希存储
func hashAppKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// getAppKeyHint 提取 AppKey 后4位作为提示
func getAppKeyHint(key string) string {
	if len(key) < 4 {
		return "****"
	}
	return "****" + key[len(key)-4:]
}

// ========== CRUD Handlers ==========

// ListAPIKeys 获取所有 API Key 列表
func ListAPIKeys(c *gin.Context) {
	var keys []models.APIKey
	query := storage.DB.Order("created_at DESC")

	// 搜索
	if keyword := c.Query("keyword"); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR app_id LIKE ? OR description LIKE ?", like, like, like)
	}

	// 状态过滤
	if status := c.Query("status"); status != "" {
		if status == "active" {
			query = query.Where("is_active = ?", true)
		} else if status == "inactive" {
			query = query.Where("is_active = ?", false)
		}
	}

	if err := query.Find(&keys).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "查询失败")
		return
	}

	// 构造返回结果（隐藏敏感字段）
	type apiKeyResp struct {
		models.APIKey
		PermissionList []string `json:"permissionList"`
		IPWhiteList    []string `json:"ipWhiteList"`
		IPBlackList    []string `json:"ipBlackList"`
		IsExpired      bool     `json:"isExpired"`
	}

	result := make([]apiKeyResp, 0)
	now := time.Now()
	for _, k := range keys {
		item := apiKeyResp{APIKey: k}

		// 解析 JSON 字段
		if k.Permissions != "" && k.Permissions != "[]" {
			json.Unmarshal([]byte(k.Permissions), &item.PermissionList)
		}
		if k.IPWhitelist != "" && k.IPWhitelist != "[]" {
			json.Unmarshal([]byte(k.IPWhitelist), &item.IPWhiteList)
		}
		if k.IPBlacklist != "" && k.IPBlacklist != "[]" {
			json.Unmarshal([]byte(k.IPBlacklist), &item.IPBlackList)
		}

		// 检查是否过期
		if k.ExpiresAt != nil && k.ExpiresAt.Before(now) {
			item.IsExpired = true
		}

		result = append(result, item)
	}

	respondOK(c, result)
}

// CreateAPIKey 创建新 API Key
func CreateAPIKey(c *gin.Context) {
	var req struct {
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description"`
		AppID       string   `json:"appId"`       // 可选自定义
		AppKey      string   `json:"appKey"`      // 可选自定义
		Permissions []string `json:"permissions"` // 权限范围
		IPWhitelist []string `json:"ipWhitelist"`
		IPBlacklist []string `json:"ipBlacklist"`
		RateLimit   int      `json:"rateLimit"`
		ExpiresAt   string   `json:"expiresAt"` // ISO8601 格式
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// AppID: 自定义或自动生成
	appID := req.AppID
	if appID == "" {
		appID = generateAppID()
	} else {
		// 检查自定义 AppID 格式
		if len(appID) < 4 || len(appID) > 64 {
			respondError(c, http.StatusBadRequest, "AppID 长度须在4-64位之间")
			return
		}
	}

	// 检查 AppID 唯一性
	var count int64
	storage.DB.Model(&models.APIKey{}).Where("app_id = ?", appID).Count(&count)
	if count > 0 {
		respondError(c, http.StatusConflict, "AppID 已存在")
		return
	}

	// AppKey: 自定义或自动生成
	rawAppKey := req.AppKey
	if rawAppKey == "" {
		rawAppKey = generateAppKey()
	} else if len(rawAppKey) < 16 {
		respondError(c, http.StatusBadRequest, "AppKey 长度不得少于16位")
		return
	}

	// 频率限制默认值
	rateLimit := req.RateLimit
	if rateLimit <= 0 {
		rateLimit = 60
	}

	// 序列化 JSON 字段
	permsJSON, _ := json.Marshal(req.Permissions)
	wlJSON, _ := json.Marshal(req.IPWhitelist)
	blJSON, _ := json.Marshal(req.IPBlacklist)

	apiKey := models.APIKey{
		AppID:       appID,
		AppKey:      hashAppKey(rawAppKey),
		AppKeyHint:  getAppKeyHint(rawAppKey),
		Name:        req.Name,
		Description: req.Description,
		Permissions: string(permsJSON),
		IPWhitelist: string(wlJSON),
		IPBlacklist: string(blJSON),
		RateLimit:   rateLimit,
		IsActive:    true,
		CreatedBy:   middleware.GetUserID(c),
	}

	// 过期时间
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			t, err = time.Parse("2006-01-02", req.ExpiresAt)
			if err != nil {
				respondError(c, http.StatusBadRequest, "过期时间格式错误")
				return
			}
		}
		apiKey.ExpiresAt = &t
	}

	if err := storage.DB.Create(&apiKey).Error; err != nil {
		log.Printf("[APIKey] 创建失败: %v", err)
		respondError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	middleware.RecordOperationLog(c, "API密钥", "创建API密钥", fmt.Sprintf("AppID: %s, 名称: %s", appID, req.Name), "")

	// 返回创建结果，**首次显示明文 AppKey**
	respondOK(c, gin.H{
		"id":     apiKey.ID,
		"appId":  apiKey.AppID,
		"appKey": rawAppKey, // 仅创建时返回明文
		"name":   apiKey.Name,
		"message": "API密钥创建成功，请妥善保存 AppKey，后续将无法再次查看完整密钥。",
	})
}

// UpdateAPIKey 更新 API Key 信息（不含重置密钥）
func UpdateAPIKey(c *gin.Context) {
	id := c.Param("id")

	var apiKey models.APIKey
	if err := storage.DB.First(&apiKey, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "API密钥不存在")
		return
	}

	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
		IPWhitelist []string `json:"ipWhitelist"`
		IPBlacklist []string `json:"ipBlacklist"`
		RateLimit   int      `json:"rateLimit"`
		IsActive    *bool    `json:"isActive"`
		ExpiresAt   *string  `json:"expiresAt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	var selectFields []string

	if req.Name != "" {
		updates["name"] = req.Name
		selectFields = append(selectFields, "name")
	}
	if req.Description != "" || c.Request.ContentLength > 0 {
		updates["description"] = req.Description
		selectFields = append(selectFields, "description")
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
		selectFields = append(selectFields, "is_active")
	}
	if req.RateLimit > 0 {
		updates["rate_limit"] = req.RateLimit
		selectFields = append(selectFields, "rate_limit")
	}

	// 权限
	if req.Permissions != nil {
		permsJSON, _ := json.Marshal(req.Permissions)
		updates["permissions"] = string(permsJSON)
		selectFields = append(selectFields, "permissions")
	}

	// IP 白名单
	if req.IPWhitelist != nil {
		wlJSON, _ := json.Marshal(req.IPWhitelist)
		updates["ip_whitelist"] = string(wlJSON)
		selectFields = append(selectFields, "ip_whitelist")
	}

	// IP 黑名单
	if req.IPBlacklist != nil {
		blJSON, _ := json.Marshal(req.IPBlacklist)
		updates["ip_blacklist"] = string(blJSON)
		selectFields = append(selectFields, "ip_blacklist")
	}

	// 过期时间
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			updates["expires_at"] = nil // 清除过期时间
		} else {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				t, err = time.Parse("2006-01-02", *req.ExpiresAt)
				if err != nil {
					respondError(c, http.StatusBadRequest, "过期时间格式错误")
					return
				}
			}
			updates["expires_at"] = &t
		}
		selectFields = append(selectFields, "expires_at")
	}

	if len(selectFields) == 0 {
		respondError(c, http.StatusBadRequest, "无更新内容")
		return
	}

	if err := storage.DB.Model(&apiKey).Select(selectFields).Updates(updates).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "更新失败")
		return
	}

	middleware.RecordOperationLog(c, "API密钥", "更新API密钥", fmt.Sprintf("AppID: %s", apiKey.AppID), "")
	respondOK(c, gin.H{"message": "更新成功"})
}

// ResetAPIKey 重置 AppKey（生成新密钥）
func ResetAPIKey(c *gin.Context) {
	id := c.Param("id")

	var apiKey models.APIKey
	if err := storage.DB.First(&apiKey, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "API密钥不存在")
		return
	}

	var req struct {
		AppKey string `json:"appKey"` // 可选自定义新密钥
	}
	c.ShouldBindJSON(&req)

	rawAppKey := req.AppKey
	if rawAppKey == "" {
		rawAppKey = generateAppKey()
	} else if len(rawAppKey) < 16 {
		respondError(c, http.StatusBadRequest, "AppKey 长度不得少于16位")
		return
	}

	if err := storage.DB.Model(&apiKey).Updates(map[string]interface{}{
		"app_key":      hashAppKey(rawAppKey),
		"app_key_hint": getAppKeyHint(rawAppKey),
	}).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "重置失败")
		return
	}

	middleware.RecordOperationLog(c, "API密钥", "重置API密钥", fmt.Sprintf("AppID: %s", apiKey.AppID), "")

	respondOK(c, gin.H{
		"appId":   apiKey.AppID,
		"appKey":  rawAppKey,
		"message": "密钥已重置，请妥善保存新的 AppKey。",
	})
}

// DeleteAPIKey 删除 API Key
func DeleteAPIKey(c *gin.Context) {
	id := c.Param("id")

	var apiKey models.APIKey
	if err := storage.DB.First(&apiKey, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "API密钥不存在")
		return
	}

	if err := storage.DB.Delete(&apiKey).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "删除失败")
		return
	}

	middleware.RecordOperationLog(c, "API密钥", "删除API密钥", fmt.Sprintf("AppID: %s, 名称: %s", apiKey.AppID, apiKey.Name), "")
	respondOK(c, gin.H{"message": "删除成功"})
}

// GetAPIKey 获取单个 API Key 详情
func GetAPIKey(c *gin.Context) {
	id := c.Param("id")

	var apiKey models.APIKey
	if err := storage.DB.First(&apiKey, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "API密钥不存在")
		return
	}

	respondOK(c, apiKey)
}

// ToggleAPIKeyStatus 启用/禁用 API Key
func ToggleAPIKeyStatus(c *gin.Context) {
	id := c.Param("id")

	var apiKey models.APIKey
	if err := storage.DB.First(&apiKey, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "API密钥不存在")
		return
	}

	newStatus := !apiKey.IsActive
	if err := storage.DB.Model(&apiKey).Update("is_active", newStatus).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "操作失败")
		return
	}

	action := "启用"
	if !newStatus {
		action = "禁用"
	}
	middleware.RecordOperationLog(c, "API密钥", action+"API密钥", fmt.Sprintf("AppID: %s", apiKey.AppID), "")
	respondOK(c, gin.H{"message": action + "成功", "isActive": newStatus})
}

// ========== API Key 认证中间件 ==========

// APIKeyAuthMiddleware 通过 AppID/AppKey 认证的中间件
// 仅支持请求头认证，不支持URL参数（安全考虑：避免AppKey出现在URL和日志中）
func APIKeyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.GetHeader("X-App-ID")
		appKey := c.GetHeader("X-App-Key")

		if appID == "" || appKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "缺少 X-App-ID 或 X-App-Key 请求头"})
			c.Abort()
			return
		}

		// 查找 API Key
		var apiKeyRecord models.APIKey
		if err := storage.DB.Where("app_id = ?", appID).First(&apiKeyRecord).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "AppID 无效"})
			c.Abort()
			return
		}

		// 检查状态
		if !apiKeyRecord.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "API密钥已被禁用"})
			c.Abort()
			return
		}

		// 检查过期
		if apiKeyRecord.ExpiresAt != nil && apiKeyRecord.ExpiresAt.Before(time.Now()) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "API密钥已过期"})
			c.Abort()
			return
		}

		// 验证 AppKey
		if hashAppKey(appKey) != apiKeyRecord.AppKey {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "AppKey 无效"})
			c.Abort()
			return
		}

		clientIP := c.ClientIP()

		// 检查 IP 黑名单
		if apiKeyRecord.IPBlacklist != "" && apiKeyRecord.IPBlacklist != "[]" {
			var blacklist []string
			if json.Unmarshal([]byte(apiKeyRecord.IPBlacklist), &blacklist) == nil {
				for _, ip := range blacklist {
					if matchIP(clientIP, ip) {
						c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "IP 在黑名单中"})
						c.Abort()
						return
					}
				}
			}
		}

		// 检查 IP 白名单（如果配置了白名单则仅允许白名单内的IP）
		if apiKeyRecord.IPWhitelist != "" && apiKeyRecord.IPWhitelist != "[]" {
			var whitelist []string
			if json.Unmarshal([]byte(apiKeyRecord.IPWhitelist), &whitelist) == nil && len(whitelist) > 0 {
				allowed := false
				for _, ip := range whitelist {
					if matchIP(clientIP, ip) {
						allowed = true
						break
					}
				}
				if !allowed {
					c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "IP 不在白名单中"})
					c.Abort()
					return
				}
			}
		}

		// 解析权限配置
		var permissions []string
		if apiKeyRecord.Permissions != "" && apiKeyRecord.Permissions != "[]" {
			json.Unmarshal([]byte(apiKeyRecord.Permissions), &permissions)
		}

		// 检查 API 权限
		if len(permissions) > 0 {
			// 从请求路径和方法推断所需权限
			requiredPerm := inferPermissionFromRequest(c.Request.Method, c.Request.URL.Path)
			if requiredPerm != "" && !hasAPIPermission(permissions, requiredPerm) {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"message": fmt.Sprintf("API密钥无此权限: %s", requiredPerm),
				})
				c.Abort()
				return
			}
		}

		// 更新使用统计（异步）
		go func() {
			now := time.Now()
			storage.DB.Model(&models.APIKey{}).Where("id = ?", apiKeyRecord.ID).Updates(map[string]interface{}{
				"last_used_at": now,
				"last_used_ip": clientIP,
				"usage_count":  apiKeyRecord.UsageCount + 1,
			})
		}()

		// 将 API Key 信息注入上下文
		c.Set("apiKeyId", apiKeyRecord.ID)
		c.Set("apiKeyAppId", apiKeyRecord.AppID)
		c.Set("apiKeyName", apiKeyRecord.Name)
		c.Set("apiKeyPermissions", permissions)
		// 标记为 API Key 认证（区别于 JWT 认证）
		c.Set("authType", "apikey")

		// 设置一个管理员用户身份（API Key 以创建者身份执行操作）
		c.Set("userId", apiKeyRecord.CreatedBy)
		c.Set("username", "apikey:"+apiKeyRecord.AppID)

		c.Next()
	}
}

// inferPermissionFromRequest 从请求路径和方法推断所需权限
func inferPermissionFromRequest(method, path string) string {
	// 移除前缀 /api/open/
	path = strings.TrimPrefix(path, "/api/open/")
	path = strings.TrimPrefix(path, "/")

	// 提取资源类型（第一段路径）
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	resource := parts[0]

	// 资源映射
	resourceMap := map[string]string{
		"users":       "user",
		"groups":      "group",
		"roles":       "role",
		"sso":         "sso",
		"vpn":         "vpn",
		"sync":        "sync",
		"connectors":  "sync",
		"logs":        "log",
		"security":    "security",
		"settings":    "settings",
		"apikeys":     "apikey",
		"notify":      "notify",
		"dingtalk":    "sync",
		"system":      "system",
		"auth":        "auth",
	}

	resourceType, ok := resourceMap[resource]
	if !ok {
		resourceType = resource
	}

	// 方法映射到操作
	var action string
	switch method {
	case "GET":
		action = "read"
	case "POST":
		// 特殊处理某些 POST 操作
		if strings.Contains(path, "/trigger") || strings.Contains(path, "/sync") {
			action = "execute"
		} else if strings.Contains(path, "/test") {
			action = "read"
		} else {
			action = "write"
		}
	case "PUT", "PATCH":
		action = "write"
	case "DELETE":
		action = "delete"
	default:
		action = "read"
	}

	return fmt.Sprintf("%s:%s", resourceType, action)
}

// hasAPIPermission 检查是否拥有所需权限
func hasAPIPermission(permissions []string, required string) bool {
	// 解析所需权限
	reqParts := strings.Split(required, ":")
	if len(reqParts) != 2 {
		return true // 无法解析则放行
	}
	reqResource := reqParts[0]
	reqAction := reqParts[1]

	for _, perm := range permissions {
		permParts := strings.Split(perm, ":")
		if len(permParts) != 2 {
			continue
		}
		permResource := permParts[0]
		permAction := permParts[1]

		// 检查资源匹配
		if permResource != "*" && permResource != reqResource {
			continue
		}

		// 检查操作匹配
		if permAction == "*" || permAction == "all" {
			return true // 拥有该资源的全部权限
		}
		if permAction == reqAction {
			return true
		}
		// write 权限包含 read
		if permAction == "write" && reqAction == "read" {
			return true
		}
		// all/full 权限包含所有操作
		if permAction == "full" {
			return true
		}
	}

	return false
}

// matchIP 简单 IP 匹配（支持精确匹配和 CIDR 前缀匹配）
func matchIP(clientIP, pattern string) bool {
	if clientIP == pattern {
		return true
	}
	// 简单前缀匹配（如 192.168.1.* 写法）
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, "*")
		if strings.HasPrefix(clientIP, prefix) {
			return true
		}
	}
	// CIDR 匹配（如 10.0.0.0/8）
	if strings.Contains(pattern, "/") {
		// 简单实现：比较网络前缀
		parts := strings.SplitN(pattern, "/", 2)
		if len(parts) == 2 {
			prefix := parts[0]
			// 粗略匹配，对于 /8, /16, /24
			switch parts[1] {
			case "8":
				p1 := strings.SplitN(prefix, ".", 2)
				c1 := strings.SplitN(clientIP, ".", 2)
				return len(p1) > 0 && len(c1) > 0 && p1[0] == c1[0]
			case "16":
				p1 := strings.SplitN(prefix, ".", 3)
				c1 := strings.SplitN(clientIP, ".", 3)
				return len(p1) >= 2 && len(c1) >= 2 && p1[0] == c1[0] && p1[1] == c1[1]
			case "24":
				p1 := strings.SplitN(prefix, ".", 4)
				c1 := strings.SplitN(clientIP, ".", 4)
				return len(p1) >= 3 && len(c1) >= 3 && p1[0] == c1[0] && p1[1] == c1[1] && p1[2] == c1[2]
			}
		}
	}
	return false
}

// ========== API Key 安全限制中间件 ==========

// APIKeySafetyMiddleware 开放 API 安全限制
// 禁止通过 API：操作 admin 用户、分配超级管理员角色、创建/删除 API 密钥
func APIKeySafetyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method

		// ========== 禁止通过 API 操作 API 密钥 ==========
		// API 密钥只能通过管理后台界面创建/删除/重置，防止权限滥用
		if strings.Contains(path, "/open/apikeys") {
			// 允许查看列表和详情（GET 请求）
			if method != "GET" {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"message": "API 安全限制：不允许通过 API 创建、修改或删除 API 密钥，请在管理后台操作",
				})
				c.Abort()
				return
			}
		}

		// ========== 禁止通过 API 操作 admin 用户 ==========
		if strings.Contains(path, "/open/users/") && method != "POST" {
			idStr := c.Param("id")
			if idStr != "" {
				var targetUser models.User
				if storage.DB.First(&targetUser, idStr).Error == nil {
					if targetUser.Username == "admin" {
						c.JSON(http.StatusForbidden, gin.H{
							"success": false,
							"message": "API 安全限制：不允许通过 API 操作管理员账户",
						})
						c.Abort()
						return
					}
				}
			}
		}

		// ========== 禁止通过 API 分配超级管理员角色 ==========
		// 检查创建用户时是否包含超级管理员角色
		if method == "POST" && strings.HasSuffix(path, "/open/users") {
			if containsSuperAdminRole(c) {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"message": "API 安全限制：不允许通过 API 分配超级管理员角色",
				})
				c.Abort()
				return
			}
		}

		// 检查更新用户时是否尝试分配超级管理员角色
		if method == "PUT" && strings.Contains(path, "/open/users/") && !strings.Contains(path, "/status") && !strings.Contains(path, "/reset-password") {
			if containsSuperAdminRole(c) {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"message": "API 安全限制：不允许通过 API 分配超级管理员角色",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// containsSuperAdminRole 检查请求体中是否包含超级管理员角色
func containsSuperAdminRole(c *gin.Context) bool {
	// 读取并恢复 body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return false
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var req struct {
		RoleIDs []uint `json:"roleIds"`
	}
	if json.Unmarshal(bodyBytes, &req) != nil || len(req.RoleIDs) == 0 {
		return false
	}

	// 查找 super_admin 角色 ID
	var superAdmin models.Role
	if storage.DB.Where("code = ?", "super_admin").First(&superAdmin).Error != nil {
		return false
	}

	for _, rid := range req.RoleIDs {
		if rid == superAdmin.ID {
			return true
		}
	}
	return false
}

// ========== OpenAPI 用户认证接口 ==========

// OpenAPIAuthenticate 外部系统用户认证接口
// 仅通过 AppID/AppKey 方式调用，用于验证用户名密码是否正确
// POST /api/open/auth/authenticate
func OpenAPIAuthenticate(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_PARAMS",
			"message": "参数错误: 需要 username 和 password",
		})
		return
	}

	clientIP := c.ClientIP()

	// 查找用户
	var user models.User
	if err := storage.DB.Where("username = ? AND is_deleted = 0", req.Username).First(&user).Error; err != nil {
		// 记录失败日志
		middleware.RecordLoginLog(0, req.Username, clientIP, "OpenAPI", false, "用户不存在")
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "USER_NOT_FOUND",
			"message": "用户不存在",
		})
		return
	}

	// 检查用户状态
	if user.Status == 0 {
		middleware.RecordLoginLog(user.ID, req.Username, clientIP, "OpenAPI", false, "用户已禁用")
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "USER_DISABLED",
			"message": "用户已被禁用",
		})
		return
	}

	// 检查用户是否过期
	if user.IsExpired() {
		middleware.RecordLoginLog(user.ID, req.Username, clientIP, "OpenAPI", false, "账户已过期")
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "USER_EXPIRED",
			"message": "账户已过期",
		})
		return
	}

	// 检查用户锁定
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		middleware.RecordLoginLog(user.ID, req.Username, clientIP, "OpenAPI", false, "账户被锁定")
		c.JSON(http.StatusOK, gin.H{
			"success":     false,
			"code":        "USER_LOCKED",
			"message":     "账户已被锁定",
			"lockedUntil": user.LockedUntil.Format(time.RFC3339),
		})
		return
	}

	// 验证密码 - 支持多种方式
	passwordValid := false

	// 方式1: 本地密码验证（bcrypt(sha256(password))）
	if user.Password != "" {
		// 先尝试直接比对（明文 -> bcrypt）
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err == nil {
			passwordValid = true
		}
		// 再尝试 sha256 后比对（兼容前端加密方式）
		if !passwordValid {
			sha256Hash := sha256.Sum256([]byte(req.Password))
			sha256Hex := hex.EncodeToString(sha256Hash[:])
			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(sha256Hex)); err == nil {
				passwordValid = true
			}
		}
	}

	// 方式2: 代理认证（如果本地验证失败且启用了代理认证）
	if !passwordValid && services.IsPasswordAuthProxyEnabled() {
		authenticated, learned, _ := services.ProxyAuthenticateUser(&user, req.Password)
		if authenticated {
			passwordValid = true
			if learned {
				syncer.DispatchSyncEvent(models.SyncEventPasswordChange, user.ID, req.Password)
				log.Printf("[OpenAPI-密码同步] 已触发用户 %s 密码同步事件", user.Username)
			}
		}
	}

	if !passwordValid {
		middleware.RecordLoginLog(user.ID, req.Username, clientIP, "OpenAPI", false, "密码错误")
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "INVALID_PASSWORD",
			"message": "用户名或密码错误",
		})
		return
	}

	// 认证成功
	middleware.RecordLoginLog(user.ID, req.Username, clientIP, "OpenAPI", true, "OpenAPI认证成功")

	// 获取用户组信息
	var groups []models.UserGroup
	storage.DB.Joins("JOIN user_group_members ON user_group_members.group_id = user_groups.id").
		Where("user_group_members.user_id = ?", user.ID).Find(&groups)
	groupNames := make([]string, len(groups))
	for i, g := range groups {
		groupNames[i] = g.Name
	}

	// 获取角色信息
	var roles []models.Role
	storage.DB.Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", user.ID).Find(&roles)
	roleNames := make([]string, len(roles))
	for i, r := range roles {
		roleNames[i] = r.Name
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"code":    "OK",
		"message": "认证成功",
		"data": gin.H{
			"userId":      user.ID,
			"username":    user.Username,
			"nickname":    user.Nickname,
			"email":       user.Email,
			"phone":       user.Phone,
			"status":      user.Status,
			"groups":      groupNames,
			"roles":       roleNames,
			"lastLoginAt": user.LastLoginAt,
		},
	})
}

// ========== OpenAPI 用户同步接口（含密码哈希） ==========

// OpenAPIGetUsersWithPassword 获取用户列表（含密码哈希）
// 此接口专门用于下游系统同步，返回密码哈希信息
// 仅通过 AppID/AppKey 方式调用，不影响前端用户安全
// GET /api/open/sync/users
func OpenAPIGetUsersWithPassword(c *gin.Context) {
	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 50
	}

	// 查询参数
	keyword := c.Query("keyword")
	status := c.Query("status")
	groupID := c.Query("group_id")
	updatedAfter := c.Query("updated_after") // 增量同步：只返回此时间之后更新的用户

	query := storage.DB.Model(&models.User{}).Where("is_deleted = 0")

	// 关键词搜索
	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR nickname LIKE ? OR email LIKE ? OR phone LIKE ?",
			likeKeyword, likeKeyword, likeKeyword, likeKeyword)
	}

	// 状态筛选
	if status != "" {
		if s, err := strconv.Atoi(status); err == nil {
			query = query.Where("status = ?", s)
		}
	}

	// 用户组筛选
	if groupID != "" {
		if gid, err := strconv.ParseUint(groupID, 10, 64); err == nil {
			query = query.Where("id IN (SELECT user_id FROM user_group_members WHERE group_id = ?)", gid)
		}
	}

	// 增量同步支持
	if updatedAfter != "" {
		if t, err := time.Parse(time.RFC3339, updatedAfter); err == nil {
			query = query.Where("updated_at > ?", t)
		}
	}

	// 统计总数
	var total int64
	query.Count(&total)

	// 分页查询
	var users []models.User
	offset := (page - 1) * pageSize
	query.Order("id ASC").Offset(offset).Limit(pageSize).Find(&users)

	// 预加载所有群组，构建路径缓存
	var allGroups []models.UserGroup
	storage.DB.Find(&allGroups)
	groupMap := make(map[uint]models.UserGroup)
	for _, g := range allGroups {
		groupMap[g.ID] = g
	}

	// 构建响应数据（含密码哈希）
	type UserSyncData struct {
		ID              uint       `json:"id"`
		Username        string     `json:"username"`
		Nickname        string     `json:"nickname"`
		Email           string     `json:"email"`
		Phone           string     `json:"phone"`
		Status          int        `json:"status"`
		PasswordHash    string     `json:"password_hash,omitempty"`    // bcrypt 哈希
		SambaNTPassword string     `json:"samba_nt_password,omitempty"` // NT Hash（用于 Samba/LDAP）
		CreatedAt       time.Time  `json:"created_at"`
		UpdatedAt       time.Time  `json:"updated_at"`
		LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
		Groups          []string   `json:"groups,omitempty"`
		GroupPath       string     `json:"group_path,omitempty"`
		Roles           []string   `json:"roles,omitempty"`
	}

	list := make([]UserSyncData, len(users))
	for i, u := range users {
		var groupNames []string
		var groupPath string
		if u.GroupID > 0 {
			if g, ok := groupMap[u.GroupID]; ok {
				groupNames = []string{g.Name}
				// 构建完整路径：从当前群组向上回溯
				var pathParts []string
				cur := g
				for {
					pathParts = append([]string{cur.Name}, pathParts...)
					if cur.ParentID == 0 {
						break
					}
					parent, ok := groupMap[cur.ParentID]
					if !ok {
						break
					}
					cur = parent
				}
				groupPath = strings.Join(pathParts, "/")
			}
		}
		if len(groupNames) == 0 {
			var groups []models.UserGroup
			storage.DB.Joins("JOIN user_group_members ON user_group_members.group_id = user_groups.id").
				Where("user_group_members.user_id = ?", u.ID).Find(&groups)
			for _, g := range groups {
				groupNames = append(groupNames, g.Name)
			}
		}

		// 获取角色
		var roles []models.Role
		storage.DB.Joins("JOIN user_roles ON user_roles.role_id = roles.id").
			Where("user_roles.user_id = ?", u.ID).Find(&roles)
		roleNames := make([]string, len(roles))
		for j, r := range roles {
			roleNames[j] = r.Name
		}

		list[i] = UserSyncData{
			ID:              u.ID,
			Username:        u.Username,
			Nickname:        u.Nickname,
			Email:           u.Email,
			Phone:           u.Phone,
			Status:          int(u.Status),
			PasswordHash:    u.Password,
			SambaNTPassword: u.SambaNTPassword,
			CreatedAt:       u.CreatedAt,
			UpdatedAt:       u.UpdatedAt,
			LastLoginAt:     u.LastLoginAt,
			Groups:          groupNames,
			GroupPath:       groupPath,
			Roles:           roleNames,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"list":     list,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
		"message": "用户同步数据获取成功",
	})
}

// OpenAPIGetUserWithPassword 获取单个用户详情（含密码哈希）
// GET /api/open/sync/users/:id
func OpenAPIGetUserWithPassword(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	if err := storage.DB.Where("id = ? AND is_deleted = 0", id).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "用户不存在",
		})
		return
	}

	// 获取用户组
	var groups []models.UserGroup
	storage.DB.Joins("JOIN user_group_members ON user_group_members.group_id = user_groups.id").
		Where("user_group_members.user_id = ?", user.ID).Find(&groups)
	groupNames := make([]string, len(groups))
	for i, g := range groups {
		groupNames[i] = g.Name
	}

	// 获取角色
	var roles []models.Role
	storage.DB.Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", user.ID).Find(&roles)
	roleNames := make([]string, len(roles))
	for i, r := range roles {
		roleNames[i] = r.Name
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":                user.ID,
			"username":          user.Username,
			"nickname":          user.Nickname,
			"email":             user.Email,
			"phone":             user.Phone,
			"status":            user.Status,
			"password_hash":     user.Password,        // bcrypt 哈希
			"samba_nt_password": user.SambaNTPassword, // NT Hash
			"created_at":        user.CreatedAt,
			"updated_at":        user.UpdatedAt,
			"last_login_at":     user.LastLoginAt,
			"groups":            groupNames,
			"roles":             roleNames,
		},
	})
}
