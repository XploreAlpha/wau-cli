package task

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
	"github.com/wau/wau-cli/internal/output"
)

var (
	submitMessage    string
	submitSource     string
	submitSourceID   string
	submitIntentType string
	submitUrgency    string
)

func newSubmitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit <message>",
		Short: "Submit a new task",
		Long: `Submit a new task to the WAU kernel for processing.

The message will be parsed by the intent engine, and the task will be
scheduled to the best matching agent.`,
		Args:  cobra.ExactArgs(1),
		Example: `  # Submit a simple task
  wau task submit "帮我查一下天气"

  # Submit with source info
  wau task submit "..." --source my-agent --source-id agent_001

  # Submit with explicit intent
  wau task submit "..." --intent-type research --urgency high`,
		RunE: runSubmit,
	}

	cmd.Flags().StringVar(&submitMessage, "msg", "", "task message (alternative to positional arg)")
	cmd.Flags().StringVar(&submitSource, "source", "wau-cli", "source peer name")
	cmd.Flags().StringVar(&submitSourceID, "source-id", "", "source agent ID")
	cmd.Flags().StringVar(&submitIntentType, "intent-type", "", "explicit intent type")
	cmd.Flags().StringVar(&submitUrgency, "urgency", "normal", "task urgency: low|normal|high|critical")

	return cmd
}

func runSubmit(cmd *cobra.Command, args []string) error {
	message := submitMessage
	if message == "" && len(args) > 0 {
		message = args[0]
	}

	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &wauclient.TaskSubmitRequest{
		Message:       message,
		SourcePeer:    submitSource,
		SourceAgentID: submitSourceID,
	}

	if submitIntentType != "" || submitUrgency != "normal" {
		req.Intent = &wauclient.IntentDTO{
			Type:    submitIntentType,
			Urgency: submitUrgency,
		}
	}

	result, err := c.SubmitTask(ctx, req)
	if err != nil {
		output.Error("Failed to submit task: %v", err)
		return err
	}

	format, _ := output.ParseFormat(getOutputFmt())

	if format == output.FormatJSON {
		output.PrintJSON(result)
		return nil
	}
	if format == output.FormatYAML {
		output.PrintYAML(result)
		return nil
	}

	output.Success("Task submitted: %s", result.TaskID)
	fmt.Printf("  Status:        %s\n", result.Status)
	if result.AssignedAgent != "" {
		fmt.Printf("  Assigned to:   %s\n", result.AssignedAgent)
	}
	if result.Result != "" {
		fmt.Printf("  Result:        %s\n", result.Result)
	}
	if result.Error != "" {
		fmt.Printf("  Error:         %s\n", result.Error)
	}

	return nil
}
