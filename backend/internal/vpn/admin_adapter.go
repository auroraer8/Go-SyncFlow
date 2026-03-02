package vpn

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
	"go-syncflow/internal/vpn/admin"
	"go-syncflow/internal/vpn/base"
	"go-syncflow/internal/vpn/dbdata"
	"go-syncflow/internal/vpn/sessdata"
)

// wrapHandler 将 net/http handler 包装为 gin handler
func wrapHandler(h func(http.ResponseWriter, *http.Request)) gin.HandlerFunc {
	return func(c *gin.Context) {
		h(c.Writer, c.Request)
	}
}

// RegisterAdminRoutes 注册 VPN 管理路由
func RegisterAdminRoutes(auth *gin.RouterGroup) {
	vpn := auth.Group("/vpn")

	// ========== 服务控制 ==========
	vpn.GET("/service/status", GetServiceStatus)
	vpn.POST("/service/start", StartService)
	vpn.POST("/service/stop", StopService)
	vpn.POST("/service/restart", RestartService)

	// ========== 仪表盘/首页 ==========
	vpn.GET("/dashboard", handleDashboard)

	// ========== 系统信息 ==========
	vpn.GET("/system", wrapHandler(admin.SetSystem))
	vpn.GET("/soft", wrapHandler(admin.SetSoft))

	// ========== 设置 ==========
	vpn.GET("/settings", handleGetSettings)
	vpn.PUT("/settings", handleUpdateSettings)
	vpn.POST("/settings/test", handleTestSettings)
	vpn.GET("/settings/interfaces", handleGetNetworkInterfaces)
	vpn.GET("/settings/group-options", handleGetGroupOptions)

	// ========== 使用范围选项 (本地用户/部门列表) ==========
	vpn.GET("/scope/users", handleGetScopeUsers)
	vpn.GET("/scope/departments", handleGetScopeDepartments)

	// ========== 上游连接器列表（用于 VPN 认证配置）==========
	vpn.GET("/ldap-connectors", handleGetLdapConnectors)         // 兼容旧版
	vpn.GET("/auth-connectors", handleGetAuthConnectors)         // 新版：获取所有可认证连接器

	// ========== 地址组管理 (RESTful) ==========
	vpn.GET("/address-groups", handleAddressGroupList)
	vpn.POST("/address-groups", handleAddressGroupCreate)
	vpn.GET("/address-groups/names", handleAddressGroupNames)
	vpn.GET("/address-groups/:id", handleAddressGroupDetail)
	vpn.PUT("/address-groups/:id", handleAddressGroupUpdate)
	vpn.DELETE("/address-groups/:id", handleAddressGroupDelete)

	// ========== 用户组管理 (RESTful 适配) ==========
	vpn.GET("/groups", handleGroupList)
	vpn.POST("/groups", handleGroupCreate)
	vpn.GET("/groups/names", wrapHandler(admin.GroupNames))
	vpn.GET("/groups/:id", handleGroupDetail)
	vpn.PUT("/groups/:id", handleGroupUpdate)
	vpn.DELETE("/groups/:id", handleGroupDelete)

	// ========== VPN 用户管理 (RESTful 适配) ==========
	vpn.GET("/users", wrapHandler(admin.UserList))
	vpn.GET("/users/:id", handleUserDetail)
	vpn.PUT("/users/:id", handleUserUpdate)
	vpn.POST("/users/:id/reset-pin", handleUserResetPin)
	vpn.GET("/users/:id/otp-qr", handleUserOTPQR)
	vpn.POST("/users/upload", wrapHandler(admin.UserUpload))

	// ========== 在线用户 ==========
	vpn.GET("/online", handleOnlineUsers)
	vpn.GET("/online/stats", handleOnlineStats)
	vpn.POST("/online/:token/kick", handleKickUser)

	// ========== IP 映射 (RESTful 适配) ==========
	vpn.GET("/ip-maps", wrapHandler(admin.UserIpMapList))
	vpn.POST("/ip-maps", handleIPMapCreate)
	vpn.PUT("/ip-maps/:id", handleIPMapUpdate)
	vpn.DELETE("/ip-maps/:id", handleIPMapDelete)

	// ========== 访问策略 ==========
	vpn.GET("/policies", wrapHandler(admin.PolicyList))
	vpn.GET("/policies/detail", wrapHandler(admin.PolicyDetail))
	vpn.POST("/policies/set", wrapHandler(admin.PolicySet))
	vpn.POST("/policies/del", wrapHandler(admin.PolicyDel))

	// ========== 审计日志 ==========
	vpn.GET("/audit", handleAuditList)
	vpn.GET("/audit/export", wrapHandler(admin.SetAuditExport))
	vpn.GET("/user-logs", handleUserActLogList)

	// ========== 统计信息 ==========
	vpn.GET("/stats", wrapHandler(admin.StatsInfoList))

	// ========== 防爆破信息 ==========
	vpn.GET("/locks", wrapHandler(admin.GetLocksInfo))
	vpn.POST("/locks/unlock", wrapHandler(admin.UnlockUser))

	// ========== 其他设置 ==========
	vpn.GET("/settings/smtp", wrapHandler(admin.SetOtherSmtp))
	vpn.POST("/settings/smtp/edit", wrapHandler(admin.SetOtherSmtpEdit))
	vpn.GET("/settings/audit_log", wrapHandler(admin.SetOtherAuditLog))
	vpn.POST("/settings/audit_log/edit", wrapHandler(admin.SetOtherAuditLogEdit))
	vpn.GET("/settings/sms", wrapHandler(admin.SetOtherSms))
	vpn.POST("/settings/sms/edit", wrapHandler(admin.SetOtherSmsEdit))
	vpn.POST("/settings/sms/test", wrapHandler(admin.SetOtherSmsTest))

	// ========== 证书管理 ==========
	vpn.GET("/cert/setting", wrapHandler(admin.GetCertSetting))
	vpn.POST("/cert/create", wrapHandler(admin.CreatCert))
	vpn.POST("/cert/custom", wrapHandler(admin.CustomCert))
}

