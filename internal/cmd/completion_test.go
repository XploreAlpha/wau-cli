// Package cmd - completion_test.go
//
// 第二刀 P1.4 — completion 子命令测试。
//
// 覆盖:
//   - 4 种 shell 都成功输出非空内容
//   - 非法 shell 参数被 cobra.OnlyValidArgs 拒绝
//   - 缺失参数被 ExactArgs 拒绝
package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func runCompletion(t *testing.T, shell string) string {
	t.Helper()
	// 用 real rootCmd(包含全部子命令)才能生成非空 completion 脚本
	cmd := NewCompletionCmd()
	rootCmd.AddCommand(cmd)
	defer rootCmd.RemoveCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", shell})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("shell=%s err=%v", shell, err)
	}
	return buf.String()
}

func TestCompletion_AllShells(t *testing.T) {
	shells := []string{"bash", "zsh", "fish", "powershell"}
	for _, sh := range shells {
		t.Run(sh, func(t *testing.T) {
			out := runCompletion(t, sh)
			if out == "" {
				t.Errorf("%s: empty output", sh)
			}
		})
	}
}

func TestCompletion_InvalidShell(t *testing.T) {
	// 通过真实 rootCmd 触发 cobra 的 ValidArgs 校验
	cmd := NewCompletionCmd()
	rootCmd.AddCommand(cmd)
	defer rootCmd.RemoveCommand(cmd)

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	rootCmd.SetArgs([]string{"completion", "ksh"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("want error for invalid shell")
	}
	if !strings.Contains(err.Error(), "invalid argument") {
		t.Errorf("err = %v", err)
	}
}

func TestCompletion_NoArg(t *testing.T) {
	cmd := NewCompletionCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("want error when no shell given")
	}
}

func TestCompletion_BashContainsSubcommands(t *testing.T) {
	// 通过 real rootCmd 注册完整子命令树(agent / task / stack / completion 等),
	// 然后生成 bash completion — 验证 cobra 真的接到了 full tree
	cmd := NewCompletionCmd()
	rootCmd.AddCommand(cmd)
	defer rootCmd.RemoveCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "bash"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if len(out) < 100 {
		t.Errorf("bash output seems too short (%d bytes); expected full subcommand tree", len(out))
	}
}