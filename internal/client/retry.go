// Package client - retry.go
//
// 第二刀 P1.1 — HTTP client retry + exponential backoff with jitter(per 2026-08-20 visa demo)。
//
// 设计原则:
//   - 默认 3 次重试,exponential backoff(base=500ms, max=8s)+ ±20% jitter
//   - 重试条件:
//     - 网络错误(dial fail / EOF / timeout)
//     - HTTP 5xx(服务器错误)
//     - HTTP 429(too many requests)
//     - 不重试:4xx 业务错误 / 401/403 认证错误(避免 lockout)
//   - 可通过 RequestOpts.PerAttemptTimeout / MaxRetries 覆盖
//   - 保留 X-Agent-Role 头(向后兼容老 server)
package client

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// RequestOpts 单请求的覆盖选项。
type RequestOpts struct {
	MaxRetries        int           // 0 = 用 client 默认(DefaultMaxRetries)
	InitialBackoff    time.Duration // 0 = 用 client 默认(DefaultInitialBackoff)
	MaxBackoff        time.Duration // 0 = 用 client 默认(DefaultMaxBackoff)
	PerAttemptTimeout time.Duration // 0 = 用 client 总 timeout
}

// Default constants(可被 RequestOpts 覆盖)。
const (
	DefaultMaxRetries     = 3
	DefaultInitialBackoff = 500 * time.Millisecond
	DefaultMaxBackoff     = 8 * time.Second
)

// doRequestWithRetry 在 doRequest 外层包一层 retry 循环。
//
// 调用 doRequest 一次最多得到:
//   - nil error(成功)
//   - *APIError(非 2xx)
//   - 网络错误(连接被拒 / timeout / EOF)
//
// 决策表:
//   - nil       → 立即返回
//   - 5xx/429   → retry(若 remaining > 0)
//   - 4xx 其他  → 立即返回(不重试)
//   - 网络错误 → retry
//   - context 取消 → 立即返回
func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body, v interface{}, opts RequestOpts) error {
	maxRetries := opts.MaxRetries
	if maxRetries == 0 {
		maxRetries = c.maxRetries
		if maxRetries == 0 {
			maxRetries = DefaultMaxRetries
		}
	}
	initialBackoff := opts.InitialBackoff
	if initialBackoff == 0 {
		initialBackoff = c.initialBackoff
		if initialBackoff == 0 {
			initialBackoff = DefaultInitialBackoff
		}
	}
	maxBackoff := opts.MaxBackoff
	if maxBackoff == 0 {
		maxBackoff = c.maxBackoff
		if maxBackoff == 0 {
			maxBackoff = DefaultMaxBackoff
		}
	}

	var lastErr error
	for attempt := 0; ; attempt++ {
		// 每次 attempt 都用独立 context(若指定 PerAttemptTimeout)
		attemptCtx := ctx
		if opts.PerAttemptTimeout > 0 {
			var cancel context.CancelFunc
			attemptCtx, cancel = context.WithTimeout(ctx, opts.PerAttemptTimeout)
			defer cancel()
		}
		err := c.doRequest(attemptCtx, method, path, body, v)
		if err == nil {
			return nil
		}
		lastErr = err
		if !shouldRetry(err, attempt, maxRetries) {
			return err
		}
		// 检查 caller ctx 是否取消
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("retry aborted (ctx %v): %w", ctxErr, err)
		}
		// backoff with jitter
		sleep := backoffDuration(attempt, initialBackoff, maxBackoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}
	}
	// unreachable
	return lastErr
}

// shouldRetry 决定 err 是否值得重试。
//
// 规则:
//   - *APIError:5xx 或 429 → retry;其他 → 不 retry
//   - 网络错误(非 *APIError)→ retry
//   - 已用完 attempt → 不 retry
func shouldRetry(err error, attempt, maxRetries int) bool {
	if attempt >= maxRetries {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500 || apiErr.StatusCode == http.StatusTooManyRequests
	}
	// 网络错误 / 其他 → retry
	return true
}

// backoffDuration 计算第 N 次重试前的等待时长,exponential + ±20% jitter。
//
// 公式:min(initial * 2^attempt, max) * (1 ± 0.2 random)
func backoffDuration(attempt int, initial, max time.Duration) time.Duration {
	d := initial
	for i := 0; i < attempt; i++ {
		d *= 2
		if d > max {
			d = max
			break
		}
	}
	// ±20% jitter
	jitter := 1.0 + (rand.Float64()*0.4 - 0.2) // [0.8, 1.2)
	return time.Duration(float64(d) * jitter)
}

// retryAfter 从 APIError 的 Retry-After header 解析等待时长(可选)。
//
// 不解析也可,jitter backoff 已经足够。
func retryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	ra := resp.Header.Get("Retry-After")
	if ra == "" {
		return 0
	}
	if secs, err := strconv.Atoi(ra); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(ra); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}
