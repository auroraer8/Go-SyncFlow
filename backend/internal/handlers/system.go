package handlers

import (
	"fmt"
	gonet "net"
	"net/http"
	"runtime"
	"time"

	"go-syncflow/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

var startTime = time.Now()

// SystemStatus 系统状态信息
type SystemStatus struct {
	CPU      CPUInfo      `json:"cpu"`
	Memory   MemoryInfo   `json:"memory"`
	Disk     DiskInfo     `json:"disk"`
	Network  NetworkInfo  `json:"network"`
	Database DatabaseInfo `json:"database"`
	Runtime  RuntimeInfo  `json:"runtime"`
	LDAP     ServiceInfo  `json:"ldap"`
	VPN      ServiceInfo  `json:"vpn"`
}

type ServiceInfo struct {
	Running bool `json:"running"`
	Port    int  `json:"port"`
}

type CPUInfo struct {
	Usage float64 `json:"usage"`
	Cores int     `json:"cores"`
}

type MemoryInfo struct {
	Used    float64 `json:"used"`
	Total   float64 `json:"total"`
	Percent float64 `json:"percent"`
}

type DiskInfo struct {
	Used    float64 `json:"used"`
	Total   float64 `json:"total"`
	Percent float64 `json:"percent"`
}

type NetworkInfo struct {
	BytesRecv uint64  `json:"bytesRecv"`
	BytesSent uint64  `json:"bytesSent"`
	InRate    float64 `json:"inRate"`
	OutRate   float64 `json:"outRate"`
}

type DatabaseInfo struct {
	Status string `json:"status"`
	Type   string `json:"type"`
}

type RuntimeInfo struct {
	Uptime     string `json:"uptime"`
	UptimeSec  int64  `json:"uptimeSec"`
	GoVersion  string `json:"goVersion"`
	Goroutines int    `json:"goroutines"`
	StartTime  string `json:"startTime"`
}

var lastNetworkStats *net.IOCountersStat
var lastNetworkTime time.Time

func GetSystemStatus(c *gin.Context) {
	status := SystemStatus{}

	// CPU
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		status.CPU.Usage = roundTo2(cpuPercent[0])
	}
	status.CPU.Cores = runtime.NumCPU()

	// Memory
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		status.Memory.Used = roundTo2(float64(memInfo.Used) / 1024 / 1024 / 1024)
		status.Memory.Total = roundTo2(float64(memInfo.Total) / 1024 / 1024 / 1024)
		status.Memory.Percent = roundTo2(memInfo.UsedPercent)
	}

	// Disk
	diskInfo, err := disk.Usage("/")
	if err == nil {
		status.Disk.Used = roundTo2(float64(diskInfo.Used) / 1024 / 1024 / 1024)
		status.Disk.Total = roundTo2(float64(diskInfo.Total) / 1024 / 1024 / 1024)
		status.Disk.Percent = roundTo2(diskInfo.UsedPercent)
	}

	// Network
	netStats, err := net.IOCounters(false)
	if err == nil && len(netStats) > 0 {
		currentStats := &netStats[0]
		status.Network.BytesRecv = currentStats.BytesRecv
		status.Network.BytesSent = currentStats.BytesSent

		if lastNetworkStats != nil {
			elapsed := time.Since(lastNetworkTime).Seconds()
			if elapsed > 0 {
				status.Network.InRate = roundTo2(float64(currentStats.BytesRecv-lastNetworkStats.BytesRecv) / elapsed / 1024 / 1024)
				status.Network.OutRate = roundTo2(float64(currentStats.BytesSent-lastNetworkStats.BytesSent) / elapsed / 1024 / 1024)
			}
		}
		lastNetworkStats = currentStats
		lastNetworkTime = time.Now()
	}

	// Database - PostgreSQL
	status.Database.Type = "postgresql"
	// 通过检测数据库连接来判断状态
	if storage.DB != nil {
		sqlDB, err := storage.DB.DB()
		if err == nil && sqlDB.Ping() == nil {
			status.Database.Status = "ok"
		} else {
			status.Database.Status = "error"
		}
	} else {
		status.Database.Status = "error"
	}

	// Runtime
	uptime := time.Since(startTime)
	status.Runtime.UptimeSec = int64(uptime.Seconds())
	status.Runtime.Uptime = formatDuration(uptime)
	status.Runtime.GoVersion = runtime.Version()
	status.Runtime.Goroutines = runtime.NumGoroutine()
	status.Runtime.StartTime = startTime.Format("2006-01-02 15:04:05")

	// LDAP service status
	status.LDAP = GetLDAPServiceStatus()

	// VPN service status
	status.VPN = GetVPNServiceStatus()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// GetLDAPServiceStatus 获取 LDAP 服务状态
func GetLDAPServiceStatus() ServiceInfo {
	info := ServiceInfo{Running: false, Port: 389}
	// 检查 LDAP 服务是否运行 - 通过检查端口是否被监听
	conn, err := gonet.DialTimeout("tcp", "127.0.0.1:389", time.Second)
	if err == nil {
		conn.Close()
		info.Running = true
	}
	return info
}

// GetVPNServiceStatus 获取 VPN 服务状态
func GetVPNServiceStatus() ServiceInfo {
	info := ServiceInfo{Running: false, Port: 443}
	// 检查 VPN 服务是否运行 - 通过检查端口是否被监听
	conn, err := gonet.DialTimeout("tcp", "127.0.0.1:443", time.Second)
	if err == nil {
		conn.Close()
		info.Running = true
	}
	return info
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d天%d小时%d分钟", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes)
	}
	return fmt.Sprintf("%d分钟", minutes)
}
