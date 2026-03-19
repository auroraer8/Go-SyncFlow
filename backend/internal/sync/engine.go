package sync

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf16"

	ber "github.com/go-asn1-ber/asn1-ber"
	ldapv3 "github.com/go-ldap/ldap/v3"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/sijms/go-ora/v2"

	"go-syncflow/internal/crypto"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

// WebSocket 广播函数（由 handlers 包注入）
var BroadcastSyncLogFunc func(log interface{})
var BroadcastSyncEventFunc func(action, syncType, connectorName string, details interface{})

func broadcastSyncLog(syncLog models.SyncLog) {
	if BroadcastSyncLogFunc != nil {
		BroadcastSyncLogFunc(syncLog)
	}
}

func broadcastSyncEvent(action, syncType, connectorName string, details interface{}) {
	if BroadcastSyncEventFunc != nil {
		BroadcastSyncEventFunc(action, syncType, connectorName, details)
	}
}

// ========== 连接器测试 ==========

// TestADConnection 测试 AD LDAP 连接
func TestADConnection(conn models.Connector) (string, error) {
	l, err := dialLDAP(conn)
	if err != nil {
		return "", fmt.Errorf("连接失败: %v", err)
	}
	defer l.Close()

	// 测试 Bind
	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		return "", fmt.Errorf("认证失败: %v", err)
	}

	// 测试搜索 - 验证 BaseDN 有效
	_, err = l.Search(ldapv3.NewSearchRequest(
		conn.BaseDN,
		ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 0, 5, false,
		"(objectClass=*)", []string{"dn"}, nil,
	))
	if err != nil {
		return "", fmt.Errorf("搜索BaseDN失败: %v", err)
	}

	// 根据连接器类型确定用户过滤器
	userFilter := "(objectClass=user)" // AD 默认
	if conn.UserFilter != "" {
		userFilter = conn.UserFilter
	}

	// 统计用户数量（限制结果数量避免超时）
	userCount := countLDAPEntries(l, conn.BaseDN, userFilter)

	// 根据连接器类型确定组过滤器
	groupFilter := "(|(objectClass=group)(objectClass=organizationalUnit))" // AD 默认
	groupCount := countLDAPEntries(l, conn.BaseDN, groupFilter)

	return fmt.Sprintf("连接成功，BaseDN有效，找到 %d 个用户、%d 个组/OU", userCount, groupCount), nil
}

// TestOpenLDAPConnection 测试 OpenLDAP 连接
func TestOpenLDAPConnection(conn models.Connector) (string, error) {
	l, err := dialLDAP(conn)
	if err != nil {
		return "", fmt.Errorf("连接失败: %v", err)
	}
	defer l.Close()

	// 测试 Bind
	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		return "", fmt.Errorf("认证失败: %v", err)
	}

	// 测试搜索 - 验证 BaseDN 有效
	_, err = l.Search(ldapv3.NewSearchRequest(
		conn.BaseDN,
		ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 0, 5, false,
		"(objectClass=*)", []string{"dn"}, nil,
	))
	if err != nil {
		return "", fmt.Errorf("搜索BaseDN失败: %v", err)
	}

	// 使用配置的用户对象类，默认 inetOrgPerson
	userObjectClass := conn.LDAPUserObjectClass
	if userObjectClass == "" {
		userObjectClass = "inetOrgPerson"
	}
	userFilter := fmt.Sprintf("(objectClass=%s)", userObjectClass)
	if conn.UserFilter != "" {
		userFilter = conn.UserFilter
	}

	// 使用配置的组对象类，默认 groupOfNames
	groupObjectClass := conn.LDAPGroupObjectClass
	if groupObjectClass == "" {
		groupObjectClass = "groupOfNames"
	}
	groupFilter := fmt.Sprintf("(|(objectClass=%s)(objectClass=organizationalUnit))", groupObjectClass)

	// 统计用户和组数量
	userCount := countLDAPEntries(l, conn.BaseDN, userFilter)
	groupCount := countLDAPEntries(l, conn.BaseDN, groupFilter)

	return fmt.Sprintf("连接成功，BaseDN有效，找到 %d 个用户、%d 个组/OU", userCount, groupCount), nil
}

// countLDAPEntries 统计 LDAP 条目数量，处理 Size Limit Exceeded 错误
func countLDAPEntries(l *ldapv3.Conn, baseDN, filter string) int {
	searchReq := ldapv3.NewSearchRequest(
		baseDN,
		ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 500, 0, false,
		filter, []string{"dn"}, nil,
	)
	
	sr, err := l.Search(searchReq)
	if err != nil {
		// 检查是否是 Size Limit Exceeded 错误
		if ldapErr, ok := err.(*ldapv3.Error); ok && ldapErr.ResultCode == ldapv3.LDAPResultSizeLimitExceeded {
			// 服务器有限制，但可能已返回部分结果
			if sr != nil {
				return len(sr.Entries)
			}
			// 如果没有结果，返回一个估计值
			return 500
		}
		return 0
	}
	return len(sr.Entries)
}

// TestMySQLConnection 测试 MySQL 连接
func TestMySQLConnection(conn models.Connector) (string, error) {
	return TestDBConnection(conn)
}

// TestDBConnection 通用数据库连接测试（支持 mysql/postgresql/oracle/sqlserver）
func TestDBConnection(conn models.Connector) (string, error) {
	dbType := conn.EffectiveDBType()
	db, err := dialDB(conn)
	if err != nil {
		return "", fmt.Errorf("连接失败: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return "", fmt.Errorf("Ping失败: %v", err)
	}

	info := []string{fmt.Sprintf("[%s] 连接成功", dbType)}
	if conn.UserTable != "" {
		if exists, cnt := checkTableForDB(db, dbType, conn.UserTable); exists {
			info = append(info, fmt.Sprintf("用户表[%s]存在(%d条)", conn.UserTable, cnt))
		} else {
			info = append(info, fmt.Sprintf("用户表[%s]不存在", conn.UserTable))
		}
	}
	if conn.GroupTable != "" {
		if exists, cnt := checkTableForDB(db, dbType, conn.GroupTable); exists {
			info = append(info, fmt.Sprintf("分组表[%s]存在(%d条)", conn.GroupTable, cnt))
		} else {
			info = append(info, fmt.Sprintf("分组表[%s]不存在", conn.GroupTable))
		}
	}
	if conn.RoleTable != "" {
		if exists, cnt := checkTableForDB(db, dbType, conn.RoleTable); exists {
			info = append(info, fmt.Sprintf("角色表[%s]存在(%d条)", conn.RoleTable, cnt))
		} else {
			info = append(info, fmt.Sprintf("角色表[%s]不存在", conn.RoleTable))
		}
	}

	return strings.Join(info, "；"), nil
}

// DiscoverMySQLColumns 兼容旧代码
func DiscoverMySQLColumns(conn models.Connector, tableName string) ([]map[string]string, error) {
	return DiscoverDBColumns(conn, tableName)
}

// DiscoverDBColumns 通用数据库字段发现（支持 mysql/postgresql/oracle/sqlserver）
func DiscoverDBColumns(conn models.Connector, tableName string) ([]map[string]string, error) {
	dbType := conn.EffectiveDBType()
	db, err := dialDB(conn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var query string
	var args []interface{}

	switch dbType {
	case "mysql":
		query = "SELECT COLUMN_NAME, DATA_TYPE, COLUMN_COMMENT, IS_NULLABLE, COLUMN_KEY FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION"
		args = []interface{}{conn.Database, tableName}
	case "postgresql":
		query = `SELECT c.column_name, c.data_type,
			COALESCE(pgd.description, '') as column_comment,
			c.is_nullable,
			CASE WHEN tc.constraint_type = 'PRIMARY KEY' THEN 'PRI' ELSE '' END as column_key
			FROM information_schema.columns c
			LEFT JOIN pg_catalog.pg_statio_all_tables st ON st.schemaname = c.table_schema AND st.relname = c.table_name
			LEFT JOIN pg_catalog.pg_description pgd ON pgd.objoid = st.relid AND pgd.objsubid = c.ordinal_position
			LEFT JOIN information_schema.key_column_usage kcu ON kcu.table_schema = c.table_schema AND kcu.table_name = c.table_name AND kcu.column_name = c.column_name
			LEFT JOIN information_schema.table_constraints tc ON tc.constraint_name = kcu.constraint_name AND tc.constraint_type = 'PRIMARY KEY'
			WHERE c.table_schema = 'public' AND c.table_name = $1
			ORDER BY c.ordinal_position`
		args = []interface{}{tableName}
	case "oracle":
		query = `SELECT c.COLUMN_NAME, c.DATA_TYPE,
			NVL(cc.COMMENTS, ' ') as COLUMN_COMMENT,
			c.NULLABLE as IS_NULLABLE,
			NVL2(p.COLUMN_NAME, 'PRI', ' ') as COLUMN_KEY
			FROM USER_TAB_COLUMNS c
			LEFT JOIN USER_COL_COMMENTS cc ON cc.TABLE_NAME = c.TABLE_NAME AND cc.COLUMN_NAME = c.COLUMN_NAME
			LEFT JOIN (SELECT cols.COLUMN_NAME FROM USER_CONSTRAINTS cons JOIN USER_CONS_COLUMNS cols ON cons.CONSTRAINT_NAME = cols.CONSTRAINT_NAME WHERE cons.CONSTRAINT_TYPE = 'P' AND cons.TABLE_NAME = :1) p ON p.COLUMN_NAME = c.COLUMN_NAME
			WHERE c.TABLE_NAME = :2
			ORDER BY c.COLUMN_ID`
		args = []interface{}{strings.ToUpper(tableName), strings.ToUpper(tableName)}
	case "sqlserver":
		query = `SELECT c.COLUMN_NAME, c.DATA_TYPE,
			ISNULL(CAST(ep.value AS NVARCHAR(500)), '') as COLUMN_COMMENT,
			c.IS_NULLABLE,
			CASE WHEN pk.COLUMN_NAME IS NOT NULL THEN 'PRI' ELSE '' END as COLUMN_KEY
			FROM INFORMATION_SCHEMA.COLUMNS c
			LEFT JOIN sys.extended_properties ep ON ep.major_id = OBJECT_ID(c.TABLE_SCHEMA + '.' + c.TABLE_NAME) AND ep.minor_id = c.ORDINAL_POSITION AND ep.name = 'MS_Description'
			LEFT JOIN (SELECT ku.COLUMN_NAME FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ku ON tc.CONSTRAINT_NAME = ku.CONSTRAINT_NAME WHERE tc.CONSTRAINT_TYPE = 'PRIMARY KEY' AND tc.TABLE_NAME = @p1) pk ON pk.COLUMN_NAME = c.COLUMN_NAME
			WHERE c.TABLE_NAME = @p2
			ORDER BY c.ORDINAL_POSITION`
		args = []interface{}{tableName, tableName}
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询字段信息失败: %v", err)
	}
	defer rows.Close()

	var columns []map[string]string
	for rows.Next() {
		var name, dataType, comment, nullable, key string
		rows.Scan(&name, &dataType, &comment, &nullable, &key)
		columns = append(columns, map[string]string{
			"name":     name,
			"type":     dataType,
			"comment":  comment,
			"nullable": nullable,
			"key":      key,
		})
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("表 %s 不存在或没有字段", tableName)
	}

	return columns, nil
}

// ========== 同步执行 ==========

// SyncResult 同步结果
type SyncResult struct {
	Success  int      `json:"success"`
	Failed   int      `json:"failed"`
	Skipped  int      `json:"skipped"`
	Total    int      `json:"total"`
	Errors   []string `json:"errors"`
	Duration int64    `json:"duration"`
	Created  []string `json:"created,omitempty"`
	Updated  []string `json:"updated,omitempty"`
	Deleted  []string `json:"deleted,omitempty"`
	Disabled []string `json:"disabled,omitempty"`
}

// ExecuteSync 执行同步（单用户 - 事件触发）
func ExecuteSync(syncr models.Synchronizer, user models.User, event string, rawPassword string) {
	start := time.Now()
	var conn models.Connector
	if err := storage.DB.First(&conn, syncr.ConnectorID).Error; err != nil {
		logSync(syncr.ID, syncr.ConnectorID, "下游", "event", event, user.ID, user.Username, "failed", "连接器不存在", 0, time.Since(start).Milliseconds())
		return
	}

	var result SyncResult
	switch {
	case conn.Type == "ldap_ad":
		result = syncUserToAD(conn, syncr, user, event, rawPassword)
	case conn.Type == "ldap_generic":
		result = syncUserToGenericLDAP(conn, syncr, user, event, rawPassword)
	case conn.Type == "mysql" || conn.IsDatabase():
		result = syncUserToDB(conn, syncr, user, event, rawPassword)
	default:
		logSync(syncr.ID, conn.ID, "下游", "event", event, user.ID, user.Username, "failed", "不支持的连接器类型: "+conn.Type, 0, time.Since(start).Milliseconds())
		return
	}

	status := "success"
	if result.Failed > 0 {
		status = "partial"
		if result.Success == 0 {
			status = "failed"
		}
	}

	msg := fmt.Sprintf("成功:%d, 失败:%d", result.Success, result.Failed)
	if len(result.Errors) > 0 {
		msg += "; " + strings.Join(result.Errors, "; ")
	}

	// 事件触发的同步日志加入用户昵称和事件描述
	detail := ""
	eventDesc := getEventDescription(event)
	if user.Nickname != "" {
		detail = fmt.Sprintf("用户: %s(%s)\n事件: %s\n目标: %s (%s)", 
			user.Nickname, user.Username, eventDesc, conn.Name, conn.ConnectorTypeName())
	} else {
		detail = fmt.Sprintf("用户: %s\n事件: %s\n目标: %s (%s)", 
			user.Username, eventDesc, conn.Name, conn.ConnectorTypeName())
	}
	if changeDetail := formatChangeDetail(result); changeDetail != "" {
		detail += "\n" + changeDetail
	}
	if len(result.Errors) > 0 {
		detail += "\n错误: " + strings.Join(result.Errors, "; ")
	}

	logSyncWithDetail(syncr.ID, conn.ID, "下游", "event", event, user.ID, user.Username, status, msg, detail, result.Success, time.Since(start).Milliseconds())
}

// getEventDescription 获取事件的中文描述
func getEventDescription(event string) string {
	eventMap := map[string]string{
		"user_create":        "用户创建",
		"user_update":        "用户信息更新",
		"user_delete":        "用户删除",
		"password_change":    "密码修改",
		"user_status_change": "用户状态变更",
		"role_change":        "角色变更",
		"full_sync":          "全量同步",
	}
	if desc, ok := eventMap[event]; ok {
		return desc
	}
	return event
}

// ExecuteFullSync 全量同步（定时/手动）
func ExecuteFullSync(syncr models.Synchronizer, triggerType string) SyncResult {
	start := time.Now()
	var conn models.Connector
	if err := storage.DB.First(&conn, syncr.ConnectorID).Error; err != nil {
		return SyncResult{Errors: []string{"连接器不存在"}}
	}

	// 查询所有活跃用户
	var users []models.User
	storage.DB.Where("is_deleted = 0 AND status = 1").Preload("Roles").Find(&users)

	// 获取属性映射
	var mappings []models.SyncAttributeMapping
	storage.DB.Where("(synchronizer_id = ? OR sync_rule_id = ?) AND object_type = ? AND is_enabled = ?", syncr.ID, syncr.ID, "user", true).Order("priority").Find(&mappings)

	var result SyncResult
	result.Total = len(users)

	switch {
	case conn.Type == "ldap_ad":
		result = batchSyncUsersToAD(conn, syncr, users, mappings)
	case conn.Type == "ldap_generic":
		result = batchSyncUsersToGenericLDAP(conn, syncr, users, mappings)
	case conn.Type == "mysql" || conn.IsDatabase():
		for _, user := range users {
			r := syncUserToDB(conn, syncr, user, "full_sync", "")
			result.Success += r.Success
			result.Failed += r.Failed
			result.Errors = append(result.Errors, r.Errors...)
			result.Created = append(result.Created, r.Created...)
			result.Updated = append(result.Updated, r.Updated...)
			result.Deleted = append(result.Deleted, r.Deleted...)
			result.Disabled = append(result.Disabled, r.Disabled...)
		}
	}

	result.Duration = time.Since(start).Milliseconds()

	status := "success"
	if result.Failed > 0 {
		status = "partial"
	}
	if result.Success == 0 && result.Failed > 0 {
		status = "failed"
	}

	msg := fmt.Sprintf("全量同步完成: 总计%d, 成功%d, 失败%d", result.Total, result.Success, result.Failed)

	// 构建详情：变更明细 + 错误信息
	var detailBuilder strings.Builder

	if changeDetail := formatChangeDetail(result); changeDetail != "" {
		detailBuilder.WriteString(changeDetail)
		detailBuilder.WriteByte('\n')
	}

	if len(result.Errors) > 0 {
		detailBuilder.WriteString("--- 错误详情 ---\n")
		for _, errStr := range result.Errors {
			cleaned := strings.ReplaceAll(errStr, "\x00", "")
			cleaned = strings.ReplaceAll(cleaned, "\n\t", " ")
			cleaned = strings.ReplaceAll(cleaned, "\n", " ")
			cleaned = strings.TrimSpace(cleaned)
			detailBuilder.WriteString(cleaned)
			detailBuilder.WriteByte('\n')
		}
	}
	detail := detailBuilder.String()
	logSyncWithDetail(syncr.ID, conn.ID, "下游", triggerType, "full_sync", 0, "", status, msg, detail, result.Success, result.Duration)

	// 更新同步器状态
	storage.DB.Model(&models.Synchronizer{}).Where("id = ?", syncr.ID).Updates(map[string]interface{}{
		"last_sync_at":      time.Now(),
		"last_sync_status":  status,
		"last_sync_message": msg,
		"sync_count":        syncr.SyncCount + 1,
	})

	return result
}

// ========== AD 批量同步（共享连接）==========

// AD 中不允许在 Modify 操作中直接修改的属性
var adReadOnlyAttrs = map[string]bool{
	"cn": true, "sAMAccountName": true, "objectClass": true,
	"unicodePwd": true, "userAccountControl": true,
	"userPrincipalName": true, "distinguishedName": true,
	"objectGUID": true, "objectSid": true, "whenCreated": true,
	"whenChanged": true, "memberOf": true, "primaryGroupID": true,
}

// AD 中创建时也不应由映射设置的属性（由代码显式设置或与 DN 冲突）
var adCreateSkipAttrs = map[string]bool{
	"sAMAccountName": true, "objectClass": true,
	"unicodePwd": true, "userAccountControl": true,
	"userPrincipalName": true, "cn": true,
}

// ========== AD "用户不能更改密码" ACL 修改 ==========
// 原理：修改用户的 ntSecurityDescriptor，添加 DENY ACE
// 拒绝 SELF 和 EVERYONE 的 "Change Password" 扩展权限

// Change Password extended right GUID: {AB721A53-1E2F-11D0-9819-00AA0040529B} (little-endian)
var changePwdGUID = []byte{0x53, 0x1a, 0x72, 0xab, 0x2f, 0x1e, 0xd0, 0x11, 0x98, 0x19, 0x00, 0xaa, 0x00, 0x40, 0x52, 0x9b}

// SELF SID: S-1-5-10
var selfSID = []byte{0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05, 0x0a, 0x00, 0x00, 0x00}

// EVERYONE SID: S-1-1-0
var everyoneSID = []byte{0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}

// buildDenyChangePwdACE 构建一个 ACCESS_DENIED_OBJECT_ACE
func buildDenyChangePwdACE(sid []byte) []byte {
	aceSize := uint16(4 + 4 + 4 + 16 + len(sid)) // header + mask + flags + guid + sid
	ace := make([]byte, aceSize)
	ace[0] = 0x06 // ACCESS_DENIED_OBJECT_ACE_TYPE
	ace[1] = 0x00 // AceFlags
	binary.LittleEndian.PutUint16(ace[2:4], aceSize)
	binary.LittleEndian.PutUint32(ace[4:8], 0x00000100)  // ADS_RIGHT_DS_CONTROL_ACCESS
	binary.LittleEndian.PutUint32(ace[8:12], 0x00000001) // ACE_OBJECT_TYPE_PRESENT
	copy(ace[12:28], changePwdGUID)
	copy(ace[28:], sid)
	return ace
}

// newSDFlagsControl 创建 LDAP_SERVER_SD_FLAGS_OID 控制，请求 DACL
func newSDFlagsControl() ldapv3.Control {
	// OID: 1.2.840.113556.1.4.801, criticality: true
	// controlValue: BER SEQUENCE { INTEGER 4 } (DACL_SECURITY_INFORMATION)
	p := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "")
	p.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, int64(4), ""))
	return ldapv3.NewControlString("1.2.840.113556.1.4.801", true, string(p.Bytes()))
}

