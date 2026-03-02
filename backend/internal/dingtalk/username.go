package dingtalk

import (
	"fmt"
	"strings"

	"github.com/mozillazg/go-pinyin"

	"go-syncflow/internal/storage"
)

// 用户名生成策略
const (
	StrategyEmailPrefix   = "email_prefix"
	StrategyEmail         = "email"
	StrategyDingTalkUID   = "dingtalk_userid"
	StrategyMobile        = "mobile"
	StrategyPinyin        = "pinyin"
)

// emailPrefix 提取邮箱 @ 前面的部分，为空则返回 ""
func emailPrefix(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.SplitN(email, "@", 2)
	prefix := strings.TrimSpace(parts[0])
	return prefix
}

// smartFallback 按优先级依次尝试：邮箱前缀 → 手机号 → 姓名拼音
// 确保在任何情况下都能生成有意义的用户名，不会退化为钉钉 UserID
func smartFallback(info *UserInfo) string {
	// 1. 邮箱前缀
	if prefix := emailPrefix(info.Email); prefix != "" {
		return prefix
	}
	// 2. 手机号
	if info.Mobile != "" {
		return info.Mobile
	}
	// 3. 姓名拼音
	if py := nameToPinyin(info.Name); py != "" && py != "user" {
		return py
	}
	// 4. 最终兜底
	return "user"
}

// GenerateUsername 根据策略生成用户名
// 所有策略在首选字段为空时，都会自动按 邮箱前缀 → 手机号 → 拼音 顺序回退
func GenerateUsername(strategy string, info *UserInfo) string {
	var base string
	switch strategy {
	case StrategyEmail:
		base = info.Email
	case StrategyDingTalkUID:
		base = info.UserID
	case StrategyMobile:
		base = info.Mobile
	case StrategyPinyin:
		base = nameToPinyin(info.Name)
	case StrategyEmailPrefix:
		fallthrough
	default:
		base = emailPrefix(info.Email)
	}

	// 如果首选策略未能产生有效值，启用智能回退
	if base == "" {
		base = smartFallback(info)
	}

	// 确保用户名唯一
	return ensureUnique(base)
}

// nameToPinyin 将中文名转为拼音
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
	// 移除非字母数字字符
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

// ensureUnique 确保用户名唯一，重名时自动追加数字
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
