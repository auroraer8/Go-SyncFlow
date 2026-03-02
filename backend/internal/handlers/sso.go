package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/imclient"
	"go-syncflow/internal/ldapserver"
	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

// SSOProvider SSO 提供者信息（返回给前端）
type SSOProvider struct {
	ConnectorID uint   `json:"connectorId"`
	Platform    string `json:"platform"`
	Label       string `json:"label"`
	CorpID      string `json:"corpId,omitempty"`
	AppID       string `json:"appId,omitempty"`
	CallbackURL string `json:"callbackUrl,omitempty"`
	AgentID     string `json:"agentId,omitempty"`
}

// GetSSOProviders 获取可用的 SSO 免登列表（公开接口）
func GetSSOProviders(c *gin.Context) {
	var connectors []models.Connector
	storage.DB.Where("im_enable_sso = ? AND status = 1", true).
		Order("im_sso_priority ASC, id ASC").
		Find(&connectors)

	providers := make([]SSOProvider, 0, len(connectors))
	for _, conn := range connectors {
		if !conn.SupportsSSO() {
			continue
		}

		label := conn.IMSSOLabel
		if label == "" {
			label = conn.IMPlatformName() + "登录"
		}

		p := SSOProvider{
			ConnectorID: conn.ID,
			Platform:    conn.Type,
			Label:       label,
			CorpID:      conn.IMCorpID,
			AppID:       conn.IMAppID,
			CallbackURL: conn.IMCallbackURL,
			AgentID:     conn.IMAgentID,
		}
		providers = append(providers, p)
	}

	respondOK(c, providers)
}

// SSOLogin 统一 SSO 免登认证
func SSOLogin(c *gin.Context) {
	var req struct {
		ConnectorID uint   `json:"connectorId" binding:"required"`
		Platform    string `json:"platform"`
		AuthCode    string `json:"authCode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// 查找连接器
	var conn models.Connector
	if err := storage.DB.First(&conn, req.ConnectorID).Error; err != nil {
		respondError(c, http.StatusNotFound, "连接器不存在")
		return
	}

	if !conn.SupportsSSO() || !conn.IMEnableSSO {
		respondError(c, http.StatusForbidden, "该连接器未启用SSO")
		return
	}

	// 创建 IM 客户端
	client, err := imclient.NewIMClient(conn)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "创建IM客户端失败: "+err.Error())
		return
	}

	// 通过 authCode 获取用户信息
	imUser, err := client.GetUserByAuthCode(req.AuthCode)
	if err != nil {
		log.Printf("[SSO] %s 授权码验证失败: %v", conn.Type, err)
		respondError(c, http.StatusUnauthorized, "授权验证失败: "+err.Error())
		return
	}

	// 匹配本地用户
	var localUser models.User
	found := false

	matchField := conn.IMMatchField
	if matchField == "" {
		matchField = "mobile"
	}

	switch matchField {
	case "mobile":
		if imUser.Mobile != "" {
			found = storage.DB.Where("phone = ? AND is_deleted = 0 AND status = 1", imUser.Mobile).First(&localUser).Error == nil
		}
	case "email":
		if imUser.Email != "" {
			found = storage.DB.Where("email = ? AND is_deleted = 0 AND status = 1", imUser.Email).First(&localUser).Error == nil
		}
	case "userid":
		found = storage.DB.Where("dingtalk_uid = ? AND is_deleted = 0 AND status = 1", imUser.UserID).First(&localUser).Error == nil
	}

	// 如果未找到，尝试自动注册
	if !found && conn.IMAutoRegister {
		rawPassword := generateRandomPassword()
		hashedPwd, _ := hashPasswordForUpstream(rawPassword)
		username := imclient.GenerateUsername(conn.IMUsernameRule, imUser)

		localUser = models.User{
			Username:        username,
			Password:        hashedPwd,
			SambaNTPassword: ldapserver.ComputeNTHash(rawPassword),
			Nickname:        imUser.Name,
			Phone:           imUser.Mobile,
			Email:           imUser.Email,
			Avatar:          imUser.Avatar,
			JobTitle:        imUser.JobTitle,
			Status:          1,
			Source:          conn.Type,
			DingTalkUID:     imUser.UserID,
		}
		if err := storage.DB.Create(&localUser).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "自动创建用户失败")
			return
		}

		// 分配角色
		if conn.IMDefaultRoleID > 0 {
			storage.DB.Create(&models.UserRole{UserID: localUser.ID, RoleID: conn.IMDefaultRoleID})
		}
		var normalRole models.Role
		if storage.DB.Where("code = ?", "user").First(&normalRole).Error == nil {
			storage.DB.Create(&models.UserRole{UserID: localUser.ID, RoleID: normalRole.ID})
		}

		found = true
		log.Printf("[SSO] 自动创建用户: %s (来源: %s)", username, conn.Type)
	}

	if !found {
		respondError(c, http.StatusNotFound, "未找到匹配的本地用户，请联系管理员")
		return
	}

	// 检查用户状态
	if localUser.Status != 1 {
		respondError(c, http.StatusForbidden, "用户已被禁用")
		return
	}

	// 生成 JWT Token
	token, err := middleware.GenerateToken(localUser.ID, localUser.Username)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "生成令牌失败")
		return
	}

	// 更新最后登录信息
	now := time.Now()
	storage.DB.Model(&localUser).Updates(map[string]interface{}{
		"last_login_at": now,
		"last_login_ip": c.ClientIP(),
	})

	// 记录登录日志
	storage.DB.Create(&models.LoginLog{
		UserID:    localUser.ID,
		Username:  localUser.Username,
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Status:    1,
		Message:   conn.IMPlatformName() + " SSO登录",
	})

	log.Printf("[SSO] %s 登录成功: %s (%s)", conn.IMPlatformName(), localUser.Username, imUser.Name)

	respondOK(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":       localUser.ID,
			"username": localUser.Username,
			"nickname": localUser.Nickname,
			"avatar":   localUser.Avatar,
		},
	})
}
