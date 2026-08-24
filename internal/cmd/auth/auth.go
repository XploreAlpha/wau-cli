// Package auth - auth.go
//
// P4.3 (v1.0.1, 2026-08-24) — `wau auth login / logout / whoami` 子命令组。
//
// 类比:
//   - docker login / logout
//   - npm login / whoami
//   - kubectl auth (whoami / can-i / ...)
//   - gh auth login
//
// 设计原则:
//   - 复用 internal/client/auth.go 的 Credentials / LoadCredentials / Save
//   - 复用 internal/client.L5Login(已有)
//   - 顶层 `wau auth login` 是 OS-level 入口;
//     `wau agent login`(L5 包管理器)继续保留,D60 additive
//   - 不接 password prompt masking(本期 fmt.Scanln,已知限制)
//   - 不改 server 端(/v1/l5/login 已有)
//
// 用法:
//   wau auth login                       # 交互式(从 stdin)
//   wau auth login --user alice --password xxx
//   wau auth logout
//   wau auth whoami
package auth

import (
	"github.com/spf13/cobra"
)

// NewAuthCmd creates the `wau auth` subcommand group.
func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage WAU authentication (login/logout/whoami)",
		Long: `Manage WAU user authentication.

Equivalent to:
  docker login / logout
  npm login / whoami
  kubectl auth whoami

Subcommands:
  login    Log in to a WAU kernel (get JWT access + refresh token)
  logout   Remove local credentials
  whoami   Show current logged-in user

Credentials are stored in ~/.wau/credentials (0600 permissions).

Examples:
  # Interactive login
  wau auth login

  # Non-interactive (for scripts)
  wau auth login --user alice --password s3cret

  # Check current login
  wau auth whoami

  # Log out (remove local credentials)
  wau auth logout`,
		Aliases: []string{"login-group"}, // 防止跟 wau agent login 冲突
	}
	cmd.AddCommand(NewLoginCmd())
	cmd.AddCommand(NewLogoutCmd())
	cmd.AddCommand(NewWhoamiCmd())
	return cmd
}