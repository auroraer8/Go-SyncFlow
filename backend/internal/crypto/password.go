package crypto

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf16"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/sha3"
)

// 密码格式常量
const (
	PwdFormatBcryptSHA256 = "bcrypt_sha256" // Go-SyncFlow 默认
	PwdFormatBcrypt       = "bcrypt"
	PwdFormatArgon2id     = "argon2id"
	PwdFormatSHA256       = "sha256"
	PwdFormatSHA512       = "sha512"
	PwdFormatSHA1         = "sha1"
	PwdFormatSHA384       = "sha384"
	PwdFormatSHA3_256     = "sha3_256"
	PwdFormatSHA3_512     = "sha3_512"
	PwdFormatMD5          = "md5"
	PwdFormatSSHA         = "ssha"
	PwdFormatSSHA256      = "ssha256"
	PwdFormatNTHash       = "nt_hash"
	PwdFormatSCRAMSHA256  = "scram_sha256"
	PwdFormatPBKDF2       = "pbkdf2"
	PwdFormatDjango       = "django"
	PwdFormatPlaintext    = "plaintext"
)

// PasswordFormatInfo 密码格式信息
type PasswordFormatInfo struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Category    string `json:"category"` // recommended, common, compatible, other
	Secure      bool   `json:"secure"`
}

// AllPasswordFormats 所有支持的密码格式
var AllPasswordFormats = []PasswordFormatInfo{
	// 推荐
	{PwdFormatBcryptSHA256, "bcrypt + SHA256", "bcrypt(SHA256(pwd)) - Go-SyncFlow 默认", "recommended", true},
	{PwdFormatArgon2id, "Argon2id", "Argon2id(pwd) - 最新标准，推荐", "recommended", true},
	{PwdFormatBcrypt, "bcrypt", "bcrypt(pwd) - 标准安全存储", "recommended", true},
	{PwdFormatPBKDF2, "PBKDF2", "PBKDF2-HMAC-SHA256 - Web 标准", "recommended", true},
	// 通用
	{PwdFormatSHA256, "SHA256", "SHA256(pwd) - 通用系统，兼容性好", "common", false},
	{PwdFormatSHA512, "SHA512", "SHA512(pwd) - 通用系统", "common", false},
	{PwdFormatSHA384, "SHA384", "SHA384(pwd) - 高安全要求系统", "common", false},
	{PwdFormatSHA3_256, "SHA3-256", "SHA3-256(pwd) - 最新标准", "common", false},
	{PwdFormatSHA3_512, "SHA3-512", "SHA3-512(pwd) - 最新标准", "common", false},
	// 兼容
	{PwdFormatSSHA, "LDAP SSHA", "{SSHA}Base64(SHA1+salt) - OpenLDAP", "compatible", true},
	{PwdFormatSSHA256, "SSHA256", "{SSHA256}Base64(SHA256+salt) - 现代 LDAP", "compatible", true},
	{PwdFormatNTHash, "NT Hash (AD/Samba)", "MD4(UTF16LE(pwd)) - AD/Samba", "compatible", false},
	{PwdFormatSCRAMSHA256, "SCRAM-SHA-256", "PostgreSQL SCRAM", "compatible", true},
	{PwdFormatDjango, "Django (PBKDF2)", "pbkdf2_sha256$... - Django 迁移", "compatible", true},
	// 其他（不推荐）
	{PwdFormatSHA1, "SHA1 [不推荐]", "SHA1(pwd) - 老旧系统兼容", "other", false},
	{PwdFormatMD5, "MD5 [不推荐]", "MD5(pwd) - 仅兼容旧系统", "other", false},
	{PwdFormatPlaintext, "明文 [不安全]", "不加密 - 仅测试用", "other", false},
}

