package stack

import (
	"net"
	"strings"
	"testing"
	"time"
)

// helper: 构造一个最小 v1.1 stack 给 status test。
func makeTestStackV11(t *testing.T) *StackV11 {
	t.Helper()
	s, err := ParseV11([]byte(`
version: "1.1"
stack_id: "test-stack"
release: "v1.3.4"
services:
  redis:
    kind: external
  wau-core:
    binary: wau-core
    ports: ["18400:18400"]
    healthcheck: { tcp: "localhost:18400" }
  wau-agent:
    binary: wau-agent
`))
	if err != nil {
		t.Fatalf("ParseV11: %v", err)
	}
	return s
}

// TestStatusV11_NilStack — 传 nil stack 应报错。
func TestStatusV11_NilStack(t *testing.T) {
	_, err := StatusV11(nil, nil, StatusOpts{})
	if err == nil {
		t.Fatal("expected error for nil stack")
	}
}

// TestStatusV11_NoRuntime — 无 runtime 数据时,redis=external,其他=unknown。
func TestStatusV11_NoRuntime(t *testing.T) {
	s := makeTestStackV11(t)
	r, err := StatusV11(s, nil, StatusOpts{})
	if err != nil {
		t.Fatalf("StatusV11: %v", err)
	}
	if r.Total != 3 {
		t.Errorf("Total = %d, want 3", r.Total)
	}
	for _, ss := range r.Services {
		if ss.Name == "redis" {
			if ss.Health != HealthExternal {
				t.Errorf("redis Health = %q, want external", ss.Health)
			}
		} else {
			if ss.Health != HealthUnknown {
				t.Errorf("%s Health = %q, want unknown", ss.Name, ss.Health)
			}
		}
	}
}

// TestStatusV11_WithRuntime_Degraded — runtime 说 running 但 PID 已死 → degraded。
func TestStatusV11_WithRuntime_Degraded(t *testing.T) {
	s := makeTestStackV11(t)
	rt := &Runtime{
		Name:     "test-stack",
		DataDir:  "/tmp",
		Services: make(map[string]*ServiceState),
	}
	rt.Services["redis"] = &ServiceState{Name: "redis", Status: "running", PID: 99999}

	r, _ := StatusV11(s, rt, StatusOpts{})
	redis := findStatusByName(r, "redis")
	if redis == nil {
		t.Fatal("redis not in status")
	}
	// PID 99999 几乎肯定不存在 → degraded
	if redis.Health != HealthDegraded {
		t.Errorf("redis Health = %q, want degraded (pid 99999 likely dead)", redis.Health)
	}
}

// TestStatusV11_FailedState — runtime 标 failed → Health=Failed。
func TestStatusV11_FailedState(t *testing.T) {
	s := makeTestStackV11(t)
	rt := &Runtime{
		Name:     "test-stack",
		Services: make(map[string]*ServiceState),
	}
	rt.Services["wau-core"] = &ServiceState{Name: "wau-core", Status: "failed", PID: 1, LastError: "exit 1"}
	r, _ := StatusV11(s, rt, StatusOpts{})
	wc := findStatusByName(r, "wau-core")
	if wc == nil || wc.Health != HealthFailed {
		t.Errorf("wau-core Health = %v, want failed", wc)
	}
	if wc.LastError != "exit 1" {
		t.Errorf("LastError = %q", wc.LastError)
	}
}

// TestStatusV11_ProbePorts_OK — ProbePorts 开启 + 可达端口 → HealthRunning。
func TestStatusV11_ProbePorts_OK(t *testing.T) {
	s := makeTestStackV11(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("can't listen: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()
	s.Services["wau-core"].Healthcheck.TCP = addr

	r, _ := StatusV11(s, nil, StatusOpts{ProbePorts: true, ProbeTimeout: time.Second})
	wc := findStatusByName(r, "wau-core")
	if wc == nil {
		t.Fatal("wau-core missing")
	}
	if wc.Health != HealthRunning {
		t.Errorf("wau-core Health = %q, want running (port reachable)", wc.Health)
	}
}

// TestStatusV11_ProbePorts_Fail — ProbePorts 开启 + 不可达端口 → HealthStopped。
func TestStatusV11_ProbePorts_Fail(t *testing.T) {
	s := makeTestStackV11(t)
	s.Services["wau-core"].Healthcheck.TCP = "127.0.0.1:1" // 几乎肯定不可达
	r, _ := StatusV11(s, nil, StatusOpts{ProbePorts: true, ProbeTimeout: 100 * time.Millisecond})
	wc := findStatusByName(r, "wau-core")
	if wc == nil {
		t.Fatal("wau-core missing")
	}
	if wc.Health != HealthStopped {
		t.Errorf("wau-core Health = %q, want stopped", wc.Health)
	}
}

// TestStatusV11_LoadRuntime_NoFile — runtime file 不存在 → 空 Runtime 仍正常 status。
func TestStatusV11_LoadRuntime_NoFile(t *testing.T) {
	s := makeTestStackV11(t)
	tmpDir := t.TempDir()
	rt, err := LoadRuntime(tmpDir, "nonexistent-stack")
	if err != nil {
		t.Fatalf("LoadRuntime: %v", err)
	}
	if rt == nil || rt.Services == nil {
		t.Error("rt should be non-nil with empty Services")
	}
	r, _ := StatusV11(s, rt, StatusOpts{})
	if r.Total != 3 {
		t.Errorf("Total = %d, want 3", r.Total)
	}
}

// TestStatusReport_String — String() 包含 matrix + summary。
func TestStatusReport_String(t *testing.T) {
	r := &StatusReport{StackID: "x", Release: "v1.3.4", Total: 2, Healthy: 1, Stopped: 1}
	r.Services = append(r.Services,
		ServiceStatus{Name: "a", Kind: "binary", Health: HealthRunning, UptimeMS: 5000},
		ServiceStatus{Name: "b", Kind: "binary", Health: HealthStopped})
	s := r.String()
	for _, want := range []string{"x", "v1.3.4", "Total: 2", "a", "b", "Healthy: 1"} {
		if !strings.Contains(s, want) {
			t.Errorf("String() missing %q: %s", want, s)
		}
	}
}

// TestStatusV11_Counters — Healthy / Stopped / Failed 计数正确。
func TestStatusV11_Counters(t *testing.T) {
	s := makeTestStackV11(t)
	rt := &Runtime{
		Name:     "x",
		Services: make(map[string]*ServiceState),
	}
	// redis=external → Healthy
	rt.Services["redis"] = &ServiceState{Name: "redis", Status: "running", PID: 99999}
	// wau-core=failed
	rt.Services["wau-core"] = &ServiceState{Name: "wau-core", Status: "failed", PID: 1}
	// wau-agent=stopped
	rt.Services["wau-agent"] = &ServiceState{Name: "wau-agent", Status: "stopped"}

	r, _ := StatusV11(s, rt, StatusOpts{})
	// redis:running 但 pid dead → degraded
	if r.Degraded != 1 {
		t.Errorf("Degraded = %d, want 1", r.Degraded)
	}
	if r.Failed != 1 {
		t.Errorf("Failed = %d, want 1", r.Failed)
	}
	if r.Stopped != 1 {
		t.Errorf("Stopped = %d, want 1", r.Stopped)
	}
}

// helpers
func findStatusByName(r *StatusReport, name string) *ServiceStatus {
	for i := range r.Services {
		if r.Services[i].Name == name {
			return &r.Services[i]
		}
	}
	return nil
}