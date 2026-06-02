package agent

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
	"github.com/wau/wau-cli/internal/output"
)

func newDeregisterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deregister <name>",
		Short: "Deregister an agent",
		Long:  `Remove an agent from the WAU network.`,
		Args:  cobra.ExactArgs(1),
		Example: `  # Deregister an agent
  wau agent deregister fox-medical`,
		RunE: runDeregister,
	}

	return cmd
}

func runDeregister(cmd *cobra.Command, args []string) error {
	name := args[0]

	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.DeregisterAgent(ctx, name); err != nil {
		output.Error("Failed to deregister agent: %v", err)
		return err
	}

	output.Success("Agent '%s' deregistered successfully", name)
	return nil
}
