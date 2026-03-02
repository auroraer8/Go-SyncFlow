package services

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"go-syncflow/internal/storage"
)

// ========== RSA 密码传输加密 ==========
//
// 安全架构说明：
// - 用途：保护前端→后端的明文密码传输（用于 AD 域同步）
// - 算法：RSA-2048 + OAEP + SHA-256
// - 与 SSL/HTTPS 的关系：完全独立，SSL 证书用于传输层加密，本模块用于应用层加密
//
// 密钥来源优先级：
// 1. 管理员自定义密钥对（存储在数据库配置中）
// 2. 复用 HTTPS 证书的 RSA 密钥对（如果 HTTPS 配置了 RSA 证书）
// 3. 自动生成并持久化到 data/rsa_private.pem（默认）

var (
	rsaPrivateKey *rsa.PrivateKey
	rsaPublicPEM  string
	rsaMu         sync.RWMutex
)

const (
	rsaKeyFile    = "data/rsa_private.pem"
	rsaConfigKey  = "rsa_crypto" // 数据库配置键
)

// RSACryptoConfig 存储在数据库中的 RSA 配置
type RSACryptoConfig struct {
	Source     string `json:"source"`     // "auto" | "custom" | "https"
	PrivateKey string `json:"privateKey"` // PEM 格式私钥（仅 custom 模式）
	PublicKey  string `json:"publicKey"`  // PEM 格式公钥（仅 custom 模式）
}

// InitRSAKeyPair 初始化 RSA 密钥对
func InitRSAKeyPair() {
	rsaMu.Lock()
	defer rsaMu.Unlock()

	// 1. 尝试从数据库配置加载（自定义或 HTTPS 模式）
	if loadFromConfig() {
		return
	}

	// 2. 尝试从持久化文件加载
	if loadFromFile() {
		return
	}

	// 3. 自动生成并持久化
	generateAndSave()
}

// loadFromConfig 从数据库配置加载密钥对
func loadFromConfig() bool {
	value, _ := storage.GetConfig(rsaConfigKey)
	if value == "" {
		return false
	}

	var cfg RSACryptoConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return false
	}

	switch cfg.Source {
	case "custom":
		if cfg.PrivateKey == "" {
			return false
		}
		key, pub, err := parsePEMKeyPair(cfg.PrivateKey, cfg.PublicKey)
		if err != nil {
			log.Printf("[加密] 自定义密钥对解析失败: %v，将回退到自动模式", err)
			return false
		}
		rsaPrivateKey = key
		rsaPublicPEM = pub
		log.Printf("[加密] RSA 密钥对已加载（来源：管理员自定义）")
		return true

	case "https":
		if loadFromHTTPS() {
			log.Printf("[加密] RSA 密钥对已加载（来源：HTTPS 证书）")
			return true
		}
		log.Printf("[加密] HTTPS 证书不可用或不是 RSA 类型，将回退到自动模式")
		return false
	}

	return false
}

// loadFromHTTPS 从 HTTPS 证书加载 RSA 密钥对
func loadFromHTTPS() bool {
	httpsVal, _ := storage.GetConfig("https")
	if httpsVal == "" {
		return false
	}

	var httpsCfg struct {
		Enabled  bool   `json:"enabled"`
		KeyFile  string `json:"keyFile"`
		CertFile string `json:"certFile"`
	}
	if err := json.Unmarshal([]byte(httpsVal), &httpsCfg); err != nil || !httpsCfg.Enabled {
		return false
	}

	if httpsCfg.KeyFile == "" {
		return false
	}

	// 读取 HTTPS 私钥
	keyData, err := os.ReadFile(httpsCfg.KeyFile)
	if err != nil {
		return false
	}

	key, pub, err := parsePEMKeyPair(string(keyData), "")
	if err != nil {
		return false
	}

	rsaPrivateKey = key
	rsaPublicPEM = pub
	return true
}

// loadFromFile 从持久化文件加载密钥对
func loadFromFile() bool {
	keyData, err := os.ReadFile(rsaKeyFile)
	if err != nil {
		return false
	}

	key, pub, err := parsePEMKeyPair(string(keyData), "")
	if err != nil {
		log.Printf("[加密] 持久化密钥文件解析失败: %v，将重新生成", err)
		return false
	}

	rsaPrivateKey = key
	rsaPublicPEM = pub
	log.Printf("[加密] RSA 密钥对已加载（来源：持久化文件 %s）", rsaKeyFile)
	return true
}

// generateAndSave 生成新密钥对并持久化
func generateAndSave() {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("[加密] RSA 密钥对生成失败: %v", err)
	}
	rsaPrivateKey = key
	rsaPublicPEM = exportPublicKeyPEM(&key.PublicKey)

	// 持久化私钥到文件
	dir := filepath.Dir(rsaKeyFile)
	os.MkdirAll(dir, 0700)

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.WriteFile(rsaKeyFile, privPEM, 0600); err != nil {
		log.Printf("[加密] 密钥持久化失败: %v（密钥仅存于内存）", err)
	} else {
		log.Printf("[加密] RSA-2048 密钥对已生成并持久化到 %s", rsaKeyFile)
	}
}

