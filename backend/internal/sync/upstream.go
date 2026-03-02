package sync

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf16"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"golang.org/x/crypto/bcrypt"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

// 注意：dialLDAP、dialDB、UpstreamSyncResult、UpstreamDetail 已在 engine.go 中定义

// ========== 上游同步统一入口 ==========

// ExecuteUpstreamSyncByType 根据连接器类型执行上游同步
func ExecuteUpstreamSyncByType(conn models.Connector, rule models.SyncRule) UpstreamSyncResult {
	switch {
	case conn.IsIM():
		// IM 类型由原有的 executeIMUpstreamSync 处理
		return UpstreamSyncResult{Error: "IM类型请使用专用同步函数"}
	case conn.IsLDAP():
		return SyncUsersFromLDAP(conn, rule)
	case conn.IsDatabase():
		return SyncUsersFromDatabase(conn, rule)
	case conn.IsHTTPAPI():
		return SyncUsersFromHTTPAPI(conn, rule)
	default:
		return UpstreamSyncResult{Error: "不支持的连接器类型: " + conn.Type}
	}
}

// ========== LDAP/AD 上游同步 ==========

// SyncUsersFromLDAP 从 LDAP/AD 同步用户到本地
func SyncUsersFromLDAP(conn models.Connector, rule models.SyncRule) UpstreamSyncResult {
	result := UpstreamSyncResult{}

	l, err := dialLDAP(conn)
	if err != nil {
		result.Error = "连接LDAP失败: " + err.Error()
		return result
	}
	defer l.Close()

	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		result.Error = "LDAP认证失败: " + err.Error()
		return result
	}

	// 获取字段映射
	mappings := getUpstreamMappings(rule.ID, conn.Type)

	// 确定用户过滤器
	userFilter := conn.UserFilter
	if userFilter == "" {
		if conn.IsOpenLDAP() {
			userObjectClass := conn.LDAPUserObjectClass
			if userObjectClass == "" {
				userObjectClass = "inetOrgPerson"
			}
			userFilter = fmt.Sprintf("(objectClass=%s)", userObjectClass)
		} else {
			userFilter = "(&(objectClass=user)(objectCategory=person))"
		}
	}

	// 构建需要获取的属性列表
	attrs := buildLDAPAttributes(conn, mappings)

	// 搜索用户
	searchReq := ldapv3.NewSearchRequest(
		conn.BaseDN,
		ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 0, 0, false,
		userFilter, attrs, nil,
	)

	sr, err := l.Search(searchReq)
	if err != nil {
		if ldapErr, ok := err.(*ldapv3.Error); ok && ldapErr.ResultCode == ldapv3.LDAPResultSizeLimitExceeded {
			if sr == nil || len(sr.Entries) == 0 {
				result.Error = "LDAP搜索失败: " + err.Error()
				return result
			}
		} else {
			result.Error = "LDAP搜索失败: " + err.Error()
			return result
		}
	}

	result.UsersTotal = len(sr.Entries)

	// 处理每个用户
	for _, entry := range sr.Entries {
		detail := processLDAPEntry(conn, rule, entry, mappings)
		result.Details = append(result.Details, detail)

		switch detail.Action {
		case "created":
			result.UsersCreated++
			username := detail.LocalUser
			if username != "" {
				var u models.User
				if storage.DB.Where("username = ? AND is_deleted = 0", username).First(&u).Error == nil {
					DispatchSyncEvent(models.SyncEventUserCreate, u.ID, "")
				}
			}
		case "updated":
			result.UsersUpdated++
			username := detail.LocalUser
			if username != "" {
				var u models.User
				if storage.DB.Where("username = ? AND is_deleted = 0", username).First(&u).Error == nil {
					DispatchSyncEvent(models.SyncEventUserUpdate, u.ID, "")
				}
			}
		case "disabled":
			result.UsersDisabled++
		}
	}

	return result
}

// buildLDAPAttributes 根据映射构建 LDAP 属性列表
func buildLDAPAttributes(conn models.Connector, mappings []models.SyncAttributeMapping) []string {
	attrSet := make(map[string]bool)
	attrSet["dn"] = true

	for _, m := range mappings {
		if m.SourceAttribute != "" {
			attrSet[m.SourceAttribute] = true
		}
	}

	// 添加默认属性
	if conn.IsOpenLDAP() {
		defaults := []string{"uid", "cn", "mail", "mobile", "memberOf"}
		for _, attr := range defaults {
			attrSet[attr] = true
		}
	} else {
		defaults := []string{"sAMAccountName", "cn", "mail", "mobile", "telephoneNumber", "userPrincipalName", "memberOf", "userAccountControl"}
		for _, attr := range defaults {
			attrSet[attr] = true
		}
	}

	attrs := make([]string, 0, len(attrSet))
	for attr := range attrSet {
		attrs = append(attrs, attr)
	}
	return attrs
}

