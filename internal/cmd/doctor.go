// Package cmd - doctor.go
//
// 4.3 MVP (2026-08-24, v1.1.0 super-binary) — `wau doctor` 极简诊断。
//
// 检查 5 件事(全部离线,无网络/无 SSH):
//   1. wau-cli version
//   2. ~/.wau 目录存在且可写
//   3. WAU_* env vars(数量,不打值,脱敏 per [[feedback-api-key-leak-2026-07-30]])
//   4. 关键 binary 在 PATH(wau-core / wau-registry / wau-agent)
//   5. `wau network` DNS / hosts 文件存在 ~/.wau/hosts(可选)
//
// 输出彩色表格 + exit code:
//   - 0:全部 OK
//   - 1:有 failed 项
//   - 2:全部 OK 但有 warning
//
// 范围明确:不做网络探测、不做 SSH push probe、不跑 healthcheck(那些在 `wau status` 里)。
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wau/wau-cli/internal/version"
)

// doctorCheck 一项检查 + 结果。
type doctorCheck struct {
	Name   string
	Status string // "ok" / "warn" / "fail"
	Detail string
}

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose wau-cli environment (offline checks)",
		Long: `Run a quick offline health check of the wau-cli environment.

Checks:
  - wau-cli version
  - ~/.wau directory writable
  - WAU_* environment variables present (values masked)
  - critical binaries in PATH (wau-core / wau-registry / wau-agent)
  - ~/.wau/hosts file (optional)

For live service health use 'wau status'.

Examples:
  wau doctor
  wau doctor --format json`,
		Aliases:        []string{"diag"},
		SilenceErrors:  true, // 自己控制错误打印,避免 cobra 双 print
		SilenceUsage:   true, // 出错时不打 usage
		RunE:           runDoctor,
	}
	// 用 --format 而不是 --output(避免与 root 的 -o/--output 冲突)
	cmd.Flags().StringP("format", "f", "table", "output format: table|json")
	return cmd
}

