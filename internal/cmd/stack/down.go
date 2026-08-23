// Package stack - down.go
//
// 第一刀 1.1 — `wau stack down` 子命令。
//
// 行为:按拓扑反序停止所有 running 服务,SIGTERM → 5s → SIGKILL。
package stack

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	stackpkg "github.com/wau/wau-cli/internal/stack"
)

var (
	downFile    string
	downProfile string
	downForce   bool
	downAll     bool
)

func newDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Bring the stack down (stop all services)",
		Long: `Stop the WAU stack gracefully.

Stops all running services in reverse topological order, sending SIGTERM
first (5s grace period) and escalating to SIGKILL if needed.

Flags:
  --file     Path to the wau-stack.yml used during 'up' (default: built-in)
  --profile  Profile used during 'up' (for filtering)
  --force    Force kill even services marked as failed
  --all      Also remove runtime state file (~/.wau/run/<stack>.json)

Examples:
  wau stack down
  wau stack down --all         # also purge runtime state
  wau stack down --force       # kill even if service already failed`,
		Aliases: []string{"stop"},
		RunE:    runDown,
	}

	cmd.Flags().StringVar(&downFile, "file", "", "path to wau-stack.yml (matches 'up')")
	cmd.Flags().StringVar(&downProfile, "profile", "", "profile (matches 'up')")
	cmd.Flags().BoolVar(&downForce, "force", false, "force kill even failed services")
	cmd.Flags().BoolVar(&downAll, "all", false, "remove runtime state file after stop")

	return cmd
}

func runDown(cmd *cobra.Command, args []string) error {
	s, err := loadStack(downFile, downProfile)
	if err != nil {
		return err
	}

	// 拓扑排序得到顺序,反向停止
	order, err := s.TopoOrder()
	if err != nil {
		return err
	}

	dataDir, _, err := s.ResolvedDirs()
	if err != nil {
		return err
	}

	rt, err := stackpkg.LoadRuntime(dataDir, s.Stack.Name)
	if err != nil {
		return err
	}

	if len(rt.Services) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(),
			"No services tracked. Use `wau stack up` first.")
		return nil
	}

	pm := stackpkg.NewProcessManager()
	stopped := 0
	failed := 0

	fmt.Fprintf(cmd.OutOrStdout(),
		"Bringing down stack %q (%d services)...\n\n", s.Stack.Name, len(order))
	startedAt := time.Now()

	// 反向:依赖方先停,基础设施后停
	for i := len(order) - 1; i >= 0; i-- {
		name := order[i]
		state, ok := rt.Services[name]
		if !ok {
			continue
		}
		// 跳过 external
		if state.Status == "external" {
			continue
		}
		// 检查 alive
		if !stackpkg.IsAlive(state.PID) {
			if !downForce {
				fmt.Fprintf(cmd.OutOrStdout(),
					"  ○ %-15s already stopped\n", name)
				continue
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "  ⠋ %-15s ... ", name)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := pm.Stop(ctx, state.PID)
		cancel()
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "✗ (%v)\n", err)
			rt.SetStatus(name, "failed", state.PID, nil)
			rt.Services[name].LastError = err.Error()
			failed++
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "✓\n")
			rt.SetStatus(name, "stopped", 0, nil)
			stopped++
		}
		// 杀进程组(防止 wau-agent 等 fork 出子进程)
		_ = stackpkg.KillProcessGroup(state.PID)
		// 清 PID 文件
		pidPath := stackpkg.PidFilePath(dataDir, name)
		_ = removeFile(pidPath)
	}

	total := time.Since(startedAt).Truncate(100 * time.Millisecond)
	fmt.Fprintf(cmd.OutOrStdout(),
		"\n✓ Stack %q down. %d stopped, %d failed, took %s.\n",
		s.Stack.Name, stopped, failed, total)

	if downAll {
		statePath := fmt.Sprintf("%s/%s.json", dataDir, s.Stack.Name)
		if err := removeFile(statePath); err == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Runtime state purged: %s\n", statePath)
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d service(s) failed to stop", failed)
	}
	return nil
}

// removeFile 删除文件,不存在不算错。
func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
