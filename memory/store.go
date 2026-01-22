package memory

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
)

var errNotFound = fmt.Errorf("not found")

type Entry struct {
	Value  any
	Expiry *time.Time
}

// Cache implements a very rudimentary in-memory cache that supports HSET style
// operations similar to Redis. Purely for testing the rate limiter.
type Cache struct {
	local *xsync.MapOf[string, Entry]
}

func New() *Cache {
	return &Cache{
		local: xsync.NewMapOf[string, Entry](),
	}
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	v, found := c.local.Load(key)
	if !found {
		return "", errNotFound
	}

	if v.Expiry != nil && v.Expiry.Before(time.Now()) {
		c.local.Delete(key)
		return "", errNotFound
	}

	str, ok := v.Value.(string)
	if !ok {
		return "", fmt.Errorf("value is not a string")
	}

	return str, nil
}

func (c *Cache) Set(ctx context.Context, key string, value string) error {
	c.local.Store(key, Entry{
		Value: value,
	})
	return nil
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	c.local.Delete(key)
	return nil
}

func (c *Cache) HIncrBy(ctx context.Context, key string, field string, incr int64) (int, error) {
	var computeErr error
	var resultValue int

	c.local.Compute(key, func(old Entry, found bool) (Entry, bool) {
		if found {
			oldHash, ok := old.Value.(map[string]string)
			if !ok {
				computeErr = fmt.Errorf("value is not a hash map")
				return old, false
			}

			hash := make(map[string]string, len(oldHash)+1)
			for k, v := range oldHash {
				hash[k] = v
			}

			if curr, ok := hash[field]; ok {
				i, err := strconv.Atoi(curr)
				if err != nil {
					computeErr = fmt.Errorf("failed to parse field value: %w", err)
					return old, false
				}

				i += int(incr)
				hash[field] = strconv.Itoa(i)
				resultValue = i
			} else {
				resultValue = int(incr)
				hash[field] = strconv.Itoa(resultValue)
			}

			old.Value = hash
			return old, false
		} else {
			resultValue = int(incr)
			hash := map[string]string{
				field: strconv.Itoa(resultValue),
			}
			old.Value = hash
			return old, false
		}
	})

	if computeErr != nil {
		return 0, computeErr
	}

	return resultValue, nil
}

func (c *Cache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	v, ok := c.local.Load(key)
	if !ok {
		return map[string]string{}, nil
	}

	hash, ok := v.Value.(map[string]string)
	if !ok {
		return nil, fmt.Errorf("value is not a hash map")
	}

	copy := make(map[string]string, len(hash))
	for k, v := range hash {
		copy[k] = v
	}

	return copy, nil
}

func (c *Cache) HDel(ctx context.Context, key string, field string) error {
	var computeErr error
	c.local.Compute(key, func(old Entry, found bool) (Entry, bool) {
		if found {
			oldHash, ok := old.Value.(map[string]string)
			if !ok {
				computeErr = fmt.Errorf("value is not a hash map")
				return old, false
			}

			hash := make(map[string]string, len(oldHash))
			for k, v := range oldHash {
				if k != field {
					hash[k] = v
				}
			}

			old.Value = hash
			return old, false
		}
		return old, false
	})

	return computeErr
}

func (c *Cache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	c.local.Compute(key, func(old Entry, found bool) (Entry, bool) {
		if found {
			expiry := time.Now().Add(expiration)
			old.Expiry = &expiry
			return old, false
		}
		return old, false
	})

	return nil
}
