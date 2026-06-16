package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"high-perf-wallet/internal/domain"
	"high-perf-wallet/pkg/logger"
)

type fxService struct {
	rdb        *redis.Client
	httpClient *http.Client
}

func NewExchangeRateService(rdb *redis.Client) domain.ExchangeRateService {
	return &fxService{
		rdb: rdb,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type erAPIResponse struct {
	Result   string             `json:"result"`
	BaseCode string             `json:"base_code"`
	Rates    map[string]float64 `json:"rates"`
}

// getFallbackRate provides a local fallback exchange rate map in case external API is unreachable.
func getFallbackRate(from, to string) (float64, error) {
	from = strings.ToUpper(from)
	to = strings.ToUpper(to)

	if from == to {
		return 1.0, nil
	}

	// Mock rates
	rates := map[string]map[string]float64{
		"USD": {
			"IDR": 16400.0,
			"SGD": 1.35,
		},
		"IDR": {
			"USD": 0.000061,
			"SGD": 0.000082,
		},
		"SGD": {
			"USD": 0.74,
			"IDR": 12100.0,
		},
	}

	if toRates, exists := rates[from]; exists {
		if rate, ok := toRates[to]; ok {
			return rate, nil
		}
	}

	return 0, fmt.Errorf("unsupported currency pair for fallback: %s -> %s", from, to)
}

func (s *fxService) GetRate(ctx context.Context, fromCurrency, toCurrency string) (float64, error) {
	fromCurrency = strings.ToUpper(fromCurrency)
	toCurrency = strings.ToUpper(toCurrency)

	if fromCurrency == toCurrency {
		return 1.0, nil
	}

	cacheKey := fmt.Sprintf("fx:rate:%s:%s", fromCurrency, toCurrency)

	// 1. Try to fetch from Redis cache
	cachedVal, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		if rate, err := strconv.ParseFloat(cachedVal, 64); err == nil {
			logger.WithCtx(ctx).Debug("FX Cache hit", zap.String("pair", fromCurrency+"->"+toCurrency), zap.Float64("rate", rate))
			return rate, nil
		}
	} else if err != redis.Nil {
		logger.WithCtx(ctx).Warn("Redis error while fetching FX cache", zap.Error(err))
	}

	// 2. Cache miss, fetch from external API (open.er-api.com)
	logger.WithCtx(ctx).Info("FX Cache miss, calling external API", zap.String("pair", fromCurrency+"->"+toCurrency))
	url := fmt.Sprintf("https://open.er-api.com/v6/latest/%s", fromCurrency)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return s.handleFallback(ctx, cacheKey, fromCurrency, toCurrency, err, "failed to create http request")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return s.handleFallback(ctx, cacheKey, fromCurrency, toCurrency, err, "http client request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.handleFallback(ctx, cacheKey, fromCurrency, toCurrency, nil, fmt.Sprintf("external API returned non-OK status: %d", resp.StatusCode))
	}

	var apiResp erAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return s.handleFallback(ctx, cacheKey, fromCurrency, toCurrency, err, "failed to decode API response JSON")
	}

	if apiResp.Result != "success" {
		return s.handleFallback(ctx, cacheKey, fromCurrency, toCurrency, nil, "external API returned success=false result")
	}

	rate, ok := apiResp.Rates[toCurrency]
	if !ok {
		return s.handleFallback(ctx, cacheKey, fromCurrency, toCurrency, nil, fmt.Sprintf("target currency %s not found in API response", toCurrency))
	}

	// 3. Cache the successful rate in Redis (TTL: 1 Hour)
	err = s.rdb.Set(ctx, cacheKey, strconv.FormatFloat(rate, 'f', -1, 64), 1*time.Hour).Err()
	if err != nil {
		logger.WithCtx(ctx).Warn("Failed to cache FX rate in Redis", zap.Error(err))
	}

	return rate, nil
}

func (s *fxService) handleFallback(ctx context.Context, cacheKey, from, to string, origErr error, reason string) (float64, error) {
	logger.WithCtx(ctx).Warn("FX API failed, utilizing fallback rates",
		zap.String("reason", reason),
		zap.String("pair", from+"->"+to),
		zap.Error(origErr),
	)

	rate, err := getFallbackRate(from, to)
	if err != nil {
		return 0, fmt.Errorf("FX service unavailable: %s: %w", reason, err)
	}

	// Cache fallback rates too, but with a shorter TTL (e.g. 5 minutes) to avoid spamming the logs/API
	_ = s.rdb.Set(ctx, cacheKey, strconv.FormatFloat(rate, 'f', -1, 64), 5*time.Minute).Err()

	return rate, nil
}