// adSetCannotChangePassword 设置或取消 AD 用户"不能更改密码"
// buildGroupOUPath 根据群组 ID 构建完整的 OU DN 路径
func buildGroupOUPath(groupID uint, baseDN string) string {
	var group models.UserGroup
	if storage.DB.First(&group, groupID).Error != nil {
		return ""
	}
	// 递归构建：从当前群组到根
	if group.ParentID > 0 {
		parentDN := buildGroupOUPath(group.ParentID, baseDN)
		if parentDN != "" {
			return fmt.Sprintf("ou=%s,%s", ldapv3.EscapeDN(group.Name), parentDN)
		}
	}
	return fmt.Sprintf("ou=%s,%s", ldapv3.EscapeDN(group.Name), baseDN)
}

// ensureOUExists 确保群组对应的 OU 在 AD 中存在（递归创建父级）
// 返回值: OU 的 DN
func ensureOUExists(l *ldapv3.Conn, groupID uint, baseDN string) string {
	var group models.UserGroup
	if storage.DB.First(&group, groupID).Error != nil {
		return ""
	}

	// 先确保父 OU 存在（递归）
	var parentDN string
	if group.ParentID > 0 {
		parentDN = ensureOUExists(l, group.ParentID, baseDN)
		if parentDN == "" {
			// 父级创建失败，无法继续
			return ""
		}
	} else {
		parentDN = baseDN
	}

	// 构建当前 OU 的 DN
	ouDN := fmt.Sprintf("ou=%s,%s", ldapv3.EscapeDN(group.Name), parentDN)

	// 检查是否已存在
	sr, err := l.Search(ldapv3.NewSearchRequest(
		ouDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 5, false,
		"(objectClass=*)", []string{"dn"}, nil,
	))
	if err == nil && len(sr.Entries) > 0 {
		return ouDN // 已存在
	}

	// 创建 OU
	addReq := ldapv3.NewAddRequest(ouDN, nil)
	addReq.Attribute("objectClass", []string{"top", "organizationalUnit"})
	addReq.Attribute("ou", []string{group.Name})
	addReq.Attribute("description", []string{fmt.Sprintf("部门: %s", group.Name)})
	if err := l.Add(addReq); err != nil {
		if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultEntryAlreadyExists) {
			return ouDN // 已存在（并发创建）
		}
		log.Printf("[同步] 自动创建OU失败 %s: %v", ouDN, err)
		return ""
	}
	log.Printf("[同步] 自动创建OU成功: %s", ouDN)
	return ouDN
}

// ========== 群组/OU 同步到下游 ==========

// SyncGroupToDownstream 同步群组变更到所有下游连接器
// event: group_create, group_update, group_delete
func SyncGroupToDownstream(group models.UserGroup, event string, oldName string, oldParentID uint) {
	// 获取所有启用的下游同步规则（direction='push'表示下游同步，status=1表示启用）
	var syncRules []models.Synchronizer
	storage.DB.Where("direction = 'push' AND status = ?", 1).Find(&syncRules)

	for _, syncr := range syncRules {
		// 检查同步器是否启用群组同步
		if !syncr.SyncGroups {
			continue
		}

		var conn models.Connector
		if err := storage.DB.First(&conn, syncr.ConnectorID).Error; err != nil {
			continue
		}

		switch conn.Type {
		case "ldap_ad":
			syncGroupToAD(conn, syncr, group, event, oldName, oldParentID)
		case "ldap_generic":
			syncGroupToGenericLDAP(conn, syncr, group, event, oldName, oldParentID)
		case "mysql", "postgresql", "oracle", "sqlserver":
			// 数据库类型的下游：同步到群组表
			if conn.GroupTable != "" {
				syncGroupToDB(conn, syncr, group, event)
			}
		}
	}
}

// syncGroupToAD 同步群组变更到 AD
func syncGroupToAD(conn models.Connector, syncr models.Synchronizer, group models.UserGroup, event string, oldName string, oldParentID uint) {
	l, err := dialLDAP(conn)
	if err != nil {
		log.Printf("[同步] AD连接失败: %v", err)
		return
	}
	defer l.Close()

	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		log.Printf("[同步] AD认证失败: %v", err)
		return
	}

	targetContainer := syncr.TargetContainer
	if targetContainer == "" {
		targetContainer = conn.BaseDN
	}

	switch event {
	case "group_create":
		// 创建 OU（确保父级存在）
		ouDN := ensureOUExists(l, group.ID, targetContainer)
		if ouDN != "" {
			log.Printf("[同步] 群组创建成功，OU: %s", ouDN)
		}

	case "group_update":
		// 群组更新（重命名或移动父级）
		// 1. 计算旧的 OU DN
		oldOUDN := buildGroupOUPathWithOverride(group.ID, targetContainer, oldName, oldParentID)
		if oldOUDN == "" {
			log.Printf("[同步] 无法计算旧的OU路径")
			return
		}

		// 2. 计算新的 OU DN
		newOUDN := buildGroupOUPath(group.ID, targetContainer)
		if newOUDN == "" {
			log.Printf("[同步] 无法计算新的OU路径")
			return
		}

		if strings.EqualFold(oldOUDN, newOUDN) {
			log.Printf("[同步] OU路径未变化，无需更新")
			return
		}

		// 3. 检查旧 OU 是否存在
		sr, _ := l.Search(ldapv3.NewSearchRequest(
			oldOUDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 5, false,
			"(objectClass=*)", []string{"dn"}, nil,
		))
		if sr == nil || len(sr.Entries) == 0 {
			// 旧 OU 不存在，直接创建新的
			ensureOUExists(l, group.ID, targetContainer)
			log.Printf("[同步] 旧OU不存在，已创建新OU: %s", newOUDN)
			return
		}

		// 4. 计算新的父 DN 和新的 RDN
		newParentDN := targetContainer
		if group.ParentID > 0 {
			newParentDN = buildGroupOUPath(group.ParentID, targetContainer)
			if newParentDN == "" {
				newParentDN = targetContainer
			}
			// 确保新父 OU 存在
			ensureOUExists(l, group.ParentID, targetContainer)
		}
		newRDN := fmt.Sprintf("ou=%s", ldapv3.EscapeDN(group.Name))

		// 5. 执行 ModifyDN（重命名/移动）
		modDNReq := ldapv3.NewModifyDNRequest(oldOUDN, newRDN, true, newParentDN)
		if err := l.ModifyDN(modDNReq); err != nil {
			log.Printf("[同步] OU更新失败 %s -> %s: %v", oldOUDN, newOUDN, err)
		} else {
			log.Printf("[同步] OU更新成功: %s -> %s", oldOUDN, newOUDN)
		}

	case "group_delete":
		// 删除 OU
		ouDN := buildGroupOUPath(group.ID, targetContainer)
		if ouDN == "" {
			return
		}

		// 检查 OU 是否存在
		sr, _ := l.Search(ldapv3.NewSearchRequest(
			ouDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 5, false,
			"(objectClass=*)", []string{"dn"}, nil,
		))
		if sr == nil || len(sr.Entries) == 0 {
			log.Printf("[同步] OU不存在，无需删除: %s", ouDN)
			return
		}

		// 计算父级 OU（用于移动子对象）
		parentOUDN := targetContainer
		if group.ParentID > 0 {
			parentOUDN = buildGroupOUPath(group.ParentID, targetContainer)
			if parentOUDN == "" {
				parentOUDN = targetContainer
			}
		}

		// 搜索 OU 下的所有子对象（用户、子OU等）
		subSR, _ := l.Search(ldapv3.NewSearchRequest(
			ouDN, ldapv3.ScopeSingleLevel, ldapv3.NeverDerefAliases, 0, 0, false,
			"(objectClass=*)", []string{"dn", "objectClass"}, nil,
		))

		if subSR != nil && len(subSR.Entries) > 0 {
			log.Printf("[同步] OU下有 %d 个子对象，尝试移动到父级: %s", len(subSR.Entries), parentOUDN)

			for _, entry := range subSR.Entries {
				childDN := entry.DN
				// 提取 RDN（cn=xxx 或 ou=xxx）
				parts := strings.SplitN(childDN, ",", 2)
				if len(parts) < 1 {
					continue
				}
				rdn := parts[0]

				// 移动到父级 OU
				modDNReq := ldapv3.NewModifyDNRequest(childDN, rdn, true, parentOUDN)
				if err := l.ModifyDN(modDNReq); err != nil {
					log.Printf("[同步] 移动子对象失败 %s -> %s: %v", childDN, parentOUDN, err)
					// 如果移动失败（如有嵌套子对象），跳过删除此 OU
					log.Printf("[同步] OU下有无法移动的对象，跳过删除: %s", ouDN)
					return
				}
				log.Printf("[同步] 已移动子对象: %s -> %s", childDN, parentOUDN)
			}
		}

		// 删除 OU
		delReq := ldapv3.NewDelRequest(ouDN, nil)
		if err := l.Del(delReq); err != nil {
			if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchObject) {
				log.Printf("[同步] OU删除失败 %s: %v", ouDN, err)
			}
		} else {
			log.Printf("[同步] OU删除成功: %s", ouDN)
		}
	}
}

