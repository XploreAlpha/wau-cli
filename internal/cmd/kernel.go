package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/wau/wau-cli/internal/output"
)

// kernelCmd represents the `wau kernel` command.
var kernelCmd = &cobra.Command{
	Use:   "kernel",
	Short: "Show kernel information",
	Long: `Display information about the WAU-core-kernel service.

Subcommands:
  info    Show detailed kernel information
  version Show kernel version`,
}

// kernelInfoCmd represents the `wau kernel info` command.
var kernelInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show kernel information",
	Long:  `Show detailed information about the running kernel.`,
	RunE:  runKernelInfo,
}

// kernelVersionCmd represents the `wau kernel version` command.
var kernelVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show kernel version",
	Long:  `Show the version of the running kernel.`,
	RunE:  runKernelVersion,
}

func init() {
	rootCmd.AddCommand(kernelCmd)
	kernelCmd.AddCommand(kernelInfoCmd)
	kernelCmd.AddCommand(kernelVersionCmd)
}

func runKernelInfo(cmd *cobra.Command, args []string) error {
	c := newClient()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := c.GetKernelInfo(ctx)
	if err != nil {
		output.Error("Failed to get kernel info: %v", err)
		return err
	}

	format, _ := output.ParseFormat(GetOutputFmt())

	switch format {
	case output.FormatJSON:
		output.PrintJSON(info)
	case output.FormatYAML:
		output.PrintYAML(info)
	default:
		output.Success("WAU-core-kernel Information")
		fmt.Printf("  Version:     %s\n", info.Version)
		fmt.Printf("  Start Time:  %s\n", info.StartTime)
		fmt.Printf("  Uptime:      %s\n", formatUptimeSimple(time.Duration(info.Uptime)))
		fmt.Printf("  Agents:      %d\n", info.AgentsCount)
		fmt.Printf("  Tasks:       %d\n", info.TasksCount)
	}

	return nil
}

func runKernelVersion(cmd *cobra.Command, args []string) error {
	c := newClient()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := c.GetKernelInfo(ctx)
	if err != nil {
		output.Error("Failed to get kernel version: %v", err)
		return err
	}

	fmt.Println(info.Version)
	return nil
}

// formatUptimeSimple 把 time.Duration 格式化为 "1h 2m 3s" / "45s" / "300ms"。
func formatUptimeSimple(d time.Duration) string {
	if d < time.Second {
		return d.String()
	}
	d = d.Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
