// Package stack - up.go
//
// 第一刀 1.1 — `wau stack up` 子命令。
//
// 行为:
//   1. 加载 stack(file 或 default)
//   2. 应用 profile(若指定)
//   3. 拓扑排序(按 depends_on)
//   4. 按序启服务:resolve binary → start process → health probe → 更新 runtime
//   5. 输出彩色表格
//
// 退出:
//   - 0:全部 running
//   - 1:有 required 服务失败
//   - 2:仅 optional 服务失败
package stack

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/wau/wau-cli/internal/output"
	stackpkg "github.com/wau/wau-cli/internal/stack"
)

var (
	upFile     string
	upProfile  string
	upDemo     bool
	upDryRun   bool
	upDetach   bool
	upWaitMax  time.Duration
)

func newUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Bring the stack up",
		Long: `Bring the WAU stack up — start all services in dependency order.

Reads configuration from --file (default: built-in 9-service stack), starts
each service in topological order, and waits for health probes to pass.

Flags:
  --file       Path to a custom wau-stack.yml (overrides built-in default)
  --profile    Apply a profile defined in the stack (e.g. 'demo', 'minimal')
  --demo       Shortcut for --profile=demo (9-service full stack)
  --dry-run    Print plan without starting anything
  --detach     Start in background (do not wait for health checks)
  --wait-max   Maximum time to wait for all health probes (default 60s)
  --remote     SSH address (e.g. ssh://root@host:22) — only valid with v1.1 schema

Examples:
  wau stack up                      # full built-in default stack
  wau stack up --demo               # visa demo: same as default (9 services)
  wau stack up --file my.yml        # custom stack from yaml
  wau stack up --profile minimal    # only redis + wau-core + registry
  wau stack up --dry-run            # plan only, no side effects
  wau stack up --file wau-stack.yml --remote ssh://root@43.134.126.126  # v1.1 + SSH push`,
		RunE: runUpDispatcher,
	}

	cmd.Flags().StringVar(&upFile, "file", "", "path to wau-stack.yml")
	cmd.Flags().StringVar(&upProfile, "profile", "", "apply profile from stack (e.g. demo, minimal)")
	cmd.Flags().BoolVar(&upDemo, "demo", false, "shortcut for --profile=demo")
	cmd.Flags().BoolVar(&upDryRun, "dry-run", false, "print plan without starting anything")
	cmd.Flags().BoolVar(&upDetach, "detach", false, "start in background without waiting for health")
	cmd.Flags().DurationVar(&upWaitMax, "wait-max", 60*time.Second, "max wait for all health probes")
	cmd.Flags().StringVar(&upRemote, "remote", "", "SSH address for v1.1 schema (e.g. ssh://root@host:22)")

	return cmd
}

// runUpDispatcher(4.1.5)在 runUp 之前按 schema 版本分派:
//   - --remote 给出:必须 v1.1 → runUpV11
//   - --file 给出且 version: "1.1":runUpV11
//   - 其余(v1 默认 9-service 或 v1 文件):runUp 老路径(D60 不动)
func runUpDispatcher(cmd *cobra.Command, args []string) error {
	if upRemote != "" {
		if upFile == "" {
			return fmt.Errorf("--remote requires --file pointing to a v1.1 wau-stack.yml")
		}
		if !isV11YAML(upFile) {
			return fmt.Errorf("--remote requires v1.1 schema (version: \"1.1\"); got v1 schema in %s", upFile)
		}
		return runUpV11(cmd, args)
	}
	if upFile != "" && isV11YAML(upFile) {
		return runUpV11(cmd, args)
	}
	return runUp(cmd, args)
}

