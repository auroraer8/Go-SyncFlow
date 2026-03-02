package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"go-syncflow/internal/handlers"
	"go-syncflow/internal/ldapserver"
	"go-syncflow/internal/middleware"
	"go-syncflow/internal/services"
	"go-syncflow/internal/storage"
	"go-syncflow/internal/vpn"
)

// DatabaseYAML 数据库配置文件结构（仅 PostgreSQL）
type DatabaseYAML struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"sslmode"`
	MaxIdle  int    `yaml:"max_idle"`
	MaxOpen  int    `yaml:"max_open"`
}

func main() {
	// 设置日志输出到文件和标准输出
	logFile, err := os.OpenFile("/tmp/go-syncflow.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(logFile)
	}
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 设置默认时区为东八区
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		log.Printf("加载时区失败，使用固定偏移: %v", err)
		loc = time.FixedZone("CST", 8*3600)
	}
	time.Local = loc

	// 加载数据库配置
	dbCfg := loadDatabaseConfig()
	if err := storage.InitDBWithConfig(dbCfg); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 初始化 IP 白名单
	middleware.InitIPWhitelist(storage.DB)

	// 初始化 RSA 密钥对（密码传输加密）
	services.InitRSAKeyPair()

	// 初始化默认通知通道和模板
	services.InitDefaultChannels()
	services.InitDefaultTemplates()

	// 初始化 VPN 模块
	if err := vpn.Init(); err != nil {
		log.Printf("VPN 模块初始化失败: %v", err)
	}

	router := gin.Default()
	handlers.RegisterRoutes(router)

	// 启动钉钉定时同步（旧方式，保持兼容）
	handlers.StartSyncScheduler()

	// 启动上下游定时同步调度器
	handlers.StartUpstreamSchedulers()
	handlers.StartDownstreamSchedulers()

	// 启动日志清理调度器
	handlers.StartLogCleanupScheduler()

	// 启动用户过期检查调度器
	services.StartUserExpiryScheduler()

	// 启动 LDAP 服务器
	ldapSrv := ldapserver.NewLDAPServer()
	handlers.SetLDAPServer(ldapSrv)
	ldapCfg := ldapserver.GetLDAPConfig()
	if ldapCfg.Enabled {
		go func() {
			if err := ldapSrv.Start(ldapCfg); err != nil {
				log.Printf("LDAP 服务启动失败: %v", err)
			}
		}()
	}

	// HTTP服务
	httpAddr := getEnv("LISTEN_ADDR", ":8080")

	// HTTPS 热重启管理器
	httpsManager := &HTTPSManager{router: router}
	handlers.SetHTTPSRestarter(httpsManager)

	// 首次运行自动生成自签证书并启用 HTTPS
	handlers.EnsureSelfSignedCert()

	// 启动时尝试启动 HTTPS
	httpsManager.Start()

	log.Printf("HTTP服务启动: http://localhost%s", httpAddr)
	if err := router.Run(httpAddr); err != nil {
		log.Fatalf("HTTP服务启动失败: %v", err)
	}
}

// HTTPSManager 管理 HTTPS 服务的热重启
type HTTPSManager struct {
	router *gin.Engine
	server *http.Server
}

func (m *HTTPSManager) Start() {
	cfg := handlers.GetHTTPSConfigForServer()
	if cfg == nil {
		log.Println("[HTTPS] 未配置或未启用，跳过")
		return
	}
	addr := ":" + cfg.Port
	tlsCert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		log.Printf("[HTTPS] 加载证书失败: %v", err)
		return
	}
	m.server = &http.Server{
		Addr:    addr,
		Handler: m.router,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		},
	}
	go func() {
		log.Printf("[HTTPS] 服务启动: https://0.0.0.0%s", addr)
		if err := m.server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTPS] 服务异常: %v", err)
		}
	}()
}

func (m *HTTPSManager) Restart() error {
	// 先关闭旧服务
	if m.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		m.server.Shutdown(ctx)
		m.server = nil
		log.Println("[HTTPS] 旧服务已关闭")
	}
	// 启动新服务
	m.Start()
	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// loadDatabaseConfig 加载 PostgreSQL 数据库配置
func loadDatabaseConfig() storage.DBConfig {
	configPath := getEnv("DB_CONFIG", "./data/database.yaml")

	// 默认配置
	defaultCfg := storage.DBConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "syncflow",
		Password: "syncflow",
		Database: "go_syncflow",
		SSLMode:  "disable",
		MaxIdle:  10,
		MaxOpen:  100,
	}

	// 尝试读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("[DB] 配置文件不存在，使用默认配置")
		return defaultCfg
	}

	var cfg DatabaseYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Printf("[DB] 解析配置文件失败: %v，使用默认配置", err)
		return defaultCfg
	}

	// 支持环境变量覆盖
	return storage.DBConfig{
		Host:     getEnvOrDefault("DB_HOST", cfg.Host, "127.0.0.1"),
		Port:     getEnvIntOrDefault("DB_PORT", cfg.Port, 5432),
		User:     getEnvOrDefault("DB_USER", cfg.User, "syncflow"),
		Password: getEnvOrDefault("DB_PASSWORD", cfg.Password, "syncflow"),
		Database: getEnvOrDefault("DB_DATABASE", cfg.Database, "go_syncflow"),
		SSLMode:  getEnvOrDefault("DB_SSLMODE", cfg.SSLMode, "disable"),
		MaxIdle:  cfg.MaxIdle,
		MaxOpen:  cfg.MaxOpen,
	}
}

func getEnvOrDefault(key, configValue, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if configValue != "" {
		return configValue
	}
	return fallback
}

func getEnvIntOrDefault(key string, configValue, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	if configValue > 0 {
		return configValue
	}
	return fallback
}
