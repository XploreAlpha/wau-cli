package stack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPProbe_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	probe, err := NewProbeRunner(&Probe{
		Type:     ProbeHTTP,
		URL:      ts.URL,
		Interval: 50 * time.Millisecond,
		Timeout:  2 * time.Second,
	}, "test")
	if err != nil {
		t.Fatalf("NewProbeRunner: %v", err)
	}
	result := probe.Run(context.Background())
	if !result.OK {
		t.Errorf("probe failed: %v (attempts=%d)", result.Err, result.Attempts)
	}
}

func TestHTTPProbe_FailThenOK(t *testing.T) {
	// Server returns 503 first 2 times, then 200
	count := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	probe, err := NewProbeRunner(&Probe{
		Type:     ProbeHTTP,
		URL:      ts.URL,
		Interval: 50 * time.Millisecond,
		Timeout:  2 * time.Second,
	}, "test")
	if err != nil {
		t.Fatalf("NewProbeRunner: %v", err)
	}
	result := probe.Run(context.Background())
	if !result.OK {
		t.Errorf("probe should succeed after retries: %v", result.Err)
	}
	if result.Attempts < 3 {
		t.Errorf("attempts=%d, want >=3", result.Attempts)
	}
}

func TestHTTPProbe_Timeout(t *testing.T) {
	// 永远不响应
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer ts.Close()

	probe, err := NewProbeRunner(&Probe{
		Type:     ProbeHTTP,
		URL:      ts.URL,
		Interval: 100 * time.Millisecond,
		Timeout:  500 * time.Millisecond,
	}, "test")
	if err != nil {
		t.Fatalf("NewProbeRunner: %v", err)
	}
	result := probe.Run(context.Background())
	if result.OK {
		t.Error("probe should timeout")
	}
	if result.Err == nil {
		t.Error("expected timeout error")
	}
}

func TestHTTPProbe_4xxIsFail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	probe, err := NewProbeRunner(&Probe{
		Type:     ProbeHTTP,
		URL:      ts.URL,
		Interval: 50 * time.Millisecond,
		Timeout:  200 * time.Millisecond,
	}, "test")
	if err != nil {
		t.Fatalf("NewProbeRunner: %v", err)
	}
	result := probe.Run(context.Background())
	if result.OK {
		t.Error("4xx should be failure")
	}
}

func TestTCPProbe_OK(t *testing.T) {
	// 起一个 httptest server 当 TCP target
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	probe, err := NewProbeRunner(&Probe{
		Type:     ProbeTCP,
		Addr:     ts.Listener.Addr().String(),
		Interval: 50 * time.Millisecond,
		Timeout:  2 * time.Second,
	}, "test")
	if err != nil {
		t.Fatalf("NewProbeRunner: %v", err)
	}
	result := probe.Run(context.Background())
	if !result.OK {
		t.Errorf("tcp probe should succeed: %v", result.Err)
	}
}

func TestTCPProbe_Fail(t *testing.T) {
	probe, err := NewProbeRunner(&Probe{
		Type:     ProbeTCP,
		Port:     1, // 几乎不可达的端口
		Interval: 50 * time.Millisecond,
		Timeout:  300 * time.Millisecond,
	}, "test")
	if err != nil {
		t.Fatalf("NewProbeRunner: %v", err)
	}
	result := probe.Run(context.Background())
	if result.OK {
		t.Error("tcp probe to port 1 should fail")
	}
}

func TestExecProbe_OK(t *testing.T) {
	probe, err := NewProbeRunner(&Probe{
		Type:     ProbeExec,
		Cmd:      "true",
		Interval: 50 * time.Millisecond,
		Timeout:  1 * time.Second,
	}, "test")
	if err != nil {
		t.Fatalf("NewProbeRunner: %v", err)
	}
	result := probe.Run(context.Background())
	if !result.OK {
		t.Errorf("exec probe should succeed: %v", result.Err)
	}
}

func TestExecProbe_Fail(t *testing.T) {
	probe, err := NewProbeRunner(&Probe{
		Type:     ProbeExec,
		Cmd:      "false",
		Interval: 50 * time.Millisecond,
		Timeout:  500 * time.Millisecond,
	}, "test")
	if err != nil {
		t.Fatalf("NewProbeRunner: %v", err)
	}
	result := probe.Run(context.Background())
	if result.OK {
		t.Error("exec probe should fail for `false`")
	}
}

func TestNewProbeRunner_BadType(t *testing.T) {
	_, err := NewProbeRunner(&Probe{Type: "udp"}, "test")
	if err == nil {
		t.Error("ProbeType=udp should fail validation")
	}
}
