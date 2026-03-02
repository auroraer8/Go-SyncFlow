package vpn

import (
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"go-syncflow/internal/vpn/base"
	"go-syncflow/internal/vpn/dbdata"
)

// Init 初始化 VPN 模块
func Init() error {
	// 获取工作目录
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// VPN 配置目录和文件路径
	confDir := filepath.Join(workDir, "data", "vpn", "conf")
	confPath := filepath.Join(confDir, "server.toml")

	// 检查配置文件是否存在，不存在则创建默认配置
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		log.Println("[VPN] 配置文件不存在，正在创建默认配置...")
		if err := createDefaultConfig(confDir, confPath); err != nil {
			log.Printf("[VPN] 创建默认配置失败: %v", err)
			return nil
		}
		log.Println("[VPN] 默认配置已创建:", confPath)
	}

	// 初始化 base 配置
	if err := base.InitEmbedded(confPath); err != nil {
		return err
	}

	// 复用 Go-SyncFlow 系统 SSL 证书
	reuseSystemCert(workDir)

	// 初始化数据库
	dbdata.Start()

	// 设置默认用户组
	if base.Cfg.DefaultGroup == "" {
		groups := dbdata.GetGroupNamesIds()
		if len(groups) > 0 {
			base.Cfg.DefaultGroup = groups[0].Name
		} else {
			base.Cfg.DefaultGroup = "all"
		}
	}

	log.Println("[VPN] 模块初始化完成")

	// 检查是否配置了开机自动启动
	if shouldAutoStart() {
		go func() {
			log.Println("[VPN] 正在自动启动 VPN 服务...")
			if err := Start(); err != nil {
				log.Printf("[VPN] 自动启动失败: %v", err)
			} else {
				log.Println("[VPN] VPN 服务已自动启动")
			}
		}()
	}

	return nil
}

// shouldAutoStart 检查是否应该自动启动 VPN 服务
func shouldAutoStart() bool {
	// 从数据库读取自动启动配置
	config, err := dbdata.SettingGetAutoStart()
	if err != nil {
		// 默认自动启动
		return true
	}
	return config.Enabled
}

// reuseSystemCert 复用 Go-SyncFlow 系统已上传的 SSL 证书
func reuseSystemCert(workDir string) {
	systemCertFile := filepath.Join(workDir, "data", "certs", "server.crt")
	systemKeyFile := filepath.Join(workDir, "data", "certs", "server.key")

	// 检查系统证书是否存在
	_, certErr := os.Stat(systemCertFile)
	_, keyErr := os.Stat(systemKeyFile)

	if certErr == nil && keyErr == nil {
		// 系统证书存在，使用系统证书
		base.Cfg.CertFile = systemCertFile
		base.Cfg.CertKey = systemKeyFile
		log.Printf("[VPN] 使用系统 SSL 证书: %s", systemCertFile)
	} else {
		log.Printf("[VPN] 系统证书不存在，使用 VPN 自签名证书: %s", base.Cfg.CertFile)
	}
}

// ReloadSystemCert 重新加载系统 SSL 证书（供外部调用）
// 当用户通过界面上传新的 SSL 证书后，调用此函数使 VPN 模块使用新证书
func ReloadSystemCert() error {
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}

	systemCertFile := filepath.Join(workDir, "data", "certs", "server.crt")
	systemKeyFile := filepath.Join(workDir, "data", "certs", "server.key")

	// 检查系统证书是否存在
	if _, err := os.Stat(systemCertFile); err != nil {
		return err
	}
	if _, err := os.Stat(systemKeyFile); err != nil {
		return err
	}

	// 更新 VPN 配置中的证书路径
	base.Cfg.CertFile = systemCertFile
	base.Cfg.CertKey = systemKeyFile

	// 重新加载证书到内存
	if err := dbdata.ReloadCertificate(); err != nil {
		log.Printf("[VPN] 重新加载证书失败: %v", err)
		return err
	}

	log.Printf("[VPN] SSL 证书已更新: %s", systemCertFile)
	return nil
}

