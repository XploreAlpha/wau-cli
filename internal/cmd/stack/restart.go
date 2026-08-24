// Package stack - restart.go
//
// 第四刀 4.4 — `wau stack restart [service...]` 子命令。
//
// 行为:`down <svc>` + `up <svc>` 的组合,跟 docker-compose / kubectl 同语义。
//
//   - 无 args:全栈按 topo 反序 down + 正序 up
//   - 有 args:只 restart 指定的 services(按给的顺序)
//
// P4.4 — 复用 pm.Stop / pm.Start + rt.SetStatus,不重写 down/up 的逻辑。
package stack

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	stackpkg "github.com/wau/wau-cli/internal/stack"
)

var (
	restartFile    string
	restartProfile string
	restartWaitMax time.Duration
)

// NewRestartCmd 构造 `wau stack restart` 子命令。
func NewRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart [service...]",
		Short: "Restart one or more services (down + up)",
		Long: `Restart services — equivalent to 'down <svc>' followed by 'up <svc>'.

With no arguments, restarts the entire stack in dependency-aware order
(reverse topology for stop, forward topology for start). With service
names, restarts only those services in the order given.

Flags:
  --file       Path to a custom wau-stack.yml (matches up/down)
  --profile    Profile to filter the stack (matches up/down)
  --wait-max   Maximum time to wait for health probes after restart (default 60s)

Examples:
  wau stack restart                       # full stack
  wau stack restart wau-core              # one service
  wau stack restart wau-core wau-router   # multiple services
  wau stack restart --wait-max 120s       # longer health wait`,
		Aliases: []string{"reload"},
		RunE:    runRestart,
	}

	cmd.Flags().StringVar(&restartFile, "file", "", "path to wau-stack.yml")
	cmd.Flags().StringVar(&restartProfile, "profile", "", "apply profile from stack")
	cmd.Flags().DurationVar(&restartWaitMax, "wait-max", 60*time.Second, "max wait for health probes after restart")

	return cmd
}

func runRestart(cmd *cobra.Command, args []string) error {
	// 1. 加载 stack
	s, err := loadStack(restartFile, restartProfile)
	if err != nil {
		return err
	}

	// 2. 决定要 restart 的 services
	var targets []string
	if len(args) == 0 {
		// 全栈:按 topo 顺序(restart = reverse down + forward up,所以 up 阶段按 topo)
		order, err := s.TopoOrder()
		if err != nil {
			return err
		}
		targets = order
	} else {
		// 用户指定:校验 service 存在
		for _, name := range args {
			svc, ok := s.ServiceByName(name)
			if !ok || svc == nil {
				return fmt.Errorf("service %q not in stack (use 'wau stack ls' to list available services)", name)
			}
			targets = append(targets, name)
		}
	}

	if len(targets) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(),
			"No services to restart. Use 'wau stack up' first to populate the runtime.")
		return nil
	}

	// 3. 准备数据目录 + runtime + pm
	dataDir, logDir, err := s.ResolvedDirs()
	if err != nil {
		return err
	}
	rt, err := stackpkg.LoadRuntime(dataDir, s.Stack.Name)
	if err != nil {
		return err
	}
	rt.DataDir = dataDir
	rt.LogDir = logDir

	pm := stackpkg.NewProcessManager()

	// 4. 计算 down 阶段顺序
	//    - 全栈:反序(targets 已经是 topo 正序,down 走反序)
	//    - 指定:按给的顺序 down(用户负责顺序)
	var downOrder []string
	if len(args) == 0 {
		downOrder = reverseStrings(targets)
	} else {
		downOrder = targets
	}

	fmt.Fprintf(cmd.OutOrStdout(),
		"Restarting %d service(s) in stack %q...\n\n", len(targets), s.Stack.Name)
	startedAt := time.Now()

	// 5. Phase 1: stop
	failedDown := []string{}
	for _, name := range downOrder {
		fmt.Fprintf(cmd.OutOrStdout(), "  ⠋ %-15s (stop) ... ", name)
		if err := stopOne(name, rt, pm, dataDir); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "✗ (%v)\n", err)
			failedDown = append(failedDown, name)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "✓")
		}
	}

	// 6. Phase 2: start
	failedUp := []string{}
	healthWarn := []string{}
	for _, name := range targets {
		fmt.Fprintf(cmd.OutOrStdout(), "  ⠋ %-15s (start) ... ", name)
		if err := startOne(cmd, name, s, rt, pm, logDir); err != nil {
			// 区分 "start failed"(exit 1)vs "health warning"(exit 2,进程已起)
			if strings.Contains(err.Error(), "health warning") {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠ (%v)\n", err)
				healthWarn = append(healthWarn, name)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "✗ (%v)\n", err)
				failedUp = append(failedUp, name)
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "✓")
		}
	}

	total := time.Since(startedAt).Truncate(100 * time.Millisecond)
	fmt.Fprintf(cmd.OutOrStdout(),
		"\n✓ Restart of stack %q completed in %s.\n",
		s.Stack.Name, total)

	if len(failedUp) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(),
			"⚠ %d service(s) failed to start after restart: %v\n",
			len(failedUp), failedUp)
		return exitCodeError(1)
	}
	if len(healthWarn) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(),
			"⚠ %d service(s) started but health check did not pass within --wait-max: %v\n",
			len(healthWarn), healthWarn)
		return exitCodeError(2)
	}
	if len(failedDown) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(),
			"⚠ %d service(s) failed to stop: %v (but were re-started)\n",
			len(failedDown), failedDown)
		return exitCodeError(2)
	}
	return nil
}

