package ldapserver

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jimlambrt/gldap"
	"golang.org/x/crypto/bcrypt"

	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/services"
	"go-syncflow/internal/storage"
	syncer "go-syncflow/internal/sync"
)

// LDAPServer 管理内嵌的 LDAP 服务器
type LDAPServer struct {
	mu      sync.Mutex
	server  *gldap.Server
	tlsSrv  *gldap.Server // LDAPS 服务器
	config  models.LDAPConfig
	running bool
}

// NewLDAPServer 创建新的 LDAP 服务器实例
func NewLDAPServer() *LDAPServer {
	return &LDAPServer{}
}

// Start 启动 LDAP 服务器
func (s *LDAPServer) Start(config models.LDAPConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("LDAP 服务器已在运行")
	}

	s.config = config

	// 创建路由
	mux, err := gldap.NewMux()
	if err != nil {
		return fmt.Errorf("创建 LDAP 路由失败: %w", err)
	}
	mux.Bind(s.handleBind)
	mux.Search(s.handleSearch)

	// 启动 LDAP 服务器
	srv, err := gldap.NewServer()
	if err != nil {
		return fmt.Errorf("创建 LDAP 服务器失败: %w", err)
	}
	if err := srv.Router(mux); err != nil {
		return fmt.Errorf("设置路由失败: %w", err)
	}

	port := config.Port
	if port == 0 {
		port = 389
	}
	addr := fmt.Sprintf(":%d", port)

	s.server = srv
	go func() {
		log.Printf("[LDAP] 服务启动: ldap://0.0.0.0%s (BaseDN: %s)", addr, config.BaseDN)
		if err := srv.Run(addr); err != nil {
			log.Printf("[LDAP] 服务异常: %v", err)
		}
	}()

	// 如果启用了 LDAPS
	if config.UseTLS {
		certFile := config.TLSCertFile
		keyFile := config.TLSKeyFile
		if certFile == "" || keyFile == "" {
			certFile = "./data/certs/server.crt"
			keyFile = "./data/certs/server.key"
		}

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Printf("[LDAPS] 加载证书失败，LDAPS 未启动: %v", err)
		} else {
			tlsCfg := &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			}

			tlsMux, _ := gldap.NewMux()
			tlsMux.Bind(s.handleBind)
			tlsMux.Search(s.handleSearch)

			tlsSrv, _ := gldap.NewServer()
			tlsSrv.Router(tlsMux)

			tlsPort := config.TLSPort
			if tlsPort == 0 {
				tlsPort = 636
			}
			tlsAddr := fmt.Sprintf(":%d", tlsPort)

			s.tlsSrv = tlsSrv
			go func() {
				log.Printf("[LDAPS] 服务启动: ldaps://0.0.0.0%s", tlsAddr)
				if err := tlsSrv.Run(tlsAddr, gldap.WithTLSConfig(tlsCfg)); err != nil {
					log.Printf("[LDAPS] 服务异常: %v", err)
				}
			}()
		}
	}

	s.running = true
	return nil
}

// stopServerWithTimeout 带超时地停止 gldap 服务器
// gldap 的 Stop() 会等待所有连接关闭（connWg.Wait），如果有客户端保持连接就会永远阻塞
// listener.Close() 在 Stop() 内部最先执行，会立即释放端口
func stopServerWithTimeout(srv *gldap.Server, label string, timeout time.Duration) {
	if srv == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		srv.Stop()
		close(done)
	}()
	select {
	case <-done:
		log.Printf("[%s] 服务已正常停止", label)
	case <-time.After(timeout):
		log.Printf("[%s] 停止超时（%v），端口已释放，残留连接将自动断开", label, timeout)
	}
}

// Stop 停止 LDAP 服务器
func (s *LDAPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	// 带超时停止，防止活跃连接导致永久阻塞
	stopServerWithTimeout(s.server, "LDAP", 3*time.Second)
	s.server = nil

	stopServerWithTimeout(s.tlsSrv, "LDAPS", 3*time.Second)
	s.tlsSrv = nil

	s.running = false
	log.Printf("[LDAP] 服务已停止")
	return nil
}

