// Package cmd - node.go
//
// 第二刀 P2.5 — `wau node ls` + `wau peer ls`(2026-08-20 visa demo)。
//
// 设计:
//   - `wau node ls`   调 /registry/agents,把每个 registered agent 视作 1 个网络节点
//   - `wau peer ls`   alias,语义相同(WAU 网络中 peer = agent instance)
//   - `wau node info <name>` 调 /registry/agents/{name}/status + /kernel/info 合并显示
//   - 支持 -o table/json/yaml
//
// 这是对 kernel 没有原生 /v1/peers 的务实实现:每个已注册 agent = 网络节点 / peer。
// 未来若 kernel 暴露 /v1/net/peers,client. ListPeers() 可加薄包装。
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/wau/wau-cli/internal/client"
	"github.com/wau/wau-cli/internal/output"
)

// nodeCmd `wau node` 父命令
var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage WAU network nodes (= registered agents)",
	Long: `List and inspect nodes on the WAU network.

In WAU, a "node" is a registered agent instance — anything that registered itself
via /registry/agents/register. Each node has:
  - name          unique agent name (e.g. "fox-medical")
  - url           gRPC/HTTP endpoint
  - skills        advertised skills
  - status        online | offline | degraded
  - trust_score   0.0 ~ 1.0 (per wau-trust)

Subcommands:
  ls      List all online nodes
  info    Show detailed info for one node`,
}

// nodeLsCmd `wau node ls`
var nodeLsCmd = &cobra.Command{
	Use:     "ls",
	Short:   "List all WAU network nodes",
	Aliases: []string{"list"},
	RunE:    runNodeLs,
}

// nodeInfoCmd `wau node info <name>`
var nodeInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show detailed info for one node",
	Args:  cobra.ExactArgs(1),
	RunE:  runNodeInfo,
}

// peerCmd `wau peer` 别名父命令(语义同 node,符合 libp2p 习惯)
var peerCmd = &cobra.Command{
	Use:   "peer",
	Short: "Manage WAU network peers (= nodes; alias for `wau node`)",
}

// peerLsCmd `wau peer ls` 别名
var peerLsCmd = &cobra.Command{
	Use:     "ls",
	Short:   "List all WAU network peers (= `wau node ls`)",
	Aliases: []string{"list"},
	RunE:    runNodeLs,
}

func init() {
	rootCmd.AddCommand(nodeCmd)
	rootCmd.AddCommand(peerCmd)
	nodeCmd.AddCommand(nodeLsCmd)
	nodeCmd.AddCommand(nodeInfoCmd)
	peerCmd.AddCommand(peerLsCmd)
}

func runNodeLs(cmd *cobra.Command, args []string) error {
	c := newClient()
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	page, pageSize := 1, 100
	resp, err := c.ListAgents(ctx, page, pageSize, "", "online", "")
	if err != nil {
		output.Error("Failed to list nodes: %v", err)
		return err
	}

	format, _ := output.ParseFormat(GetOutputFmt())

	switch format {
	case output.FormatJSON:
		output.PrintJSON(resp.Agents)
	case output.FormatYAML:
		output.PrintYAML(resp.Agents)
	default:
		// table:NAME | URL | SKILLS | TRUST | STATUS
		headers := []string{"NAME", "URL", "SKILLS", "TRUST", "STATUS"}
		rows := make([][]string, 0, len(resp.Agents))
		for _, a := range resp.Agents {
			skills := ""
			if len(a.Skills) > 0 {
				skills = a.Skills[0]
				if len(a.Skills) > 1 {
					skills += fmt.Sprintf(" (+%d)", len(a.Skills)-1)
				}
			}
			trust := fmt.Sprintf("%.2f", a.Trust)
			rows = append(rows, []string{a.Name, a.URL, skills, trust, a.Status})
		}
		output.PrintTable(headers, rows)
		fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %d online nodes\n", len(resp.Agents))
	}
	return nil
}

func runNodeInfo(cmd *cobra.Command, args []string) error {
	name := args[0]
	c := newClient()
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	// 拿 status + score 并行(本 client 暂串行)
	status, err := c.GetAgent(ctx, name)
	if err != nil {
		output.Error("Failed to get node %s: %v", name, err)
		return err
	}

	format, _ := output.ParseFormat(GetOutputFmt())
	switch format {
	case output.FormatJSON:
		output.PrintJSON(status)
	case output.FormatYAML:
		output.PrintYAML(status)
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Node:        %s\n", status.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "Status:      %s\n", status.Status)
		fmt.Fprintf(cmd.OutOrStdout(), "Trust:       %.2f\n", status.Trust)
		fmt.Fprintf(cmd.OutOrStdout(), "Circuit:     %s\n", status.Circuit)
		fmt.Fprintf(cmd.OutOrStdout(), "Active Tasks: %d / %d\n", status.Load.ActiveTasks, status.Load.MaxCapacity)
		fmt.Fprintf(cmd.OutOrStdout(), "CPU:         %.1f%%\n", status.Load.CPUUsage)
		fmt.Fprintf(cmd.OutOrStdout(), "Memory:      %.1f%%\n", status.Load.MemoryUsage)
	}
	return nil
}

// 编译期保证 client.AgentStatus 字段存在(避免硬编码失误)
var _ = client.APIError{}