// ========== 用户组 RESTful 适配器 ==========

func handleGroupList(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	pageSize := dbdata.PageSize
	count := dbdata.CountAll(&dbdata.Group{})

	var datas []dbdata.Group
	err := dbdata.Find(&datas, pageSize, page)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"list":      datas,
			"total":     count,
			"page_size": pageSize,
		},
	})
}

func handleGroupDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 ID"})
		return
	}

	var data dbdata.Group
	err = dbdata.One("Id", id, &data)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户组不存在"})
		return
	}

	if len(data.Auth) == 0 {
		data.Auth = make(map[string]interface{})
		data.Auth["type"] = "local"
	}
	if data.SplitDns == nil {
		data.SplitDns = []dbdata.ValData{}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func handleGroupCreate(c *gin.Context) {
	var group dbdata.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	group.Id = 0 // 确保是新建
	if err := dbdata.SetGroup(&group); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "创建成功", "data": group})
}

func handleGroupUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 ID"})
		return
	}

	var group dbdata.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	group.Id = id
	if err := dbdata.SetGroup(&group); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "更新成功"})
}

func handleGroupDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 ID"})
		return
	}

	data := dbdata.Group{Id: id}
	if err := dbdata.Del(&data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "删除成功"})
}

// ========== VPN 用户 RESTful 适配器 ==========

func handleUserDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 ID"})
		return
	}

	var data dbdata.User
	err = dbdata.One("Id", id, &data)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func handleUserUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 ID"})
		return
	}

	var user dbdata.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	user.Id = id
	if err := dbdata.SetUser(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "更新成功"})
}

func handleUserResetPin(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 ID"})
		return
	}

	var user dbdata.User
	err = dbdata.One("Id", id, &user)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	user.PinCode = ""
	user.OtpSecret = ""
	if err := dbdata.SetUser(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "PIN 码已重置"})
}

func handleUserOTPQR(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 ID"})
		return
	}

	// 直接调用原 admin handler（通过 query 参数）
	c.Request.URL.RawQuery = "id=" + idStr
	admin.UserOtpQr(c.Writer, c.Request)
}

// ========== 在线用户适配器 ==========

func handleOnlineStats(c *gin.Context) {
	onlineUsers := sessdata.OnlineSess()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"online_count": len(onlineUsers),
		},
	})
}

func handleKickUser(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 token"})
		return
	}

	// 调用原 admin handler
	c.Request.URL.RawQuery = "token=" + token
	admin.UserOffline(c.Writer, c.Request)
}

// ========== IP 映射适配器 ==========

func handleIPMapCreate(c *gin.Context) {
	var ipMap dbdata.IpMap
	if err := c.ShouldBindJSON(&ipMap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	ipMap.Id = 0
	if err := dbdata.SetIpMap(&ipMap); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "创建成功", "data": ipMap})
}

func handleIPMapUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 ID"})
		return
	}

	var ipMap dbdata.IpMap
	if err := c.ShouldBindJSON(&ipMap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	ipMap.Id = id
	if err := dbdata.SetIpMap(&ipMap); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "更新成功"})
}

func handleIPMapDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 ID"})
		return
	}

	data := dbdata.IpMap{Id: id}
	if err := dbdata.Del(&data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "删除成功"})
}

// ========== 仪表盘适配器 ==========

func handleDashboard(c *gin.Context) {
	// 获取统计数据
	groupCount := dbdata.CountAll(&dbdata.Group{})
	userCount := dbdata.CountAll(&dbdata.User{})
	ipMapCount := dbdata.CountAll(&dbdata.IpMap{})
	onlineUsers := sessdata.OnlineSess()

	// 组装统计数据
	groupStats := make(map[string]int)
	for _, sess := range onlineUsers {
		groupStats[sess.Group]++
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"online_count":   len(onlineUsers),
			"group_count":    groupCount,
			"vpn_user_count": userCount,
			"ip_map_count":   ipMapCount,
			"today_connect":  0,
			"total_connect":  0,
			"group_stats":    groupStats,
			"bandwidth": gin.H{
				"upload":   0,
				"download": 0,
			},
		},
	})
}

// ========== 设置相关适配器 ==========

func handleTestSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "设置测试成功"})
}

type NetworkInterface struct {
	Name      string   `json:"name"`
	IPAddrs   []string `json:"ip_addrs"`
	IsDefault bool     `json:"is_default"`
}

func handleGetNetworkInterfaces(c *gin.Context) {
	interfaces, err := net.Interfaces()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	var result []NetworkInterface
	var defaultInterface string

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// 跳过常见虚拟网卡
		name := strings.ToLower(iface.Name)
		if strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "br-") ||
			strings.HasPrefix(name, "veth") || strings.HasPrefix(name, "virbr") ||
			strings.HasPrefix(name, "lo") || strings.HasPrefix(name, "tun") ||
			strings.HasPrefix(name, "tap") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		var ipAddrs []string
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				ipAddrs = append(ipAddrs, ipnet.IP.String())
				// 记录第一个有效网卡作为默认
				if defaultInterface == "" {
					defaultInterface = iface.Name
				}
			}
		}

		if len(ipAddrs) > 0 {
			result = append(result, NetworkInterface{
				Name:      iface.Name,
				IPAddrs:   ipAddrs,
				IsDefault: false,
			})
		}
	}

	// 标记默认网卡
	for i := range result {
		if result[i].Name == defaultInterface {
			result[i].IsDefault = true
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"data":              result,
		"default_interface": defaultInterface,
	})
}

func handleGetGroupOptions(c *gin.Context) {
	names := dbdata.GetGroupNamesIds()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    names,
	})
}

func handleGetSettings(c *gin.Context) {
	// 从数据库获取 SettingOther
	var other dbdata.SettingOther
	dbdata.SettingGet(&other)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":          true,
			"server_addr":      base.Cfg.ServerAddr,
			"server_dtls":      base.Cfg.ServerDTLS,
			"server_dtls_addr": base.Cfg.ServerDTLSAddr,
			"link_mode":        base.Cfg.LinkMode,
			"ipv4_master":      base.Cfg.Ipv4Master,
			"ipv4_cidr":        base.Cfg.Ipv4CIDR,
			"ipv4_gateway":     base.Cfg.Ipv4Gateway,
			"ipv4_start":       base.Cfg.Ipv4Start,
			"ipv4_end":         base.Cfg.Ipv4End,
			"max_client":       base.Cfg.MaxClient,
			"max_user_client":  base.Cfg.MaxUserClient,
			"mtu":              base.Cfg.Mtu,
			"session_timeout":  base.Cfg.SessionTimeout,
			"idle_timeout":     base.Cfg.IdleTimeout,
			"iptables_nat":     base.Cfg.IptablesNat,
			"issuer":           base.Cfg.Issuer,
			"default_group":    base.Cfg.DefaultGroup,
			// SettingOther 中的显示配置
			"banner":          other.Banner,
			"login_page_title": other.Homeindex,
			"login_page_html":  other.AccountMail,
			"link_addr":        other.LinkAddr,
		},
	})
}