// Restart 重启 LDAP 服务器
func (s *LDAPServer) Restart(config models.LDAPConfig) error {
	s.Stop()
	return s.Start(config)
}

// IsRunning 返回服务器是否正在运行
func (s *LDAPServer) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// GetConfig 获取当前配置
func (s *LDAPServer) GetConfig() models.LDAPConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// ========== Bind Handler ==========

func (s *LDAPServer) handleBind(w *gldap.ResponseWriter, r *gldap.Request) {
	resp := r.NewBindResponse(gldap.WithResponseCode(gldap.ResultInvalidCredentials))
	defer func() {
		w.Write(resp)
	}()

	msg, err := r.GetSimpleBindMessage()
	if err != nil {
		log.Printf("[LDAP] Bind 请求解析失败: %v", err)
		return
	}

	bindDN := msg.UserName
	password := string(msg.Password)

	log.Printf("[LDAP] Bind 请求: DN=%s", bindDN)

	// 允许匿名 Bind（空 DN 和空密码）
	if bindDN == "" && password == "" {
		resp.SetResultCode(gldap.ResultSuccess)
		return
	}

	// Manager 管理员 Bind
	if strings.EqualFold(bindDN, s.config.ManagerDN) {
		if password == s.config.ManagerPassword {
			resp.SetResultCode(gldap.ResultSuccess)
			log.Printf("[LDAP] Manager Bind 成功")
		} else {
			log.Printf("[LDAP] Manager Bind 失败: 密码错误")
		}
		return
	}

	// Readonly 只读账号 Bind
	if strings.EqualFold(bindDN, s.config.ReadonlyDN) {
		if password == s.config.ReadonlyPassword {
			resp.SetResultCode(gldap.ResultSuccess)
			log.Printf("[LDAP] Readonly Bind 成功")
		} else {
			log.Printf("[LDAP] Readonly Bind 失败: 密码错误")
		}
		return
	}

	// 兼容旧配置的 AdminDN（迁移过渡期）
	if s.config.AdminDN != "" && strings.EqualFold(bindDN, s.config.AdminDN) {
		if password == s.config.AdminPassword {
			resp.SetResultCode(gldap.ResultSuccess)
			log.Printf("[LDAP] Admin(旧) Bind 成功")
		} else {
			log.Printf("[LDAP] Admin(旧) Bind 失败: 密码错误")
		}
		return
	}

	// 用户 Bind: 从 DN 中提取 uid=xxx 部分
	username := extractUIDFromDN(bindDN)
	if username == "" {
		log.Printf("[LDAP] Bind 失败: 无法解析用户名 DN=%s", bindDN)
		return
	}

	var user models.User
	if err := storage.DB.Where("username = ? AND is_deleted = 0 AND status = 1", username).First(&user).Error; err != nil {
		log.Printf("[LDAP] Bind 失败: 用户不存在 username=%s", username)
		middleware.RecordLoginLog(0, username, "", "LDAP", false, "LDAP认证失败: 用户不存在")
		return
	}

	// 检查用户是否已过期
	if user.IsExpired() {
		log.Printf("[LDAP] Bind 失败: 用户已过期 username=%s expiresAt=%s", username, user.ExpiresAt.Format("2006-01-02"))
		middleware.RecordLoginLog(user.ID, username, "", "LDAP", false, "LDAP认证失败: 账户已过期")
		return
	}

	// 验证密码: 用户发送明文密码，我们检查 bcrypt(SHA256(password))
	sha256Hash := sha256Sum(password)
	localAuthOK := false
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(sha256Hash)); err != nil {
		// 也尝试直接比较（兼容旧格式）
		if err2 := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err2 != nil {
			// 本地密码验证失败，尝试密码代理认证
			if services.IsPasswordAuthProxyEnabled() {
				log.Printf("[LDAP-密码代理] 本地密码验证失败，尝试代理认证: %s", username)
				authenticated, learned, proxyErr := services.ProxyAuthenticateUser(&user, password)
				if authenticated {
					log.Printf("[LDAP-密码代理] 用户 %s 代理认证成功, 密码已学习: %v", username, learned)
					localAuthOK = true
					// 如果密码学习成功，触发同步事件
					if learned {
						syncer.DispatchSyncEvent(models.SyncEventPasswordChange, user.ID, password)
						log.Printf("[LDAP-密码同步] 已触发用户 %s 密码同步事件", username)
					}
				} else {
					log.Printf("[LDAP-密码代理] 用户 %s 代理认证失败: %v", username, proxyErr)
				}
			}
			
			if !localAuthOK {
				log.Printf("[LDAP] Bind 失败: 密码错误 username=%s", username)
				middleware.RecordLoginLog(user.ID, username, "", "LDAP", false, "LDAP认证失败: 密码错误")
				return
			}
		} else {
			localAuthOK = true
		}
	} else {
		localAuthOK = true
	}
	
	_ = localAuthOK // 避免未使用变量警告

	// 检查该用户是否有被终止的 LDAP 会话（管理员主动踢出）
	var terminatedCount int64
	storage.DB.Model(&models.Session{}).Where(
		"user_id = ? AND user_agent = 'LDAP' AND is_active = false AND expires_at > ?",
		user.ID, time.Now().Add(-10*time.Minute),
	).Count(&terminatedCount)
	// 清理旧的已终止记录（只检查最近10分钟内被终止的）

	resp.SetResultCode(gldap.ResultSuccess)
	log.Printf("[LDAP] 用户 Bind 成功: username=%s", username)
	middleware.RecordLoginLog(user.ID, username, "", "LDAP", true, "LDAP认证成功")

	// 创建 LDAP 会话记录（用于会话管理页面展示）
	sessionID := fmt.Sprintf("ldap-%d-%d", user.ID, time.Now().UnixNano())
	ldapSession := models.Session{
		ID:           sessionID,
		UserID:       user.ID,
		AccessToken:  fmt.Sprintf("ldap-bind-%s-%d", username, time.Now().UnixNano()),
		RefreshToken: "",
		IPAddress:    "",
		UserAgent:    "LDAP",
		IsActive:     true,
		LastActivity: time.Now(),
		ExpiresAt:    time.Now().Add(1 * time.Hour), // LDAP 会话1小时过期
		CreatedAt:    time.Now(),
	}
	// 先清理该用户旧的已过期 LDAP 会话
	storage.DB.Where("user_id = ? AND user_agent = 'LDAP' AND expires_at < ?", user.ID, time.Now()).
		Delete(&models.Session{})
	storage.DB.Create(&ldapSession)
}

