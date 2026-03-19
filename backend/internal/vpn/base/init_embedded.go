package base

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"go-syncflow/internal/vpn/pkg/utils"
	"github.com/spf13/viper"
)

// InitEmbedded 程序化初始化 base 模块（不使用命令行参数）
// confPath 是配置文件的绝对路径
func InitEmbedded(confPath string) error {
	linkViper = viper.New()

	// 设置默认值
	for _, v := range configs {
		if v.Typ == cfgStr {
			linkViper.SetDefault(v.Name, v.ValStr)
		}
		if v.Typ == cfgInt {
			linkViper.SetDefault(v.Name, v.ValInt)
		}
		if v.Typ == cfgBool {
			linkViper.SetDefault(v.Name, v.ValBool)
		}
	}

	// 读取配置文件
	linkViper.SetConfigFile(confPath)
	if err := linkViper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取 VPN 配置文件失败: %w", err)
	}

	// 应用配置到 Cfg 结构体
	initCfgFromViper()

	// 初始化日志
	initLog()

	// 初始化模块
	initMod()

	return nil
}

// initCfgFromViper 从 viper 读取配置到 Cfg
func initCfgFromViper() {
	ref := reflect.ValueOf(Cfg)
	s := ref.Elem()

	typ := s.Type()
	numFields := s.NumField()
	for i := 0; i < numFields; i++ {
		field := typ.Field(i)
		value := s.Field(i)
		tag := field.Tag.Get("json")

		for _, v := range configs {
			if v.Name == tag {
				if v.Typ == cfgStr {
					value.SetString(linkViper.GetString(v.Name))
				}
				if v.Typ == cfgInt {
					value.SetInt(int64(linkViper.GetInt(v.Name)))
				}
				if v.Typ == cfgBool {
					value.SetBool(linkViper.GetBool(v.Name))
				}
			}
		}
	}

	// 获取配置文件目录作为基础路径
	confDir := filepath.Dir(linkViper.ConfigFileUsed())
	workDir, _ := os.Getwd()

	// 转换相对路径为绝对路径（仅针对 SQLite 文件路径，不处理 PostgreSQL/MySQL 等 DSN 字符串）
	// SQLite 路径特征：不包含 "host=" 或 "://" 等 DSN 特征
	if Cfg.DbSource != "" && !filepath.IsAbs(Cfg.DbSource) &&
		!strings.Contains(Cfg.DbSource, "host=") &&
		!strings.Contains(Cfg.DbSource, "://") &&
		!strings.Contains(Cfg.DbSource, "@tcp(") {
		Cfg.DbSource = filepath.Join(workDir, Cfg.DbSource)
	}
	if Cfg.CertFile != "" && !filepath.IsAbs(Cfg.CertFile) {
		Cfg.CertFile = filepath.Join(workDir, Cfg.CertFile)
	}
	if Cfg.CertKey != "" && !filepath.IsAbs(Cfg.CertKey) {
		Cfg.CertKey = filepath.Join(workDir, Cfg.CertKey)
	}
	if Cfg.Profile != "" && !filepath.IsAbs(Cfg.Profile) {
		Cfg.Profile = filepath.Join(workDir, Cfg.Profile)
	}
	if Cfg.FilesPath != "" && !filepath.IsAbs(Cfg.FilesPath) {
		Cfg.FilesPath = filepath.Join(workDir, Cfg.FilesPath)
	}

	// 检查 JWT Secret
	if Cfg.JwtSecret == defaultJwt || Cfg.JwtSecret == "" {
		jwtSecret, _ := utils.RandSecret(40, 60)
		jwtSecret = strings.Trim(jwtSecret, "=")
		Cfg.JwtSecret = jwtSecret
	}

	// 设置 DTLS 广播地址
	if Cfg.AdvertiseDTLSAddr == "" {
		Cfg.AdvertiseDTLSAddr = Cfg.ServerDTLSAddr
	}

	// 确保 files 目录存在
	if Cfg.FilesPath != "" {
		os.MkdirAll(Cfg.FilesPath, 0755)
	}

	// 确保 profile name 有值
	if Cfg.ProfileName == "" {
		Cfg.ProfileName = "syncflow-vpn"
	}

	_ = confDir // 暂时保留以避免编译警告
	log.Printf("[VPN] ServerCfg initialized: server_addr=%s, link_mode=%s", Cfg.ServerAddr, Cfg.LinkMode)
}