func handleUpdateSettings(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	// 更新配置到 base.Cfg
	if v, ok := req["server_addr"].(string); ok {
		base.Cfg.ServerAddr = v
	}
	if v, ok := req["server_dtls"].(bool); ok {
		base.Cfg.ServerDTLS = v
	}
	if v, ok := req["server_dtls_addr"].(string); ok {
		base.Cfg.ServerDTLSAddr = v
	}
	if v, ok := req["link_mode"].(string); ok {
		base.Cfg.LinkMode = v
	}
	if v, ok := req["ipv4_master"].(string); ok {
		base.Cfg.Ipv4Master = v
	}
	if v, ok := req["ipv4_cidr"].(string); ok {
		base.Cfg.Ipv4CIDR = v
	}
	if v, ok := req["ipv4_gateway"].(string); ok {
		base.Cfg.Ipv4Gateway = v
	}
	if v, ok := req["ipv4_start"].(string); ok {
		base.Cfg.Ipv4Start = v
	}
	if v, ok := req["ipv4_end"].(string); ok {
		base.Cfg.Ipv4End = v
	}
	if v, ok := req["max_client"].(float64); ok {
		base.Cfg.MaxClient = int(v)
	}
	if v, ok := req["max_user_client"].(float64); ok {
		base.Cfg.MaxUserClient = int(v)
	}
	if v, ok := req["mtu"].(float64); ok {
		base.Cfg.Mtu = int(v)
	}
	if v, ok := req["session_timeout"].(float64); ok {
		base.Cfg.SessionTimeout = int(v)
	}
	if v, ok := req["idle_timeout"].(float64); ok {
		base.Cfg.IdleTimeout = int(v)
	}
	if v, ok := req["issuer"].(string); ok {
		base.Cfg.Issuer = v
	}
	if v, ok := req["default_group"].(string); ok {
		base.Cfg.DefaultGroup = v
	}

	// 更新 SettingOther（页面显示配置）
	var other dbdata.SettingOther
	dbdata.SettingGet(&other)

	needUpdateOther := false
	if v, ok := req["banner"].(string); ok {
		other.Banner = v
		needUpdateOther = true
	}
	if v, ok := req["login_page_title"].(string); ok {
		other.Homeindex = v
		needUpdateOther = true
	}
	if v, ok := req["login_page_html"].(string); ok {
		other.AccountMail = v
		needUpdateOther = true
	}
	if v, ok := req["link_addr"].(string); ok {
		other.LinkAddr = v
		needUpdateOther = true
	}

	if needUpdateOther {
		if err := dbdata.SettingSet(&other); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存显示配置失败: " + err.Error()})
			return
		}
	}

	// 持久化配置到 server.toml
	if err := saveConfigToFile(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存配置文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "配置已保存"})
}

// saveConfigToFile 将当前配置保存到 server.toml 文件
func saveConfigToFile() error {
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	confPath := filepath.Join(workDir, "data", "vpn", "conf", "server.toml")

	// 读取原始文件内容
	content, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	configMap := map[string]string{
		"server_addr":       base.Cfg.ServerAddr,
		"server_dtls":       fmt.Sprintf("%v", base.Cfg.ServerDTLS),
		"server_dtls_addr":  base.Cfg.ServerDTLSAddr,
		"link_mode":         base.Cfg.LinkMode,
		"ipv4_master":       base.Cfg.Ipv4Master,
		"ipv4_cidr":         base.Cfg.Ipv4CIDR,
		"ipv4_gateway":      base.Cfg.Ipv4Gateway,
		"ipv4_start":        base.Cfg.Ipv4Start,
		"ipv4_end":          base.Cfg.Ipv4End,
		"max_client":        fmt.Sprintf("%d", base.Cfg.MaxClient),
		"max_user_client":   fmt.Sprintf("%d", base.Cfg.MaxUserClient),
		"mtu":               fmt.Sprintf("%d", base.Cfg.Mtu),
		"session_timeout":   fmt.Sprintf("%d", base.Cfg.SessionTimeout),
		"idle_timeout":      fmt.Sprintf("%d", base.Cfg.IdleTimeout),
		"issuer":            base.Cfg.Issuer,
		"default_group":     base.Cfg.DefaultGroup,
		"iptables_nat":      fmt.Sprintf("%v", base.Cfg.IptablesNat),
	}

	// 更新已有配置项
	updatedKeys := make(map[string]bool)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		if newVal, exists := configMap[key]; exists {
			// 判断是否需要引号
			if key == "server_dtls" || key == "iptables_nat" || key == "max_client" ||
				key == "max_user_client" || key == "mtu" || key == "session_timeout" || key == "idle_timeout" {
				lines[i] = fmt.Sprintf("%s = %s", key, newVal)
			} else {
				lines[i] = fmt.Sprintf("%s = \"%s\"", key, newVal)
			}
			updatedKeys[key] = true
		}
	}

	// 添加不存在的配置项
	var newLines []string
	for key, val := range configMap {
		if !updatedKeys[key] {
			if key == "server_dtls" || key == "iptables_nat" || key == "max_client" ||
				key == "max_user_client" || key == "mtu" || key == "session_timeout" || key == "idle_timeout" {
				newLines = append(newLines, fmt.Sprintf("%s = %s", key, val))
			} else {
				newLines = append(newLines, fmt.Sprintf("%s = \"%s\"", key, val))
			}
		}
	}

	if len(newLines) > 0 {
		lines = append(lines, newLines...)
	}

	return os.WriteFile(confPath, []byte(strings.Join(lines, "\n")), 0644)
}

