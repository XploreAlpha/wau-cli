// Package cmd - completion.go
//
// 第二刀 P1.4 — bash/zsh/fish/powershell shell completion。
//
// `wau completion <shell>` 生成对应 shell 的 completion 脚本,用户 eval 或 source。
//
// 设计:
//   - bash:基于 spf13/cobra bash completion(已内置)
//   - zsh:同上(已内置)
//   - fish:同上(已内置)
//   - powershell:同上(已内置)
//
// 用法:
//   # bash
//   source <(wau completion bash)
//   # zsh
//   wau completion zsh > "${fpath[1]}/_wau"
//   # fish
//   wau completion fish | source
//   # powershell
//   wau completion powershell | Out-String | Invoke-Expression
package cmd

import (
	"github.com/spf13/cobra"
)

// NewCompletionCmd 返回 `wau completion <shell>` 子命令。
//
// shell ∈ {bash, zsh, fish, powershell}。
func NewCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion <bash|zsh|fish|powershell>",
		Short: "Generate shell completion script",
		Long: `Generate a shell completion script for wau and output it to stdout.

Examples:
  # bash (add to ~/.bashrc)
  source <(wau completion bash)

  # zsh (wau completion zsh > "${fpath[1]}/_wau"; autoload -U compinit; compinit)
  wau completion zsh > "${fpath[1]}/_wau"

  # fish (add to ~/.config/fish/config.fish)
  wau completion fish | source

  # powershell (add to $PROFILE)
  wau completion powershell | Out-String | Invoke-Expression`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			}
			return nil
		},
	}
}