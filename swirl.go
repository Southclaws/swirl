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
	limitPeriod   time.Duration // 1 hour for example
	counterWindow time.Duration // 1 minute for example, 1/60 of the period
}

func New(store Store, limit int, period, counterWindow time.Duration) *Limiter {
	if counterWindow <= 0 {
		counterWindow = period / 60
		if counterWindow < time.Second {
			counterWindow = time.Second
		}
	}
	if counterWindow > period {
		counterWindow = period
	}

	return &Limiter{
		store:         store,
		limit:         limit,
		limitPeriod:   period,
		counterWindow: counterWindow,
	}
}

func (l *Limiter) Increment(ctx context.Context, key string, incr int) (status *Status, allowed bool, err error) {
	now := time.Now()
	timestamp := now.Truncate(l.counterWindow).Unix()
	timestampStr := strconv.FormatInt(timestamp, 10)

	_, err = l.store.HIncrBy(ctx, key, timestampStr, int64(incr))
	if err != nil {
		return nil, false, err
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
	thresholdTime := now.Add(-l.limitPeriod).Unix()

	// NOTE: This sums ALL the values in the hash, for more information, see the
	// "Practical Considerations" section of the associated Figma blog post.
	total := 0
	var oldestBucketTime int64 = timestamp
	foundInWindow := false
	toDelete := []string{}

	for k, v := range vals {
		bucketTime, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return nil, false, err
		}

		if bucketTime > thresholdTime {
			i, err := strconv.Atoi(v)
			if err != nil {
				return nil, false, err
			}
			total += i

			// track the oldest bucket still in the window
			if !foundInWindow || bucketTime < oldestBucketTime {
				oldestBucketTime = bucketTime
				foundInWindow = true
			}
		} else {
			toDelete = append(toDelete, k)
		}
	}

	for _, k := range toDelete {
		if err = l.store.HDel(ctx, key, k); err != nil {
			return nil, false, err
		}
	}

	// Calculate when the oldest bucket will expire (slide out of the window.)
	resetTime := time.Unix(oldestBucketTime, 0).Add(l.limitPeriod)

	if total >= l.limit {
		return &Status{
			Remaining: 0,
			Limit:     l.limit,
			Period:    l.limitPeriod,
			Reset:     resetTime,
		}, false, nil
	}

	// allowed
	return &Status{
		Remaining: l.limit - total,
		Limit:     l.limit,
		Period:    l.limitPeriod,
		Reset:     resetTime,
	}, true, nil
}
