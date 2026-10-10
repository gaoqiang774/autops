package service

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCloudAPILimiterSpacesConcurrentRequests(t *testing.T) {
	const interval = 15 * time.Millisecond
	limiter := &cloudAPILimiter{interval: interval}
	times := make([]time.Time, 4)
	var wg sync.WaitGroup
	for i := range times {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			if err := limiter.Wait(t.Context()); err != nil {
				t.Errorf("Wait() error = %v", err)
				return
			}
			times[index] = time.Now()
		}(i)
	}
	wg.Wait()
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	for i := 1; i < len(times); i++ {
		if gap := times[i].Sub(times[i-1]); gap < interval-3*time.Millisecond {
			t.Fatalf("request gap %d = %v, want at least %v", i, gap, interval-3*time.Millisecond)
		}
	}
}

func TestCallCloudAPIRetriesRateLimitErrors(t *testing.T) {
	limiter := &cloudAPILimiter{interval: time.Nanosecond}
	policy := cloudAPIRetryPolicy{
		maxAttempts: 3,
		baseDelay:   time.Millisecond,
		maxDelay:    2 * time.Millisecond,
	}
	var calls atomic.Int32
	value, err := callCloudAPIWithPolicy(t.Context(), limiter, policy, func() (string, error) {
		if calls.Add(1) < 3 {
			return "", errors.New("[TencentCloudSDKError] Code=RequestLimitExceeded, Message=超过了每秒频率上限")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("callCloudAPIWithPolicy() error = %v", err)
	}
	if value != "ok" || calls.Load() != 3 {
		t.Fatalf("value/calls = %q/%d, want ok/3", value, calls.Load())
	}
}

func TestCallCloudAPIDoesNotRetryPermanentErrors(t *testing.T) {
	limiter := &cloudAPILimiter{interval: time.Nanosecond}
	policy := cloudAPIRetryPolicy{maxAttempts: 4, baseDelay: time.Millisecond}
	var calls atomic.Int32
	_, err := callCloudAPIWithPolicy(t.Context(), limiter, policy, func() (struct{}, error) {
		calls.Add(1)
		return struct{}{}, errors.New("InvalidAccessKeyId")
	})
	if err == nil || calls.Load() != 1 {
		t.Fatalf("error/calls = %v/%d, want permanent error after one call", err, calls.Load())
	}
}

func TestCloudAPILimiterHonorsContextCancellation(t *testing.T) {
	limiter := &cloudAPILimiter{interval: 200 * time.Millisecond}
	if err := limiter.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if err := limiter.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait() error = %v, want context deadline exceeded", err)
	}
}

func TestCloudAPIRateLimitErrorMarkers(t *testing.T) {
	for _, message := range []string{
		"Code=RequestLimitExceeded",
		"Throttling.User: request was throttled",
		"HTTP 429 Too Many Requests",
		"FlowControl: QPS limit exceeded",
		"FlowLimitExceeded",
		`{"status_code":429}`,
		"请求过于频繁，请稍后重试",
	} {
		if !isCloudAPIRateLimitError(errors.New(message)) {
			t.Fatalf("expected rate-limit classification for %q", message)
		}
	}
	if isCloudAPIRateLimitError(errors.New("InvalidAccessKeyId")) {
		t.Fatal("credential error must not be classified as rate limiting")
	}
}

func TestCloudAPILimiterRegistrySharesProviderCredential(t *testing.T) {
	first := cloudAPILimiterFor("tencentcloud", "credential-a")
	second := cloudAPILimiterFor("tencentcloud", "credential-a")
	otherProvider := cloudAPILimiterFor("aliyun", "credential-a")
	if first != second {
		t.Fatal("same provider credential should share one limiter")
	}
	if first == otherProvider {
		t.Fatal("different providers should not share one limiter")
	}
}

func TestCloudBillClientsInstallSharedRateLimiter(t *testing.T) {
	tencentClient := newRateLimitedTencentBillClient(fakeTencentBillClient{}, "credential-tencent")
	if _, ok := tencentClient.(*rateLimitedTencentBillClient); !ok {
		t.Fatalf("Tencent client type = %T", tencentClient)
	}

	baiduClient, err := newBaiduBillClient("credential-baidu", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := baiduClient.(*rateLimitedBaiduBillClient); !ok {
		t.Fatalf("Baidu client type = %T", baiduClient)
	}

	volcengineClient := newVolcengineBillClient("credential-volcengine", "secret")
	if _, ok := volcengineClient.(*rateLimitedVolcengineBillClient); !ok {
		t.Fatalf("Volcengine client type = %T", volcengineClient)
	}
}
