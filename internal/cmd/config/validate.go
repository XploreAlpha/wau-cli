package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/wau/wau-cli/internal/output"
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the current configuration",
		Long: `Validate the current wau-cli configuration file.

Checks:
- File exists and is readable
- YAML syntax is valid
- Required fields are present`,
		Example: `  # Validate config
  wau config validate`,
		RunE: runValidate,
	}

	return cmd
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Try to find config file
	configPath, err := findConfigFile()
	if err != nil {
		output.Error("%v", err)
		return err
	}

	output.Info("Validating config: %s", configPath)

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		output.Error("Failed to read config: %v", err)
		return err
	}

	// Parse YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		output.Error("YAML syntax error: %v", err)
		return err
	}

	// Validate kernel section
	kernel, ok := config["kernel"].(map[string]interface{})
	if !ok {
		output.Error("Missing 'kernel' section")
		return fmt.Errorf("invalid config: missing kernel section")
	}

	// Check kernel.addr
	addr, ok := kernel["addr"].(string)
	if !ok || addr == "" {
		output.Error("Missing or empty 'kernel.addr'")
		return fmt.Errorf("invalid config: missing kernel.addr")
	}

	// Check kernel.role
	role, _ := kernel["role"].(string)
	if role == "" {
		role = "external_agent"
	}
	if !isValidRole(role) {
		output.Error("Invalid role '%s' (must be: kernel_core, trusted_agent, external_agent)", role)
		return fmt.Errorf("invalid role")
	}

	// All good
	output.Success("Config is valid")
	output.Info("  Kernel address: %s", addr)
	output.Info("  Role:           %s", role)

	return nil
}

func findConfigFile() (string, error) {
	// Check current directory
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml", nil
	}

	// Check ~/.wau/config.yaml
	home, err := os.UserHomeDir()
	if err == nil {
		homePath := filepath.Join(home, ".wau", "config.yaml")
		if _, err := os.Stat(homePath); err == nil {
			return homePath, nil
		}
	}

	return "", fmt.Errorf("no config file found (run 'wau config init' to create one)")
}

func isValidRole(role string) bool {
	switch role {
	case "kernel_core", "trusted_agent", "external_agent":
		return true
	}
	return false
}
