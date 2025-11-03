package ratelimit

import (
	"context"
	"golang.org/x/time/rate"
	"time"
)

// RateLimiter QPS 限制器
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter 创建新的速率限制器
// qps: 每秒允许的请求数，如果为0或负数则不限制
func NewRateLimiter(qps int) *RateLimiter {
	if qps <= 0 {
		// 不限制，设置一个很大的值
		return &RateLimiter{
			limiter: rate.NewLimiter(rate.Inf, 1),
		}
	}

	// 创建令牌桶，允许短时间内的突发请求（桶大小为QPS）
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(qps), qps),
	}
}

// Wait 等待直到获得令牌，如果超时则返回错误
func (r *RateLimiter) Wait(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}

// Allow 检查是否允许当前请求，不阻塞
func (r *RateLimiter) Allow() bool {
	return r.limiter.Allow()
}

// WaitN 等待获得 n 个令牌
func (r *RateLimiter) WaitN(ctx context.Context, n int) error {
	return r.limiter.WaitN(ctx, n)
}

// AllowN 检查是否允许 n 个请求
func (r *RateLimiter) AllowN(now time.Time, n int) bool {
	return r.limiter.AllowN(now, n)
}

// SetQPS 动态设置QPS限制
func (r *RateLimiter) SetQPS(qps int) {
	if qps <= 0 {
		r.limiter.SetLimit(rate.Inf)
		r.limiter.SetBurst(1)
	} else {
		r.limiter.SetLimit(rate.Limit(qps))
		r.limiter.SetBurst(qps)
	}
}

// GetQPS 获取当前QPS限制
func (r *RateLimiter) GetQPS() int {
	limit := r.limiter.Limit()
	if limit == rate.Inf {
		return 0 // 表示无限制
	}
	return int(limit)
}
