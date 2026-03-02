package middleware

import (
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"

	"github.com/gin-gonic/gin"
)

func RecordLoginLog(userID uint, username, ip, userAgent string, success bool, message string) {
	status := int8(0)
	if success {
		status = 1
	}
	log := models.LoginLog{
		UserID:    userID,
		Username:  username,
		IP:        ip,
		UserAgent: userAgent,
		Status:    status,
		Message:   message,
	}
	storage.DB.Create(&log)
}

func RecordOperationLog(c *gin.Context, module, action, target, content string) {
	userID := GetUserID(c)
	username := GetUsername(c)
	ip := c.ClientIP()

	log := models.OperationLog{
		UserID:   userID,
		Username: username,
		Module:   module,
		Action:   action,
		Target:   target,
		Content:  content,
		IP:       ip,
	}
	storage.DB.Create(&log)
}

func OperationLogMiddleware(module, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 只记录成功的操作
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			target := c.Request.URL.Path
			RecordOperationLog(c, module, action, target, "")
		}
	}
}
