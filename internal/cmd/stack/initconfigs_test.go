package stack

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── NewInitConfigsCmd flag 注册 ──────────────────────────────────────────

func TestNewInitConfigsCmd_BasicArgs(t *testing.T) {
	cmd := NewInitConfigsCmd()
	if cmd.Use != "init-configs" {
		t.Errorf("Use = %q, want init-configs", cmd.Use)
	}
	for _, name := range []string{"service", "output-dir", "force", "dry-run"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag %q missing", name)
		}
	}
}

func TestRunInitConfigs_DryRun(t *testing.T) {
	resetInitConfigsFlags()
	cmd := NewInitConfigsCmd()
	cmd.SetArgs([]string{"--dry-run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Dry run") {
		t.Errorf("missing 'Dry run' in output: %s", got)
	}
	if !strings.Contains(got, "wau-store") {
		t.Errorf("missing 'wau-store' in output: %s", got)
	}
}

func TestRunInitConfigs_WriteAll(t *testing.T) {
	resetInitConfigsFlags()
	dir := t.TempDir()
	cmd := NewInitConfigsCmd()
	cmd.SetArgs([]string{"--output-dir", dir})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "4 wrote") {
		t.Errorf("missing '4 wrote' in output: %s", got)
	}
	// 4 个文件都应存在
	for _, f := range []string{"store.yaml", "router.yaml", "edge.yaml", "channel.yaml"} {
		p := filepath.Join(dir, f)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestRunInitConfigs_ServiceFilter(t *testing.T) {
	resetInitConfigsFlags()
	dir := t.TempDir()
	cmd := NewInitConfigsCmd()
	cmd.SetArgs([]string{"--service", "wau-store", "--output-dir", dir})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "1 wrote") {
		t.Errorf("missing '1 wrote' in output: %s", got)
	}
	// 只有 store.yaml
	if _, err := os.Stat(filepath.Join(dir, "store.yaml")); err != nil {
		t.Errorf("missing store.yaml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "router.yaml")); !os.IsNotExist(err) {
		t.Errorf("router.yaml should NOT exist: %v", err)
	}
}

func TestRunInitConfigs_ServiceNotFound(t *testing.T) {
	resetInitConfigsFlags()
	cmd := NewInitConfigsCmd()
	cmd.SetArgs([]string{"--service", "wau-ghost"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("want error for unknown service")
	}
	if !strings.Contains(err.Error(), "no template") {
		t.Errorf("err = %v, want 'no template'", err)
	}
}

func TestRunInitConfigs_SkipAndForce(t *testing.T) {
	resetInitConfigsFlags()
	dir := t.TempDir()
	// 预先写一个
	if err := os.WriteFile(filepath.Join(dir, "store.yaml"), []byte("# pre\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 第一次跑:应 skip store.yaml,写其他 3 个
	cmd := NewInitConfigsCmd()
	cmd.SetArgs([]string{"--output-dir", dir})
	var out1 bytes.Buffer
	cmd.SetOut(&out1)
	cmd.SetErr(&out1)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out1.String(), "3 wrote, 1 skipped") {
		t.Errorf("want '3 wrote, 1 skipped', got: %s", out1.String())
	}

	// 第二次跑 --force:应 4 个全 overwrite
	resetInitConfigsFlags()
	cmd2 := NewInitConfigsCmd()
	cmd2.SetArgs([]string{"--output-dir", dir, "--force"})
	var out2 bytes.Buffer
	cmd2.SetOut(&out2)
	cmd2.SetErr(&out2)
	if err := cmd2.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out2.String(), "4 wrote") {
		t.Errorf("want '4 wrote', got: %s", out2.String())
	}
}

// ─── helpers ───────────────────────────────────────────────────────────────

func resetInitConfigsFlags() {
	flagInitService = ""
	flagInitOutputDir = "~/.wau/configs"
	flagInitForce = false
	flagInitDryRun = false
	flagInitEnvSubst = false
}

// ─── P4.5 envsubst flag tests ───────────────────────────────────────────────

func TestNewInitConfigsCmd_EnvSubstFlag(t *testing.T) {
	cmd := NewInitConfigsCmd()
	if cmd.Flags().Lookup("envsubst") == nil {
		t.Fatal("--envsubst flag missing")
	}
}

func TestRunInitConfigs_EnvSubstDryRun_ShowsVars(t *testing.T) {
	t.Setenv("WAU_STORE_PG_DSN", "postgres://demo:demo@localhost:5432/x")
	t.Setenv("WAU_STORE_REDIS_PASSWORD", "")
	t.Setenv("WAU_STORE_ADMIN_TOKEN", "token123")

	resetInitConfigsFlags()
	flagInitDryRun = true
	flagInitEnvSubst = true
	defer resetInitConfigsFlags()

	cmd := NewInitConfigsCmd()
	cmd.SetArgs([]string{"--service", "wau-store", "--dry-run", "--envsubst"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	// 应显示 "Variables referenced: ..." 一行
	if !strings.Contains(got, "Variables referenced") {
		t.Errorf("missing 'Variables referenced' in output:\n%s", got)
	}
	// WAU_STORE_PG_DSN 是 set 的 → ✓ 标记
	if !strings.Contains(got, "✓$WAU_STORE_PG_DSN") {
		t.Errorf("missing '✓$WAU_STORE_PG_DSN' marker:\n%s", got)
	}
	// WAU_STORE_REDIS_PASSWORD 是空 → ✗ 标记
	if !strings.Contains(got, "✗$WAU_STORE_REDIS_PASSWORD") {
		t.Errorf("missing '✗$WAU_STORE_REDIS_PASSWORD' marker:\n%s", got)
	}
}

func TestRunInitConfigs_EnvSubst_WritesExpanded(t *testing.T) {
	t.Setenv("WAU_STORE_PG_DSN", "postgres://demo:demo@localhost:5432/x")
	t.Setenv("WAU_STORE_PG_PASSWORD", "pgpass")
	t.Setenv("WAU_STORE_REDIS_PASSWORD", "redispass")
	t.Setenv("WAU_STORE_ADMIN_TOKEN", "admintoken")

	resetInitConfigsFlags()
	dir := t.TempDir()
	cmd := NewInitConfigsCmd()
	cmd.SetArgs([]string{
		"--service", "wau-store",
		"--output-dir", dir,
		"--envsubst",
		"--force",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// 检查写出文件内容
	data, err := os.ReadFile(filepath.Join(dir, "store.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "$WAU_STORE_PG_DSN") {
		t.Errorf("$VAR not substituted in written file:\n%s", string(data))
	}
	if !strings.Contains(string(data), "postgres://demo:demo@localhost:5432/x") {
		t.Errorf("expanded DSN not in file:\n%s", string(data))
	}
	// 全部 set 后应 exit 0(无 unsubstituted warning)
	if !strings.Contains(out.String(), "1 wrote") {
		t.Errorf("want '1 wrote', got:\n%s", out.String())
	}
}

func TestRunInitConfigs_EnvSubst_MissingEnv_WarnsExit2(t *testing.T) {
	// 不 set 任何 env → 写完文件里全是空字符串 → unsubstituted warning + exit 2
	t.Setenv("WAU_STORE_PG_DSN", "")
	t.Setenv("WAU_STORE_REDIS_PASSWORD", "")
	t.Setenv("WAU_STORE_ADMIN_TOKEN", "")

	resetInitConfigsFlags()
	dir := t.TempDir()
	cmd := NewInitConfigsCmd()
	cmd.SetArgs([]string{
		"--service", "wau-store",
		"--output-dir", dir,
		"--envsubst",
		"--force",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("want exit 2 for unsubstituted vars")
	}
	// 检查 exit code
	if ee, ok := err.(interface{ ExitCode() int }); ok {
		if ee.ExitCode() != 2 {
			t.Errorf("ExitCode = %d, want 2", ee.ExitCode())
		}
	} else {
		t.Errorf("err not exitCodeError: %T", err)
	}
	if !strings.Contains(out.String(), "were empty") {
		t.Errorf("missing 'were empty' warning:\n%s", out.String())
	}
}