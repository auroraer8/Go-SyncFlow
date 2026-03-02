package handler

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

const (
	globalDownloadRate = 20               // 全局令牌桶：每分钟允许的下载请求数
	bandwidthPerConn   = 2 * 1024 * 1024  // 每连接限速 2MB/s
	burstSize          = 64 * 1024        // 令牌桶突发大小 64KB
	maxGlobalDownloads = 30               // 全局最大并发下载数
	maxPerIPDownloads  = 5                // 单IP最大并发下载数
)

// ---- 全局令牌桶限流 ----

var globalLimiter = rate.NewLimiter(rate.Limit(float64(globalDownloadRate)/60.0), 5)

func downloadRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !globalLimiter.Allow() {
			w.Header().Set("Retry-After", "30")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			log.Printf("[下载防护] 全局限流触发, IP=%s, Path=%s", extractIP(r), r.URL.Path)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- 并发下载控制 ----

var (
	globalSem     = make(chan struct{}, maxGlobalDownloads)
	perIPCount    sync.Map // map[string]*int32
)

func getIPCounter(ip string) *int32 {
	val, _ := perIPCount.LoadOrStore(ip, new(int32))
	return val.(*int32)
}

func concurrencyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		counter := getIPCounter(ip)

		if int(atomic.LoadInt32(counter)) >= maxPerIPDownloads {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			log.Printf("[下载防护] 单IP并发超限, IP=%s (%d/%d)", ip, atomic.LoadInt32(counter), maxPerIPDownloads)
			return
		}

		select {
		case globalSem <- struct{}{}:
		default:
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			log.Printf("[下载防护] 全局并发超限 (%d/%d), IP=%s", maxGlobalDownloads, maxGlobalDownloads, ip)
			return
		}

		atomic.AddInt32(counter, 1)
		defer func() {
			<-globalSem
			atomic.AddInt32(counter, -1)
		}()

		next.ServeHTTP(w, r)
	})
}

// ---- 带宽限速 ----

type throttledResponseWriter struct {
	http.ResponseWriter
	limiter *rate.Limiter
}

func (t *throttledResponseWriter) Write(p []byte) (int, error) {
	totalWritten := 0
	for len(p) > 0 {
		chunk := len(p)
		burst := t.limiter.Burst()
		if chunk > burst {
			chunk = burst
		}
		if err := t.limiter.WaitN(context.Background(), chunk); err != nil {
			return totalWritten, err
		}
		n, err := t.ResponseWriter.Write(p[:chunk])
		totalWritten += n
		if err != nil {
			return totalWritten, err
		}
		p = p[n:]
	}
	return totalWritten, nil
}

func bandwidthLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limiter := rate.NewLimiter(rate.Limit(bandwidthPerConn), burstSize)
		tw := &throttledResponseWriter{
			ResponseWriter: w,
			limiter:        limiter,
		}
		next.ServeHTTP(tw, r)
	})
}

// ---- 禁止目录列表 ----

type noDirFileSystem struct {
	fs http.FileSystem
}

func (n noDirFileSystem) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	if info.IsDir() {
		indexPath := filepath.Join(name, "index.html")
		if _, err := n.fs.Open(indexPath); err != nil {
			f.Close()
			return nil, os.ErrPermission
		}
	}

	return f, nil
}

// ---- 下载审计日志 ----

func downloadLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lw, r)
		log.Printf("[下载日志] IP=%s, Path=%s, Status=%d, Duration=%v, UA=%s",
			extractIP(r), r.URL.Path, lw.statusCode, time.Since(start).Round(time.Millisecond), truncateUA(r.UserAgent()))
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.statusCode = code
	lw.ResponseWriter.WriteHeader(code)
}

func (lw *loggingResponseWriter) Write(b []byte) (int, error) {
	return lw.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController access the underlying ResponseWriter
func (lw *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return lw.ResponseWriter
}

// ---- 工具函数 ----

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func truncateUA(ua string) string {
	if len(ua) > 100 {
		return ua[:100] + "..."
	}
	return ua
}

// GuardedFileServer 构建带完整防护的文件服务 Handler
func GuardedFileServer(root string) http.Handler {
	fs := noDirFileSystem{fs: http.Dir(root)}
	fileServer := http.FileServer(fs)

	var handler http.Handler = fileServer
	handler = bandwidthLimitMiddleware(handler)
	handler = downloadLogMiddleware(handler)
	handler = concurrencyLimitMiddleware(handler)
	handler = downloadRateLimitMiddleware(handler)

	return handler
}

// io.Writer interface assertion
var _ io.Writer = (*throttledResponseWriter)(nil)
