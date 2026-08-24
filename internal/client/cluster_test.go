package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeKernel 启一个 httptest server,handle /health + /kernel/info + /registry/agents。
//
// 可配 fail map 让指定 endpoint 返回 5xx。
type fakeKernel struct {
	healthOK   bool
	kernelOK   bool
	agentsOK   bool
	agentsBody string // raw JSON body for /registry/agents
	healthBody string
	kernelBody string
}

func newFakeKernel(fk *fakeKernel) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if !fk.healthOK {
			http.Error(w, "boom", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fk.healthBody))
	})
	mux.HandleFunc("/kernel/info", func(w http.ResponseWriter, r *http.Request) {
		if !fk.kernelOK {
			http.Error(w, "boom", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fk.kernelBody))
	})
	mux.HandleFunc("/registry/agents", func(w http.ResponseWriter, r *http.Request) {
		if !fk.agentsOK {
			http.Error(w, "boom", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fk.agentsBody))
	})
	return httptest.NewServer(mux)
}

func TestClusterStatus_AllOK(t *testing.T) {
	srv := newFakeKernel(&fakeKernel{
		healthOK: true,
		kernelOK: true,
		agentsOK: true,
		healthBody: `{"status":"ok","version":"v0.5.1","uptime":100.5,"redis":"connected"}`,
		kernelBody: `{"version":"v0.5.1","startTime":"2026-07-15T14:33:47Z","modules":["scheduler","registry"]}`,
		agentsBody: `[{"id":"matwau","name":"matwau","url":"http://x","skills":["multi_agent"]}]`,
	})
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL, Role: "external_agent"})
	st, err := c.GetClusterStatus(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if st.Health == nil || st.Health.Version != "v0.5.1" {
		t.Errorf("Health missing or wrong: %+v", st.Health)
	}
	if st.Kernel == nil || st.Kernel.Version != "v0.5.1" {
		t.Errorf("Kernel missing or wrong: %+v", st.Kernel)
	}
	if len(st.Modules) != 2 {
		t.Errorf("Modules = %v, want 2", st.Modules)
	}
	if st.AgentsTotal != 1 {
		t.Errorf("AgentsTotal = %d, want 1", st.AgentsTotal)
	}
	if st.HealthErr != nil || st.KernelErr != nil || st.AgentsErr != nil {
		t.Errorf("unexpected partial errs: h=%v k=%v a=%v", st.HealthErr, st.KernelErr, st.AgentsErr)
	}
}

func TestClusterStatus_HealthFail(t *testing.T) {
	srv := newFakeKernel(&fakeKernel{
		healthOK:   false, // 500
		kernelOK:   true,
		agentsOK:   true,
		kernelBody: `{"version":"v0.5.1","modules":["x"]}`,
		agentsBody: `[{"id":"a"}]`,
	})
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL})
	st, err := c.GetClusterStatus(context.Background())
	if err != nil {
		t.Fatalf("partial err expected nil, got %v", err)
	}
	if st.Health != nil {
		t.Error("Health should be nil after /health 500")
	}
	if st.HealthErr == nil {
		t.Error("HealthErr should be set")
	}
	if st.Kernel == nil || st.AgentsTotal == 0 {
		t.Error("kernel + agents should still succeed (partial)")
	}
}

func TestClusterStatus_KernelFail(t *testing.T) {
	srv := newFakeKernel(&fakeKernel{
		healthOK:   true,
		kernelOK:   false, // 500
		agentsOK:   true,
		healthBody: `{"status":"ok","version":"v0.5.1","uptime":10,"redis":"ok"}`,
		agentsBody: `[{"id":"a"}]`,
	})
	defer srv.Close()
	c := NewClient(Options{BaseURL: srv.URL})
	st, err := c.GetClusterStatus(context.Background())
	if err != nil {
		t.Fatalf("partial err expected nil, got %v", err)
	}
	if st.Kernel != nil {
		t.Error("Kernel should be nil")
	}
	if st.Modules != nil {
		t.Error("Modules should be nil")
	}
	if st.KernelErr == nil {
		t.Error("KernelErr should be set")
	}
}

func TestClusterStatus_AllFail(t *testing.T) {
	srv := newFakeKernel(&fakeKernel{
		healthOK: false, kernelOK: false, agentsOK: false,
	})
	defer srv.Close()
	c := NewClient(Options{BaseURL: srv.URL})
	st, err := c.GetClusterStatus(context.Background())
	if err == nil {
		t.Fatal("want err when all 3 endpoints fail")
	}
	if st == nil {
		t.Fatal("want partial status even when all fail")
	}
	if st.HealthErr == nil || st.KernelErr == nil || st.AgentsErr == nil {
		t.Error("all 3 partial errs should be set")
	}
}

func TestClusterStatus_AgentsRawArray(t *testing.T) {
	// 验证 live server 行为:/registry/agents 返回 raw array 不是 object
	srv := newFakeKernel(&fakeKernel{
		healthOK:   true,
		kernelOK:   true,
		agentsOK:   true,
		healthBody: `{"status":"ok","version":"v0.5.1","uptime":10,"redis":"ok"}`,
		kernelBody: `{"version":"v0.5.1"}`,
		agentsBody: `[{"id":"a1"},{"id":"a2"},{"id":"a3"}]`,
	})
	defer srv.Close()
	c := NewClient(Options{BaseURL: srv.URL})
	st, _ := c.GetClusterStatus(context.Background())
	if st.AgentsTotal != 3 {
		t.Errorf("AgentsTotal = %d, want 3 (from raw array len)", st.AgentsTotal)
	}
}

func TestClusterStatus_AgentsObjectFormat(t *testing.T) {
	// server 改用 object 格式 → total 字段也工作
	srv := newFakeKernel(&fakeKernel{
		healthOK:   true,
		kernelOK:   true,
		agentsOK:   true,
		healthBody: `{"status":"ok","version":"v0.5.1","uptime":10,"redis":"ok"}`,
		kernelBody: `{"version":"v0.5.1"}`,
		agentsBody: `{"agents":[{"id":"a1"},{"id":"a2"}],"total":42,"page":1,"pageSize":2}`,
	})
	defer srv.Close()
	c := NewClient(Options{BaseURL: srv.URL})
	st, _ := c.GetClusterStatus(context.Background())
	if st.AgentsTotal != 42 {
		t.Errorf("AgentsTotal = %d, want 42 (from object total)", st.AgentsTotal)
	}
}

func TestListAgentsRaw_RawArray(t *testing.T) {
	// 单独测 listAgentsRaw
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/registry/agents" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{"id": "1"},
			{"id": "2"},
		})
	}))
	defer srv.Close()
	c := NewClient(Options{BaseURL: srv.URL})
	agents, total, err := c.ListAgentsRaw(context.Background(), 1, 10, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(agents) != 2 {
		t.Errorf("agents=%d total=%d, want 2/2", len(agents), total)
	}
}