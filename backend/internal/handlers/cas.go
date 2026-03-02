package handlers

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

type casProtocolConfig struct {
	Services []string `json:"services"`
}

type casFailure struct {
	XMLName xml.Name `xml:"cas:authenticationFailure"`
	Code    string   `xml:"code,attr"`
	Message string   `xml:",chardata"`
}

type casAttributes struct {
	Roles       []string `xml:"cas:roles,omitempty"`
	Permissions []string `xml:"cas:permissions,omitempty"`
	Email       string   `xml:"cas:email,omitempty"`
	Phone       string   `xml:"cas:phone,omitempty"`
}

type casSuccess struct {
	XMLName    xml.Name       `xml:"cas:authenticationSuccess"`
	User       string         `xml:"cas:user"`
	Attributes *casAttributes `xml:"cas:attributes,omitempty"`
}

type casServiceResponse struct {
	XMLName xml.Name    `xml:"cas:serviceResponse"`
	Xmlns   string      `xml:"xmlns:cas,attr"`
	Success *casSuccess `xml:",omitempty"`
	Failure *casFailure `xml:",omitempty"`
}

func CASLogin(c *gin.Context) {
	serviceInput := strings.TrimSpace(c.Query("service"))
	serviceKey := normalizeServiceKey(serviceInput)
	if serviceKey == "" {
		respondError(c, http.StatusBadRequest, "service 不能为空")
		return
	}

	app, err := findCASAppByService(serviceKey)
	if err != nil {
		_ = recordSSOAudit(0, "cas", 0, "", "login", false, "service 未授权: "+serviceKey, c.ClientIP())
		respondError(c, http.StatusBadRequest, "service 未授权")
		return
	}
	if !app.Enabled {
		_ = recordSSOAudit(app.ID, "cas", 0, "", "login", false, "应用已禁用", c.ClientIP())
		respondError(c, http.StatusForbidden, "应用已禁用")
		return
	}

	claims, err := parseBearerUser(c)
	if err != nil {
		redirectToLogin(c)
		return
	}
	if !canUserAccessApp(claims.UserID, app.ID, app.AccessMode) {
		_ = recordSSOAudit(app.ID, "cas", claims.UserID, claims.Username, "login", false, "无权访问该应用", c.ClientIP())
		respondError(c, http.StatusForbidden, "无权访问该应用")
		return
	}

	ticket := "ST-" + randomURLSafeString(32)
	row := models.SSOCasTicket{
		TicketHash: sha256Hex(ticket),
		AppID:      app.ID,
		Service:    serviceKey,
		UserID:     claims.UserID,
		Username:   claims.Username,
		ExpiresAt:  time.Now().Add(3 * time.Minute),
		CreatedAt:  time.Now(),
	}
	if err := storage.DB.Create(&row).Error; err != nil {
		_ = recordSSOAudit(app.ID, "cas", claims.UserID, claims.Username, "login", false, "生成 ticket 失败", c.ClientIP())
		respondError(c, http.StatusInternalServerError, "生成 ticket 失败")
		return
	}
	_ = recordSSOAudit(app.ID, "cas", claims.UserID, claims.Username, "login", true, "service="+serviceKey, c.ClientIP())

	redirectService := serviceInput
	if redirectService == "" {
		redirectService = serviceKey
	}
	u, err := url.Parse(redirectService)
	if err != nil {
		respondError(c, http.StatusBadRequest, "service 无效")
		return
	}
	q := u.Query()
	q.Set("ticket", ticket)
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, u.String())
}