// ========== Search Handler ==========

func (s *LDAPServer) handleSearch(w *gldap.ResponseWriter, r *gldap.Request) {
	resp := r.NewSearchDoneResponse()
	defer func() {
		w.Write(resp)
	}()

	msg, err := r.GetSearchMessage()
	if err != nil {
		log.Printf("[LDAP] Search 请求解析失败: %v", err)
		return
	}

	searchBaseDN := msg.BaseDN
	filter := msg.Filter
	scope := msg.Scope

	log.Printf("[LDAP] Search 请求: baseDN=%s scope=%d filter=%s", searchBaseDN, scope, filter)

	cfg := s.config

	// 确定操作属性中使用的管理者 DN
	ownerDN := cfg.ManagerDN
	if ownerDN == "" {
		ownerDN = cfg.AdminDN // 兼容旧配置
	}

	// Root DSE 查询: baseDN="" scope=base
	if searchBaseDN == "" && int(scope) == 0 {
		dseAttrs := map[string][]string{
			"namingContexts":     {cfg.BaseDN},
			"subschemaSubentry":  {"cn=subschema"},
			"supportedLDAPVersion": {"3"},
			"vendorName":         {"BI-Dashboard LDAP Server"},
		}
		entry := r.NewSearchResponseEntry("", gldap.WithAttributes(dseAttrs))
		w.Write(entry)
		resp.SetResultCode(gldap.ResultSuccess)
		return
	}

	// Schema 查询: baseDN="cn=subschema" 或 filter 包含 subschema
	if strings.EqualFold(searchBaseDN, "cn=subschema") {
		schemaAttrs := map[string][]string{
			"objectClass": {"top", "subSchema"},
			"cn":          {"subschema"},
			"attributeTypes": {
				// 标准属性
				"( 2.5.4.0 NAME 'objectClass' EQUALITY objectIdentifierMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.38 )",
				"( 2.5.4.3 NAME 'cn' SUP name )",
				"( 2.5.4.4 NAME 'sn' SUP name )",
				"( 2.5.4.20 NAME 'telephoneNumber' EQUALITY telephoneNumberMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.50 )",
				"( 2.5.4.35 NAME 'userPassword' EQUALITY octetStringMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.40 )",
				"( 0.9.2342.19200300.100.1.1 NAME 'uid' EQUALITY caseIgnoreMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
				"( 0.9.2342.19200300.100.1.3 NAME 'mail' EQUALITY caseIgnoreIA5Match SYNTAX 1.3.6.1.4.1.1466.115.121.1.26 )",
				"( 2.16.840.1.113730.3.1.241 NAME 'displayName' EQUALITY caseIgnoreMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 SINGLE-VALUE )",
				"( 1.3.6.1.1.1.1.0 NAME 'uidNumber' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.1.1.1.1 NAME 'gidNumber' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.1.1.1.2 NAME 'homeDirectory' EQUALITY caseExactIA5Match SYNTAX 1.3.6.1.4.1.1466.115.121.1.26 SINGLE-VALUE )",
				"( 1.3.6.1.1.1.1.4 NAME 'loginShell' EQUALITY caseExactIA5Match SYNTAX 1.3.6.1.4.1.1466.115.121.1.26 SINGLE-VALUE )",
				"( 1.3.6.1.1.1.1.12 NAME 'memberUid' EQUALITY caseExactIA5Match SYNTAX 1.3.6.1.4.1.1466.115.121.1.26 )",
				"( 2.5.4.31 NAME 'member' SUP distinguishedName )",
				"( 2.16.840.1.113730.3.1.39 NAME 'preferredLanguage' EQUALITY caseIgnoreMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.250.1.57 NAME 'labeledURI' EQUALITY caseExactMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
				// Samba 属性
				"( 1.3.6.1.4.1.7165.2.1.24 NAME 'sambaLMPassword' DESC 'LanManager Password' EQUALITY caseIgnoreIA5Match SYNTAX 1.3.6.1.4.1.1466.115.121.1.26{32} SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.25 NAME 'sambaNTPassword' DESC 'MD4 hash of the unicode password' EQUALITY caseIgnoreIA5Match SYNTAX 1.3.6.1.4.1.1466.115.121.1.26{32} SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.26 NAME 'sambaAcctFlags' DESC 'Account Flags' EQUALITY caseIgnoreIA5Match SYNTAX 1.3.6.1.4.1.1466.115.121.1.26{16} SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.27 NAME 'sambaPwdLastSet' DESC 'Timestamp of the last password update' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.28 NAME 'sambaPwdCanChange' DESC 'Timestamp of when the user is allowed to update the password' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.29 NAME 'sambaPwdMustChange' DESC 'Timestamp of when the password will expire' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.30 NAME 'sambaLogonTime' DESC 'Timestamp of last logon' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.31 NAME 'sambaLogoffTime' DESC 'Timestamp of last logoff' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.32 NAME 'sambaKickoffTime' DESC 'Timestamp of when the user will be logged off automatically' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.33 NAME 'sambaHomeDrive' DESC 'Driver letter of home directory mapping' EQUALITY caseIgnoreIA5Match SYNTAX 1.3.6.1.4.1.1466.115.121.1.26{4} SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.34 NAME 'sambaLogonScript' DESC 'Logon script path' EQUALITY caseIgnoreMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15{255} SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.35 NAME 'sambaProfilePath' DESC 'Roaming profile path' EQUALITY caseIgnoreMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15{255} SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.36 NAME 'sambaUserWorkstations' DESC 'List of user workstations the user is allowed to logon to' EQUALITY caseIgnoreMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15{255} SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.37 NAME 'sambaHomePath' DESC 'Home directory UNC path' EQUALITY caseIgnoreMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15{128} )",
				"( 1.3.6.1.4.1.7165.2.1.38 NAME 'sambaDomainName' DESC 'Windows NT domain to which the user belongs' EQUALITY caseIgnoreMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15{128} )",
				"( 1.3.6.1.4.1.7165.2.1.20 NAME 'sambaSID' DESC 'Security ID' EQUALITY caseIgnoreIA5Match SYNTAX 1.3.6.1.4.1.1466.115.121.1.26{64} SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.23 NAME 'sambaPrimaryGroupSID' DESC 'Primary Group Security ID' EQUALITY caseIgnoreIA5Match SYNTAX 1.3.6.1.4.1.1466.115.121.1.26{64} SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.51 NAME 'sambaSIDList' DESC 'Security ID List' EQUALITY caseIgnoreIA5Match SYNTAX 1.3.6.1.4.1.1466.115.121.1.26{64} )",
				"( 1.3.6.1.4.1.7165.2.1.19 NAME 'sambaGroupType' DESC 'NT Group Type' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.47 NAME 'sambaMungedDial' EQUALITY caseExactMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15{1050} )",
				"( 1.3.6.1.4.1.7165.2.1.48 NAME 'sambaBadPasswordCount' DESC 'Bad password attempt count' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.49 NAME 'sambaBadPasswordTime' DESC 'Time of the last bad password attempt' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.21 NAME 'sambaNextUserRid' DESC 'Next NT rid to give out for users' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.22 NAME 'sambaNextGroupRid' DESC 'Next NT rid to give out for groups' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.39 NAME 'sambaNextRid' DESC 'Next NT rid to give out for anything' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
				"( 1.3.6.1.4.1.7165.2.1.40 NAME 'sambaAlgorithmicRidBase' DESC 'Base at which the samba RID generation algorithm should operate' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE )",
			},
			"objectClasses": {
				// 标准 objectClasses
				"( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )",
				"( 2.5.6.6 NAME 'person' SUP top STRUCTURAL MUST ( sn $ cn ) MAY ( userPassword $ telephoneNumber $ description ) )",
				"( 2.5.6.7 NAME 'organizationalPerson' SUP person STRUCTURAL MAY ( title $ ou $ street $ postalAddress $ postalCode $ st $ l ) )",
				"( 2.16.840.1.113730.3.2.2 NAME 'inetOrgPerson' SUP organizationalPerson STRUCTURAL MAY ( mail $ uid $ displayName $ title $ givenName $ preferredLanguage $ labeledURI ) )",
				"( 1.3.6.1.1.1.2.0 NAME 'posixAccount' SUP top AUXILIARY MUST ( cn $ uid $ uidNumber $ gidNumber $ homeDirectory ) MAY ( userPassword $ loginShell $ description ) )",
				"( 1.3.6.1.1.1.2.1 NAME 'shadowAccount' SUP top AUXILIARY MUST uid )",
				"( 2.5.6.5 NAME 'organizationalUnit' SUP top STRUCTURAL MUST ou MAY description )",
				"( 2.5.6.9 NAME 'groupOfNames' SUP top STRUCTURAL MUST ( member $ cn ) MAY ( description $ ou ) )",
				"( 1.3.6.1.1.1.2.2 NAME 'posixGroup' SUP top STRUCTURAL MUST ( cn $ gidNumber ) MAY ( memberUid $ description ) )",
				"( 1.3.6.1.4.1.1466.344 NAME 'dcObject' SUP top AUXILIARY MUST dc )",
				"( 2.5.6.4 NAME 'organization' SUP top STRUCTURAL MUST o MAY description )",
				// Samba objectClasses
				"( 1.3.6.1.4.1.7165.2.2.6 NAME 'sambaSamAccount' SUP top AUXILIARY DESC 'Samba 3.0 Auxilary SAM Account' MUST ( uid $ sambaSID ) MAY ( cn $ sambaLMPassword $ sambaNTPassword $ sambaPwdLastSet $ sambaLogonTime $ sambaLogoffTime $ sambaKickoffTime $ sambaPwdCanChange $ sambaPwdMustChange $ sambaAcctFlags $ displayName $ sambaHomePath $ sambaHomeDrive $ sambaLogonScript $ sambaProfilePath $ description $ sambaUserWorkstations $ sambaPrimaryGroupSID $ sambaDomainName $ sambaMungedDial $ sambaBadPasswordCount $ sambaBadPasswordTime ) )",
				"( 1.3.6.1.4.1.7165.2.2.4 NAME 'sambaGroupMapping' SUP top AUXILIARY DESC 'Samba Group Mapping' MUST ( gidNumber $ sambaSID $ sambaGroupType ) MAY ( displayName $ description $ sambaSIDList ) )",
				"( 1.3.6.1.4.1.7165.2.2.5 NAME 'sambaDomain' SUP top STRUCTURAL DESC 'Samba Domain Information' MUST ( sambaDomainName $ sambaSID ) MAY ( sambaNextRid $ sambaNextGroupRid $ sambaNextUserRid $ sambaAlgorithmicRidBase ) )",
				"( 1.3.6.1.4.1.7165.1.2.2.7 NAME 'sambaUnixIdPool' SUP top AUXILIARY DESC 'Pool for allocating UNIX uids/gids' MUST ( uidNumber $ gidNumber ) )",
				"( 1.3.6.1.4.1.7165.1.2.2.8 NAME 'sambaIdmapEntry' SUP top AUXILIARY DESC 'Mapping from a SID to an ID' MUST ( sambaSID ) MAY ( uidNumber $ gidNumber ) )",
				"( 1.3.6.1.4.1.7165.1.2.2.9 NAME 'sambaSidEntry' SUP top STRUCTURAL DESC 'Structural Class for a SID' MUST ( sambaSID ) )",
			},
		}
		entry := r.NewSearchResponseEntry("cn=subschema", gldap.WithAttributes(schemaAttrs))
		w.Write(entry)
		resp.SetResultCode(gldap.ResultSuccess)
		return
	}

	// 构建群组层级 DN 映射
	groupDNMap := BuildGroupDNMap(cfg.BaseDN)

	// 收集所有待返回的条目
	type ldapEntry struct {
		dn    string
		attrs map[string][]string
	}
	var entries []ldapEntry

	// 1. Base DN 根条目
	dn, attrs := BuildBaseDNEntry(cfg.BaseDN, cfg.Domain, ownerDN)
	entries = append(entries, ldapEntry{dn, attrs})

	// 2. ou=roles 容器
	dn, attrs = BuildOUEntry("roles", cfg.BaseDN, ownerDN)
	entries = append(entries, ldapEntry{dn, attrs})

	// 3-4. 先加载所有用户，构建群组成员映射，再生成群组和用户条目
	// 只返回启用且未过期的用户
	var users []models.User
	storage.DB.Where("is_deleted = 0 AND status = 1 AND (expires_at IS NULL OR expires_at > ?)", time.Now()).Preload("Roles").Find(&users)

	// 构建群组 ID -> 成员用户 DN 列表的映射
	groupMemberDNs := make(map[uint][]string)
	type userEntry struct {
		dn    string
		attrs map[string][]string
	}
	var userEntries []userEntry

	for _, user := range users {
		var roleNames []string
		for _, role := range user.Roles {
			code := role.Code
			if code == "" {
				code = role.Name
			}
			roleNames = append(roleNames, code)
		}
		udn, uattrs := BuildUserEntry(user, cfg.BaseDN, ownerDN, groupDNMap, roleNames, cfg.SambaEnabled, cfg.SambaSID)
		userEntries = append(userEntries, userEntry{udn, uattrs})

		// 记录该用户属于哪个群组
		if user.GroupID > 0 {
			groupMemberDNs[user.GroupID] = append(groupMemberDNs[user.GroupID], udn)
		}
	}

	// 生成群组条目（带成员列表）
	for _, group := range groupDNMap.Groups {
		memberDNs := groupMemberDNs[group.ID]
		dn, attrs := BuildGroupEntry(group, ownerDN, groupDNMap, memberDNs)
		if dn != "" {
			entries = append(entries, ldapEntry{dn, attrs})
		}
	}

	// 生成用户条目
	for _, ue := range userEntries {
		entries = append(entries, ldapEntry{ue.dn, ue.attrs})
	}

	// 5. 所有角色条目
	var roles []models.Role
	storage.DB.Where("status = 1").Find(&roles)
	for _, role := range roles {
		memberUsernames := getRoleMembers(role.ID)
		dn, attrs := BuildRoleEntry(role, cfg.BaseDN, ownerDN, memberUsernames, cfg.SambaEnabled, cfg.SambaSID)
		entries = append(entries, ldapEntry{dn, attrs})
	}

	// 6. sambaDomain 条目（群晖 NAS 需要此条目来确认 Samba 支持）
	if cfg.SambaEnabled && cfg.SambaSID != "" {
		dn, attrs := BuildSambaDomainEntry(cfg.BaseDN, ownerDN, cfg.Domain, cfg.SambaSID)
		entries = append(entries, ldapEntry{dn, attrs})
	}

	// 根据 searchBaseDN 和 scope 过滤条目
	for _, e := range entries {
		if !dnMatchesSearch(e.dn, searchBaseDN, scope) {
			continue
		}
		if !matchesFilter(filter, e.attrs) {
			continue
		}
		entry := r.NewSearchResponseEntry(e.dn, gldap.WithAttributes(e.attrs))
		w.Write(entry)
	}

	resp.SetResultCode(gldap.ResultSuccess)
}

