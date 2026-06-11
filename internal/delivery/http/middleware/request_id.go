package middleware

import (
	"crypto/rand"
	"fmt"
	"github.com/gin-gonic/gin"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ambil X-Request-ID dari request header (jika dikirim oleh API Gateway / client)
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			// Generate UUIDv4-like string menggunakan crypto/rand bawaan Go
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			reqID = fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
		}

		// Simpan request ID di context Gin untuk diakses oleh logger
		c.Set("RequestID", reqID)

		// Set header X-Request-ID di HTTP Response
		c.Header("X-Request-ID", reqID)

		c.Next()
	}
}
