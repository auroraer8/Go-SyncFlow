package vpn

import (
	"errors"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"go-syncflow/internal/vpn/base"
	"go-syncflow/internal/vpn/handler"
	"go-syncflow/internal/vpn/sessdata"
)

var (
	running   bool
	startedAt time.Time
	serverMux sync.Mutex
)

// ServiceStatus 服务状态
type ServiceStatus struct {
	Running     bool      `json:"running"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	ListenAddr  string    `json:"listen_addr"`
	DTLSEnabled bool      `json:"dtls_enabled"`
	OnlineCount int       `json:"online_count"`
	LinkMode    string    `json:"link_mode"`
	IPv4CIDR    string    `json:"ipv4_cidr"`
}

// Start 启动 VPN 服务
func Start() error {
	serverMux.Lock()
	defer serverMux.Unlock()

	if running {
		return errors.New("VPN 服务已在运行")
	}

	// 调用 VPN handler 的启动函数
	handler.Start()

	running = true
	startedAt = time.Now()

	log.Printf("[VPN] 服务已启动 - 监听 %s, 模式: %s, 网段: %s",
		base.Cfg.ServerAddr, base.Cfg.LinkMode, base.Cfg.Ipv4CIDR)

	return nil
}

// Stop 停止 VPN 服务
func Stop() error {
	serverMux.Lock()
	defer serverMux.Unlock()

	if !running {
		return nil
	}

	handler.Stop()

	running = false
	log.Println("[VPN] 服务已停止")

	return nil
}

// Status 获取服务状态
func Status() ServiceStatus {
	serverMux.Lock()
	defer serverMux.Unlock()

	status := ServiceStatus{
		Running: running,
	}

	if running {
		status.StartedAt = startedAt
		status.ListenAddr = resolveListenAddr(base.Cfg.ServerAddr)
		status.DTLSEnabled = base.Cfg.ServerDTLS
		status.LinkMode = base.Cfg.LinkMode
		status.IPv4CIDR = base.Cfg.Ipv4CIDR
		status.OnlineCount = len(sessdata.OnlineSess())
	}

	return status
}

// resolveListenAddr 将 ":443" 格式转为 "IP:443"
func resolveListenAddr(addr string) string {
	if !strings.HasPrefix(addr, ":") {
		return addr
	}
	port := addr[1:]
	if ip := getOutboundIP(); ip != "" {
		return ip + ":" + port
	}
	return "0.0.0.0:" + port
}

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// IsRunning 检查服务是否运行
func IsRunning() bool {
	serverMux.Lock()
	defer serverMux.Unlock()
	return running
}

// Restart 重启服务
func Restart() error {
	if err := Stop(); err != nil {
		return err
	}
	time.Sleep(time.Second)
	return Start()
}