// HashPassword 根据格式哈希密码
func HashPassword(password, format string) (string, error) {
	switch format {
	case PwdFormatBcryptSHA256:
		h := sha256.Sum256([]byte(password))
		return bcryptHash(hex.EncodeToString(h[:]))
	case PwdFormatBcrypt:
		return bcryptHash(password)
	case PwdFormatArgon2id:
		return argon2idHash(password)
	case PwdFormatSHA256:
		h := sha256.Sum256([]byte(password))
		return hex.EncodeToString(h[:]), nil
	case PwdFormatSHA512:
		h := sha512.Sum512([]byte(password))
		return hex.EncodeToString(h[:]), nil
	case PwdFormatSHA1:
		h := sha1.Sum([]byte(password))
		return hex.EncodeToString(h[:]), nil
	case PwdFormatSHA384:
		h := sha512.Sum384([]byte(password))
		return hex.EncodeToString(h[:]), nil
	case PwdFormatSHA3_256:
		h := sha3.Sum256([]byte(password))
		return hex.EncodeToString(h[:]), nil
	case PwdFormatSHA3_512:
		h := sha3.Sum512([]byte(password))
		return hex.EncodeToString(h[:]), nil
	case PwdFormatMD5:
		h := md5.Sum([]byte(password))
		return hex.EncodeToString(h[:]), nil
	case PwdFormatSSHA:
		return generateSSHA(password)
	case PwdFormatSSHA256:
		return generateSSHA256(password)
	case PwdFormatNTHash:
		return ComputeNTHash(password), nil
	case PwdFormatSCRAMSHA256:
		return generateSCRAMSHA256(password)
	case PwdFormatPBKDF2:
		return generatePBKDF2(password)
	case PwdFormatDjango:
		return generateDjangoPBKDF2(password)
	case PwdFormatPlaintext:
		return password, nil
	default:
		return "", fmt.Errorf("unsupported password format: %s", format)
	}
}

// bcryptHash 使用 bcrypt 哈希
func bcryptHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// argon2idHash 使用 Argon2id 哈希
func argon2idHash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

// generateSSHA 生成 LDAP SSHA 格式: {SSHA}Base64(SHA1(password+salt)+salt)
func generateSSHA(password string) (string, error) {
	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := sha1.New()
	h.Write([]byte(password))
	h.Write(salt)
	digest := h.Sum(nil)
	combined := append(digest, salt...)
	return "{SSHA}" + base64.StdEncoding.EncodeToString(combined), nil
}

// generateSSHA256 生成 SSHA256 格式: {SSHA256}Base64(SHA256(password+salt)+salt)
func generateSSHA256(password string) (string, error) {
	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte(password))
	h.Write(salt)
	digest := h.Sum(nil)
	combined := append(digest, salt...)
	return "{SSHA256}" + base64.StdEncoding.EncodeToString(combined), nil
}

// ComputeNTHash 计算 NT Hash (Samba/AD): MD4(UTF16LE(password))
func ComputeNTHash(password string) string {
	encoded := utf16.Encode([]rune(password))
	buf := make([]byte, len(encoded)*2)
	for i, v := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	digest := md4(buf)
	return strings.ToUpper(hex.EncodeToString(digest[:]))
}

// generateSCRAMSHA256 生成 PostgreSQL SCRAM-SHA-256 格式
func generateSCRAMSHA256(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	iterations := 4096
	saltedPassword := pbkdf2.Key([]byte(password), salt, iterations, 32, sha256.New)
	clientKeyHMAC := hmacSHA256(saltedPassword, []byte("Client Key"))
	storedKey := sha256Sum(clientKeyHMAC)
	serverKey := hmacSHA256(saltedPassword, []byte("Server Key"))
	return fmt.Sprintf("SCRAM-SHA-256$%d:%s$%s:%s",
		iterations,
		base64.StdEncoding.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(storedKey),
		base64.StdEncoding.EncodeToString(serverKey)), nil
}

// generatePBKDF2 生成 PBKDF2-HMAC-SHA256 格式
func generatePBKDF2(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	iterations := 100000
	dk := pbkdf2.Key([]byte(password), salt, iterations, 32, sha256.New)
	return fmt.Sprintf("pbkdf2:sha256:%d$%s$%s",
		iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk)), nil
}

