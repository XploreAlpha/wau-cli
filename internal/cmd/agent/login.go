// Package agent - login.go
//
// v1.0.0 M11 P4.5 L5 包管理器 login 子命令(per D72/D73/D74,2026-07-10)。
//
// wau agent login — 登入 WAU 账户(类比 docker login / npm login)
//
// 拿 token 存 ~/.wau/credentials(本期 stub,只展示 token 不存盘)
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
)

var (
	loginEndpoint string
	loginStore    bool
)

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to a WAU registry",
		Long: `Log in to a WAU registry (per D72 ⭐L5 package manager).

Equivalent to:
  docker login
  npm login
  brew login

Saves credentials to ~/.wau/credentials (per D74).

Examples:
  # Interactive login (prompts for username/password)
  wau agent login

  # Specify endpoint
  wau agent login --endpoint=https://wau.example.com

  # Skip storing credentials (test only)
  wau agent login --no-store`,
		Args: cobra.NoArgs,
		RunE: runLogin,
	}

	cmd.Flags().StringVar(&loginEndpoint, "endpoint", "", "registry endpoint (default: current kernel)")
	cmd.Flags().BoolVar(&loginStore, "store", true, "store credentials to ~/.wau/credentials")

	return cmd
}

func runLogin(cmd *cobra.Command, args []string) error {
	var username, password string

	// 本期:从 stdin 读(后续接 M2 OAuth)
	fmt.Fprint(cmd.OutOrStdout(), "Username: ")
	if _, err := fmt.Scanln(&username); err != nil {
		return fmt.Errorf("read username: %w", err)
	}
	fmt.Fprint(cmd.OutOrStdout(), "Password: ")
	if _, err := fmt.Scanln(&password); err != nil {
		return fmt.Errorf("read password: %w", err)
	}

	c := wauclient.NewClient(wauclient.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	resp, err := c.L5Login(ctx, &wauclient.L5LoginRequest{
		Username: username,
		Password: password,
		Endpoint: loginEndpoint,
	})
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("login failed: %s", resp.Error)
	}

	// 落 ~/.wau/credentials(per D74 协议)
	if loginStore {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find home: %w", err)
		}
		credDir := filepath.Join(home, ".wau")
		if err := os.MkdirAll(credDir, 0o700); err != nil {
			return fmt.Errorf("create ~/.wau: %w", err)
		}
		credPath := filepath.Join(credDir, "credentials")
		creds := map[string]string{
			"user_id":       resp.UserID,
			"access_token":  resp.AccessToken,
			"refresh_token": resp.RefreshToken,
			"expires_at":    fmt.Sprintf("%d", resp.ExpiresAt),
		}
		buf, err := json.MarshalIndent(creds, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		if err := os.WriteFile(credPath, buf, 0o600); err != nil {
			return fmt.Errorf("write credentials: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ logged in as %s (credentials saved to %s)\n", resp.UserID, credPath)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "✅ logged in as %s (credentials NOT stored per --no-store)\n", resp.UserID)
		fmt.Fprintf(cmd.OutOrStdout(), "  access_token: %s\n", resp.AccessToken)
	}
	return nil
}