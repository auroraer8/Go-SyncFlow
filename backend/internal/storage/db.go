package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"go-syncflow/internal/models"
)

// DBConfig 数据库配置（仅支持 PostgreSQL）
type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string // PostgreSQL SSL 模式
	MaxIdle  int    // 最大空闲连接数
	MaxOpen  int    // 最大打开连接数
}

// CurrentDBType 当前数据库类型
var CurrentDBType = "postgresql"

// HashPasswordForStorage 将密码哈希为存储格式：bcrypt(SHA256(password))
func HashPasswordForStorage(password string) (string, error) {
	h := sha256.Sum256([]byte(password))
	sha256Hash := hex.EncodeToString(h[:])
	hashed, err := bcrypt.GenerateFromPassword([]byte(sha256Hash), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

var DB *gorm.DB

// InitDB 初始化数据库（使用默认 PostgreSQL 配置）
func InitDB() error {
	return InitDBWithConfig(DBConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "syncflow",
		Password: "syncflow",
		Database: "go_syncflow",
		SSLMode:  "disable",
	})
}

// InitDBWithConfig 使用配置初始化数据库（仅支持 PostgreSQL）
func InitDBWithConfig(cfg DBConfig) error {
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, sslMode)
	dialector := postgres.Open(dsn)
	CurrentDBType = "postgresql"
	log.Printf("[DB] 使用 PostgreSQL 数据库: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return err
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err == nil {
		maxIdle := cfg.MaxIdle
		if maxIdle <= 0 {
			maxIdle = 10
		}
		maxOpen := cfg.MaxOpen
		if maxOpen <= 0 {
			maxOpen = 100
		}
		sqlDB.SetMaxIdleConns(maxIdle)
		sqlDB.SetMaxOpenConns(maxOpen)
		sqlDB.SetConnMaxLifetime(time.Hour)
		log.Printf("[DB] 连接池配置: MaxIdle=%d, MaxOpen=%d", maxIdle, maxOpen)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.Permission{},
		&models.UserRole{},
		&models.RolePermission{},
		&models.LoginLog{},
		&models.OperationLog{},
		&models.SystemConfig{},
		&models.DingTalkDepartment{},
		&models.DingTalkUser{},
		&models.UserGroup{},
		&models.RoleAutoAssignRule{},
		// 安全相关表
		&models.LoginAttempt{},
		&models.Lockout{},
		&models.IPBlacklist{},
		&models.IPWhitelist{},
		&models.SecurityEvent{},
		&models.Session{},
		&models.SecurityConfig{},
		&models.NotifyChannel{},
		&models.AlertRule{},
		&models.AlertLog{},
		&models.MessageTemplate{},
		&models.MessagePolicy{},
		// API Key
		&models.APIKey{},
		// 连接器和同步器（保留旧表兼容）
		&models.Connector{},
		&models.Synchronizer{},
		&models.SyncAttributeMapping{},
		&models.SyncLog{},
		// === 新增表 ===
		&models.SyncRule{},
		&models.IMDepartment{},
		&models.IMUser{},
		&models.APIAccessLog{},
		// SSO（CAS/SAML/OAuth2/OIDC）
		&models.SSOApp{},
		&models.SSOAppProtocol{},
		&models.SSOAppMapping{},
		&models.SSOAuditLog{},
		&models.SSOOAuthCode{},
		&models.SSOCasTicket{},
		&models.SSOAppEntitlement{},
		&models.SSOAppGrant{},
		&models.SSOAppGrantItem{},
		&models.SSOAppTemplate{},
		&models.SSOAppOwner{},
	); err != nil {
		return err
	}

	DB = db

	// 移除 MessagePolicy.scene 的旧唯一索引
	db.Exec("DROP INDEX IF EXISTS idx_message_policies_scene")
	db.Exec("DROP INDEX IF EXISTS uni_message_policies_scene")

	// 修复 GORM 自动命名的列名 (imsso_priority → im_sso_priority)
	fixConnectorColumnNames(db)

	// 执行数据迁移（旧模型 → 新模型）
	migrateOldData(db)

	if err := initDefaultData(); err != nil {
		return err
	}

	return nil
}

// checkColumnExists 检查表中是否存在指定列（PostgreSQL）
func checkColumnExists(db *gorm.DB, table, column string) bool {
	var count int64
	db.Raw(`SELECT COUNT(*) FROM information_schema.columns 
			WHERE table_name = $1 AND column_name = $2`, table, column).Scan(&count)
	return count > 0
}

// fixConnectorColumnNames 修复 GORM 自动命名导致的列名不一致
// GORM 把 IMSSOPriority → imsso_priority，但我们需要 im_sso_priority
func fixConnectorColumnNames(db *gorm.DB) {
	// 检查是否存在旧列名
	if !checkColumnExists(db, "connectors", "imsso_priority") {
		return // 无旧列名，无需迁移
	}

	// 检查新列名是否已存在（由 AutoMigrate 先创建）
	if checkColumnExists(db, "connectors", "im_sso_priority") {
		// 新列已存在，只需迁移数据
		db.Exec("UPDATE connectors SET im_sso_priority = imsso_priority WHERE imsso_priority != 0 AND (im_sso_priority IS NULL OR im_sso_priority = 0)")
		db.Exec("UPDATE connectors SET im_sso_label = imsso_label WHERE imsso_label IS NOT NULL AND imsso_label != '' AND (im_sso_label IS NULL OR im_sso_label = '')")
		log.Println("[DB迁移] 已将旧列 imsso_priority/imsso_label 的数据迁移至新列")
	} else {
		// 新列不存在，直接重命名
		db.Exec("ALTER TABLE connectors RENAME COLUMN imsso_priority TO im_sso_priority")
		db.Exec("ALTER TABLE connectors RENAME COLUMN imsso_label TO im_sso_label")
		log.Println("[DB迁移] 修复连接器列名: imsso_priority → im_sso_priority")
	}
}

// migrateOldData 将旧的 Synchronizer → SyncRule，DingTalk 配置 → IM 连接器
func migrateOldData(db *gorm.DB) {
	// 1. 迁移旧 Synchronizer → SyncRule
	migrateSynchronizersToSyncRules(db)

	// 2. 迁移旧钉钉配置到 IM 连接器
	migrateDingTalkConfigToConnector(db)

	// 3. 迁移旧 DingTalk 缓存表到 IM 通用表
	migrateDingTalkCacheToIM(db)
}

// migrateSynchronizersToSyncRules 迁移同步器到同步规则
func migrateSynchronizersToSyncRules(db *gorm.DB) {
	var syncRuleCount int64
	db.Model(&models.SyncRule{}).Count(&syncRuleCount)
	if syncRuleCount > 0 {
		return // 已迁移过
	}

	var syncs []models.Synchronizer
	db.Find(&syncs)
	if len(syncs) == 0 {
		return
	}

	log.Println("[数据迁移] 迁移 Synchronizer → SyncRule ...")
	for _, s := range syncs {
		rule := models.SyncRule{
			Name:             s.Name,
			ConnectorID:      s.ConnectorID,
			Direction:        "downstream", // 旧同步器全部为下游
			EnableSchedule:   s.EnableSchedule,
			ScheduleTime:     s.ScheduleTime,
			CronExpr:         s.CronExpr,
			EnableEvent:      s.EnableEvent,
			Events:           s.Events,
			SyncUsers:        s.SyncUsers,
			SyncGroups:       s.SyncGroups,
			SyncRoles:        s.SyncRoles,
			SourceType:       s.SourceType,
			TargetContainer:  s.TargetContainer,
			PreventPwdChange: s.PreventPwdChange,
			Status:           s.Status,
			LastSyncAt:       s.LastSyncAt,
			LastSyncStatus:   s.LastSyncStatus,
			LastSyncMessage:  s.LastSyncMessage,
			SyncCount:        s.SyncCount,
		}
		if err := db.Create(&rule).Error; err != nil {
			log.Printf("[数据迁移] 迁移同步器 %s 失败: %v", s.Name, err)
			continue
		}

		// 迁移属性映射
		var mappings []models.SyncAttributeMapping
		db.Where("synchronizer_id = ?", s.ID).Find(&mappings)
		for _, m := range mappings {
			db.Model(&m).Update("sync_rule_id", rule.ID)
		}

		// 迁移同步日志
		db.Model(&models.SyncLog{}).Where("synchronizer_id = ?", s.ID).Updates(map[string]interface{}{
			"sync_rule_id": rule.ID,
			"direction":    "downstream",
		})

		log.Printf("[数据迁移] 同步器 %s → 同步规则 #%d", s.Name, rule.ID)
	}
}

// migrateDingTalkConfigToConnector 将旧钉钉配置迁移为 IM 连接器
func migrateDingTalkConfigToConnector(db *gorm.DB) {
	// 检查是否已存在 im_dingtalk 类型连接器
	var count int64
	db.Model(&models.Connector{}).Where("type = ?", "im_dingtalk").Count(&count)
	if count > 0 {
		return // 已迁移
	}

	// 读取旧钉钉配置
	var cfg models.SystemConfig
	if db.Where("key = ?", "dingtalk").First(&cfg).Error != nil {
		return // 没有旧配置
	}

	var dtCfg struct {
		Enabled       bool   `json:"enabled"`
		AppKey        string `json:"appKey"`
		AppSecret     string `json:"appSecret"`
		AgentID       string `json:"agentId"`
		CorpID        string `json:"corpId"`
		UsernameField string `json:"usernameField"`
		SyncInterval  int    `json:"syncInterval"`
		DefaultRoleID uint   `json:"defaultRoleId"`
		AutoRegister  bool   `json:"autoRegister"`
	}
	if json.Unmarshal([]byte(cfg.Value), &dtCfg) != nil {
		return
	}

	if dtCfg.AppKey == "" {
		return // 未配置
	}

	log.Println("[数据迁移] 迁移钉钉配置 → IM 连接器 ...")
	status := int8(0)
	if dtCfg.Enabled {
		status = 1
	}

	connector := models.Connector{
		Name:            "钉钉 (迁移)",
		Type:            "im_dingtalk",
		Direction:       "upstream",
		Status:          status,
		IMAppID:         dtCfg.AppKey,
		IMAppSecret:     dtCfg.AppSecret,
		IMAgentID:       dtCfg.AgentID,
		IMCorpID:        dtCfg.CorpID,
		IMMatchField:    dtCfg.UsernameField,
		IMSyncInterval:  dtCfg.SyncInterval,
		IMDefaultRoleID: dtCfg.DefaultRoleID,
		IMAutoRegister:  dtCfg.AutoRegister,
		IMUsernameRule:  "pinyin",
		IMEnableSSO:     dtCfg.Enabled, // 如果启用了就也开启SSO
		IMSSOLabel:      "钉钉登录",
	}
	if dtCfg.UsernameField == "" {
		connector.IMMatchField = "mobile"
	}

	if err := db.Create(&connector).Error; err != nil {
		log.Printf("[数据迁移] 迁移钉钉配置失败: %v", err)
		return
	}

	log.Printf("[数据迁移] 钉钉配置 → 连接器 #%d", connector.ID)
}

// migrateDingTalkCacheToIM 将旧钉钉缓存表迁移到 IM 通用表
func migrateDingTalkCacheToIM(db *gorm.DB) {
	// 找到迁移后的钉钉连接器
	var conn models.Connector
	if db.Where("type = ?", "im_dingtalk").First(&conn).Error != nil {
		return
	}

	// 检查是否已迁移
	var imDeptCount int64
	db.Model(&models.IMDepartment{}).Where("connector_id = ?", conn.ID).Count(&imDeptCount)
	if imDeptCount > 0 {
		return
	}

	// 迁移部门
	var depts []models.DingTalkDepartment
	db.Find(&depts)
	for _, d := range depts {
		imDept := models.IMDepartment{
			ConnectorID:  conn.ID,
			PlatformType: "im_dingtalk",
			RemoteDeptID: itoa64(d.DeptID),
			Name:         d.Name,
			ParentDeptID: itoa64(d.ParentID),
			SortOrder:    int(d.Order),
			MemberCount:  d.MemberCount,
		}
		db.Create(&imDept)
	}

	// 迁移用户
	var users []models.DingTalkUser
	db.Find(&users)
	for _, u := range users {
		imUser := models.IMUser{
			ConnectorID:    conn.ID,
			PlatformType:   "im_dingtalk",
			RemoteUserID:   u.DingTalkUID,
			Name:           u.Name,
			Mobile:         u.Mobile,
			Email:          u.Email,
			Avatar:         u.Avatar,
			JobTitle:       u.JobTitle,
			DepartmentID:   itoa64(u.DepartmentID),
			DepartmentName: u.DepartmentName,
			Active:         u.Active,
			LocalUserID:    u.LocalUserID,
		}
		db.Create(&imUser)
	}

	if len(depts) > 0 || len(users) > 0 {
		log.Printf("[数据迁移] 迁移钉钉缓存: %d 部门, %d 用户 → IM通用表", len(depts), len(users))
	}
}

func itoa64(v int64) string {
	return json.Number(json.Number(itoa64str(v))).String()
}

func itoa64str(v int64) string {
	if v == 0 {
		return "0"
	}
	s := ""
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	for v > 0 {
		s = string(rune('0'+v%10)) + s
		v /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

func initDefaultData() error {
	if err := initPermissions(); err != nil {
		return err
	}
	if err := initRoles(); err != nil {
		return err
	}
	if err := initAdmin(); err != nil {
		return err
	}
	if err := initConfigs(); err != nil {
		return err
	}
	if err := initSecurityConfigs(); err != nil {
		return err
	}
	return nil
}

func initPermissions() error {
	var count int64
	DB.Model(&models.Permission{}).Count(&count)
	if count > 0 {
		// 已有数据，补充新增的权限
		ensurePermissionExists(19, 11, "重置用户密码", "user:reset_password", "button", 5)
		ensurePermissionExists(70, 11, "启用/禁用用户", "user:toggle_status", "button", 6)
		ensurePermissionExists(71, 11, "新增群组", "user:create_group", "button", 7)
		ensurePermissionExists(72, 11, "分配角色", "user:assign_role", "button", 8)

		// 同步管理权限
		ensurePermissionExists(80, 0, "同步管理", "sync", "menu", 3)
		ensurePermissionExists(81, 80, "上游同步", "sync:upstream", "menu", 1)
		ensurePermissionExists(82, 80, "下游同步", "sync:downstream", "menu", 2)
		ensurePermissionExists(83, 80, "同步操作", "sync:trigger", "button", 3)

		// API 日志权限
		ensurePermissionExists(33, 30, "查看API日志", "log:api", "button", 3)
		ensurePermissionExists(34, 30, "日志设置", "log:settings", "button", 4)

		// 单点登录权限（多协议：CAS/SAML/OIDC）
		ensurePermissionExists(90, 0, "单点登录", "sso", "menu", 5)
		ensurePermissionExists(91, 90, "应用管理", "sso:apps", "menu", 1)
		ensurePermissionExists(92, 90, "权限管理", "sso:perms", "menu", 2)
		ensurePermissionExists(93, 90, "授权策略", "sso:mappings", "menu", 3)
		ensurePermissionExists(94, 90, "接入配置", "sso:integration", "menu", 4)
		ensurePermissionExists(95, 90, "登录日志", "sso:logs", "menu", 5)
		ensurePermissionExists(96, 90, "应用负责人", "sso:owner", "button", 6)

		// VPN 接入权限
		ensurePermissionExists(100, 0, "VPN接入", "vpn", "menu", 6)
		ensurePermissionExists(101, 100, "服务概览", "vpn:dashboard", "menu", 1)
		ensurePermissionExists(102, 100, "用户组管理", "vpn:groups", "menu", 2)
		ensurePermissionExists(103, 100, "在线用户", "vpn:online", "menu", 3)
		ensurePermissionExists(104, 100, "服务配置", "vpn:settings", "menu", 4)
		ensurePermissionExists(105, 100, "VPN日志", "vpn:logs", "menu", 5)

		// 通知管理权限
		ensurePermissionExists(41, 40, "通知渠道", "notify:channels", "menu", 1)
		ensurePermissionExists(42, 40, "消息模板", "notify:templates", "menu", 2)
		ensurePermissionExists(43, 40, "规则与策略", "notify:rules", "menu", 3)
		DB.Model(&models.Permission{}).Where("code = ?", "notify").Update("path", "/admin/notify")

		DB.Model(&models.Permission{}).Where("code = ?", "log").Update("sort", 7)
		DB.Model(&models.Permission{}).Where("code = ?", "notify").Update("sort", 8)
		DB.Model(&models.Permission{}).Where("code = ?", "settings").Update("sort", 9)
		DB.Model(&models.Permission{}).Where("code = ?", "security").Update("sort", 10)
		return nil
	}

	permissions := []models.Permission{
		// 系统首页
		{ID: 1, ParentID: 0, Name: "系统首页", Code: "admin:home", Type: "menu", Path: "/admin", Icon: "HomeFilled", Sort: 1},

		// 用户源
		{ID: 10, ParentID: 0, Name: "用户管理", Code: "usersource", Type: "menu", Path: "/admin/users", Icon: "User", Sort: 2},
		{ID: 11, ParentID: 10, Name: "本地用户", Code: "user", Type: "menu", Path: "/admin/users/local", Sort: 1},
		{ID: 12, ParentID: 11, Name: "查看用户", Code: "user:list", Type: "button", Sort: 1},
		{ID: 13, ParentID: 11, Name: "新增用户", Code: "user:create", Type: "button", Sort: 2},
		{ID: 14, ParentID: 11, Name: "编辑用户", Code: "user:update", Type: "button", Sort: 3},
		{ID: 15, ParentID: 11, Name: "删除用户", Code: "user:delete", Type: "button", Sort: 4},
		{ID: 19, ParentID: 11, Name: "重置用户密码", Code: "user:reset_password", Type: "button", Sort: 5},
		{ID: 70, ParentID: 11, Name: "启用/禁用用户", Code: "user:toggle_status", Type: "button", Sort: 6},
		{ID: 71, ParentID: 11, Name: "新增群组", Code: "user:create_group", Type: "button", Sort: 7},
		{ID: 72, ParentID: 11, Name: "分配角色", Code: "user:assign_role", Type: "button", Sort: 8},
		{ID: 73, ParentID: 11, Name: "导出用户", Code: "user:export", Type: "button", Sort: 9},

		// 同步管理
		{ID: 80, ParentID: 0, Name: "同步管理", Code: "sync", Type: "menu", Path: "/admin/sync", Icon: "Refresh", Sort: 3},
		{ID: 81, ParentID: 80, Name: "上游同步", Code: "sync:upstream", Type: "menu", Path: "/admin/sync/upstream", Sort: 1},
		{ID: 82, ParentID: 80, Name: "下游同步", Code: "sync:downstream", Type: "menu", Path: "/admin/sync/downstream", Sort: 2},
		{ID: 83, ParentID: 80, Name: "同步操作", Code: "sync:trigger", Type: "button", Sort: 3},

		// 角色管理（一级菜单）
		{ID: 20, ParentID: 0, Name: "角色管理", Code: "role", Type: "menu", Path: "/admin/roles", Icon: "UserFilled", Sort: 4},
		{ID: 21, ParentID: 20, Name: "查看角色", Code: "role:list", Type: "button", Sort: 1},
		{ID: 22, ParentID: 20, Name: "新增角色", Code: "role:create", Type: "button", Sort: 2},
		{ID: 23, ParentID: 20, Name: "编辑角色", Code: "role:update", Type: "button", Sort: 3},
		{ID: 24, ParentID: 20, Name: "删除角色", Code: "role:delete", Type: "button", Sort: 4},
		{ID: 25, ParentID: 20, Name: "分配权限", Code: "role:permission", Type: "button", Sort: 5},

		// 单点登录
		{ID: 90, ParentID: 0, Name: "单点登录", Code: "sso", Type: "menu", Path: "/admin/sso", Icon: "Link", Sort: 5},
		{ID: 91, ParentID: 90, Name: "应用管理", Code: "sso:apps", Type: "menu", Path: "/admin/sso/apps", Sort: 1},
		{ID: 92, ParentID: 90, Name: "权限管理", Code: "sso:perms", Type: "menu", Path: "/admin/sso/perms", Sort: 2},
		{ID: 93, ParentID: 90, Name: "授权策略", Code: "sso:mappings", Type: "menu", Path: "/admin/sso/mappings", Sort: 3},
		{ID: 94, ParentID: 90, Name: "接入配置", Code: "sso:integration", Type: "menu", Path: "/admin/sso/integration", Sort: 4},
		{ID: 95, ParentID: 90, Name: "登录日志", Code: "sso:logs", Type: "menu", Path: "/admin/sso/logs", Sort: 5},
		{ID: 96, ParentID: 90, Name: "应用负责人", Code: "sso:owner", Type: "button", Sort: 6},

		// VPN 接入
		{ID: 100, ParentID: 0, Name: "VPN接入", Code: "vpn", Type: "menu", Path: "/admin/vpn", Icon: "Connection", Sort: 6},
		{ID: 101, ParentID: 100, Name: "服务概览", Code: "vpn:dashboard", Type: "menu", Path: "/admin/vpn", Sort: 1},
		{ID: 102, ParentID: 100, Name: "用户组管理", Code: "vpn:groups", Type: "menu", Path: "/admin/vpn/groups", Sort: 2},
		{ID: 103, ParentID: 100, Name: "在线用户", Code: "vpn:online", Type: "menu", Path: "/admin/vpn/online", Sort: 3},
		{ID: 104, ParentID: 100, Name: "服务配置", Code: "vpn:settings", Type: "menu", Path: "/admin/vpn/settings", Sort: 4},
		{ID: 105, ParentID: 100, Name: "VPN日志", Code: "vpn:logs", Type: "menu", Path: "/admin/logs/vpn", Sort: 5},

		// 日志管理
		{ID: 30, ParentID: 0, Name: "日志管理", Code: "log", Type: "menu", Path: "/admin/logs", Icon: "Document", Sort: 7},
		{ID: 31, ParentID: 30, Name: "查看登录日志", Code: "log:login", Type: "button", Sort: 1},
		{ID: 32, ParentID: 30, Name: "查看操作日志", Code: "log:operation", Type: "button", Sort: 2},
		{ID: 33, ParentID: 30, Name: "查看API日志", Code: "log:api", Type: "button", Sort: 3},
		{ID: 34, ParentID: 30, Name: "日志设置", Code: "log:settings", Type: "button", Sort: 4},

		// 通知管理
		{ID: 40, ParentID: 0, Name: "通知管理", Code: "notify", Type: "menu", Path: "/admin/notify", Icon: "Bell", Sort: 8},
		{ID: 41, ParentID: 40, Name: "通知渠道", Code: "notify:channels", Type: "menu", Path: "/admin/notify/channels", Sort: 1},
		{ID: 42, ParentID: 40, Name: "消息模板", Code: "notify:templates", Type: "menu", Path: "/admin/notify/templates", Sort: 2},
		{ID: 43, ParentID: 40, Name: "规则与策略", Code: "notify:rules", Type: "menu", Path: "/admin/notify/rules", Sort: 3},

		// 系统设置
		{ID: 50, ParentID: 0, Name: "系统设置", Code: "settings", Type: "menu", Path: "/admin/settings", Icon: "Setting", Sort: 9},
		{ID: 51, ParentID: 50, Name: "界面配置", Code: "settings:ui", Type: "button", Sort: 1},
		{ID: 52, ParentID: 50, Name: "系统管理", Code: "settings:system", Type: "button", Sort: 2},

		// 安全中心
		{ID: 60, ParentID: 0, Name: "安全中心", Code: "security", Type: "menu", Path: "/admin/security", Icon: "Lock", Sort: 10},
	}

	return DB.Create(&permissions).Error
}

// ensurePermissionExists 确保指定权限存在（用于已有数据库的增量迁移）
func ensurePermissionExists(id, parentID uint, name, code, pType string, sort int) {
	var existing models.Permission
	if DB.Where("code = ?", code).First(&existing).Error == nil {
		return // 已存在
	}
	perm := models.Permission{
		ID:       id,
		ParentID: parentID,
		Name:     name,
		Code:     code,
		Type:     pType,
		Sort:     sort,
	}
	DB.Create(&perm)

	// 自动给超级管理员（role_id=1）添加新权限
	DB.Create(&models.RolePermission{RoleID: 1, PermissionID: perm.ID})
}

func initRoles() error {
	var count int64
	DB.Model(&models.Role{}).Count(&count)
	if count > 0 {
		return nil
	}

	adminRole := models.Role{
		ID:          1,
		Name:        "超级管理员",
		Code:        "super_admin",
		Description: "拥有所有权限",
		Status:      1,
	}
	if err := DB.Create(&adminRole).Error; err != nil {
		return err
	}

	var permissions []models.Permission
	DB.Find(&permissions)
	for _, p := range permissions {
		DB.Create(&models.RolePermission{RoleID: 1, PermissionID: p.ID})
	}

	userRole := models.Role{
		ID:          2,
		Name:        "普通用户",
		Code:        "user",
		Description: "默认无权限，需管理员分配",
		Status:      1,
	}
	if err := DB.Create(&userRole).Error; err != nil {
		return err
	}

	// 普通用户默认不分配任何权限，由管理员按需配置

	return nil
}

func initAdmin() error {
	var count int64
	DB.Model(&models.User{}).Count(&count)
	if count > 0 {
		return nil
	}

	hashed, _ := HashPasswordForStorage("Admin@2024")
	admin := models.User{
		ID:       1,
		Username: "admin",
		Password: hashed,
		Nickname: "管理员",
		Status:   1,
		Source:   "local",
	}
	if err := DB.Create(&admin).Error; err != nil {
		return err
	}

	return DB.Create(&models.UserRole{UserID: 1, RoleID: 1}).Error
}

func initConfigs() error {
	configs := []models.SystemConfig{
		{
			Key: "ui",
			Value: mustJSON(models.UIConfig{
				BrowserTitle:   "Go-SyncFlow",
				LoginTitle:     "Go-SyncFlow",
				LoginBrandName: "Go-SyncFlow",
				LoginSubtitle:  "欢迎使用统一用户管理平台",
				Logo:           "",
				FooterShortName: "Go-SyncFlow",
				FooterCompany:   "Go-SyncFlow",
				FooterICP:       "",
				FooterText:      "",
				AdminTheme:      "white",
				LoginTheme:      "light",
			}),
			Description: "界面配置",
		},
		{
			Key: "ldap",
			Value: mustJSON(models.LDAPConfig{
				Enabled:      true,
				Port:         389,
				TLSPort:      636,
				SambaEnabled: true,
			}),
			Description: "LDAP服务配置（默认启用Samba）",
		},
	}

	for _, cfg := range configs {
		var existing models.SystemConfig
		if DB.Where("key = ?", cfg.Key).First(&existing).Error == nil {
			// 对于已有的 LDAP 配置，确保 Samba 字段不为空
			if cfg.Key == "ldap" {
				var ldapCfg models.LDAPConfig
				json.Unmarshal([]byte(existing.Value), &ldapCfg)
				// 如果 SambaSID 为空但 Samba 已启用且有域名，自动生成 SID
				if ldapCfg.SambaEnabled && ldapCfg.SambaSID == "" && ldapCfg.Domain != "" {
					ldapCfg.SambaSID = generateSimpleDomainSID(ldapCfg.Domain)
					data, _ := json.Marshal(ldapCfg)
					DB.Model(&existing).Update("value", string(data))
				}
			}
			continue
		}
		if err := DB.Create(&cfg).Error; err != nil {
			return err
		}
	}
	return nil
}

// generateSimpleDomainSID 从域名生成一个确定性的域 SID
func generateSimpleDomainSID(domain string) string {
	h := sha256.Sum256([]byte(domain))
	a := uint32(h[0])<<24 | uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3])
	b := uint32(h[4])<<24 | uint32(h[5])<<16 | uint32(h[6])<<8 | uint32(h[7])
	c := uint32(h[8])<<24 | uint32(h[9])<<16 | uint32(h[10])<<8 | uint32(h[11])
	return fmt.Sprintf("S-1-5-21-%d-%d-%d", a, b, c)
}

func mustJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func GetConfig(key string) (string, error) {
	var cfg models.SystemConfig
	if err := DB.Where("key = ?", key).First(&cfg).Error; err != nil {
		return "", err
	}
	return cfg.Value, nil
}

func SetConfig(key, value string) error {
	var config models.SystemConfig
	result := DB.Where("key = ?", key).First(&config)
	if result.Error != nil {
		config = models.SystemConfig{
			Key:   key,
			Value: value,
		}
		return DB.Create(&config).Error
	}
	return DB.Model(&config).Update("value", value).Error
}

func initSecurityConfigs() error {
	configs := []models.SecurityConfig{
		{
			ConfigKey: "password_policy",
			ConfigValue: mustJSON(map[string]interface{}{
				"min_length":          8,
				"max_length":          128,
				"require_uppercase":   true,
				"require_lowercase":   true,
				"require_number":      true,
				"require_special":     false,
				"special_chars":       "!@#$%^&*()_+-=[]{}|;:,.<>?",
				"history_count":       5,
				"max_age_days":        0,
				"min_age_days":        0,
				"weak_password_check": true,
			}),
			Description: "密码策略配置",
		},
		{
			ConfigKey: "login_security",
			ConfigValue: mustJSON(map[string]interface{}{
				"account_lockout": map[string]interface{}{
					"enabled":                  true,
					"max_attempts":             5,
					"lockout_duration_minutes": 15,
					"progressive_lockout":      true,
					"second_lockout_minutes":   60,
					"third_lockout_minutes":    1440,
				},
				"ip_lockout": map[string]interface{}{
					"enabled":                true,
					"max_attempts":           20,
					"lockout_duration_hours": 24,
					"whitelist_bypass":       true,
				},
				"captcha": map[string]interface{}{
					"enabled":                false,
					"trigger_after_failures": 3,
				},
			}),
			Description: "登录安全配置",
		},
		{
			ConfigKey: "session",
			ConfigValue: mustJSON(map[string]interface{}{
				"access_token_ttl_minutes": 60,
				"refresh_token_ttl_days":   7,
				"max_concurrent_sessions":  5,
				"single_session_mode":      false,
				"ip_binding":               false,
				"idle_timeout_minutes":     30,
				"absolute_timeout_hours":   24,
			}),
			Description: "会话管理配置",
		},
		{
			ConfigKey: "ip_security",
			ConfigValue: mustJSON(map[string]interface{}{
				"blacklist_enabled": true,
				"whitelist_enabled": false,
				"auto_block": map[string]interface{}{
					"enabled":                 true,
					"threshold_failed_logins": 30,
					"block_duration_hours":    24,
					"permanent_after_blocks":  3,
				},
			}),
			Description: "IP安全配置",
		},
		{
			ConfigKey: "http_security",
			ConfigValue: mustJSON(map[string]interface{}{
				"headers": map[string]interface{}{
					"x_frame_options":        "SAMEORIGIN",
					"x_content_type_options": true,
					"x_xss_protection":       true,
				},
				"rate_limit": map[string]interface{}{
					"enabled":             true,
					"requests_per_minute": 120,
					"by_ip":               true,
				},
			}),
			Description: "HTTP安全配置",
		},
		{
			ConfigKey: "alerting",
			ConfigValue: mustJSON(map[string]interface{}{
				"global_enabled": false,
				"email": map[string]interface{}{
					"enabled":   false,
					"smtp_host": "",
					"smtp_port": 587,
					"smtp_tls":  true,
				},
				"webhook": map[string]interface{}{
					"enabled": false,
					"url":     "",
				},
				"sms": map[string]interface{}{
					"enabled":  false,
					"provider": "aliyun",
				},
			}),
			Description: "告警通知配置",
		},
	}

	for _, cfg := range configs {
		var existing models.SecurityConfig
		if DB.Where("config_key = ?", cfg.ConfigKey).First(&existing).Error == nil {
			continue
		}
		if err := DB.Create(&cfg).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetSecurityConfig 获取安全配置
func GetSecurityConfig(key string) (string, error) {
	var cfg models.SecurityConfig
	if err := DB.Where("config_key = ?", key).First(&cfg).Error; err != nil {
		return "", err
	}
	return cfg.ConfigValue, nil
}

// SetSecurityConfig 设置安全配置（不存在则创建）
func SetSecurityConfig(key, value string, updatedBy *uint) error {
	var cfg models.SecurityConfig
	err := DB.Where("config_key = ?", key).First(&cfg).Error
	if err != nil {
		// 不存在，创建新记录
		cfg = models.SecurityConfig{
			ConfigKey:   key,
			ConfigValue: value,
			UpdatedBy:   updatedBy,
		}
		return DB.Create(&cfg).Error
	}
	// 存在，更新
	return DB.Model(&cfg).Updates(map[string]interface{}{
		"config_value": value,
		"updated_by":   updatedBy,
	}).Error
}
