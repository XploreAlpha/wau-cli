package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
	"github.com/wau/wau-cli/internal/output"
)

func newScoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "score <name>",
		Short: "Get agent score",
		Long:  `Get the 15-dimension score of a specific agent.`,
		Args:  cobra.ExactArgs(1),
		Example: `  # Get agent score
  wau agent score fox

  # JSON output
  wau agent score fox -o json`,
		RunE: runScore,
	}

	return cmd
}

func runScore(cmd *cobra.Command, args []string) error {
	name := args[0]

	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	score, err := c.GetAgentScore(ctx, name)
	if err != nil {
		output.Error("Failed to get agent score: %v", err)
		return err
	}

	format, _ := output.ParseFormat(getOutputFmt())

	if format == output.FormatJSON {
		output.PrintJSON(score)
		return nil
	}
	if format == output.FormatYAML {
		output.PrintYAML(score)
		return nil
	}

	output.Success("Agent Score: %s", score.Name)
	fmt.Printf("  Total Score:  %.3f\n", score.TotalScore)
	fmt.Printf("  Trust Score:  %.3f\n", score.TrustScore)
	fmt.Printf("  Skill Match:  %.3f\n", score.SkillMatch)
	fmt.Printf("  Health Score: %.3f\n", score.HealthScore)
	fmt.Printf("  Load Score:   %.3f\n", score.LoadScore)

	return nil
}