// doctorResult 聚合检查结果,JSON 序列化用。
type doctorResult struct {
	Version string        `json:"version"`
	OK      bool          `json:"ok"`
	Checks  []doctorCheck `json:"checks"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	format, _ := cmd.Flags().GetString("format")

	result := doctorResult{
		Version: fmt.Sprintf("wau-cli %s", version.Version),
		Checks:  runDoctorChecks(),
	}
	// 计算 OK
	hasFail, hasWarn := false, false
	for _, c := range result.Checks {
		switch c.Status {
		case "fail":
			hasFail = true
		case "warn":
			hasWarn = true
		}
	}
	result.OK = !hasFail

	if format == "json" {
		// 写 JSON — 简化,不用 encoding/json 以免加 import 路径负担
		fmt.Fprintf(out, `{"version":%q,"ok":%t,"checks":[`, result.Version, result.OK)
		for i, c := range result.Checks {
			if i > 0 {
				fmt.Fprint(out, ",")
			}
			fmt.Fprintf(out, `{"name":%q,"status":%q,"detail":%q}`, c.Name, c.Status, c.Detail)
		}
		fmt.Fprintln(out, "]}")
	} else {
		fmt.Fprintf(out, "\n%s\n\n", result.Version)
		fmt.Fprintf(out, "%-30s %-6s  %s\n", "CHECK", "STATUS", "DETAIL")
		fmt.Fprintf(out, "%-30s %-6s  %s\n", strings.Repeat("-", 30), "------", strings.Repeat("-", 30))
		for _, c := range result.Checks {
			marker := "✓"
			switch c.Status {
			case "warn":
				marker = "⚠"
			case "fail":
				marker = "✗"
			}
			fmt.Fprintf(out, "%-30s %s %-4s  %s\n", c.Name, marker, c.Status, c.Detail)
		}
		fmt.Fprintln(out)
	}

	if hasFail {
		fmt.Fprintln(cmd.ErrOrStderr(), "✗ doctor: some checks failed.")
		return doctorExitError(1)
	}
	if hasWarn {
		fmt.Fprintln(cmd.ErrOrStderr(), "⚠ doctor: all checks passed with warnings.")
		return doctorExitError(2)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "✓ doctor: all checks passed.")
	return nil
}

// runDoctorChecks 跑 5 项检查。
func runDoctorChecks() []doctorCheck {
	checks := []doctorCheck{
		checkWauDir(),
		checkEnvVars(),
		checkBinaries(),
	}
	checks = append(checks, checkHostsFile())
	return checks
}

// checkWauDir 检查 ~/.wau 存在 + 可写。
func checkWauDir() doctorCheck {
	home, err := os.UserHomeDir()
	if err != nil {
		return doctorCheck{"~/.wau directory", "fail", fmt.Sprintf("UserHomeDir: %v", err)}
	}
	wauDir := filepath.Join(home, ".wau")
	if _, err := os.Stat(wauDir); err != nil {
		if os.IsNotExist(err) {
			// 自动尝试创建(per D60 additive 不动 stack package,但 doctor 这层是新增,允许 mkdir)
			if mkErr := os.MkdirAll(wauDir, 0o755); mkErr != nil {
				return doctorCheck{"~/.wau directory", "fail",
					fmt.Sprintf("not exists at %s and mkdir failed: %v", wauDir, mkErr)}
			}
			return doctorCheck{"~/.wau directory", "ok",
				fmt.Sprintf("created at %s", wauDir)}
		}
		return doctorCheck{"~/.wau directory", "fail", fmt.Sprintf("stat %s: %v", wauDir, err)}
	}
	// 可写测试
	probe := filepath.Join(wauDir, ".doctor-write-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return doctorCheck{"~/.wau directory", "fail",
			fmt.Sprintf("not writable at %s: %v", wauDir, err)}
	}
	_ = os.Remove(probe)
	return doctorCheck{"~/.wau directory", "ok", fmt.Sprintf("writable at %s", wauDir)}
}

// checkEnvVars 检查 WAU_* env vars 数量 + 脱敏 per [[feedback-api-key-leak-2026-07-30]]。
func checkEnvVars() doctorCheck {
	wauVars := []string{}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "WAU_") && !strings.Contains(kv, "=") {
			continue
		}
		idx := strings.Index(kv, "=")
		if idx < 0 {
			continue
		}
		name := kv[:idx]
		if strings.HasPrefix(name, "WAU_") {
			wauVars = append(wauVars, name)
		}
	}
	if len(wauVars) == 0 {
		return doctorCheck{"WAU_* env vars", "warn",
			"none set — agent / cluster login may not work"}
	}
	// 脱敏:只列名字,不打值
	return doctorCheck{"WAU_* env vars", "ok",
		fmt.Sprintf("%d set: %s", len(wauVars), strings.Join(wauVars, ", "))}
}

// checkBinaries 检查关键 binary 是否在 PATH。
func checkBinaries() doctorCheck {
	critical := []string{"wau-core", "wau-registry", "wau-agent"}
	found := []string{}
	missing := []string{}
	for _, name := range critical {
		if _, err := exec.LookPath(name); err == nil {
			found = append(found, name)
		} else {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return doctorCheck{"critical binaries", "ok",
			fmt.Sprintf("%d/%d in PATH: %s", len(found), len(critical), strings.Join(found, ", "))}
	}
	if len(found) == 0 {
		return doctorCheck{"critical binaries", "fail",
			fmt.Sprintf("none of %s in PATH — run 'go install' or add to PATH",
				strings.Join(critical, ", "))}
	}
	return doctorCheck{"critical binaries", "warn",
		fmt.Sprintf("missing: %s (found: %s)",
			strings.Join(missing, ", "), strings.Join(found, ", "))}
}

// checkHostsFile 检查 ~/.wau/hosts 是否存在(可选,wau-network DNS 用)。
func checkHostsFile() doctorCheck {
	home, err := os.UserHomeDir()
	if err != nil {
		return doctorCheck{"~/.wau/hosts", "warn", "UserHomeDir unavailable"}
	}
	hostsPath := filepath.Join(home, ".wau", "hosts")
	if _, err := os.Stat(hostsPath); err != nil {
		return doctorCheck{"~/.wau/hosts", "warn",
			fmt.Sprintf("not found at %s (optional; needed for wau-network DNS)", hostsPath)}
	}
	return doctorCheck{"~/.wau/hosts", "ok", fmt.Sprintf("present at %s", hostsPath)}
}

// doctorExitError 实现 cobra ExitCode。
//
// 完全静默(Error() 返回固定 marker + ErrorMarker 返回 false),避免 cobra 在 RunE
// 返回错误时再打 "Error: ..." 前缀。我们自己已经把 summary 打到 stderr 了。
type doctorExitError int

const doctorErrorMarker = "doctor_silent_exit_marker_do_not_print"

func (e doctorExitError) Error() string { return doctorErrorMarker }

// SilenceError 告诉 cobra 不要打印这个 error(实现 cobra 的 Silent interface)。
// 见 cobra.Command.SilenceError 文档 / 内置 silentErr 类型。
func (doctorExitError) SilenceError() bool { return true }
func (doctorExitError) ExitCode() int      { return 1 }