func CASServiceValidate(c *gin.Context) {
	service := normalizeServiceKey(strings.TrimSpace(c.Query("service")))
	ticket := strings.TrimSpace(c.Query("ticket"))
	if service == "" || ticket == "" {
		_ = recordSSOAudit(0, "cas", 0, "", "validate", false, "INVALID_REQUEST: service/ticket 不能为空", c.ClientIP())
		writeCASFailure(c, "INVALID_REQUEST", "service/ticket 不能为空")
		return
	}

	var row models.SSOCasTicket
	if err := storage.DB.Where("ticket_hash = ?", sha256Hex(ticket)).First(&row).Error; err != nil {
		_ = recordSSOAudit(0, "cas", 0, "", "validate", false, "INVALID_TICKET: ticket 无效", c.ClientIP())
		writeCASFailure(c, "INVALID_TICKET", "ticket 无效")
		return
	}
	if row.Service != service {
		_ = recordSSOAudit(row.AppID, "cas", row.UserID, row.Username, "validate", false, "INVALID_SERVICE: service 不匹配", c.ClientIP())
		writeCASFailure(c, "INVALID_SERVICE", "service 不匹配")
		return
	}
	if time.Now().After(row.ExpiresAt) {
		_ = recordSSOAudit(row.AppID, "cas", row.UserID, row.Username, "validate", false, "INVALID_TICKET: ticket 已过期", c.ClientIP())
		writeCASFailure(c, "INVALID_TICKET", "ticket 已过期")
		return
	}
	if row.UsedAt != nil {
		if time.Since(*row.UsedAt) > 30*time.Second {
			_ = recordSSOAudit(row.AppID, "cas", row.UserID, row.Username, "validate", false, "INVALID_TICKET: ticket 已使用", c.ClientIP())
			writeCASFailure(c, "INVALID_TICKET", "ticket 已使用")
			return
		}
	} else {
		now := time.Now()
		storage.DB.Model(&row).Update("used_at", now)
	}

	var user models.User
	if err := storage.DB.Preload("Roles").First(&user, row.UserID).Error; err != nil {
		_ = recordSSOAudit(row.AppID, "cas", row.UserID, row.Username, "validate", false, "INVALID_USER: 用户不存在", c.ClientIP())
		writeCASFailure(c, "INVALID_USER", "用户不存在")
		return
	}
	roles, perms := buildUserRBACForApp(row.AppID, user)

	attrs := &casAttributes{
		Roles:       roles,
		Permissions: perms,
		Email:       user.Email,
		Phone:       user.Phone,
	}
	resp := casServiceResponse{
		Xmlns: "http://www.yale.edu/tp/cas",
		Success: &casSuccess{
			User:       user.Username,
			Attributes: attrs,
		},
	}
	_ = recordSSOAudit(row.AppID, "cas", user.ID, user.Username, "validate", true, "service="+service, c.ClientIP())
	c.Header("Content-Type", "text/xml; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	_, _ = c.Writer.WriteString(xml.Header)
	_ = xml.NewEncoder(c.Writer).Encode(resp)
}

func CASLogout(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func writeCASFailure(c *gin.Context, code string, message string) {
	resp := casServiceResponse{
		Xmlns: "http://www.yale.edu/tp/cas",
		Failure: &casFailure{
			Code:    code,
			Message: message,
		},
	}
	c.Header("Content-Type", "text/xml; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	_, _ = c.Writer.WriteString(xml.Header)
	_ = xml.NewEncoder(c.Writer).Encode(resp)
}

func findCASAppByService(service string) (*models.SSOApp, error) {
	var protocols []models.SSOAppProtocol
	if err := storage.DB.Where("protocol = ? AND enabled = ?", "cas", true).Find(&protocols).Error; err != nil {
		return nil, err
	}
	for _, p := range protocols {
		var cfg casProtocolConfig
		if strings.TrimSpace(p.Config) != "" {
			_ = json.Unmarshal([]byte(p.Config), &cfg)
		}
		if len(cfg.Services) == 0 {
			continue
		}
		allowed := false
		for _, s := range cfg.Services {
			if normalizeServiceKey(strings.TrimSpace(s)) == service {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}
		var app models.SSOApp
		if err := storage.DB.First(&app, p.AppID).Error; err != nil {
			continue
		}
		return &app, nil
	}
	return nil, errors.New("not allowed")
}

func normalizeServiceKey(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil {
		return strings.TrimRight(strings.TrimSpace(s), "/")
	}
	if u.Scheme == "" || u.Host == "" {
		return strings.TrimRight(strings.TrimSpace(s), "/")
	}
	u.Fragment = ""
	if u.Path == "/" {
		u.Path = ""
		u.RawPath = ""
	}
	return u.String()
}
