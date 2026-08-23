// Package cmd - node_test.go
//
// 第二刀 P2.5 — `wau node ls` + `wau node info` 测试。
//
// 覆盖:
//   - `wau node ls` happy path(table format)
//   - `wau node ls` JSON format
//   - `wau node info <name>` happy path
//   - `wau node info` 缺参数
//   - `wau peer ls` alias 走相同代码路径
//   - kernel 报 5xx → 错误透传
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeKernelServer 起一个 httptest server,模拟 wau-core-kernel 的
// /registry/agents + /registry/agents/{name}/status 路由
type fakeKernelServer struct {
	*httptest.Server
	agents []map[string]interface{}
}

// 把 fakeKernelServer 接到 wau-cli kernelAddr 全局变量
func withFakeKernel(srv *httptest.Server) func() {
	old := kernelAddr
	kernelAddr = srv.URL
	return func() { kernelAddr = old }
}

func newFakeKernel() *fakeKernelServer {
	srv := &fakeKernelServer{}
	srv.agents = []map[string]interface{}{
		{
			"name": "fox-medical", "id": "a-1", "url": "http://10.0.0.1:18800",
			"description": "medical fox agent",
			"skills":      []string{"medical", "clinical"},
			"universes":   []string{"medical"},
			"trust":       0.92, "status": "online", "lastSeen": "2026-08-20T00:00:00Z",
		},
		{
			"name": "chinese-medicine", "id": "a-2", "url": "http://10.0.0.2:18801",
			"description": "tcm agent",
			"skills":      []string{"medical", "tcm"},
			"universes":   []string{"medical"},
			"trust":       0.78, "status": "online", "lastSeen": "2026-08-20T00:00:00Z",
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/registry/agents", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"agents":     srv.agents,
			"total":      len(srv.agents),
			"page":       1,
			"pageSize":   100,
			"totalPages": 1,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/registry/agents/fox-medical/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"name":"fox-medical","status":"online","trust":0.92,"circuit":"closed","load":{"activeTasks":1,"maxCapacity":10,"cpuUsage":12.5,"memoryUsage":33.3}}`)
	})
	srv.Server = httptest.NewServer(mux)
	return srv
}

func (srv *fakeKernelServer) URL() string { return srv.Server.URL }

// ─── node ls ────────────────────────────────────────────────────────────────

func TestNodeLs_HappyPath(t *testing.T) {
	srv := newFakeKernel()
	defer srv.Close()
	restore := withFakeKernel(srv.Server)
	defer restore()

	rootCmd.SetArgs([]string{"node", "ls"})
	rootCmd.SetOut(&strings.Builder{})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNodeLs_JSON(t *testing.T) {
	srv := newFakeKernel()
	defer srv.Close()
	restore := withFakeKernel(srv.Server)
	defer restore()

	rootCmd.SetArgs([]string{"node", "ls", "-o", "json"})
	rootCmd.SetOut(&strings.Builder{})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPeerLs_Alias(t *testing.T) {
	srv := newFakeKernel()
	defer srv.Close()
	restore := withFakeKernel(srv.Server)
	defer restore()

	rootCmd.SetArgs([]string{"peer", "ls"})
	rootCmd.SetOut(&strings.Builder{})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNodeLs_KernelError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	restore := withFakeKernel(srv)
	defer restore()

	rootCmd.SetArgs([]string{"node", "ls"})
	rootCmd.SetOut(&strings.Builder{})
	if err := rootCmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("want error when kernel returns 500")
	}
}

// ─── node info ──────────────────────────────────────────────────────────────

func TestNodeInfo_HappyPath(t *testing.T) {
	srv := newFakeKernel()
	defer srv.Close()
	restore := withFakeKernel(srv.Server)
	defer restore()

	rootCmd.SetArgs([]string{"node", "info", "fox-medical"})
	rootCmd.SetOut(&strings.Builder{})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNodeInfo_NoArg(t *testing.T) {
	rootCmd.SetArgs([]string{"node", "info"})
	rootCmd.SetOut(&strings.Builder{})
	if err := rootCmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("want error when name omitted")
	}
}

func TestNodeInfo_UnknownAgent(t *testing.T) {
	srv := newFakeKernel()
	defer srv.Close()
	restore := withFakeKernel(srv.Server)
	defer restore()

	rootCmd.SetArgs([]string{"node", "info", "ghost-agent"})
	rootCmd.SetOut(&strings.Builder{})
	if err := rootCmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("want error for unknown agent")
	}
}