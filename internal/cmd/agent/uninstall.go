// Package agent - uninstall.go
//
// v1.0.0 M11 P4.5 L5 包管理器 uninstall 子命令(per D72/D73/D74,2026-07-10)。
//
// wau agent uninstall <name> — 卸 agent(类比 apt remove / npm uninstall)
package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
)

var (
	uninstallPurge bool
	uninstallUser  string
)

func newUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <name>",
		Short: "Uninstall an installed agent",
		Long: `Uninstall a WAU agent (per D72 ⭐L5 package manager).

Equivalent to:
  apt remove <name>   (default: keeps user data)
  apt purge <name>    (with --purge: removes all data)

By default, wau-profile data (user_config + agent_memory + skill_pool)
is preserved as a snapshot for re-install. Use --purge to remove everything.

Examples:
  # Uninstall weather-agent, keep data
  wau agent uninstall weather-agent

  # Uninstall and remove all data
  wau agent uninstall weather-agent --purge

  # Specify user
  wau agent uninstall weather-agent --user=alice`,
		Args: cobra.ExactArgs(1),
		RunE: runUninstall,
	}

	cmd.Flags().BoolVar(&uninstallPurge, "purge", false, "remove all data (no snapshot)")
	cmd.Flags().StringVar(&uninstallUser, "user", "", "owner user_id (default: $WAU_USER or \"default\")")

	return cmd
}

func runUninstall(cmd *cobra.Command, args []string) error {
	agentName := args[0]
	userID := uninstallUser
	if userID == "" {
		userID = getDefaultUser()
	}

	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	resp, err := c.L5Uninstall(ctx, &wauclient.L5UninstallRequest{
		UserID:    userID,
		AgentName: agentName,
		Purge:     uninstallPurge,
	})
	if err != nil {
		return fmt.Errorf("uninstall failed: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("uninstall failed: %s", resp.Error)
	}

	if uninstallPurge {
		fmt.Fprintf(cmd.OutOrStdout(), "✅ %s fully removed (data + memory + config)\n", agentName)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "✅ %s uninstalled (data preserved at %s)\n", agentName, resp.SnapshotPath)
	}
	return nil
}