// Package cmd - doctor_test.go
//
// 4.3 MVP — `wau doctor` 离线诊断测试。
//
// 关键 case:WAU_* env vars 脱敏(per [[feedback-api-key-leak-2026-07-30]] — 任何
// 含 WAU_*SECRET 的值都不出现在 stdout)。
package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =================================================================
// Flag / 命令结构
// =================================================================

func TestNewDoctorCmd_Flags(t *testing.T) {
	cmd := newDoctorCmd()
	if cmd.Use != "doctor" {
		t.Errorf("Use = %q, want %q", cmd.Use, "doctor")
	}
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "diag" {
		t.Errorf("Aliases = %v, want [diag]", cmd.Aliases)
	}
	flag := cmd.Flags().Lookup("format")
	if flag == nil {
		t.Fatal("--format flag missing")
	}
	if flag.Shorthand != "f" {
		t.Errorf("shorthand = %q, want f", flag.Shorthand)
	}
}

// =================================================================
// doctorCheck helper
// =================================================================

func TestDoctorCheck_Fields(t *testing.T) {
	c := doctorCheck{Name: "x", Status: "ok", Detail: "y"}
	if c.Name != "x" || c.Status != "ok" || c.Detail != "y" {
		t.Errorf("field assignment wrong: %+v", c)
	}
}

// =================================================================
// checkEnvVars 脱敏(关键安全 case)
// =================================================================

func TestCheckEnvVars_MasksSecretValues(t *testing.T) {
	// 模拟含 secret 的 env var(per [[feedback-api-key-leak-2026-07-30]])
	// Key = WAU_JWT_SHARED_SECRET,Value = secretvalue123
	t.Setenv("WAU_JWT_SHARED_SECRET", "secretvalue123")
	t.Setenv("WAU_CLUSTER_ADDR", "https://cluster.example.com")

	c := checkEnvVars()
	if c.Status != "ok" {
		t.Errorf("expected ok status with 2 WAU_ vars set, got %s", c.Status)
	}
	// 关键断言:secret value 绝对不能出现在 detail 里
	if strings.Contains(c.Detail, "secretvalue123") {
		t.Errorf("LEAK: secret value in doctor output: %s", c.Detail)
	}
	// key 名字应该出现
	if !strings.Contains(c.Detail, "WAU_JWT_SHARED_SECRET") {
		t.Errorf("env var name missing from output: %s", c.Detail)
	}
	if !strings.Contains(c.Detail, "WAU_CLUSTER_ADDR") {
		t.Errorf("env var name missing from output: %s", c.Detail)
	}
}

func TestCheckEnvVars_NoneSet(t *testing.T) {
	// 清空所有 WAU_* env vars
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "WAU_") {
			idx := strings.Index(kv, "=")
			if idx > 0 {
				t.Setenv(kv[:idx], "") // 清空 value
				_ = os.Unsetenv(kv[:idx])
			}
		}
	}
	c := checkEnvVars()
	if c.Status != "warn" {
		t.Errorf("expected warn with no WAU_ vars, got %s: %s", c.Status, c.Detail)
	}
	if !strings.Contains(c.Detail, "none set") {
		t.Errorf("expected 'none set' in detail, got %s", c.Detail)
	}
}

// =================================================================
// checkBinaries
// =================================================================

func TestCheckBinaries_NoneFound(t *testing.T) {
	// 把 PATH 设为空,确保三个 critical binary 都找不到
	t.Setenv("PATH", "")
	c := checkBinaries()
	if c.Status != "fail" {
		t.Errorf("expected fail with empty PATH, got %s: %s", c.Status, c.Detail)
	}
	if !strings.Contains(c.Detail, "wau-core") {
		t.Errorf("expected critical list in detail: %s", c.Detail)
	}
}

