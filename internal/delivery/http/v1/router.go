package v1

import (
	"github.com/gin-gonic/gin"
	"high-perf-wallet/internal/delivery/http/middleware"
)

func MapRoutes(r *gin.Engine, handler *WalletHandler) {
	// Pasang middleware RequestID secara global ke router Gin
	r.Use(middleware.RequestID())

	// Kelompokkan rute API v1
	v1 := r.Group("/api/v1")
	{
		v1.POST("/wallets/transfer", handler.Transfer)
		v1.GET("/wallets/:id", handler.GetByID)
		v1.GET("/wallets/:id/transfers", handler.GetTransfers)
	}
}
