package handlers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/dingtalk"
	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/services"
	"go-syncflow/internal/storage"
	"go-syncflow/internal/vpn"
)

func GetUIConfig(c *gin.Context) {
	value, err := storage.GetConfig("ui")
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取配置失败")
		return
	}

	var cfg models.UIConfig
	json.Unmarshal([]byte(value), &cfg)
	respondOK(c, cfg)
}

func UpdateUIConfig(c *gin.Context) {
	var cfg models.UIConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	data, _ := json.Marshal(cfg)
	if err := storage.SetConfig("ui", string(data)); err != nil {
		respondError(c, http.StatusInternalServerError, "保存失败")
		return
	}

	middleware.RecordOperationLog(c, "系统设置", "更新界面配置", "", "")
	respondOK(c, cfg)
}

// ========== 钉钉配置 ==========

// GetDingTalkConfigFull 获取钉钉完整配置（包含密钥，仅管理员）
func GetDingTalkConfigFull(c *gin.Context) {
	value, _ := storage.GetConfig("dingtalk")

	var cfg models.DingTalkConfig
	if value != "" {
		json.Unmarshal([]byte(value), &cfg)
	}
	respondOK(c, cfg)
}

// UpdateDingTalkConfig 更新钉钉配置
func UpdateDingTalkConfig(c *gin.Context) {
	var cfg models.DingTalkConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 如果没传AppSecret，保留原来的
	if cfg.AppSecret == "" {
		oldValue, _ := storage.GetConfig("dingtalk")
		var oldCfg models.DingTalkConfig
		json.Unmarshal([]byte(oldValue), &oldCfg)
		cfg.AppSecret = oldCfg.AppSecret
	}

	data, _ := json.Marshal(cfg)
	if err := storage.SetConfig("dingtalk", string(data)); err != nil {
		respondError(c, http.StatusInternalServerError, "保存失败")
		return
	}

	// 重置token
	dingtalk.GetClient().ResetToken()

	middleware.RecordOperationLog(c, "系统设置", "更新钉钉配置", "", "")
	// 返回时不包含敏感信息
	cfg.AppSecret = ""
	respondOK(c, cfg)
}

