// Package remote — proc.go
//
// 4.1.4 (2026-08-24, v1.1.0 子项 4.1) — StartRemote / StopRemote / StatusRemote:
// 远端 host 上的进程管理(基于 ssh exec,无 systemd / nohup 简化版)。
//
// 设计简化(4.1.x follow-up 可加):
//   - StartRemote:用 `setsid <cmd> &` + `$!` 拿 PID → echo PID 回 stdout
//   - StopRemote:`pkill -TERM -f <name>` → 5s 等 → `pkill -KILL -f <name>`(per KillProcessGroup 模式)
//   - StatusRemote:`pgrep -f <name>` + IsAlive(本地拿不到 PID,所以只返 PID 字符串)
//
// D60 additive:不动 stack pkg 现有 ProcessManager;StartRemote/StopRemote 是
// 远端版本,cmd 层按 `--remote` flag 路由。
package remote

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	stackpkg "github.com/wau/wau-cli/internal/stack"
)

// StartRemote 在远端 host 上 start 一个 service,返回 PID(int)。
//
// cmd 构造:kind=binary → `setsid <binary> <args...> & echo $!`;
//           kind=external → error(无需 start);
//           kind=docker → error(v1.1.x 预留)。
//
// 返回 PID 让 caller 写本地 Runtime(runtime state 是本机的,远端 PID 只是参考)。
func StartRemote(ctx context.Context, c RemoteClient, svc stackpkg.ServiceV11, name string) (int, error) {
	if svc.Kind == stackpkg.KindExternal {
		return 0, fmt.Errorf("service %s is external, not startable", name)
	}
	if svc.Kind == stackpkg.KindDocker {
		return 0, fmt.Errorf("service %s kind=docker reserved for v1.1.x", name)
	}

	// 构造远端命令
	var exec string
	switch {
	case svc.Binary != "":
		exec = svc.Binary
		if len(svc.Args) > 0 {
			exec += " " + strings.Join(svc.Args, " ")
		}
	case len(svc.Command) > 0:
		exec = strings.Join(svc.Command, " ")
	default:
		return 0, fmt.Errorf("service %s has no binary or command", name)
	}

	// setsid 起后台进程 + 写 PID 文件 + 立刻 echo PID
	cmd := fmt.Sprintf(
		`setsid bash -c '%s >/tmp/wau-%s.log 2>&1 & echo $! > /tmp/wau-%s.pid; disown'`,
		exec, name, name,
	)
	out, err := c.Exec(ctx, cmd)
	if err != nil {
		return 0, fmt.Errorf("start remote %s: %w (out: %s)", name, err, string(out))
	}

	// 读 PID file(更可靠,echo $! 可能因 race 返回错值)
	pidBytes, perr := c.Exec(ctx, fmt.Sprintf("cat /tmp/wau-%s.pid 2>/dev/null || echo 0", name))
	if perr != nil {
		return 0, fmt.Errorf("read remote PID file for %s: %w", name, perr)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid PID for %s: %q", name, strings.TrimSpace(string(pidBytes)))
	}
	return pid, nil
}

// StopRemote 远端停 service(TERM → 5s → KILL,跟 process.go KillProcessGroup 思路)。
//
// 用 service 名字做 pattern match(`pgrep -f wau-<name>`),避免按 PID(PID 在新进程可能复用)。
func StopRemote(ctx context.Context, c RemoteClient, name string) error {
	pattern := fmt.Sprintf("wau-%s", name)
	// TERM
	if _, err := c.Exec(ctx, fmt.Sprintf("pkill -TERM -f %s || true", pattern)); err != nil {
		return fmt.Errorf("pkill TERM %s: %w", name, err)
	}
	// 5s 等
	time.Sleep(5 * time.Second)
	// KILL(剩下的)
	if _, err := c.Exec(ctx, fmt.Sprintf("pkill -KILL -f %s || true", pattern)); err != nil {
		return fmt.Errorf("pkill KILL %s: %w", name, err)
	}
	// 清 PID file
	_, _ = c.Exec(ctx, fmt.Sprintf("rm -f /tmp/wau-%s.pid /tmp/wau-%s.log", name, name))
	return nil
}

// StatusRemote 查远端 service 状态。
//
// 返回:pid(0=没找到)+ running(bool,基于 pgrep 结果)。
func StatusRemote(ctx context.Context, c RemoteClient, name string) (int, bool, error) {
	pattern := fmt.Sprintf("wau-%s", name)
	out, err := c.Exec(ctx, fmt.Sprintf("pgrep -f %s | head -1", pattern))
	if err != nil {
		// pgrep 没 match 时 exit code 1,不算 error
		if strings.Contains(err.Error(), "exit 1") {
			return 0, false, nil
		}
		return 0, false, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, false, fmt.Errorf("parse pid %q: %w", string(out), err)
	}
	return pid, true, nil
}

// StopAll 远端停所有 wau-* 进程(用于 `wau stack down --remote` 时全停)。
func StopAll(ctx context.Context, c RemoteClient) error {
	_, err := c.Exec(ctx, "pkill -TERM -f 'wau-' || true")
	if err != nil {
		return err
	}
	time.Sleep(5 * time.Second)
	_, err = c.Exec(ctx, "pkill -KILL -f 'wau-' || true")
	return err
}