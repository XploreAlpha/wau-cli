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
	agentsJSON        bool
	agentsPage        int
	agentsPageSize    int
	agentsSkill       string
	agentsStatus      string
	agentsSearch      string
)

// NewAgentsCmd 构造 `wau cluster agents` 子命令。
//
// 跟 `wau agent list` 类似但走 cluster 上下文(可 --addr 远程 + 默认 list ALL skills)。
func NewAgentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "List agents registered in the cluster",
		Long: `List agents registered in the WAU cluster (wraps /registry/agents).

Similar to 'wau agent list' but in the cluster context — supports --addr
override for remote clusters and additional filtering.

Flags:
  --json        Output as JSON
  --page N      Page number (default 1)
  --page-size N Items per page (default 50)
  --skill NAME  Filter by skill (e.g. multi_agent)
  --status S    Filter by status (online / offline)
  --search Q    Search by name/description

Examples:
  wau cluster agents
  wau cluster agents --skill multi_agent
  wau cluster agents --status online --json`,
		Aliases: []string{"ls"},
		RunE:    runAgents,
	}
	cmd.Flags().BoolVar(&agentsJSON, "json", false, "output as JSON")
	cmd.Flags().IntVar(&agentsPage, "page", 1, "page number")
	cmd.Flags().IntVar(&agentsPageSize, "page-size", 50, "items per page")
	cmd.Flags().StringVar(&agentsSkill, "skill", "", "filter by skill")
	cmd.Flags().StringVar(&agentsStatus, "status", "", "filter by status")
	cmd.Flags().StringVar(&agentsSearch, "search", "", "search by name/description")
	return cmd
}

func runAgents(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()
	c := newClient()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	agents, total, err := c.ListAgentsRaw(ctx, agentsPage, agentsPageSize, agentsSkill, agentsStatus, agentsSearch)
	if err != nil {
		return fmt.Errorf("list agents: %w", err)
	}

	if agentsJSON {
		return printAgentsJSON(w, agents, total)
	}
	return printAgentsPretty(w, agents, total)
}

func printAgentsPretty(w io.Writer, agents []client.Agent, total int) error {
	if len(agents) == 0 {
		fmt.Fprintln(w, "(no agents registered)")
		return nil
	}
	fmt.Fprintf(w, "Total: %d agent(s)\n\n", total)
	fmt.Fprintf(w, "  %-20s  %-30s  %-15s  %s\n", "NAME", "URL", "STATUS", "SKILLS")
	fmt.Fprintf(w, "  %-20s  %-30s  %-15s  %s\n", "────────────────────", "──────────────────────────────", "───────────────", "────────────")
	for _, a := range agents {
		skills := joinStrings(a.Skills)
		fmt.Fprintf(w, "  %-20s  %-30s  %-15s  %s\n",
			truncate(a.Name, 20),
			truncate(a.URL, 30),
			a.Status,
			skills,
		)
	}
	return nil
}

func printAgentsJSON(w io.Writer, agents []client.Agent, total int) error {
	out := struct {
		Total  int            `json:"total"`
		Agents []client.Agent `json:"agents"`
	}{
		Total:  total,
		Agents: agents,
	}
	data, err := json.MarshalIndent(out, "", "  ")
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

// truncate 截断字符串到 n 字符,超过加 "..."。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}