func TestCheckBinaries_AllFound(t *testing.T) {
	// 在 temp dir 放 fake binaries
	dir := t.TempDir()
	for _, name := range []string{"wau-core", "wau-registry", "wau-agent"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	c := checkBinaries()
	if c.Status != "ok" {
		t.Errorf("expected ok with all 3 in PATH, got %s: %s", c.Status, c.Detail)
	}
}

// =================================================================
// checkWauDir
// =================================================================

func TestCheckWauDir_OK(t *testing.T) {
	// UserHomeDir 应该可访问;如果 ~/.wau 不存在,自动创建
	c := checkWauDir()
	if c.Status != "ok" {
		t.Errorf("expected ok, got %s: %s", c.Status, c.Detail)
	}
}

func TestCheckWauDir_Unwritable(t *testing.T) {
	// 模拟 HOME 不可写 — 通过把 HOME 指到不存在 /read-only 路径
	if os.Getuid() == 0 {
		t.Skip("running as root; cannot test unwritable home")
	}
	tmp := t.TempDir()
	roDir := filepath.Join(tmp, "readonly")
	if err := os.Mkdir(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", roDir)
	c := checkWauDir()
	// mkdir 会失败 → fail
	if c.Status != "fail" {
		t.Errorf("expected fail with read-only HOME, got %s: %s", c.Status, c.Detail)
	}
}

// =================================================================
// checkHostsFile
// =================================================================

func TestCheckHostsFile_Missing(t *testing.T) {
	// HOME 指到 temp dir,~/.wau/hosts 肯定不存在
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	c := checkHostsFile()
	if c.Status != "warn" {
		t.Errorf("expected warn when hosts file missing, got %s: %s", c.Status, c.Detail)
	}
}

func TestCheckHostsFile_Present(t *testing.T) {
	tmp := t.TempDir()
	wauDir := filepath.Join(tmp, ".wau")
	if err := os.MkdirAll(wauDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wauDir, "hosts"), []byte("127.0.0.1 wau-core\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmp)
	c := checkHostsFile()
	if c.Status != "ok" {
		t.Errorf("expected ok when hosts file present, got %s: %s", c.Status, c.Detail)
	}
}

// =================================================================
// runDoctor 整体输出
// =================================================================

func TestRunDoctor_TableOutput(t *testing.T) {
	cmd := newDoctorCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.ParseFlags([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if e := runDoctor(cmd, nil); e != nil {
		// 可能因为环境返回 warn/fail,但函数本身不应 panic
		t.Logf("runDoctor returned (acceptable for warn/fail): %v", e)
	}
	out := buf.String()
	for _, want := range []string{"wau-cli", "CHECK", "STATUS", "DETAIL"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRunDoctor_JSONOutput(t *testing.T) {
	cmd := newDoctorCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	// 直接设置 flag(避免 cobra ParseFlags 副作用)
	if err := cmd.Flags().Set("format", "json"); err != nil {
		t.Fatal(err)
	}
	_ = runDoctor(cmd, nil) // JSON 路径不管 exit code
	// JSON 只在 stdout,summary 走 stderr
	out := stdout.String()
	var r doctorResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &r); err != nil {
		t.Errorf("invalid JSON output: %v\nstdout: %s\nstderr: %s",
			err, out, stderr.String())
	}
	if !strings.HasPrefix(r.Version, "wau-cli ") {
		t.Errorf("Version field wrong: %q", r.Version)
	}
	if len(r.Checks) == 0 {
		t.Error("expected at least 1 check, got 0")
	}
	for _, c := range r.Checks {
		switch c.Status {
		case "ok", "warn", "fail":
		default:
			t.Errorf("invalid check status: %q", c.Status)
		}
	}
}

// =================================================================
// doctorExitError
// =================================================================

func TestDoctorExitError_ExitCode1(t *testing.T) {
	var err error = doctorExitError(1)
	type ec interface{ ExitCode() int }
	type se interface{ SilenceError() bool }
	if e, ok := err.(ec); !ok || e.ExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1", e.ExitCode())
	}
	if s, ok := err.(se); !ok || !s.SilenceError() {
		t.Error("SilenceError() should return true to suppress cobra auto-print")
	}
}

// =================================================================
// 4 个 top-level aliases 注册检查(回归防护)
// =================================================================

func TestRootCmd_HasUpDownStatusDoctor(t *testing.T) {
	// rootCmd 是 package-level var(由 init() 注册所有子命令)
	// `wau status` 是 `wau ls` 的 alias,所以 Use="ls" 而 Name()=="ls",Aliases 包含 "status"
	want := map[string][]string{
		"up":     {},
		"down":   {},
		"ls":     {"status", "ps"}, // Use=ls + Aliases=[status,ps]
		"doctor": {"diag"},
	}
	for name, aliases := range want {
		var found *cobra.Command
		for _, c := range rootCmd.Commands() {
			if c.Name() == name {
				found = c
				break
			}
		}
		if found == nil {
			t.Errorf("top-level %q command not registered", name)
			continue
		}
		for _, a := range aliases {
			ok := false
			for _, ca := range found.Aliases {
				if ca == a {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("%q missing alias %q (have %v)", name, a, found.Aliases)
			}
		}
	}
}

// =================================================================
// sanity:exec.LookPath sanity(防止本地 env 把 critical binaries 都暴露)
// =================================================================

func TestExecLookPath_PathOverride(t *testing.T) {
	// 配 PATH 包含 fake binary
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wau-core"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	path, err := exec.LookPath("wau-core")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, dir) {
		t.Errorf("LookPath returned %q, expected prefix %q", path, dir)
	}
}
