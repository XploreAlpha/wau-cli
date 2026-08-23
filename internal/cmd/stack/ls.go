// Package stack - ls.go
//
// 第一刀 1.1 — `wau stack ls / status / ps` 子命令。
//
// 行为:从 runtime state 文件读所有服务,检查每个 PID 是否还活着,
// 输出彩色表格(name / http port / grpc port / pid / status / uptime)。
package stack

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	stackpkg "github.com/wau/wau-cli/internal/stack"
)

var (
	lsFile    string
	lsProfile string
	lsFormat  string
)

func newLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List services with status",
		Long: `List all WAU stack services with their current status.

Reads the runtime state file (~/.wau/run/<stack>.json), verifies each PID
is still alive (kill -0), and prints a colored table.

Flags:
  --file     Path to the wau-stack.yml (matches 'up')
  --profile  Profile (matches 'up')
  -o         Output format: table | json | yaml

Examples:
  wau stack ls                    # default table
  wau stack ls -o json            # machine-readable
  wau stack status                # alias for ls`,
		Aliases: []string{"status", "ps"},
		RunE:    runLs,
	}

	cmd.Flags().StringVar(&lsFile, "file", "", "path to wau-stack.yml")
	cmd.Flags().StringVar(&lsProfile, "profile", "", "profile (matches 'up')")
	cmd.Flags().StringVarP(&lsFormat, "output", "o", "table", "output format: table|json|yaml")

	return cmd
}

func runLs(cmd *cobra.Command, args []string) error {
	s, err := loadStack(lsFile, lsProfile)
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

	// 校验每个 PID 是否还活着,更新 status
	now := time.Now()
	for _, state := range rt.Services {
		if state.PID > 0 && !stackpkg.IsAlive(state.PID) {
			if state.Status == "running" {
				state.Status = "stopped"
			}
		}
	}

	// 按拓扑顺序输出(确定性)
	order, _ := s.TopoOrder()
	orderedNames := make([]string, 0, len(rt.Services))
	seen := make(map[string]bool)
	for _, name := range order {
		if _, ok := rt.Services[name]; ok {
			orderedNames = append(orderedNames, name)
			seen[name] = true
		}
	}
	// 任何不在 order 里的(race condition)也加上
	var extras []string
	for name := range rt.Services {
		if !seen[name] {
			extras = append(extras, name)
		}
	}
	sort.Strings(extras)
	orderedNames = append(orderedNames, extras...)

	// 输出
	if lsFormat == "json" {
		return printJSON(cmd, rt, orderedNames)
	}
	if lsFormat == "yaml" {
		return printYAML(cmd, rt, orderedNames)
	}
	return printTable(cmd, rt, orderedNames, now)
}

func printTable(cmd *cobra.Command, rt *stackpkg.Runtime, names []string, now time.Time) error {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "NAME             HTTP    GRPC    PID     STATUS      UPTIME")
	fmt.Fprintln(out, "───────────────  ──────  ──────  ──────  ─────────  ────────")
	running := 0
	stopped := 0
	failed := 0
	for _, name := range names {
		state := rt.Services[name]
		uptime := "-"
		if !state.StartedAt.IsZero() && (state.Status == "running" || state.Status == "starting") {
			uptime = now.Sub(state.StartedAt).Truncate(time.Second).String()
		}
		icon := statusIcon(state.Status, state.PID)
		if state.Status == "running" {
			running++
		} else if state.Status == "stopped" {
			stopped++
		} else if state.Status == "failed" {
			failed++
		}
		fmt.Fprintf(out, "%s %-15s %-7d %-7d %-7d %-11s %s\n",
			icon,
			state.Name,
			state.HTTPPort,
			state.GRPCPort,
			state.PID,
			state.Status,
			uptime)
	}
	fmt.Fprintf(out,
		"\nSummary: %d running, %d stopped, %d failed (total %d)\n",
		running, stopped, failed, len(rt.Services))
	return nil
}

func statusIcon(status string, pid int) string {
	switch status {
	case "running":
		if !stackpkg.IsAlive(pid) {
			return "○" // registered as running but pid dead
		}
		return "✓"
	case "starting":
		return "⠋"
	case "stopped":
		return "○"
	case "failed":
		return "✗"
	case "external":
		return "→"
	default:
		return "?"
	}
}

func printJSON(cmd *cobra.Command, rt *stackpkg.Runtime, names []string) error {
	type row struct {
		Name      string     `json:"name"`
		HTTP      int        `json:"http_port"`
		GRPC      int        `json:"grpc_port"`
		PID       int        `json:"pid"`
		Status    string     `json:"status"`
		StartedAt *time.Time `json:"started_at,omitempty"`
		UptimeMS  int64      `json:"uptime_ms,omitempty"`
	}
	var rows []row
	for _, n := range names {
		state := rt.Services[n]
		var upMS int64
		var startedAt *time.Time
		if !state.StartedAt.IsZero() {
			t := state.StartedAt
			startedAt = &t
			upMS = time.Since(state.StartedAt).Milliseconds()
		}
		rows = append(rows, row{
			Name: state.Name, HTTP: state.HTTPPort, GRPC: state.GRPCPort,
			PID: state.PID, Status: state.Status,
			StartedAt: startedAt, UptimeMS: upMS,
		})
	}
	return writeJSON(cmd, rows)
}

func printYAML(cmd *cobra.Command, rt *stackpkg.Runtime, names []string) error {
	return writeYAML(cmd, map[string]interface{}{
		"stack":    rt.Name,
		"services": rt.Services,
	})
}

// JSON / YAML writers — 走 output 包复用
func writeJSON(cmd *cobra.Command, v interface{}) error {
	out := cmd.OutOrStdout()
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeYAML(cmd *cobra.Command, v interface{}) error {
	out := cmd.OutOrStdout()
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
