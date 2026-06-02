package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/wau/wau-cli/internal/client"
	"github.com/wau/wau-cli/internal/output"
)

var (
	healthWait    bool
	healthTimeout time.Duration
)

// healthCmd represents the `wau health` command.
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check kernel health",
	Long: `Check the health status of the WAU-core-kernel.

This command pings the kernel and reports its current state,
including version, uptime, and Redis connectivity.`,
	Example: `  # Simple health check
  wau health

  # Wait for kernel to be healthy (useful in CI/CD)
  wau health --wait --timeout 30s`,
	RunE: runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)

	healthCmd.Flags().BoolVar(&healthWait, "wait", false, "wait for kernel to be healthy")
	healthCmd.Flags().DurationVar(&healthTimeout, "timeout", 30*time.Second, "timeout for wait mode")
}

func runHealth(cmd *cobra.Command, args []string) error {
	c := newClient()

	if healthWait {
		return waitForHealthy(c)
	}

	return checkHealth(c)
}

func checkHealth(c *client.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health, err := c.Health(ctx)
	if err != nil {
		output.Error("Health check failed: %v", err)
		return err
	}

	if health.Status == "unhealthy" {
		output.Warn("WAU-core is unhealthy")
		fmt.Printf("  Redis: %s\n", health.Redis)
		if health.Error != "" {
			fmt.Printf("  Error: %s\n", health.Error)
		}
		return fmt.Errorf("kernel is unhealthy")
	}

	output.Success("WAU-core is healthy")
	fmt.Printf("  Version: %s\n", health.Version)
	fmt.Printf("  Uptime:  %.1fs\n", health.Uptime)
	fmt.Printf("  Redis:   %s\n", health.Redis)

	return nil
}

func waitForHealthy(c *client.Client) error {
	deadline := time.Now().Add(healthTimeout)
	attempt := 0

	output.Info("Waiting for WAU-core to be healthy (timeout: %s)...", healthTimeout)

	for time.Now().Before(deadline) {
		attempt++
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		health, err := c.Health(ctx)
		cancel()

		if err == nil && health.Status == "ok" {
			output.Success("WAU-core is healthy (attempt #%d)", attempt)
			fmt.Printf("  Version: %s\n", health.Version)
			fmt.Printf("  Uptime:  %.1fs\n", health.Uptime)
			fmt.Printf("  Redis:   %s\n", health.Redis)
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout: kernel did not become healthy within %s", healthTimeout)
}

func newClient() *client.Client {
	return client.NewClient(client.Options{
		BaseURL: GetKernelAddr(),
		Role:    GetRole(),
	})
}
