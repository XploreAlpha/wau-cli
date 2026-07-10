// Package agent - search.go
//
// v1.0.0 M11 P4.5 L5 包管理器 search 子命令(per D72/D73/D74,2026-07-10)。
//
// wau agent search <query> — 搜 wau-registry(类比 apt search / npm search)
package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
	"github.com/wau/wau-cli/internal/output"
)

var (
	searchUniverse string
	searchLimit    int
	searchUser     string
)

func newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search wau-registry for agents",
		Long: `Search wau-registry for agents matching <query> (per D72 ⭐L5).

Equivalent to:
  apt search <query>
  npm search <query>
  brew search <query>

Searches name + description fields, returns up to --limit hits (default 20).

Examples:
  # Search for weather agents
  wau agent search weather

  # Filter by universe
  wau agent search --universe=productivity

  # Limit results
  wau agent search weather --limit=5

  # Combine
  wau agent search weather --universe=general --limit=10`,
		Args: cobra.MaximumNArgs(1),
		RunE: runSearch,
	}

	cmd.Flags().StringVar(&searchUniverse, "universe", "", "filter by universe (e.g. general, productivity)")
	cmd.Flags().IntVar(&searchLimit, "limit", 20, "max results (default 20)")
	cmd.Flags().StringVar(&searchUser, "user", "", "owner user_id (default: $WAU_USER or \"default\")")

	return cmd
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}
	userID := searchUser
	if userID == "" {
		userID = getDefaultUser()
	}

	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	resp, err := c.L5Search(ctx, &wauclient.L5SearchRequest{
		UserID:   userID,
		Query:    query,
		Universe: searchUniverse,
		Limit:    searchLimit,
	})
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("search failed: %s", resp.Error)
	}

	if len(resp.Results) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "no agents found for query=%q universe=%q\n", query, searchUniverse)
		return nil
	}

	headers := []string{"NAME", "VERSION", "UNIVERSE", "TRUST", "DESCRIPTION"}
	rows := make([][]string, 0, len(resp.Results))
	for _, h := range resp.Results {
		rows = append(rows, []string{
			h.Name,
			h.Version,
			h.Universe,
			fmt.Sprintf("%.2f", h.TrustScore),
			h.Description,
		})
	}
	output.PrintTable(headers, rows)
	fmt.Fprintf(cmd.OutOrStdout(), "\n%d result(s)\n", resp.Total)
	return nil
}