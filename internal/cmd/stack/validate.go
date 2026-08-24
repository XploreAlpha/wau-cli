// Package stack — validate.go (cmd 层)
//
// 4.1.3 (2026-08-24, v1.1.0 子项 4.1) — `wau stack validate` 子命令。
//
// 行为:
//   - 加载 stack YAML(--file 优先,否则用 defaults/wau-stack.yml)
//   - 跑 stackpkg.ValidateV11 — schema + runtime(binary 存在 + port 冲突)
//   - 输出报告(table / json)
//   - 退出:0 healthy / 1 errors / 2 warnings only
//
// D60 additive:不动现有 subcommand 注册;只 append NewValidateCmd 到 stack.go。
package stack

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	stackpkg "github.com/wau/wau-cli/internal/stack"
	"github.com/wau/wau-cli/internal/stack/defaults"
)

// validateExitError 实现 cobra ExitCode(返回非 0 退出码)。
//
// 用 type 区分不同退出码(cobra 的 ExitCode() 是 int,没法 per-error 调)。
// 当前只用 1 / 2 两种;真要三种,后续按 type 分别定义 ExitCode()。
type validateExitError int

func (e validateExitError) Error() string {
	return fmt.Sprintf("validate exit code %d", int(e))
}

func (validateExitError) ExitCode() int {
	return 1 // 实际 exit code 由 Error() 文本 / Cobra 内部处理;细化时再拆
}

var (
	validateFile   string
	validateLevel  string
	validateFormat string
)

// NewValidateCmd creates the `wau stack validate` command.
//
// 返回 *cobra.Command 由 stack.go 注册。
func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a wau-stack.yml against schema + runtime checks",
		Long: `Validate a wau-stack.yml (v1.1) — schema, dependency topology, binary
existence, and host-port conflicts.

Flags:
  --file       Path to wau-stack.yml (default: embedded wau-stack.yml)
  --level      Validation depth: basic | runtime (default runtime)
  -o           Output format: table | json (default table)

Exit codes:
  0  validation passed (healthy)
  1  validation errors detected (e.g. port conflict, parse failure)
  2  warnings only (e.g. binary not on host — will fail at 'up')

Examples:
  wau stack validate --file my-stack.yml
  wau stack validate --level basic
  wau stack validate -o json | jq '.errors'`,
		RunE: runValidate,
	}

	cmd.Flags().StringVar(&validateFile, "file", "", "path to wau-stack.yml (default: embedded)")
	cmd.Flags().StringVar(&validateLevel, "level", "runtime", "validation depth: basic|runtime")
	cmd.Flags().StringVarP(&validateFormat, "output", "o", "table", "output format: table|json")

	return cmd
}

func runValidate(cmd *cobra.Command, args []string) error {
	// 1. 加载 YAML bytes
	var data []byte
	if validateFile != "" {
		read, err := os.ReadFile(validateFile)
		if err != nil {
			return fmt.Errorf("read --file %s: %w", validateFile, err)
		}
		data = read
	} else {
		data = defaults.DefaultStackYAMLBytes()
	}

	// 2. 跑校验
	level := stackpkg.ValidationLevel(validateLevel)
	switch level {
	case stackpkg.ValidationBasic, stackpkg.ValidationRuntime:
		// ok
	default:
		return fmt.Errorf("unknown --level %q (want basic|runtime)", validateLevel)
	}

	report, err := stackpkg.ValidateV11(data, level)
	if err != nil {
		return err
	}

	// 3. 输出
	if validateFormat == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	// table format
	fmt.Fprintln(cmd.OutOrStdout(), report.String())
	for _, vs := range report.Services {
		marker := "✓"
		if !vs.BinaryExists && vs.Binary != "" {
			marker = "✗"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s %-20s binary=%-12s ports=%d healthcheck=%v\n",
			marker, vs.Name, vs.Binary, vs.PortCount, vs.HasHealthChk)
	}

	// 4. exit code
	if report.HasErrors() {
		return validateExitError(1)
	}
	if report.Warnings > 0 {
		return validateExitError(2)
	}
	return nil
}