// stopOne 停掉一个 service(复用 process.go 的 Stop / KillProcessGroup)。
//
// 不存在 / 不在 runtime / 已 stopped → 当成功(幂等)。
func stopOne(name string, rt *stackpkg.Runtime, pm *stackpkg.ProcessManager, dataDir string) error {
	state, ok := rt.Services[name]
	if !ok {
		// 没起过:跳过
		return nil
	}
	// external 类型(redis 之类)不归本地管
	if state.Status == "external" {
		return nil
	}
	// 已不 alive
	if !stackpkg.IsAlive(state.PID) {
		_ = rt.SetStatus(name, "stopped", 0, nil)
		pidPath := stackpkg.PidFilePath(dataDir, name)
		_ = removeFile(pidPath)
		return nil
	}

	// SIGTERM → 等 5s → SIGKILL
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := pm.Stop(ctx, state.PID); err != nil {
		_ = rt.SetStatus(name, "failed", state.PID, nil)
		return fmt.Errorf("stop: %w", err)
	}
	_ = stackpkg.KillProcessGroup(state.PID)
	if err := rt.SetStatus(name, "stopped", 0, nil); err != nil {
		return fmt.Errorf("save stopped: %w", err)
	}
	pidPath := stackpkg.PidFilePath(dataDir, name)
	_ = removeFile(pidPath)
	return nil
}

// startOne 启动一个 service(复用 process.go 的 Start + 健康探针)。
//
// 写 PID + 更新 runtime + (可选)等 health。
func startOne(cmd *cobra.Command, name string, s *stackpkg.Stack, rt *stackpkg.Runtime, pm *stackpkg.ProcessManager, logDir string) error {
	svc, _ := s.ServiceByName(name)
	if svc == nil {
		return fmt.Errorf("service %q vanished from stack", name)
	}
	// external 类型跳启动
	if svc.Kind != stackpkg.KindBinary {
		_ = rt.SetStatus(name, "external", 0, nil)
		return nil
	}

	ctx := context.Background()
	pid, err := pm.Start(ctx, svc, logDir)
	if err != nil {
		_ = rt.SetStatus(name, "failed", 0, nil)
		if rt.Services[name] != nil {
			rt.Services[name].LastError = err.Error()
			_ = rt.Save()
		}
		return fmt.Errorf("start: %w", err)
	}

	// 写 PID
	pidPath := stackpkg.PidFilePath(rt.DataDir, name)
	_ = stackpkg.WritePIDFile(pidPath, pid)

	// 更新 runtime
	stateExtras := map[string]interface{}{
		"binary":     svc.Binary,
		"binaryPath": pm.Lookup.Home,
		"logFile":    filepath.Join(logDir, name+".log"),
		"httpPort":   svc.HTTPPort,
		"grpcPort":   svc.GRPCPort,
	}
	if err := rt.SetStatus(name, "running", pid, stateExtras); err != nil {
		return fmt.Errorf("save running: %w", err)
	}

	// 健康探针(可选)— 失败**不**当成 restart 失败(进程已起),
	// 只记录 warning + LastError。让用户从 `wau stack ls` 看到 "running" 但 health 标 ⚠。
	if svc.Health != nil {
		probe, perr := stackpkg.NewProbeRunner(svc.Health, name)
		if perr != nil {
			return nil
		}
		probeCtx, cancel := context.WithTimeout(ctx, restartWaitMax)
		result := probe.Run(probeCtx)
		cancel()
		if !result.OK {
			if rt.Services[name] != nil {
				rt.Services[name].LastError = fmt.Sprintf("health: %v", result.Err)
				_ = rt.Save()
			}
			// 返回 special sentinel 让 caller 区分"start failed" vs "health warning"
			return fmt.Errorf("health warning (process up, pid %d): %w", pid, result.Err)
		}
	}
	return nil
}

// reverseStrings 返回 s 的反序副本(不修改原 slice)。
func reverseStrings(s []string) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return out
}