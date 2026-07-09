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
  publish      Publish a skill bundle (manifest + tarball) to wau-registry`,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newRegisterCmd())
	cmd.AddCommand(newDeregisterCmd())
	cmd.AddCommand(newScoreCmd())
	cmd.AddCommand(newPublishCmd())

	return cmd
}
