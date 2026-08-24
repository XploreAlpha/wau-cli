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
}