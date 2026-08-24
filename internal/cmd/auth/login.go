// Package auth - login.go
//
// P4.3 — `wau auth login` 实现。
//
// 跟 `wau agent login` 的关系:
//   - `wau agent login` 是 L5 包管理器 specific 入口(2026-07-10 旧路径,保留)
//   - `wau auth login` 是 OS-level 顶层入口(P4.3,2026-08-24 新增)
//   - 两个都调 client.Login(DRY)
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	wauclient "github.com/wau/wau-cli/internal/client"
)

// GetKernelAddr / GetRole 由 root.go 注入(避免直接 import cmd 包,会 cycle)。
var (
	GetKernelAddr = func() string { return "http://localhost:18400" }
	GetRole       = func() string { return "external_agent" }
)

// SetAccessors 注入 kernel addr / role getter(root.go 用)。
func SetAccessors(kernelAddr, role func() string) {
	GetKernelAddr = kernelAddr
	GetRole = role
}

var (
	flagLoginUser     string
	flagLoginPassword string
	flagLoginEndpoint string
	flagLoginNoStore  bool
)

// NewLoginCmd creates `wau auth login`.
func NewLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to a WAU kernel",
		Long: `Log in to a WAU kernel (acquire JWT access + refresh token).

Equivalent to docker login / npm login.

By default, prompts for username and password via stdin (plain text — known
limitation, see P4.x plan for password masking). Use --user / --password for
non-interactive mode (e.g. scripts).

On success, credentials are stored in ~/.wau/credentials (mode 0600).
Use --no-store to skip persistence (test only).

Examples:
  # Interactive
  wau auth login

  # Non-interactive
  wau auth login --user alice --password s3cret

  # Custom kernel endpoint
  wau auth login --endpoint http://localhost:18400

  # Don't persist (test only)
  wau auth login --no-store`,
		Args: cobra.NoArgs,
		RunE: runAuthLogin,
	}
	cmd.Flags().StringVar(&flagLoginUser, "user", "", "username (skip interactive prompt)")
	cmd.Flags().StringVar(&flagLoginPassword, "password", "", "password (skip interactive prompt; visible in shell history!)")
	cmd.Flags().StringVar(&flagLoginEndpoint, "endpoint", "", "registry endpoint override (default: kernel addr)")
	cmd.Flags().BoolVar(&flagLoginNoStore, "no-store", false, "don't persist credentials to disk (test only)")
	return cmd
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	// 1. 取 user/pass
	username := flagLoginUser
	password := flagLoginPassword
	if username == "" || password == "" {
		if username == "" {
			fmt.Fprint(out, "Username: ")
			if _, err := fmt.Scanln(&username); err != nil {
				return fmt.Errorf("read username: %w", err)
			}
		}
		if password == "" {
			fmt.Fprint(out, "Password: ")
			if _, err := fmt.Scanln(&password); err != nil {
				return fmt.Errorf("read password: %w", err)
			}
		}
	}

	// 2. 调 client.Login
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	creds, err := wauclient.Login(ctx, wauclient.LoginOptions{
		BaseURL:  GetKernelAddr(),
		Role:     GetRole(),
		Username: username,
		Password: password,
		Endpoint: flagLoginEndpoint,
	})
	if err != nil {
		return err
	}

	// 3. 存盘(默认开启)
	if !flagLoginNoStore {
		if err := creds.Save(""); err != nil {
			return fmt.Errorf("save credentials: %w", err)
		}
		fmt.Fprintf(out, "✓ Logged in as %s\n", creds.UserID)
		fmt.Fprintf(out, "  Token expires at: %s\n",
			time.Unix(creds.ExpiresAt, 0).Format(time.RFC3339))
		fmt.Fprintf(out, "  Credentials saved to: %s (mode 0600)\n",
			wauclient.DefaultCredentialsPath())
	} else {
		fmt.Fprintf(out, "✓ Logged in as %s (NOT stored per --no-store)\n", creds.UserID)
		fmt.Fprintf(out, "  access_token: %s\n", creds.AccessToken)
	}
	return nil
}