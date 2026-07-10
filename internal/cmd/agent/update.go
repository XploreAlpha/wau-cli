// Package agent - update.go
//
// v1.0.0 M11 P4.5 L5 包管理器 update 子命令(per D72/D73/D74,2026-07-10)。
//
// wau agent update [<name>] — 更新 agent(类比 apt upgrade / npm update)
//
// 不传 name = 全更新。
package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
)

var (
	updateTargetVersion string
	updateUser          string
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [<name>]",
		Short: "Update installed agent(s)",
		Long: `Update installed agent(s) to latest or specified version (per D72 ⭐L5).

Equivalent to:
  apt upgrade           (no <name>: update all)
  apt upgrade <name>    (with <name>: update one)

Examples:
  # Update all installed agents
  wau agent update

  # Update specific agent
  wau agent update weather-agent

  # Pin to specific version
  wau agent update weather-agent --version=1.2.3

  # Specify user
  wau agent update --user=alice`,
		Args: cobra.MaximumNArgs(1),
		RunE: runUpdate,
	}

	cmd.Flags().StringVar(&updateTargetVersion, "version", "", "target version (default: latest)")
	cmd.Flags().StringVar(&updateUser, "user", "", "owner user_id (default: $WAU_USER or \"default\")")

	return cmd
}

func runUpdate(cmd *cobra.Command, args []string) error {
	agentName := ""
	if len(args) > 0 {
		agentName = args[0]
	}
	userID := updateUser
	if userID == "" {
		userID = getDefaultUser()
	}

	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})
	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	resp, err := c.L5Update(ctx, &wauclient.L5UpdateRequest{
		UserID:        userID,
		AgentName:     agentName,
		TargetVersion: updateTargetVersion,
	})
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("update failed: %s", resp.Error)
	}

	if resp.UpdatedCount == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no agents to update")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✅ updated %d agent(s): %v\n", resp.UpdatedCount, resp.UpdatedAgents)
	return nil
}