// ========== 辅助函数 ==========

// extractUIDFromDN 从任意 DN 中提取 uid=xxx 部分
// 例如: uid=admin,ou=行政,ou=职能,ou=根部门,dc=example,dc=com -> admin
func extractUIDFromDN(dn string) string {
	parts := splitDN(dn)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "uid=") {
			return part[4:]
		}
	}
	return ""
}

// dnMatchesSearch 根据 searchBaseDN 和 scope 判断一个条目 DN 是否应被返回
func dnMatchesSearch(entryDN, searchBaseDN string, scope gldap.Scope) bool {
	entryLower := strings.ToLower(entryDN)
	baseLower := strings.ToLower(searchBaseDN)

	switch int(scope) {
	case 0: // BaseObject - 仅返回 baseDN 本身
		return entryLower == baseLower
	case 1: // SingleLevel - 仅返回 baseDN 的直接子条目
		if entryLower == baseLower {
			return false
		}
		if !strings.HasSuffix(entryLower, ","+baseLower) {
			return false
		}
		// 去掉 baseDN 后缀，检查是否还有逗号（说明不是直接子条目）
		prefix := entryDN[:len(entryDN)-len(searchBaseDN)-1]
		return !strings.Contains(prefix, ",")
	case 2: // WholeSubtree - 返回 baseDN 本身及其所有子条目
		return entryLower == baseLower || strings.HasSuffix(entryLower, ","+baseLower)
	default:
		return strings.HasSuffix(entryLower, ","+baseLower) || entryLower == baseLower
	}
}

