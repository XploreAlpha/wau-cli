// Package agent - install_test.go
//
// 第二刀 P1.3 — `wau agent install` 命令测试。
//
// 覆盖:
//   - parseConfig:happy path
//   - parseConfig:missing '=' 返回 error
//   - parseConfig:empty 输入返回 nil map
//   - parseConfig:重复 key 后者覆盖前者
//   - getDefaultUser:返回 "default"
//   - runInstall:happy path(httptest mock server)
//   - runInstall:server 返回 ok=false
//   - runInstall:server 返回 5xx(走 retry 后失败)
//   - runInstall:no arg
//   - runInstall:request body 字段(版本/purge/config/user 透传)
//   - newInstallCmd:Use/Args/Flags 注册
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	wauclient "github.com/wau/wau-cli/internal/client"
)

// withMockAddr 临时把 kernelAddrAccessor 指向 srv.URL,返回 restore。
func withMockAddr(srvURL string) func() {
	old := kernelAddrAccessor
	kernelAddrAccessor = func() string { return srvURL }
	return func() { kernelAddrAccessor = old }
}

// ─── parseConfig ────────────────────────────────────────────────────────────

func TestParseConfig_HappyPath(t *testing.T) {
	got, err := parseConfig([]string{"city=北京", "lang=zh", "max=10"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"city": "北京", "lang": "zh", "max": "10"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("got[%q]=%q, want %q", k, got[k], v)
		}
	}
}

func TestParseConfig_Empty(t *testing.T) {
	got, err := parseConfig(nil)
	if err != nil || got != nil {
		t.Errorf("got=%v err=%v, want nil/nil", got, err)
	}
	got, err = parseConfig([]string{})
	if err != nil || got != nil {
		t.Errorf("got=%v err=%v, want nil/nil", got, err)
	}
}

func TestParseConfig_MissingEquals(t *testing.T) {
	_, err := parseConfig([]string{"city=北京", "badvalue", "lang=zh"})
	if err == nil {
		t.Fatal("want error for item without '='")
	}
	if !strings.Contains(err.Error(), "invalid --config") {
		t.Errorf("err = %v", err)
	}
}

func TestParseConfig_DuplicateKey(t *testing.T) {
	got, err := parseConfig([]string{"k=v1", "k=v2"})
	if err != nil {
		t.Fatal(err)
	}
	if got["k"] != "v2" {
		t.Errorf("dup key: got[k]=%q, want v2 (later wins)", got["k"])
	}
}

// ─── getDefaultUser ─────────────────────────────────────────────────────────

func TestGetDefaultUser_Default(t *testing.T) {
	if got := getDefaultUser(); got != "default" {
		t.Errorf("got %q, want \"default\"", got)
	}
}

// ─── runInstall ─────────────────────────────────────────────────────────────

func TestRunInstall_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/l5/install" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"ok":true,"agent_id":"a-1","version":"1.2.0","installed_at":1700000000,"duration_ms":42.5,"sandbox_docker_id":"d-9"}`)
	}))
	defer srv.Close()
	restore := withMockAddr(srv.URL)
	defer restore()

	cmd := newInstallCmd()
	cmd.SetArgs([]string{"fox-medical"})
	cmd.SetOut(&strings.Builder{})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRunInstall_ServerReturnsNotOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"ok":false,"error":"sha256 mismatch"}`)
	}))
	defer srv.Close()
	restore := withMockAddr(srv.URL)
	defer restore()

	cmd := newInstallCmd()
	cmd.SetArgs([]string{"fox-medical"})
	cmd.SetOut(&strings.Builder{})
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("want error when server returns ok=false")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Errorf("err = %v", err)
	}
}

func TestRunInstall_Server500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	restore := withMockAddr(srv.URL)
	defer restore()

	cmd := newInstallCmd()
	cmd.SetArgs([]string{"fox-medical"})
	cmd.SetOut(&strings.Builder{})
	// 60s retry × backoff 太久,改用 short retry client
	// 但这里没法改 client,只能让它跑(实际 wait ~ 几秒)
	// 用 context timeout 强制退出
	ctx, cancel := context.WithTimeout(context.Background(), 2*1000_000_000) // 2s
	defer cancel()
	err := cmd.ExecuteContext(ctx)
	if err == nil {
		t.Fatal("want error after retries")
	}
}

func TestRunInstall_NoArg(t *testing.T) {
	cmd := newInstallCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&strings.Builder{})
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("want error when no name given")
	}
}

func TestRunInstall_RequestShape(t *testing.T) {
	var got wauclient.L5InstallRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"ok":true,"agent_id":"x","version":"2.0.0","duration_ms":1}`)
	}))
	defer srv.Close()
	restore := withMockAddr(srv.URL)
	defer restore()

	cmd := newInstallCmd()
	cmd.SetArgs([]string{"weather", "--version=2.0.0", "--config=city=北京", "--purge", "--user=alice"})
	cmd.SetOut(&strings.Builder{})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.AgentName != "weather" {
		t.Errorf("AgentName = %q", got.AgentName)
	}
	if got.Version != "2.0.0" {
		t.Errorf("Version = %q", got.Version)
	}
	if !got.Purge {
		t.Error("Purge should be true")
	}
	if got.UserID != "alice" {
		t.Errorf("UserID = %q", got.UserID)
	}
	if got.Config["city"] != "北京" {
		t.Errorf("Config[city] = %q", got.Config["city"])
	}
}

// ─── newInstallCmd wiring ───────────────────────────────────────────────────

func TestNewInstallCmd_BasicArgs(t *testing.T) {
	cmd := newInstallCmd()
	if cmd.Use != "install <name>" {
		t.Errorf("Use = %q", cmd.Use)
	}
	if cmd.Args == nil {
		t.Error("Args validator not set")
	}
	for _, name := range []string{"version", "purge", "config", "user"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag %q missing", name)
		}
	}
}