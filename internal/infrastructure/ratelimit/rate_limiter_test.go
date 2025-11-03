package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_Basic(t *testing.T) {
	// 创建QPS为2的限制器
	limiter := NewRateLimiter(2)

	// 测试获取QPS
	if qps := limiter.GetQPS(); qps != 2 {
		t.Errorf("expected QPS 2, got %d", qps)
	}

	// 测试Allow方法
	if !limiter.Allow() {
		t.Error("first request should be allowed")
	}
}

func TestRateLimiter_NoLimit(t *testing.T) {
	// 创建无限制的限制器
	limiter := NewRateLimiter(0)

	// 测试获取QPS
	if qps := limiter.GetQPS(); qps != 0 {
		t.Errorf("expected QPS 0 (unlimited), got %d", qps)
	}

	// 测试连续请求都应该被允许
	for i := 0; i < 100; i++ {
		if !limiter.Allow() {
			t.Error("unlimited limiter should allow all requests")
		}
	}
}

func TestRateLimiter_SetQPS(t *testing.T) {
	limiter := NewRateLimiter(10)

	// 修改QPS
	limiter.SetQPS(20)
	if qps := limiter.GetQPS(); qps != 20 {
		t.Errorf("expected QPS 20 after SetQPS, got %d", qps)
	}

	// 设置为无限制
	limiter.SetQPS(0)
	if qps := limiter.GetQPS(); qps != 0 {
		t.Errorf("expected QPS 0 after SetQPS(0), got %d", qps)
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	limiter := NewRateLimiter(1) // 每秒1个请求

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 第一个请求应该立即通过
	start := time.Now()
	err := limiter.Wait(ctx)
	if err != nil {
		t.Errorf("first wait should not error: %v", err)
	}

	// 第二个请求应该需要等待约1秒
	err = limiter.Wait(ctx)
	duration := time.Since(start)
	if err != nil {
		t.Errorf("second wait should not error: %v", err)
	}

	// 应该等待了大约1秒（允许一些误差）
	if duration < 900*time.Millisecond || duration > 1100*time.Millisecond {
		t.Errorf("expected wait duration around 1s, got %v", duration)
	}
}