// syncGroupToGenericLDAP 同步群组变更到通用 LDAP
func syncGroupToGenericLDAP(conn models.Connector, syncr models.Synchronizer, group models.UserGroup, event string, oldName string, oldParentID uint) {
	// 通用 LDAP 的处理逻辑与 AD 类似，但 objectClass 不同
	l, err := dialLDAP(conn)
	if err != nil {
		log.Printf("[同步] LDAP连接失败: %v", err)
		return
	}
	defer l.Close()

	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		log.Printf("[同步] LDAP认证失败: %v", err)
		return
	}

	targetContainer := syncr.TargetContainer
	if targetContainer == "" {
		targetContainer = conn.BaseDN
	}

	switch event {
	case "group_create":
		ouDN := buildGroupOUPath(group.ID, targetContainer)
		if ouDN == "" {
			return
		}
		// 先确保父级存在
		if group.ParentID > 0 {
			ensureOUExistsGenericLDAP(l, group.ParentID, targetContainer)
		}
		// 创建 OU
		addReq := ldapv3.NewAddRequest(ouDN, nil)
		addReq.Attribute("objectClass", []string{"top", "organizationalUnit"})
		addReq.Attribute("ou", []string{group.Name})
		if err := l.Add(addReq); err != nil {
			if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultEntryAlreadyExists) {
				log.Printf("[同步] LDAP OU创建失败 %s: %v", ouDN, err)
			}
		} else {
			log.Printf("[同步] LDAP OU创建成功: %s", ouDN)
		}

	case "group_update", "group_delete":
		// 与 AD 逻辑类似
		syncGroupToAD(conn, syncr, group, event, oldName, oldParentID)
	}
}

// ensureOUExistsGenericLDAP 确保通用 LDAP 的 OU 存在
func ensureOUExistsGenericLDAP(l *ldapv3.Conn, groupID uint, baseDN string) string {
	var group models.UserGroup
	if storage.DB.First(&group, groupID).Error != nil {
		return ""
	}

	var parentDN string
	if group.ParentID > 0 {
		parentDN = ensureOUExistsGenericLDAP(l, group.ParentID, baseDN)
		if parentDN == "" {
			return ""
		}
	} else {
		parentDN = baseDN
	}

	ouDN := fmt.Sprintf("ou=%s,%s", ldapv3.EscapeDN(group.Name), parentDN)

	sr, err := l.Search(ldapv3.NewSearchRequest(
		ouDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 5, false,
		"(objectClass=*)", []string{"dn"}, nil,
	))
	if err == nil && len(sr.Entries) > 0 {
		return ouDN
	}

	addReq := ldapv3.NewAddRequest(ouDN, nil)
	addReq.Attribute("objectClass", []string{"top", "organizationalUnit"})
	addReq.Attribute("ou", []string{group.Name})
	if err := l.Add(addReq); err != nil {
		if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultEntryAlreadyExists) {
			return ouDN
		}
		log.Printf("[同步] LDAP OU创建失败 %s: %v", ouDN, err)
		return ""
	}
	return ouDN
}

// buildGroupOUPathWithOverride 使用覆盖的名称和父ID构建 OU 路径（用于计算旧路径）
func buildGroupOUPathWithOverride(groupID uint, baseDN string, overrideName string, overrideParentID uint) string {
	var group models.UserGroup
	if storage.DB.First(&group, groupID).Error != nil {
		return ""
	}

	name := group.Name
	parentID := group.ParentID
	if overrideName != "" {
		name = overrideName
	}
	if overrideParentID > 0 || overrideParentID == 0 {
		// 使用覆盖的父ID（包括0表示根级）
		parentID = overrideParentID
	}

	if parentID > 0 {
		parentDN := buildGroupOUPath(parentID, baseDN)
		if parentDN != "" {
			return fmt.Sprintf("ou=%s,%s", ldapv3.EscapeDN(name), parentDN)
		}
	}
	return fmt.Sprintf("ou=%s,%s", ldapv3.EscapeDN(name), baseDN)
}

// adSyncUserStatus 显式同步用户启用/禁用状态到 AD
// 根据本地 user.Status 设置 userAccountControl：
//   status=1 → 66048 (NORMAL_ACCOUNT + DONT_EXPIRE_PASSWORD = 启用)
//   status=0 → 66050 (ACCOUNTDISABLE + NORMAL_ACCOUNT + DONT_EXPIRE_PASSWORD = 禁用)
func adSyncUserStatus(l *ldapv3.Conn, userDN string, user models.User) {
	uac := "66050" // 禁用
	statusLabel := "禁用"
	if user.Status == 1 {
		uac = "66048" // 启用
		statusLabel = "启用"
	}
	modUAC := ldapv3.NewModifyRequest(userDN, nil)
	modUAC.Replace("userAccountControl", []string{uac})
	if err := l.Modify(modUAC); err != nil {
		// 新创建的无密码用户无法启用，这是正常的 AD 限制
		log.Printf("[同步] [%s] 设置AD状态(%s)失败: %v", user.Username, statusLabel, err)
	} else {
		log.Printf("[同步] [%s] AD状态已设为: %s", user.Username, statusLabel)
	}
}

// ensureRoleGroupOU 确保安全组的专用 OU 存在，返回安全组应存放的父 DN
func ensureRoleGroupOU(l *ldapv3.Conn, roleGroupContainer string, targetContainer string) string {
	if roleGroupContainer == "" {
		return targetContainer
	}

	// 先尝试 cn= 前缀（AD 内置容器如 cn=Users 用 cn）
	cnDN := fmt.Sprintf("cn=%s,%s", ldapv3.EscapeDN(roleGroupContainer), targetContainer)
	sr, _ := l.Search(ldapv3.NewSearchRequest(
		cnDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 5, false,
		"(objectClass=*)", []string{"dn"}, nil,
	))
	if sr != nil && len(sr.Entries) > 0 {
		log.Printf("[同步] 复用已有容器: %s", cnDN)
		return cnDN
	}

	// 再尝试 ou= 前缀
	ouDN := fmt.Sprintf("ou=%s,%s", ldapv3.EscapeDN(roleGroupContainer), targetContainer)
	sr, _ = l.Search(ldapv3.NewSearchRequest(
		ouDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 5, false,
		"(objectClass=*)", []string{"dn"}, nil,
	))
	if sr != nil && len(sr.Entries) > 0 {
		log.Printf("[同步] 复用已有OU: %s", ouDN)
		return ouDN
	}

	// 都不存在，创建 ou=
	addReq := ldapv3.NewAddRequest(ouDN, nil)
	addReq.Attribute("objectClass", []string{"top", "organizationalUnit"})
	addReq.Attribute("ou", []string{roleGroupContainer})
	addReq.Attribute("description", []string{"角色安全组"})
	if err := l.Add(addReq); err != nil {
		if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultEntryAlreadyExists) {
			log.Printf("[同步] 创建安全组OU失败: %v", err)
			return targetContainer
		}
	} else {
		log.Printf("[同步] 安全组OU创建成功: %s", ouDN)
	}
	return ouDN
}

// adSyncUserRoles 同步用户角色到 AD 安全组（事件触发时使用）
func adSyncUserRoles(l *ldapv3.Conn, conn models.Connector, userDN string, roleGroupBaseDN string, user models.User) {
	if len(user.Roles) == 0 {
		return
	}
	for _, role := range user.Roles {
		groupDN := fmt.Sprintf("cn=%s,%s", ldapv3.EscapeDN(role.Name), roleGroupBaseDN)
		// 确保安全组存在
		sr, _ := l.Search(ldapv3.NewSearchRequest(
			groupDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 5, false,
			"(objectClass=*)", []string{"dn"}, nil,
		))
		if sr == nil || len(sr.Entries) == 0 {
			addReq := ldapv3.NewAddRequest(groupDN, nil)
			addReq.Attribute("objectClass", []string{"top", "group"})
			addReq.Attribute("cn", []string{role.Name})
			addReq.Attribute("sAMAccountName", []string{role.Name})
			addReq.Attribute("groupType", []string{"-2147483646"}) // 全局安全组
			addReq.Attribute("description", []string{fmt.Sprintf("角色: %s", role.Name)})
			if err := l.Add(addReq); err != nil {
				if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultEntryAlreadyExists) {
					log.Printf("[同步] [%s] 创建角色安全组失败 %s: %v", user.Username, role.Name, err)
					continue
				}
			}
		}
		// 将用户添加到安全组
		modReq := ldapv3.NewModifyRequest(groupDN, nil)
		modReq.Add("member", []string{userDN})
		if err := l.Modify(modReq); err != nil {
			if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultEntryAlreadyExists) &&
				!strings.Contains(err.Error(), "ENTRY_EXISTS") &&
				!strings.Contains(err.Error(), "already") {
				log.Printf("[同步] [%s] 添加到角色组 %s 失败: %v", user.Username, role.Name, err)
			}
		} else {
			log.Printf("[同步] [%s] 已添加到角色组: %s", user.Username, role.Name)
		}
	}
}

// prevent=true: 添加 DENY ACE（勾选"用户不能更改密码"）
// prevent=false: 移除 DENY ACE（取消勾选"用户不能更改密码"）
func adSetCannotChangePassword(l *ldapv3.Conn, userDN string, prevent bool) error {
	sdControl := newSDFlagsControl()

	// 读取当前安全描述符（仅 DACL 部分）
	searchReq := ldapv3.NewSearchRequest(
		userDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 10, false,
		"(objectClass=*)", []string{"nTSecurityDescriptor"}, []ldapv3.Control{sdControl},
	)
	sr, err := l.Search(searchReq)
	if err != nil || len(sr.Entries) == 0 {
		return fmt.Errorf("读取安全描述符失败: %v", err)
	}

	sdBytes := sr.Entries[0].GetRawAttributeValue("nTSecurityDescriptor")
	if len(sdBytes) < 20 {
		return fmt.Errorf("安全描述符数据无效(长度=%d)", len(sdBytes))
	}

	// 解析 SD 头部
	offsetDacl := binary.LittleEndian.Uint32(sdBytes[16:20])
	if offsetDacl == 0 || int(offsetDacl)+8 > len(sdBytes) {
		return fmt.Errorf("DACL偏移无效")
	}

	daclStart := int(offsetDacl)
	daclSize := int(binary.LittleEndian.Uint16(sdBytes[daclStart+2 : daclStart+4]))
	aceCount := int(binary.LittleEndian.Uint16(sdBytes[daclStart+4 : daclStart+6]))
	daclEnd := daclStart + daclSize

	// 扫描现有 ACE，记录 Deny Change Password ACE 的位置和信息
	type aceInfo struct {
		offset int
		size   int
	}
	hasDenySelf := false
	hasDenyEveryone := false
	var denyChangePwdACEs []aceInfo // 需要移除的 ACE

	pos := daclStart + 8
	for i := 0; i < aceCount && pos+4 <= daclEnd; i++ {
		aceType := sdBytes[pos]
		aceSize := int(binary.LittleEndian.Uint16(sdBytes[pos+2 : pos+4]))
		if aceSize < 4 {
			break
		}
		if aceType == 0x06 && aceSize >= 40 { // ACCESS_DENIED_OBJECT_ACE_TYPE
			if bytes.Equal(sdBytes[pos+12:pos+28], changePwdGUID) {
				sid := sdBytes[pos+28 : pos+aceSize]
				if bytes.Equal(sid, selfSID) {
					hasDenySelf = true
					denyChangePwdACEs = append(denyChangePwdACEs, aceInfo{pos, aceSize})
				} else if bytes.Equal(sid, everyoneSID) {
					hasDenyEveryone = true
					denyChangePwdACEs = append(denyChangePwdACEs, aceInfo{pos, aceSize})
				}
			}
		}
		pos += aceSize
	}

	if prevent {
		// === 添加 DENY ACE ===
		if hasDenySelf && hasDenyEveryone {
			return nil // 已设置，无需修改
		}

		var newACEs []byte
		newAceCount := aceCount
		if !hasDenySelf {
			newACEs = append(newACEs, buildDenyChangePwdACE(selfSID)...)
			newAceCount++
		}
		if !hasDenyEveryone {
			newACEs = append(newACEs, buildDenyChangePwdACE(everyoneSID)...)
			newAceCount++
		}

		// 构建新 DACL：DENY ACE 放在最前面（ACL header 之后）
		newDACL := make([]byte, 0, daclSize+len(newACEs))
		newDACL = append(newDACL, sdBytes[daclStart:daclStart+8]...) // ACL header
		newDACL = append(newDACL, newACEs...)                        // 新 DENY ACE
		newDACL = append(newDACL, sdBytes[daclStart+8:daclEnd]...)   // 原有 ACE

		// 更新 DACL header
		binary.LittleEndian.PutUint16(newDACL[2:4], uint16(len(newDACL)))
		binary.LittleEndian.PutUint16(newDACL[4:6], uint16(newAceCount))

		return writeNewSD(l, userDN, sdBytes, daclStart, daclEnd, offsetDacl, newDACL, len(newACEs), sdControl)
	} else {
		// === 移除 DENY ACE ===
		if !hasDenySelf && !hasDenyEveryone {
			return nil // 没有 DENY ACE，无需修改
		}

		// 构建新 DACL：跳过 Deny Change Password 的 ACE
		removedSize := 0
		for _, a := range denyChangePwdACEs {
			removedSize += a.size
		}

		newDACL := make([]byte, 0, daclSize-removedSize)
		newDACL = append(newDACL, sdBytes[daclStart:daclStart+8]...) // ACL header

		// 遍历原有 ACE，跳过要移除的
		removeSet := make(map[int]bool)
		for _, a := range denyChangePwdACEs {
			removeSet[a.offset] = true
		}
		p := daclStart + 8
		newAceCount := 0
		for i := 0; i < aceCount && p+4 <= daclEnd; i++ {
			sz := int(binary.LittleEndian.Uint16(sdBytes[p+2 : p+4]))
			if sz < 4 {
				break
			}
			if !removeSet[p] {
				newDACL = append(newDACL, sdBytes[p:p+sz]...)
				newAceCount++
			}
			p += sz
		}

		// 更新 DACL header
		binary.LittleEndian.PutUint16(newDACL[2:4], uint16(len(newDACL)))
		binary.LittleEndian.PutUint16(newDACL[4:6], uint16(newAceCount))

		return writeNewSD(l, userDN, sdBytes, daclStart, daclEnd, offsetDacl, newDACL, -removedSize, sdControl)
	}
}

// writeNewSD 将修改后的 DACL 写回 AD 安全描述符
func writeNewSD(l *ldapv3.Conn, userDN string, sdBytes []byte, daclStart, daclEnd int, offsetDacl uint32, newDACL []byte, sizeDiff int, sdControl ldapv3.Control) error {
	newSD := make([]byte, 0, len(sdBytes)+sizeDiff)
	newSD = append(newSD, sdBytes[:daclStart]...)
	newSD = append(newSD, newDACL...)
	newSD = append(newSD, sdBytes[daclEnd:]...)

	// 调整 SD 头部中在 DACL 之后的偏移量
	offsetOwner := binary.LittleEndian.Uint32(newSD[4:8])
	offsetGroup := binary.LittleEndian.Uint32(newSD[8:12])
	offsetSacl := binary.LittleEndian.Uint32(newSD[12:16])
	if offsetOwner > offsetDacl {
		binary.LittleEndian.PutUint32(newSD[4:8], uint32(int(offsetOwner)+sizeDiff))
	}
	if offsetGroup > offsetDacl {
		binary.LittleEndian.PutUint32(newSD[8:12], uint32(int(offsetGroup)+sizeDiff))
	}
	if offsetSacl > 0 && offsetSacl > offsetDacl {
		binary.LittleEndian.PutUint32(newSD[12:16], uint32(int(offsetSacl)+sizeDiff))
	}

	// 写回修改后的安全描述符
	modReq := ldapv3.NewModifyRequest(userDN, []ldapv3.Control{sdControl})
	modReq.Replace("nTSecurityDescriptor", []string{string(newSD)})
	return l.Modify(modReq)
}

