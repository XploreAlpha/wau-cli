// Package remote — dial.go
//
// 4.1.4 (2026-08-24, v1.1.0 子项 4.1) — DialRemote wrapper:cmd 层入口。
//
// 设计:
//   - 空 addr → 返 (nil, nil):表示"本地模式",不连 SSH
//   - 非空 addr → Dial 然后 ping(Exec "echo ok")验证连通
//   - 错误:返 error 给 cmd 层
//
// 放本包内(而不是 stack pkg)是为了避 import cycle(stack 包已经 import remote,
// 不能再让 remote import stack pkg)。cmd 层用 remote.DialRemote。
package remote

import (
	"context"
	"fmt"
)

// DialRemote 解析 + 连接远端 host(空 addr 表示本地模式)。
//
// 返 (nil, nil) = 本地模式(ProcessManager 走老路径)。
// 返 (client, nil) = 已连通远端,走 PushStack / StartRemote。
// 返 (nil, err) = 连接失败,cmd 层报 user。
func DialRemote(addr string) (RemoteClient, error) {
	if addr == "" {
		return nil, nil // 本地模式
	}
	client, err := Dial(addr, DialOpts{})
	if err != nil {
		return nil, fmt.Errorf("dial remote %s: %w", addr, err)
	}
	// ping 验证连通
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := client.Exec(ctx, "echo ok"); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping remote %s: %w", client.Host(), err)
	}
	return client, nil
}