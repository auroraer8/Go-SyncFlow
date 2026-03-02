package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	ldapv3 "github.com/go-ldap/ldap/v3"

	"go-syncflow/internal/ldapserver"
	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

// 全局 LDAP 服务器实例
var ldapSrv *ldapserver.LDAPServer

// SetLDAPServer 设置全局 LDAP 服务器实例
func SetLDAPServer(srv *ldapserver.LDAPServer) {
	ldapSrv = srv
}

// GetLDAPServer 获取全局 LDAP 服务器实例
func GetLDAPServer() *ldapserver.LDAPServer {
	return ldapSrv
}

// GetLDAPConfig 获取 LDAP 配置
func GetLDAPConfig(c *gin.Context) {
	value, _ := storage.GetConfig("ldap")
	var cfg models.LDAPConfig
	if value != "" {
		json.Unmarshal([]byte(value), &cfg)
	} else {
		// 新部署默认配置：Samba 默认启用（兼容群晖NAS）
		cfg.Port = 389
		cfg.TLSPort = 636
		cfg.SambaEnabled = true
	}
	// 自动迁移旧配置
	ldapserver.MigrateLDAPConfig(&cfg)
	// 不返回密码
	cfg.ManagerPassword = ""
	cfg.ReadonlyPassword = ""
	cfg.AdminPassword = ""
	respondOK(c, cfg)
}

// UpdateLDAPConfig 更新 LDAP 配置并重启服务
func UpdateLDAPConfig(c *gin.Context) {
	var req models.LDAPConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 加载旧配置用于密码保留
	oldValue, _ := storage.GetConfig("ldap")
	var oldCfg models.LDAPConfig
	if oldValue != "" {
		json.Unmarshal([]byte(oldValue), &oldCfg)
		ldapserver.MigrateLDAPConfig(&oldCfg)
	}

	// 如果未传密码，保留原来的
	if req.ManagerPassword == "" {
		req.ManagerPassword = oldCfg.ManagerPassword
	}
	if req.ReadonlyPassword == "" {
		req.ReadonlyPassword = oldCfg.ReadonlyPassword
	}

	// 自动从域名推导 BaseDN
	if req.Domain != "" && req.BaseDN == "" {
		req.BaseDN = domainToBaseDN(req.Domain)
	}

	// 自动生成 ManagerDN 和 ReadonlyDN
	if req.ManagerDN == "" && req.BaseDN != "" {
		req.ManagerDN = "cn=Manager," + req.BaseDN
	}
	if req.ReadonlyDN == "" && req.BaseDN != "" {
		req.ReadonlyDN = "cn=readonly," + req.BaseDN
	}

	// 清除旧字段（已迁移到新字段）
	req.AdminDN = ""
	req.AdminPassword = ""

	// 自动生成域 SID
	if req.SambaEnabled && req.SambaSID == "" && req.Domain != "" {
		req.SambaSID = ldapserver.GenerateDomainSID(req.Domain)
	}

	// 默认端口
	if req.Port == 0 {
		req.Port = 389
	}
	if req.TLSPort == 0 {
		req.TLSPort = 636
	}

	data, _ := json.Marshal(req)
	if err := storage.SetConfig("ldap", string(data)); err != nil {
		respondError(c, http.StatusInternalServerError, "保存失败")
		return
	}

	// 根据开关状态管理 LDAP 服务
	if ldapSrv != nil {
		if req.Enabled {
			if err := ldapSrv.Restart(req); err != nil {
				respondError(c, http.StatusInternalServerError, "LDAP 服务启动失败: "+err.Error())
				return
			}
		} else {
			ldapSrv.Stop()
		}
	}

	middleware.RecordOperationLog(c, "系统设置", "更新LDAP配置", "", "")
	// 不返回密码
	req.ManagerPassword = ""
	req.ReadonlyPassword = ""
	respondOK(c, req)
}

// TestLDAPService 测试 LDAP 服务是否正常运行（分别测试两个账号）
func TestLDAPService(c *gin.Context) {
	if ldapSrv == nil || !ldapSrv.IsRunning() {
		respondError(c, http.StatusBadRequest, "LDAP 服务未启动")
		return
	}

	cfg := ldapSrv.GetConfig()
	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)

	results := make(map[string]string)

	// 测试 Manager 账号
	managerOK := testLDAPBind(addr, cfg.ManagerDN, cfg.ManagerPassword)
	if managerOK {
		results["manager"] = "连接成功"
	} else {
		results["manager"] = "连接失败"
	}

	// 测试 Readonly 账号
	if cfg.ReadonlyDN != "" && cfg.ReadonlyPassword != "" {
		readonlyOK := testLDAPBind(addr, cfg.ReadonlyDN, cfg.ReadonlyPassword)
		if readonlyOK {
			results["readonly"] = "连接成功"
		} else {
			results["readonly"] = "连接失败"
		}
	} else {
		results["readonly"] = "未配置密码"
	}

	// 测试 Search（使用 Manager 账号）
	entryCount := 0
	if managerOK {
		l, err := ldapv3.Dial("tcp", addr)
		if err == nil {
			defer l.Close()
			if l.Bind(cfg.ManagerDN, cfg.ManagerPassword) == nil {
				searchReq := ldapv3.NewSearchRequest(
					cfg.BaseDN,
					ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 5, 0, false,
					"(objectClass=*)", []string{"dn"}, nil,
				)
				sr, err := l.Search(searchReq)
				if err == nil {
					entryCount = len(sr.Entries)
				}
			}
		}
	}

	respondOK(c, gin.H{
		"message":    "LDAP 服务测试完成",
		"entries":    entryCount,
		"address":    addr,
		"baseDN":     cfg.BaseDN,
		"managerDN":  cfg.ManagerDN,
		"readonlyDN": cfg.ReadonlyDN,
		"results":    results,
	})
}

// testLDAPBind 测试 LDAP Bind 是否成功
func testLDAPBind(addr, dn, password string) bool {
	l, err := ldapv3.Dial("tcp", addr)
	if err != nil {
		return false
	}
	defer l.Close()
	return l.Bind(dn, password) == nil
}

// GetLDAPStatus 获取 LDAP 服务状态
func GetLDAPStatus(c *gin.Context) {
	running := false
	if ldapSrv != nil {
		running = ldapSrv.IsRunning()
	}

	cfg := ldapserver.GetLDAPConfig()

	respondOK(c, gin.H{
		"running":      running,
		"enabled":      cfg.Enabled,
		"port":         cfg.Port,
		"tlsEnabled":   cfg.UseTLS,
		"tlsPort":      cfg.TLSPort,
		"baseDN":       cfg.BaseDN,
		"managerDN":    cfg.ManagerDN,
		"readonlyDN":   cfg.ReadonlyDN,
		"sambaEnabled": cfg.SambaEnabled,
	})
}

// domainToBaseDN 将域名转换为 Base DN
// 例如: example.com -> dc=example,dc=com
func domainToBaseDN(domain string) string {
	parts := strings.Split(domain, ".")
	var dcParts []string
	for _, p := range parts {
		dcParts = append(dcParts, "dc="+p)
	}
	return strings.Join(dcParts, ",")
}