func batchSyncUsersToAD(conn models.Connector, syncr models.Synchronizer, users []models.User, mappings []models.SyncAttributeMapping) SyncResult {
	result := SyncResult{Total: len(users)}

	l, err := dialLDAP(conn)
	if err != nil {
		result.Failed = len(users)
		result.Errors = append(result.Errors, fmt.Sprintf("LDAP连接失败: %v", err))
		return result
	}
	defer l.Close()

	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		result.Failed = len(users)
		result.Errors = append(result.Errors, fmt.Sprintf("LDAP认证失败: %v", err))
		return result
	}

	targetContainer := syncr.TargetContainer
	if targetContainer == "" {
		targetContainer = conn.BaseDN
	}

	// UPN 域名后缀
	upnSuffix := conn.UPNSuffix
	if upnSuffix == "" {
		// 从 baseDN 推导，如 dc=duiba,dc=com,dc=cn -> @duiba.com.cn
		upnSuffix = "@" + baseDNToDomain(conn.BaseDN)
	}

	// ===== 第一步：构建本地群组树并在 AD 中创建 OU 层级结构 =====
	var allGroups []models.UserGroup
	storage.DB.Order("parent_id, \"order\"").Find(&allGroups)

	// 构建群组 map 和父子关系
	groupMap := make(map[uint]models.UserGroup)
	childrenMap := make(map[uint][]models.UserGroup) // parentID -> children
	var rootGroups []models.UserGroup
	for _, g := range allGroups {
		groupMap[g.ID] = g
		childrenMap[g.ParentID] = append(childrenMap[g.ParentID], g)
	}
	rootGroups = childrenMap[0] // parent_id=0 的是根节点

	// 计算每个群组对应的 OU DN（递归构建完整路径）
	groupDNMap := make(map[uint]string)   // groupID -> OU DN
	groupLevelMap := make(map[uint]int)   // groupID -> level (用于按层级排序)
	var buildOUTree func(groups []models.UserGroup, parentDN string, level int)
	buildOUTree = func(groups []models.UserGroup, parentDN string, level int) {
		for _, g := range groups {
			ouDN := fmt.Sprintf("ou=%s,%s", ldapv3.EscapeDN(g.Name), parentDN)
			groupDNMap[g.ID] = ouDN
			groupLevelMap[g.ID] = level
			buildOUTree(childrenMap[g.ID], ouDN, level+1)
		}
	}
	buildOUTree(rootGroups, targetContainer, 1)

	// 按层级顺序创建 OU（严格先父后子）
	// 收集所有群组并按层级排序
	type groupWithLevel struct {
		group models.UserGroup
		level int
		ouDN  string
	}
	var sortedGroups []groupWithLevel
	for _, g := range allGroups {
		if dn, ok := groupDNMap[g.ID]; ok {
			sortedGroups = append(sortedGroups, groupWithLevel{
				group: g,
				level: groupLevelMap[g.ID],
				ouDN:  dn,
			})
		}
	}
	// 按层级排序（level 小的先创建）
	for i := 0; i < len(sortedGroups); i++ {
		for j := i + 1; j < len(sortedGroups); j++ {
			if sortedGroups[j].level < sortedGroups[i].level {
				sortedGroups[i], sortedGroups[j] = sortedGroups[j], sortedGroups[i]
			}
		}
	}

	ouCreated := 0
	ouSkipped := 0
	ouFailed := 0
	for _, sg := range sortedGroups {
		// 检查 OU 是否已存在
		sr, err := l.Search(ldapv3.NewSearchRequest(
			sg.ouDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 5, false,
			"(objectClass=*)", []string{"dn"}, nil,
		))
		if err == nil && len(sr.Entries) > 0 {
			ouSkipped++
			continue
		}
		// 创建 OU
		addReq := ldapv3.NewAddRequest(sg.ouDN, nil)
		addReq.Attribute("objectClass", []string{"top", "organizationalUnit"})
		addReq.Attribute("ou", []string{sg.group.Name})
		addReq.Attribute("description", []string{fmt.Sprintf("部门: %s", sg.group.Name)})
		if err := l.Add(addReq); err != nil {
			if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultEntryAlreadyExists) {
				ouSkipped++
			} else {
				ouFailed++
				result.Errors = append(result.Errors, fmt.Sprintf("[OU:%s] 创建OU失败(level=%d): %v", sg.group.Name, sg.level, err))
			}
		} else {
			ouCreated++
			log.Printf("[同步] OU创建成功(level=%d): %s", sg.level, sg.ouDN)
		}
	}
	log.Printf("[同步] OU同步完成: 新建%d, 已存在%d, 失败%d", ouCreated, ouSkipped, ouFailed)

	// ===== 第二步：同步角色为 AD 安全组 =====
	var roles []models.Role
	storage.DB.Find(&roles)
	roleDNMap := make(map[uint]string)
	roleGroupDN := ensureRoleGroupOU(l, syncr.RoleGroupContainer, targetContainer)
	for _, role := range roles {
		groupDN := fmt.Sprintf("cn=%s,%s", ldapv3.EscapeDN(role.Name), roleGroupDN)
		roleDNMap[role.ID] = groupDN
		// 检查是否存在
		sr, _ := l.Search(ldapv3.NewSearchRequest(
			groupDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 5, false,
			"(objectClass=*)", []string{"dn"}, nil,
		))
		if sr == nil || len(sr.Entries) == 0 {
			addReq := ldapv3.NewAddRequest(groupDN, nil)
			addReq.Attribute("objectClass", []string{"top", "group"})
			addReq.Attribute("cn", []string{role.Name})
			addReq.Attribute("sAMAccountName", []string{role.Name})
			addReq.Attribute("groupType", []string{"-2147483646"}) // 全局安全组
			addReq.Attribute("description", []string{fmt.Sprintf("角色: %s", role.Name)})
			if err := l.Add(addReq); err != nil {
				if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultEntryAlreadyExists) {
					result.Errors = append(result.Errors, fmt.Sprintf("[角色:%s] 创建安全组失败: %v", role.Name, err))
				}
			} else {
				log.Printf("[同步] 角色安全组创建成功: %s", groupDN)
			}
		}
	}

	// 加载所有用户的角色关系
	var userRoles []models.UserRole
	storage.DB.Find(&userRoles)
	userRoleMap := make(map[uint][]uint) // userID -> []roleID
	for _, ur := range userRoles {
		userRoleMap[ur.UserID] = append(userRoleMap[ur.UserID], ur.RoleID)
	}

	// 记录创建失败的 OU，避免重复错误
	failedOUs := make(map[uint]bool)

	// ===== 第三步：同步用户到对应的 OU =====
	for _, user := range users {
		// 确定用户应该在哪个 OU
		userParentDN := targetContainer // 默认
		if user.GroupID > 0 {
			// 先检查该 OU 是否之前创建失败
			if failedOUs[uint(user.GroupID)] {
				// OU 创建失败，尝试再次创建
				ouDN := ensureOUExists(l, uint(user.GroupID), targetContainer)
				if ouDN != "" {
					userParentDN = ouDN
					delete(failedOUs, uint(user.GroupID))
				} else {
					// 仍然失败，用户放到默认容器
					result.Errors = append(result.Errors, fmt.Sprintf("[%s] 所属分组OU创建失败，将用户放入默认容器", user.Username))
				}
			} else if dn, ok := groupDNMap[uint(user.GroupID)]; ok {
				userParentDN = dn
			}
		}
		userDN := fmt.Sprintf("cn=%s,%s", ldapv3.EscapeDN(user.Username), userParentDN)

		// 搜索用户是否已存在
		realDN := searchUserDN(l, conn.BaseDN, user.Username)
		if realDN != "" {
			// 用户已存在
			// 检查是否需要移动到正确的 OU
			if !strings.EqualFold(realDN, userDN) {
				// 需要移动：使用 ModifyDN
				newRDN := fmt.Sprintf("cn=%s", ldapv3.EscapeDN(user.Username))
				modDNReq := ldapv3.NewModifyDNRequest(realDN, newRDN, true, userParentDN)
				if err := l.ModifyDN(modDNReq); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("[%s] 移动OU失败: %v", user.Username, err))
					// 移动失败不影响后续更新，继续使用 realDN
				} else {
					realDN = userDN
					log.Printf("[同步] [%s] 已移动到 %s", user.Username, userParentDN)
				}
			}

			// 更新属性
			modReq := ldapv3.NewModifyRequest(realDN, nil)
			for _, m := range mappings {
				if adReadOnlyAttrs[m.TargetAttribute] {
					continue
				}
				val := resolveSourceValue(m, user, "")
				if val != "" {
					modReq.Replace(m.TargetAttribute, []string{val})
				}
			}
			if len(modReq.Changes) > 0 {
				if err := l.Modify(modReq); err != nil {
					result.Failed++
					result.Errors = append(result.Errors, fmt.Sprintf("[%s] 更新失败: %v", user.Username, err))
					continue
				}
			}

			// 确保 userAccountControl 包含"密码永不过期"标志
			// 读取当前 userAccountControl
			uacSR, _ := l.Search(ldapv3.NewSearchRequest(
				realDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 5, false,
				"(objectClass=*)", []string{"userAccountControl"}, nil,
			))
			if uacSR != nil && len(uacSR.Entries) > 0 {
				curUAC := uacSR.Entries[0].GetAttributeValue("userAccountControl")
				if curUAC != "" {
					uacVal := 0
					fmt.Sscanf(curUAC, "%d", &uacVal)
					// 如果没有 DONT_EXPIRE_PASSWORD(65536) 标志，则添加
					if uacVal&65536 == 0 {
						newUAC := uacVal | 65536
						modUAC := ldapv3.NewModifyRequest(realDN, nil)
						modUAC.Replace("userAccountControl", []string{fmt.Sprintf("%d", newUAC)})
						if err := l.Modify(modUAC); err != nil {
							log.Printf("[同步] [%s] 设置密码永不过期失败: %v", user.Username, err)
						}
					}
				}
			}

			result.Success++
			result.Updated = append(result.Updated, user.Username)
		} else {
			// 用户不存在 -> 创建
			addReq := ldapv3.NewAddRequest(userDN, nil)
			addReq.Attribute("objectClass", []string{"top", "person", "organizationalPerson", "user"})
			addReq.Attribute("sAMAccountName", []string{user.Username})
			addReq.Attribute("userPrincipalName", []string{user.Username + upnSuffix})
			// ACCOUNTDISABLE(2) + NORMAL_ACCOUNT(512) + DONT_EXPIRE_PASSWORD(65536) = 66050
			addReq.Attribute("userAccountControl", []string{"66050"})

			// displayName 和 sn
			displayName := user.Nickname
			if displayName == "" {
				displayName = user.Username
			}
			addReq.Attribute("displayName", []string{displayName})
			runes := []rune(displayName)
			if len(runes) > 0 {
				addReq.Attribute("sn", []string{string(runes[0])})
			}
			if len(runes) > 1 {
				addReq.Attribute("givenName", []string{string(runes[1:])})
			}

			// 从映射设置其他属性
			for _, m := range mappings {
				if adCreateSkipAttrs[m.TargetAttribute] {
					continue
				}
				if m.TargetAttribute == "displayName" || m.TargetAttribute == "sn" || m.TargetAttribute == "givenName" {
					continue
				}
				val := resolveSourceValue(m, user, "")
				if val != "" {
					addReq.Attribute(m.TargetAttribute, []string{val})
				}
			}

			if err := l.Add(addReq); err != nil {
				// 检查是否是 OU 不存在导致的错误 (LDAP Result Code 32 "No Such Object")
				if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchObject) && user.GroupID > 0 {
					// 尝试创建 OU 层级
					log.Printf("[同步] [%s] 父级OU不存在，尝试创建...", user.Username)
					ouDN := ensureOUExists(l, uint(user.GroupID), targetContainer)
					if ouDN != "" {
						// OU 创建成功，更新 userDN 并重试
						userDN = fmt.Sprintf("cn=%s,%s", ldapv3.EscapeDN(user.Username), ouDN)
						addReq2 := ldapv3.NewAddRequest(userDN, nil)
						for _, attr := range addReq.Attributes {
							addReq2.Attribute(attr.Type, attr.Vals)
						}
						if err2 := l.Add(addReq2); err2 != nil {
							result.Failed++
							result.Errors = append(result.Errors, fmt.Sprintf("[%s] 重试创建失败: %v", user.Username, err2))
							failedOUs[uint(user.GroupID)] = true
							continue
						}
						result.Success++
						result.Created = append(result.Created, user.Username)
						continue
					} else {
						failedOUs[uint(user.GroupID)] = true
					}
				}
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] 创建失败: %v", user.Username, err))
				continue
			}
			result.Success++
			result.Created = append(result.Created, user.Username)
		}

		// ===== 第四步：同步用户角色到 AD 安全组 =====
		actualDN := searchUserDN(l, conn.BaseDN, user.Username)
		if actualDN == "" {
			actualDN = userDN
		}
		roleIDs := userRoleMap[user.ID]
		for _, roleID := range roleIDs {
			groupDN, ok := roleDNMap[roleID]
			if !ok {
				continue
			}
			// 将用户添加到安全组（如果还不是成员）
			modReq := ldapv3.NewModifyRequest(groupDN, nil)
			modReq.Add("member", []string{actualDN})
			if err := l.Modify(modReq); err != nil {
				// 忽略"已经是成员"错误 (LDAP Result Code 68 "Entry Already Exists")
				if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultEntryAlreadyExists) &&
					!strings.Contains(err.Error(), "ENTRY_EXISTS") &&
					!strings.Contains(err.Error(), "already") {
					// 仅记录日志，不计入失败
					log.Printf("[同步] [%s] 添加角色组失败: %v", user.Username, err)
				}
			}
		}

		// ===== 第五步：设置/取消"用户不能更改密码" =====
		if err := adSetCannotChangePassword(l, actualDN, syncr.PreventPwdChange); err != nil {
			log.Printf("[同步] [%s] 设置'用户不能更改密码'(%v)失败: %v", user.Username, syncr.PreventPwdChange, err)
		}
	}

	return result
}

// baseDNToDomain 将 BaseDN 转换为域名：dc=duiba,dc=com,dc=cn -> duiba.com.cn
func baseDNToDomain(baseDN string) string {
	parts := strings.Split(baseDN, ",")
	var domain []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(strings.ToLower(p), "dc=") {
			domain = append(domain, p[3:])
		}
	}
	return strings.Join(domain, ".")
}

// ========== AD 单用户同步（事件触发）==========