// sha256Sum 计算 SHA256 哈希
func sha256Sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// matchesFilter 简单 LDAP 过滤器匹配
func matchesFilter(filter string, attrs map[string][]string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" || filter == "(objectClass=*)" {
		return true
	}

	// AND 过滤器: (&(a=b)(c=d))
	if strings.HasPrefix(filter, "(&") {
		inner := filter[2 : len(filter)-1]
		parts := splitFilterParts(inner)
		for _, p := range parts {
			if !matchesFilter(p, attrs) {
				return false
			}
		}
		return true
	}

	// OR 过滤器: (|(a=b)(c=d))
	if strings.HasPrefix(filter, "(|") {
		inner := filter[2 : len(filter)-1]
		parts := splitFilterParts(inner)
		for _, p := range parts {
			if matchesFilter(p, attrs) {
				return true
			}
		}
		return false
	}

	// NOT 过滤器: (!(a=b))
	if strings.HasPrefix(filter, "(!") {
		inner := filter[2 : len(filter)-1]
		return !matchesFilter(inner, attrs)
	}

	// 简单属性过滤器: (attr=value)
	if strings.HasPrefix(filter, "(") && strings.HasSuffix(filter, ")") {
		inner := filter[1 : len(filter)-1]
		eqIdx := strings.Index(inner, "=")
		if eqIdx > 0 {
			attrName := strings.ToLower(inner[:eqIdx])
			value := inner[eqIdx+1:]

			// 存在性检查: (attr=*)
			if value == "*" {
				for k := range attrs {
					if strings.ToLower(k) == attrName {
						return true
					}
				}
				return false
			}

			// 精确匹配
			for k, vals := range attrs {
				if strings.ToLower(k) == attrName {
					for _, v := range vals {
						if strings.EqualFold(v, value) {
							return true
						}
						// 通配符匹配: (attr=*value*)
						if strings.Contains(value, "*") {
							pattern := strings.ToLower(value)
							if wildcardMatch(pattern, strings.ToLower(v)) {
								return true
							}
						}
					}
				}
			}
			return false
		}
	}

	return true
}

