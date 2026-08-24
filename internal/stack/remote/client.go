// Package remote — client.go
//
// 4.1.4 (2026-08-24, v1.1.0 子项 4.1) — SSH 远程执行 client(shell-out 到 ssh/scp CLI)。
//
// 设计:不引入 golang.org/x/crypto/ssh(避免新增 dep + SSH key parsing 复杂度),
// 而是包一层 `ssh`/`scp` 子进程。这样:
//   - 0 新 dep(D60 spirit 干净)
//   - 自动复用 user 的 ~/.ssh/config + identity files
//   - 测试可注入 mock(RemoteClient interface)
//
// 所有 cmd 层调用方走 RemoteClient interface(不要直接用 *Client)。
package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// RemoteClient SSH 远程执行 client 的抽象接口(cmd 层 + 测试用)。
//
// 所有方法接受 ctx,远端命令/文件操作可取消。
type RemoteClient interface {
	// Host 标识 "user@host[:port]"。
	Host() string

	// Exec 在远端 host 跑一条命令,返回 stdout。
	Exec(ctx context.Context, cmd string) ([]byte, error)

	// ScpFile 把本地 src 文件复制到远端 dstPath(mode 给文件权限,octal 如 0755)。
	ScpFile(ctx context.Context, src, dstPath string, mode os.FileMode) error

	// Stat 判断远端 path 是否存在(file 或 dir 都算)。
	Stat(ctx context.Context, path string) (bool, error)

	// MkdirAll 递归创建远端目录。
	MkdirAll(ctx context.Context, path string) error

	// Close 关连接(shell-out client 是 no-op;crypto/ssh client 实际关)。
	Close() error
}

// DialOpts Dial 选项。
type DialOpts struct {
	IdentityFile  string // -i path/to/key
	Port          int    // default 22(从 addr 解析)
	StrictHostKey bool   // 默认 false(用 -o StrictHostKeyChecking=accept-new)
}

// sshArgs 构造 ssh CLI 通用 flags。
func sshArgs(c *Client) []string {
	args := []string{}
	if c.identityFile != "" {
		args = append(args, "-i", c.identityFile)
	}
	args = append(args,
		"-p", strconv.Itoa(c.port),
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking="+hostKeyOpt(c.strictHostKey),
		"-o", "ConnectTimeout=10",
	)
	return args
}

func hostKeyOpt(strict bool) string {
	if strict {
		return "yes"
	}
	return "accept-new"
}

// Client 是 RemoteClient 的 ssh/scp shell-out 实现。
type Client struct {
	host        string
	user        string
	port        int
	identityFile string
	strictHostKey bool
}

// Dial 解析 addr + 构造 Client(不真连 — 真连发生在首次 Exec/ScpFile)。
//
// addr 支持:
//   - "user@host" / "user@host:port"
//   - "ssh://user@host[:port]"
//
// 解析失败返回 error。
func Dial(addr string, opts DialOpts) (*Client, error) {
	user, host, port, err := parseAddr(addr)
	if err != nil {
		return nil, err
	}
	if opts.Port != 0 {
		port = opts.Port
	}
	if port == 0 {
		port = 22
	}
	return &Client{
		host:          host,
		user:          user,
		port:          port,
		identityFile:  opts.IdentityFile,
		strictHostKey: opts.StrictHostKey,
	}, nil
}

// Host 实现 RemoteClient。
func (c *Client) Host() string {
	return fmt.Sprintf("%s@%s:%d", c.user, c.host, c.port)
}

// hostArg 返回 "user@host"(scp/ssh CLI target 格式)。
func (c *Client) hostArg() string {
	return fmt.Sprintf("%s@%s", c.user, c.host)
}

// Exec 实现 RemoteClient。
func (c *Client) Exec(ctx context.Context, cmd string) ([]byte, error) {
	args := append(sshArgs(c), c.hostArg(), "--", cmd)
	out, err := exec.CommandContext(ctx, "ssh", args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return out, fmt.Errorf("ssh exec failed (exit %d): %s",
				ee.ExitCode(), strings.TrimSpace(string(ee.Stderr)))
		}
		return out, fmt.Errorf("ssh exec: %w", err)
	}
	return out, nil
}

// ScpFile 实现 RemoteClient。
func (c *Client) ScpFile(ctx context.Context, src, dstPath string, mode os.FileMode) error {
	// scp -P port src user@host:dstPath
	args := []string{"-P", strconv.Itoa(c.port), "-q"}
	if c.identityFile != "" {
		args = append(args, "-i", c.identityFile)
	}
	args = append(args, src, c.hostArg()+":"+dstPath)
	cmd := exec.CommandContext(ctx, "scp", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp %s -> %s:%s: %w (out: %s)",
			src, c.Host(), dstPath, err, string(out))
	}
	// 改权限(独立 ssh exec,失败仅 warning)
	if mode != 0 {
		_, _ = c.Exec(ctx, fmt.Sprintf("chmod %o %s", mode.Perm(), dstPath))
	}
	return nil
}

// Stat 实现 RemoteClient。
func (c *Client) Stat(ctx context.Context, path string) (bool, error) {
	out, err := c.Exec(ctx, fmt.Sprintf("[ -e %s ] && echo Y || echo N", path))
	if err != nil {
		return false, err
	}
	return bytes.Equal(bytes.TrimSpace(out), []byte("Y")), nil
}

// MkdirAll 实现 RemoteClient。
func (c *Client) MkdirAll(ctx context.Context, path string) error {
	_, err := c.Exec(ctx, fmt.Sprintf("mkdir -p %s", path))
	return err
}

// Close 实现 RemoteClient(shell-out 是 no-op)。
func (c *Client) Close() error { return nil }

// parseAddr 解析 "user@host[:port]" 或 "ssh://user@host[:port]"。
//
// 返回 user / host / port(0 表示未指定,由 caller 用默认 22 填)。
func parseAddr(addr string) (user, host string, port int, err error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", "", 0, errors.New("remote addr is empty")
	}
	// 1. ssh:// 形式
	if strings.HasPrefix(addr, "ssh://") {
		u, perr := url.Parse(addr)
		if perr != nil {
			return "", "", 0, fmt.Errorf("parse ssh URL: %w", perr)
		}
		user = u.User.Username()
		host = u.Hostname()
		if p := u.Port(); p != "" {
			port, _ = strconv.Atoi(p)
		}
		if user == "" {
			user = currentUser()
		}
		return user, host, port, nil
	}
	// 2. user@host[:port] 形式
	at := strings.LastIndex(addr, "@")
	if at < 0 {
		return currentUser(), addr, 0, nil
	}
	user = addr[:at]
	rest := addr[at+1:]
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		// 检查是不是 IPv6(简化:含 ":" 但 host 里有 ":" 多半是 IPv6)
		if !strings.Contains(rest[:i], ":") {
			host = rest[:i]
			port, _ = strconv.Atoi(rest[i+1:])
			return user, host, port, nil
		}
	}
	host = rest
	return user, host, 0, nil
}

// currentUser 取 $USER,fallback 当前 OS user。
func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("LOGNAME"); u != "" {
		return u
	}
	return "root"
}