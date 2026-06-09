package redis

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/redis/go-redis/v9"
	"high-perf-wallet/internal/domain"
	"time"
)

type idempotencyRepository struct {
	rdb *redis.Client
}

func NewIdempotencyRepository(rdb *redis.Client) domain.IdempotencyRepository {
	return &idempotencyRepository{rdb: rdb}
}

func (r *idempotencyRepository) Get(ctx context.Context, key string) (*domain.Idempotency, error) {
	val, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // Kunci tidak ditemukan, return nil tanpa error
		}
		return nil, err
	}

	var idem domain.Idempotency
	err = json.Unmarshal([]byte(val), &idem)
	if err != nil {
		return nil, err
	}

	return &idem, nil
}

func (r *idempotencyRepository) Set(ctx context.Context, key string, val *domain.Idempotency, ttl time.Duration) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	// Jika status 'started', gunakan SetNX agar operasi ini atomik (hanya berhasil jika belum ada)
	// Ini mencegah dua request dengan key sama dieksekusi di mili-detik yang sama.
	if val.Status == "started" {
		success, err := r.rdb.SetNX(ctx, key, string(data), ttl).Result()
		if err != nil {
			return err
		}
		if !success {
			return errors.New("key_already_exists")
		}
		return nil
	}

	// Jika 'completed', overwrite/timpa key 'started' yang sudah ada
	return r.rdb.Set(ctx, key, string(data), ttl).Err()
}
