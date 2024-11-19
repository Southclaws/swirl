package swirl

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

type Status struct {
	Remaining int
	Limit     int
	Period    time.Duration
	Reset     time.Time
}

func (e *Status) Error() string {
	return fmt.Sprintf(
		"rate limit of %d per %v has been exceeded and resets at %v",
		e.Limit, e.Period, e.Reset)
}

type Limiter struct {
	store         Store
	limit         int
	limitPeriod   time.Duration // 1 hour for exanple
	counterWindow time.Duration // 1 minute for example, 1/60 of the period
}

func New(store Store, limit int, period, expiry time.Duration) *Limiter {
	return &Limiter{
		store:         store,
		limit:         limit,
		limitPeriod:   period,
		counterWindow: expiry,
	}
}

func (l *Limiter) Increment(ctx context.Context, key string, incr int) (*Status, bool, error) {
	now := time.Now()
	timestamp := fmt.Sprint(now.Truncate(l.counterWindow).Unix())

	val, err := l.store.HIncrBy(ctx, key, timestamp, int64(incr))
	if err != nil {
		return nil, false, err
	}

	// check if current window has exceeded the limit
	if val >= l.limit {
		// Otherwise, check if just this fixed window counter period is over
		return &Status{
			Remaining: 0,
			Limit:     l.limit,
			Period:    l.limitPeriod,
			Reset:     now.Add(l.limitPeriod),
		}, true, nil
	}

	// create or move whole limit period window expiry
	err = l.store.Expire(ctx, key, l.limitPeriod)
	if err != nil {
		return nil, false, err
	}

	// Get all the bucket values and sum them.
	vals, err := l.store.HGetAll(ctx, key)
	if err != nil {
		return nil, false, err
	}

	// The time to start summing from, any buckets before this are ignored.
	threshold := fmt.Sprint(now.Add(-l.limitPeriod).Unix())

	// NOTE: This sums ALL the values in the hash, for more information, see the
	// "Practical Considerations" section of the associated Figma blog post.
	total := 0
	for k, v := range vals {
		if k > threshold {
			i, _ := strconv.Atoi(v)
			total += i
		} else {
			// Clear the old hash keys
			if err = l.store.HDel(ctx, key, k); err != nil {
				return nil, false, err
			}
		}
	}

	// exceeded
	if total >= int(l.limit) {
		return &Status{
			Remaining: 0,
			Limit:     l.limit,
			Period:    l.limitPeriod,
			Reset:     now.Add(l.limitPeriod),
		}, true, nil
	}

	// not exceeded
	return &Status{
		Remaining: int(l.limit) - total,
		Limit:     l.limit,
		Period:    l.limitPeriod,
		Reset:     now.Add(l.limitPeriod),
	}, false, nil
}