// ========== 使用范围选项 API ==========

// handleGetScopeUsers 获取可选的本地用户列表（用于 VPN 使用范围配置）
func handleGetScopeUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := storage.DB.Model(&models.User{}).
		Where("is_deleted = 0 AND status = 1").
		Select("id, username, nickname, phone, email")

	if keyword != "" {
		query = query.Where("username LIKE ? OR nickname LIKE ? OR phone LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	var users []struct {
		ID       uint   `json:"id"`
		Username string `json:"username"`
		Nickname string `json:"nickname"`
		Phone    string `json:"phone"`
		Email    string `json:"email"`
	}
	query.Limit(limit).Find(&users)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    users,
	})
}

// DepartmentTreeNode 部门树节点
type DepartmentTreeNode struct {
	ID       uint                  `json:"id"`
	Value    uint                  `json:"value"`
	Name     string                `json:"name"`
	Label    string                `json:"label"`
	ParentID *uint                 `json:"parent_id,omitempty"`
	Children []*DepartmentTreeNode `json:"children,omitempty"`
}

// handleGetScopeDepartments 获取可选的部门列表（用于 VPN 使用范围配置）
// 返回树形结构
func handleGetScopeDepartments(c *gin.Context) {
	var departments []struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		ParentID *uint  `json:"parent_id"`
	}
	storage.DB.Model(&models.UserGroup{}).
		Select("id, name, parent_id").
		Order("id ASC").
		Find(&departments)

	// 构建树形结构
	nodeMap := make(map[uint]*DepartmentTreeNode)
	var roots []*DepartmentTreeNode

	// 第一遍：创建所有节点
	for _, dept := range departments {
		node := &DepartmentTreeNode{
			ID:       dept.ID,
			Value:    dept.ID,
			Name:     dept.Name,
			Label:    dept.Name,
			ParentID: dept.ParentID,
			Children: nil, // 初始为 nil，有子节点时才会变成数组
		}
		nodeMap[dept.ID] = node
	}

	// 第二遍：构建父子关系
	for _, node := range nodeMap {
		if node.ParentID == nil || *node.ParentID == 0 {
			roots = append(roots, node)
		} else {
			if parent, ok := nodeMap[*node.ParentID]; ok {
				if parent.Children == nil {
					parent.Children = []*DepartmentTreeNode{}
				}
				parent.Children = append(parent.Children, node)
			} else {
				// 父节点不存在，作为根节点
				roots = append(roots, node)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    roots,
	})
}

// handleOnlineUsers 在线用户列表（适配 Go-SyncFlow 格式）
func handleOnlineUsers(c *gin.Context) {
	searchCate := c.Query("search_cate")
	searchText := c.Query("search_text")
	showSleeper, _ := strconv.ParseBool(c.DefaultQuery("show_sleeper", "false"))

	datas := sessdata.GetOnlineSess(searchCate, searchText, showSleeper)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"list":  datas,
			"total": len(datas),
		},
	})
}

// handleUserActLogList 用户活动日志列表（适配 Go-SyncFlow 格式）
func handleUserActLogList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	username := c.Query("username")
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var logs []dbdata.UserActLog
	session := dbdata.GetXdb().Desc("id")

	if username != "" {
		session = session.Where("username LIKE ?", "%"+username+"%")
	}
	if startTime != "" {
		session = session.Where("created_at >= ?", startTime)
	}
	if endTime != "" {
		session = session.Where("created_at <= ?", endTime)
	}

	total, err := session.Limit(pageSize, (page-1)*pageSize).FindAndCount(&logs)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"list":  logs,
			"total": total,
		},
	})
}