// generateDjangoPBKDF2 生成 Django PBKDF2 格式: pbkdf2_sha256$iterations$salt$hash
func generateDjangoPBKDF2(password string) (string, error) {
	salt := make([]byte, 12)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	saltB64 := base64.RawURLEncoding.EncodeToString(salt)
	iterations := 390000 // Django 4.x 默认
	dk := pbkdf2.Key([]byte(password), []byte(saltB64), iterations, 32, sha256.New)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s",
		iterations,
		saltB64,
		base64.StdEncoding.EncodeToString(dk)), nil
}

// hmacSHA256 计算 HMAC-SHA256
func hmacSHA256(key, message []byte) []byte {
	const blockSize = 64
	if len(key) > blockSize {
		h := sha256.Sum256(key)
		key = h[:]
	}
	if len(key) < blockSize {
		key = append(key, make([]byte, blockSize-len(key))...)
	}
	ipad := make([]byte, blockSize)
	opad := make([]byte, blockSize)
	for i := 0; i < blockSize; i++ {
		ipad[i] = key[i] ^ 0x36
		opad[i] = key[i] ^ 0x5c
	}
	inner := sha256.Sum256(append(ipad, message...))
	outer := sha256.Sum256(append(opad, inner[:]...))
	return outer[:]
}

// sha256Sum 计算 SHA256
func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// GetDefaultPasswordFormat 获取连接器类型的默认密码格式
func GetDefaultPasswordFormat(connectorType, dbType string) string {
	switch {
	case connectorType == "db_postgresql" || dbType == "postgresql":
		return PwdFormatSCRAMSHA256
	case connectorType == "db_mysql" || connectorType == "db_oracle" || connectorType == "db_sqlserver" ||
		dbType == "mysql" || dbType == "oracle" || dbType == "sqlserver":
		return PwdFormatSHA256
	case connectorType == "ldap_generic" || connectorType == "ldap_openldap":
		return PwdFormatSSHA
	case connectorType == "ldap_ad":
		return PwdFormatNTHash
	case connectorType == "radius":
		return PwdFormatPlaintext
	case connectorType == "http_api":
		return PwdFormatSHA256
	default:
		return PwdFormatBcryptSHA256
	}
}

// GetPasswordFormatLabel 获取密码格式的显示标签
func GetPasswordFormatLabel(format string) string {
	for _, f := range AllPasswordFormats {
		if f.Value == format {
			return f.Label
		}
	}
	return format
}

// md4 计算 MD4 哈希 (用于 NT Hash)
func md4(data []byte) [16]byte {
	var digest [16]byte
	var block [16]uint32
	
	// 填充
	msgLen := uint64(len(data))
	data = append(data, 0x80)
	for (len(data)+8)%64 != 0 {
		data = append(data, 0x00)
	}
	lenBits := msgLen * 8
	data = append(data, byte(lenBits), byte(lenBits>>8), byte(lenBits>>16), byte(lenBits>>24),
		byte(lenBits>>32), byte(lenBits>>40), byte(lenBits>>48), byte(lenBits>>56))
	
	// 初始化
	a, b, c, d := uint32(0x67452301), uint32(0xefcdab89), uint32(0x98badcfe), uint32(0x10325476)
	
	// 处理每个块
	for i := 0; i < len(data); i += 64 {
		for j := 0; j < 16; j++ {
			block[j] = uint32(data[i+j*4]) | uint32(data[i+j*4+1])<<8 | uint32(data[i+j*4+2])<<16 | uint32(data[i+j*4+3])<<24
		}
		
		aa, bb, cc, dd := a, b, c, d
		
		// Round 1
		for _, k := range []int{0, 4, 8, 12} {
			a = rotl(a+((b&c)|(^b&d))+block[k], 3)
			d = rotl(d+((a&b)|(^a&c))+block[k+1], 7)
			c = rotl(c+((d&a)|(^d&b))+block[k+2], 11)
			b = rotl(b+((c&d)|(^c&a))+block[k+3], 19)
		}
		// Round 2
		for _, k := range []int{0, 1, 2, 3} {
			a = rotl(a+((b&c)|(b&d)|(c&d))+block[k]+0x5a827999, 3)
			d = rotl(d+((a&b)|(a&c)|(b&c))+block[k+4]+0x5a827999, 5)
			c = rotl(c+((d&a)|(d&b)|(a&b))+block[k+8]+0x5a827999, 9)
			b = rotl(b+((c&d)|(c&a)|(d&a))+block[k+12]+0x5a827999, 13)
		}
		// Round 3
		for _, k := range []int{0, 2, 1, 3} {
			a = rotl(a+(b^c^d)+block[k]+0x6ed9eba1, 3)
			d = rotl(d+(a^b^c)+block[k+8]+0x6ed9eba1, 9)
			c = rotl(c+(d^a^b)+block[k+4]+0x6ed9eba1, 11)
			b = rotl(b+(c^d^a)+block[k+12]+0x6ed9eba1, 15)
		}
		
		a += aa
		b += bb
		c += cc
		d += dd
	}
	
	binary.LittleEndian.PutUint32(digest[0:], a)
	binary.LittleEndian.PutUint32(digest[4:], b)
	binary.LittleEndian.PutUint32(digest[8:], c)
	binary.LittleEndian.PutUint32(digest[12:], d)
	
	return digest
}