// parsePEMKeyPair 解析 PEM 格式的私钥，提取公钥
func parsePEMKeyPair(privatePEM string, publicPEM string) (*rsa.PrivateKey, string, error) {
	block, _ := pem.Decode([]byte(privatePEM))
	if block == nil {
		return nil, "", fmt.Errorf("无法解析 PEM 数据")
	}

	var rsaKey *rsa.PrivateKey
	var err error

	switch block.Type {
	case "RSA PRIVATE KEY":
		rsaKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, e := x509.ParsePKCS8PrivateKey(block.Bytes)
		if e != nil {
			return nil, "", fmt.Errorf("PKCS8 解析失败: %v", e)
		}
		var ok bool
		rsaKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, "", fmt.Errorf("私钥不是 RSA 类型（可能是 ECDSA 或 Ed25519）")
		}
	default:
		return nil, "", fmt.Errorf("不支持的 PEM 类型: %s", block.Type)
	}
	if err != nil {
		return nil, "", fmt.Errorf("私钥解析失败: %v", err)
	}

	// 如果提供了公钥 PEM 就用它，否则从私钥导出
	pub := publicPEM
	if pub == "" {
		pub = exportPublicKeyPEM(&rsaKey.PublicKey)
	}

	return rsaKey, pub, nil
}

// exportPublicKeyPEM 从公钥导出 PEM 字符串
func exportPublicKeyPEM(pub *rsa.PublicKey) string {
	pubASN1, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		log.Printf("[加密] 公钥序列化失败: %v", err)
		return ""
	}
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	}))
}

// ========== 对外接口 ==========

// GetRSAPublicKeyPEM 获取 PEM 格式的公钥（提供给前端）
func GetRSAPublicKeyPEM() string {
	rsaMu.RLock()
	defer rsaMu.RUnlock()
	return rsaPublicPEM
}

// GetRSACryptoConfig 获取当前 RSA 加密配置
func GetRSACryptoConfig() RSACryptoConfig {
	value, _ := storage.GetConfig(rsaConfigKey)
	if value == "" {
		return RSACryptoConfig{Source: "auto"}
	}
	var cfg RSACryptoConfig
	json.Unmarshal([]byte(value), &cfg)
	// 不暴露私钥内容，只返回来源和是否已配置
	cfg.PrivateKey = ""
	if cfg.Source == "custom" {
		cfg.PrivateKey = "[已配置]"
	}
	return cfg
}

// SetRSACryptoSource 设置 RSA 密钥来源模式
func SetRSACryptoSource(source string, privatePEM string, publicPEM string) error {
	rsaMu.Lock()
	defer rsaMu.Unlock()

	cfg := RSACryptoConfig{Source: source}

	switch source {
	case "custom":
		if privatePEM == "" {
			return fmt.Errorf("自定义模式需要提供 RSA 私钥")
		}
		// 验证密钥对是否有效
		key, pub, err := parsePEMKeyPair(privatePEM, publicPEM)
		if err != nil {
			return fmt.Errorf("密钥对无效: %v", err)
		}
		cfg.PrivateKey = privatePEM
		cfg.PublicKey = pub

		// 立即生效
		rsaPrivateKey = key
		rsaPublicPEM = pub
		log.Printf("[加密] RSA 密钥对已切换为自定义密钥")

	case "https":
		if !loadFromHTTPS() {
			return fmt.Errorf("HTTPS 证书不可用或不是 RSA 类型")
		}
		log.Printf("[加密] RSA 密钥对已切换为 HTTPS 证书")

	case "auto":
		// 恢复自动模式，重新加载持久化文件或重新生成
		if !loadFromFile() {
			generateAndSave()
		}
		log.Printf("[加密] RSA 密钥对已切换为自动模式")

	default:
		return fmt.Errorf("不支持的模式: %s", source)
	}

	// 保存配置
	saveCfg := cfg
	if source != "custom" {
		saveCfg.PrivateKey = ""
		saveCfg.PublicKey = ""
	}
	data, _ := json.Marshal(saveCfg)
	storage.SetConfig(rsaConfigKey, string(data))

	return nil
}

// RSADecrypt 使用私钥解密前端 RSA-OAEP 加密的数据
func RSADecrypt(cipherBase64 string) (string, error) {
	if cipherBase64 == "" {
		return "", nil
	}

	rsaMu.RLock()
	key := rsaPrivateKey
	rsaMu.RUnlock()

	if key == nil {
		return "", fmt.Errorf("RSA 私钥未初始化")
	}

	cipherBytes, err := base64.StdEncoding.DecodeString(cipherBase64)
	if err != nil {
		return "", fmt.Errorf("Base64 解码失败: %v", err)
	}

	plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, key, cipherBytes, nil)
	if err != nil {
		return "", fmt.Errorf("RSA 解密失败: %v", err)
	}

	return string(plaintext), nil
}
