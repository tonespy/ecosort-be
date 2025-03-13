package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func DefaultClientAuth(recognizedKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != recognizedKey {
			c.JSON(http.StatusNotImplemented, gin.H{})
			c.Abort()
			return
		}
		c.Next()
	}
}
