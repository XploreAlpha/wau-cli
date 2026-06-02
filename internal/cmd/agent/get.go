package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
	"github.com/wau/wau-cli/internal/output"
)

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get agent details",
		Long:  `Get detailed information about a specific agent by name.`,
		Args:  cobra.ExactArgs(1),
		Example: `  # Get agent details
  wau agent get fox

  # JSON output
  wau agent get fox -o json`,
		RunE: runGet,
	}

	return cmd
}

func runGet(cmd *cobra.Command, args []string) error {
	name := args[0]

	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	agent, err := c.GetAgent(ctx, name)
	if err != nil {
		output.Error("Failed to get agent: %v", err)
		return err
	}

	format, _ := output.ParseFormat(getOutputFmt())

	if format == output.FormatJSON {
		output.PrintJSON(agent)
		return nil
	}
	if format == output.FormatYAML {
		output.PrintYAML(agent)
		return nil
	}

	output.Success("Agent: %s", agent.Name)
	fmt.Printf("  Status:      %s\n", agent.Status)
	fmt.Printf("  Trust:       %.2f\n", agent.Trust)
	fmt.Printf("  Circuit:     %s\n", agent.Circuit)
	fmt.Printf("  Active Tasks:%d\n", agent.Load.ActiveTasks)
	fmt.Printf("  Max Capacity:%d\n", agent.Load.MaxCapacity)
	fmt.Printf("  CPU Usage:   %.1f%%\n", agent.Load.CPUUsage)
	fmt.Printf("  Memory Usage:%.1f%%\n", agent.Load.MemoryUsage)

	return nil
}
