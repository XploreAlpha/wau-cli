package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/wau/wau-cli/internal/output"
)

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the current configuration",
		Long: `Display the current wau-cli configuration.

Reads from:
1. --config flag
2. ./config.yaml
3. ~/.wau/config.yaml`,
		Example: `  # Show config
  wau config show

  # Show in JSON format
  wau config show -o json`,
		RunE: runShow,
	}

	return cmd
}

func runShow(cmd *cobra.Command, args []string) error {
	configPath, err := findConfigFile()
	if err != nil {
		output.Error("%v", err)
		// Show effective config from CLI flags/env
		showEffectiveConfig()
		return err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		output.Error("Failed to read config: %v", err)
		return err
	}

	format, _ := output.ParseFormat(getOutputFmt())

	// Parse YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		output.Error("YAML parse error: %v", err)
		return err
	}

	// Add file source
	config["_source"] = configPath
	config["_effective"] = map[string]interface{}{
		"kernel_addr": getKernelAddr(),
	}

	switch format {
	case output.FormatJSON:
		output.PrintJSON(config)
	case output.FormatYAML:
		// Re-marshal for clean YAML output
		cleanData, _ := yaml.Marshal(config)
		fmt.Print(string(cleanData))
	default:
		output.Success("Config loaded from: %s", configPath)
		prettyPrintYAML(config)
	}

	return nil
}

func showEffectiveConfig() {
	output.Info("No config file found. Effective configuration from flags/env:")
	fmt.Printf("  Kernel address: %s\n", getKernelAddr())
	fmt.Println()
	output.Info("Run 'wau config init' to create a config file")
}

func prettyPrintYAML(data map[string]interface{}) {
	for k, v := range data {
		fmt.Printf("  %s: %v\n", k, v)
	}
}

// Avoid unused import warnings
var _ = filepath.Join