// detectDefaultInterface 自动检测系统默认网卡
func detectDefaultInterface() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "eth0"
	}

	// 优先查找有 IPv4 地址且启用的非回环网卡
	for _, iface := range interfaces {
		// 跳过未启用、回环、虚拟网卡
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		// 跳过常见虚拟网卡
		name := strings.ToLower(iface.Name)
		if strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "br-") ||
			strings.HasPrefix(name, "veth") || strings.HasPrefix(name, "virbr") ||
			strings.HasPrefix(name, "lo") || strings.HasPrefix(name, "tun") ||
			strings.HasPrefix(name, "tap") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				// 找到了有效的 IPv4 地址
				log.Printf("[VPN] 自动检测到默认网卡: %s (%s)", iface.Name, ipnet.IP.String())
				return iface.Name
			}
		}
	}

	return "eth0"
}

// createDefaultConfig 创建默认 VPN 配置文件
func createDefaultConfig(confDir, confPath string) error {
	// 创建目录
	if err := os.MkdirAll(confDir, 0755); err != nil {
		return err
	}

	// 创建 files 子目录
	filesDir := filepath.Join(confDir, "files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return err
	}

	// 根据主应用数据库配置生成 VPN 数据库配置
	dbType, dbSource := getVPNDatabaseConfig()

	// 自动检测默认网卡
	defaultInterface := detectDefaultInterface()

	// 默认配置内容
	defaultConfig := `# Go-SyncFlow VPN 配置（自动生成）

# 数据文件
db_type = "` + dbType + `"
db_source = "` + dbSource + `"
profile = "./data/vpn/conf/profile.xml"

# 证书文件（首次启动会自动生成自签名证书）
cert_file = "./data/vpn/conf/vpn_cert.pem"
cert_key = "./data/vpn/conf/vpn_cert.key"
files_path = "./data/vpn/conf/files"

# 日志
log_level = "info"

# 系统名称
issuer = "Go-SyncFlow VPN"

# 后台管理
admin_user = "admin"
admin_pass = "$2a$10$UQ7C.EoPifDeJh6d8.31TeSPQU7hM/NOM2nixmBucJpAuXDQNqNke"
jwt_secret = "go-syncflow-vpn-jwt-secret"

# VPN 服务监听地址
server_addr = ":443"
server_dtls = true
server_dtls_addr = ":443"
admin_addr = ":8800"

# 客户端限制
max_client = 200
max_user_client = 3

# 虚拟网络配置
link_mode = "tun"
ipv4_master = "` + defaultInterface + `"
ipv4_cidr = "192.168.90.0/24"
ipv4_gateway = "192.168.90.1"
ipv4_start = "192.168.90.100"
ipv4_end = "192.168.90.200"

# NAT
iptables_nat = true

# 调试
display_error = false
`

	if err := os.WriteFile(confPath, []byte(defaultConfig), 0644); err != nil {
		return err
	}

	// 创建默认 profile.xml
	profilePath := filepath.Join(confDir, "profile.xml")
	defaultProfile := `<?xml version="1.0" encoding="UTF-8"?>
<AnyConnectProfile xmlns="http://schemas.xmlsoap.org/encoding/">
  <ServerList>
    <HostEntry>
      <HostName>Go-SyncFlow VPN</HostName>
      <HostAddress>vpn.example.com</HostAddress>
    </HostEntry>
  </ServerList>
</AnyConnectProfile>
`
	if err := os.WriteFile(profilePath, []byte(defaultProfile), 0644); err != nil {
		return err
	}

	return nil
}

// getVPNDatabaseConfig 返回 VPN PostgreSQL 数据库配置
func getVPNDatabaseConfig() (dbType, dbSource string) {
	host := getEnvOrDefault("DB_HOST", "127.0.0.1")
	port := getEnvOrDefault("DB_PORT", "5432")
	user := getEnvOrDefault("DB_USER", "syncflow")
	password := getEnvOrDefault("DB_PASSWORD", "syncflow")
	database := getEnvOrDefault("VPN_DB_DATABASE", "go_syncflow_vpn")
	sslmode := getEnvOrDefault("DB_SSLMODE", "disable")
	return "postgres", "host=" + host + " port=" + port + " user=" + user + " password=" + password + " dbname=" + database + " sslmode=" + sslmode
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
