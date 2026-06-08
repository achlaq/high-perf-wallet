package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"high-perf-wallet/internal/repository/postgres"
	"high-perf-wallet/internal/usecase"
	"high-perf-wallet/pkg/logger"
	"net/http"
	"os"
	"strconv"
)

func main() {
	logger.InitLogger()
	defer logger.Log.Sync()

	ctx := context.Background()
	dbURI := os.Getenv("DATABASE_URL")
	if dbURI == "" {
		dbURI = "postgres://postgres:secretpassword@localhost:5432/wallet_db?sslmode=disable"
	}

	dbPool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		logger.Log.Fatal("Koneksi DB gagal", zap.Error(err))
	}
	defer dbPool.Close()

	walletRepo := postgres.NewWalletRepository(dbPool)
	transferUC := usecase.NewTransferUsecase(walletRepo)
	walletUC := usecase.NewWalletUsecase(walletRepo)

	r := gin.New()
	r.Use(gin.Recovery()) // Mencegah server mati jika panic

	r.POST("/api/v1/wallets/transfer", func(c *gin.Context) {
		var req struct {
			FromAccountID int64 `json:"from_account_id" binding:"required"`
			ToAccountID   int64 `json:"to_account_id" binding:"required"`
			Amount        int64 `json:"amount" binding:"required"`
		}

		idempotencyKey := c.GetHeader("X-Idempotency-Key")

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad_request"})
			return
		}

		err := transferUC.ExecuteTransfer(c.Request.Context(), idempotencyKey, req.FromAccountID, req.ToAccountID, req.Amount)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "funds_transferred_successfully"})
	})

	r.GET("/api/v1/wallets/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
			return
		}

		acc, err := walletUC.GetByID(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, acc)
	})

	r.GET("/api/v1/wallets/:id/transfers", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
			return
		}

		transfers, err := walletUC.GetTransfers(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, transfers)
	})

	logger.Log.Info("Engine running on port :8080")
	r.Run(":8080")
}