// processLDAPEntry 处理单个 LDAP 条目
func processLDAPEntry(conn models.Connector, rule models.SyncRule, entry *ldapv3.Entry, mappings []models.SyncAttributeMapping) UpstreamDetail {
	detail := UpstreamDetail{
		RemoteUID: entry.DN,
	}

	// 根据映射提取字段值
	fieldValues := extractLDAPFieldValues(conn, entry, mappings)

	username := fieldValues["username"]
	if username == "" {
		detail.Action = "skipped"
		detail.Message = "用户名为空"
		return detail
	}

	detail.RemoteName = fieldValues["nickname"]
	if detail.RemoteName == "" {
		detail.RemoteName = username
	}

	// 从 DN 中提取 OU 层级作为群组路径（如果映射中没有显式指定 group_path）
	if fieldValues["group_path"] == "" && fieldValues["groups"] == "" {
		if ouPath := extractOUPathFromDN(entry.DN, conn.BaseDN); ouPath != "" {
			fieldValues["group_path"] = ouPath
		}
	}

	// 查找或创建本地用户
	result := upsertLocalUser(conn, rule, username, fieldValues)

	// 用户创建/更新/跳过后，都尝试同步群组归属（用户已存在但群组未关联的情况）
	if result.Action != "failed" {
		if gp := fieldValues["group_path"]; gp != "" {
			syncUserGroupsByPath(username, gp)
		} else if gn := fieldValues["groups"]; gn != "" {
			syncUserGroups(username, strings.Split(gn, ","))
		}
	}

	return result
}

// extractOUPathFromDN 从 LDAP DN 中提取 OU 层级路径。
// 例如 "uid=zhangsan,ou=技术部,ou=研发中心,dc=example,dc=com" + baseDN "dc=example,dc=com"
// 返回 "研发中心/技术部"（从根到叶的正序路径）。
func extractOUPathFromDN(dn, baseDN string) string {
	// 移除 baseDN 后缀，得到相对 DN
	rel := dn
	if baseDN != "" {
		idx := strings.Index(strings.ToLower(dn), strings.ToLower(","+baseDN))
		if idx >= 0 {
			rel = dn[:idx]
		}
	}

	// 解析 RDN 组件，提取 OU 部分
	parts := strings.Split(rel, ",")
	var ous []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		lower := strings.ToLower(part)
		if strings.HasPrefix(lower, "ou=") {
			ous = append(ous, part[3:])
		}
	}

	if len(ous) == 0 {
		return ""
	}

	// DN 中 OU 是从叶到根排列，反转为从根到叶的路径
	for i, j := 0, len(ous)-1; i < j; i, j = i+1, j-1 {
		ous[i], ous[j] = ous[j], ous[i]
	}

	return strings.Join(ous, "/")
}

// extractLDAPFieldValues 从 LDAP 条目提取字段值
func extractLDAPFieldValues(conn models.Connector, entry *ldapv3.Entry, mappings []models.SyncAttributeMapping) map[string]string {
	values := make(map[string]string)

	for _, m := range mappings {
		if m.SourceAttribute != "" && m.TargetAttribute != "" {
			// 跳过密码相关属性映射（上游同步不支持密码映射，统一通过密码代理学习）
			if m.SourceAttribute == "userPassword" || strings.HasSuffix(m.TargetAttribute, "password") {
				continue
			}
			val := entry.GetAttributeValue(m.SourceAttribute)
			values[m.TargetAttribute] = val
		}
	}

	// 如果没有映射，使用默认属性
	if values["username"] == "" {
		if conn.IsOpenLDAP() {
			loginAttr := conn.LDAPLoginAttr
			if loginAttr == "" {
				loginAttr = "uid"
			}
			values["username"] = entry.GetAttributeValue(loginAttr)
		} else {
			values["username"] = entry.GetAttributeValue("sAMAccountName")
			if values["username"] == "" {
				upn := entry.GetAttributeValue("userPrincipalName")
				if upn != "" {
					parts := strings.Split(upn, "@")
					values["username"] = parts[0]
				}
			}
		}
	}

	if values["nickname"] == "" {
		if conn.IsOpenLDAP() {
			nameAttr := conn.LDAPNameAttr
			if nameAttr == "" {
				nameAttr = "cn"
			}
			values["nickname"] = entry.GetAttributeValue(nameAttr)
		} else {
			values["nickname"] = entry.GetAttributeValue("cn")
		}
	}

	if values["email"] == "" {
		if conn.IsOpenLDAP() {
			emailAttr := conn.LDAPEmailAttr
			if emailAttr == "" {
				emailAttr = "mail"
			}
			values["email"] = entry.GetAttributeValue(emailAttr)
		} else {
			values["email"] = entry.GetAttributeValue("mail")
		}
	}

	if values["phone"] == "" {
		if conn.IsOpenLDAP() {
			mobileAttr := conn.LDAPMobileAttr
			if mobileAttr == "" {
				mobileAttr = "mobile"
			}
			values["phone"] = entry.GetAttributeValue(mobileAttr)
		} else {
			values["phone"] = entry.GetAttributeValue("mobile")
			if values["phone"] == "" {
				values["phone"] = entry.GetAttributeValue("telephoneNumber")
			}
		}
	}

	return values
}

// ========== 数据库上游同步 ==========