func rotl(x uint32, n uint) uint32 {
	return (x << n) | (x >> (32 - n))
}

// VerifyPassword 验证密码，支持多种存储格式
// storedPassword: 数据库中存储的密码哈希
// inputPassword: 用户输入的密码（可能是原文或已哈希）
// inputIsSHA256: 输入是否已经是 SHA256 哈希（前端预处理）
func VerifyPassword(storedPassword, inputPassword string, inputIsSHA256 bool) bool {
	if storedPassword == "" {
		return false
	}

	// 处理 {LDAP} 前缀格式（从 LDAP 同步的密码）
	if strings.HasPrefix(storedPassword, "{LDAP}") {
		ldapHash := storedPassword[6:] // 去掉 {LDAP} 前缀
		return verifyLDAPPassword(ldapHash, inputPassword, inputIsSHA256)
	}

	// 标准 bcrypt 格式
	if strings.HasPrefix(storedPassword, "$2a$") ||
		strings.HasPrefix(storedPassword, "$2b$") ||
		strings.HasPrefix(storedPassword, "$2y$") {
		// Go-SyncFlow 存储格式是 bcrypt(SHA256(password))
		// 如果输入已经是 SHA256，直接验证
		if inputIsSHA256 {
			return bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(inputPassword)) == nil
		}
		// 否则先计算 SHA256
		h := sha256.Sum256([]byte(inputPassword))
		sha256Hex := hex.EncodeToString(h[:])
		return bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(sha256Hex)) == nil
	}

	// Argon2id 格式
	if strings.HasPrefix(storedPassword, "$argon2id$") {
		return verifyArgon2id(storedPassword, inputPassword)
	}

	// 纯哈希格式（SHA256、MD5 等）
	if len(storedPassword) == 64 { // SHA256
		h := sha256.Sum256([]byte(inputPassword))
		return hex.EncodeToString(h[:]) == storedPassword
	}
	if len(storedPassword) == 32 { // MD5
		h := md5.Sum([]byte(inputPassword))
		return hex.EncodeToString(h[:]) == storedPassword
	}
	if len(storedPassword) == 40 { // SHA1
		h := sha1.Sum([]byte(inputPassword))
		return hex.EncodeToString(h[:]) == storedPassword
	}

	return false
}

// verifyLDAPPassword 验证 LDAP 格式的密码
func verifyLDAPPassword(ldapHash, inputPassword string, inputIsSHA256 bool) bool {
	// 如果输入是 SHA256 预处理的，需要先解密才能验证 LDAP 格式
	// 但这不可能，所以 LDAP 密码验证需要原始密码
	// 这种情况下应该通过密码代理认证

	// 直接支持的 LDAP 格式
	if strings.HasPrefix(ldapHash, "{SSHA}") {
		return verifySSHA(ldapHash, inputPassword)
	}
	if strings.HasPrefix(ldapHash, "{SSHA256}") {
		return verifySSHA256(ldapHash, inputPassword)
	}
	if strings.HasPrefix(ldapHash, "{SHA}") {
		return verifySHA(ldapHash, inputPassword)
	}
	if strings.HasPrefix(ldapHash, "{MD5}") {
		return verifyMD5LDAP(ldapHash, inputPassword)
	}
	if strings.HasPrefix(ldapHash, "{CLEARTEXT}") || strings.HasPrefix(ldapHash, "{PLAIN}") {
		endIdx := strings.Index(ldapHash, "}")
		if endIdx > 0 {
			return ldapHash[endIdx+1:] == inputPassword
		}
	}
	// bcrypt 格式（有些 LDAP 也支持）
	if strings.HasPrefix(ldapHash, "{BCRYPT}") ||
		strings.HasPrefix(ldapHash, "$2a$") ||
		strings.HasPrefix(ldapHash, "$2b$") ||
		strings.HasPrefix(ldapHash, "$2y$") {
		hash := ldapHash
		if strings.HasPrefix(ldapHash, "{BCRYPT}") {
			hash = ldapHash[8:]
		}
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(inputPassword)) == nil
	}

	return false
}

