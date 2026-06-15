package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port            string
	DatabaseURL     string
	RedisAddr       string
	ShutdownTimeout int
}

// LoadEnv reads a .env file and sets environment variables if they are not already set.
func LoadEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		// If the file does not exist, we return nil to allow production configuration via environment variables.
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines or comment lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first '='
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Strip quotes if they surround the value
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		// Set environment variable only if it is not already set
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

func LoadConfig() *Config {
	// Load environment variables from .env file
	_ = LoadEnv(".env")

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