// SyncUsersFromDatabase 从数据库同步用户到本地
func SyncUsersFromDatabase(conn models.Connector, rule models.SyncRule) UpstreamSyncResult {
	result := UpstreamSyncResult{}

	db, err := dialDB(conn)
	if err != nil {
		result.Error = "连接数据库失败: " + err.Error()
		return result
	}
	defer db.Close()

	if conn.UserTable == "" {
		result.Error = "未配置用户表名"
		return result
	}

	// 获取字段映射
	mappings := getUpstreamMappings(rule.ID, conn.Type)

	// 构建字段映射 (target -> source)
	fieldMap := make(map[string]string)
	for _, m := range mappings {
		if m.SourceAttribute != "" && m.TargetAttribute != "" {
			fieldMap[m.TargetAttribute] = m.SourceAttribute
		}
	}

	// 默认映射
	if fieldMap["username"] == "" {
		fieldMap["username"] = "username"
	}

	// 构建查询
	selectFields := []string{}
	for _, sourceField := range fieldMap {
		selectFields = append(selectFields, sourceField)
	}

	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(selectFields, ", "), conn.UserTable)
	rows, err := db.Query(query)
	if err != nil {
		result.Error = "查询失败: " + err.Error()
		return result
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	colIndexMap := make(map[string]int)
	for i, col := range columns {
		colIndexMap[strings.ToLower(col)] = i
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		result.UsersTotal++

		// 提取字段值
		fieldValues := make(map[string]string)
		for targetField, sourceField := range fieldMap {
			if idx, ok := colIndexMap[strings.ToLower(sourceField)]; ok {
				if values[idx] != nil {
					fieldValues[targetField] = fmt.Sprintf("%v", values[idx])
				}
			}
		}

		username := fieldValues["username"]
		if username == "" {
			continue
		}

		detail := upsertLocalUser(conn, rule, username, fieldValues)
		result.Details = append(result.Details, detail)

		switch detail.Action {
		case "created":
			result.UsersCreated++
			var u models.User
			if storage.DB.Where("username = ? AND is_deleted = 0", username).First(&u).Error == nil {
				DispatchSyncEvent(models.SyncEventUserCreate, u.ID, fieldValues["password_raw"])
			}
		case "updated":
			result.UsersUpdated++
			var u models.User
			if storage.DB.Where("username = ? AND is_deleted = 0", username).First(&u).Error == nil {
				DispatchSyncEvent(models.SyncEventUserUpdate, u.ID, "")
			}
		}
	}

	return result
}

// ========== HTTP API 上游同步 ==========

// extractByPath 从嵌套 map 中按点分路径提取值（如 "data.list"）
func extractByPath(obj map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = obj
	for _, p := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[p]
	}
	return current
}