// verifySSHA 验证 LDAP SSHA 格式: {SSHA}Base64(SHA1(password+salt)+salt)
func verifySSHA(storedHash, password string) bool {
	if !strings.HasPrefix(storedHash, "{SSHA}") {
		return false
	}
	encoded := storedHash[6:]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) < 21 { // SHA1(20) + salt(至少1)
		return false
	}
	storedDigest := decoded[:20]
	salt := decoded[20:]

	h := sha1.New()
	h.Write([]byte(password))
	h.Write(salt)
	computed := h.Sum(nil)

	if len(storedDigest) != len(computed) {
		return false
	}
	for i := range storedDigest {
		if storedDigest[i] != computed[i] {
			return false
		}
	}
	return true
}

// verifySSHA256 验证 SSHA256 格式: {SSHA256}Base64(SHA256(password+salt)+salt)
func verifySSHA256(storedHash, password string) bool {
	if !strings.HasPrefix(storedHash, "{SSHA256}") {
		return false
	}
	encoded := storedHash[9:]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) < 33 { // SHA256(32) + salt(至少1)
		return false
	}
	storedDigest := decoded[:32]
	salt := decoded[32:]

	h := sha256.New()
	h.Write([]byte(password))
	h.Write(salt)
	computed := h.Sum(nil)

	if len(storedDigest) != len(computed) {
		return false
	}
	for i := range storedDigest {
		if storedDigest[i] != computed[i] {
			return false
		}
	}
	return true
}

// verifySHA 验证 LDAP {SHA} 格式: {SHA}Base64(SHA1(password))
func verifySHA(storedHash, password string) bool {
	if !strings.HasPrefix(storedHash, "{SHA}") {
		return false
	}
	encoded := storedHash[5:]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) != 20 {
		return false
	}
	h := sha1.Sum([]byte(password))
	for i := range decoded {
		if decoded[i] != h[i] {
			return false
		}
	}
	return true
}

// verifyMD5LDAP 验证 LDAP {MD5} 格式: {MD5}Base64(MD5(password))
func verifyMD5LDAP(storedHash, password string) bool {
	if !strings.HasPrefix(storedHash, "{MD5}") {
		return false
	}
	encoded := storedHash[5:]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) != 16 {
		return false
	}
	h := md5.Sum([]byte(password))
	for i := range decoded {
		if decoded[i] != h[i] {
			return false
		}
	}
	return true
}

// verifyArgon2id 验证 Argon2id 格式
func verifyArgon2id(storedHash, password string) bool {
	// 格式: $argon2id$v=19$m=65536,t=1,p=4$salt$hash
	if !strings.HasPrefix(storedHash, "$argon2id$") {
		return false
	}
	parts := strings.Split(storedHash, "$")
	if len(parts) != 6 {
		return false
	}
	// parts[0] = "", parts[1] = "argon2id", parts[2] = "v=19", parts[3] = params, parts[4] = salt, parts[5] = hash
	var m, t, p uint32
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p)
	if err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	computed := argon2.IDKey([]byte(password), salt, t, m, uint8(p), uint32(len(expectedHash)))

	if len(expectedHash) != len(computed) {
		return false
	}
	for i := range expectedHash {
		if expectedHash[i] != computed[i] {
			return false
		}
	}
	return true
}