// createADUser 创建 AD 用户（辅助函数，复用于 user_create/user_enable/password_change 等场景）
func createADUser(l *ldapv3.Conn, conn models.Connector, syncr models.Synchronizer, mappings []models.SyncAttributeMapping, userDN string, targetContainer string, user models.User, rawPassword string) SyncResult {
	result := SyncResult{}

	addReq := ldapv3.NewAddRequest(userDN, nil)
	addReq.Attribute("objectClass", []string{"top", "person", "organizationalPerson", "user"})
	addReq.Attribute("sAMAccountName", []string{user.Username})

	// UPN: 优先用 conn.UPNSuffix，没有则从 baseDN 推导
	upnSuffix := conn.UPNSuffix
	if upnSuffix == "" {
		upnSuffix = "@" + baseDNToDomain(conn.BaseDN)
	}
	addReq.Attribute("userPrincipalName", []string{user.Username + upnSuffix})

	// ACCOUNTDISABLE(2) + NORMAL_ACCOUNT(512) + DONT_EXPIRE_PASSWORD(65536) = 66050
	addReq.Attribute("userAccountControl", []string{"66050"})

	// displayName, sn, givenName
	displayName := user.Nickname
	if displayName == "" {
		displayName = user.Username
	}
	addReq.Attribute("displayName", []string{displayName})
	runes := []rune(displayName)
	if len(runes) > 0 {
		addReq.Attribute("sn", []string{string(runes[0])})
	}
	if len(runes) > 1 {
		addReq.Attribute("givenName", []string{string(runes[1:])})
	}

	for _, m := range mappings {
		if adCreateSkipAttrs[m.TargetAttribute] {
			continue
		}
		if m.TargetAttribute == "displayName" || m.TargetAttribute == "sn" || m.TargetAttribute == "givenName" {
			continue
		}
		val := resolveSourceValue(m, user, rawPassword)
		if val != "" {
			addReq.Attribute(m.TargetAttribute, []string{val})
		}
	}
	if err := l.Add(addReq); err != nil {
		result.Failed++
		result.Errors = append(result.Errors, fmt.Sprintf("[%s] 创建失败: %v", user.Username, err))
		return result
	}
	log.Printf("[同步] [%s] AD用户已创建", user.Username)

	// 创建后设置密码并启用账户
	if rawPassword != "" && conn.UseTLS {
		modPwd := ldapv3.NewModifyRequest(userDN, nil)
		modPwd.Replace("unicodePwd", []string{string(encodeADPassword(rawPassword))})
		if err := l.Modify(modPwd); err != nil {
			log.Printf("[同步] [%s] 创建后设置密码失败: %v", user.Username, err)
		} else {
			log.Printf("[同步] [%s] 密码已设置", user.Username)
		}
	}

	// 显式同步用户启用/禁用状态到 AD
	adSyncUserStatus(l, userDN, user)

	// 同步角色到 AD 安全组
	rgBaseDN := ensureRoleGroupOU(l, syncr.RoleGroupContainer, targetContainer)
	adSyncUserRoles(l, conn, userDN, rgBaseDN, user)

	// 设置/取消"用户不能更改密码"
	if err := adSetCannotChangePassword(l, userDN, syncr.PreventPwdChange); err != nil {
		log.Printf("[同步] [%s] 设置'用户不能更改密码'(%v)失败: %v", user.Username, syncr.PreventPwdChange, err)
	}

	result.Success++
	result.Created = append(result.Created, user.Username)
	return result
}

func syncUserToAD(conn models.Connector, syncr models.Synchronizer, user models.User, event string, rawPassword string) SyncResult {
	result := SyncResult{}

	l, err := dialLDAP(conn)
	if err != nil {
		result.Failed++
		result.Errors = append(result.Errors, fmt.Sprintf("[%s] LDAP连接失败: %v", user.Username, err))
		return result
	}
	defer l.Close()

	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		result.Failed++
		result.Errors = append(result.Errors, fmt.Sprintf("[%s] LDAP认证失败: %v", user.Username, err))
		return result
	}

	// 获取属性映射
	var mappings []models.SyncAttributeMapping
	storage.DB.Where("(synchronizer_id = ? OR sync_rule_id = ?) AND object_type = ? AND is_enabled = ?", syncr.ID, syncr.ID, "user", true).Order("priority").Find(&mappings)

	// 构造用户DN — 根据用户所属群组确定 OU（与批量同步一致）
	targetContainer := syncr.TargetContainer
	if targetContainer == "" {
		targetContainer = conn.BaseDN
	}
	roleGroupBaseDN := ensureRoleGroupOU(l, syncr.RoleGroupContainer, targetContainer)

	// 如果用户有群组，构建对应的 OU DN 层级并确保 OU 存在
	userParentDN := targetContainer
	if user.GroupID > 0 {
		// 确保 OU 层级存在（递归创建），返回创建后的 OU DN
		ouDN := ensureOUExists(l, user.GroupID, targetContainer)
		if ouDN != "" {
			userParentDN = ouDN
		}
	}
	userDN := fmt.Sprintf("cn=%s,%s", ldapv3.EscapeDN(user.Username), userParentDN)

	switch event {
	case models.SyncEventUserDelete:
		// 先搜索用户在 AD 中的实际 DN（可能与构造的 DN 不同）
		realDN := searchUserDN(l, conn.BaseDN, user.Username)
		if realDN == "" {
			// AD 中不存在，也算成功
			log.Printf("[同步] [%s] AD中未找到用户，无需删除", user.Username)
			result.Success++
			result.Deleted = append(result.Deleted, user.Username)
			return result
		}
		delReq := ldapv3.NewDelRequest(realDN, nil)
		if err := l.Del(delReq); err != nil {
			if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchObject) {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] 删除失败: %v", user.Username, err))
				return result
			}
		}
		log.Printf("[同步] [%s] AD用户已删除: %s", user.Username, realDN)
		result.Success++
		result.Deleted = append(result.Deleted, user.Username)
		return result

	case models.SyncEventUserEnable, models.SyncEventUserDisable:
		// 启用/禁用用户 - 检查是否配置了 status_to_delete
		log.Printf("[同步] [%s] 处理状态变更事件: %s, 本地状态=%d", user.Username, event, user.Status)
		realDN := searchUserDN(l, conn.BaseDN, user.Username)

		// 检查是否配置了 status_to_delete
		hasStatusToDelete := false
		for _, m := range mappings {
			if m.TransformRule == "status_to_delete" {
				hasStatusToDelete = true
				break
			}
		}

		// 用户被禁用且配置了 status_to_delete → 从 AD 删除
		if (event == models.SyncEventUserDisable || user.Status == 0) && hasStatusToDelete {
			if realDN != "" {
				log.Printf("[同步] [%s] 检测到 status_to_delete 转换规则，将从AD删除用户", user.Username)
				delReq := ldapv3.NewDelRequest(realDN, nil)
				if err := l.Del(delReq); err != nil {
					if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchObject) {
						result.Failed++
						result.Errors = append(result.Errors, fmt.Sprintf("[%s] AD删除失败: %v", user.Username, err))
						return result
					}
				}
				log.Printf("[同步] [%s] 用户已禁用，已从AD删除", user.Username)
				result.Deleted = append(result.Deleted, user.Username)
			} else {
				log.Printf("[同步] [%s] 用户已禁用，AD中不存在，无需删除", user.Username)
			}
			result.Success++
			return result
		}

		// 用户被启用且配置了 status_to_delete → 如果 AD 中不存在则重新创建
		if (event == models.SyncEventUserEnable || user.Status == 1) && hasStatusToDelete && realDN == "" {
			log.Printf("[同步] [%s] 用户被重新启用，AD中不存在，将重新创建", user.Username)
			// 重新创建用户（内联 user_create 逻辑）
			return createADUser(l, conn, syncr, mappings, userDN, userParentDN, user, rawPassword)
		}

		// AD 中用户不存在
		if realDN == "" {
			if event == models.SyncEventUserEnable || user.Status == 1 {
				log.Printf("[同步] [%s] 用户被启用但AD中不存在，自动创建", user.Username)
				return createADUser(l, conn, syncr, mappings, userDN, userParentDN, user, rawPassword)
			}
			log.Printf("[同步] [%s] AD中未找到用户，跳过状态同步", user.Username)
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("[%s] AD中未找到用户", user.Username))
			return result
		}

		// 没有配置 status_to_delete，执行正常的状态同步（启用/禁用）
		adSyncUserStatus(l, realDN, user)
		// 设置/取消"用户不能更改密码"
		if err := adSetCannotChangePassword(l, realDN, syncr.PreventPwdChange); err != nil {
			log.Printf("[同步] [%s] 设置'用户不能更改密码'(%v)失败: %v", user.Username, syncr.PreventPwdChange, err)
		}
		log.Printf("[同步] [%s] 状态已同步到AD", user.Username)
		result.Success++
		if event == models.SyncEventUserDisable || user.Status == 0 {
			result.Disabled = append(result.Disabled, user.Username)
		}
		return result

	case models.SyncEventPasswordChange:
		// 修改密码并同步状态
		if rawPassword != "" && conn.UseTLS {
			realDN := searchUserDN(l, conn.BaseDN, user.Username)

			// 如果用户在 AD 中不存在（可能之前被 status_to_delete 删除），先创建用户
			if realDN == "" {
				log.Printf("[同步] [%s] AD中用户不存在，先创建用户再同步密码", user.Username)
				// 创建用户
				createResult := createADUser(l, conn, syncr, mappings, userDN, userParentDN, user, rawPassword)
				if createResult.Failed > 0 {
					return createResult
				}
				// 重新搜索用户 DN
				realDN = searchUserDN(l, conn.BaseDN, user.Username)
				if realDN == "" {
					realDN = userDN
				}
				// 用户已创建并设置了密码，直接返回成功
				log.Printf("[同步] [%s] 用户已重新创建并设置密码", user.Username)
				return createResult
			}

			// 设置密码
			modReq := ldapv3.NewModifyRequest(realDN, nil)
			modReq.Replace("unicodePwd", []string{string(encodeADPassword(rawPassword))})
			if err := l.Modify(modReq); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] 密码同步失败: %v", user.Username, err))
				return result
			}
			// 同步用户状态（根据本地 status 决定启用/禁用）
			adSyncUserStatus(l, realDN, user)
			log.Printf("[同步] [%s] 密码已同步，状态已更新", user.Username)
			// 设置/取消"用户不能更改密码"
			if err := adSetCannotChangePassword(l, realDN, syncr.PreventPwdChange); err != nil {
				log.Printf("[同步] [%s] 设置'用户不能更改密码'(%v)失败: %v", user.Username, syncr.PreventPwdChange, err)
			}
			result.Success++
			result.Updated = append(result.Updated, user.Username)
		} else {
			result.Skipped++
			if !conn.UseTLS {
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] AD密码同步需要LDAPS连接", user.Username))
			}
		}
		return result

	default:
		// 检查映射中是否配置了 status_to_delete 且用户已禁用 → 从 AD 删除
		if user.Status == 0 {
			for _, m := range mappings {
				if m.TransformRule == "status_to_delete" {
					realDN := searchUserDN(l, conn.BaseDN, user.Username)
					if realDN != "" {
						delReq := ldapv3.NewDelRequest(realDN, nil)
						if err := l.Del(delReq); err != nil {
							if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchObject) {
								result.Failed++
								result.Errors = append(result.Errors, fmt.Sprintf("[%s] AD删除失败: %v", user.Username, err))
								return result
							}
						}
						log.Printf("[同步] [%s] 用户已禁用，已从AD删除", user.Username)
						result.Deleted = append(result.Deleted, user.Username)
					}
					result.Success++
					return result
				}
			}
		}

		// 创建或更新用户
		// 先搜索用户是否存在
		realDN := searchUserDN(l, conn.BaseDN, user.Username)
		if realDN != "" {
			// 用户存在 -> 检查是否需要移动 OU
			if !strings.EqualFold(realDN, userDN) {
				newRDN := fmt.Sprintf("cn=%s", ldapv3.EscapeDN(user.Username))
				modDNReq := ldapv3.NewModifyDNRequest(realDN, newRDN, true, userParentDN)
				if err := l.ModifyDN(modDNReq); err != nil {
					log.Printf("[同步] [%s] 移动OU失败: %v (从 %s 到 %s)", user.Username, err, realDN, userParentDN)
				} else {
					log.Printf("[同步] [%s] 已移动到正确OU: %s", user.Username, userParentDN)
					realDN = userDN
				}
			}

			// 更新属性
			modReq := ldapv3.NewModifyRequest(realDN, nil)
			for _, m := range mappings {
				if adReadOnlyAttrs[m.TargetAttribute] {
					continue
				}
				val := resolveSourceValue(m, user, rawPassword)
				if val != "" {
					modReq.Replace(m.TargetAttribute, []string{val})
				}
			}
			if len(modReq.Changes) > 0 {
				if err := l.Modify(modReq); err != nil {
					result.Failed++
					result.Errors = append(result.Errors, fmt.Sprintf("[%s] 更新失败: %v", user.Username, err))
					return result
				}
			}
			// 密码同步
			if rawPassword != "" && conn.UseTLS {
				modPwd := ldapv3.NewModifyRequest(realDN, nil)
				modPwd.Replace("unicodePwd", []string{string(encodeADPassword(rawPassword))})
				if err := l.Modify(modPwd); err != nil {
					log.Printf("[同步] [%s] AD密码同步失败: %v", user.Username, err)
				} else {
					log.Printf("[同步] [%s] 密码已同步到AD", user.Username)
				}
			}
			// 显式同步用户启用/禁用状态到 AD（不依赖属性映射）
			adSyncUserStatus(l, realDN, user)
			// 同步角色到 AD 安全组
			adSyncUserRoles(l, conn, realDN, roleGroupBaseDN, user)
			// 设置/取消"用户不能更改密码"
			if err := adSetCannotChangePassword(l, realDN, syncr.PreventPwdChange); err != nil {
				log.Printf("[同步] [%s] 设置'用户不能更改密码'(%v)失败: %v", user.Username, syncr.PreventPwdChange, err)
			}
			result.Success++
			result.Updated = append(result.Updated, user.Username)
		} else {
			// 用户不存在 -> 复用 createADUser
			return createADUser(l, conn, syncr, mappings, userDN, targetContainer, user, rawPassword)
		}
		return result
	}
}

// ========== 通用 LDAP 同步实现 ==========

