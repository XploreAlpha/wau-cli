// Package stack provides `wau stack` subcommand group.
//
// 第一刀 1.1 — wau stack up/down/status/ls(per visa demo + 子项 4.1,2026-08-20)。
//
// 沿用 internal/cmd/agent/agent.go 的 NewCmd() 工厂模式。
package stack

import (
	"github.com/spf13/cobra"

	stackpkg "github.com/wau/wau-cli/internal/stack"
)

// NewStackCmd creates the `wau stack` command group.
func NewStackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage the local WAU stack (services lifecycle)",
		Long: `Manage the local WAU stack — bring services up, tear them down, and inspect status.

Equivalent to docker compose / kubectl for a single-node WAU deployment.

Subcommands:
  up       Bring the stack up (start all services in dependency order)
  down     Bring the stack down (stop all services gracefully)
  restart  Restart one or more services (down + up)
  ls       List services with status (alias: status, ps)
  logs     Show logs for one or all services
  log      Show logs for a single service
  validate Validate wau-stack.yml (schema + binary + port conflict)

Examples:
  # Start the full demo stack (9 services)
  wau stack up --demo

  # Start from a custom wau-stack.yml
  wau stack up --file /path/to/wau-stack.yml

  # List running services
  wau stack ls

  # Tail logs (P4.1, like docker compose logs)
  wau stack logs --follow

  # Stop everything
  wau stack down

See also: wau service <name> {start,stop,restart,logs} for per-service control.`,
		Aliases: []string{"st"},
	}

	cmd.AddCommand(newUpCmd())
	cmd.AddCommand(newDownCmd())
	cmd.AddCommand(NewRestartCmd()) // P4.4
	cmd.AddCommand(newLsCmd())
	cmd.AddCommand(NewLogCmd())
	cmd.AddCommand(NewStackLogsCmd())
	cmd.AddCommand(NewInitConfigsCmd())
	cmd.AddCommand(NewValidateCmd()) // 4.1.3 v1.1.0 子项 4.1

	return cmd
}

// Public accessors for top-level `wau up/down/status` aliases (4.3 MVP, v1.1.0).
//
// D60 additive:这些函数返回一个新的 cobra.Command 实例,共享包级 flag var(upFile / downFile / lsFile)。
// 由于 cobra 每个 root 只会调一次 flag parse,共享 flag var 在 root + stack 双注册下是安全的
// (顺序执行,不并发)。
func UpCmd() *cobra.Command    { return newUpCmd() }
func DownCmd() *cobra.Command  { return newDownCmd() }
func LsCmd() *cobra.Command    { return newLsCmd() }

// loadStack 解析 stack 配置(file flag → default stack)。
//
// 优先级:
//   1. --file 指定的文件
//   2. 内置 stackpkg.DefaultStack()
//   3. --profile 用于过滤(无则全起)
func loadStack(stackFile, profileName string) (*stackpkg.Stack, error) {
	var s *stackpkg.Stack
	if stackFile != "" {
		loaded, err := stackpkg.LoadFile(stackFile)
		if err != nil {
			return nil, err
		}
		s = loaded
	} else {
		s = stackpkg.DefaultStack()
	}
	if profileName == "" {
		return s, nil
	}
	filtered, err := s.ApplyProfile(profileName)
	if err != nil {
		return nil, err
	}
	svcCopy := *s
	svcCopy.Services = filtered
	return &svcCopy, nil
}
