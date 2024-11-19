# Swirl - Sliding Window Increment Rate Limit

> Sliding Window ~~Counters~~ Increment\* rate limit implementation for Go

_(\*the name ["swc"](https://swc.rs/) is already taken and who doesn't love a good backronym?)_

This is a simple rate limiter built based on [this blog post](https://www.figma.com/blog/an-alternative-approach-to-rate-limiting) from Figma's engineering team.

See the post for information about the requirements and design of the actual algorithm.

## Usage

The rate limiter satisfies this interface:

```go
Increment(ctx context.Context, key string, incr int) (*Status, bool, error)
```

- Status includes information you'd want to set in [`RateLimit` headers](https://datatracker.ietf.org/doc/draft-ietf-httpapi-ratelimit-headers/).
- Bool is whether the limit was exceeded or not, true means reject the request.
- Errors occur for cache issues, such as Redis connectivity or malformed data.

The implementation is store agnostic, however due to the way it works, Redis is the recommended approach due to the usage of [hash sets](https://redis.io/docs/latest/develop/data-types/hashes/).

The `incr` argument allows you to assign different weights to the action being rate limited. For example, a simple request may use a value of 1 and an expensive request may use a value of 10.

```go
status, exceeded, err := m.rl.Increment(ctx, key, cost)
if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    return
}

limit := status.Limit
remaining := status.Remaining
resetTime := status.Reset.UTC().Format(time.RFC1123)

w.Header().Set(RateLimitLimit, strconv.FormatUint(uint64(limit), 10))
w.Header().Set(RateLimitRemaining, strconv.FormatUint(uint64(remaining), 10))
w.Header().Set(RateLimitReset, resetTime)

if exceeded {
    // you shall not pass.
    w.Header().Set(RetryAfter, resetTime)
    http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
    return
}

// all good my g come thru
next.ServeHTTP(w, r)
```

Note that according to the IETF spec, the `X-RateLimit-*` headers are not standardised, but commonly used. See the spec for advisories on `RateLimit-Policy` etc.

## `memory`

This is a very basic in-memory cache that mirrors the tiny subset of Redis-based hash set APIs necessary to use the rate limiter in pure Go. You can probably use this in a very basic single-server application but it's not covered by tests and has not been extensively used in production so... beware. Treat it as a testing mock.

## HTTP middleware

This package, unlike most rate limit packages, purposely does not include HTTP middleware, you probably want to write your own with your own logging, response logic, etc. anyway. It's super simple and the code above gets you most of the way already.
