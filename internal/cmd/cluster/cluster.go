// Package cluster provides `wau cluster` subcommand group.
//
// P4.6 (v1.0.1, 2026-08-24) — 顶层 `wau cluster {status, agents}` 把已有
// /health + /kernel/info + /registry/agents 三个 endpoint 组合成集群视图。
//
// 设计原则:
//   - 类比 kubectl cluster-info / docker system info
//   - 不新增 kernel 端点(kernel v0.5.1 还没 /v1/cluster/*),纯 CLI 组合
//   - 跟 wau health / wau kernel info / wau agent list 是 additive — 后三者继续可用
//   - 集群视图应该一次给完整 overview(不调 3 个命令)
package cluster

import (
	"github.com/spf13/cobra"
)

// NewClusterCmd 构造 `wau cluster` 子命令组。
func NewClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Show cluster-wide status and list agents",
		Long: `Show cluster-wide status — a unified view of the WAU kernel,
its health, modules, and registered agents.

Combines three existing endpoints (/health, /kernel/info, /registry/agents)
into a single command for at-a-glance cluster overview. No new kernel
endpoints are required.

Subcommands:
  status   Show cluster status (health + kernel info + agent count)
  agents   List agents registered in the cluster

Examples:
  wau cluster status                            # default kernel (localhost)
  wau cluster status --addr http://43.134.126.126:18400
  wau cluster agents --skill multi_agent        # filter by skill
  wau cluster agents --json                     # JSON output`,
		Aliases: []string{"cl"},
	}
	cmd.AddCommand(NewStatusCmd())
	cmd.AddCommand(NewAgentsCmd())
	return cmd
}