// extractListByPath 按路径提取用户列表
func extractListByPath(obj map[string]interface{}, path string) []map[string]interface{} {
	raw := extractByPath(obj, path)
	if raw == nil {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var result []map[string]interface{}
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

// extractIntByPath 按路径提取整数值
func extractIntByPath(obj map[string]interface{}, path string) int {
	raw := extractByPath(obj, path)
	if raw == nil {
		return 0
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// applyValueMapping 应用值映射转换，规则格式: "源值1=目标值1,源值2=目标值2"
// 例如: "true=1,false=0" 或 "enabled=1,disabled=0" 或 "success=1,fail=0"
func applyValueMapping(value string, rule string) string {
	pairs := strings.Split(rule, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == value {
			return strings.TrimSpace(kv[1])
		}
	}
	return value
}

// SyncUsersFromHTTPAPI 从 HTTP API 同步用户到本地
func SyncUsersFromHTTPAPI(conn models.Connector, rule models.SyncRule) UpstreamSyncResult {
	result := UpstreamSyncResult{}
	log.Printf("[HTTPAPISync] 开始同步, 连接器=%s, URL=%s, Method=%s, Timeout=%d", conn.Name, conn.HTTPURL, conn.HTTPMethod, conn.Timeout)

	if conn.HTTPURL == "" {
		result.Error = "HTTP API 连接器未配置接口地址"
		return result
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !conn.HTTPVerifyCert,
		},
	}

	timeout := conn.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeout) * time.Second,
	}

	mappings := getUpstreamMappings(rule.ID, conn.Type)

	// 分页配置
	paginationMode := conn.HTTPPaginationMode
	if paginationMode == "" {
		paginationMode = "page"
	}
	pageParam := conn.HTTPPageParam
	pageSizeParam := conn.HTTPPageSizeParam
	pageSize := conn.HTTPPageSize
	if pageSize <= 0 {
		pageSize = 200
	}

	switch paginationMode {
	case "offset":
		if pageParam == "" {
			pageParam = "firstResult"
		}
		if pageSizeParam == "" {
			pageSizeParam = "maxResult"
		}
	case "none":
		// 不分页
	default: // "page"
		if pageParam == "" {
			pageParam = "page"
		}
		if pageSizeParam == "" {
			pageSizeParam = "size"
		}
	}

	// 响应解析路径
	dataPath := conn.HTTPDataPath
	if dataPath == "" {
		dataPath = "data.list"
	}
	totalPath := conn.HTTPTotalPath
	if totalPath == "" {
		totalPath = "data.total"
	}

	page := 1
	offset := 0
	for {
		apiURL := conn.HTTPURL
		method := conn.HTTPMethod
		if method == "" {
			method = "GET"
		}

		// 构建分页参数字符串
		var paginationParams string
		if paginationMode != "none" {
			if paginationMode == "offset" {
				paginationParams = fmt.Sprintf("%s=%d&%s=%d", pageParam, offset, pageSizeParam, pageSize)
			} else {
				paginationParams = fmt.Sprintf("%s=%d&%s=%d", pageParam, page, pageSizeParam, pageSize)
			}
		}

		// POST/PUT: 分页参数追加到请求体; GET: 追加到 URL
		var reqBody io.Reader
		if method == "POST" || method == "PUT" {
			bodyStr := conn.HTTPBodyTemplate
			if paginationParams != "" {
				if bodyStr != "" {
					bodyStr += "&" + paginationParams
				} else {
					bodyStr = paginationParams
				}
			}
			if bodyStr != "" {
				reqBody = strings.NewReader(bodyStr)
			}
		} else {
			if paginationParams != "" {
				sep := "?"
				if strings.Contains(apiURL, "?") {
					sep = "&"
				}
				apiURL += sep + paginationParams
			}
		}

		req, err := http.NewRequest(method, apiURL, reqBody)
		if err != nil {
			result.Error = "创建请求失败: " + err.Error()
			return result
		}

		contentType := conn.HTTPContentType
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)

		if conn.HTTPHeaders != "" {
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

		log.Printf("[HTTPAPISync] 请求: %s %s", method, apiURL)
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[HTTPAPISync] 请求失败: %v", err)
			result.Error = "请求上游 API 失败: " + err.Error()
			return result
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		log.Printf("[HTTPAPISync] 响应状态: %d, 长度: %d", resp.StatusCode, len(body))

		if resp.StatusCode != http.StatusOK {
			result.Error = fmt.Sprintf("上游 API 返回 HTTP %d: %s", resp.StatusCode, string(body))
			return result
		}

		var rawResp map[string]interface{}
		if err := json.Unmarshal(body, &rawResp); err != nil {
			result.Error = "解析上游 API 响应失败: " + err.Error()
			return result
		}

		// 检查 success 字段
		if s, ok := rawResp["success"]; ok {
			if sb, ok := s.(bool); ok && !sb {
				msg := "未知错误"
				if m, ok := rawResp["message"]; ok && m != nil {
					msg = fmt.Sprintf("%v", m)
				}
				result.Error = "上游 API 返回失败: " + msg
				return result
			}
		}

		// 按配置路径提取数据列表和总数
		userList := extractListByPath(rawResp, dataPath)
		total := extractIntByPath(rawResp, totalPath)

		log.Printf("[HTTPAPISync] 解析结果: total=%d, list_count=%d (dataPath=%s, totalPath=%s)", total, len(userList), dataPath, totalPath)

		if len(userList) == 0 {
			log.Printf("[HTTPAPISync] 没有更多数据, 退出分页循环")
			break
		}

		for _, userData := range userList {
			result.UsersTotal++

			fieldValues := make(map[string]string)
			for _, m := range mappings {
				src := m.SourceAttribute
				tgt := m.TargetAttribute
				if val, ok := userData[src]; ok {
					var rawStr string
					switch v := val.(type) {
					case string:
						rawStr = v
					case []interface{}:
						var parts []string
						for _, item := range v {
							if s, ok := item.(string); ok && s != "" {
								parts = append(parts, s)
							}
						}
						rawStr = strings.Join(parts, ",")
					case bool:
						rawStr = fmt.Sprintf("%v", v)
					case float64:
						if v == float64(int64(v)) {
							rawStr = fmt.Sprintf("%d", int64(v))
						} else {
							rawStr = fmt.Sprintf("%v", v)
						}
					default:
						if v != nil {
							rawStr = fmt.Sprintf("%v", val)
						}
					}
				// 应用值映射转换规则（如 "true=1,false=0"）
				if m.TransformRule != "" {
					rawStr = applyValueMapping(rawStr, m.TransformRule)
				}
					fieldValues[tgt] = rawStr
				}
			}

			if fieldValues["username"] == "" {
				if val, ok := userData["username"]; ok {
					fieldValues["username"] = fmt.Sprintf("%v", val)
				}
			}

			username := fieldValues["username"]
			if username == "" {
				continue
			}

			detail := upsertLocalUser(conn, rule, username, fieldValues)
			result.Details = append(result.Details, detail)

			if detail.Action != "failed" {
				if gp := fieldValues["group_path"]; gp != "" {
					syncUserGroupsByPath(username, gp)
				} else if gn := fieldValues["groups"]; gn != "" {
					syncUserGroups(username, strings.Split(gn, ","))
				}
				if rn := fieldValues["roles"]; rn != "" {
					syncUserRoles(username, strings.Split(rn, ","))
				}
			}

			switch detail.Action {
			case "created":
				result.UsersCreated++
				var u models.User
				if storage.DB.Where("username = ? AND is_deleted = 0", username).First(&u).Error == nil {
					log.Printf("[上游同步] 用户 %s 创建成功(id=%d)，群组和角色已分配，触发下游事件同步", username, u.ID)
					DispatchSyncEvent(models.SyncEventUserCreate, u.ID, fieldValues["password_raw"])
				}
			case "updated":
				result.UsersUpdated++
				var u models.User
				if storage.DB.Where("username = ? AND is_deleted = 0", username).First(&u).Error == nil {
					log.Printf("[上游同步] 用户 %s 更新成功(id=%d)，触发下游事件同步", username, u.ID)
					DispatchSyncEvent(models.SyncEventUserUpdate, u.ID, "")
				}
			}
		}

		// 分页终止判断
		if paginationMode == "none" {
			break
		}
		if len(userList) < pageSize || (total > 0 && result.UsersTotal >= total) {
			break
		}
		page++
		offset += pageSize
	}

	return result
}

// syncUserGroups 根据群组名称列表，自动创建群组并关联用户
func syncUserGroups(username string, groupNames []string) {
	if len(groupNames) == 0 {
		return
	}

	var user models.User
	if storage.DB.Where("username = ? AND is_deleted = 0", username).First(&user).Error != nil {
		return
	}

	// 找到根群组
	var rootGroup models.UserGroup
	storage.DB.Where("parent_id = 0 OR parent_id IS NULL").Order("id asc").First(&rootGroup)
	rootID := rootGroup.ID

	// 用第一个群组名称作为用户的主群组
	primaryGroupName := groupNames[0]

	var group models.UserGroup
	if storage.DB.Where("name = ?", primaryGroupName).First(&group).Error != nil {
		group = models.UserGroup{
			Name:     primaryGroupName,
			ParentID: rootID,
		}
		if err := storage.DB.Create(&group).Error; err != nil {
			log.Printf("[HTTPAPISync] 创建群组 %s 失败: %v", primaryGroupName, err)
			return
		}
		log.Printf("[HTTPAPISync] 自动创建群组: %s (id=%d)", primaryGroupName, group.ID)
	}

	if user.GroupID != group.ID {
		storage.DB.Model(&user).Update("group_id", group.ID)
	}
}

// syncUserGroupsByPath 根据群组路径（如 "广告业务/平台支持/创意中心"）逐级创建群组并关联用户
func syncUserGroupsByPath(username string, path string) {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return
	}

	var user models.User
	if storage.DB.Where("username = ? AND is_deleted = 0", username).First(&user).Error != nil {
		return
	}

	var parentID uint
	var lastGroup models.UserGroup

	for _, name := range parts {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		var group models.UserGroup
		if storage.DB.Where("name = ? AND parent_id = ?", name, parentID).First(&group).Error != nil {
			group = models.UserGroup{
				Name:     name,
				ParentID: parentID,
			}
			if err := storage.DB.Create(&group).Error; err != nil {
				log.Printf("[HTTPAPISync] 创建群组 %s (parent=%d) 失败: %v", name, parentID, err)
				return
			}
		}
		parentID = group.ID
		lastGroup = group
	}

	if lastGroup.ID > 0 && user.GroupID != lastGroup.ID {
		storage.DB.Model(&user).Update("group_id", lastGroup.ID)
	}
}

