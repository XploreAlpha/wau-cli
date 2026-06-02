// Package config implements the `wau config` command.
package config

import (
	"github.com/spf13/cobra"
)

// NewConfigCmd creates the `wau config` command and its subcommands.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage wau-cli configuration",
		Long: `Manage wau-cli configuration files.

Subcommands:
  init      Generate a default config file
  validate  Validate the current configuration
  show      Show the current configuration`,
	}

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newValidateCmd())
	cmd.AddCommand(newShowCmd())

	return cmd
}
