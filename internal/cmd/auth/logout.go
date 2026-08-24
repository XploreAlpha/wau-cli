// Package auth - logout.go
//
// P4.3 — `wau auth logout` 实现(本地删 ~/.wau/credentials)。
package auth

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
)

// NewLogoutCmd creates `wau auth logout`.
func NewLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out (remove local credentials)",
		Long: `Log out by removing local credentials file (~/.wau/credentials).

Note: this only clears the local JWT tokens. The server side has no
explicit logout (JWTs are stateless). However, since credentials are
removed, subsequent requests will be anonymous (X-Agent-Role only).

If no credentials file exists, prints a friendly message and returns
success (not an error).`,
		Args: cobra.NoArgs,
		RunE: runAuthLogout,
	}
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	path := wauclient.DefaultCredentialsPath()

	// 检查文件是否存在
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(out, "Not logged in (no credentials file at %s)\n", path)
			return nil
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}

	// 删除
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	fmt.Fprintf(out, "✓ Credentials removed from %s\n", path)
	return nil
}