// handleAuditList 访问审计日志列表（适配 Go-SyncFlow 格式）
func handleAuditList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	username := c.Query("username")
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var logs []dbdata.AccessAudit
	session := dbdata.GetXdb().Desc("id")

	if username != "" {
		session = session.Where("username LIKE ?", "%"+username+"%")
	}
	if startTime != "" {
		session = session.Where("created_at >= ?", startTime)
	}
	if endTime != "" {
		session = session.Where("created_at <= ?", endTime)
	}

	total, err := session.Limit(pageSize, (page-1)*pageSize).FindAndCount(&logs)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"list":  logs,
			"total": total,
		},
	})
}

// handleGetLdapConnectors 获取上游 LDAP 连接器列表（用于 VPN 认证配置）- 兼容旧版
func handleGetLdapConnectors(c *gin.Context) {
	var connectors []struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}

	// 获取已启用的上游 LDAP 连接器（方向为 upstream 或 both）
	storage.DB.Model(&models.Connector{}).
		Where("status = ? AND direction IN ? AND (type = ? OR type = ?)",
			1, []string{"upstream", "both"}, "ldap_ad", "ldap_openldap").
		Select("id, name, type").
		Find(&connectors)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    connectors,
	})
}

// handleGetAuthConnectors 获取所有可用于认证的连接器列表
func handleGetAuthConnectors(c *gin.Context) {
	var connectors []struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		TypeName string `json:"typeName"`
		Category string `json:"category"`
	}

	// 获取已启用的、支持认证的上游连接器
	// 支持的类型：LDAP (ldap_ad, ldap_openldap), 数据库 (db_*), RADIUS, HTTP API
	authTypes := []string{
		"ldap_ad", "ldap_openldap",
		"db_mysql", "db_postgresql", "db_oracle", "db_sqlserver",
		"radius", "http_api",
	}

	storage.DB.Model(&models.Connector{}).
		Where("status = ? AND direction IN ? AND type IN ?",
			1, []string{"upstream", "both"}, authTypes).
		Select("id, name, type").
		Find(&connectors)

	// 添加类型名称和分类
	for i := range connectors {
		conn := &connectors[i]
		switch conn.Type {
		case "ldap_ad":
			conn.TypeName = "LDAP/AD"
			conn.Category = "ldap"
		case "ldap_openldap":
			conn.TypeName = "OpenLDAP"
			conn.Category = "ldap"
		case "db_mysql":
			conn.TypeName = "MySQL"
			conn.Category = "database"
		case "db_postgresql":
			conn.TypeName = "PostgreSQL"
			conn.Category = "database"
		case "db_oracle":
			conn.TypeName = "Oracle"
			conn.Category = "database"
		case "db_sqlserver":
			conn.TypeName = "SQL Server"
			conn.Category = "database"
		case "radius":
			conn.TypeName = "RADIUS"
			conn.Category = "radius"
		case "http_api":
			conn.TypeName = "HTTP API"
			conn.Category = "http"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    connectors,
	})
}

// ========== 地址组 RESTful 适配器 ==========

func handleAddressGroupList(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	pageSize := 50
	count := dbdata.CountAll(&dbdata.AddressGroup{})

	var datas []dbdata.AddressGroup
	err := dbdata.Find(&datas, pageSize, page)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"list":      datas,
			"total":     count,
			"page_size": pageSize,
		},
	})
}

func handleAddressGroupDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 ID"})
		return
	}

	var data dbdata.AddressGroup
	err = dbdata.One("Id", id, &data)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "地址组不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func handleAddressGroupCreate(c *gin.Context) {
	var ag dbdata.AddressGroup
	if err := c.ShouldBindJSON(&ag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	ag.Id = 0
	if err := dbdata.SetAddressGroup(&ag); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "创建成功", "data": ag})
}

func handleAddressGroupUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 ID"})
		return
	}

	var ag dbdata.AddressGroup
	if err := c.ShouldBindJSON(&ag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	ag.Id = id
	if err := dbdata.SetAddressGroup(&ag); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "更新成功"})
}

func handleAddressGroupDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 ID"})
		return
	}

	if inUse, groupName := dbdata.IsAddressGroupInUse(id); inUse {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "地址组正被用户组「" + groupName + "」引用，无法删除"})
		return
	}

	data := dbdata.AddressGroup{Id: id}
	if err := dbdata.Del(&data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "删除成功"})
}

func handleAddressGroupNames(c *gin.Context) {
	names := dbdata.GetAddressGroupNamesIds()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    names,
	})
}
