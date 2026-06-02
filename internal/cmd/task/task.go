// Package task implements the `wau task` command.
package task

import (
	"github.com/spf13/cobra"
)

// NewTaskCmd creates the `wau task` command and its subcommands.
func NewTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
		Long: `Manage WAU tasks - submit, query, and monitor tasks.

Subcommands:
  submit     Submit a new task
  get        Get task details
  list       List recent tasks`,
	}

	cmd.AddCommand(newSubmitCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newListCmd())

	return cmd
}
