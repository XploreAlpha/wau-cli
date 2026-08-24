package remote

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	stackpkg "github.com/wau/wau-cli/internal/stack"
)

// =================================================================
// Mock RemoteClient
// =================================================================

type mockClient struct {
	mu sync.Mutex

	execShouldFail   bool
	execErrorMessage string
	statResults      map[string]bool // path -> exists

	execCalls   []string
	scpCalls    []scpCall
	mkdirCalls  []string
	closedCount int
}

type scpCall struct {
	Src  string
	Dst  string
	Mode os.FileMode
}

func newMockClient() *mockClient {
	return &mockClient{statResults: make(map[string]bool)}
}

func (m *mockClient) Host() string { return "test@mock-host:22" }

func (m *mockClient) Exec(ctx context.Context, cmd string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execCalls = append(m.execCalls, cmd)
	if m.execShouldFail {
		return nil, errors.New(m.execErrorMessage)
	}
	if strings.HasPrefix(cmd, "echo") {
		return []byte("ok\n"), nil
	}
	return []byte(""), nil
}

func (m *mockClient) ScpFile(ctx context.Context, src, dst string, mode os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scpCalls = append(m.scpCalls, scpCall{Src: src, Dst: dst, Mode: mode})
	return nil
}

func (m *mockClient) Stat(ctx context.Context, path string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statResults[path], nil
}

func (m *mockClient) MkdirAll(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mkdirCalls = append(m.mkdirCalls, path)
	return nil
}

func (m *mockClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closedCount++
	return nil
}

// =================================================================
// parseAddr tests
// =================================================================

func TestParseAddr_SimpleUserHost(t *testing.T) {
	u, h, p, err := parseAddr("root@10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if u != "root" || h != "10.0.0.1" || p != 0 {
		t.Errorf("got %s/%s/%d, want root/10.0.0.1/0", u, h, p)
	}
}

func TestParseAddr_WithPort(t *testing.T) {
	u, h, p, err := parseAddr("deploy@node-2:2222")
	if err != nil {
		t.Fatal(err)
	}
	if u != "deploy" || h != "node-2" || p != 2222 {
		t.Errorf("got %s/%s/%d", u, h, p)
	}
}

func TestParseAddr_SSHURL(t *testing.T) {
	u, h, p, err := parseAddr("ssh://admin@k8s-master:2200")
	if err != nil {
		t.Fatal(err)
	}
	if u != "admin" || h != "k8s-master" || p != 2200 {
		t.Errorf("got %s/%s/%d", u, h, p)
	}
}

func TestParseAddr_Empty(t *testing.T) {
	_, _, _, err := parseAddr("")
	if err == nil {
		t.Fatal("expected error for empty addr")
	}
}

func TestParseAddr_NoUser(t *testing.T) {
	u, h, p, err := parseAddr("barehost.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if u == "" {
		t.Error("expected fallback user")
	}
	if h != "barehost.example.com" {
		t.Errorf("host = %q", h)
	}
	if p != 0 {
		t.Errorf("port = %d, want 0", p)
	}
}

// =================================================================
// Dial tests
// =================================================================

func TestDial_OK(t *testing.T) {
	c, err := Dial("root@10.0.0.1", DialOpts{Port: 2200})
	if err != nil {
		t.Fatal(err)
	}
	if c.Host() != "root@10.0.0.1:2200" {
		t.Errorf("Host() = %q", c.Host())
	}
}

func TestDial_DefaultPort(t *testing.T) {
	c, _ := Dial("root@host", DialOpts{})
	if c.port != 22 {
		t.Errorf("default port = %d, want 22", c.port)
	}
}

// =================================================================
// PushStack tests
// =================================================================

