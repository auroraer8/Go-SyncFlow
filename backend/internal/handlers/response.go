package handlers

import (
	"math"

	"github.com/gin-gonic/gin"
)

func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}

func respondOK(c *gin.Context, data interface{}) {
	c.JSON(200, gin.H{"success": true, "data": data})
}

func respondList(c *gin.Context, list interface{}, total int64) {
	c.JSON(200, gin.H{"success": true, "data": gin.H{"list": list, "total": total}})
}

func respondError(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{"success": false, "message": message})
}
