package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the `wau version` command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show wau-cli version",
	Long:  `Show the wau-cli version information.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("wau-cli %s \"%s\"\n", Version, ReleaseName)
		fmt.Println("Official CLI for WAU-core-kernel")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
