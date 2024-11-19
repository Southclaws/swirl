package swirl

import (
	"context"
	"time"
)

// Store describes a Redis-like cache suitable for implementing the sliding
// window counters rate limiter. You can use Redis, or implement a custom store
// using whichever cache/database you prefer as long as it supports hash sets.
type Store interface {
	HIncrBy(ctx context.Context, key string, field string, incr int64) (int, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HDel(ctx context.Context, key string, field string) error
	Expire(ctx context.Context, key string, expiration time.Duration) error
}
