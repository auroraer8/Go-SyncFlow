package imclient

import (
	"fmt"
	"strings"

	"github.com/mozillazg/go-pinyin"

	"go-syncflow/internal/storage"
)

// 用户名生成策略常量
const (
	StrategyEmailPrefix = "email_prefix"
	StrategyEmail       = "email"
	StrategyIMUserID    = "im_userid"
	StrategyMobile      = "mobile"
	StrategyPinyin      = "pinyin"
)

// GenerateUsername 根据策略从 IM 用户信息生成本地用户名
func GenerateUsername(strategy string, info *IMUserInfo) string {
	var base string
	switch strategy {
	case StrategyEmail:
		base = info.Email
	case StrategyIMUserID:
		base = info.UserID
	case StrategyMobile:
		base = info.Mobile
	case StrategyPinyin:
		base = nameToPinyin(info.Name)
	case StrategyEmailPrefix:
		base = emailPrefix(info.Email)
	default:
		base = emailPrefix(info.Email)
	}

	if base == "" {
		base = smartFallback(info)
	}

	return ensureUnique(base)
}

func emailPrefix(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.SplitN(email, "@", 2)
	return strings.TrimSpace(parts[0])
}

func smartFallback(info *IMUserInfo) string {
	if prefix := emailPrefix(info.Email); prefix != "" {
		return prefix
	}
	if info.Mobile != "" {
		return info.Mobile
	}
	if py := nameToPinyin(info.Name); py != "" && py != "user" {
		return py
	}
	return "user"
}

func nameToPinyin(name string) string {
	a := pinyin.NewArgs()
	a.Style = pinyin.Normal
	a.Fallback = func(r rune, a pinyin.Args) []string {
		return []string{string(r)}
	}

	result := pinyin.Pinyin(name, a)
	parts := make([]string, 0, len(result))
	for _, p := range result {
		if len(p) > 0 {
			parts = append(parts, p[0])
		}
	}

	py := strings.Join(parts, "")
	var clean strings.Builder
	for _, c := range py {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			clean.WriteRune(c)
		}
	}
	result2 := strings.ToLower(clean.String())
	if result2 == "" {
		return "user"
	}
	return result2
}

func ensureUnique(base string) string {
	candidate := base
	counter := 2

	for {
		var count int64
		storage.DB.Table("users").Where("username = ? AND is_deleted = 0", candidate).Count(&count)
		if count == 0 {
			return candidate
		}
		candidate = fmt.Sprintf("%s%d", base, counter)
		counter++
	}
}