// syncUserRoles 根据角色名称列表，自动查找或创建角色并关联用户
func syncUserRoles(username string, roleNames []string) {
	if len(roleNames) == 0 {
		return
	}

	var user models.User
	if storage.DB.Where("username = ? AND is_deleted = 0", username).First(&user).Error != nil {
		return
	}

	// 收集需要关联的角色ID
	var roleIDs []uint
	for _, name := range roleNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		var role models.Role
		if storage.DB.Where("name = ? OR code = ?", name, name).First(&role).Error != nil {
			// 角色不存在，自动创建
			role = models.Role{
				Name:   name,
				Code:   strings.ToLower(strings.ReplaceAll(name, " ", "_")),
				Status: 1,
			}
			if err := storage.DB.Create(&role).Error; err != nil {
				log.Printf("[HTTPAPISync] 创建角色 %s 失败: %v", name, err)
				continue
			}
			log.Printf("[HTTPAPISync] 自动创建角色: %s (id=%d)", name, role.ID)
		}
		roleIDs = append(roleIDs, role.ID)
	}

	if len(roleIDs) == 0 {
		return
	}

	// 获取用户当前的角色ID列表
	var existingRoleIDs []uint
	storage.DB.Model(&models.UserRole{}).Where("user_id = ?", user.ID).Pluck("role_id", &existingRoleIDs)
	existingSet := make(map[uint]bool)
	for _, id := range existingRoleIDs {
		existingSet[id] = true
	}

	// 添加缺失的角色关联
	for _, roleID := range roleIDs {
		if !existingSet[roleID] {
			storage.DB.Create(&models.UserRole{UserID: user.ID, RoleID: roleID})
		}
	}
}

// ========== 通用函数 ==========

