package services

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"unicode/utf16"

	"golang.org/x/crypto/bcrypt"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
	syncEngine "go-syncflow/internal/sync"
)

var _ = json.Marshal

// PasswordAuthProxyConfig 密码代理认证配置
type PasswordAuthProxyConfig struct {
	Enabled         bool   `json:"enabled"`         // 是否启用
	ConnectorID     uint   `json:"connectorId"`     // 连接器 ID（支持 LDAP/数据库/RADIUS/HTTP API）
	ConnectorType   string `json:"connectorType"`   // 连接器类型（显示用）
	LearnPassword   bool   `json:"learnPassword"`   // 是否学习密码
	LearnSambaNT    bool   `json:"learnSambaNT"`    // 是否同时学习 Samba NT Hash
	FallbackToLocal bool   `json:"fallbackToLocal"` // 本地优先（先尝试本地，失败再代理）
	MatchField      string `json:"matchField"`      // 匹配字段: username/email/phone
}

// GetPasswordAuthProxyConfig 获取密码代理配置
func GetPasswordAuthProxyConfig() *PasswordAuthProxyConfig {
	cfg := &PasswordAuthProxyConfig{
		MatchField:      "username",
		FallbackToLocal: true,
		LearnPassword:   true,
		LearnSambaNT:    true,
	}

	var sysConfig models.SystemConfig
	if err := storage.DB.Where("key = ?", "password_auth_proxy").First(&sysConfig).Error; err == nil {
		json.Unmarshal([]byte(sysConfig.Value), cfg)
	}

	return cfg
}

// SavePasswordAuthProxyConfig 保存密码代理配置
func SavePasswordAuthProxyConfig(cfg *PasswordAuthProxyConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	var sysConfig models.SystemConfig
	if err := storage.DB.Where("key = ?", "password_auth_proxy").First(&sysConfig).Error; err != nil {
		sysConfig = models.SystemConfig{
			Key:   "password_auth_proxy",
			Value: string(data),
		}
		return storage.DB.Create(&sysConfig).Error
	}

	sysConfig.Value = string(data)
	return storage.DB.Save(&sysConfig).Error
}

// ProxyAuthenticateUser 代理认证用户
// 参数：
//   - user: 本地用户
//   - rawPassword: 用户输入的明文密码
//
// 返回：
//   - authenticated: 是否认证成功
//   - learned: 是否学习了密码
//   - error: 错误信息
func ProxyAuthenticateUser(user *models.User, rawPassword string) (authenticated bool, learned bool, err error) {
	cfg := GetPasswordAuthProxyConfig()
	if !cfg.Enabled || cfg.ConnectorID == 0 {
		return false, false, fmt.Errorf("密码代理认证未启用")
	}

	// 1. 获取连接器配置
	var conn models.Connector
	if err := storage.DB.First(&conn, cfg.ConnectorID).Error; err != nil {
		return false, false, fmt.Errorf("连接器不存在: %d", cfg.ConnectorID)
	}

	// 2. 检查连接器是否支持认证
	if !conn.CanAuthenticate() {
		return false, false, fmt.Errorf("连接器类型不支持认证: %s", conn.Type)
	}

	// 3. 确定用于搜索/匹配的用户名
	var searchValue string
	switch cfg.MatchField {
	case "email":
		searchValue = user.Email
	case "phone":
		searchValue = user.Phone
	default:
		searchValue = user.Username
	}

	if searchValue == "" {
		return false, false, fmt.Errorf("用户 %s 的 %s 字段为空", user.Username, cfg.MatchField)
	}

	log.Printf("[密码代理] 使用连接器 %s (%s) 认证用户: %s", conn.Name, conn.Type, searchValue)

	// 4. 调用统一认证接口
	ok, authErr := AuthenticateWithConnector(&conn, searchValue, rawPassword, cfg.MatchField)
	if authErr != nil {
		return false, false, fmt.Errorf("认证失败: %v", authErr)
	}

	if !ok {
		return false, false, fmt.Errorf("密码验证失败")
	}

	log.Printf("[密码代理] 用户 %s 认证成功", user.Username)

	// 5. 认证成功，学习密码
	if cfg.LearnPassword {
		if err := learnUserPassword(user, rawPassword, cfg.LearnSambaNT); err != nil {
			log.Printf("[密码学习] 用户 %s 密码学习失败: %v", user.Username, err)
			storage.DB.Create(&models.OperationLog{
				UserID:   user.ID,
				Username: user.Username,
				Module:   "密码代理",
				Action:   "密码学习失败",
				Target:   user.Username,
				Content:  fmt.Sprintf("认证后端: %s, 错误: %v", cfg.ConnectorType, err),
			})
		} else {
			log.Printf("[密码学习] 用户 %s 密码学习成功", user.Username)
			learned = true
			storage.DB.Create(&models.OperationLog{
				UserID:   user.ID,
				Username: user.Username,
				Module:   "密码代理",
				Action:   "密码学习成功",
				Target:   user.Username,
				Content:  fmt.Sprintf("认证后端: %s", cfg.ConnectorType),
			})
		}
	}

	return true, learned, nil
}

// learnUserPassword 学习用户密码并触发事件同步
func learnUserPassword(user *models.User, rawPassword string, learnSambaNT bool) error {
	// SHA256 哈希（与前端登录一致）
	passwordHash := hashSHA256ForProxy(rawPassword)
	// bcrypt 哈希
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordHash), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"password": string(hashedPassword),
	}

	if learnSambaNT {
		updates["samba_nt_password"] = computeNTHashForProxy(rawPassword)
	}

	if err := storage.DB.Model(user).Updates(updates).Error; err != nil {
		return err
	}

	// 触发密码变更事件同步
	go triggerPasswordChangeSync(user, rawPassword)

	return nil
}

