// Package cmd implements all wau-cli commands using Cobra.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wau/wau-cli/internal/cmd/agent"
	"github.com/wau/wau-cli/internal/cmd/auth"
	"github.com/wau/wau-cli/internal/cmd/cluster"
	"github.com/wau/wau-cli/internal/cmd/stack"
	"github.com/wau/wau-cli/internal/cmd/task"
	wauconfig "github.com/wau/wau-cli/internal/cmd/config"
	"github.com/wau/wau-cli/internal/version"
)

var (
	// Version of wau-cli (sourced from internal/version package — per D92
	// v1.1.0 子项 4.2 version alignment, all 14 server 仓 + wau-cli share
	// the same const value).
	Version = version.Version

	// ReleaseName of the current version (e.g. "Phoenix", "Iris", "Jade").
	ReleaseName = version.ReleaseName

	// Global flags
	cfgFile     string
	kernelAddr  string
	outputFmt   string
	role        string
	showVersion bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "wau",
	Short: "WAU command-line tool",
	Long: `wau-cli is the official command-line client for WAU-core-kernel.

It provides a kubectl/docker-like experience for managing WAU services,
including agents, tasks, kernel info, and configuration.

Examples:
  # Check kernel health
  wau health

  # List all online agents
  wau agent list

  # Submit a task
  wau task submit "帮我查一下天气"

  # Validate configuration
  wau config validate`,
	SilenceUsage:          true,
	SilenceErrors:         false,
	DisableAutoGenTag:     true,
	CompletionOptions:     cobra.CompletionOptions{DisableDefaultCmd: true}, // 我们的 completion.go 自定义
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Printf("wau-cli %s \"%s\"\n", Version, ReleaseName)
			fmt.Println("Official CLI for WAU-core-kernel")
			return nil
		}
		return cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main().
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: $HOME/.wau/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&kernelAddr, "addr", "", "kernel address (default: http://localhost:18400 or from config)")
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "table", "output format: table|json|yaml|csv")
	rootCmd.PersistentFlags().StringVar(&role, "role", "external_agent", "agent role: kernel_core|trusted_agent|external_agent")

	// Local flags
	rootCmd.Flags().BoolVarP(&showVersion, "version", "V", false, "show wau-cli version")

	// Register subcommands
	rootCmd.AddCommand(agent.NewAgentCmd())
	rootCmd.AddCommand(task.NewTaskCmd())
	rootCmd.AddCommand(wauconfig.NewConfigCmd())
	rootCmd.AddCommand(stack.NewStackCmd())
	rootCmd.AddCommand(auth.NewAuthCmd())    // P4.3 wau auth login/logout/whoami
	rootCmd.AddCommand(cluster.NewClusterCmd()) // P4.6 wau cluster status/agents
	rootCmd.AddCommand(NewCompletionCmd())
	rootCmd.AddCommand(stack.NewLogCmd()) // 顶层 `wau log <svc>` alias

	// 4.3 MVP — 顶层 super-binary aliases(`wau up/down/status/doctor`)
	rootCmd.AddCommand(stack.UpCmd())    // wau up       ≡ wau stack up
	rootCmd.AddCommand(stack.DownCmd())  // wau down     ≡ wau stack down
	rootCmd.AddCommand(stack.LsCmd())    // wau status   ≡ wau stack ls (Use=ls, Aliases=[status,ps])
	rootCmd.AddCommand(newDoctorCmd())   // wau doctor   离线诊断

	// Wire up accessors for sub-packages
	agent.SetAccessors(GetKernelAddr, GetRole, GetOutputFmt)
	task.SetAccessors(GetKernelAddr, GetRole, GetOutputFmt)
	wauconfig.SetAccessors(GetKernelAddr, GetOutputFmt)
	auth.SetAccessors(GetKernelAddr, GetRole) // P4.3
	cluster.SetAccessors(GetKernelAddr, GetRole) // P4.6
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".wau" (without extension).
		viper.AddConfigPath(filepath.Join(home, ".wau"))
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		// Set kernel address from config if not specified via flag
		if kernelAddr == "" {
			if addr := viper.GetString("kernel.addr"); addr != "" {
				kernelAddr = addr
			}
		}
		// Set role from config if not specified via flag
		if role == "external_agent" {
			if r := viper.GetString("kernel.role"); r != "" {
				role = r
			}
		}
	}

	// Default kernel address
	if kernelAddr == "" {
		kernelAddr = "http://localhost:18400"
	}
}

// GetKernelAddr returns the kernel address (used by subcommands)
func GetKernelAddr() string {
	return kernelAddr
}

// GetRole returns the agent role (used by subcommands)
func GetRole() string {
	return role
}

// GetOutputFmt returns the output format
func GetOutputFmt() string {
	return outputFmt
}
