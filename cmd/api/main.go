package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"high-perf-wallet/internal/domain"
	"high-perf-wallet/internal/repository/postgres"
	redisRepo "high-perf-wallet/internal/repository/redis"
	"high-perf-wallet/internal/usecase"
	"high-perf-wallet/pkg/logger"
	"net/http"
	"os"
	"strconv"
	"time"
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

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Log.Fatal("Koneksi Redis gagal", zap.Error(err))
	}
	defer rdb.Close()

	walletRepo := postgres.NewWalletRepository(dbPool)
	idempotencyRepo := redisRepo.NewIdempotencyRepository(rdb)
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

		if idempotencyKey != "" {
			cached, err := idempotencyRepo.Get(c.Request.Context(), idempotencyKey)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "idempotency_check_failed"})
				return
			}
			if cached != nil {
				if cached.Status == "started" {
					c.JSON(http.StatusConflict, gin.H{"error": "request_in_progress"})
					return
				}
				c.Data(cached.ResponseCode, "application/json; charset=utf-8", []byte(cached.ResponseBody))
				return
			}

			err = idempotencyRepo.Set(c.Request.Context(), idempotencyKey, &domain.Idempotency{
				Status: "started",
			}, 24*time.Hour)
			if err != nil {
				if err.Error() == "key_already_exists" {
					c.JSON(http.StatusConflict, gin.H{"error": "request_in_progress"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "idempotency_lock_failed"})
				return
			}
		}

		err := transferUC.ExecuteTransfer(c.Request.Context(), idempotencyKey, req.FromAccountID, req.ToAccountID, req.Amount)

		var respCode int
		var respBody []byte
		if err != nil {
			respCode = http.StatusUnprocessableEntity
			respBody = []byte(`{"error":"` + err.Error() + `"}`)
		} else {
			respCode = http.StatusOK
			respBody = []byte(`{"status":"success","message":"funds_transferred_successfully"}`)
		}

		if idempotencyKey != "" {
			_ = idempotencyRepo.Set(c.Request.Context(), idempotencyKey, &domain.Idempotency{
				Status:       "completed",
				ResponseCode: respCode,
				ResponseBody: string(respBody),
			}, 24*time.Hour)
		}

		c.Data(respCode, "application/json; charset=utf-8", respBody)
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