// wildcardMatch 简单通配符匹配
func wildcardMatch(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(s[pos:], part)
		if idx < 0 {
			return false
		}
		if i == 0 && idx != 0 {
			return false
		}
		pos += idx + len(part)
	}
	if !strings.HasSuffix(pattern, "*") && pos != len(s) {
		return false
	}
	return true
}

// splitFilterParts 拆分过滤器的子部分
func splitFilterParts(s string) []string {
	var parts []string
	depth := 0
	start := -1
	for i, ch := range s {
		if ch == '(' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 && start >= 0 {
				parts = append(parts, s[start:i+1])
				start = -1
			}
		}
	}
	return parts
}

// getRoleMembers 获取角色的所有成员用户名
func getRoleMembers(roleID uint) []string {
	var userRoles []models.UserRole
	storage.DB.Where("role_id = ?", roleID).Find(&userRoles)
	if len(userRoles) == 0 {
		return nil
	}

	var userIDs []uint
	for _, ur := range userRoles {
		userIDs = append(userIDs, ur.UserID)
	}

	var users []models.User
	storage.DB.Where("id IN ? AND is_deleted = 0 AND status = 1", userIDs).Find(&users)

	var names []string
	for _, u := range users {
		names = append(names, u.Username)
	}
	return names
}

// GetLDAPConfig 从数据库加载 LDAP 配置
func GetLDAPConfig() models.LDAPConfig {
	value, err := storage.GetConfig("ldap")
	if err != nil || value == "" {
		return models.LDAPConfig{Port: 389, TLSPort: 636, SambaEnabled: true}
	}
	var cfg models.LDAPConfig
	json.Unmarshal([]byte(value), &cfg)
	if cfg.Port == 0 {
		cfg.Port = 389
	}
	if cfg.TLSPort == 0 {
		cfg.TLSPort = 636
	}
	// 自动迁移旧配置: AdminDN -> ManagerDN
	MigrateLDAPConfig(&cfg)
	return cfg
}

// MigrateLDAPConfig 将旧的 AdminDN/AdminPassword 迁移到 ManagerDN/ManagerPassword
func MigrateLDAPConfig(cfg *models.LDAPConfig) {
	if cfg.ManagerDN == "" && cfg.AdminDN != "" {
		cfg.ManagerDN = strings.Replace(cfg.AdminDN, "cn=admin,", "cn=Manager,", 1)
		cfg.ManagerPassword = cfg.AdminPassword
	}
	if cfg.ReadonlyDN == "" && cfg.BaseDN != "" {
		cfg.ReadonlyDN = "cn=readonly," + cfg.BaseDN
	}
}
