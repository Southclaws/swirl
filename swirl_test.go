package swirl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Southclaws/swirl/memory"
)

func TestRateLimit(t *testing.T) {
	t.Parallel()

	a := assert.New(t)
	r := require.New(t)
	ctx := context.Background()

	period := time.Second * 3
	expire := time.Second
	now := time.Now()
	ratelimiter := New(memory.New(), 10, period, expire)

	check := func(t *testing.T, wantStatus Status, wantExceeded bool, wantExpire time.Time) func(gotStatus *Status, gotExceeded bool, gotErr error) {
		return func(gotStatus *Status, gotExceeded bool, gotErr error) {
			t.Helper()

			r.NoError(gotErr)
			r.NotNil(gotStatus)

			a.Equal(wantExceeded, gotExceeded)

			a.Equal(wantStatus.Limit, gotStatus.Limit)
			a.Equal(wantStatus.Remaining, gotStatus.Remaining)

			a.Equal(period, gotStatus.Period)
			a.WithinDuration(wantExpire, gotStatus.Reset, time.Millisecond*10)
		}
	}

	check(t, Status{Remaining: 9, Limit: 10}, false, now.Add(period))(ratelimiter.Increment(ctx, "k", 1))
	check(t, Status{Remaining: 8, Limit: 10}, false, now.Add(period))(ratelimiter.Increment(ctx, "k", 1))
	check(t, Status{Remaining: 7, Limit: 10}, false, now.Add(period))(ratelimiter.Increment(ctx, "k", 1))
	check(t, Status{Remaining: 6, Limit: 10}, false, now.Add(period))(ratelimiter.Increment(ctx, "k", 1))
	check(t, Status{Remaining: 5, Limit: 10}, false, now.Add(period))(ratelimiter.Increment(ctx, "k", 1))
	check(t, Status{Remaining: 4, Limit: 10}, false, now.Add(period))(ratelimiter.Increment(ctx, "k", 1))
	check(t, Status{Remaining: 3, Limit: 10}, false, now.Add(period))(ratelimiter.Increment(ctx, "k", 1))
	check(t, Status{Remaining: 2, Limit: 10}, false, now.Add(period))(ratelimiter.Increment(ctx, "k", 1))
	check(t, Status{Remaining: 1, Limit: 10}, false, now.Add(period))(ratelimiter.Increment(ctx, "k", 1))
	check(t, Status{Remaining: 0, Limit: 10}, true, now.Add(period))(ratelimiter.Increment(ctx, "k", 1))

	// Wait for after the length of period (~3s) to reset the rate limit
	delay := period + time.Millisecond*10
	time.Sleep(delay)

	check(t, Status{Remaining: 10, Limit: 10}, false, time.Now().Add(period))(ratelimiter.Increment(ctx, "k", 1))
}
