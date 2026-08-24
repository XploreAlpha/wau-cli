// Package stack — status.go
//
// 4.1.3 (2026-08-24, v1.1.0 子项 4.1) — stack v1.1 status matrix 报告层。
//
// 跟现有 ls.go 的区别:ls 从 Runtime(ServiceState)读;StatusV11 从 stack v1.1
// 配置 + 叠加 Runtime 状态,产出矩阵(name/version/host/pid/ports/health/uptime)。
//
// 当前 ls.go 不消费 StatusReport(继续用 Runtime 直接渲染),所以本文件是
// "data layer for future v1.1-aware status display";cmd 层后续按需接入。
//
// D60 additive:不动 Runtime / ServiceState / ls.go;StatusReport 是新类型。
package stack

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

// HealthState 健康状态(由 status matrix 算)。
type HealthState string

const (
	HealthUnknown   HealthState = "unknown"   // 未跑 / 无 runtime 数据
	HealthRunning   HealthState = "running"   // PID alive + runtime says running
	HealthStarting  HealthState = "starting"  // runtime says starting
	HealthDegraded  HealthState = "degraded"  // PID alive 但 runtime says stopped
	HealthStopped   HealthState = "stopped"   // runtime says stopped
	HealthFailed    HealthState = "failed"    // runtime says failed
	HealthExternal  HealthState = "external"  // kind=external(无 process,走 probe)
)

// ServiceStatus 单服务 status 矩阵行。
type ServiceStatus struct {
	Name      string      `json:"name"`
	Kind      string      `json:"kind"`
	Binary    string      `json:"binary,omitempty"`
	PID       int         `json:"pid,omitempty"`
	Ports     []string    `json:"ports,omitempty"`
	Health    HealthState `json:"health"`
	StartedAt *time.Time  `json:"started_at,omitempty"`
	UptimeMS  int64       `json:"uptime_ms,omitempty"`
	LastError string      `json:"last_error,omitempty"`
}

// StatusReport — stack 状态矩阵聚合。
type StatusReport struct {
	StackID   string          `json:"stack_id"`
	Release   string          `json:"release,omitempty"`
	FetchedAt time.Time       `json:"fetched_at"`
	Services  []ServiceStatus `json:"services"`
	Healthy   int             `json:"healthy"`
	Degraded  int             `json:"degraded"`
	Stopped   int             `json:"stopped"`
	Failed    int             `json:"failed"`
	Total     int             `json:"total"`
}

// StatusOpts — StatusV11 的可选参数。
type StatusOpts struct {
	// ProbePorts — 是否对 binary kind 跑端口可达性探测(默认 false,纯 data 层)。
	ProbePorts bool
	// ProbeTimeout — 端口探测超时(默认 500ms)。
	ProbeTimeout time.Duration
}

// StatusV11 从 stack v1.1 配置 + Runtime 状态产出 status matrix。
//
// 算法:
//   1. 字母序遍历 stack v1.1 Services
//   2. 叠加 Runtime.Services[name] 的 PID / status / 启动时间
//   3. PID alive 检查:把 "running" 但 IsAlive(pid)=false 的标 "degraded"
//   4. kind=external 且 healthcheck.tcp 存在时跑 ProbePorts(可选)
//
// 注意:本函数不发起 SSH remote probe(那是 4.1.4+ 的活)。
func StatusV11(stack *StackV11, rt *Runtime, opts StatusOpts) (*StatusReport, error) {
	if stack == nil {
		return nil, fmt.Errorf("stack is nil")
	}
	now := time.Now()
	if opts.ProbeTimeout == 0 {
		opts.ProbeTimeout = 500 * time.Millisecond
	}

	report := &StatusReport{
		StackID:   stack.StackID,
		Release:   stack.Release,
		FetchedAt: now,
	}

	names := make([]string, 0, len(stack.Services))
	for n := range stack.Services {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		svc := stack.Services[name]
		ss := ServiceStatus{
			Name:   name,
			Kind:   string(svc.Kind),
			Binary: svc.Binary,
			Ports:  append([]string(nil), svc.Ports...),
			Health: HealthUnknown,
		}

		// 叠加 runtime state
		if rt != nil {
			if st, ok := rt.Services[name]; ok {
				ss.PID = st.PID
				ss.LastError = st.LastError
				if !st.StartedAt.IsZero() {
					t := st.StartedAt
					ss.StartedAt = &t
					ss.UptimeMS = now.Sub(st.StartedAt).Milliseconds()
				}
				// status mapping
				switch st.Status {
				case "running":
					if IsAlive(st.PID) {
						ss.Health = HealthRunning
					} else {
						ss.Health = HealthDegraded
					}
				case "starting":
					ss.Health = HealthStarting
				case "stopped":
					ss.Health = HealthStopped
				case "failed":
					ss.Health = HealthFailed
				default:
					ss.Health = HealthUnknown
				}
			}
		}
		// kind=external 但 runtime 没记录 → external
		if ss.Health == HealthUnknown && svc.Kind == KindExternal {
			ss.Health = HealthExternal
		}

		// 可选端口探测
		if opts.ProbePorts && svc.Healthcheck != nil && svc.Healthcheck.TCP != "" {
			ss.Health = probeTCP(svc.Healthcheck.TCP, opts.ProbeTimeout)
		}

		// counter
		switch ss.Health {
		case HealthRunning, HealthExternal:
			report.Healthy++
		case HealthDegraded, HealthStarting:
			report.Degraded++
		case HealthStopped, HealthUnknown:
			report.Stopped++
		case HealthFailed:
			report.Failed++
		}
		report.Services = append(report.Services, ss)
	}
	report.Total = len(report.Services)
	return report, nil
}

// probeTCP — 短超时 TCP 探测,返回 HealthRunning / HealthStopped。
func probeTCP(addr string, timeout time.Duration) HealthState {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return HealthStopped
	}
	_ = conn.Close()
	return HealthRunning
}

// String — 人类可读 matrix(给 cmd 层 table 用)。
func (r *StatusReport) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Stack: %s (release: %s)\n", r.StackID, r.Release)
	fmt.Fprintf(&b, "Total: %d  Healthy: %d  Degraded: %d  Stopped: %d  Failed: %d\n\n",
		r.Total, r.Healthy, r.Degraded, r.Stopped, r.Failed)
	b.WriteString("NAME             KIND      PID      HEALTH      UPTIME\n")
	b.WriteString("───────────────  ────────  ───────  ──────────  ──────────\n")
	for _, ss := range r.Services {
		uptime := "-"
		if ss.UptimeMS > 0 {
			uptime = (time.Duration(ss.UptimeMS) * time.Millisecond).Truncate(time.Second).String()
		}
		fmt.Fprintf(&b, "%-15s  %-8s  %-7d  %-10s  %s\n",
			ss.Name, ss.Kind, ss.PID, ss.Health, uptime)
	}
	return b.String()
}