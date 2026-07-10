// Package agent implements the `wau agent` command.
package agent

import (
	"github.com/spf13/cobra"
)

// NewAgentCmd creates the `wau agent` command and its subcommands.
func NewAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agents",
		Long: `Manage WAU agents - register, list, view details, and deregister.

Subcommands:
  list         List online agents
  get          Get agent details
  register     Register a new agent
  deregister   Deregister an agent
  score        Get agent score
  publish      Publish a skill bundle (manifest + tarball) to wau-registry
  install      Install an agent from wau-registry  (L5 ⭐, per D72/D73/D74)
  uninstall    Uninstall an installed agent        (L5 ⭐)
  update       Update installed agent(s)            (L5 ⭐)
  search       Search wau-registry for agents       (L5 ⭐)
  login        Log in to a WAU registry             (L5 ⭐)`,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newRegisterCmd())
	cmd.AddCommand(newDeregisterCmd())
	cmd.AddCommand(newScoreCmd())
	cmd.AddCommand(newPublishCmd())
	// v1.0.0 M11 P4.5 ⭐L5 包管理器 5 subcommand (per D72/D73/D74,2026-07-10)
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newUninstallCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newSearchCmd())
	cmd.AddCommand(newLoginCmd())

	return cmd
}
