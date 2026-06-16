package main

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"high-perf-wallet/internal/config"
	v1 "high-perf-wallet/internal/delivery/http/v1"
	"high-perf-wallet/internal/repository/postgres"
	redisRepo "high-perf-wallet/internal/repository/redis"
	"high-perf-wallet/internal/usecase"
	"high-perf-wallet/pkg/logger"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger.InitLogger()
	defer logger.Log.Sync()

	// 1. Load Config terpusat
	cfg := config.LoadConfig()
	logger.Log.Info("Configuration loaded successfully",
		zap.String("port", cfg.Port),
		zap.String("redis_addr", cfg.RedisAddr),
	)

	ctx := context.Background()

	// 2. Inisialisasi Database PostgreSQL
	dbPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Log.Fatal("Koneksi DB gagal", zap.Error(err))
	}

	// 3. Jalankan Database Migrations otomatis
	if err := postgres.RunMigrations(ctx, dbPool); err != nil {
		logger.Log.Fatal("Database migrations gagal", zap.Error(err))
	}

	// 4. Inisialisasi Redis Client
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Log.Fatal("Koneksi Redis gagal", zap.Error(err))
	}

		// 5. Inisialisasi Repositories & Usecases (Dependency Injection)
	walletRepo := postgres.NewWalletRepository(dbPool)
	idempotencyRepo := redisRepo.NewIdempotencyRepository(rdb)
	fxService := redisRepo.NewExchangeRateService(rdb)
	transferUC := usecase.NewTransferUsecase(walletRepo, fxService)
	walletUC := usecase.NewWalletUsecase(walletRepo)

	// 6. Inisialisasi HTTP Handler & Router
	r := gin.New()
	r.Use(gin.Recovery()) // Mencegah server mati jika terjadi panic

	walletHandler := v1.NewWalletHandler(transferUC, walletUC, idempotencyRepo)
	v1.MapRoutes(r, walletHandler)

	// 7. Setup HTTP Server untuk Asynchronous Running (Graceful Shutdown)
	srv := &http.Server{
		Addr:    cfg.Port,
		Handler: r,
	}

	// Jalankan server di goroutine terpisah agar main thread tidak terblok
	go func() {
		logger.Log.Info("Engine running", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal("ListenAndServe failed", zap.Error(err))
		}
	}()

	// 8. Graceful Shutdown: Tunggu sinyal sistem operasi untuk mematikan server
	quit := make(chan os.Signal, 1)
	// Listen sinyal interupsi (Ctrl+C / SIGINT) atau kill command (SIGTERM)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // Blok main thread sampai ada sinyal masuk

	logger.Log.Warn("Shutting down server...")

	// Buat context dengan batas waktu timeout dari config
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownTimeout)*time.Second)
	defer cancel()

	// Stop menerima request HTTP baru, tunggu yang sedang jalan selesai
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("Server forced to shutdown", zap.Error(err))
	}

	// Tutup koneksi resources secara aman
	logger.Log.Warn("Closing database and cache connections...")
	dbPool.Close()
	_ = rdb.Close()

	logger.Log.Info("Server exited gracefully")
}
