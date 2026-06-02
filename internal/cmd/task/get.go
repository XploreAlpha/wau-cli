package task

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
		Use:   "get <task-id>",
		Short: "Get task details",
		Long:  `Get detailed information about a specific task by ID.`,
		Args:  cobra.ExactArgs(1),
		Example: `  # Get task details
  wau task get task_1700000000

  # JSON output
  wau task get task_1700000000 -o json`,
		RunE: runGet,
	}

	return cmd
}

func runGet(cmd *cobra.Command, args []string) error {
	taskID := args[0]

	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t, err := c.GetTask(ctx, taskID)
	if err != nil {
		output.Error("Failed to get task: %v", err)
		return err
	}

	format, _ := output.ParseFormat(getOutputFmt())

	if format == output.FormatJSON {
		output.PrintJSON(t)
		return nil
	}
	if format == output.FormatYAML {
		output.PrintYAML(t)
		return nil
	}

	output.Success("Task: %s", t.TaskID)
	fmt.Printf("  Status:        %s\n", t.Status)
	fmt.Printf("  Message:       %s\n", t.Message)
	fmt.Printf("  Source:        %s\n", t.SourcePeer)
	if t.AssignedAgent != "" {
		fmt.Printf("  Assigned to:   %s\n", t.AssignedAgent)
	}
	if t.Result != "" {
		fmt.Printf("  Result:        %s\n", t.Result)
	}
	fmt.Printf("  Created At:    %s\n", time.Unix(t.CreatedAt, 0).Format(time.RFC3339))
	fmt.Printf("  Updated At:    %s\n", time.Unix(t.UpdatedAt, 0).Format(time.RFC3339))

	return nil
}
