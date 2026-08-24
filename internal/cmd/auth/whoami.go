// Package auth - whoami.go
//
// P4.3 — `wau auth whoami` 实现(读本地 credentials,显示当前 user)。
//
// 不调 server(节省 round trip,凭证已经在本地);
// 想看实时 user_id 信息可调 GET /v1/l5/me(后续 P4.x)。
package auth

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
)

// NewWhoamiCmd creates `wau auth whoami`.
func NewWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the currently logged-in user",
		Long: `Show the currently logged-in user (reads local ~/.wau/credentials).

Displays:
  - user_id
  - token expiration time (and remaining time)
  - token prefix (first 20 chars, for sanity check)

If no credentials file exists, prints a friendly hint and returns success.

Examples:
  wau auth whoami
  wau auth whoami -o json   # (future) machine-readable output`,
		Aliases: []string{"status"},
		Args:    cobra.NoArgs,
		RunE:    runAuthWhoami,
	}
}

func runAuthWhoami(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	creds, err := wauclient.LoadCredentials("")
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}

	if creds.AccessToken == "" {
		fmt.Fprintln(out, "Not logged in.")
		fmt.Fprintln(out, "Hint: run `wau auth login` to authenticate.")
		return nil
	}

	fmt.Fprintf(out, "User:        %s\n", creds.UserID)
	if creds.ExpiresAt > 0 {
		expTime := time.Unix(creds.ExpiresAt, 0)
		remaining := time.Until(expTime)
		fmt.Fprintf(out, "Expires:     %s (in %s)\n",
			expTime.Format(time.RFC3339), formatDuration(remaining))
		// 过期警告
		if remaining < 0 {
			fmt.Fprintln(out, "⚠️  Token has EXPIRED; refresh needed")
		} else if remaining < 5*time.Minute {
			fmt.Fprintln(out, "⚠️  Token expires in <5 minutes; refresh soon")
		}
	} else {
		fmt.Fprintln(out, "Expires:     never (no expires_at)")
	}
	if creds.Endpoint != "" {
		fmt.Fprintf(out, "Endpoint:    %s\n", creds.Endpoint)
	}
	// Token 前 20 字符(sanity check,不暴露全 token)
	tokPrefix := creds.AccessToken
	if len(tokPrefix) > 20 {
		tokPrefix = tokPrefix[:20] + "..."
	}
	fmt.Fprintf(out, "Token:       %s\n", tokPrefix)
	return nil
}

// formatDuration 人类友好的 duration 字符串。
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "EXPIRED"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
}