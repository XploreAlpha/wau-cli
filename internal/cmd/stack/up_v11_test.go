// Package stack — up_v11_test.go
//
// 4.1.5 (2026-08-24) — runUpV11 + dispatcher + 转换 helper 测试。
//
// 测试范围:
//   - isV11YAML / runUpDispatcher 版本分派
//   - runUpV11:profile/demo 互斥 / dry-run plan 输出
//   - convertToV1Service 字段映射
//   - healthToProbe 四种类型
//   - extractPortsFromList 多格式
//
// 注:ProcessManager.Start / remote.StartRemote 走真 subprocess,不在这层 mock
//   (留给 e2e + remote_test.go 用 mock RemoteClient 覆盖)。
package stack

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	stackpkg "github.com/wau/wau-cli/internal/stack"
)

// writeYAMLFileAt helper:写 YAML 到 t.TempDir 并返回路径(避免与 validate_test.go 的 writeYAMLFile 冲突)。
func writeYAMLFileAt(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// resetUpFlags 把 up 全局 flag 还原到默认值,避免 case 间污染。
func resetUpFlags() {
	upFile = ""
	upProfile = ""
	upDemo = false
	upDryRun = false
	upDetach = false
	upWaitMax = 60 * 1_000_000_000 // 60s in ns
	upRemote = ""
}

// =================================================================
// isV11YAML
// =================================================================

func TestIsV11YAML_V11(t *testing.T) {
	resetUpFlags()
	path := writeYAMLFileAt(t, "v11.yml", `
version: "1.1"
stack_id: "test"
release: "v1.3.4"
services:
  redis: { kind: external }
`)
	if !isV11YAML(path) {
		t.Error("expected isV11YAML=true for version: 1.1")
	}
}

func TestIsV11YAML_V1(t *testing.T) {
	resetUpFlags()
	path := writeYAMLFileAt(t, "v1.yml", `
version: "1"
stack:
  name: "x"
  release: "v1.3.4"
services:
  - name: redis
    kind: external
`)
	if isV11YAML(path) {
		t.Error("expected isV11YAML=false for version: 1")
	}
}

func TestIsV11YAML_MissingFile(t *testing.T) {
	if isV11YAML("/nonexistent/path/foo.yml") {
		t.Error("expected false for missing file")
	}
}

func TestIsV11YAML_MissingVersion(t *testing.T) {
	path := writeYAMLFileAt(t, "noversion.yml", `
stack_id: "x"
services:
  redis: { kind: external }
`)
	if isV11YAML(path) {
		t.Error("expected false when version missing")
	}
}

func TestIsV11YAML_InvalidYAML(t *testing.T) {
	path := writeYAMLFileAt(t, "bad.yml", "this is: : not yaml: at all: :")
	if isV11YAML(path) {
		t.Error("expected false for unparseable YAML")
	}
}

// =================================================================
// runUpDispatcher
// =================================================================

func TestRunUpDispatcher_RemoteRequiresFile(t *testing.T) {
	resetUpFlags()
	upRemote = "ssh://root@example.com:22"
	cmd := &cobra.Command{Use: "up"}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	err := runUpDispatcher(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--remote requires --file") {
		t.Errorf("expected --remote requires --file error, got %v", err)
	}
}

func TestRunUpDispatcher_RemoteRequiresV11(t *testing.T) {
	resetUpFlags()
	upRemote = "ssh://root@example.com:22"
	upFile = writeYAMLFileAt(t, "v1.yml", `
version: "1"
stack: { name: x, release: v1.3.4 }
services:
  - name: redis
    kind: external
`)
	cmd := &cobra.Command{Use: "up"}
	cmd.SetOut(&bytes.Buffer{})
	err := runUpDispatcher(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "requires v1.1 schema") {
		t.Errorf("expected v1.1 schema required, got %v", err)
	}
}

func TestRunUpDispatcher_V11FileRoutesToV11(t *testing.T) {
	resetUpFlags()
	upFile = writeYAMLFileAt(t, "v11.yml", `
version: "1.1"
stack_id: "test"
release: "v1.3.4"
services:
  redis: { kind: external }
  wau-core: { binary: wau-fake, healthcheck: { tcp: "x" } }
`)
	upDryRun = true
	cmd := &cobra.Command{Use: "up"}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := runUpDispatcher(cmd, nil); err != nil {
		t.Fatalf("dispatcher: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "v1.1") {
		t.Errorf("expected v1.1 marker in output, got:\n%s", out)
	}
	if !strings.Contains(out, "stack_id=\"test\"") {
		t.Errorf("expected stack_id in output, got:\n%s", out)
	}
	if !strings.Contains(out, "redis") {
		t.Errorf("expected redis in plan, got:\n%s", out)
	}
}

func TestRunUpDispatcher_V1FileRoutesToOldPath(t *testing.T) {
	// v1 文件 + 不带 --remote + 不带 --demo,走老 runUp 路径。
	// 给一个最小 v1 binary 服务让 dry-run 跑通(不需要 external 避免 health required 检查)。
	resetUpFlags()
	upFile = writeYAMLFileAt(t, "v1.yml", `
version: "1"
stack:
  name: "test"
  release: "v1.3.4"
services:
  - name: wau-core
    binary: wau-fake
    health: { type: tcp, addr: "127.0.0.1:9999" }
`)
	upDryRun = true
	cmd := &cobra.Command{Use: "up"}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := runUpDispatcher(cmd, nil); err != nil {
		t.Fatalf("dispatcher v1 path: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "v1.1") {
		t.Errorf("v1 path should NOT show v1.1 marker, got:\n%s", out)
	}
	if !strings.Contains(out, "Plan (dry-run):") {
		t.Errorf("expected v1 Plan (dry-run) marker, got:\n%s", out)
	}
}

// =================================================================
// runUpV11 dry-run
// =================================================================

func TestRunUpV11_DryRunPrintsPlan(t *testing.T) {
	resetUpFlags()
	upFile = writeYAMLFileAt(t, "v11.yml", `
version: "1.1"
stack_id: "demo"
release: "v1.3.4"
services:
  redis: { kind: external }
  wau-core:
    binary: wau-core
    required: true
    ports: ["18400:18400"]
    healthcheck: { tcp: "127.0.0.1:18400" }
  wau-registry:
    binary: wau-registry
    depends_on: ["wau-core"]
    healthcheck: { tcp: "127.0.0.1:18403" }
`)
	upDryRun = true
	cmd := &cobra.Command{Use: "up"}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := runUpV11(cmd, nil); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Plan (dry-run, v1.1",
		`stack_id="demo"`,
		`release="v1.3.4"`,
		"wau-core",
		"wau-registry",
		"[required]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	// topo:redis → wau-core → wau-registry
	idxRedis := strings.Index(out, "redis")
	idxCore := strings.Index(out, "wau-core")
	idxReg := strings.Index(out, "wau-registry")
	if !(idxRedis < idxCore && idxCore < idxReg) {
		t.Errorf("topo order wrong: redis=%d core=%d reg=%d", idxRedis, idxCore, idxReg)
	}
}

func TestRunUpV11_DryRunWithRemotePrintsRemoteHint(t *testing.T) {
	resetUpFlags()
	upFile = writeYAMLFileAt(t, "v11.yml", `
version: "1.1"
stack_id: "demo"
release: "v1.3.4"
services:
  redis: { kind: external }
`)
	upDryRun = true
	upRemote = "ssh://root@example.com:22"
	cmd := &cobra.Command{Use: "up"}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := runUpV11(cmd, nil); err != nil {
		t.Fatalf("dry-run remote: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[remote=ssh://root@example.com:22] PushStack") {
		t.Errorf("missing remote PushStack hint:\n%s", out)
	}
}

func TestRunUpV11_DemoAndProfileMutuallyExclusive(t *testing.T) {
	resetUpFlags()
	upFile = writeYAMLFileAt(t, "v11.yml", `
version: "1.1"
stack_id: "demo"
release: "v1.3.4"
services:
  redis: { kind: external }
`)
	upDemo = true
	upProfile = "minimal"
	cmd := &cobra.Command{Use: "up"}
	cmd.SetOut(&bytes.Buffer{})
	err := runUpV11(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutually exclusive error, got %v", err)
	}
}

func TestRunUpV11_DefaultStackWhenNoFile(t *testing.T) {
	// 不给 --file,应该走 defaults.DefaultStackYAMLBytes()(embed)
	resetUpFlags()
	upDryRun = true
	cmd := &cobra.Command{Use: "up"}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := runUpV11(cmd, nil); err != nil {
		t.Fatalf("default embed dry-run: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "stack_id=") {
		t.Errorf("expected stack_id from embedded default, got:\n%s", out)
	}
}

// =================================================================
// convertToV1Service
// =================================================================

func TestConvertToV1Service_FieldMapping(t *testing.T) {
	svc := &stackpkg.ServiceV11{
		Kind:      stackpkg.KindBinary,
		Binary:    "wau-core",
		Args:      []string{"--config", "/etc/wau/core.yml"},
		Command:   []string{"exec", "wau-core"},
		Ports:     []string{"18400:18400", "18401:18401"},
		Env:       map[string]string{"FOO": "bar"},
		DependsOn: []string{"redis"},
		Required:  true,
	}
	v1 := convertToV11_Helper("wau-core", svc)
	if v1.Name != "wau-core" || v1.Binary != "wau-core" {
		t.Errorf("name/binary mapping wrong: %+v", v1)
	}
	if len(v1.Args) != 4 {
		t.Errorf("expected 4 args (2 args + 2 command), got %d", len(v1.Args))
	}
	if v1.Args[2] != "exec" || v1.Args[3] != "wau-core" {
		t.Errorf("command → args merge wrong: %v", v1.Args)
	}
	if v1.HTTPPort != 18400 || v1.GRPCPort != 18401 {
		t.Errorf("port mapping: http=%d grpc=%d", v1.HTTPPort, v1.GRPCPort)
	}
	if !v1.Required {
		t.Error("Required not preserved")
	}
	if len(v1.DependsOn) != 1 || v1.DependsOn[0] != "redis" {
		t.Errorf("depends_on not preserved: %v", v1.DependsOn)
	}
	if v1.Health != nil {
		t.Errorf("expected nil Probe when no healthcheck set, got %+v", v1.Health)
	}
}

func TestConvertToV1Service_WithHealthcheck(t *testing.T) {
	svc := &stackpkg.ServiceV11{
		Kind:        stackpkg.KindBinary,
		Binary:      "x",
		Healthcheck: &stackpkg.HealthcheckSpec{TCP: "127.0.0.1:9999"},
	}
	v1 := convertToV11_Helper("svc", svc)
	if v1.Health == nil || v1.Health.Type != stackpkg.ProbeTCP || v1.Health.Addr != "127.0.0.1:9999" {
		t.Errorf("healthcheck → Probe mapping wrong: %+v", v1.Health)
	}
}

// convertToV11_Helper 包装(测试用)— 实际函数是 unexported convertToV1Service。
// 直接用 unexported 名是 OK 的,因为测试在同一 package。
func convertToV11_Helper(name string, svc *stackpkg.ServiceV11) *stackpkg.Service {
	return convertToV1Service(name, svc)
}

// =================================================================
// healthToProbe
// =================================================================

func TestHealthToProbe_TCP(t *testing.T) {
	p := healthToProbe(&stackpkg.HealthcheckSpec{TCP: "127.0.0.1:9999"})
	if p.Type != stackpkg.ProbeTCP || p.Addr != "127.0.0.1:9999" {
		t.Errorf("TCP mapping wrong: %+v", p)
	}
}

func TestHealthToProbe_HTTP(t *testing.T) {
	p := healthToProbe(&stackpkg.HealthcheckSpec{HTTP: "http://x/healthz"})
	if p.Type != stackpkg.ProbeHTTP || p.URL != "http://x/healthz" {
		t.Errorf("HTTP mapping wrong: %+v", p)
	}
}

func TestHealthToProbe_GRPCDowngradesToTCP(t *testing.T) {
	p := healthToProbe(&stackpkg.HealthcheckSpec{GRPC: "127.0.0.1:9999/wau.Health/Check"})
	if p.Type != stackpkg.ProbeTCP || p.Addr != "127.0.0.1:9999/wau.Health/Check" {
		t.Errorf("GRPC→TCP mapping wrong: %+v", p)
	}
}

func TestHealthToProbe_Exec(t *testing.T) {
	p := healthToProbe(&stackpkg.HealthcheckSpec{Exec: "curl -sf localhost"})
	if p.Type != stackpkg.ProbeExec || p.Cmd != "curl -sf localhost" {
		t.Errorf("Exec mapping wrong: %+v", p)
	}
}

func TestHealthToProbe_NilReturnsNil(t *testing.T) {
	if healthToProbe(nil) != nil {
		t.Error("expected nil Probe for nil HealthcheckSpec")
	}
}

func TestHealthToProbe_DefaultsIntervalAndTimeout(t *testing.T) {
	p := healthToProbe(&stackpkg.HealthcheckSpec{TCP: "x"})
	if p.Interval == 0 || p.Timeout == 0 {
		t.Errorf("interval/timeout defaults not set: %+v", p)
	}
}

// =================================================================
// extractPortsFromList
// =================================================================

func TestExtractPortsFromList_SinglePort(t *testing.T) {
	http, grpc := extractPortsFromList([]string{"18400:18400"})
	if http != 18400 || grpc != 0 {
		t.Errorf("got http=%d grpc=%d, want 18400/0", http, grpc)
	}
}

func TestExtractPortsFromList_TwoPorts(t *testing.T) {
	http, grpc := extractPortsFromList([]string{"18400:18400", "18401:18401"})
	if http != 18400 || grpc != 18401 {
		t.Errorf("got http=%d grpc=%d, want 18400/18401", http, grpc)
	}
}

func TestExtractPortsFromList_HostAndContainerDifferent(t *testing.T) {
	http, grpc := extractPortsFromList([]string{"9000:18400", "9001:18401"})
	if http != 9000 || grpc != 9001 {
		t.Errorf("host port extraction wrong: got http=%d grpc=%d, want 9000/9001", http, grpc)
	}
}

func TestExtractPortsFromList_Empty(t *testing.T) {
	http, grpc := extractPortsFromList(nil)
	if http != 0 || grpc != 0 {
		t.Errorf("empty ports should give 0/0, got %d/%d", http, grpc)
	}
}

func TestExtractPortsFromList_NonNumeric(t *testing.T) {
	http, grpc := extractPortsFromList([]string{"abc:def"})
	if http != 0 || grpc != 0 {
		t.Errorf("non-numeric should give 0/0, got %d/%d", http, grpc)
	}
}

func TestExtractHostPortFromSpec(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"18400:18400", "18400"},
		{"9000", "9000"},
		{"127.0.0.1:18400", "127.0.0.1"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := extractHostPortFromSpec(tc.in); got != tc.want {
			t.Errorf("extractHostPortFromSpec(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAtoiSafe(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"0", 0},
		{"18400", 18400},
		{"18400a", 0},
		{"-1", 0},
		{"abc", 0},
	}
	for _, tc := range cases {
		if got := atoiSafe(tc.in); got != tc.want {
			t.Errorf("atoiSafe(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// =================================================================
// resolveStackDirsV11
// =================================================================

func TestResolveStackDirsV11_Defaults(t *testing.T) {
	stack := &stackpkg.StackV11{StackID: "test", DataDir: ""}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	data, log, err := resolveStackDirsV11(stack)
	if err != nil {
		t.Fatalf("resolveStackDirsV11: %v", err)
	}
	if data != home+"/.wau/run" {
		t.Errorf("data = %q, want %q", data, home+"/.wau/run")
	}
	if log != home+"/.wau/log" {
		t.Errorf("log = %q, want %q", log, home+"/.wau/log")
	}
}

func TestResolveStackDirsV11_TildeExpansion(t *testing.T) {
	stack := &stackpkg.StackV11{StackID: "test", DataDir: "~/custom/wau"}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	data, _, err := resolveStackDirsV11(stack)
	if err != nil {
		t.Fatal(err)
	}
	if data != home+"/custom/wau" {
		t.Errorf("data = %q, want %q", data, home+"/custom/wau")
	}
}

// =================================================================
// upV11ExitError
// =================================================================

func TestUpV11ExitError_ExitCode(t *testing.T) {
	var err error = upV11ExitError(2)
	if err.Error() == "" {
		t.Error("Error() should not be empty")
	}
	type ec interface{ ExitCode() int }
	if e, ok := err.(ec); ok {
		if e.ExitCode() != 2 {
			t.Errorf("ExitCode = %d, want 2", e.ExitCode())
		}
	} else {
		t.Error("upV11ExitError does not implement ExitCode() int")
	}
}
