package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port            string
	DatabaseURL     string
	RedisAddr       string
	ShutdownTimeout int
}

func LoadConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	} else if port[0] != ':' {
		port = ":" + port
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:secretpassword@localhost:5432/wallet_db?sslmode=disable"
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	shutdownTimeoutStr := os.Getenv("SHUTDOWN_TIMEOUT")
	shutdownTimeout := 10 // default 10 detik
	if shutdownTimeoutStr != "" {
		if val, err := strconv.Atoi(shutdownTimeoutStr); err == nil {
			shutdownTimeout = val
		}
	}

	return &Config{
		Port:            port,
		DatabaseURL:     databaseURL,
		RedisAddr:       redisAddr,
		ShutdownTimeout: shutdownTimeout,
	}
}
