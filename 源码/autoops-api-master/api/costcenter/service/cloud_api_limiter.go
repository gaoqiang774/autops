package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const (
	cloudAPIDefaultInterval = 300 * time.Millisecond
	cloudAPIMaxAttempts     = 4
	cloudAPIRetryBaseDelay  = time.Second
	cloudAPIRetryMaxDelay   = 8 * time.Second
)

type cloudAPILimiter struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}

type cloudAPIRetryPolicy struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	jitter      bool
}

var cloudAPILimiters sync.Map

var defaultCloudAPIRetryPolicy = cloudAPIRetryPolicy{
	maxAttempts: cloudAPIMaxAttempts,
	baseDelay:   cloudAPIRetryBaseDelay,
	maxDelay:    cloudAPIRetryMaxDelay,
	jitter:      true,
}

func cloudAPILimiterFor(provider, credentialID string) *cloudAPILimiter {
	digest := sha256.Sum256([]byte(strings.TrimSpace(provider) + "\x00" + strings.TrimSpace(credentialID)))
	key := normalizeOperator(provider) + ":" + hex.EncodeToString(digest[:])
	if existing, ok := cloudAPILimiters.Load(key); ok {
		return existing.(*cloudAPILimiter)
	}
	created := &cloudAPILimiter{interval: cloudAPIDefaultInterval}
	actual, _ := cloudAPILimiters.LoadOrStore(key, created)
	return actual.(*cloudAPILimiter)
}

func (l *cloudAPILimiter) Wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if l == nil || l.interval <= 0 {
		return nil
	}

	l.mu.Lock()
	now := time.Now()
	reservedAt := now
	if l.next.After(now) {
		reservedAt = l.next
	}
	l.next = reservedAt.Add(l.interval)
	l.mu.Unlock()

	wait := time.Until(reservedAt)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func callCloudAPI[T any](ctx context.Context, limiter *cloudAPILimiter, call func() (T, error)) (T, error) {
	return callCloudAPIWithPolicy(ctx, limiter, defaultCloudAPIRetryPolicy, call)
}

func callCloudAPIWithPolicy[T any](ctx context.Context, limiter *cloudAPILimiter, policy cloudAPIRetryPolicy, call func() (T, error)) (T, error) {
	var zero T
	if policy.maxAttempts <= 0 {
		policy.maxAttempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= policy.maxAttempts; attempt++ {
		if err := limiter.Wait(ctx); err != nil {
			return zero, err
		}
		value, err := call()
		if err == nil {
			return value, nil
		}
		lastErr = err
		if !isCloudAPIRateLimitError(err) || attempt == policy.maxAttempts {
			break
		}
		if err := waitCloudAPIRetry(ctx, cloudAPIRetryDelay(policy, attempt)); err != nil {
			return zero, err
		}
	}
	if isCloudAPIRateLimitError(lastErr) && policy.maxAttempts > 1 {
		return zero, fmt.Errorf("云厂商 API 限流，重试 %d 次后仍失败: %w", policy.maxAttempts, lastErr)
	}
	return zero, lastErr
}

func isCloudAPIRateLimitError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"requestlimitexceeded",
		"request limit exceeded",
		"throttling",
		"throttled",
		"throttle",
		"too many requests",
		"toomanyrequests",
		"flowcontrol",
		"flow control",
		"flowlimitexceeded",
		"flow limit",
		"rate exceeded",
		"rate limit",
		"ratelimit",
		"qps limit",
		"http 429",
		"http code 429",
		"status code 429",
		"statuscode=429",
		"status_code\":429",
		"频率上限",
		"请求过于频繁",
		"访问频率",
		"超过了每秒",
		"限流",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func cloudAPIRetryDelay(policy cloudAPIRetryPolicy, attempt int) time.Duration {
	delay := policy.baseDelay
	if delay <= 0 {
		return 0
	}
	for step := 1; step < attempt; step++ {
		if policy.maxDelay > 0 && delay >= policy.maxDelay/2 {
			delay = policy.maxDelay
			break
		}
		delay *= 2
	}
	if policy.maxDelay > 0 && delay > policy.maxDelay {
		delay = policy.maxDelay
	}
	if policy.jitter && delay > 4 {
		delay += time.Duration(rand.Int63n(int64(delay / 4)))
	}
	return delay
}

func waitCloudAPIRetry(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
