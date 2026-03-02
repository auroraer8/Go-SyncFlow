package middleware

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ========== IP 白名单准入 ==========

// ipWhitelistCache 缓存白名单数据，避免每次请求查库
// 逻辑：白名单为空时不做任何限制；白名单有条目时仅允许白名单内IP访问
var (
	ipWhitelistEntries   []ipWhitelistEntry
	ipWhitelistMu        sync.RWMutex
	ipWhitelistLastLoad  time.Time
	ipWhitelistDB        *gorm.DB
	ipWhitelistCacheTime = 30 * time.Second
)

type ipWhitelistEntry struct {
	IPAddress string
	IPType    string // single / cidr
}

// InitIPWhitelist 初始化白名单（在 main 中调用，传入 DB）
func InitIPWhitelist(db *gorm.DB) {
	ipWhitelistDB = db
	refreshIPWhitelist()
}

// GetIPWhitelistCount 返回当前白名单条目数量（供前端状态展示）
func GetIPWhitelistCount() int {
	ipWhitelistMu.RLock()
	defer ipWhitelistMu.RUnlock()
	return len(ipWhitelistEntries)
}

func refreshIPWhitelist() {
	if ipWhitelistDB == nil {
		return
	}
	ipWhitelistMu.Lock()
	defer ipWhitelistMu.Unlock()

	// 从 ip_whitelists 读取条目
	type wlRow struct {
		IPAddress string
		IPType    string
	}
	var rows []wlRow
	ipWhitelistDB.Table("ip_whitelists").
		Where("is_active = ?", true).
		Select("ip_address, ip_type").Find(&rows)

	entries := make([]ipWhitelistEntry, 0, len(rows))
	for _, r := range rows {
		entries = append(entries, ipWhitelistEntry{IPAddress: r.IPAddress, IPType: r.IPType})
	}
	ipWhitelistEntries = entries
	ipWhitelistLastLoad = time.Now()
}

// ForceRefreshIPWhitelist 强制刷新白名单缓存（增删白名单后调用）
func ForceRefreshIPWhitelist() {
	refreshIPWhitelist()
}

func isIPInWhitelist(ip string) bool {
	ipWhitelistMu.RLock()
	entries := ipWhitelistEntries
	ipWhitelistMu.RUnlock()

	parsedIP := net.ParseIP(ip)
	for _, e := range entries {
		if e.IPType == "cidr" {
			_, ipNet, err := net.ParseCIDR(e.IPAddress)
			if err == nil && parsedIP != nil && ipNet.Contains(parsedIP) {
				return true
			}
		} else {
			if e.IPAddress == ip {
				return true
			}
		}
	}
	return false
}

// IPWhitelistMiddleware 全局IP白名单准入中间件
// 白名单为空 → 不做限制（放行所有IP）
// 白名单有条目 → 仅允许白名单内IP访问
func IPWhitelistMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 定时刷新缓存
		ipWhitelistMu.RLock()
		needRefresh := time.Since(ipWhitelistLastLoad) > ipWhitelistCacheTime
		isEmpty := len(ipWhitelistEntries) == 0
		ipWhitelistMu.RUnlock()

		if needRefresh {
			refreshIPWhitelist()
			ipWhitelistMu.RLock()
			isEmpty = len(ipWhitelistEntries) == 0
			ipWhitelistMu.RUnlock()
		}

		// 白名单为空时放行所有请求
		if isEmpty {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		if !isIPInWhitelist(clientIP) {
			log.Printf("[安全] IP %s 不在白名单中，拒绝访问", clientIP)
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "访问被拒绝：您的IP不在允许列表中",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimiter 基于内存的限流器
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string]*requestCounter
	limit    int           // 限制次数
	window   time.Duration // 时间窗口
}

type requestCounter struct {
	count     int
	resetTime time.Time
}

// NewRateLimiter 创建限流器
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*requestCounter),
		limit:    limit,
		window:   window,
	}
	// 定期清理过期记录
	go rl.cleanup()
	return rl
}

// cleanup 定期清理过期的请求记录
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, counter := range rl.requests {
			if now.After(counter.resetTime) {
				delete(rl.requests, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	counter, exists := rl.requests[key]

	if !exists || now.After(counter.resetTime) {
		rl.requests[key] = &requestCounter{
			count:     1,
			resetTime: now.Add(rl.window),
		}
		return true
	}

	if counter.count >= rl.limit {
		return false
	}

	counter.count++
	return true
}

// RemainingRequests 获取剩余请求数
func (rl *RateLimiter) RemainingRequests(key string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	counter, exists := rl.requests[key]
	if !exists {
		return rl.limit
	}

	if time.Now().After(counter.resetTime) {
		return rl.limit
	}

	return rl.limit - counter.count
}

// 全局限流器实例
var (
	// 未认证请求限流：每分钟60次（登录页、公开接口）
	unauthLimiter = NewRateLimiter(60, time.Minute)
	// 已认证请求限流：每分钟600次（已登录用户正常操作）
	authLimiter = NewRateLimiter(600, time.Minute)
	// 登录限流：每分钟10次
	loginLimiter = NewRateLimiter(10, time.Minute)
	// 敏感操作限流：每分钟5次
	sensitiveLimiter = NewRateLimiter(5, time.Minute)
)

// RateLimitMiddleware 通用API限流中间件
// 已登录用户享有更高的请求限额
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		// 检查是否携带有效的 Authorization header
		authHeader := c.GetHeader("Authorization")
		isAuthenticated := len(authHeader) > 10 // Bearer token 通常远大于10字符

		if isAuthenticated {
			// 已认证用户：每分钟600次
			key := "auth:" + ip
			if !authLimiter.Allow(key) {
				c.Header("X-RateLimit-Remaining", "0")
				c.Header("Retry-After", "60")
				c.JSON(http.StatusTooManyRequests, gin.H{
					"success": false,
					"message": "请求过于频繁，请稍后再试",
				})
				c.Abort()
				return
			}
		} else {
			// 未认证请求：每分钟60次
			key := "unauth:" + ip
			if !unauthLimiter.Allow(key) {
				c.Header("X-RateLimit-Remaining", "0")
				c.Header("Retry-After", "60")
				c.JSON(http.StatusTooManyRequests, gin.H{
					"success": false,
					"message": "请求过于频繁，请稍后再试",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// LoginRateLimitMiddleware 登录接口限流中间件
func LoginRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "login:" + c.ClientIP()

		if !loginLimiter.Allow(key) {
			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "登录尝试次数过多，请1分钟后再试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// SensitiveRateLimitMiddleware 敏感操作限流中间件
func SensitiveRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "sensitive:" + c.ClientIP()

		if !sensitiveLimiter.Allow(key) {
			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "操作过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
