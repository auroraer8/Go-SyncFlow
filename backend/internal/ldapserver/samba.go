package ldapserver

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"unicode/utf16"
)

// ============================================================
// MD4 实现 (RFC 1320) - 仅用于 Samba NT Hash 计算
// ============================================================

func leftRotate(x uint32, s uint) uint32 {
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
				a = leftRotate(a+md4F(b, c, d)+x[k], s)
			case 1:
				d = leftRotate(d+md4F(a, b, c)+x[k], s)
			case 2:
				c = leftRotate(c+md4F(d, a, b)+x[k], s)
			case 3:
				b = leftRotate(b+md4F(c, d, a)+x[k], s)
			}
		}

		// Round 2
		r2Order := []int{0, 4, 8, 12, 1, 5, 9, 13, 2, 6, 10, 14, 3, 7, 11, 15}
		r2Shifts := []uint{3, 5, 9, 13}
		for i, k := range r2Order {
			s := r2Shifts[i%4]
			switch i % 4 {
			case 0:
				a = leftRotate(a+md4G(b, c, d)+x[k]+0x5A827999, s)
			case 1:
				d = leftRotate(d+md4G(a, b, c)+x[k]+0x5A827999, s)
			case 2:
				c = leftRotate(c+md4G(d, a, b)+x[k]+0x5A827999, s)
			case 3:
				b = leftRotate(b+md4G(c, d, a)+x[k]+0x5A827999, s)
			}
		}

		// Round 3
		r3Order := []int{0, 8, 4, 12, 2, 10, 6, 14, 1, 9, 5, 13, 3, 11, 7, 15}
		r3Shifts := []uint{3, 9, 11, 15}
		for i, k := range r3Order {
			s := r3Shifts[i%4]
			switch i % 4 {
			case 0:
				a = leftRotate(a+md4H(b, c, d)+x[k]+0x6ED9EBA1, s)
			case 1:
				d = leftRotate(d+md4H(a, b, c)+x[k]+0x6ED9EBA1, s)
			case 2:
				c = leftRotate(c+md4H(d, a, b)+x[k]+0x6ED9EBA1, s)
			case 3:
				b = leftRotate(b+md4H(c, d, a)+x[k]+0x6ED9EBA1, s)
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

// ComputeNTHash 计算 Samba NT 密码哈希: MD4(UTF16LE(password))
// 返回 32 字符大写十六进制字符串
func ComputeNTHash(password string) string {
	encoded := utf16.Encode([]rune(password))
	buf := make([]byte, len(encoded)*2)
	for i, v := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	digest := md4(buf)
	return strings.ToUpper(hex.EncodeToString(digest[:]))
}

// GenerateUserSID 生成用户 Samba SID
func GenerateUserSID(domainSID string, userID uint) string {
	rid := userID*2 + 1000
	return fmt.Sprintf("%s-%d", domainSID, rid)
}

// GenerateGroupSID 生成角色 Samba SID
func GenerateGroupSID(domainSID string, groupID uint) string {
	rid := groupID*2 + 1001
	return fmt.Sprintf("%s-%d", domainSID, rid)
}

// GenerateDomainSID 根据域名生成一个确定性的域 SID
func GenerateDomainSID(domain string) string {
	h := sha256.Sum256([]byte(domain))
	a := binary.LittleEndian.Uint32(h[0:4]) % uint32(math.MaxInt32)
	b := binary.LittleEndian.Uint32(h[4:8]) % uint32(math.MaxInt32)
	c := binary.LittleEndian.Uint32(h[8:12]) % uint32(math.MaxInt32)
	return fmt.Sprintf("S-1-5-21-%d-%d-%d", a, b, c)
}