// syncUserToGenericLDAP 通用 LDAP 同步（非 AD，使用标准 LDAP 属性）
func syncUserToGenericLDAP(conn models.Connector, syncr models.Synchronizer, user models.User, event string, rawPassword string) SyncResult {
	result := SyncResult{}

	l, err := dialLDAP(conn)
	if err != nil {
		result.Failed++
		result.Errors = append(result.Errors, fmt.Sprintf("[%s] LDAP连接失败: %v", user.Username, err))
		return result
	}
	defer l.Close()

	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		result.Failed++
		result.Errors = append(result.Errors, fmt.Sprintf("[%s] LDAP认证失败: %v", user.Username, err))
		return result
	}

	var mappings []models.SyncAttributeMapping
	storage.DB.Where("(synchronizer_id = ? OR sync_rule_id = ?) AND object_type = ? AND is_enabled = ?", syncr.ID, syncr.ID, "user", true).Order("priority").Find(&mappings)

	targetContainer := syncr.TargetContainer
	if targetContainer == "" {
		targetContainer = conn.BaseDN
	}
	userDN := fmt.Sprintf("uid=%s,%s", ldapv3.EscapeDN(user.Username), targetContainer)

	switch event {
	case models.SyncEventUserDelete:
		delReq := ldapv3.NewDelRequest(userDN, nil)
		if err := l.Del(delReq); err != nil {
			if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchObject) {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] 删除失败: %v", user.Username, err))
				return result
			}
		}
		result.Success++
		result.Deleted = append(result.Deleted, user.Username)
		return result

	case models.SyncEventPasswordChange:
		if rawPassword != "" {
			realDN := searchUserDNGeneric(l, conn.BaseDN, user.Username)
			if realDN == "" {
				realDN = userDN
			}
			modReq := ldapv3.NewModifyRequest(realDN, nil)
			modReq.Replace("userPassword", []string{rawPassword})
			if err := l.Modify(modReq); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] 密码同步失败: %v", user.Username, err))
				return result
			}
			result.Success++
			result.Updated = append(result.Updated, user.Username)
		} else {
			result.Skipped++
		}
		return result

	default:
		// 搜索是否已存在
		realDN := searchUserDNGeneric(l, conn.BaseDN, user.Username)
		if realDN != "" {
			// 更新
			modReq := ldapv3.NewModifyRequest(realDN, nil)
			skipAttrs := map[string]bool{"uid": true, "objectClass": true, "cn": true}
			for _, m := range mappings {
				if skipAttrs[m.TargetAttribute] {
					continue
				}
				val := resolveSourceValue(m, user, rawPassword)
				if val != "" {
					modReq.Replace(m.TargetAttribute, []string{val})
				}
			}
			if len(modReq.Changes) > 0 {
				if err := l.Modify(modReq); err != nil {
					result.Failed++
					result.Errors = append(result.Errors, fmt.Sprintf("[%s] 更新失败: %v", user.Username, err))
					return result
				}
			}
			result.Success++
			result.Updated = append(result.Updated, user.Username)
		} else {
			// 创建
			displayName := user.Nickname
			if displayName == "" {
				displayName = user.Username
			}
			addReq := ldapv3.NewAddRequest(userDN, nil)
			addReq.Attribute("objectClass", []string{"top", "person", "organizationalPerson", "inetOrgPerson"})
			addReq.Attribute("cn", []string{displayName})
			addReq.Attribute("sn", []string{displayName})

			skipAttrs := map[string]bool{"uid": true, "objectClass": true, "cn": true, "sn": true}
			for _, m := range mappings {
				if skipAttrs[m.TargetAttribute] {
					continue
				}
				val := resolveSourceValue(m, user, rawPassword)
				if val != "" {
					addReq.Attribute(m.TargetAttribute, []string{val})
				}
			}

			if err := l.Add(addReq); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] 创建失败: %v", user.Username, err))
				return result
			}
			result.Success++
			result.Created = append(result.Created, user.Username)
		}
		return result
	}
}

// batchSyncUsersToGenericLDAP 通用 LDAP 批量同步
func batchSyncUsersToGenericLDAP(conn models.Connector, syncr models.Synchronizer, users []models.User, mappings []models.SyncAttributeMapping) SyncResult {
	result := SyncResult{Total: len(users)}

	l, err := dialLDAP(conn)
	if err != nil {
		result.Failed = len(users)
		result.Errors = append(result.Errors, "LDAP连接失败: "+err.Error())
		return result
	}
	defer l.Close()

	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		result.Failed = len(users)
		result.Errors = append(result.Errors, "LDAP认证失败: "+err.Error())
		return result
	}

	targetContainer := syncr.TargetContainer
	if targetContainer == "" {
		targetContainer = conn.BaseDN
	}

	for _, user := range users {
		r := syncUserToGenericLDAP(conn, syncr, user, models.SyncEventUserUpdate, "")
		result.Success += r.Success
		result.Failed += r.Failed
		result.Errors = append(result.Errors, r.Errors...)
		result.Created = append(result.Created, r.Created...)
		result.Updated = append(result.Updated, r.Updated...)
		result.Deleted = append(result.Deleted, r.Deleted...)
		result.Disabled = append(result.Disabled, r.Disabled...)
	}

	return result
}

// searchUserDNGeneric 在通用 LDAP 中搜索用户 DN (按 uid)
func searchUserDNGeneric(l *ldapv3.Conn, baseDN, username string) string {
	searchReq := ldapv3.NewSearchRequest(
		baseDN, ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases,
		1, 10, false,
		fmt.Sprintf("(uid=%s)", ldapv3.EscapeFilter(username)),
		[]string{"dn"}, nil,
	)
	sr, err := l.Search(searchReq)
	if err != nil || len(sr.Entries) == 0 {
		return ""
	}
	return sr.Entries[0].DN
}

// ========== MySQL 同步实现 ==========

// syncUserToDB 通用数据库同步（支持 mysql/postgresql/oracle/sqlserver）
func syncUserToDB(conn models.Connector, syncr models.Synchronizer, user models.User, event string, rawPassword string) SyncResult {
	result := SyncResult{}
	dbType := conn.EffectiveDBType()

	db, err := dialDB(conn)
	if err != nil {
		result.Failed++
		result.Errors = append(result.Errors, fmt.Sprintf("[%s] %s连接失败: %v", user.Username, dbType, err))
		return result
	}
	defer db.Close()

	if conn.UserTable == "" {
		result.Failed++
		result.Errors = append(result.Errors, "未配置用户表名")
		return result
	}

	var mappings []models.SyncAttributeMapping
	storage.DB.Where("(synchronizer_id = ? OR sync_rule_id = ?) AND object_type = ? AND is_enabled = ?", syncr.ID, syncr.ID, "user", true).Order("priority").Find(&mappings)

	// 使用 placeholder 函数根据数据库类型生成参数占位符
	ph := func(idx int) string {
		switch dbType {
		case "postgresql":
			return fmt.Sprintf("$%d", idx)
		case "oracle":
			return fmt.Sprintf(":%d", idx)
		default: // mysql, sqlserver
			return "?"
		}
	}

	switch event {
	case models.SyncEventUserDelete:
		q := fmt.Sprintf("DELETE FROM %s WHERE %s = %s",
			quoteIdentifier(dbType, conn.UserTable),
			quoteIdentifier(dbType, "username"),
			ph(1))
		_, err := db.Exec(q, user.Username)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("[%s] 删除失败: %v", user.Username, err))
			return result
		}
		result.Success++
		result.Deleted = append(result.Deleted, user.Username)
		return result

	default:
		// 构造字段值映射
		cols := make(map[string]string)
		for _, m := range mappings {
			val := resolveSourceValue(m, user, rawPassword)
			if val != "" {
				cols[m.TargetAttribute] = val
			}
		}

		if len(cols) == 0 {
			result.Skipped++
			return result
		}

		usernameCol := "username"
		for _, m := range mappings {
			if m.SourceAttribute == "username" {
				usernameCol = m.TargetAttribute
				break
			}
		}

		var count int
		countQ := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = %s",
			quoteIdentifier(dbType, conn.UserTable),
			quoteIdentifier(dbType, usernameCol),
			ph(1))
		err := db.QueryRow(countQ, user.Username).Scan(&count)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("[%s] 查询失败: %v", user.Username, err))
			return result
		}

		if count > 0 {
			// 更新
			setClauses := make([]string, 0, len(cols))
			vals := make([]interface{}, 0, len(cols)+1)
			paramIdx := 1
			for col, val := range cols {
				if col == usernameCol {
					continue
				}
				setClauses = append(setClauses, fmt.Sprintf("%s = %s", quoteIdentifier(dbType, col), ph(paramIdx)))
				vals = append(vals, val)
				paramIdx++
			}
			vals = append(vals, user.Username)

			if len(setClauses) > 0 {
				query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s",
					quoteIdentifier(dbType, conn.UserTable),
					strings.Join(setClauses, ", "),
					quoteIdentifier(dbType, usernameCol),
					ph(paramIdx))
				if _, err := db.Exec(query, vals...); err != nil {
					result.Failed++
					result.Errors = append(result.Errors, fmt.Sprintf("[%s] 更新失败: %v", user.Username, err))
					return result
				}
			}
			result.Success++
			result.Updated = append(result.Updated, user.Username)
		} else if event != models.SyncEventPasswordChange {
			colNames := make([]string, 0, len(cols))
			placeholders := make([]string, 0, len(cols))
			vals := make([]interface{}, 0, len(cols))
			paramIdx := 1
			for col, val := range cols {
				colNames = append(colNames, quoteIdentifier(dbType, col))
				placeholders = append(placeholders, ph(paramIdx))
				vals = append(vals, val)
				paramIdx++
			}
			query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
				quoteIdentifier(dbType, conn.UserTable),
				strings.Join(colNames, ", "),
				strings.Join(placeholders, ", "))
			if _, err := db.Exec(query, vals...); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] 插入失败: %v", user.Username, err))
				return result
			}
			result.Success++
			result.Created = append(result.Created, user.Username)
		} else {
			result.Skipped++
		}
		return result
	}
}

// syncUserToMySQL 兼容旧代码调用
func syncUserToMySQL(conn models.Connector, syncr models.Synchronizer, user models.User, event string, rawPassword string) SyncResult {
	return syncUserToDB(conn, syncr, user, event, rawPassword)
}

// ========== 数据库群组同步 ==========

// syncGroupToDB 同步群组变更到数据库
func syncGroupToDB(conn models.Connector, syncr models.Synchronizer, group models.UserGroup, event string) {
	if conn.GroupTable == "" {
		return
	}

	dbType := conn.EffectiveDBType()
	db, err := dialDB(conn)
	if err != nil {
		log.Printf("[同步] 数据库连接失败: %v", err)
		return
	}
	defer db.Close()

	// 获取群组属性映射
	var mappings []models.SyncAttributeMapping
	storage.DB.Where("(synchronizer_id = ? OR sync_rule_id = ?) AND object_type = ? AND is_enabled = ?", syncr.ID, syncr.ID, "group", true).Order("priority").Find(&mappings)

	ph := func(idx int) string {
		switch dbType {
		case "postgresql":
			return fmt.Sprintf("$%d", idx)
		case "oracle":
			return fmt.Sprintf(":%d", idx)
		default:
			return "?"
		}
	}

	switch event {
	case "group_delete":
		// 删除群组记录
		idCol := "id"
		for _, m := range mappings {
			if m.SourceAttribute == "id" {
				idCol = m.TargetAttribute
				break
			}
		}
		q := fmt.Sprintf("DELETE FROM %s WHERE %s = %s",
			quoteIdentifier(dbType, conn.GroupTable),
			quoteIdentifier(dbType, idCol),
			ph(1))
		if _, err := db.Exec(q, group.ID); err != nil {
			log.Printf("[同步] 删除群组失败: %v", err)
		} else {
			log.Printf("[同步] 群组已从数据库删除: %s (ID=%d)", group.Name, group.ID)
		}

	case "group_create", "group_update":
		// 构造字段值映射
		cols := make(map[string]interface{})
		for _, m := range mappings {
			val := resolveGroupSourceValue(m, group)
			if val != nil {
				cols[m.TargetAttribute] = val
			}
		}

		// 默认映射
		if len(cols) == 0 {
			cols["id"] = group.ID
			cols["name"] = group.Name
			cols["parent_id"] = group.ParentID
		}

		idCol := "id"
		for _, m := range mappings {
			if m.SourceAttribute == "id" {
				idCol = m.TargetAttribute
				break
			}
		}

		// 检查是否存在
		var count int
		countQ := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = %s",
			quoteIdentifier(dbType, conn.GroupTable),
			quoteIdentifier(dbType, idCol),
			ph(1))
		if err := db.QueryRow(countQ, group.ID).Scan(&count); err != nil {
			log.Printf("[同步] 查询群组失败: %v", err)
			return
		}

		if count > 0 {
			// 更新
			setClauses := make([]string, 0)
			vals := make([]interface{}, 0)
			paramIdx := 1
			for col, val := range cols {
				if col == idCol {
					continue
				}
				setClauses = append(setClauses, fmt.Sprintf("%s = %s", quoteIdentifier(dbType, col), ph(paramIdx)))
				vals = append(vals, val)
				paramIdx++
			}
			vals = append(vals, group.ID)

			if len(setClauses) > 0 {
				query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s",
					quoteIdentifier(dbType, conn.GroupTable),
					strings.Join(setClauses, ", "),
					quoteIdentifier(dbType, idCol),
					ph(paramIdx))
				if _, err := db.Exec(query, vals...); err != nil {
					log.Printf("[同步] 更新群组失败: %v", err)
				} else {
					log.Printf("[同步] 群组已更新: %s (ID=%d)", group.Name, group.ID)
				}
			}
		} else if event == "group_create" {
			// 插入
			colNames := make([]string, 0)
			placeholders := make([]string, 0)
			vals := make([]interface{}, 0)
			paramIdx := 1
			for col, val := range cols {
				colNames = append(colNames, quoteIdentifier(dbType, col))
				placeholders = append(placeholders, ph(paramIdx))
				vals = append(vals, val)
				paramIdx++
			}
			query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
				quoteIdentifier(dbType, conn.GroupTable),
				strings.Join(colNames, ", "),
				strings.Join(placeholders, ", "))
			if _, err := db.Exec(query, vals...); err != nil {
				log.Printf("[同步] 插入群组失败: %v", err)
			} else {
				log.Printf("[同步] 群组已创建: %s (ID=%d)", group.Name, group.ID)
			}
		}
	}
}

// resolveGroupSourceValue 解析群组属性值
func resolveGroupSourceValue(m models.SyncAttributeMapping, group models.UserGroup) interface{} {
	var val interface{}
	switch m.SourceAttribute {
	case "id":
		val = group.ID
	case "name":
		val = group.Name
	case "parent_id", "parentId":
		val = group.ParentID
	case "order":
		val = group.Order
	case "dingtalk_dept_id", "dingTalkDeptId":
		val = group.DingTalkDeptID
	}
	return val
}

// ========== 数据库角色同步 ==========

// SyncRoleToDownstream 同步角色变更到所有下游连接器
func SyncRoleToDownstream(role models.Role, event string) {
	var syncRules []models.Synchronizer
	storage.DB.Where("direction = 'push' AND status = ? AND sync_roles = ?", 1, true).Find(&syncRules)

	for _, syncr := range syncRules {
		var conn models.Connector
		if err := storage.DB.First(&conn, syncr.ConnectorID).Error; err != nil {
			continue
		}

		switch conn.Type {
		case "ldap_ad":
			// AD 角色映射为安全组
			syncRoleToAD(conn, syncr, role, event)
		case "ldap_generic":
			// 通用 LDAP 角色映射为 groupOfNames
			syncRoleToGenericLDAP(conn, syncr, role, event)
		case "mysql", "postgresql", "oracle", "sqlserver":
			if conn.RoleTable != "" {
				syncRoleToDB(conn, syncr, role, event)
			}
		}
	}
}

