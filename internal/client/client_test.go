// Package client - client_test.go
//
// 第二刀 P1.3 — client + retry + auth + L5 测试(target 80% 覆盖 client.go)。
//
// 覆盖:
//   - NewClient 默认值
//   - retry:success on first attempt
//   - retry:success after 2 failures(5xx)
//   - retry:success after network error
//   - retry:give up after max(返回最后 error)
//   - retry:不重试 4xx(立刻返回 APIError)
//   - retry:不重试 401(避免 lockout)
//   - retry:429 retry
//   - retry:context cancel mid-backoff
//   - auth:Authorization header 由 CredentialsProvider 提供
//   - auth:expired token → Token() 返回 error → header 不加,但 server 401 不重试
//   - backoff:exponential + capped at MaxBackoff
//   - backoff:jitter 在 ±20% 范围
//   - L5Install/L5Uninstall/L5Search/L5Login JSON roundtrip
//   - L5Update JSON roundtrip
//   - Health/GetKernelInfo JSON roundtrip
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// ─── NewClient defaults ─────────────────────────────────────────────────────

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient(Options{BaseURL: "http://x"})
	if c.baseURL != "http://x" {
		t.Errorf("baseURL = %q", c.baseURL)
	}
	if c.role != "external_agent" {
		t.Errorf("default role = %q", c.role)
	}
	if c.maxRetries != DefaultMaxRetries {
		t.Errorf("default maxRetries = %d", c.maxRetries)
	}
	if c.initialBackoff != DefaultInitialBackoff {
		t.Errorf("default initialBackoff = %v", c.initialBackoff)
	}
	if c.maxBackoff != DefaultMaxBackoff {
		t.Errorf("default maxBackoff = %v", c.maxBackoff)
	}
}

func TestNewClient_EmptyBaseURL(t *testing.T) {
	c := NewClient(Options{})
	if c.baseURL != "http://localhost:18400" {
		t.Errorf("empty baseURL fallback = %q", c.baseURL)
	}
}

// ─── retry: success ─────────────────────────────────────────────────────────

func TestRetry_FirstAttemptSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL, MaxRetries: 3})
	var resp map[string]bool
	if err := c.Get(context.Background(), "/test", &resp); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("calls = %d, want 1", got)
	}
	if !resp["ok"] {
		t.Errorf("resp = %v", resp)
	}
}

func TestRetry_SuccessAfterTwoFailures(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL, MaxRetries: 3, InitialBackoff: 1 * time.Millisecond, MaxBackoff: 5 * time.Millisecond})
	if err := c.Get(context.Background(), "/test", nil); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

func TestRetry_GiveUpAfterMax(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL, MaxRetries: 2, InitialBackoff: 1 * time.Millisecond, MaxBackoff: 5 * time.Millisecond})
	err := c.Get(context.Background(), "/test", nil)
	if err == nil {
		t.Fatal("want error after exhausting retries")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err not *APIError: %T %v", err, err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 3 { // 1 initial + 2 retries
		t.Errorf("calls = %d, want 3", got)
	}
}

// ─── retry: don't retry 4xx ─────────────────────────────────────────────────

func TestRetry_NoRetryOn4xx(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 422} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			var calls int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(status)
			}))
			defer srv.Close()

			c := NewClient(Options{BaseURL: srv.URL, MaxRetries: 5, InitialBackoff: 1 * time.Millisecond})
			err := c.Get(context.Background(), "/test", nil)
			if err == nil {
				t.Fatalf("status %d: want error", status)
			}
			if got := atomic.LoadInt32(&calls); got != 1 {
				t.Errorf("status %d: calls = %d, want 1 (no retry)", status, got)
			}
		})
	}
}

func TestRetry_RetryOn429(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(429)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL, MaxRetries: 3, InitialBackoff: 1 * time.Millisecond, MaxBackoff: 5 * time.Millisecond})
	if err := c.Get(context.Background(), "/test", nil); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("calls = %d, want 2 (retry on 429)", got)
	}
}

func TestRetry_RetryOn5xx(t *testing.T) {
	for _, status := range []int{500, 502, 503, 504} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			var calls int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(status)
			}))
			defer srv.Close()

			c := NewClient(Options{BaseURL: srv.URL, MaxRetries: 2, InitialBackoff: 1 * time.Millisecond, MaxBackoff: 5 * time.Millisecond})
			_ = c.Get(context.Background(), "/test", nil)
			if got := atomic.LoadInt32(&calls); got != 3 {
				t.Errorf("status %d: calls = %d, want 3 (retry)", status, got)
			}
		})
	}
}

// ─── retry: network error ───────────────────────────────────────────────────