// getUpstreamMappings 获取上游同步的字段映射
func getUpstreamMappings(ruleID uint, connType string) []models.SyncAttributeMapping {
	var mappings []models.SyncAttributeMapping
	storage.DB.Where("sync_rule_id = ? AND object_type = 'user' AND is_enabled = true", ruleID).
		Order("priority DESC").Find(&mappings)

	// 如果没有映射，返回默认映射
	if len(mappings) == 0 {
		mappings = GetDefaultUpstreamMappings(connType)
	}

	return mappings
}

// GetDefaultUpstreamMappings 获取默认的上游字段映射
func GetDefaultUpstreamMappings(connType string) []models.SyncAttributeMapping {
	mappings := []models.SyncAttributeMapping{}

	switch {
	case strings.HasPrefix(connType, "ldap_"):
		if connType == "ldap_openldap" {
			mappings = []models.SyncAttributeMapping{
				{ObjectType: "user", SourceAttribute: "uid", TargetAttribute: "username", MappingType: "mapping", Priority: 100, IsEnabled: true},
				{ObjectType: "user", SourceAttribute: "cn", TargetAttribute: "nickname", MappingType: "mapping", Priority: 90, IsEnabled: true},
				{ObjectType: "user", SourceAttribute: "mail", TargetAttribute: "email", MappingType: "mapping", Priority: 80, IsEnabled: true},
				{ObjectType: "user", SourceAttribute: "mobile", TargetAttribute: "phone", MappingType: "mapping", Priority: 70, IsEnabled: true},
				{ObjectType: "user", SourceAttribute: "title", TargetAttribute: "jobTitle", MappingType: "mapping", Priority: 60, IsEnabled: true},
				// 上游同步不支持密码映射，密码通过密码代理学习
			}
		} else {
			// AD
			mappings = []models.SyncAttributeMapping{
				{ObjectType: "user", SourceAttribute: "sAMAccountName", TargetAttribute: "username", MappingType: "mapping", Priority: 100, IsEnabled: true},
				{ObjectType: "user", SourceAttribute: "cn", TargetAttribute: "nickname", MappingType: "mapping", Priority: 90, IsEnabled: true},
				{ObjectType: "user", SourceAttribute: "mail", TargetAttribute: "email", MappingType: "mapping", Priority: 80, IsEnabled: true},
				{ObjectType: "user", SourceAttribute: "mobile", TargetAttribute: "phone", MappingType: "mapping", Priority: 70, IsEnabled: true},
				{ObjectType: "user", SourceAttribute: "title", TargetAttribute: "jobTitle", MappingType: "mapping", Priority: 60, IsEnabled: true},
				{ObjectType: "user", SourceAttribute: "department", TargetAttribute: "departmentName", MappingType: "mapping", Priority: 50, IsEnabled: true},
			}
		}
	case strings.HasPrefix(connType, "db_") || connType == "database":
		// 上游同步不支持密码映射，密码通过密码代理学习
		mappings = []models.SyncAttributeMapping{
			{ObjectType: "user", SourceAttribute: "username", TargetAttribute: "username", MappingType: "mapping", Priority: 100, IsEnabled: true},
			{ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "nickname", MappingType: "mapping", Priority: 90, IsEnabled: true},
			{ObjectType: "user", SourceAttribute: "name", TargetAttribute: "nickname", MappingType: "mapping", Priority: 89, IsEnabled: true},
			{ObjectType: "user", SourceAttribute: "email", TargetAttribute: "email", MappingType: "mapping", Priority: 80, IsEnabled: true},
			{ObjectType: "user", SourceAttribute: "phone", TargetAttribute: "phone", MappingType: "mapping", Priority: 70, IsEnabled: true},
			{ObjectType: "user", SourceAttribute: "mobile", TargetAttribute: "phone", MappingType: "mapping", Priority: 69, IsEnabled: true},
		}
	case connType == "http_api":
		mappings = []models.SyncAttributeMapping{
			{ObjectType: "user", SourceAttribute: "username", TargetAttribute: "username", MappingType: "mapping", Priority: 100, IsEnabled: true},
			{ObjectType: "user", SourceAttribute: "nickname", TargetAttribute: "nickname", MappingType: "mapping", Priority: 90, IsEnabled: true},
			{ObjectType: "user", SourceAttribute: "email", TargetAttribute: "email", MappingType: "mapping", Priority: 80, IsEnabled: true},
			{ObjectType: "user", SourceAttribute: "phone", TargetAttribute: "phone", MappingType: "mapping", Priority: 70, IsEnabled: true},
			{ObjectType: "user", SourceAttribute: "password_hash", TargetAttribute: "password_hash", MappingType: "mapping", Priority: 60, IsEnabled: true},
			{ObjectType: "user", SourceAttribute: "samba_nt_password", TargetAttribute: "samba_nt_password", MappingType: "mapping", Priority: 55, IsEnabled: true},
			{ObjectType: "user", SourceAttribute: "group_path", TargetAttribute: "group_path", MappingType: "mapping", Priority: 50, IsEnabled: true},
			{ObjectType: "user", SourceAttribute: "groups", TargetAttribute: "groups", MappingType: "mapping", Priority: 45, IsEnabled: true},
		}
	}

	return mappings
}

