package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/wau/wau-cli/internal/client"
)

var (
	statusJSON     bool
	statusTimeout  time.Duration
)

// NewStatusCmd 构造 `wau cluster status` 子命令。
func NewStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show cluster status (kernel health + modules + agent count)",
		Long: `Show cluster-wide status — fetches /health, /kernel/info, and /registry/agents
concurrently and prints a unified overview.

By default uses the configured kernel address (--addr or wau config).
Override with --addr for remote cluster inspection (e.g. visa demo production server).

Flags:
  --json     Output as JSON (for piping to jq / dashboards)
  --timeout  Request timeout (default 10s, used per endpoint)

Exit codes:
  0   All 3 endpoints succeeded
  1   All 3 endpoints failed (kernel unreachable)
  2   Partial — at least 1 endpoint OK, others failed (marked ⚠)

Examples:
  wau cluster status
  wau cluster status --addr http://43.134.126.126:18400
  wau cluster status --json | jq '.kernel.version'`,
		RunE: runStatus,
	}
	cmd.Flags().BoolVar(&statusJSON, "json", false, "output as JSON")
	cmd.Flags().DurationVar(&statusTimeout, "timeout", 10*time.Second, "per-endpoint timeout")
	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()
	c := newClient()

	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()

	st, err := c.GetClusterStatus(ctx)
	if err != nil {
		// 全部 endpoint fail
		fmt.Fprintf(w, "✗ Kernel unreachable at %s\n", c.BaseURL())
		fmt.Fprintf(w, "  %v\n", err)
		return exitCodeError(1)
	}

	if statusJSON {
		return printStatusJSON(w, st)
	}
	return printStatusPretty(w, st, c.BaseURL())
}

func printStatusPretty(w io.Writer, st *client.ClusterStatus, endpoint string) error {
	// Partial check
	partial := 0
	if st.HealthErr != nil {
		partial++
	}
	if st.KernelErr != nil {
		partial++
	}
	if st.AgentsErr != nil {
		partial++
	}

	fmt.Fprintf(w, "Cluster: %s\n", endpoint)
	fmt.Fprintf(w, "Fetched: %s\n\n", st.FetchedAt.Format(time.RFC3339))

	// ──── Health ────
	if st.Health != nil {
		mark := "✓"
		if st.Health.Status != "ok" {
			mark = "✗"
		}
		fmt.Fprintf(w, "  %s Health:      %s\n", mark, st.Health.Status)
		fmt.Fprintf(w, "    Version:     %s\n", st.Health.Version)
		fmt.Fprintf(w, "    Uptime:      %s\n", formatUptime(st.Health.Uptime))
		fmt.Fprintf(w, "    Redis:       %s\n", st.Health.Redis)
	} else {
		fmt.Fprintf(w, "  ✗ Health:      <unreachable> %v\n", st.HealthErr)
	}
	fmt.Fprintln(w)

	// ──── Kernel ────
	if st.Kernel != nil {
		fmt.Fprintf(w, "  ✓ Kernel:      %s\n", st.Kernel.Version)
		fmt.Fprintf(w, "    Started:     %s\n", st.Kernel.StartTime)
		if len(st.Modules) > 0 {
			fmt.Fprintf(w, "    Modules:     %s\n", joinStrings(st.Modules))
		}
	} else {
		fmt.Fprintf(w, "  ✗ Kernel info: <unreachable> %v\n", st.KernelErr)
	}
	fmt.Fprintln(w)

	// ──── Agents ────
	if st.AgentsErr == nil {
		fmt.Fprintf(w, "  ✓ Agents:      %d registered\n", st.AgentsTotal)
	} else {
		fmt.Fprintf(w, "  ✗ Agents:      <unreachable> %v\n", st.AgentsErr)
	}

	if partial > 0 {
		fmt.Fprintf(w, "\n⚠ %d endpoint(s) unreachable — partial result\n", partial)
		return exitCodeError(2)
	}
	fmt.Fprintln(w, "\n✓ All endpoints healthy.")
	return nil
}

func printStatusJSON(w io.Writer, st *client.ClusterStatus) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w)
	return err
}

// formatUptime 把秒数转成 "1d 2h 3m 4s" 形式。
func formatUptime(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", seconds)
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
}

func joinStrings(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}