// TestDingTalkConnection 测试钉钉连接
func TestDingTalkConnection(c *gin.Context) {
	if err := dingtalk.GetClient().TestConnection(); err != nil {
		respondError(c, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(c, gin.H{"message": "连接成功"})
}

// GetDingTalkStatus 获取钉钉免登状态（公开接口）
func GetDingTalkStatus(c *gin.Context) {
	client := dingtalk.GetClient()
	enabled := client.IsEnabled()

	var corpId, agentId string
	if enabled {
		if cfg, err := client.GetConfig(); err == nil {
			corpId = cfg.CorpID
			agentId = cfg.AgentID
		}
	}

	respondOK(c, gin.H{
		"enabled": enabled,
		"corpId":  corpId,
		"agentId": agentId,
	})
}

// ========== HTTPS配置 ==========

const certDir = "./data/certs"

func GetHTTPSConfig(c *gin.Context) {
	value, _ := storage.GetConfig("https")

	var cfg models.HTTPSConfig
	if value != "" {
		json.Unmarshal([]byte(value), &cfg)
	} else {
		cfg.Port = "8443"
		cfg.Enabled = false
	}

	certExists := false
	keyExists := false
	if cfg.CertFile != "" {
		if _, err := os.Stat(cfg.CertFile); err == nil {
			certExists = true
		}
	}
	if cfg.KeyFile != "" {
		if _, err := os.Stat(cfg.KeyFile); err == nil {
			keyExists = true
		}
	}

	respondOK(c, gin.H{
		"enabled":     cfg.Enabled,
		"port":        cfg.Port,
		"domain":      cfg.Domain,
		"certExpiry":  cfg.CertExpiry,
		"certSubject": cfg.CertSubject,
		"certExists":  certExists,
		"keyExists":   keyExists,
	})
}

func UpdateHTTPSConfig(c *gin.Context) {
	var req struct {
		Enabled bool   `json:"enabled"`
		Port    string `json:"port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	value, _ := storage.GetConfig("https")
	var cfg models.HTTPSConfig
	if value != "" {
		json.Unmarshal([]byte(value), &cfg)
	}

	cfg.Enabled = req.Enabled
	if req.Port != "" {
		cfg.Port = req.Port
	} else if cfg.Port == "" {
		cfg.Port = "8443"
	}

	if cfg.Enabled && (cfg.CertFile == "" || cfg.KeyFile == "") {
		respondError(c, http.StatusBadRequest, "请先上传SSL证书")
		return
	}

	data, _ := json.Marshal(cfg)
	if err := storage.SetConfig("https", string(data)); err != nil {
		respondError(c, http.StatusInternalServerError, "保存失败")
		return
	}

	middleware.RecordOperationLog(c, "系统设置", "更新HTTPS配置", "", "")

	// 自动重启 HTTPS 服务
	if httpsRestarter != nil {
		go func() {
			if err := httpsRestarter.Restart(); err != nil {
				log.Printf("[HTTPS] 重启失败: %v", err)
			}
		}()
	}

	respondOK(c, gin.H{"message": "HTTPS配置已保存，服务正在重启", "config": cfg})
}

// HTTPSRestarter HTTPS 服务热重启接口
type HTTPSRestarter interface {
	Restart() error
}

var httpsRestarter HTTPSRestarter

// SetHTTPSRestarter 注入 HTTPS 重启器（由 main.go 调用）
func SetHTTPSRestarter(r HTTPSRestarter) {
	httpsRestarter = r
}

func UploadSSLCert(c *gin.Context) {
	certFile, err := c.FormFile("cert")
	if err != nil {
		respondError(c, http.StatusBadRequest, "请上传证书文件")
		return
	}

	keyFile, err := c.FormFile("key")
	if err != nil {
		respondError(c, http.StatusBadRequest, "请上传私钥文件")
		return
	}

	if err := os.MkdirAll(certDir, 0755); err != nil {
		respondError(c, http.StatusInternalServerError, "创建证书目录失败")
		return
	}

	certPath := filepath.Join(certDir, "server.crt")
	if err := c.SaveUploadedFile(certFile, certPath); err != nil {
		respondError(c, http.StatusInternalServerError, "保存证书文件失败")
		return
	}

	keyPath := filepath.Join(certDir, "server.key")
	if err := c.SaveUploadedFile(keyFile, keyPath); err != nil {
		respondError(c, http.StatusInternalServerError, "保存私钥文件失败")
		return
	}

	ci, err := parseCertificate(certPath)
	if err != nil {
		os.Remove(certPath)
		os.Remove(keyPath)
		respondError(c, http.StatusBadRequest, "证书无效: "+err.Error())
		return
	}

	value, _ := storage.GetConfig("https")
	var cfg models.HTTPSConfig
	if value != "" {
		json.Unmarshal([]byte(value), &cfg)
	}

	cfg.CertFile = certPath
	cfg.KeyFile = keyPath
	cfg.Domain = ci.Domain
	cfg.CertExpiry = ci.Expiry
	cfg.CertSubject = ci.Subject
	if cfg.Port == "" {
		cfg.Port = "8443"
	}

	data, _ := json.Marshal(cfg)
	if err := storage.SetConfig("https", string(data)); err != nil {
		respondError(c, http.StatusInternalServerError, "保存配置失败")
		return
	}

	middleware.RecordOperationLog(c, "系统设置", "上传SSL证书", "", "")

	// 通知 VPN 模块更新证书
	if err := vpn.ReloadSystemCert(); err != nil {
		log.Printf("[SSL] VPN 证书更新失败: %v", err)
	} else {
		log.Printf("[SSL] VPN 已同步使用新证书")
	}

	respondOK(c, gin.H{
		"domain":     ci.Domain,
		"expiry":     ci.Expiry,
		"subject":    ci.Subject,
		"certExists": true,
		"keyExists":  true,
	})
}

func DeleteSSLCert(c *gin.Context) {
	value, _ := storage.GetConfig("https")
	var cfg models.HTTPSConfig
	if value != "" {
		json.Unmarshal([]byte(value), &cfg)
	}

	if cfg.CertFile != "" {
		os.Remove(cfg.CertFile)
	}
	if cfg.KeyFile != "" {
		os.Remove(cfg.KeyFile)
	}

	cfg.Enabled = false
	cfg.CertFile = ""
	cfg.KeyFile = ""
	cfg.Domain = ""
	cfg.CertExpiry = ""
	cfg.CertSubject = ""

	data, _ := json.Marshal(cfg)
	storage.SetConfig("https", string(data))

	middleware.RecordOperationLog(c, "系统设置", "删除SSL证书", "", "")
	respondOK(c, nil)
}

type certInfo struct {
	Domain  string
	Expiry  string
	Subject string
}

func parseCertificate(certPath string) (*certInfo, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, io.EOF
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	domain := ""
	if len(cert.DNSNames) > 0 {
		domain = cert.DNSNames[0]
	} else if cert.Subject.CommonName != "" {
		domain = cert.Subject.CommonName
	}

	return &certInfo{
		Domain:  domain,
		Expiry:  cert.NotAfter.Format(time.RFC3339),
		Subject: cert.Subject.String(),
	}, nil
}

// GetHTTPSConfigForServer 获取HTTPS配置（供服务器启动使用）
func GetHTTPSConfigForServer() *models.HTTPSConfig {
	value, _ := storage.GetConfig("https")
	if value == "" {
		return nil
	}

	var cfg models.HTTPSConfig
	json.Unmarshal([]byte(value), &cfg)

	if !cfg.Enabled || cfg.CertFile == "" || cfg.KeyFile == "" {
		return nil
	}

	if _, err := os.Stat(cfg.CertFile); err != nil {
		return nil
	}
	if _, err := os.Stat(cfg.KeyFile); err != nil {
		return nil
	}

	return &cfg
}

// EnsureSelfSignedCert 首次启动时自动生成自签证书并启用 HTTPS。
// 仅在证书文件不存在时生成，不会覆盖用户手动上传的证书。
func EnsureSelfSignedCert() {
	certPath := filepath.Join(certDir, "server.crt")
	keyPath := filepath.Join(certDir, "server.key")

	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return // 证书已存在
		}
	}

	log.Println("[HTTPS] 首次启动，自动生成自签证书...")

	if err := os.MkdirAll(certDir, 0755); err != nil {
		log.Printf("[HTTPS] 创建证书目录失败: %v", err)
		return
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Printf("[HTTPS] 生成密钥失败: %v", err)
		return
	}

	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	now := time.Now()

	// 收集本机所有 IP 作为 SAN
	ips := []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				ips = append(ips, ipNet.IP)
			}
		}
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: "Go-SyncFlow", Organization: []string{"Go-SyncFlow"}},
		NotBefore:    now,
		NotAfter:     now.AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  ips,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		log.Printf("[HTTPS] 生成证书失败: %v", err)
		return
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		log.Printf("[HTTPS] 写入证书文件失败: %v", err)
		return
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certOut.Close()

	keyBytes, _ := x509.MarshalECPrivateKey(priv)
	keyOut, err := os.Create(keyPath)
	if err != nil {
		log.Printf("[HTTPS] 写入密钥文件失败: %v", err)
		os.Remove(certPath)
		return
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	keyOut.Close()
	os.Chmod(keyPath, 0600)

	// 解析证书信息并写入数据库配置
	ci, _ := parseCertificate(certPath)
	cfg := models.HTTPSConfig{
		Enabled:  true,
		Port:     "8443",
		CertFile: certPath,
		KeyFile:  keyPath,
	}
	if ci != nil {
		cfg.Domain = ci.Domain
		cfg.CertExpiry = ci.Expiry
		cfg.CertSubject = ci.Subject
	}
	data, _ := json.Marshal(cfg)
	storage.SetConfig("https", string(data))

	log.Printf("[HTTPS] 自签证书已生成，HTTPS 将在 :8443 启动 (SAN IPs: %v)", ips)
}

// DownloadDoc 下载/预览系统文档（PDF）
func DownloadDoc(c *gin.Context) {
	name := c.Param("name")
	mode := c.DefaultQuery("mode", "download")

	docMap := map[string]string{
		"manual": "系统使用手册.pdf",
		"api":    "API接口文档.pdf",
	}

	filename, ok := docMap[name]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文档不存在"})
		return
	}

	docPaths := []string{
		filepath.Join("/opt/Go-SyncFlow/docs", filename),
		filepath.Join("..", "docs", filename),
		filepath.Join("docs", filename),
	}

	for _, p := range docPaths {
		if _, err := os.Stat(p); err == nil {
			escaped := url.PathEscape(filename)
			disposition := "attachment"
			if mode == "preview" {
				disposition = "inline"
			}
			c.Header("Content-Disposition", fmt.Sprintf(`%s; filename="document.pdf"; filename*=UTF-8''%s`, disposition, escaped))
			c.Header("Content-Type", "application/pdf")
			c.File(p)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文档文件未找到"})
}

// ListDocs 获取可用文档列表
func ListDocs(c *gin.Context) {
	docs := []gin.H{
		{
			"id":          "manual",
			"name":        "系统使用手册",
			"description": "各功能模块操作指南、配置说明与常见问题解答",
			"filename":    "系统使用手册.pdf",
			"icon":        "Document",
		},
		{
			"id":          "api",
			"name":        "API接口文档",
			"description": "REST API 接口规范、认证方式与调用示例",
			"filename":    "API接口文档.pdf",
			"icon":        "Connection",
		},
	}

	respondOK(c, docs)
}

// ========== 密码代理认证配置 ==========

// GetPasswordAuthProxyConfig 获取密码代理认证配置
func GetPasswordAuthProxyConfig(c *gin.Context) {
	cfg := services.GetPasswordAuthProxyConfig()

	// 获取所有可用于认证的上游连接器（支持 LDAP、数据库、RADIUS、HTTP API）
	authTypes := []string{
		"ldap_ad", "ldap_openldap",
		"db_mysql", "db_postgresql", "db_oracle", "db_sqlserver",
		"radius", "http_api",
	}

	var connectors []models.Connector
	storage.DB.Where("status = ? AND direction IN ? AND type IN ?",
		1, []string{"upstream", "both"}, authTypes).Find(&connectors)

	connectorList := []gin.H{}
	for _, conn := range connectors {
		connectorList = append(connectorList, gin.H{
			"id":       conn.ID,
			"name":     conn.Name,
			"type":     conn.Type,
			"typeName": conn.ConnectorTypeName(),
			"category": getConnectorCategory(conn.Type),
		})
	}

	// 获取当前连接器名称
	var connectorName string
	var connectorType string
	if cfg.ConnectorID > 0 {
		var conn models.Connector
		if storage.DB.First(&conn, cfg.ConnectorID).Error == nil {
			connectorName = conn.Name
			connectorType = conn.Type
		}
	}

	respondOK(c, gin.H{
		"config":        cfg,
		"connectors":    connectorList,
		"connectorName": connectorName,
		"connectorType": connectorType,
	})
}

// getConnectorCategory 根据连接器类型返回分类
func getConnectorCategory(connType string) string {
	switch connType {
	case "ldap_ad", "ldap_openldap", "ldap_generic":
		return "ldap"
	case "db_mysql", "db_postgresql", "db_oracle", "db_sqlserver":
		return "database"
	case "radius":
		return "radius"
	case "http_api":
		return "http"
	default:
		return "other"
	}
}

// UpdatePasswordAuthProxyConfig 更新密码代理认证配置
func UpdatePasswordAuthProxyConfig(c *gin.Context) {
	var cfg services.PasswordAuthProxyConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 验证连接器
	if cfg.Enabled && cfg.ConnectorID > 0 {
		var conn models.Connector
		if storage.DB.First(&conn, cfg.ConnectorID).Error != nil {
			respondError(c, http.StatusBadRequest, "选择的连接器不存在")
			return
		}
		if !conn.CanAuthenticate() {
			respondError(c, http.StatusBadRequest, "选择的连接器不支持认证功能")
			return
		}
	}

	if err := services.SavePasswordAuthProxyConfig(&cfg); err != nil {
		respondError(c, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}

	middleware.RecordOperationLog(c, "系统设置", "更新密码代理认证配置", "", "")
	respondOK(c, gin.H{"message": "保存成功"})
}

// TestPasswordAuthProxy 测试密码代理认证
func TestPasswordAuthProxy(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 查找用户
	var user models.User
	if err := storage.DB.Where("username = ? AND is_deleted = 0", req.Username).First(&user).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}

	// 尝试代理认证
	authenticated, learned, err := services.ProxyAuthenticateUser(&user, req.Password)
	if err != nil {
		respondOK(c, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if authenticated {
		msg := "认证成功"
		if learned {
			msg = "认证成功，密码已学习"
		}
		respondOK(c, gin.H{
			"success":  true,
			"message":  msg,
			"learned":  learned,
			"username": user.Username,
		})
	} else {
		respondOK(c, gin.H{
			"success": false,
			"message": "认证失败",
		})
	}
}

// ========== 密码存储策略 ==========

// GetPasswordStoragePolicy 获取系统密码存储策略
func GetPasswordStoragePolicy(c *gin.Context) {
	policy := services.GetSystemPasswordPolicy()
	label := services.GetPasswordFormatLabel(policy)
	
	respondOK(c, gin.H{
		"policy":   policy,
		"label":    label,
		"formats":  services.AllPasswordFormats,
	})
}

// UpdatePasswordStoragePolicy 更新系统密码存储策略
func UpdatePasswordStoragePolicy(c *gin.Context) {
	var req struct {
		Policy string `json:"policy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	if err := services.SetSystemPasswordPolicy(req.Policy); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	middleware.RecordOperationLog(c, "系统设置", "更新密码存储策略", req.Policy, "")
	respondOK(c, gin.H{
		"message": "保存成功",
		"policy":  req.Policy,
		"label":   services.GetPasswordFormatLabel(req.Policy),
	})
}