// targetToDBColumn 映射目标属性名 -> 数据库列名/User结构体字段
// 这是所有支持通过映射规则同步的字段白名单
var targetToDBColumn = map[string]string{
	"nickname":            "nickname",
	"email":               "email",
	"phone":               "phone",
	"jobTitle":            "job_title",
	"job_title":           "job_title",
	"avatar":              "avatar",
	"department_name":     "department_name",
	"departmentName":      "department_name",
	"department_path":     "department_path",
	"departmentPath":      "department_path",
	"password_hash":       "password",
	"password":            "password",
	"samba_nt_password":   "samba_nt_password",
	"sambaNTPassword":     "samba_nt_password",
	"samba_nt_hash":       "samba_nt_password",
	"source":              "source",
	"im_user_id":          "im_user_id",
	"imUserId":            "im_user_id",
	"dingtalk_uid":        "ding_talk_uid",
	"dingtalkUid":         "ding_talk_uid",
	"status":              "status",
}

// targetToStructField 映射目标属性名 -> User 结构体 setter
func applyFieldToUser(user *models.User, target string, value string) {
	switch target {
	case "nickname":
		user.Nickname = value
	case "email":
		user.Email = value
	case "phone":
		user.Phone = value
	case "jobTitle", "job_title":
		user.JobTitle = value
	case "avatar":
		user.Avatar = value
	case "departmentName", "department_name":
		user.DepartmentName = value
	case "departmentPath", "department_path":
		user.DepartmentPath = value
	case "password_hash", "password":
		user.Password = value
	case "samba_nt_password", "sambaNTPassword", "samba_nt_hash":
		user.SambaNTPassword = value
	case "source":
		user.Source = value
	case "imUserId", "im_user_id":
		user.IMUserID = value
	case "dingtalkUid", "dingtalk_uid":
		user.DingTalkUID = value
	case "status":
		if value == "1" || value == "true" {
			user.Status = 1
		} else if value == "0" || value == "false" {
			user.Status = 0
		}
	}
}

// getStructFieldValue 获取 User 结构体当前字段值
func getStructFieldValue(user *models.User, target string) string {
	switch target {
	case "nickname":
		return user.Nickname
	case "email":
		return user.Email
	case "phone":
		return user.Phone
	case "jobTitle", "job_title":
		return user.JobTitle
	case "avatar":
		return user.Avatar
	case "departmentName", "department_name":
		return user.DepartmentName
	case "departmentPath", "department_path":
		return user.DepartmentPath
	case "password_hash", "password":
		return user.Password
	case "samba_nt_password", "sambaNTPassword", "samba_nt_hash":
		return user.SambaNTPassword
	case "source":
		return user.Source
	case "imUserId", "im_user_id":
		return user.IMUserID
	case "status":
		return fmt.Sprintf("%d", user.Status)
	case "dingtalkUid", "dingtalk_uid":
		return user.DingTalkUID
	}
	return ""
}

// upsertLocalUser 创建或更新本地用户（完全由映射规则驱动）
func upsertLocalUser(conn models.Connector, rule models.SyncRule, username string, fieldValues map[string]string) UpstreamDetail {
	detail := UpstreamDetail{
		RemoteUID:  username,
		RemoteName: fieldValues["nickname"],
		LocalUser:  username,
	}
	if detail.RemoteName == "" {
		detail.RemoteName = username
	}

	// 不参与用户字段更新的映射目标（由上层单独处理）
	skipTargets := map[string]bool{
		"username": true, "groups": true, "group_path": true, "roles": true,
	}

	var user models.User
	err := storage.DB.Where("username = ? AND is_deleted = 0", username).First(&user).Error

	if err != nil {
		if !rule.AutoCreateUser {
			detail.Action = "skipped"
			detail.Message = "用户不存在且未启用自动创建"
			return detail
		}

		user = models.User{
			Username: username,
			Status:   1,
			Source:   getSourceByConnType(conn.Type),
		}

		for target, value := range fieldValues {
			if skipTargets[target] || value == "" {
				continue
			}
			applyFieldToUser(&user, target, value)
		}

		if user.Password == "" && conn.IMGeneratePassword {
			password := generateRandomPassword(12)
			hash, _ := bcrypt.GenerateFromPassword([]byte(hashSHA256(password)), bcrypt.DefaultCost)
			user.Password = string(hash)
			if user.SambaNTPassword == "" {
				user.SambaNTPassword = computeNTHash(password)
			}
		}

		if err := storage.DB.Create(&user).Error; err != nil {
			detail.Action = "failed"
			detail.Message = "创建用户失败: " + err.Error()
			return detail
		}

		if conn.IMDefaultRoleID > 0 {
			storage.DB.Create(&models.UserRole{UserID: user.ID, RoleID: conn.IMDefaultRoleID})
		}

		detail.Action = "created"
		detail.Message = "用户创建成功"
	} else {
		updates := map[string]interface{}{}

		for target, value := range fieldValues {
			if skipTargets[target] || value == "" {
				continue
			}
			dbCol, ok := targetToDBColumn[target]
			if !ok {
				continue
			}
			oldValue := getStructFieldValue(&user, target)
			if value != oldValue {
			if dbCol == "status" {
				var s int8
				if value == "1" || value == "true" || value == "enabled" || value == "active" {
					s = 1
				}
				updates[dbCol] = s
				} else {
					updates[dbCol] = value
				}
			}
		}

		if len(updates) > 0 {
			storage.DB.Model(&user).Updates(updates)
			detail.Action = "updated"
			detail.Message = fmt.Sprintf("更新了 %d 个字段", len(updates))
		} else {
			detail.Action = "skipped"
			detail.Message = "无需更新"
		}
	}

	return detail
}