// syncRoleToAD 同步角色变更到 AD（作为安全组）
func syncRoleToAD(conn models.Connector, syncr models.Synchronizer, role models.Role, event string) {
	l, err := dialLDAP(conn)
	if err != nil {
		log.Printf("[同步] AD连接失败: %v", err)
		return
	}
	defer l.Close()

	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		log.Printf("[同步] AD认证失败: %v", err)
		return
	}

	roleGroupBaseDN := syncr.RoleGroupContainer
	if roleGroupBaseDN == "" {
		roleGroupBaseDN = conn.BaseDN
	}

	groupDN := fmt.Sprintf("cn=%s,%s", ldapv3.EscapeDN(role.Name), roleGroupBaseDN)

	switch event {
	case "role_create":
		// 确保角色组 OU 存在
		ensureRoleGroupOU(l, syncr.RoleGroupContainer, conn.BaseDN)

		// 创建安全组
		addReq := ldapv3.NewAddRequest(groupDN, nil)
		addReq.Attribute("objectClass", []string{"top", "group"})
		addReq.Attribute("cn", []string{role.Name})
		addReq.Attribute("sAMAccountName", []string{role.Code})
		addReq.Attribute("groupType", []string{"-2147483646"}) // Universal Security Group
		if role.Description != "" {
			addReq.Attribute("description", []string{role.Description})
		}
		if err := l.Add(addReq); err != nil {
			if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultEntryAlreadyExists) {
				log.Printf("[同步] 创建AD安全组失败 %s: %v", groupDN, err)
			}
		} else {
			log.Printf("[同步] AD安全组已创建: %s", groupDN)
		}

	case "role_update":
		// 更新安全组属性
		realDN := searchGroupDN(l, conn.BaseDN, role.Name)
		if realDN == "" {
			// 不存在则创建
			SyncRoleToDownstream(role, "role_create")
			return
		}
		modReq := ldapv3.NewModifyRequest(realDN, nil)
		if role.Description != "" {
			modReq.Replace("description", []string{role.Description})
		}
		if len(modReq.Changes) > 0 {
			if err := l.Modify(modReq); err != nil {
				log.Printf("[同步] 更新AD安全组失败 %s: %v", realDN, err)
			} else {
				log.Printf("[同步] AD安全组已更新: %s", realDN)
			}
		}

	case "role_delete":
		realDN := searchGroupDN(l, conn.BaseDN, role.Name)
		if realDN != "" {
			delReq := ldapv3.NewDelRequest(realDN, nil)
			if err := l.Del(delReq); err != nil {
				if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchObject) {
					log.Printf("[同步] 删除AD安全组失败 %s: %v", realDN, err)
				}
			} else {
				log.Printf("[同步] AD安全组已删除: %s", realDN)
			}
		}
	}
}

// syncRoleToGenericLDAP 同步角色变更到通用 LDAP（作为 groupOfNames）
func syncRoleToGenericLDAP(conn models.Connector, syncr models.Synchronizer, role models.Role, event string) {
	l, err := dialLDAP(conn)
	if err != nil {
		log.Printf("[同步] LDAP连接失败: %v", err)
		return
	}
	defer l.Close()

	if err := l.Bind(conn.BindDN, conn.BindPassword); err != nil {
		log.Printf("[同步] LDAP认证失败: %v", err)
		return
	}

	roleGroupBaseDN := syncr.RoleGroupContainer
	if roleGroupBaseDN == "" {
		roleGroupBaseDN = conn.BaseDN
	}

	groupDN := fmt.Sprintf("cn=%s,%s", ldapv3.EscapeDN(role.Name), roleGroupBaseDN)

	switch event {
	case "role_create":
		// 创建 groupOfNames（OpenLDAP 标准）
		addReq := ldapv3.NewAddRequest(groupDN, nil)
		addReq.Attribute("objectClass", []string{"top", "groupOfNames"})
		addReq.Attribute("cn", []string{role.Name})
		// groupOfNames 必须有至少一个 member，使用占位符
		addReq.Attribute("member", []string{conn.BindDN})
		if role.Description != "" {
			addReq.Attribute("description", []string{role.Description})
		}
		if err := l.Add(addReq); err != nil {
			if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultEntryAlreadyExists) {
				log.Printf("[同步] 创建LDAP组失败 %s: %v", groupDN, err)
			}
		} else {
			log.Printf("[同步] LDAP组已创建: %s", groupDN)
		}

	case "role_update":
		// 更新组属性
		realDN := searchGroupDNGeneric(l, conn.BaseDN, role.Name)
		if realDN == "" {
			syncRoleToGenericLDAP(conn, syncr, role, "role_create")
			return
		}
		modReq := ldapv3.NewModifyRequest(realDN, nil)
		if role.Description != "" {
			modReq.Replace("description", []string{role.Description})
		}
		if len(modReq.Changes) > 0 {
			if err := l.Modify(modReq); err != nil {
				log.Printf("[同步] 更新LDAP组失败 %s: %v", realDN, err)
			} else {
				log.Printf("[同步] LDAP组已更新: %s", realDN)
			}
		}

	case "role_delete":
		realDN := searchGroupDNGeneric(l, conn.BaseDN, role.Name)
		if realDN != "" {
			delReq := ldapv3.NewDelRequest(realDN, nil)
			if err := l.Del(delReq); err != nil {
				if !ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchObject) {
					log.Printf("[同步] 删除LDAP组失败 %s: %v", realDN, err)
				}
			} else {
				log.Printf("[同步] LDAP组已删除: %s", realDN)
			}
		}
	}
}

// searchGroupDNGeneric 搜索通用LDAP中的组DN
func searchGroupDNGeneric(l *ldapv3.Conn, baseDN string, groupName string) string {
	sr, err := l.Search(ldapv3.NewSearchRequest(
		baseDN, ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 1, 10, false,
		fmt.Sprintf("(&(objectClass=groupOfNames)(cn=%s))", ldapv3.EscapeFilter(groupName)),
		[]string{"dn"}, nil,
	))
	if err != nil || len(sr.Entries) == 0 {
		return ""
	}
	return sr.Entries[0].DN
}

// searchGroupDN 搜索AD中的组DN
func searchGroupDN(l *ldapv3.Conn, baseDN string, groupName string) string {
	sr, err := l.Search(ldapv3.NewSearchRequest(
		baseDN, ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 1, 10, false,
		fmt.Sprintf("(&(objectClass=group)(cn=%s))", ldapv3.EscapeFilter(groupName)),
		[]string{"dn"}, nil,
	))
	if err != nil || len(sr.Entries) == 0 {
		return ""
	}
	return sr.Entries[0].DN
}

// syncRoleToDB 同步角色变更到数据库
func syncRoleToDB(conn models.Connector, syncr models.Synchronizer, role models.Role, event string) {
	if conn.RoleTable == "" {
		return
	}

	dbType := conn.EffectiveDBType()
	db, err := dialDB(conn)
	if err != nil {
		log.Printf("[同步] 数据库连接失败: %v", err)
		return
	}
	defer db.Close()

	// 获取角色属性映射
	var mappings []models.SyncAttributeMapping
	storage.DB.Where("(synchronizer_id = ? OR sync_rule_id = ?) AND object_type = ? AND is_enabled = ?", syncr.ID, syncr.ID, "role", true).Order("priority").Find(&mappings)

	ph := func(idx int) string {
		switch dbType {
		case "postgresql":
			return fmt.Sprintf("$%d", idx)
		case "oracle":
			return fmt.Sprintf(":%d", idx)
		default:
			return "?"
		}
	}

	switch event {
	case "role_delete":
		idCol := "id"
		for _, m := range mappings {
			if m.SourceAttribute == "id" {
				idCol = m.TargetAttribute
				break
			}
		}
		q := fmt.Sprintf("DELETE FROM %s WHERE %s = %s",
			quoteIdentifier(dbType, conn.RoleTable),
			quoteIdentifier(dbType, idCol),
			ph(1))
		if _, err := db.Exec(q, role.ID); err != nil {
			log.Printf("[同步] 删除角色失败: %v", err)
		} else {
			log.Printf("[同步] 角色已从数据库删除: %s (ID=%d)", role.Name, role.ID)
		}

	case "role_create", "role_update":
		cols := make(map[string]interface{})
		for _, m := range mappings {
			val := resolveRoleSourceValue(m, role)
			if val != nil {
				cols[m.TargetAttribute] = val
			}
		}

		// 默认映射
		if len(cols) == 0 {
			cols["id"] = role.ID
			cols["name"] = role.Name
			cols["code"] = role.Code
			cols["description"] = role.Description
			cols["status"] = role.Status
		}

		idCol := "id"
		for _, m := range mappings {
			if m.SourceAttribute == "id" {
				idCol = m.TargetAttribute
				break
			}
		}

		var count int
		countQ := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = %s",
			quoteIdentifier(dbType, conn.RoleTable),
			quoteIdentifier(dbType, idCol),
			ph(1))
		if err := db.QueryRow(countQ, role.ID).Scan(&count); err != nil {
			log.Printf("[同步] 查询角色失败: %v", err)
			return
		}

		if count > 0 {
			// 更新
			setClauses := make([]string, 0)
			vals := make([]interface{}, 0)
			paramIdx := 1
			for col, val := range cols {
				if col == idCol {
					continue
				}
				setClauses = append(setClauses, fmt.Sprintf("%s = %s", quoteIdentifier(dbType, col), ph(paramIdx)))
				vals = append(vals, val)
				paramIdx++
			}
			vals = append(vals, role.ID)

			if len(setClauses) > 0 {
				query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s",
					quoteIdentifier(dbType, conn.RoleTable),
					strings.Join(setClauses, ", "),
					quoteIdentifier(dbType, idCol),
					ph(paramIdx))
				if _, err := db.Exec(query, vals...); err != nil {
					log.Printf("[同步] 更新角色失败: %v", err)
				} else {
					log.Printf("[同步] 角色已更新: %s (ID=%d)", role.Name, role.ID)
				}
			}
		} else if event == "role_create" {
			colNames := make([]string, 0)
			placeholders := make([]string, 0)
			vals := make([]interface{}, 0)
			paramIdx := 1
			for col, val := range cols {
				colNames = append(colNames, quoteIdentifier(dbType, col))
				placeholders = append(placeholders, ph(paramIdx))
				vals = append(vals, val)
				paramIdx++
			}
			query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
				quoteIdentifier(dbType, conn.RoleTable),
				strings.Join(colNames, ", "),
				strings.Join(placeholders, ", "))
			if _, err := db.Exec(query, vals...); err != nil {
				log.Printf("[同步] 插入角色失败: %v", err)
			} else {
				log.Printf("[同步] 角色已创建: %s (ID=%d)", role.Name, role.ID)
			}
		}
	}
}

// resolveRoleSourceValue 解析角色属性值
func resolveRoleSourceValue(m models.SyncAttributeMapping, role models.Role) interface{} {
	var val interface{}
	switch m.SourceAttribute {
	case "id":
		val = role.ID
	case "name":
		val = role.Name
	case "code":
		val = role.Code
	case "description":
		val = role.Description
	case "status":
		val = role.Status
	}
	return val
}

// ========== 工具函数 ==========

func dialLDAP(conn models.Connector) (*ldapv3.Conn, error) {
	addr := fmt.Sprintf("%s:%d", conn.Host, conn.Port)
	if conn.UseTLS {
		return ldapv3.DialTLS("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	}
	return ldapv3.Dial("tcp", addr)
}

// dialDB 统一数据库连接入口，支持 mysql/postgresql/oracle/sqlserver
func dialDB(conn models.Connector) (*sql.DB, error) {
	dbType := conn.EffectiveDBType()
	switch dbType {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&timeout=%ds",
			conn.DBUser, conn.DBPassword, conn.Host, conn.Port, conn.Database, conn.Charset, conn.Timeout)
		return sql.Open("mysql", dsn)
	case "postgresql":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable connect_timeout=%d",
			conn.Host, conn.Port, conn.DBUser, conn.DBPassword, conn.Database, conn.Timeout)
		return sql.Open("postgres", dsn)
	case "oracle":
		svc := conn.ServiceName
		if svc == "" {
			svc = conn.Database
		}
		dsn := fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
			conn.DBUser, conn.DBPassword, conn.Host, conn.Port, svc)
		return sql.Open("oracle", dsn)
	case "sqlserver":
		dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s&connection+timeout=%d",
			conn.DBUser, conn.DBPassword, conn.Host, conn.Port, conn.Database, conn.Timeout)
		return sql.Open("sqlserver", dsn)
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
}

// dialMySQL 兼容旧代码调用
func dialMySQL(conn models.Connector) (*sql.DB, error) {
	return dialDB(conn)
}

// OpenDBConnection 公开的数据库连接函数，供外部包使用
func OpenDBConnection(conn models.Connector) (*sql.DB, error) {
	return dialDB(conn)
}

// quoteIdentifier 根据数据库类型对标识符（表名/列名）加引号
func quoteIdentifier(dbType, name string) string {
	switch dbType {
	case "mysql":
		return "`" + name + "`"
	case "sqlserver":
		return "[" + name + "]"
	default: // postgresql, oracle
		return "\"" + name + "\""
	}
}

// checkTableForDB 检查表是否存在（支持多种数据库）
func checkTableForDB(db *sql.DB, dbType, table string) (bool, int) {
	q := quoteIdentifier(dbType, table)
	var count int
	err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", q)).Scan(&count)
	if err != nil {
		return false, 0
	}
	return true, count
}

// checkTable 兼容旧调用（默认 MySQL）
func checkTable(db *sql.DB, table string) (bool, int) {
	return checkTableForDB(db, "mysql", table)
}

func searchUserDN(l *ldapv3.Conn, baseDN, username string) string {
	sr, err := l.Search(ldapv3.NewSearchRequest(
		baseDN,
		ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 1, 5, false,
		fmt.Sprintf("(sAMAccountName=%s)", ldapv3.EscapeFilter(username)),
		[]string{"dn"}, nil,
	))
	if err != nil || len(sr.Entries) == 0 {
		return ""
	}
	return sr.Entries[0].DN
}

// encodeADPassword 将密码编码为 AD unicodePwd 格式
func encodeADPassword(password string) []byte {
	quoted := "\"" + password + "\""
	encoded := utf16.Encode([]rune(quoted))
	result := make([]byte, len(encoded)*2)
	for i, v := range encoded {
		result[i*2] = byte(v)
		result[i*2+1] = byte(v >> 8)
	}
	return result
}

