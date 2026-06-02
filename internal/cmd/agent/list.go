package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
	"github.com/wau/wau-cli/internal/output"
)

var (
	listPage     int
	listPageSize int
	listSkill    string
	listStatus   string
	listSearch   string
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List online agents",
		Long: `List all registered and online agents in the WAU network.

Supports pagination, filtering by skill/status, and search by name.`,
		Example: `  # List all agents
  wau agent list

  # Paginate
  wau agent list --page 2 --page-size 50

  # Filter by skill
  wau agent list --skill medical

  # JSON output
  wau agent list -o json`,
		RunE: runList,
	}

	cmd.Flags().IntVarP(&listPage, "page", "p", 1, "page number")
	cmd.Flags().IntVarP(&listPageSize, "page-size", "n", 20, "items per page")
	cmd.Flags().StringVarP(&listSkill, "skill", "s", "", "filter by skill")
	cmd.Flags().StringVar(&listStatus, "status", "", "filter by status (online/offline)")
	cmd.Flags().StringVar(&listSearch, "search", "", "search by name")

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := c.ListAgents(ctx, listPage, listPageSize, listSkill, listStatus, listSearch)
	if err != nil {
		output.Error("Failed to list agents: %v", err)
		return err
	}

	format, _ := output.ParseFormat(getOutputFmt())

	if format == output.FormatJSON {
		output.PrintJSON(resp)
		return nil
	}
	if format == output.FormatYAML {
		output.PrintYAML(resp)
		return nil
	}

	if len(resp.Agents) == 0 {
		output.Info("No agents found")
		return nil
	}

	headers := []string{"NAME", "SKILLS", "TRUST", "STATUS", "URL"}
	rows := make([][]string, 0, len(resp.Agents))
	for _, a := range resp.Agents {
		skills := strings.Join(a.Skills, ", ")
		if skills == "" {
			skills = "-"
		}
		rows = append(rows, []string{
			a.Name,
			skills,
			fmt.Sprintf("%.2f", a.Trust),
			a.Status,
			a.URL,
		})
	}

	output.PrintTable(headers, rows)
	fmt.Printf("\nTotal: %d agents (showing %d-%d, page %d/%d)\n",
		resp.Total,
		(resp.Page-1)*resp.PageSize+1,
		(resp.Page-1)*resp.PageSize+len(resp.Agents),
		resp.Page,
		resp.TotalPages,
	)

	return nil
}
