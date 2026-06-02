package task

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
	"github.com/wau/wau-cli/internal/output"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent tasks",
		Long: `List recent tasks in the WAU system.

Note: This is a placeholder. The kernel may not have a list endpoint yet
in v0.2.0; use 'wau task get' for specific tasks.`,
		RunE: runList,
	}

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to get kernel info to check if list endpoint exists
	_, err := c.GetKernelInfo(ctx)
	if err != nil {
		output.Error("Failed to connect to kernel: %v", err)
		return err
	}

	output.Info("Task list endpoint is not yet implemented in kernel v0.2.0")
	output.Info("Use 'wau task get <task-id>' to query specific tasks")

	format, _ := output.ParseFormat(getOutputFmt())
	if format == output.FormatJSON {
		fmt.Println("[]")
	}
	if format == output.FormatYAML {
		fmt.Println("[]")
	}

	return nil
}