// triggerPasswordChangeSync 触发密码变更的下游事件同步
func triggerPasswordChangeSync(user *models.User, rawPassword string) {
	log.Printf("[密码学习] 分发密码变更同步事件: user=%s(id=%d)", user.Username, user.ID)
	syncEngine.DispatchSyncEvent("password_change", user.ID, rawPassword)
}


// hashSHA256ForProxy 计算字符串的 SHA256 哈希
func hashSHA256ForProxy(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// IsPasswordAuthProxyEnabled 检查密码代理是否启用
func IsPasswordAuthProxyEnabled() bool {
	cfg := GetPasswordAuthProxyConfig()
	return cfg.Enabled && cfg.ConnectorID > 0
}

// GetProxyConnectorInfo 获取代理连接器信息（用于前端显示）
func GetProxyConnectorInfo() (connectorName string, connectorID uint) {
	cfg := GetPasswordAuthProxyConfig()
	if !cfg.Enabled || cfg.ConnectorID == 0 {
		return "", 0
	}

	var conn models.Connector
	if err := storage.DB.First(&conn, cfg.ConnectorID).Error; err != nil {
		return "", 0
	}

	return conn.Name, conn.ID
}

// ============================================================
// MD4 实现 (RFC 1320) - 仅用于 Samba NT Hash 计算
// ============================================================

func md4LeftRotate(x uint32, s uint) uint32 {
	return (x << s) | (x >> (32 - s))
}

func md4F(x, y, z uint32) uint32 { return (x & y) | (^x & z) }
func md4G(x, y, z uint32) uint32 { return (x & y) | (x & z) | (y & z) }
func md4H(x, y, z uint32) uint32 { return x ^ y ^ z }

func md4(data []byte) [16]byte {
	var a0, b0, c0, d0 uint32 = 0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476

	origLen := uint64(len(data)) * 8
	data = append(data, 0x80)
	for len(data)%64 != 56 {
		data = append(data, 0)
	}
	lenBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(lenBytes, origLen)
	data = append(data, lenBytes...)

	for offset := 0; offset < len(data); offset += 64 {
		var x [16]uint32
		for j := 0; j < 16; j++ {
			x[j] = binary.LittleEndian.Uint32(data[offset+j*4 : offset+j*4+4])
		}

		a, b, c, d := a0, b0, c0, d0

		// Round 1
		for _, v := range [][2]int{{0, 3}, {1, 7}, {2, 11}, {3, 19}, {4, 3}, {5, 7}, {6, 11}, {7, 19}, {8, 3}, {9, 7}, {10, 11}, {11, 19}, {12, 3}, {13, 7}, {14, 11}, {15, 19}} {
			k, s := v[0], uint(v[1])
			switch k % 4 {
			case 0:
				a = md4LeftRotate(a+md4F(b, c, d)+x[k], s)
			case 1:
				d = md4LeftRotate(d+md4F(a, b, c)+x[k], s)
			case 2:
				c = md4LeftRotate(c+md4F(d, a, b)+x[k], s)
			case 3:
				b = md4LeftRotate(b+md4F(c, d, a)+x[k], s)
			}
		}

		// Round 2
		r2Order := []int{0, 4, 8, 12, 1, 5, 9, 13, 2, 6, 10, 14, 3, 7, 11, 15}
		r2Shifts := []uint{3, 5, 9, 13}
		for i, k := range r2Order {
			s := r2Shifts[i%4]
			switch i % 4 {
			case 0:
				a = md4LeftRotate(a+md4G(b, c, d)+x[k]+0x5A827999, s)
			case 1:
				d = md4LeftRotate(d+md4G(a, b, c)+x[k]+0x5A827999, s)
			case 2:
				c = md4LeftRotate(c+md4G(d, a, b)+x[k]+0x5A827999, s)
			case 3:
				b = md4LeftRotate(b+md4G(c, d, a)+x[k]+0x5A827999, s)
			}
		}

		// Round 3
		r3Order := []int{0, 8, 4, 12, 2, 10, 6, 14, 1, 9, 5, 13, 3, 11, 7, 15}
		r3Shifts := []uint{3, 9, 11, 15}
		for i, k := range r3Order {
			s := r3Shifts[i%4]
			switch i % 4 {
			case 0:
				a = md4LeftRotate(a+md4H(b, c, d)+x[k]+0x6ED9EBA1, s)
			case 1:
				d = md4LeftRotate(d+md4H(a, b, c)+x[k]+0x6ED9EBA1, s)
			case 2:
				c = md4LeftRotate(c+md4H(d, a, b)+x[k]+0x6ED9EBA1, s)
			case 3:
				b = md4LeftRotate(b+md4H(c, d, a)+x[k]+0x6ED9EBA1, s)
			}
		}

		a0 += a
		b0 += b
		c0 += c
		d0 += d
	}

	var digest [16]byte
	binary.LittleEndian.PutUint32(digest[0:4], a0)
	binary.LittleEndian.PutUint32(digest[4:8], b0)
	binary.LittleEndian.PutUint32(digest[8:12], c0)
	binary.LittleEndian.PutUint32(digest[12:16], d0)
	return digest
}

// computeNTHashForProxy 计算 Samba NT 密码哈希: MD4(UTF16LE(password))
// 返回 32 字符大写十六进制字符串
func computeNTHashForProxy(password string) string {
	encoded := utf16.Encode([]rune(password))
	buf := make([]byte, len(encoded)*2)
	for i, v := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	digest := md4(buf)
	return strings.ToUpper(hex.EncodeToString(digest[:]))
}
