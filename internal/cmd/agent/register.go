package agent

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
	"github.com/wau/wau-cli/internal/output"
)

var (
	regName        string
	regURL         string
	regDescription string
	regSkills      []string
	regUniverses   []string
)

func newRegisterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new agent",
		Long: `Register a new agent in the WAU network.

Required flags: --name, --url`,
		Example: `  # Register a new medical agent
  wau agent register \
    --name fox-medical \
    --url http://100.125.99.209:18800 \
    --skills medical,clinical \
    --universes medical`,
		RunE: runRegister,
	}

	cmd.Flags().StringVar(&regName, "name", "", "agent name (required)")
	cmd.Flags().StringVar(&regURL, "url", "", "agent URL (required)")
	cmd.Flags().StringVar(&regDescription, "description", "", "agent description")
	cmd.Flags().StringSliceVar(&regSkills, "skills", []string{}, "comma-separated skills")
	cmd.Flags().StringSliceVar(&regUniverses, "universes", []string{}, "comma-separated universes")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("url")

	return cmd
}

func runRegister(cmd *cobra.Command, args []string) error {
	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &wauclient.AgentRegisterRequest{
		Name:        regName,
		URL:         regURL,
		Description: regDescription,
		Skills:      regSkills,
		Universes:   regUniverses,
	}

	if err := c.RegisterAgent(ctx, req); err != nil {
		output.Error("Failed to register agent: %v", err)
		return err
	}

	output.Success("Agent '%s' registered successfully", regName)
	if len(req.Skills) > 0 {
		output.Info("  Skills: %s", strings.Join(req.Skills, ", "))
	}
	if len(req.Universes) > 0 {
		output.Info("  Universes: %s", strings.Join(req.Universes, ", "))
	}

	return nil
}