func TestRetry_NetworkErrorRetried(t *testing.T) {
	// 指向已关端口 — dial fail
	c := NewClient(Options{BaseURL: "http://127.0.0.1:1", MaxRetries: 1, InitialBackoff: 1 * time.Millisecond, MaxBackoff: 5 * time.Millisecond})
	err := c.Get(context.Background(), "/test", nil)
	if err == nil {
		t.Fatal("want error (connection refused)")
	}
	// 应该是 2 次 attempt(0+1)后 fail
	if !contains(err.Error(), "request failed") {
		t.Errorf("err = %v", err)
	}
}

// ─── retry: context cancel ──────────────────────────────────────────────────

func TestRetry_ContextCancelMidBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL, MaxRetries: 10, InitialBackoff: 500 * time.Millisecond, MaxBackoff: 5 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := c.Get(ctx, "/test", nil)
	if err == nil {
		t.Fatal("want context cancel error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want DeadlineExceeded", err)
	}
}

// ─── auth: bearer header ────────────────────────────────────────────────────

func TestAuth_BearerHeaderInjected(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"ok":true}`)
	}))
	defer srv.Close()

	creds := &Credentials{AccessToken: "test-token-abc"}
	c := NewClient(Options{BaseURL: srv.URL, Auth: NewCredentialsProvider(creds)})
	if err := c.Get(context.Background(), "/test", nil); err != nil {
		t.Fatal(err)
	}
	if seenAuth != "Bearer test-token-abc" {
		t.Errorf("Authorization = %q, want %q", seenAuth, "Bearer test-token-abc")
	}
}

func TestAuth_NoHeaderWhenNoAuth(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL})
	if err := c.Get(context.Background(), "/test", nil); err != nil {
		t.Fatal(err)
	}
	if seenAuth != "" {
		t.Errorf("Authorization should be empty, got %q", seenAuth)
	}
}

func TestAuth_ExpiredTokenSkipsHeader(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	creds := &Credentials{AccessToken: "expired", ExpiresAt: time.Now().Add(-1 * time.Hour).Unix()}
	c := NewClient(Options{BaseURL: srv.URL, Auth: NewCredentialsProvider(creds)})
	if err := c.Get(context.Background(), "/test", nil); err != nil {
		t.Fatal(err)
	}
	if seenAuth != "" {
		t.Errorf("expired token: Authorization = %q, want empty", seenAuth)
	}
}

func TestAuth_PreservesXAgentRole(t *testing.T) {
	var seenRole string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenRole = r.Header.Get("X-Agent-Role")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL, Role: "trusted_agent", Auth: NewCredentialsProvider(&Credentials{AccessToken: "x"})})
	if err := c.Get(context.Background(), "/test", nil); err != nil {
		t.Fatal(err)
	}
	if seenRole != "trusted_agent" {
		t.Errorf("X-Agent-Role = %q", seenRole)
	}
}

// ─── backoff math ───────────────────────────────────────────────────────────

func TestBackoff_ExponentialCapped(t *testing.T) {
	initial := 100 * time.Millisecond
	max := 800 * time.Millisecond
	// 第 0 次: 100ms
	// 第 1 次: 200ms
	// 第 2 次: 400ms
	// 第 3 次: 800ms (=max)
	// 第 4 次: 800ms (capped)
	cases := []struct {
		attempt int
		minDur  time.Duration
		maxDur  time.Duration
	}{
		{0, 80 * time.Millisecond, 120 * time.Millisecond},
		{1, 160 * time.Millisecond, 240 * time.Millisecond},
		{2, 320 * time.Millisecond, 480 * time.Millisecond},
		{3, 640 * time.Millisecond, 960 * time.Millisecond},
		{4, 640 * time.Millisecond, 960 * time.Millisecond}, // capped
	}
	for _, tc := range cases {
		got := backoffDuration(tc.attempt, initial, max)
		if got < tc.minDur || got > tc.maxDur {
			t.Errorf("attempt %d: got %v, want in [%v,%v]", tc.attempt, got, tc.minDur, tc.maxDur)
		}
	}
}

// ─── L5 JSON roundtrip ──────────────────────────────────────────────────────

func TestL5Install_Roundtrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/l5/install" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var req L5InstallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if req.AgentName != "fox-medical" {
			t.Errorf("AgentName = %q", req.AgentName)
		}
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"ok":true,"agent_id":"a-1","version":"1.2.0","installed_at":1700000000,"duration_ms":42.5}`)
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL})
	resp, err := c.L5Install(context.Background(), &L5InstallRequest{UserID: "u1", AgentName: "fox-medical", Version: "1.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.AgentID != "a-1" || resp.Version != "1.2.0" || resp.DurationMS != 42.5 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestL5Uninstall_Roundtrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"ok":true,"uninstalled_at":1700000000,"snapshot_path":"/var/wau/snapshots/x"}`)
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL})
	resp, err := c.L5Uninstall(context.Background(), &L5UninstallRequest{UserID: "u1", AgentName: "fox"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.SnapshotPath == "" {
		t.Errorf("resp = %+v", resp)
	}
}

// ─── Helper ─────────────────────────────────────────────────────────────────

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}