// resolveSourceValue 解析源字段值
func resolveSourceValue(m models.SyncAttributeMapping, user models.User, rawPassword string) string {
	var baseValue string

	switch m.SourceAttribute {
	case "username":
		baseValue = user.Username
	case "password", "password_raw":
		baseValue = rawPassword // 原文密码
	case "password_hash":
		baseValue = user.Password // bcrypt哈希
	case "samba_nt_password":
		baseValue = user.SambaNTPassword
	case "nickname":
		baseValue = user.Nickname
	case "phone":
		baseValue = user.Phone
	case "email":
		baseValue = user.Email
	case "avatar":
		baseValue = user.Avatar
	case "status":
		if user.Status == 1 {
			baseValue = "1"
		} else {
			baseValue = "0"
		}
	case "source":
		baseValue = user.Source
	case "job_title":
		baseValue = user.JobTitle
	case "department_name":
		baseValue = user.DepartmentName
	case "group_name":
		var group models.UserGroup
		if user.GroupID > 0 {
			storage.DB.First(&group, user.GroupID)
			baseValue = group.Name
		}
	case "group_id":
		baseValue = fmt.Sprintf("%d", user.GroupID)
	case "roles":
		roleNames := make([]string, 0, len(user.Roles))
		for _, r := range user.Roles {
			roleNames = append(roleNames, r.Code)
		}
		baseValue = strings.Join(roleNames, ",")
	case "role_names":
		roleNames := make([]string, 0, len(user.Roles))
		for _, r := range user.Roles {
			roleNames = append(roleNames, r.Name)
		}
		baseValue = strings.Join(roleNames, ",")
	case "dingtalk_uid":
		baseValue = user.DingTalkUID
	case "created_at":
		baseValue = user.CreatedAt.Format("2006-01-02 15:04:05")
	case "updated_at":
		baseValue = user.UpdatedAt.Format("2006-01-02 15:04:05")
	case "last_login_ip":
		baseValue = user.LastLoginIP
	case "last_login_at":
		if user.LastLoginAt != nil {
			baseValue = user.LastLoginAt.Format("2006-01-02 15:04:05")
		}
	case "password_changed_at":
		if user.PasswordChangedAt != nil {
			baseValue = user.PasswordChangedAt.Format("2006-01-02 15:04:05")
		}
	case "mfa_enabled":
		if user.MFAEnabled {
			baseValue = "1"
		} else {
			baseValue = "0"
		}
	case "id":
		baseValue = fmt.Sprintf("%d", user.ID)
	}

	// 应用转换规则
	switch m.MappingType {
	case "constant":
		return m.TransformRule
	case "transform":
		return applyTransform(baseValue, m.TransformRule, user)
	case "expression":
		return applyExpression(m.TransformRule, user)
	default:
		// 检查是否需要密码格式转换
		if m.PasswordFormat != "" && (m.SourceAttribute == "password" || m.SourceAttribute == "password_raw") && baseValue != "" {
			hashed, err := hashPasswordWithFormat(baseValue, m.PasswordFormat)
			if err != nil {
				log.Printf("[密码转换] 格式 %s 转换失败: %v", m.PasswordFormat, err)
				return baseValue
			}
			return hashed
		}
		return baseValue
	}
}

func applyTransform(value, rule string, user models.User) string {
	switch {
	case strings.HasPrefix(rule, "append:"):
		return value + strings.TrimPrefix(rule, "append:")
	case strings.HasPrefix(rule, "prepend:"):
		return strings.TrimPrefix(rule, "prepend:") + value
	case rule == "status_to_uac":
		if value == "1" {
			return "512"
		}
		return "514"
	case rule == "status_to_delete":
		// 特殊标记：status=0 时返回 "DELETE"，由 syncUserToAD 处理删除逻辑
		if value == "0" {
			return "AD_DELETE_USER"
		}
		return "66048" // 启用
	case rule == "chinese_surname":
		runes := []rune(value)
		if len(runes) > 0 {
			return string(runes[0])
		}
		return value
	case rule == "chinese_given_name":
		runes := []rune(value)
		if len(runes) > 1 {
			return string(runes[1:])
		}
		return ""
	case rule == "upper":
		return strings.ToUpper(value)
	case rule == "lower":
		return strings.ToLower(value)
	case rule == "password_to_unicode":
		return string(encodeADPassword(value))
	case rule == "time_to_filetime":
		// Go time string -> Windows FILETIME
		t, err := time.Parse("2006-01-02 15:04:05", value)
		if err != nil {
			return "0"
		}
		// Windows FILETIME: 100-nanosecond intervals since 1601-01-01
		epoch := time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)
		return fmt.Sprintf("%d", (t.Unix()-epoch.Unix())*10000000)
	case strings.Contains(rule, "="):
		// 通用值映射: "sourceVal1=targetVal1,sourceVal2=targetVal2"
		for _, pair := range strings.Split(rule, ",") {
			kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
			if len(kv) == 2 && strings.TrimSpace(kv[0]) == value {
				return strings.TrimSpace(kv[1])
			}
		}
		return value
	}
	return value
}

func applyExpression(expr string, user models.User) string {
	// 简单模板替换
	result := expr
	result = strings.ReplaceAll(result, "{{.username}}", user.Username)
	result = strings.ReplaceAll(result, "{{.nickname}}", user.Nickname)
	result = strings.ReplaceAll(result, "{{.email}}", user.Email)
	result = strings.ReplaceAll(result, "{{.phone}}", user.Phone)
	result = strings.ReplaceAll(result, "{{.jobTitle}}", user.JobTitle)
	result = strings.ReplaceAll(result, "{{.departmentName}}", user.DepartmentName)
	return result
}

// logSync 记录同步日志
// formatChangeDetail 将 SyncResult 的变更明细格式化为可读文本
func formatChangeDetail(r SyncResult) string {
	var parts []string
	if len(r.Created) > 0 {
		parts = append(parts, "新增: "+joinWithLimit(r.Created, 20))
	}
	if len(r.Deleted) > 0 {
		parts = append(parts, "删除: "+joinWithLimit(r.Deleted, 20))
	}
	if len(r.Disabled) > 0 {
		parts = append(parts, "禁用: "+joinWithLimit(r.Disabled, 20))
	}
	if len(r.Updated) > 0 {
		parts = append(parts, "更新: "+joinWithLimit(r.Updated, 20))
	}
	return strings.Join(parts, "\n")
}

func joinWithLimit(names []string, limit int) string {
	if len(names) <= limit {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:limit], ", ") + fmt.Sprintf(" 等%d人", len(names))
}

func logSync(syncID uint, connID uint, direction, triggerType, event string, userID uint, username, status, message string, affected int, duration int64) {
	logSyncWithDetail(syncID, connID, direction, triggerType, event, userID, username, status, message, "", affected, duration)
}

func logSyncWithDetail(syncID uint, connID uint, direction, triggerType, event string, userID uint, username, status, message, detail string, affected int, duration int64) {
	log.Printf("[同步] syncID=%d connID=%d dir=%s trigger=%s event=%s user=%s status=%s msg=%s", syncID, connID, direction, triggerType, event, username, status, message)
	if detail != "" {
		log.Printf("[同步] 错误详情(前几条):\n%s", detail)
	}
	syncLog := models.SyncLog{
		SynchronizerID: syncID,
		ConnectorID:    connID,
		Direction:      direction,
		TriggerType:    triggerType,
		TriggerEvent:   event,
		UserID:         userID,
		Username:       username,
		Status:         status,
		Message:        message,
		Detail:         detail,
		AffectedCount:  affected,
		Duration:       duration,
	}
	storage.DB.Create(&syncLog)
	
	// 广播同步日志事件（供前端实时刷新）
	broadcastSyncLog(syncLog)
}

// ========== 事件分发 ==========

// DispatchSyncEventSync 同步版本的事件分发（阻塞等待完成，用于删除前触发）
func DispatchSyncEventSync(event string, user models.User, rawPassword string) {
	// 1. 旧同步器
	var synchronizers []models.Synchronizer
	storage.DB.Where("status = ? AND enable_event = ?", 1, true).Find(&synchronizers)
	for _, syncr := range synchronizers {
		var events []string
		if json.Unmarshal([]byte(syncr.Events), &events) != nil { continue }
		for _, e := range events {
			if e == event {
				log.Printf("[同步] 触发事件同步(同步): syncer=%s event=%s user=%s", syncr.Name, event, user.Username)
				ExecuteSync(syncr, user, event, rawPassword)
				break
			}
		}
	}
	// 2. 新同步规则
	var rules []models.SyncRule
	storage.DB.Where("status = ? AND enable_event = ? AND direction = ?", 1, true, "downstream").Find(&rules)
	for _, rule := range rules {
		var events []string
		if json.Unmarshal([]byte(rule.Events), &events) != nil { continue }
		for _, e := range events {
			if e == event {
				log.Printf("[同步] 触发下游规则同步(同步): rule=%s event=%s user=%s", rule.Name, event, user.Username)
				ExecuteSyncRule(rule, user, event, rawPassword)
				break
			}
		}
	}
}

// DispatchSyncEvent 分发同步事件（异步）
// 同时查询旧的 Synchronizer 表和新的 SyncRule 表
func DispatchSyncEvent(event string, userID uint, rawPassword string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[同步] 事件处理panic: %v", r)
			}
		}()

		time.Sleep(500 * time.Millisecond)

		var user models.User
		if err := storage.DB.Preload("Roles").First(&user, userID).Error; err != nil {
			log.Printf("[同步] 事件分发失败，用户不存在: %d", userID)
			return
		}

		log.Printf("[同步] 事件分发: event=%s user=%s(id=%d) status=%d", event, user.Username, userID, user.Status)

		// 1. 旧同步器 (Synchronizer)
		var synchronizers []models.Synchronizer
		storage.DB.Where("status = ? AND enable_event = ?", 1, true).Find(&synchronizers)
		log.Printf("[同步] 找到 %d 个旧同步器(enable_event=true)", len(synchronizers))
		for _, syncr := range synchronizers {
			var events []string
			if err := json.Unmarshal([]byte(syncr.Events), &events); err != nil {
				log.Printf("[同步] 同步器 %s 事件配置解析失败: %v", syncr.Name, err)
				continue
			}
			subscribed := false
			for _, e := range events {
				if e == event {
					subscribed = true
					break
				}
			}
			if !subscribed {
				log.Printf("[同步] 同步器 %s 未订阅事件 %s (已订阅: %v)", syncr.Name, event, events)
				continue
			}
			log.Printf("[同步] 触发事件同步: syncer=%s event=%s user=%s", syncr.Name, event, user.Username)
			ExecuteSync(syncr, user, event, rawPassword)
		}

		// 2. 新同步规则 (SyncRule) - 仅下游
		var rules []models.SyncRule
		storage.DB.Where("status = ? AND enable_event = ? AND direction = ?", 1, true, "downstream").Find(&rules)
		log.Printf("[同步] 找到 %d 个下游同步规则(enable_event=true)", len(rules))
		for _, rule := range rules {
			var events []string
			if err := json.Unmarshal([]byte(rule.Events), &events); err != nil {
				continue
			}
			subscribed := false
			for _, e := range events {
				if e == event {
					subscribed = true
					break
				}
			}
			if !subscribed {
				log.Printf("[同步] 下游规则 %s 未订阅事件 %s (已订阅: %v)", rule.Name, event, events)
				continue
			}
			log.Printf("[同步] 触发下游规则同步: rule=%s event=%s user=%s", rule.Name, event, user.Username)
			ExecuteSyncRule(rule, user, event, rawPassword)
		}
	}()
}

// ExecuteSyncRule 执行同步规则（单用户事件触发 - 下游）
func ExecuteSyncRule(rule models.SyncRule, user models.User, event string, rawPassword string) {
	start := time.Now()
	var conn models.Connector
	if err := storage.DB.First(&conn, rule.ConnectorID).Error; err != nil {
		logSyncRule(rule.ID, conn.ID, "downstream", "event", event, user.ID, user.Username, "failed", "连接器不存在", 0, time.Since(start).Milliseconds())
		return
	}

	// 构造兼容的 Synchronizer 以复用现有下游逻辑
	syncr := models.Synchronizer{
		ID:                 rule.ID,
		Name:               rule.Name,
		ConnectorID:        rule.ConnectorID,
		Direction:          "push",
		SourceType:         rule.SourceType,
		TargetContainer:    rule.TargetContainer,
		RoleGroupContainer: rule.RoleGroupContainer,
		EnableSchedule:     rule.EnableSchedule,
		ScheduleTime:       rule.ScheduleTime,
		CronExpr:           rule.CronExpr,
		EnableEvent:        rule.EnableEvent,
		Events:             rule.Events,
		SyncUsers:          rule.SyncUsers,
		SyncGroups:         rule.SyncGroups,
		SyncRoles:          rule.SyncRoles,
		PreventPwdChange:   rule.PreventPwdChange,
		Status:             rule.Status,
	}

	ExecuteSync(syncr, user, event, rawPassword)
}

// ExecuteFullSyncRule 执行同步规则全量同步（下游）
func ExecuteFullSyncRule(rule models.SyncRule, triggerType string) SyncResult {
	syncr := models.Synchronizer{
		ID:               rule.ID,
		Name:             rule.Name,
		ConnectorID:      rule.ConnectorID,
		Direction:        "push",
		SourceType:       rule.SourceType,
		TargetContainer:  rule.TargetContainer,
		EnableSchedule:   rule.EnableSchedule,
		ScheduleTime:     rule.ScheduleTime,
		CronExpr:         rule.CronExpr,
		EnableEvent:      rule.EnableEvent,
		Events:           rule.Events,
		SyncUsers:        rule.SyncUsers,
		SyncGroups:       rule.SyncGroups,
		SyncRoles:        rule.SyncRoles,
		PreventPwdChange: rule.PreventPwdChange,
		Status:           rule.Status,
	}
	return ExecuteFullSync(syncr, triggerType)
}

// logSyncRule 记录同步规则日志
func logSyncRule(ruleID, connID uint, direction, triggerType, event string, userID uint, username, status, message string, affected int, duration int64) {
	log.Printf("[同步规则] ruleID=%d dir=%s trigger=%s event=%s user=%s status=%s msg=%s", ruleID, direction, triggerType, event, username, status, message)
	storage.DB.Create(&models.SyncLog{
		SyncRuleID:  ruleID,
		ConnectorID: connID,
		Direction:   direction,
		TriggerType: triggerType,
		TriggerEvent: event,
		UserID:      userID,
		Username:    username,
		Status:      status,
		Message:     message,
		AffectedCount: affected,
		Duration:    duration,
	})
}

// ========== 上游同步引擎 ==========

// UpstreamSyncResult 上游同步结果
type UpstreamSyncResult struct {
	DepartmentsSynced int              `json:"departmentsSynced"`
	GroupsDeleted     int              `json:"groupsDeleted"`
	UsersCreated      int              `json:"usersCreated"`
	UsersUpdated      int              `json:"usersUpdated"`
	UsersDisabled     int              `json:"usersDisabled"`
	UsersTotal        int              `json:"usersTotal"`
	Duration          string           `json:"duration"`
	Error             string           `json:"error,omitempty"`
	Details           []UpstreamDetail `json:"details,omitempty"`
}

type UpstreamDetail struct {
	RemoteUID  string `json:"remoteUid"`
	RemoteName string `json:"remoteName"`
	LocalUser  string `json:"localUser"`
	Department string `json:"department"`
	Action     string `json:"action"` // created / updated / skipped / failed / disabled
	Message    string `json:"message,omitempty"`
}

// hashPasswordWithFormat 根据指定格式哈希密码
func hashPasswordWithFormat(password, format string) (string, error) {
	return crypto.HashPassword(password, format)
}
