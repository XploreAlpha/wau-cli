// Package agent - install.go
//
// v1.0.0 M11 P4.5 ⭐L5 包管理器 install 子命令(per D72/D73/D74,2026-07-10)。
//
// wau agent install <name> — 装 agent(类比 apt install / npm install)
//
// 走 WAU-core-kernel POST /v1/l5/install + L5Service HTTP API。
package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
)

var (
	installVersion string
	installPurge   bool
	installConfig  []string
	installUser    string
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <name>",
		Short: "Install an agent from wau-registry",
		Long: `Install a WAU agent from wau-registry (per D72 ⭐L5 package manager).

Equivalent to:
  apt install <name>     (Debian/Ubuntu)
  brew install <name>    (macOS)
  npm install <name>     (Node.js)

This walks:
  1. Pulls manifest.yaml from wau-registry
  2. Downloads agent tarball + verifies SHA256
  3. Spawns wau-agent sandbox (Docker + seccomp + RO fs + net ns, per D68)
  4. Writes wau-profile row (installed_agents, user_skill_pool, per D73)
  5. Returns agent_id + version + sandbox_docker_id

Examples:
  # Install latest weather-agent
  wau agent install weather-agent

  # Install specific version with config
  wau agent install weather-agent --version=1.2.3 --config=city=北京

  # Force reinstall (purge old data)
  wau agent install weather-agent --purge

  # Specify user (defaults to $WAU_USER or "default")
  wau agent install weather-agent --user=alice`,
		Args: cobra.ExactArgs(1),
		RunE: runInstall,
	}

	cmd.Flags().StringVar(&installVersion, "version", "", "specific version (default: latest)")
	cmd.Flags().BoolVar(&installPurge, "purge", false, "purge old data before install")
	cmd.Flags().StringSliceVar(&installConfig, "config", nil, "agent config k=v (repeatable)")
	cmd.Flags().StringVar(&installUser, "user", "", "owner user_id (default: $WAU_USER or \"default\")")

	return cmd
}

func runInstall(cmd *cobra.Command, args []string) error {
	agentName := args[0]
	userID := installUser
	if userID == "" {
		userID = getDefaultUser()
	}
	config, err := parseConfig(installConfig)
	if err != nil {
		return err
	}

	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})
	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	resp, err := c.L5Install(ctx, &wauclient.L5InstallRequest{
		UserID:    userID,
		AgentName: agentName,
		Version:   installVersion,
		Purge:     installPurge,
		Config:    config,
	})
	if err != nil {
		return fmt.Errorf("install failed: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("install failed: %s", resp.Error)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ %s %s installed (agent_id=%s, sandbox=%s, took %.1fms)\n",
		agentName, resp.Version, resp.AgentID, resp.SandboxDockerID, resp.DurationMS)
	return nil
}

// parseConfig --config k=v --config k2=v2 → map
func parseConfig(items []string) (map[string]string, error) {
	if len(items) == 0 {
		return nil, nil
	}
	m := make(map[string]string, len(items))
	for _, item := range items {
		for i := 0; i < len(item); i++ {
			if item[i] == '=' {
				m[item[:i]] = item[i+1:]
				goto next
			}
		}
		return nil, fmt.Errorf("invalid --config %q (want k=v)", item)
	next:
	}
	return m, nil
}

func getDefaultUser() string {
	// 后续接 $WAU_USER env,本期 default
	return "default"
}