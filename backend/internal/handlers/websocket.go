package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
	syncer "go-syncflow/internal/sync"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WebSocket 客户端管理
type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

type wsHub struct {
	clients    map[*wsClient]bool
	broadcast  chan []byte
	register   chan *wsClient
	unregister chan *wsClient
	mu         sync.RWMutex
}

var dashboardHub = &wsHub{
	clients:    make(map[*wsClient]bool),
	broadcast:  make(chan []byte, 256),
	register:   make(chan *wsClient),
	unregister: make(chan *wsClient),
}

func init() {
	// 注入 WebSocket 广播函数到 sync 包
	syncer.BroadcastSyncLogFunc = func(logData interface{}) {
		BroadcastEvent("sync_log", logData)
	}
	syncer.BroadcastSyncEventFunc = func(action, syncType, connectorName string, details interface{}) {
		BroadcastSyncEvent(action, syncType, connectorName, details)
	}
	go dashboardHub.run()
	go startRealtimeBroadcast()
}

func (h *wsHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[WebSocket] 客户端连接，当前连接数: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[WebSocket] 客户端断开，当前连接数: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// RealtimeData 实时数据结构
type RealtimeData struct {
	Type      string      `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPU struct {
		Usage float64 `json:"usage"`
		Cores int     `json:"cores"`
	} `json:"cpu"`
	Memory struct {
		Total   float64 `json:"total"`
		Used    float64 `json:"used"`
		Percent float64 `json:"percent"`
	} `json:"memory"`
	Disk struct {
		Total   float64 `json:"total"`
		Used    float64 `json:"used"`
		Percent float64 `json:"percent"`
	} `json:"disk"`
	Network struct {
		InRate  float64 `json:"inRate"`
		OutRate float64 `json:"outRate"`
	} `json:"network"`
}

// 网络流量追踪
var (
	lastNetIn   uint64
	lastNetOut  uint64
	lastNetTime time.Time
)

// 收集系统指标
func collectSystemMetrics() SystemMetrics {
	var metrics SystemMetrics

	// CPU
	cpuPercent, _ := cpu.Percent(0, false)
	if len(cpuPercent) > 0 {
		metrics.CPU.Usage = cpuPercent[0]
	}
	cpuInfo, _ := cpu.Info()
	metrics.CPU.Cores = len(cpuInfo)
	if metrics.CPU.Cores == 0 {
		counts, _ := cpu.Counts(true)
		metrics.CPU.Cores = counts
	}

	// Memory
	memInfo, _ := mem.VirtualMemory()
	if memInfo != nil {
		metrics.Memory.Total = float64(memInfo.Total) / 1024 / 1024 / 1024
		metrics.Memory.Used = float64(memInfo.Used) / 1024 / 1024 / 1024
		metrics.Memory.Percent = memInfo.UsedPercent
	}

	// Disk
	diskInfo, _ := disk.Usage("/")
	if diskInfo != nil {
		metrics.Disk.Total = float64(diskInfo.Total) / 1024 / 1024 / 1024
		metrics.Disk.Used = float64(diskInfo.Used) / 1024 / 1024 / 1024
		metrics.Disk.Percent = diskInfo.UsedPercent
	}

	// Network
	netIO, _ := net.IOCounters(false)
	if len(netIO) > 0 {
		now := time.Now()
		if !lastNetTime.IsZero() {
			elapsed := now.Sub(lastNetTime).Seconds()
			if elapsed > 0 {
				metrics.Network.InRate = float64(netIO[0].BytesRecv-lastNetIn) / elapsed / 1024 / 1024
				metrics.Network.OutRate = float64(netIO[0].BytesSent-lastNetOut) / elapsed / 1024 / 1024
			}
		}
		lastNetIn = netIO[0].BytesRecv
		lastNetOut = netIO[0].BytesSent
		lastNetTime = now
	}

	return metrics
}

// 收集业务统计
func collectBusinessStats() DashboardStats {
	stats := DashboardStats{
		Timestamp: time.Now().Unix(),
	}

	// 并行查询
	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		storage.DB.Model(&models.User{}).Where("is_deleted = 0").Count(&stats.Users.Total)
		storage.DB.Model(&models.User{}).Where("is_deleted = 0 AND status = 1").Count(&stats.Users.Active)
		storage.DB.Model(&models.User{}).Where("is_deleted = 0 AND status = 0").Count(&stats.Users.Disabled)
		today := time.Now().Format("2006-01-02")
		storage.DB.Model(&models.User{}).Where("is_deleted = 0 AND DATE(created_at) = ?", today).Count(&stats.Users.Today)
	}()

	go func() {
		defer wg.Done()
		storage.DB.Model(&models.Connector{}).Where("status = 1").Count(&stats.Connectors.Online)
		storage.DB.Model(&models.Connector{}).Count(&stats.Connectors.Total)
		storage.DB.Model(&models.Connector{}).Where("direction = 'upstream' OR direction = 'both'").Count(&stats.Connectors.Upstream)
		storage.DB.Model(&models.Connector{}).Where("direction = 'downstream' OR direction = 'both'").Count(&stats.Connectors.Downstream)
	}()

	go func() {
		defer wg.Done()
		storage.DB.Model(&models.Role{}).Count(&stats.Roles.Total)
		storage.DB.Model(&models.RolePermission{}).Count(&stats.Roles.Permissions)
		storage.DB.Model(&models.UserRole{}).Count(&stats.Roles.Assignments)
	}()

	go func() {
		defer wg.Done()
		storage.DB.Model(&models.SSOApp{}).Count(&stats.SSOApps.Total)
		storage.DB.Model(&models.SSOApp{}).Where("enabled = ?", true).Count(&stats.SSOApps.Enabled)
	}()

	go func() {
		defer wg.Done()
		storage.DB.Model(&models.Synchronizer{}).Count(&stats.Sync.Total)
		storage.DB.Model(&models.Synchronizer{}).Where("status = ?", 1).Count(&stats.Sync.Enabled)
		var lastLog models.SyncLog
		if storage.DB.Order("created_at DESC").First(&lastLog).Error == nil {
			stats.Sync.LastSync = lastLog.CreatedAt.Format("01-02 15:04")
		} else {
			stats.Sync.LastSync = "-"
		}
	}()

	wg.Wait()
	return stats
}

// 启动实时广播
func startRealtimeBroadcast() {
	// 系统指标 - 每 2 秒推送
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			dashboardHub.mu.RLock()
			clientCount := len(dashboardHub.clients)
			dashboardHub.mu.RUnlock()

			if clientCount == 0 {
				continue
			}

			metrics := collectSystemMetrics()
			data := RealtimeData{
				Type:      "system_metrics",
				Timestamp: time.Now().Unix(),
				Data:      metrics,
			}
			msg, _ := json.Marshal(data)
			dashboardHub.broadcast <- msg
		}
	}()

	// 业务统计 - 每 10 秒推送
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			dashboardHub.mu.RLock()
			clientCount := len(dashboardHub.clients)
			dashboardHub.mu.RUnlock()

			if clientCount == 0 {
				continue
			}

			stats := collectBusinessStats()
			data := RealtimeData{
				Type:      "business_stats",
				Timestamp: time.Now().Unix(),
				Data:      stats,
			}
			msg, _ := json.Marshal(data)
			dashboardHub.broadcast <- msg
		}
	}()
}

// DashboardWebSocket WebSocket 连接处理
func DashboardWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WebSocket] 升级失败: %v", err)
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan []byte, 256),
	}
	dashboardHub.register <- client

	// 立即发送一次完整数据
	go func() {
		metrics := collectSystemMetrics()
		metricsData := RealtimeData{
			Type:      "system_metrics",
			Timestamp: time.Now().Unix(),
			Data:      metrics,
		}
		msg, _ := json.Marshal(metricsData)
		client.send <- msg

		stats := collectBusinessStats()
		statsData := RealtimeData{
			Type:      "business_stats",
			Timestamp: time.Now().Unix(),
			Data:      stats,
		}
		msg2, _ := json.Marshal(statsData)
		client.send <- msg2
	}()

	// 写 goroutine
	go func() {
		defer func() {
			conn.Close()
		}()
		for msg := range client.send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// 读 goroutine（保持连接活跃）
	go func() {
		defer func() {
			dashboardHub.unregister <- client
			conn.Close()
		}()
		conn.SetReadLimit(512)
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()

	// Ping 保活
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()
}

// BroadcastEvent 广播事件（供其他模块调用）
// eventType: system_metrics, business_stats, user_change, group_change, role_change, sync_event, sync_log, connector_change, sso_change, notification
func BroadcastEvent(eventType string, data interface{}) {
	msg := RealtimeData{
		Type:      eventType,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}
	msgBytes, _ := json.Marshal(msg)
	
	// 非阻塞发送
	select {
	case dashboardHub.broadcast <- msgBytes:
	default:
		log.Printf("[WebSocket] 广播队列已满，丢弃事件: %s", eventType)
	}
}

// BroadcastUserChange 广播用户变更事件
func BroadcastUserChange(action string, userID uint, username string) {
	BroadcastEvent("user_change", map[string]interface{}{
		"action":   action, // created, updated, deleted, enabled, disabled
		"userId":   userID,
		"username": username,
	})
}

// BroadcastGroupChange 广播群组变更事件
func BroadcastGroupChange(action string, groupID uint, groupName string) {
	BroadcastEvent("group_change", map[string]interface{}{
		"action":    action, // created, updated, deleted
		"groupId":   groupID,
		"groupName": groupName,
	})
}

// BroadcastRoleChange 广播角色变更事件
func BroadcastRoleChange(action string, roleID uint, roleName string) {
	BroadcastEvent("role_change", map[string]interface{}{
		"action":   action,
		"roleId":   roleID,
		"roleName": roleName,
	})
}

// BroadcastSyncEvent 广播同步事件
func BroadcastSyncEvent(action string, syncType string, connectorName string, details interface{}) {
	BroadcastEvent("sync_event", map[string]interface{}{
		"action":        action, // started, progress, completed, failed
		"syncType":      syncType, // upstream, downstream, full
		"connectorName": connectorName,
		"details":       details,
	})
}

// BroadcastSyncLog 广播新同步日志
func BroadcastSyncLog(log interface{}) {
	BroadcastEvent("sync_log", log)
}

// BroadcastConnectorChange 广播连接器变更事件
func BroadcastConnectorChange(action string, connectorID uint, connectorName string) {
	BroadcastEvent("connector_change", map[string]interface{}{
		"action":        action,
		"connectorId":   connectorID,
		"connectorName": connectorName,
	})
}

// BroadcastNotification 广播系统通知
func BroadcastNotification(level string, title string, message string) {
	BroadcastEvent("notification", map[string]interface{}{
		"level":   level, // info, success, warning, error
		"title":   title,
		"message": message,
	})
}
