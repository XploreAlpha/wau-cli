package stack

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestNewValidateCmd_Flags — flag 注册正确。
func TestNewValidateCmd_Flags(t *testing.T) {
	cmd := NewValidateCmd()
	if cmd.Use != "validate" {
		t.Errorf("Use = %q, want validate", cmd.Use)
	}
	for _, want := range []string{"file", "level", "output"} {
		if cmd.Flags().Lookup(want) == nil {
			t.Errorf("flag --%s not registered", want)
		}
	}
}

// TestRunValidate_DefaultStack — 默认 embed YAML 跑 validate 应该 healthy 或仅 warnings。
func TestRunValidate_DefaultStack(t *testing.T) {
	cmd := NewValidateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.ParseFlags([]string{}); err != nil {
		t.Fatal(err)
	}
	// 不要真跑 exit,只验 report.String() 包含 stack_id
	out := cmd.OutOrStdout().(*bytes.Buffer)
	validateFile = ""
	validateLevel = "basic"
	validateFormat = "table"
	if err := runValidate(cmd, nil); err != nil {
		// basic 不查 binary,应该 0 error/warning(healthcheck 缺失是 info,不计数)
		// 但 default stack 所有服务都有 healthcheck 所以 0 issue
		t.Fatalf("runValidate: %v (out=%s)", err, out.String())
	}
	if !strings.Contains(out.String(), "wau-default") {
		t.Errorf("output missing stack_id 'wau-default': %s", out.String())
	}
}

// TestRunValidate_BadYAML — 不可解析的 YAML 报 parse error(exit code 1)。
func TestRunValidate_BadYAML(t *testing.T) {
	cmd := NewValidateCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	validateFile = "" // 用 default
	validateLevel = "basic"
	validateFormat = "table"
	// 写一个临时文件,用 version 错的
	tmp := t.TempDir() + "/bad.yml"
	if err := writeYAMLFile(tmp, "version: \"99\"\nstack_id: x\nservices: { a: { binary: x } }"); err != nil {
		t.Fatal(err)
	}
	validateFile = tmp
	err := runValidate(cmd, nil)
	if err == nil {
		t.Fatal("expected error for bad YAML")
	}
	// 应该 exit code 1(parse error)
	if !strings.Contains(err.Error(), "exit code 1") {
		t.Errorf("err = %v, want exit code 1", err)
	}
}

// helper
func writeYAMLFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}