package stack

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newRootCmdForTest() *cobra.Command {
	// 构造一个 root cmd + stack 子命令,避免依赖 wau-cli 全局 root 的 accessor
	root := &cobra.Command{Use: "wau"}
	root.AddCommand(NewStackCmd())
	return root
}

func executeCmd(t *testing.T, args []string) (string, error) {
	t.Helper()
	root := newRootCmdForTest()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestUpCmd_DryRun_Default(t *testing.T) {
	out, err := executeCmd(t, []string{"stack", "up", "--dry-run"})
	if err != nil {
		t.Fatalf("Execute: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Plan (dry-run): 10 services in order") {
		t.Errorf("expected 10 services plan, got:\n%s", out)
	}
	if !strings.Contains(out, "wau-core") {
		t.Errorf("output should mention wau-core:\n%s", out)
	}
}

func TestUpCmd_DryRun_Demo(t *testing.T) {
	out, err := executeCmd(t, []string{"stack", "up", "--demo", "--dry-run"})
	if err != nil {
		t.Fatalf("Execute: %v\n%s", err, out)
	}
	if !strings.Contains(out, "redis") {
		t.Error("demo should include redis")
	}
}

func TestUpCmd_DryRun_Minimal(t *testing.T) {
	out, err := executeCmd(t, []string{"stack", "up", "--profile", "minimal", "--dry-run"})
	if err != nil {
		t.Fatalf("Execute: %v\n%s", err, out)
	}
	if !strings.Contains(out, "3 services in order") {
		t.Errorf("minimal should have 3 services, got:\n%s", out)
	}
}

func TestUpCmd_DryRun_FileOverride(t *testing.T) {
	// 临时 yaml
	tmpDir := t.TempDir()
	yamlPath := tmpDir + "/custom.yml"
	yamlContent := `version: "1"
stack:
  name: custom
services:
  - name: foo
    binary: foo
  - name: bar
    binary: bar
    depends_on: [foo]
`
	if err := writeFile(yamlPath, yamlContent); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	out, err := executeCmd(t, []string{"stack", "up", "--file", yamlPath, "--dry-run"})
	if err != nil {
		t.Fatalf("Execute: %v\n%s", err, out)
	}
	if !strings.Contains(out, "2 services in order") {
		t.Errorf("custom file should have 2 services, got:\n%s", out)
	}
	if !strings.Contains(out, "foo") || !strings.Contains(out, "bar") {
		t.Error("output should mention both services")
	}
}

func TestUpCmd_DryRun_FileNotExist(t *testing.T) {
	_, err := executeCmd(t, []string{"stack", "up", "--file", "/nonexistent/never.yml", "--dry-run"})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLsCmd_EmptyState(t *testing.T) {
	out, err := executeCmd(t, []string{"stack", "ls"})
	if err != nil {
		t.Fatalf("Execute: %v\n%s", err, out)
	}
	if !strings.Contains(out, "No services tracked") {
		t.Errorf("expected helpful empty message, got:\n%s", out)
	}
}

// writeFile is a tiny helper for tests.
func writeFile(path, content string) error {
	return osWriteFile(path, []byte(content), 0o644)
}

// indirection so we don't import os here
var osWriteFile = func(path string, data []byte, mode uint32) error {
	return osWriteFileImpl(path, data, mode)
}
