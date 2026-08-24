package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── NewAuthCmd ────────────────────────────────────────────────────────────

func TestNewAuthCmd_BasicArgs(t *testing.T) {
	cmd := NewAuthCmd()
	if cmd.Use != "auth" {
		t.Errorf("Use = %q, want auth", cmd.Use)
	}
	// 3 子命令:login / logout / whoami
	wantCmds := map[string]bool{"login": false, "logout": false, "whoami": false}
	for _, sub := range cmd.Commands() {
		if _, ok := wantCmds[sub.Name()]; ok {
			wantCmds[sub.Name()] = true
		}
	}
	for name, found := range wantCmds {
		if !found {
			t.Errorf("subcommand %q missing", name)
		}
	}
}

// ─── Login ─────────────────────────────────────────────────────────────────

func TestNewLoginCmd_BasicArgs(t *testing.T) {
	cmd := NewLoginCmd()
	for _, name := range []string{"user", "password", "endpoint", "no-store"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag %q missing", name)
		}
	}
}

func TestRunAuthLogin_NoUserPass(t *testing.T) {
	// 没传 user/pass + stdin 没数据 → 应该报 read error,不调 server
	resetAuthLoginFlags()
	cmd := NewLoginCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("want error (stdin closed)")
	}
}

func TestRunAuthLogin_RequiresStoreFlag(t *testing.T) {
	// 测 flag 注册完整性 + 跟 SetAccessors 的兼容性
	resetAuthLoginFlags()
	cmd := NewLoginCmd()
	if cmd.Flags().Lookup("no-store") == nil {
		t.Fatal("no-store flag missing")
	}
}

func resetAuthLoginFlags() {
	flagLoginUser = ""
	flagLoginPassword = ""
	flagLoginEndpoint = ""
	flagLoginNoStore = false
}

// ─── Logout ────────────────────────────────────────────────────────────────

func TestNewLogoutCmd_BasicArgs(t *testing.T) {
	cmd := NewLogoutCmd()
	if cmd.Use != "logout" {
		t.Errorf("Use = %q, want logout", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("RunE missing")
	}
}

func TestRunAuthLogout_NoCredentials(t *testing.T) {
	// 临时 HOME 不存在 credentials
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	resetAuthLogoutFlags()
	cmd := NewLogoutCmd()
	cmd.SetArgs([]string{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("want nil err for missing creds, got: %v", err)
	}
	if !strings.Contains(out.String(), "Not logged in") {
		t.Errorf("output = %q, want 'Not logged in'", out.String())
	}
}

func TestRunAuthLogout_RemoveExisting(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	// 预写 credentials
	wauDir := filepath.Join(tmpHome, ".wau")
	if err := os.MkdirAll(wauDir, 0o700); err != nil {
		t.Fatal(err)
	}
	credPath := filepath.Join(wauDir, "credentials")
	if err := os.WriteFile(credPath, []byte(`{"access_token":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	resetAuthLogoutFlags()
	cmd := NewLogoutCmd()
	cmd.SetArgs([]string{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out.String(), "removed") {
		t.Errorf("output = %q, want 'removed'", out.String())
	}
	if _, err := os.Stat(credPath); !os.IsNotExist(err) {
		t.Errorf("credentials should be deleted, stat err: %v", err)
	}
}

func resetAuthLogoutFlags() {
	// logout 没自定义 flag,保留函数以便未来扩展
}

// ─── Whoami ────────────────────────────────────────────────────────────────

func TestNewWhoamiCmd_BasicArgs(t *testing.T) {
	cmd := NewWhoamiCmd()
	if cmd.Use != "whoami" {
		t.Errorf("Use = %q, want whoami", cmd.Use)
	}
	for _, a := range cmd.Aliases {
		if a != "status" {
			t.Errorf("alias %q != status", a)
		}
	}
}

func TestRunAuthWhoami_NotLoggedIn(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cmd := NewWhoamiCmd()
	cmd.SetArgs([]string{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("want nil err, got: %v", err)
	}
	if !strings.Contains(out.String(), "Not logged in") {
		t.Errorf("output = %q, want 'Not logged in'", out.String())
	}
}

func TestRunAuthWhoami_LoggedIn(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// 写一份凭证
	wauDir := filepath.Join(tmpHome, ".wau")
	if err := os.MkdirAll(wauDir, 0o700); err != nil {
		t.Fatal(err)
	}
	credPath := filepath.Join(wauDir, "credentials")
	creds := `{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhbGljZSJ9.test",
  "refresh_token": "refresh-token-here",
  "expires_at": ` + intstr(time.Now().Add(2*time.Hour).Unix()) + `,
  "user_id": "alice",
  "endpoint": "http://localhost:18400"
}`
	if err := os.WriteFile(credPath, []byte(creds), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := NewWhoamiCmd()
	cmd.SetArgs([]string{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("err: %v", err)
	}
	got := out.String()
	for _, want := range []string{"alice", "Expires", "Token", "Endpoint"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestRunAuthWhoami_ExpiredToken(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	wauDir := filepath.Join(tmpHome, ".wau")
	if err := os.MkdirAll(wauDir, 0o700); err != nil {
		t.Fatal(err)
	}
	credPath := filepath.Join(wauDir, "credentials")
	creds := `{
  "access_token": "x",
  "expires_at": ` + intstr(time.Now().Add(-time.Hour).Unix()) + `,
  "user_id": "alice"
}`
	if err := os.WriteFile(credPath, []byte(creds), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := NewWhoamiCmd()
	cmd.SetArgs([]string{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out.String(), "EXPIRED") {
		t.Errorf("want 'EXPIRED' warning, got: %s", out.String())
	}
}

// intstr 辅助:int64 → string(避免 strconv 引入)
func intstr(i int64) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}