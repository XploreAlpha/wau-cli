// Package stack - health.go
//
// 第一刀 1.1 — 健康探针 3 类型实现 + 轮询逻辑。
//
// 设计原则:
//   - 每种探针独立 type,统一 Probe interface
//   - 轮询:interval 决定频率,timeout 总超时
//   - HTTP 探针:接受 2xx/3xx,4xx/5xx/timeout 都算 fail
//   - TCP 探针:dial 成功即 OK
//   - Exec 探针:exit 0 即 OK
package stack

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// ProbeResult 探针结果。
type ProbeResult struct {
	OK       bool
	Attempts int
	Elapsed  time.Duration
	Err      error
}

// ProbeRunner 探针执行器接口。
type ProbeRunner interface {
	// Run 轮询直到成功或 ctx 取消。返回 ProbeResult。
	Run(ctx context.Context) ProbeResult
	// String 描述当前探针(用于日志)
	String() string
}

// NewProbeRunner 根据 probe 定义构造对应 runner。
func NewProbeRunner(p *Probe, serviceName string) (ProbeRunner, error) {
	if p == nil {
		return nil, fmt.Errorf("nil probe for service %s", serviceName)
	}
	interval := p.Interval
	if interval == 0 {
		interval = 500 * time.Millisecond
	}
	timeout := p.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx := &probeCtx{interval: interval, timeout: timeout}
	switch p.Type {
	case ProbeTCP:
		addr := p.Addr
		if addr == "" {
			addr = fmt.Sprintf("localhost:%d", p.Port)
		} else if !strings.Contains(addr, ":") {
			addr = fmt.Sprintf("%s:%d", addr, p.Port)
		}
		return &tcpProbe{service: serviceName, addr: addr, ctx: ctx}, nil
	case ProbeHTTP:
		return &httpProbe{service: serviceName, url: p.URL, ctx: ctx, client: &http.Client{Timeout: 0}}, nil
	case ProbeExec:
		return &execProbe{service: serviceName, cmd: p.Cmd, args: p.Args, ctx: ctx}, nil
	default:
		return nil, fmt.Errorf("unknown probe type %q", p.Type)
	}
}

// probeCtx 探针轮询参数。
type probeCtx struct {
	interval time.Duration
	timeout  time.Duration
}

// tcpProbe TCP 端口探针。
type tcpProbe struct {
	service string
	addr    string
	ctx     *probeCtx
}

func (t *tcpProbe) String() string {
	return fmt.Sprintf("tcp(%s)", t.addr)
}

func (t *tcpProbe) Run(ctx context.Context) ProbeResult {
	start := time.Now()
	probeCtx, cancel := context.WithTimeout(ctx, t.ctx.timeout)
	defer cancel()
	attempts := 0
	var lastErr error
	for {
		attempts++
		select {
		case <-probeCtx.Done():
			return ProbeResult{OK: false, Attempts: attempts, Elapsed: time.Since(start), Err: lastErr}
		default:
		}
		conn, err := net.DialTimeout("tcp", t.addr, 2*time.Second)
		if err == nil {
			_ = conn.Close()
			return ProbeResult{OK: true, Attempts: attempts, Elapsed: time.Since(start)}
		}
		lastErr = err
		if probeCtx.Err() != nil {
			return ProbeResult{OK: false, Attempts: attempts, Elapsed: time.Since(start), Err: lastErr}
		}
		select {
		case <-probeCtx.Done():
			return ProbeResult{OK: false, Attempts: attempts, Elapsed: time.Since(start), Err: lastErr}
		case <-time.After(t.ctx.interval):
		}
	}
}

// httpProbe HTTP 探针。
type httpProbe struct {
	service string
	url     string
	ctx     *probeCtx
	client  *http.Client
}

func (h *httpProbe) String() string {
	return fmt.Sprintf("http(%s)", h.url)
}

func (h *httpProbe) Run(ctx context.Context) ProbeResult {
	start := time.Now()
	// 派生带 probe 自己 timeout 的 context,同时尊重 caller ctx 的取消
	probeCtx, cancel := context.WithTimeout(ctx, h.ctx.timeout)
	defer cancel()
	attempts := 0
	for {
		attempts++
		select {
		case <-probeCtx.Done():
			return ProbeResult{OK: false, Attempts: attempts, Elapsed: time.Since(start), Err: probeCtx.Err()}
		default:
		}
		req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, h.url, nil)
		if err != nil {
			// request build error — non-recoverable for this iteration, retry
		} else {
			resp, err := h.client.Do(req)
			if err != nil {
				// network error — retry
			} else {
				_ = resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					return ProbeResult{OK: true, Attempts: attempts, Elapsed: time.Since(start)}
				}
				// non-2xx/3xx — retry
			}
		}
		if probeCtx.Err() != nil {
			return ProbeResult{OK: false, Attempts: attempts, Elapsed: time.Since(start), Err: probeCtx.Err()}
		}
		select {
		case <-probeCtx.Done():
			return ProbeResult{OK: false, Attempts: attempts, Elapsed: time.Since(start), Err: probeCtx.Err()}
		case <-time.After(h.ctx.interval):
		}
	}
}

// execProbe exec 探针。
type execProbe struct {
	service string
	cmd     string
	args    []string
	ctx     *probeCtx
}

func (e *execProbe) String() string {
	if len(e.args) == 0 {
		return fmt.Sprintf("exec(%s)", e.cmd)
	}
	return fmt.Sprintf("exec(%s %s)", e.cmd, strings.Join(e.args, " "))
}

func (e *execProbe) Run(ctx context.Context) ProbeResult {
	start := time.Now()
	probeCtx, cancel := context.WithTimeout(ctx, e.ctx.timeout)
	defer cancel()
	attempts := 0
	var lastErr error
	for {
		attempts++
		select {
		case <-probeCtx.Done():
			return ProbeResult{OK: false, Attempts: attempts, Elapsed: time.Since(start), Err: probeCtx.Err()}
		default:
		}
		// exec 探针一次性,不等 interval(因为它本身就是同步命令)
		cmd := exec.CommandContext(probeCtx, e.cmd, e.args...)
		err := cmd.Run()
		if err == nil {
			return ProbeResult{OK: true, Attempts: attempts, Elapsed: time.Since(start)}
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			lastErr = fmt.Errorf("%s exit %d", e.cmd, exitErr.ExitCode())
		} else {
			lastErr = err
		}
		if probeCtx.Err() != nil {
			return ProbeResult{OK: false, Attempts: attempts, Elapsed: time.Since(start), Err: lastErr}
		}
		select {
		case <-probeCtx.Done():
			return ProbeResult{OK: false, Attempts: attempts, Elapsed: time.Since(start), Err: probeCtx.Err()}
		case <-time.After(e.ctx.interval):
		}
	}
}
