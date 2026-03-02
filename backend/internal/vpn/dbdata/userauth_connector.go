package dbdata

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"unicode/utf16"

	"go-syncflow/internal/models"
	"go-syncflow/internal/services"
	"go-syncflow/internal/storage"
	"go-syncflow/internal/sync"

	"golang.org/x/crypto/bcrypt"
)

// AuthConnector 统一连接器认证（支持 LDAP/数据库/RADIUS/HTTP API）
// 替代原有的 AuthLdapConnector，支持所有可认证的连接器类型
type AuthConnector struct {
	ConnectorID uint `json:"connector_id"`
}

func init() {
	// 注册为新的认证类型 "connector"
	authRegistry["connector"] = reflect.TypeOf(AuthConnector{})
}

func (auth AuthConnector) checkData(authData map[string]interface{}) error {
	connectorIDFloat, ok := authData["connector_id"].(float64)
	if !ok || connectorIDFloat <= 0 {
		return errors.New("请选择上游连接器")
	}

	connectorID := uint(connectorIDFloat)
	var connector models.Connector
	if err := storage.DB.Where("id = ? AND status = 1", connectorID).First(&connector).Error; err != nil {
		return errors.New("指定的连接器不存在或未启用")
	}

	if !connector.CanAuthenticate() {
		return errors.New("指定的连接器不支持认证功能")
	}

	return nil
}

func (auth AuthConnector) checkUser(name, pwd string, g *Group, ext map[string]interface{}) error {
	if name == "" || len(pwd) < 1 {
		return fmt.Errorf("%s 用户名或密码不能为空", name)
	}

	connectorIDFloat, ok := g.Auth["connector_id"].(float64)
	if !ok {
		return fmt.Errorf("%s 连接器配置错误", name)
	}

	connectorID := uint(connectorIDFloat)
	var connector models.Connector
	if err := storage.DB.Where("id = ? AND status = 1", connectorID).First(&connector).Error; err != nil {
		return fmt.Errorf("%s 指定的连接器不存在或未启用", name)
	}

	log.Printf("[VPN-连接器认证] 使用 %s (%s) 认证用户: %s", connector.Name, connector.Type, name)

	// 使用统一认证接口
	ok, err := services.AuthenticateWithConnector(&connector, name, pwd, "username")
	if err != nil {
		log.Printf("[VPN-连接器认证] 用户 %s 认证失败: %v", name, err)
		return fmt.Errorf("%s 认证失败: %v", name, err)
	}

	if !ok {
		return fmt.Errorf("%s 密码验证失败", name)
	}

	log.Printf("[VPN-连接器认证] 用户 %s 认证成功", name)

	// 检查是否需要密码学习（如果本地有对应用户）
	learnPassword := false
	if learnPwdVal, exists := g.Auth["learn_password"]; exists {
		if lp, ok := learnPwdVal.(bool); ok {
			learnPassword = lp
		}
	}

	if learnPassword {
		var localUser models.User
		if err := storage.DB.Where("username = ? AND is_deleted = 0", name).First(&localUser).Error; err == nil {
			// 本地有对应用户，学习密码
			cfg := services.GetPasswordAuthProxyConfig()
			if err := learnUserPasswordForVPN(&localUser, pwd, cfg.LearnSambaNT); err != nil {
				log.Printf("[VPN-密码学习] 用户 %s 密码学习失败: %v", name, err)
			} else {
				log.Printf("[VPN-密码学习] 用户 %s 密码学习成功", name)
				// 触发密码同步事件
				sync.DispatchSyncEvent(models.SyncEventPasswordChange, localUser.ID, pwd)
			}
		}
	}

	return nil
}

// GetConnectorUserPhone 获取连接器中用户的手机号
// 目前仅支持 LDAP 类型连接器
func GetConnectorUserPhone(username string, connectorID uint) (string, error) {
	// 复用已有的 LDAP 连接器获取手机号方法
	return GetLdapConnectorUserPhone(username, connectorID)
}

// learnUserPasswordForVPN VPN 认证时学习用户密码
func learnUserPasswordForVPN(user *models.User, rawPassword string, learnSambaNT bool) error {
	// SHA256 哈希（与前端登录一致）
	h := sha256.Sum256([]byte(rawPassword))
	passwordHash := hex.EncodeToString(h[:])

	// bcrypt 哈希
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordHash), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"password": string(hashedPassword),
	}

	if learnSambaNT {
		updates["samba_nt_password"] = computeNTHashForVPN(rawPassword)
	}

	return storage.DB.Model(user).Updates(updates).Error
}

// computeNTHashForVPN 计算 Samba NT 密码哈希
func computeNTHashForVPN(password string) string {
	encoded := utf16.Encode([]rune(password))
	buf := make([]byte, len(encoded)*2)
	for i, v := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	digest := md4ForVPN(buf)
	return strings.ToUpper(hex.EncodeToString(digest[:]))
}

// md4ForVPN MD4 实现（用于 NT Hash）
func md4ForVPN(data []byte) [16]byte {
	var a0, b0, c0, d0 uint32 = 0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476

	origLen := uint64(len(data)) * 8
	data = append(data, 0x80)
	for len(data)%64 != 56 {
		data = append(data, 0)
	}
	lenBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(lenBytes, origLen)
	data = append(data, lenBytes...)

	md4F := func(x, y, z uint32) uint32 { return (x & y) | (^x & z) }
	md4G := func(x, y, z uint32) uint32 { return (x & y) | (x & z) | (y & z) }
	md4H := func(x, y, z uint32) uint32 { return x ^ y ^ z }
	md4Rot := func(x uint32, s uint) uint32 { return (x << s) | (x >> (32 - s)) }

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
				a = md4Rot(a+md4F(b, c, d)+x[k], s)
			case 1:
				d = md4Rot(d+md4F(a, b, c)+x[k], s)
			case 2:
				c = md4Rot(c+md4F(d, a, b)+x[k], s)
			case 3:
				b = md4Rot(b+md4F(c, d, a)+x[k], s)
			}
		}

		// Round 2
		r2Order := []int{0, 4, 8, 12, 1, 5, 9, 13, 2, 6, 10, 14, 3, 7, 11, 15}
		r2Shifts := []uint{3, 5, 9, 13}
		for i, k := range r2Order {
			s := r2Shifts[i%4]
			switch i % 4 {
			case 0:
				a = md4Rot(a+md4G(b, c, d)+x[k]+0x5A827999, s)
			case 1:
				d = md4Rot(d+md4G(a, b, c)+x[k]+0x5A827999, s)
			case 2:
				c = md4Rot(c+md4G(d, a, b)+x[k]+0x5A827999, s)
			case 3:
				b = md4Rot(b+md4G(c, d, a)+x[k]+0x5A827999, s)
			}
		}

		// Round 3
		r3Order := []int{0, 8, 4, 12, 2, 10, 6, 14, 1, 9, 5, 13, 3, 11, 7, 15}
		r3Shifts := []uint{3, 9, 11, 15}
		for i, k := range r3Order {
			s := r3Shifts[i%4]
			switch i % 4 {
			case 0:
				a = md4Rot(a+md4H(b, c, d)+x[k]+0x6ED9EBA1, s)
			case 1:
				d = md4Rot(d+md4H(a, b, c)+x[k]+0x6ED9EBA1, s)
			case 2:
				c = md4Rot(c+md4H(d, a, b)+x[k]+0x6ED9EBA1, s)
			case 3:
				b = md4Rot(b+md4H(c, d, a)+x[k]+0x6ED9EBA1, s)
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
