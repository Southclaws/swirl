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
	ratelimiter := New(memory.New(), 10, period, expire)

	checkRemaining := func(t *testing.T, wantRemaining int, wantAllowed bool) func(gotStatus *Status, gotAllowed bool, gotErr error) {
		return func(gotStatus *Status, gotAllowed bool, gotErr error) {
			t.Helper()

			r.NoError(gotErr)
			r.NotNil(gotStatus)

			a.Equal(wantAllowed, gotAllowed)
			a.Equal(wantRemaining, gotStatus.Remaining)
			a.Equal(10, gotStatus.Limit)
			a.Equal(period, gotStatus.Period)
		}
	}

	checkRemaining(t, 9, true)(ratelimiter.Increment(ctx, "k", 1))
	checkRemaining(t, 8, true)(ratelimiter.Increment(ctx, "k", 1))
	checkRemaining(t, 7, true)(ratelimiter.Increment(ctx, "k", 1))
	checkRemaining(t, 6, true)(ratelimiter.Increment(ctx, "k", 1))
	checkRemaining(t, 5, true)(ratelimiter.Increment(ctx, "k", 1))
	checkRemaining(t, 4, true)(ratelimiter.Increment(ctx, "k", 1))
	checkRemaining(t, 3, true)(ratelimiter.Increment(ctx, "k", 1))
	checkRemaining(t, 2, true)(ratelimiter.Increment(ctx, "k", 1))
	checkRemaining(t, 1, true)(ratelimiter.Increment(ctx, "k", 1))

	checkRemaining(t, 0, false)(ratelimiter.Increment(ctx, "k", 1))

	checkRemaining(t, 0, false)(ratelimiter.Increment(ctx, "k", 1))

	delay := period + time.Millisecond*10
	time.Sleep(delay)

	checkRemaining(t, 9, true)(ratelimiter.Increment(ctx, "k", 1))
}

func TestSingleBucketShouldNotExceedLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	limit := 100
	period := time.Second * 2
	counterWindow := time.Second * 1

	ratelimiter := New(memory.New(), limit, period, counterWindow)

	for i := 0; i < 60; i++ {
		_, allowed, err := ratelimiter.Increment(ctx, "user1", 1)
		require.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i+1)
	}

	time.Sleep(counterWindow + time.Millisecond*10)

	for i := 0; i < 50; i++ {
		_, allowed, err := ratelimiter.Increment(ctx, "user1", 1)
		require.NoError(t, err)

		currentTotal := 60 + i + 1

		if currentTotal < limit {
			assert.True(t, allowed, "request %d (total %d) should be allowed", i+1, currentTotal)
		} else {
			assert.False(t, allowed, "request %d (total %d) should be DENIED - total >= limit of %d", i+1, currentTotal, limit)
		}
	}
}

func TestConcurrentIncrement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	limit := 100
	period := time.Second * 5
	counterWindow := time.Second * 1

	ratelimiter := New(memory.New(), limit, period, counterWindow)

	const goroutines = 10
	const incrPerGoroutine = 15

	results := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			_, allowed, err := ratelimiter.Increment(ctx, "user1", incrPerGoroutine)
			require.NoError(t, err)
			results <- allowed
		}()
	}

	allowedCount := 0
	for i := 0; i < goroutines; i++ {
		if <-results {
			allowedCount++
		}
	}

	assert.LessOrEqual(t, allowedCount, 7, "too many concurrent requests allowed due to race condition")

	status, _, err := ratelimiter.Increment(ctx, "user1", 0)
	require.NoError(t, err)

	totalUsed := limit - status.Remaining
	assert.LessOrEqual(t, totalUsed, limit, "total usage should never exceed limit")
}