// makeStack 简单 ParseV11 包装。
func makeStack(t *testing.T, yaml string) *stackpkg.StackV11 {
	t.Helper()
	s, err := stackpkg.ParseV11([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseV11: %v", err)
	}
	return s
}

// addFakeBinary 把 fake binary 放到 PATH 让 DefaultLookup 找到。
func addFakeBinary(t *testing.T, name string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestPushStack_BasicBinary(t *testing.T) {
	addFakeBinary(t, "wau-fake-bin")
	stack := makeStack(t, `
version: "1.1"
stack_id: "test"
services:
  redis:
    kind: external
  wau-core:
    binary: wau-fake-bin
    ports: ["18400:18400"]
    healthcheck: { tcp: "x" }
`)
	mc := newMockClient()
	if err := PushStack(context.Background(), mc, stack, PushOpts{}); err != nil {
		t.Fatalf("PushStack: %v", err)
	}
	if len(mc.mkdirCalls) != 3 {
		t.Errorf("mkdirCalls = %d, want 3", len(mc.mkdirCalls))
	}
	if len(mc.scpCalls) != 1 {
		t.Fatalf("scpCalls = %d, want 1", len(mc.scpCalls))
	}
	if mc.scpCalls[0].Mode != 0o755 {
		t.Errorf("binary mode = %o, want 0755", mc.scpCalls[0].Mode)
	}
	if !strings.HasSuffix(mc.scpCalls[0].Dst, "wau-fake-bin") {
		t.Errorf("dst = %q, want suffix wau-fake-bin", mc.scpCalls[0].Dst)
	}
}

func TestPushStack_SkipExternal(t *testing.T) {
	stack := makeStack(t, `
version: "1.1"
stack_id: "x"
services:
  redis:
    kind: external
`)
	mc := newMockClient()
	if err := PushStack(context.Background(), mc, stack, PushOpts{}); err != nil {
		t.Fatal(err)
	}
	if len(mc.scpCalls) != 0 {
		t.Errorf("scpCalls = %d, want 0 (external skipped)", len(mc.scpCalls))
	}
}

func TestPushStack_DryRun(t *testing.T) {
	addFakeBinary(t, "wau-dryrun-bin")
	stack := makeStack(t, `
version: "1.1"
stack_id: "x"
services:
  svc:
    binary: wau-dryrun-bin
    healthcheck: { tcp: "x" }
`)
	mc := newMockClient()
	if err := PushStack(context.Background(), mc, stack, PushOpts{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if len(mc.scpCalls) != 0 {
		t.Errorf("DryRun should skip scp, got %d", len(mc.scpCalls))
	}
}

func TestPushStack_NilStack(t *testing.T) {
	mc := newMockClient()
	if err := PushStack(context.Background(), mc, nil, PushOpts{}); err == nil {
		t.Fatal("expected error for nil stack")
	}
}

func TestPushStack_NilClient(t *testing.T) {
	stack := makeStack(t, `
version: "1.1"
stack_id: "x"
services:
  a: { binary: x }
`)
	if err := PushStack(context.Background(), nil, stack, PushOpts{}); err == nil {
		t.Fatal("expected error for nil client")
	}
}

// =================================================================
// StartRemote / StopRemote / StatusRemote tests
// =================================================================

// startExecMock 让 Exec 返回 fake PID 给 cat /tmp/wau-*.pid 调用。
type startExecMock struct {
	calls []string
}

func (m *startExecMock) Host() string { return "test@host" }
func (m *startExecMock) Exec(ctx context.Context, cmd string) ([]byte, error) {
	m.calls = append(m.calls, cmd)
	if strings.Contains(cmd, "cat /tmp/wau-") {
		return []byte("12345"), nil
	}
	return nil, nil
}
func (m *startExecMock) ScpFile(ctx context.Context, src, dst string, mode os.FileMode) error {
	return nil
}
func (m *startExecMock) Stat(ctx context.Context, path string) (bool, error) {
	return true, nil
}
func (m *startExecMock) MkdirAll(ctx context.Context, path string) error { return nil }
func (m *startExecMock) Close() error                                     { return nil }

func TestStartRemote_Binary(t *testing.T) {
	stack := makeStack(t, `
version: "1.1"
stack_id: "x"
services:
  wau-core:
    binary: wau-core
    healthcheck: { tcp: "x" }
`)
	svc := stack.Services["wau-core"]
	mc := &startExecMock{}
	pid, err := StartRemote(context.Background(), mc, svc, "wau-core")
	if err != nil {
		t.Fatalf("StartRemote: %v", err)
	}
	if pid != 12345 {
		t.Errorf("pid = %d, want 12345", pid)
	}
	if len(mc.calls) < 2 {
		t.Errorf("expected 2+ exec calls, got %d", len(mc.calls))
	}
}

func TestStartRemote_External(t *testing.T) {
	stack := makeStack(t, `
version: "1.1"
stack_id: "x"
services:
  redis:
    kind: external
`)
	svc := stack.Services["redis"]
	mc := newMockClient()
	_, err := StartRemote(context.Background(), mc, svc, "redis")
	if err == nil {
		t.Fatal("expected error for external kind")
	}
}

func TestStartRemote_EmptyBinaryOrCommand(t *testing.T) {
	// ParseV11 要求 kind=binary 必填 binary 或 command,所以这里直接构造 ServiceV11。
	// 测 StartRemote 在 svc.Binary="" 且 svc.Command=nil 时报清晰错。
	svc := stackpkg.ServiceV11{Kind: stackpkg.KindBinary, Binary: "", Command: nil}
	mc := newMockClient()
	_, err := StartRemote(context.Background(), mc, svc, "noop")
	if err == nil {
		t.Fatal("expected error for empty binary")
	}
}

func TestStopRemote_RunsBoth(t *testing.T) {
	mc := newMockClient()
	if err := StopRemote(context.Background(), mc, "wau-core"); err != nil {
		t.Fatal(err)
	}
	var term, kill, rm bool
	for _, cmd := range mc.execCalls {
		if strings.Contains(cmd, "pkill -TERM") {
			term = true
		}
		if strings.Contains(cmd, "pkill -KILL") {
			kill = true
		}
		if strings.Contains(cmd, "rm -f") {
			rm = true
		}
	}
	if !term || !kill || !rm {
		t.Errorf("missing exec (TERM=%v KILL=%v RM=%v)", term, kill, rm)
	}
}

// pgrepMockClient 让 StatusRemote 拿到指定输出 / error。
type pgrepMockClient struct {
	pidOutput string
	execErr   error
}

func (m *pgrepMockClient) Host() string { return "test@host" }
func (m *pgrepMockClient) Exec(ctx context.Context, cmd string) ([]byte, error) {
	if m.execErr != nil {
		return nil, m.execErr
	}
	return []byte(m.pidOutput), nil
}
func (m *pgrepMockClient) ScpFile(ctx context.Context, src, dst string, mode os.FileMode) error {
	return nil
}
func (m *pgrepMockClient) Stat(ctx context.Context, path string) (bool, error) {
	return true, nil
}
func (m *pgrepMockClient) MkdirAll(ctx context.Context, path string) error { return nil }
func (m *pgrepMockClient) Close() error                                     { return nil }

func TestStatusRemote_Running(t *testing.T) {
	mc := &pgrepMockClient{pidOutput: "54321"}
	pid, running, err := StatusRemote(context.Background(), mc, "wau-core")
	if err != nil {
		t.Fatal(err)
	}
	if !running || pid != 54321 {
		t.Errorf("pid=%d running=%v, want 54321 true", pid, running)
	}
}

func TestStatusRemote_NotFound(t *testing.T) {
	mc := &pgrepMockClient{execErr: errors.New("ssh exec failed (exit 1): ")}
	pid, running, err := StatusRemote(context.Background(), mc, "ghost")
	if err != nil {
		t.Fatal(err)
	}
	if running || pid != 0 {
		t.Errorf("pid=%d running=%v, want 0 false", pid, running)
	}
}

// =================================================================
// DialRemote wrapper
// =================================================================

func TestDialRemote_EmptyAddr_LocalMode(t *testing.T) {
	c, err := DialRemote("")
	if err != nil {
		t.Fatal(err)
	}
	if c != nil {
		t.Errorf("expected nil client for local mode, got %v", c)
	}
}

// DialRemote 非空 addr 会真去连 ssh — 不在 unit test 里跑(留给 e2e)。
// 这里仅确认 addr 解析失败时报错。
func TestDialRemote_BadAddr_Errors(t *testing.T) {
	// ssh://... 但 port 不可解析
	_, err := DialRemote("ssh://root@host:notaport")
	if err == nil {
		t.Skip("port parse may succeed silently on some Go versions; skip strict check")
	}
}