// getSourceByConnType 根据连接器类型返回用户来源
func getSourceByConnType(connType string) string {
	switch {
	case strings.HasPrefix(connType, "ldap_"):
		return "ldap"
	case strings.HasPrefix(connType, "db_") || connType == "database":
		return "database"
	case connType == "http_api":
		return "http_api"
	default:
		return "sync"
	}
}

// generateRandomPassword 生成随机密码
func generateRandomPassword(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

// hashSHA256 计算 SHA256 哈希
func hashSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// 上游同步支持两种密码方式：
// 1. 属性映射：映射 password_hash（bcrypt）和 samba_nt_password（NT Hash）直接同步密码哈希
// 2. 密码代理学习：不映射密码字段，通过密码代理认证后自动学习

// CreateDefaultUpstreamMappings 为同步规则创建默认映射
func CreateDefaultUpstreamMappings(ruleID uint, connType string) error {
	defaults := GetDefaultUpstreamMappings(connType)

	for _, m := range defaults {
		m.SyncRuleID = ruleID
		if err := storage.DB.Create(&m).Error; err != nil {
			return err
		}
	}

	return nil
}

// computeNTHash 计算 NT Hash (Samba/AD): MD4(UTF16LE(password))
func computeNTHash(password string) string {
	encoded := utf16.Encode([]rune(password))
	buf := make([]byte, len(encoded)*2)
	for i, v := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	digest := md4(buf)
	return strings.ToUpper(hex.EncodeToString(digest[:]))
}

// md4 计算 MD4 哈希
func md4(data []byte) [16]byte {
	var digest [16]byte
	var block [16]uint32

	msgLen := uint64(len(data))
	data = append(data, 0x80)
	for (len(data)+8)%64 != 0 {
		data = append(data, 0x00)
	}
	lenBits := msgLen * 8
	data = append(data, byte(lenBits), byte(lenBits>>8), byte(lenBits>>16), byte(lenBits>>24),
		byte(lenBits>>32), byte(lenBits>>40), byte(lenBits>>48), byte(lenBits>>56))

	a, b, c, d := uint32(0x67452301), uint32(0xefcdab89), uint32(0x98badcfe), uint32(0x10325476)

	for i := 0; i < len(data); i += 64 {
		for j := 0; j < 16; j++ {
			block[j] = uint32(data[i+j*4]) | uint32(data[i+j*4+1])<<8 | uint32(data[i+j*4+2])<<16 | uint32(data[i+j*4+3])<<24
		}
		aa, bb, cc, dd := a, b, c, d
		for _, k := range []int{0, 4, 8, 12} {
			a = rotl32(a+((b&c)|(^b&d))+block[k], 3)
			d = rotl32(d+((a&b)|(^a&c))+block[k+1], 7)
			c = rotl32(c+((d&a)|(^d&b))+block[k+2], 11)
			b = rotl32(b+((c&d)|(^c&a))+block[k+3], 19)
		}
		for _, k := range []int{0, 1, 2, 3} {
			a = rotl32(a+((b&c)|(b&d)|(c&d))+block[k]+0x5a827999, 3)
			d = rotl32(d+((a&b)|(a&c)|(b&c))+block[k+4]+0x5a827999, 5)
			c = rotl32(c+((d&a)|(d&b)|(a&b))+block[k+8]+0x5a827999, 9)
			b = rotl32(b+((c&d)|(c&a)|(d&a))+block[k+12]+0x5a827999, 13)
		}
		for _, k := range []int{0, 2, 1, 3} {
			a = rotl32(a+(b^c^d)+block[k]+0x6ed9eba1, 3)
			d = rotl32(d+(a^b^c)+block[k+8]+0x6ed9eba1, 9)
			c = rotl32(c+(d^a^b)+block[k+4]+0x6ed9eba1, 11)
			b = rotl32(b+(c^d^a)+block[k+12]+0x6ed9eba1, 15)
		}
		a += aa
		b += bb
		c += cc
		d += dd
	}

	binary.LittleEndian.PutUint32(digest[0:], a)
	binary.LittleEndian.PutUint32(digest[4:], b)
	binary.LittleEndian.PutUint32(digest[8:], c)
	binary.LittleEndian.PutUint32(digest[12:], d)

	return digest
}

func rotl32(x uint32, n uint) uint32 {
	return (x << n) | (x >> (32 - n))
}
