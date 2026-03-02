package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/middleware"
	"go-syncflow/internal/vpn"
)

func RegisterRoutes(r *gin.Engine) {
	r.Use(securityHeadersMiddleware())
	r.Use(corsMiddleware())
	r.Use(httpToHTTPSRedirectMiddleware())

	// OAuth2 / OpenID Connect（公开协议端点）
	r.GET("/.well-known/openid-configuration", OIDCDiscovery)
	r.GET("/oidc/jwks", OIDCJWKS)
	r.GET("/oauth/authorize", OAuthAuthorize)
	r.POST("/oauth/token", OAuthToken)
	r.GET("/oidc/userinfo", OIDCUserInfo)
	r.POST("/oidc/userinfo", OIDCUserInfo)

	// CAS（公开协议端点）
	r.GET("/cas", CASLogin)
	r.HEAD("/cas", CASLogin)
	r.GET("/cas/login", CASLogin)
	r.HEAD("/cas/login", CASLogin)
	r.GET("/cas/serviceValidate", CASServiceValidate)
	r.GET("/cas/logout", CASLogout)

	// SAML2（公开协议端点）
	r.GET("/saml/:appCode/metadata", SAMLMetadata)
	r.POST("/saml/:appCode/sso", SAMLSso)

	// 静态文件服务
	staticDir := "./static"
	if _, err := os.Stat(staticDir); err == nil {
		r.Static("/assets", filepath.Join(staticDir, "assets"))
		r.StaticFile("/favicon.ico", filepath.Join(staticDir, "favicon.ico"))

		r.NoRoute(func(c *gin.Context) {
			if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "接口不存在"})
				return
			}
			c.File(filepath.Join(staticDir, "index.html"))
		})
	}

	api := r.Group("/api")
	api.Use(middleware.IPWhitelistMiddleware())
	api.Use(middleware.RateLimitMiddleware())
	{
		// ========== 公开接口 ==========
		api.GET("/auth/csrf", GetLoginCSRFToken)
		api.POST("/auth/login", middleware.LoginRateLimitMiddleware(), Login)
		api.POST("/auth/dingtalk", DingTalkLogin) // 保留旧钉钉登录兼容
		api.POST("/auth/forgot-password/check", middleware.SensitiveRateLimitMiddleware(), ForgotPasswordCheck)
		api.POST("/auth/forgot-password/send-code", middleware.SensitiveRateLimitMiddleware(), ForgotPasswordSendCode)
		api.POST("/auth/forgot-password/reset", middleware.SensitiveRateLimitMiddleware(), ForgotPasswordReset)

		// 双因素认证（公开）
		api.POST("/auth/2fa/send", middleware.SensitiveRateLimitMiddleware(), Send2FACode)
		api.POST("/auth/2fa/verify", middleware.SensitiveRateLimitMiddleware(), Verify2FACode)
		api.GET("/settings/ui", GetUIConfig)
		api.GET("/settings/dingtalk/status", GetDingTalkStatus) // 保留兼容
		api.GET("/crypto/public-key", GetRSAPublicKey)

		// SSO 免登（公开）
		api.GET("/auth/sso-providers", GetSSOProviders)
		api.POST("/auth/sso/login", SSOLogin)

		// ========== 需要登录的接口 ==========
		auth := api.Group("")
		auth.Use(middleware.AuthMiddleware())
		{
			// 认证相关
			auth.POST("/auth/logout", Logout)
			auth.GET("/auth/info", GetUserInfo)
			auth.PUT("/auth/password", ChangePassword)

			// 文档（需登录）
			auth.GET("/docs", ListDocs)
			auth.GET("/docs/:name", DownloadDoc)

			// 个人中心
			auth.GET("/profile", GetProfile)
			auth.PUT("/profile", UpdateProfile)
			auth.PUT("/profile/password", ChangePasswordMultiMethod)
			auth.POST("/profile/verify-code", SendVerifyCode)

			// 权限树
			auth.GET("/permissions/tree", GetPermissionTree)

			// 消息策略查询
			auth.GET("/notify/policies/scene", GetPolicyByScene)

			// ========== 用户管理 ==========
			auth.GET("/users/export", middleware.PermissionAnyMiddleware("user:export", "settings:system"), ExportUsers)
			auth.GET("/users", middleware.PermissionMiddleware("user:list"), ListUsers)
			auth.POST("/users", middleware.PermissionMiddleware("user:create"), CreateUser)
			auth.GET("/users/:id", middleware.PermissionMiddleware("user:list"), GetUser)
			auth.PUT("/users/:id", middleware.PermissionMiddleware("user:update"), UpdateUser)
			auth.DELETE("/users/:id", middleware.PermissionMiddleware("user:delete"), DeleteUser)
			auth.PUT("/users/:id/status", middleware.PermissionAnyMiddleware("user:toggle_status", "user:update"), UpdateUserStatus)
			auth.PUT("/users/:id/reset-password", middleware.PermissionAnyMiddleware("user:reset_password", "user:update"), ResetUserPassword)
			auth.POST("/users/batch-reset-password", middleware.PermissionAnyMiddleware("user:reset_password", "user:update"), BatchResetPassword)
			auth.POST("/users/check-expired", middleware.PermissionMiddleware("user:update"), CheckExpiredUsers)

			// 用户分组
			auth.GET("/groups", middleware.PermissionMiddleware("user:list"), ListUserGroups)
			auth.POST("/groups", middleware.PermissionAnyMiddleware("user:create_group", "user:create"), CreateUserGroup)
			auth.PUT("/groups/:id", middleware.PermissionMiddleware("user:update"), UpdateUserGroup)
			auth.DELETE("/groups/:id", middleware.PermissionMiddleware("user:delete"), DeleteUserGroup)

			// ========== 角色管理 ==========
			auth.GET("/roles", middleware.PermissionAnyMiddleware("role:list", "user:list"), ListRoles)
			auth.POST("/roles", middleware.PermissionMiddleware("role:create"), CreateRole)
			auth.POST("/roles/auto-assign/apply", middleware.PermissionMiddleware("role:update"), ApplyAutoAssignRules)
			auth.GET("/roles/:id", middleware.PermissionMiddleware("role:list"), GetRole)
			auth.PUT("/roles/:id", middleware.PermissionMiddleware("role:update"), UpdateRole)
			auth.DELETE("/roles/:id", middleware.PermissionMiddleware("role:delete"), DeleteRole)
			auth.GET("/roles/:id/permissions", middleware.PermissionMiddleware("role:list"), GetRolePermissions)
			auth.PUT("/roles/:id/permissions", middleware.PermissionMiddleware("role:permission"), UpdateRolePermissions)
			auth.GET("/roles/:id/auto-assign", middleware.PermissionMiddleware("role:list"), GetRoleAutoAssignRules)
			auth.PUT("/roles/:id/auto-assign", middleware.PermissionMiddleware("role:update"), UpdateRoleAutoAssignRules)

			// ========== 单点登录（CAS/SAML/OAuth2/OIDC）==========
			auth.GET("/sso/apps", middleware.PermissionMiddleware("sso:apps"), ListSSOApps)
			auth.GET("/sso/apps/mine", ListMySSOApps)
			auth.POST("/sso/apps", middleware.PermissionMiddleware("sso:apps"), CreateSSOApp)
			auth.GET("/sso/apps/:id", middleware.PermissionMiddleware("sso:apps"), GetSSOApp)
			auth.PUT("/sso/apps/:id", middleware.PermissionMiddleware("sso:apps"), UpdateSSOApp)
			auth.DELETE("/sso/apps/:id", middleware.PermissionMiddleware("sso:apps"), DeleteSSOApp)
			auth.GET("/sso/apps/:id/owners", middleware.PermissionMiddleware("sso:apps"), ListSSOAppOwners)
			auth.PUT("/sso/apps/:id/owners", middleware.PermissionMiddleware("sso:apps"), PutSSOAppOwners)

			auth.GET("/sso/apps/:id/protocols", middleware.PermissionMiddleware("sso:integration"), GetSSOAppProtocols)
			auth.PUT("/sso/apps/:id/protocols", middleware.PermissionMiddleware("sso:integration"), PutSSOAppProtocols)

			auth.GET("/sso/apps/:id/mappings", middleware.SSOAppPermissionOrOwnerMiddleware("id", "manage_mappings", "sso:mappings", "sso:perms"), GetSSOAppMappings)
			auth.PUT("/sso/apps/:id/mappings", middleware.SSOAppPermissionOrOwnerMiddleware("id", "manage_mappings", "sso:mappings", "sso:perms"), PutSSOAppMappings)
			auth.GET("/sso/apps/:id/entitlements", middleware.SSOAppPermissionOrOwnerMiddleware("id", "manage_entitlements", "sso:mappings", "sso:perms"), ListSSOAppEntitlements)
			auth.POST("/sso/apps/:id/entitlements", middleware.SSOAppPermissionOrOwnerMiddleware("id", "manage_entitlements", "sso:mappings", "sso:perms"), CreateSSOAppEntitlement)
			auth.PUT("/sso/entitlements/:eid", middleware.SSOEntitlementPermissionOrOwnerMiddleware("eid", "manage_entitlements", "sso:mappings", "sso:perms"), UpdateSSOEntitlement)
			auth.DELETE("/sso/entitlements/:eid", middleware.SSOEntitlementPermissionOrOwnerMiddleware("eid", "manage_entitlements", "sso:mappings", "sso:perms"), DeleteSSOEntitlement)
			auth.GET("/sso/apps/:id/grants", middleware.SSOAppPermissionOrOwnerMiddleware("id", "manage_grants", "sso:mappings", "sso:perms"), ListSSOAppGrants)
			auth.PUT("/sso/apps/:id/grants", middleware.SSOAppPermissionOrOwnerMiddleware("id", "manage_grants", "sso:mappings", "sso:perms"), PutSSOAppGrants)
			auth.GET("/sso/apps/:id/templates", middleware.SSOAppPermissionOrOwnerMiddleware("id", "manage_templates", "sso:mappings", "sso:perms"), ListSSOAppTemplates)
			auth.PUT("/sso/apps/:id/templates", middleware.SSOAppPermissionOrOwnerMiddleware("id", "manage_templates", "sso:mappings", "sso:perms"), PutSSOAppTemplates)
			auth.GET("/sso/integration/overview", middleware.PermissionAnyMiddleware("sso:integration", "sso:owner"), GetSSOIntegrationOverview)
			auth.GET("/sso/logs", middleware.PermissionAnyMiddleware("sso:logs", "sso:owner"), ListSSOAuditLogs)

			// ========== 日志管理 ==========
			auth.GET("/logs/system", middleware.PermissionAnyMiddleware("log:login", "log:operation"), ListSystemLogs)
			auth.GET("/logs/system/stats", middleware.PermissionAnyMiddleware("log:login", "log:operation"), SystemLogStats)
			auth.GET("/logs/login", middleware.PermissionMiddleware("log:login"), ListLoginLogs)
			auth.GET("/logs/login/stats", middleware.PermissionMiddleware("log:login"), LoginLogStats)
			auth.GET("/logs/operation", middleware.PermissionMiddleware("log:operation"), ListOperationLogs)
			auth.GET("/logs/operation/stats", middleware.PermissionMiddleware("log:operation"), OperationLogStats)
			auth.GET("/logs/sync", middleware.PermissionMiddleware("log:operation"), ListAllSyncLogs)

			// API 调用日志
			auth.GET("/logs/api", middleware.PermissionAnyMiddleware("log:api", "settings:system"), ListAPIAccessLogs)
			auth.GET("/logs/api/stats", middleware.PermissionAnyMiddleware("log:api", "settings:system"), GetAPIAccessLogStats)

			// 日志设置
			auth.GET("/settings/log-retention", middleware.PermissionAnyMiddleware("log:settings", "settings:system"), GetLogRetention)
			auth.PUT("/settings/log-retention", middleware.PermissionAnyMiddleware("log:settings", "settings:system"), UpdateLogRetention)
			auth.POST("/settings/log-retention/clean", middleware.PermissionAnyMiddleware("log:settings", "settings:system"), CleanLogsNow)
			auth.GET("/settings/log-retention/stats", middleware.PermissionAnyMiddleware("log:settings", "settings:system"), GetLogRetentionStats)

			// ========== 系统设置 ==========
			auth.PUT("/settings/ui", middleware.PermissionMiddleware("settings:ui"), UpdateUIConfig)

			// 钉钉配置（保留旧接口兼容）
			auth.GET("/settings/dingtalk", middleware.PermissionMiddleware("settings:system"), GetDingTalkConfigFull)
			auth.PUT("/settings/dingtalk", middleware.PermissionMiddleware("settings:system"), UpdateDingTalkConfig)
			auth.POST("/settings/dingtalk/test", middleware.PermissionMiddleware("settings:system"), TestDingTalkConnection)

			// LDAP 配置
			auth.GET("/settings/ldap", middleware.PermissionMiddleware("settings:system"), GetLDAPConfig)
			auth.PUT("/settings/ldap", middleware.PermissionMiddleware("settings:system"), UpdateLDAPConfig)
			auth.POST("/settings/ldap/test", middleware.PermissionMiddleware("settings:system"), TestLDAPService)
			auth.GET("/settings/ldap/status", middleware.PermissionMiddleware("settings:system"), GetLDAPStatus)

			// HTTPS 配置
			auth.GET("/settings/https", middleware.PermissionMiddleware("settings:system"), GetHTTPSConfig)
			auth.PUT("/settings/https", middleware.PermissionMiddleware("settings:system"), UpdateHTTPSConfig)
			auth.POST("/settings/https/cert", middleware.PermissionMiddleware("settings:system"), UploadSSLCert)
			auth.DELETE("/settings/https/cert", middleware.PermissionMiddleware("settings:system"), DeleteSSLCert)

			// RSA 加密密钥配置
			auth.GET("/settings/crypto", middleware.PermissionMiddleware("settings:system"), GetRSACryptoConfig)
			auth.PUT("/settings/crypto", middleware.PermissionMiddleware("settings:system"), UpdateRSACryptoConfig)

			// 双因素认证配置
			auth.GET("/settings/2fa", middleware.PermissionMiddleware("settings:system"), Get2FAConfig)
			auth.PUT("/settings/2fa", middleware.PermissionMiddleware("settings:system"), Update2FAConfig)
			auth.POST("/settings/2fa/test", middleware.PermissionMiddleware("settings:system"), Test2FAConfig)

			// 系统状态
			auth.GET("/system/status", middleware.PermissionMiddleware("settings:system"), GetSystemStatus)

			// ========== 安全中心 ==========
			auth.GET("/security/dashboard", middleware.PermissionMiddleware("settings:system"), GetSecurityDashboard)
			auth.GET("/security/events", middleware.PermissionMiddleware("settings:system"), GetSecurityEvents)
			auth.PUT("/security/events/:id/resolve", middleware.PermissionMiddleware("settings:system"), ResolveSecurityEvent)
			auth.GET("/security/login-attempts", middleware.PermissionMiddleware("log:login"), GetLoginAttempts)
			auth.GET("/security/lockouts", middleware.PermissionMiddleware("settings:system"), GetLockouts)
			auth.POST("/security/lockouts/unlock-account", middleware.PermissionMiddleware("settings:system"), UnlockAccount)
			auth.POST("/security/lockouts/unlock-ip", middleware.PermissionMiddleware("settings:system"), UnlockIP)
			auth.GET("/security/ip/blacklist", middleware.PermissionMiddleware("settings:system"), GetIPBlacklist)
			auth.POST("/security/ip/blacklist", middleware.PermissionMiddleware("settings:system"), AddIPBlacklist)
			auth.DELETE("/security/ip/blacklist/:id", middleware.PermissionMiddleware("settings:system"), RemoveIPBlacklist)
			auth.GET("/security/ip/whitelist", middleware.PermissionMiddleware("settings:system"), GetIPWhitelist)
			auth.POST("/security/ip/whitelist", middleware.PermissionMiddleware("settings:system"), AddIPWhitelist)
			auth.DELETE("/security/ip/whitelist/:id", middleware.PermissionMiddleware("settings:system"), RemoveIPWhitelist)
			auth.GET("/security/ip/whitelist/mode", middleware.PermissionMiddleware("settings:system"), GetIPWhitelistMode)
			auth.POST("/security/ip/check", middleware.PermissionMiddleware("settings:system"), CheckIP)
			auth.GET("/security/sessions", middleware.PermissionMiddleware("settings:system"), GetAllSessions)
			auth.GET("/security/sessions/my", GetMySessions)
			auth.DELETE("/security/sessions/:id", middleware.PermissionMiddleware("settings:system"), TerminateSession)
			auth.DELETE("/security/sessions/user/:userId", middleware.PermissionMiddleware("settings:system"), TerminateUserSessions)
			auth.GET("/security/config", middleware.PermissionMiddleware("settings:system"), GetSecurityConfigs)
			auth.GET("/security/config/:key", middleware.PermissionMiddleware("settings:system"), GetSecurityConfig)
			auth.PUT("/security/config/:key", middleware.PermissionMiddleware("settings:system"), UpdateSecurityConfig)
			auth.GET("/security/alerts/channels", middleware.PermissionMiddleware("settings:system"), GetNotifyChannels)
			auth.POST("/security/alerts/channels", middleware.PermissionMiddleware("settings:system"), CreateNotifyChannel)
			auth.PUT("/security/alerts/channels/:id", middleware.PermissionMiddleware("settings:system"), UpdateNotifyChannel)
			auth.DELETE("/security/alerts/channels/:id", middleware.PermissionMiddleware("settings:system"), DeleteNotifyChannel)
			auth.POST("/security/alerts/channels/:id/test", middleware.PermissionMiddleware("settings:system"), TestNotifyChannel)
			auth.GET("/security/alerts/rules", middleware.PermissionMiddleware("settings:system"), GetAlertRules)
			auth.POST("/security/alerts/rules", middleware.PermissionMiddleware("settings:system"), CreateAlertRule)
			auth.PUT("/security/alerts/rules/:id", middleware.PermissionMiddleware("settings:system"), UpdateAlertRule)
			auth.DELETE("/security/alerts/rules/:id", middleware.PermissionMiddleware("settings:system"), DeleteAlertRule)
			auth.GET("/security/alerts/logs", middleware.PermissionMiddleware("settings:system"), GetAlertLogs)

			// ========== 消息模板管理 ==========
			auth.GET("/notify/templates", middleware.PermissionMiddleware("settings:system"), GetMessageTemplates)
			auth.GET("/notify/templates/:id", middleware.PermissionMiddleware("settings:system"), GetMessageTemplate)
			auth.POST("/notify/templates", middleware.PermissionMiddleware("settings:system"), CreateMessageTemplate)
			auth.PUT("/notify/templates/:id", middleware.PermissionMiddleware("settings:system"), UpdateMessageTemplate)
			auth.DELETE("/notify/templates/:id", middleware.PermissionMiddleware("settings:system"), DeleteMessageTemplate)

			// ========== 消息策略路由 ==========
			auth.GET("/notify/policies", middleware.PermissionMiddleware("settings:system"), GetMessagePolicies)
			auth.POST("/notify/policies", middleware.PermissionMiddleware("settings:system"), UpsertMessagePolicy)
			auth.PUT("/notify/policies/batch", middleware.PermissionMiddleware("settings:system"), BatchUpdateMessagePolicies)
			auth.POST("/notify/policies/group", middleware.PermissionMiddleware("settings:system"), CreateGroupPolicy)
			auth.PUT("/notify/policies/group/:id", middleware.PermissionMiddleware("settings:system"), UpdateGroupPolicy)
			auth.DELETE("/notify/policies/group/:id", middleware.PermissionMiddleware("settings:system"), DeleteGroupPolicy)

			// ========== 连接器管理（旧，保持兼容）==========
			auth.GET("/connectors", middleware.PermissionMiddleware("settings:system"), ListConnectors)
			auth.POST("/connectors", middleware.PermissionMiddleware("settings:system"), CreateConnector)
			auth.GET("/connectors/:id", middleware.PermissionMiddleware("settings:system"), GetConnector)
			auth.PUT("/connectors/:id", middleware.PermissionMiddleware("settings:system"), UpdateConnector)
			auth.DELETE("/connectors/:id", middleware.PermissionMiddleware("settings:system"), DeleteConnector)
			auth.POST("/connectors/:id/test", middleware.PermissionMiddleware("settings:system"), TestConnector)
			auth.GET("/connectors/:id/columns", middleware.PermissionMiddleware("settings:system"), DiscoverColumns)

			// ========== 同步器管理（旧，保持兼容）==========
			auth.GET("/synchronizers", middleware.PermissionMiddleware("settings:system"), ListSynchronizers)
			auth.POST("/synchronizers", middleware.PermissionMiddleware("settings:system"), CreateSynchronizer)
			auth.GET("/synchronizers/:id", middleware.PermissionMiddleware("settings:system"), GetSynchronizer)
			auth.PUT("/synchronizers/:id", middleware.PermissionMiddleware("settings:system"), UpdateSynchronizer)
			auth.DELETE("/synchronizers/:id", middleware.PermissionMiddleware("settings:system"), DeleteSynchronizer)
			auth.POST("/synchronizers/:id/trigger", middleware.PermissionMiddleware("settings:system"), TriggerSync)
			auth.GET("/synchronizers/:id/logs", middleware.PermissionMiddleware("settings:system"), GetSyncLogs)
			auth.GET("/synchronizers/:id/mappings", middleware.PermissionMiddleware("settings:system"), ListMappings)
			auth.POST("/synchronizers/:id/mappings", middleware.PermissionMiddleware("settings:system"), CreateMapping)
			auth.PUT("/synchronizers/:id/mappings/:mid", middleware.PermissionMiddleware("settings:system"), UpdateMapping)
			auth.DELETE("/synchronizers/:id/mappings/:mid", middleware.PermissionMiddleware("settings:system"), DeleteMapping)
			auth.PUT("/synchronizers/:id/mappings-batch", middleware.PermissionMiddleware("settings:system"), BatchUpdateMappings)
			auth.GET("/sync/events", middleware.PermissionMiddleware("settings:system"), GetSyncEvents)
			auth.GET("/sync/source-fields", middleware.PermissionMiddleware("settings:system"), GetSourceFields)
			auth.GET("/sync/target-fields", middleware.PermissionMiddleware("settings:system"), GetTargetFields)

			// ========== 新同步管理 ==========
			syncPerm := middleware.PermissionAnyMiddleware("sync:upstream", "sync:downstream", "settings:system")

			// 上游连接器
			auth.GET("/sync/upstream/connectors", syncPerm, ListUpstreamConnectors)
			auth.POST("/sync/upstream/connectors", syncPerm, CreateUpstreamConnector)
			auth.GET("/sync/upstream/connectors/:id", syncPerm, GetUpstreamConnector)
			auth.PUT("/sync/upstream/connectors/:id", syncPerm, UpdateUpstreamConnector)
			auth.DELETE("/sync/upstream/connectors/:id", syncPerm, DeleteUpstreamConnector)
			auth.POST("/sync/upstream/connectors/:id/test", syncPerm, TestUpstreamConnector)
			auth.GET("/sync/upstream/connectors/:id/departments", syncPerm, GetUpstreamDepartments)
			auth.GET("/sync/upstream/connectors/:id/users", syncPerm, GetUpstreamUsers)

			// 上游同步规则
			auth.GET("/sync/upstream/rules", syncPerm, ListUpstreamRules)
			auth.POST("/sync/upstream/rules", syncPerm, CreateUpstreamRule)
			auth.GET("/sync/upstream/rules/:id", syncPerm, GetUpstreamRule)
			auth.PUT("/sync/upstream/rules/:id", syncPerm, UpdateUpstreamRule)
			auth.DELETE("/sync/upstream/rules/:id", syncPerm, DeleteUpstreamRule)
			auth.POST("/sync/upstream/rules/:id/trigger", syncPerm, TriggerUpstreamSync)
			auth.GET("/sync/upstream/rules/:id/mappings", syncPerm, ListUpstreamRuleMappings)
			auth.PUT("/sync/upstream/rules/:id/mappings", syncPerm, BatchUpdateUpstreamRuleMappings)
			auth.POST("/sync/upstream/rules/:id/mappings/reset", syncPerm, ResetUpstreamRuleMappings)

			// 下游连接器
			auth.GET("/sync/downstream/connectors", syncPerm, ListDownstreamConnectors)
			auth.POST("/sync/downstream/connectors", syncPerm, CreateDownstreamConnector)
			auth.GET("/sync/downstream/connectors/:id", syncPerm, GetDownstreamConnector)
			auth.PUT("/sync/downstream/connectors/:id", syncPerm, UpdateDownstreamConnector)
			auth.DELETE("/sync/downstream/connectors/:id", syncPerm, DeleteDownstreamConnector)
			auth.POST("/sync/downstream/connectors/:id/test", syncPerm, TestUpstreamConnector) // 复用测试逻辑
			auth.GET("/sync/downstream/connectors/:id/columns", syncPerm, DiscoverDownstreamColumns)

			// 下游同步规则
			auth.GET("/sync/downstream/rules", syncPerm, ListDownstreamRules)
			auth.POST("/sync/downstream/rules", syncPerm, CreateDownstreamRule)
			auth.GET("/sync/downstream/rules/:id", syncPerm, GetDownstreamRule)
			auth.PUT("/sync/downstream/rules/:id", syncPerm, UpdateDownstreamRule)
			auth.DELETE("/sync/downstream/rules/:id", syncPerm, DeleteDownstreamRule)
			auth.POST("/sync/downstream/rules/:id/trigger", syncPerm, TriggerDownstreamSync)
			auth.GET("/sync/downstream/rules/:id/mappings", syncPerm, ListDownstreamRuleMappings)
			auth.PUT("/sync/downstream/rules/:id/mappings", syncPerm, BatchUpdateDownstreamRuleMappings)

			// 连接器类型列表
			auth.GET("/sync/connector-types", syncPerm, GetConnectorTypes)

			// 全局同步触发
			auth.POST("/sync/trigger-all", middleware.PermissionAnyMiddleware("sync:trigger", "user:update", "settings:system"), TriggerAllSyncNew)

			// ========== 钉钉组织架构（保留旧接口兼容）==========
			auth.GET("/dingtalk/departments", middleware.PermissionAnyMiddleware("dingtalk:list", "sync:upstream"), GetDingTalkDepartments)
			auth.GET("/dingtalk/users", middleware.PermissionAnyMiddleware("dingtalk:list", "sync:upstream"), GetDingTalkUsers)
			auth.POST("/dingtalk/sync", middleware.PermissionAnyMiddleware("dingtalk:sync", "sync:trigger"), TriggerDingTalkSync)
			auth.GET("/dingtalk/sync/status", middleware.PermissionAnyMiddleware("dingtalk:list", "sync:upstream"), GetDingTalkSyncStatus)
			auth.GET("/dingtalk/settings", middleware.PermissionAnyMiddleware("dingtalk:sync", "sync:upstream"), GetDingTalkSyncSettings)
			auth.PUT("/dingtalk/settings", middleware.PermissionAnyMiddleware("dingtalk:sync", "sync:upstream"), UpdateDingTalkSyncSettings)

			// ========== API 密钥管理 ==========
			auth.GET("/apikeys", middleware.PermissionMiddleware("settings:system"), ListAPIKeys)
			auth.POST("/apikeys", middleware.PermissionMiddleware("settings:system"), CreateAPIKey)
			auth.GET("/apikeys/:id", middleware.PermissionMiddleware("settings:system"), GetAPIKey)
			auth.PUT("/apikeys/:id", middleware.PermissionMiddleware("settings:system"), UpdateAPIKey)
			auth.POST("/apikeys/:id/reset", middleware.PermissionMiddleware("settings:system"), ResetAPIKey)
			auth.PUT("/apikeys/:id/toggle", middleware.PermissionMiddleware("settings:system"), ToggleAPIKeyStatus)
			auth.DELETE("/apikeys/:id", middleware.PermissionMiddleware("settings:system"), DeleteAPIKey)

			// ========== 密码代理认证 ==========
			auth.GET("/settings/password-auth-proxy", middleware.PermissionMiddleware("settings:system"), GetPasswordAuthProxyConfig)
			auth.PUT("/settings/password-auth-proxy", middleware.PermissionMiddleware("settings:system"), UpdatePasswordAuthProxyConfig)
			auth.POST("/settings/password-auth-proxy/test", middleware.PermissionMiddleware("settings:system"), TestPasswordAuthProxy)

			// ========== 密码存储策略 ==========
			auth.GET("/settings/password-storage-policy", middleware.PermissionMiddleware("settings:system"), GetPasswordStoragePolicy)
			auth.PUT("/settings/password-storage-policy", middleware.PermissionMiddleware("settings:system"), UpdatePasswordStoragePolicy)

			// ========== VPN 管理 ==========
			vpn.RegisterRoutes(auth)
		}

		// ========== 开放 API（通过 AppID/AppKey 认证）==========
		openAPI := api.Group("/open")
		openAPI.Use(APIKeyAuthMiddleware())
		openAPI.Use(APIKeySafetyMiddleware())
		openAPI.Use(APIAccessLogMiddleware())
		{
			// ========== 用户认证（外部系统调用）==========
			openAPI.POST("/auth/authenticate", OpenAPIAuthenticate)

			// ========== 用户同步接口（含密码哈希，专用于下游同步）==========
			openAPI.GET("/sync/users", OpenAPIGetUsersWithPassword)
			openAPI.GET("/sync/users/:id", OpenAPIGetUserWithPassword)

			// ========== 用户管理 ==========
			openAPI.GET("/users", ListUsers)
			openAPI.GET("/users/export", ExportUsers)
			openAPI.POST("/users", CreateUser)
			openAPI.GET("/users/:id", GetUser)
			openAPI.PUT("/users/:id", UpdateUser)
			openAPI.DELETE("/users/:id", DeleteUser)
			openAPI.PUT("/users/:id/status", UpdateUserStatus)
			openAPI.PUT("/users/:id/reset-password", ResetUserPassword)
			openAPI.POST("/users/batch-reset-password", BatchResetPassword)

			// ========== 用户组管理 ==========
			openAPI.GET("/groups", ListUserGroups)
			openAPI.POST("/groups", CreateUserGroup)
			openAPI.PUT("/groups/:id", UpdateUserGroup)
			openAPI.DELETE("/groups/:id", DeleteUserGroup)

			// ========== 角色管理 ==========
			openAPI.GET("/roles", ListRoles)
			openAPI.POST("/roles", CreateRole)
			openAPI.GET("/roles/:id", GetRole)
			openAPI.PUT("/roles/:id", UpdateRole)
			openAPI.DELETE("/roles/:id", DeleteRole)
			openAPI.GET("/roles/:id/permissions", GetRolePermissions)
			openAPI.PUT("/roles/:id/permissions", UpdateRolePermissions)

			// ========== 单点登录 (SSO) ==========
			openAPI.GET("/sso/apps", ListSSOApps)
			openAPI.POST("/sso/apps", CreateSSOApp)
			openAPI.GET("/sso/apps/:id", GetSSOApp)
			openAPI.PUT("/sso/apps/:id", UpdateSSOApp)
			openAPI.DELETE("/sso/apps/:id", DeleteSSOApp)
			openAPI.GET("/sso/apps/:id/owners", ListSSOAppOwners)
			openAPI.PUT("/sso/apps/:id/owners", PutSSOAppOwners)
			openAPI.GET("/sso/apps/:id/protocols", GetSSOAppProtocols)
			openAPI.PUT("/sso/apps/:id/protocols", PutSSOAppProtocols)
			openAPI.GET("/sso/apps/:id/mappings", GetSSOAppMappings)
			openAPI.PUT("/sso/apps/:id/mappings", PutSSOAppMappings)
			openAPI.GET("/sso/apps/:id/entitlements", ListSSOAppEntitlements)
			openAPI.POST("/sso/apps/:id/entitlements", CreateSSOAppEntitlement)
			openAPI.PUT("/sso/entitlements/:eid", UpdateSSOEntitlement)
			openAPI.DELETE("/sso/entitlements/:eid", DeleteSSOEntitlement)
			openAPI.GET("/sso/apps/:id/grants", ListSSOAppGrants)
			openAPI.PUT("/sso/apps/:id/grants", PutSSOAppGrants)
			openAPI.GET("/sso/apps/:id/templates", ListSSOAppTemplates)
			openAPI.PUT("/sso/apps/:id/templates", PutSSOAppTemplates)
			openAPI.GET("/sso/integration/overview", GetSSOIntegrationOverview)
			openAPI.GET("/sso/logs", ListSSOAuditLogs)

			// ========== 日志管理 ==========
			openAPI.GET("/logs/system", ListSystemLogs)
			openAPI.GET("/logs/system/stats", SystemLogStats)
			openAPI.GET("/logs/login", ListLoginLogs)
			openAPI.GET("/logs/login/stats", LoginLogStats)
			openAPI.GET("/logs/operation", ListOperationLogs)
			openAPI.GET("/logs/operation/stats", OperationLogStats)
			openAPI.GET("/logs/sync", ListAllSyncLogs)
			openAPI.GET("/logs/api", ListAPIAccessLogs)
			openAPI.GET("/logs/api/stats", GetAPIAccessLogStats)

			// ========== 日志设置 ==========
			openAPI.GET("/settings/log-retention", GetLogRetention)
			openAPI.PUT("/settings/log-retention", UpdateLogRetention)
			openAPI.POST("/settings/log-retention/clean", CleanLogsNow)
			openAPI.GET("/settings/log-retention/stats", GetLogRetentionStats)

			// ========== 系统设置 ==========
			openAPI.GET("/settings/ui", GetUIConfig)
			openAPI.PUT("/settings/ui", UpdateUIConfig)
			openAPI.GET("/settings/ldap", GetLDAPConfig)
			openAPI.PUT("/settings/ldap", UpdateLDAPConfig)
			openAPI.POST("/settings/ldap/test", TestLDAPService)
			openAPI.GET("/settings/ldap/status", GetLDAPStatus)
			openAPI.GET("/settings/https", GetHTTPSConfig)
			openAPI.PUT("/settings/https", UpdateHTTPSConfig)
			openAPI.POST("/settings/https/cert", UploadSSLCert)
			openAPI.DELETE("/settings/https/cert", DeleteSSLCert)
			openAPI.GET("/settings/crypto", GetRSACryptoConfig)
			openAPI.PUT("/settings/crypto", UpdateRSACryptoConfig)
			openAPI.GET("/settings/2fa", Get2FAConfig)
			openAPI.PUT("/settings/2fa", Update2FAConfig)
			openAPI.POST("/settings/2fa/test", Test2FAConfig)
			openAPI.GET("/system/status", GetSystemStatus)

			// ========== 安全中心 ==========
			openAPI.GET("/security/dashboard", GetSecurityDashboard)
			openAPI.GET("/security/events", GetSecurityEvents)
			openAPI.PUT("/security/events/:id/resolve", ResolveSecurityEvent)
			openAPI.GET("/security/login-attempts", GetLoginAttempts)
			openAPI.GET("/security/lockouts", GetLockouts)
			openAPI.POST("/security/lockouts/unlock-account", UnlockAccount)
			openAPI.POST("/security/lockouts/unlock-ip", UnlockIP)
			openAPI.GET("/security/ip/blacklist", GetIPBlacklist)
			openAPI.POST("/security/ip/blacklist", AddIPBlacklist)
			openAPI.DELETE("/security/ip/blacklist/:id", RemoveIPBlacklist)
			openAPI.GET("/security/ip/whitelist", GetIPWhitelist)
			openAPI.POST("/security/ip/whitelist", AddIPWhitelist)
			openAPI.DELETE("/security/ip/whitelist/:id", RemoveIPWhitelist)
			openAPI.GET("/security/ip/whitelist/mode", GetIPWhitelistMode)
			openAPI.POST("/security/ip/check", CheckIP)
			openAPI.GET("/security/sessions", GetAllSessions)
			openAPI.DELETE("/security/sessions/:id", TerminateSession)
			openAPI.DELETE("/security/sessions/user/:userId", TerminateUserSessions)
			openAPI.GET("/security/config", GetSecurityConfigs)
			openAPI.GET("/security/config/:key", GetSecurityConfig)
			openAPI.PUT("/security/config/:key", UpdateSecurityConfig)

			// ========== 通知渠道 ==========
			openAPI.GET("/security/alerts/channels", GetNotifyChannels)
			openAPI.POST("/security/alerts/channels", CreateNotifyChannel)
			openAPI.PUT("/security/alerts/channels/:id", UpdateNotifyChannel)
			openAPI.DELETE("/security/alerts/channels/:id", DeleteNotifyChannel)
			openAPI.POST("/security/alerts/channels/:id/test", TestNotifyChannel)
			openAPI.GET("/security/alerts/rules", GetAlertRules)
			openAPI.POST("/security/alerts/rules", CreateAlertRule)
			openAPI.PUT("/security/alerts/rules/:id", UpdateAlertRule)
			openAPI.DELETE("/security/alerts/rules/:id", DeleteAlertRule)
			openAPI.GET("/security/alerts/logs", GetAlertLogs)

			// ========== 消息模板 ==========
			openAPI.GET("/notify/templates", GetMessageTemplates)
			openAPI.GET("/notify/templates/:id", GetMessageTemplate)
			openAPI.POST("/notify/templates", CreateMessageTemplate)
			openAPI.PUT("/notify/templates/:id", UpdateMessageTemplate)
			openAPI.DELETE("/notify/templates/:id", DeleteMessageTemplate)

			// ========== 消息策略 ==========
			openAPI.GET("/notify/policies", GetMessagePolicies)
			openAPI.POST("/notify/policies", UpsertMessagePolicy)
			openAPI.PUT("/notify/policies/batch", BatchUpdateMessagePolicies)
			openAPI.POST("/notify/policies/group", CreateGroupPolicy)
			openAPI.PUT("/notify/policies/group/:id", UpdateGroupPolicy)
			openAPI.DELETE("/notify/policies/group/:id", DeleteGroupPolicy)

			// ========== 连接器管理 ==========
			openAPI.GET("/connectors", ListConnectors)
			openAPI.POST("/connectors", CreateConnector)
			openAPI.GET("/connectors/:id", GetConnector)
			openAPI.PUT("/connectors/:id", UpdateConnector)
			openAPI.DELETE("/connectors/:id", DeleteConnector)
			openAPI.POST("/connectors/:id/test", TestConnector)
			openAPI.GET("/connectors/:id/columns", DiscoverColumns)

			// ========== 同步器管理 ==========
			openAPI.GET("/synchronizers", ListSynchronizers)
			openAPI.POST("/synchronizers", CreateSynchronizer)
			openAPI.GET("/synchronizers/:id", GetSynchronizer)
			openAPI.PUT("/synchronizers/:id", UpdateSynchronizer)
			openAPI.DELETE("/synchronizers/:id", DeleteSynchronizer)
			openAPI.POST("/synchronizers/:id/trigger", TriggerSync)
			openAPI.GET("/synchronizers/:id/logs", GetSyncLogs)
			openAPI.GET("/synchronizers/:id/mappings", ListMappings)
			openAPI.POST("/synchronizers/:id/mappings", CreateMapping)
			openAPI.PUT("/synchronizers/:id/mappings/:mid", UpdateMapping)
			openAPI.DELETE("/synchronizers/:id/mappings/:mid", DeleteMapping)
			openAPI.PUT("/synchronizers/:id/mappings-batch", BatchUpdateMappings)
			openAPI.GET("/sync/events", GetSyncEvents)
			openAPI.GET("/sync/source-fields", GetSourceFields)
			openAPI.GET("/sync/target-fields", GetTargetFields)

			// ========== 新同步管理 ==========
			// 上游连接器
			openAPI.GET("/sync/upstream/connectors", ListUpstreamConnectors)
			openAPI.POST("/sync/upstream/connectors", CreateUpstreamConnector)
			openAPI.GET("/sync/upstream/connectors/:id", GetUpstreamConnector)
			openAPI.PUT("/sync/upstream/connectors/:id", UpdateUpstreamConnector)
			openAPI.DELETE("/sync/upstream/connectors/:id", DeleteUpstreamConnector)
			openAPI.POST("/sync/upstream/connectors/:id/test", TestUpstreamConnector)
			openAPI.GET("/sync/upstream/connectors/:id/departments", GetUpstreamDepartments)
			openAPI.GET("/sync/upstream/connectors/:id/users", GetUpstreamUsers)
			// 上游同步规则
			openAPI.GET("/sync/upstream/rules", ListUpstreamRules)
			openAPI.POST("/sync/upstream/rules", CreateUpstreamRule)
			openAPI.GET("/sync/upstream/rules/:id", GetUpstreamRule)
			openAPI.PUT("/sync/upstream/rules/:id", UpdateUpstreamRule)
			openAPI.DELETE("/sync/upstream/rules/:id", DeleteUpstreamRule)
			openAPI.POST("/sync/upstream/rules/:id/trigger", TriggerUpstreamSync)
			openAPI.GET("/sync/upstream/rules/:id/mappings", ListUpstreamRuleMappings)
			openAPI.PUT("/sync/upstream/rules/:id/mappings", BatchUpdateUpstreamRuleMappings)
			openAPI.POST("/sync/upstream/rules/:id/mappings/reset", ResetUpstreamRuleMappings)
			// 下游连接器
			openAPI.GET("/sync/downstream/connectors", ListDownstreamConnectors)
			openAPI.POST("/sync/downstream/connectors", CreateDownstreamConnector)
			openAPI.GET("/sync/downstream/connectors/:id", GetDownstreamConnector)
			openAPI.PUT("/sync/downstream/connectors/:id", UpdateDownstreamConnector)
			openAPI.DELETE("/sync/downstream/connectors/:id", DeleteDownstreamConnector)
			openAPI.POST("/sync/downstream/connectors/:id/test", TestUpstreamConnector)
			openAPI.GET("/sync/downstream/connectors/:id/columns", DiscoverDownstreamColumns)
			// 下游同步规则
			openAPI.GET("/sync/downstream/rules", ListDownstreamRules)
			openAPI.POST("/sync/downstream/rules", CreateDownstreamRule)
			openAPI.GET("/sync/downstream/rules/:id", GetDownstreamRule)
			openAPI.PUT("/sync/downstream/rules/:id", UpdateDownstreamRule)
			openAPI.DELETE("/sync/downstream/rules/:id", DeleteDownstreamRule)
			openAPI.POST("/sync/downstream/rules/:id/trigger", TriggerDownstreamSync)
			openAPI.GET("/sync/downstream/rules/:id/mappings", ListDownstreamRuleMappings)
			openAPI.PUT("/sync/downstream/rules/:id/mappings", BatchUpdateDownstreamRuleMappings)
			// 连接器类型
			openAPI.GET("/sync/connector-types", GetConnectorTypes)
			// 全局同步触发
			openAPI.POST("/sync/trigger-all", TriggerAllSyncNew)

			// ========== 钉钉（兼容旧接口）==========
			openAPI.GET("/dingtalk/departments", GetDingTalkDepartments)
			openAPI.GET("/dingtalk/users", GetDingTalkUsers)
			openAPI.POST("/dingtalk/sync", TriggerDingTalkSync)
			openAPI.GET("/dingtalk/sync/status", GetDingTalkSyncStatus)
			openAPI.GET("/dingtalk/settings", GetDingTalkSyncSettings)
			openAPI.PUT("/dingtalk/settings", UpdateDingTalkSyncSettings)

			// ========== API 密钥管理 ==========
			openAPI.GET("/apikeys", ListAPIKeys)
			openAPI.POST("/apikeys", CreateAPIKey)
			openAPI.GET("/apikeys/:id", GetAPIKey)
			openAPI.PUT("/apikeys/:id", UpdateAPIKey)
			openAPI.POST("/apikeys/:id/reset", ResetAPIKey)
			openAPI.PUT("/apikeys/:id/toggle", ToggleAPIKeyStatus)
			openAPI.DELETE("/apikeys/:id", DeleteAPIKey)

			// ========== 密码代理认证 ==========
			openAPI.GET("/settings/password-auth-proxy", GetPasswordAuthProxyConfig)
			openAPI.PUT("/settings/password-auth-proxy", UpdatePasswordAuthProxyConfig)
			openAPI.POST("/settings/password-auth-proxy/test", TestPasswordAuthProxy)

			// ========== VPN 管理 ==========
			vpn.RegisterOpenAPIRoutes(openAPI)
		}
	}
}

func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://g.alicdn.com https://*.dingtalk.com https://open.feishu.cn https://open.work.weixin.qq.com; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' ws: wss: https://*.dingtalk.com https://oapi.dingtalk.com https://open.feishu.cn https://qyapi.weixin.qq.com; frame-ancestors 'self' https://*.dingtalk.com https://*.feishu.cn")
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
			c.Header("Pragma", "no-cache")
		}
		c.Next()
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// httpToHTTPSRedirectMiddleware 当 HTTPS 已启用时，将浏览器页面请求
// 从 HTTP 重定向到 HTTPS。API 请求不受影响。
func httpToHTTPSRedirectMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			c.Next()
			return
		}

		cfg := GetHTTPSConfigForServer()
		if cfg == nil {
			c.Next()
			return
		}

		accept := c.GetHeader("Accept")
		isBrowser := strings.Contains(accept, "text/html")
		if !isBrowser {
			c.Next()
			return
		}

		host := c.Request.Host
		if h, _, found := strings.Cut(host, ":"); found {
			host = h
		}
		target := fmt.Sprintf("https://%s:%s%s", host, cfg.Port, c.Request.RequestURI)
		c.Redirect(http.StatusFound, target)
		c.Abort()
	}
}