func runUp(cmd *cobra.Command, args []string) error {
	// 1. profile 选择
	profile := upProfile
	if upDemo {
		if profile != "" {
			return fmt.Errorf("--demo and --profile are mutually exclusive")
		}
		profile = "demo"
	}

	// 2. 加载 stack
	s, err := loadStack(upFile, profile)
	if err != nil {
		return err
	}

	// 3. 拓扑排序
	order, err := s.TopoOrder()
	if err != nil {
		return err
	}

	// 4. dry-run 只打印 plan
	if upDryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Plan (dry-run): %d services in order:\n", len(order))
		for i, name := range order {
			svc, _ := s.ServiceByName(name)
			required := ""
			if svc.Required {
				required = " [required]"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s%s\n", i+1, name, required)
		}
		return nil
	}

	// 5. 解析数据目录
	dataDir, logDir, err := s.ResolvedDirs()
	if err != nil {
		return err
	}

	// 6. 加载 runtime(可能为空)
	rt, err := stackpkg.LoadRuntime(dataDir, s.Stack.Name)
	if err != nil {
		return err
	}
	rt.DataDir = dataDir
	rt.LogDir = logDir

	// 7. 准备 process manager
	pm := stackpkg.NewProcessManager()

	// 8. 按序启动
	fmt.Fprintf(cmd.OutOrStdout(), "Bringing up stack %q (%d services)...\n\n", s.Stack.Name, len(order))
	results := make(map[string]error)
	startedAt := time.Now()

	for i, name := range order {
		svc, _ := s.ServiceByName(name)
		if svc == nil {
			continue
		}
		// 跳过 non-binary 服务(比如 external redis,假设已存在)
		if svc.Kind != stackpkg.KindBinary {
			fmt.Fprintf(cmd.OutOrStdout(), "  ⠋ %-15s ... external (skipped startup)\n", name)
			rt.SetStatus(name, "external", 0, nil)
			continue
		}

		// 检查是否已 running
		if existing, ok := rt.Services[name]; ok && existing.Status == "running" && stackpkg.IsAlive(existing.PID) {
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %-15s already running (pid %d)\n", name, existing.PID)
			continue
		}

		// 启动
		fmt.Fprintf(cmd.OutOrStdout(), "  ⠋ %-15s ... ", name)
		startT := time.Now()
		ctx := context.Background()
		pid, err := pm.Start(ctx, svc, logDir)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "✗ (%v)\n", err)
			results[name] = err
			rt.SetStatus(name, "failed", 0, nil)
			if rt.Services[name] != nil {
				rt.Services[name].LastError = err.Error()
				_ = rt.Save()
			}
			if svc.Required {
				fmt.Fprintf(cmd.OutOrStdout(),
					"\n✗ required service %q failed; aborting stack up\n", name)
				return fmt.Errorf("required service %q failed: %w", name, err)
			}
			continue
		}
		// 写 PID 文件
		pidPath := stackpkg.PidFilePath(dataDir, name)
		if err := stackpkg.WritePIDFile(pidPath, pid); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "✗ (pid file: %v)\n", err)
		}
		// 更新 runtime
		stateExtras := map[string]interface{}{
			"binary":     svc.Binary,
			"binaryPath": pm.Lookup.Home, // best effort
			"logFile":    fmt.Sprintf("%s/%s.log", logDir, name),
			"httpPort":   svc.HTTPPort,
			"grpcPort":   svc.GRPCPort,
		}
		if err := rt.SetStatus(name, "running", pid, stateExtras); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "✗ (runtime save: %v)\n", err)
		}

		// 健康探针(可选)
		if !upDetach && svc.Health != nil {
			probe, perr := stackpkg.NewProbeRunner(svc.Health, name)
			if perr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "✗ (probe init: %v)\n", perr)
				continue
			}
			probeCtx, cancel := context.WithTimeout(ctx, upWaitMax)
			result := probe.Run(probeCtx)
			cancel()
			if !result.OK {
				fmt.Fprintf(cmd.OutOrStdout(), "✗ (health: %v after %v)\n", result.Err, result.Elapsed.Truncate(100*time.Millisecond))
				results[name] = result.Err
				if svc.Required {
					return fmt.Errorf("required service %q health check failed: %w", name, result.Err)
				}
				continue
			}
			took := time.Since(startT).Truncate(100 * time.Millisecond)
			fmt.Fprintf(cmd.OutOrStdout(), "✓ (%.1fs)\n", took.Seconds())
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "✓ (pid %d)\n", pid)
		}
		_ = i
	}

	// 9. 汇总
	total := time.Since(startedAt).Truncate(100 * time.Millisecond)
	fmt.Fprintf(cmd.OutOrStdout(),
		"\n✓ Stack %q up. %d services, took %s.\n",
		s.Stack.Name, len(order), total)
	if len(results) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(),
			"⚠ %d service(s) had issues: %v\n", len(results), results)
		// optional-only 失败退出 2,required 失败已在上面 abort
		return exitCodeError(2)
	}

	// 输出友好提示
	fmt.Fprintf(cmd.OutOrStdout(),
		"\nNext: wau stack ls | wau agent search medical | wau task submit \"...\"\n")

	// 输出 progress bar demo(展示 progress helper 正常)
	_ = output.Progress // keep import
	return nil
}

// exitCodeError 实现 Cobra 的 ExitCode 接口(让 runE 能返回特定退出码)。
type exitCodeError int

func (e exitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", int(e))
}

func (exitCodeError) ExitCode() int {
	return 2
}

// keep os import referenced
var _ = os.Stderr
