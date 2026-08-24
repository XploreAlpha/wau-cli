// Package remote — push.go
//
// 4.1.4 (2026-08-24, v1.1.0 子项 4.1) — PushStack:把 stack v1.1 服务的 binary
// + configs + secrets 推到远端 host。
//
// 推送策略(简化版,4.1.x follow-up 可加并发):
//   - 1 个 service = 1 个 binary(或 command 解析到的二进制)
//   - 目标路径:/usr/local/bin/<binary name>(可改,4.1.x)
//   - 远端目录结构:~/.wau/run / ~/.wau/log / /usr/local/bin
//   - configs:写到 ~/.wau/configs/<name>(顶层 configs map 的 file 内容)
//   - secrets:写到 /run/secrets/<name>(顶层 secrets map 的 file 内容)
//
// D60 additive:不动 stack pkg 现有结构;只新增 PushStack 函数。
package remote

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	stackpkg "github.com/wau/wau-cli/internal/stack"
)

// PushOpts 控制 PushStack 行为。
type PushOpts struct {
	// BinDir — 远端 binary 目标目录(default /usr/local/bin)。
	BinDir string
	// ConfigDir — 远端 config 目录(default ~/.wau/configs)。
	ConfigDir string
	// SecretsDir — 远端 secret 目录(default /run/secrets)。
	SecretsDir string
	// DryRun — 只打印 plan,不真传(默认 false)。
	DryRun bool
}

// PushStack 把 stack v1.1 服务 binary + 顶层 configs/secrets 推到远端 host。
//
// 流程:
//   1. mkdir -p BinDir / ConfigDir / SecretsDir
//   2. 对每个 service(kind=binary):DefaultLookup 解析本机路径 → scp → chmod 0755
//   3. 对每个顶层 configs[name]:写本地 tmp 文件 → scp 到 ConfigDir/<name>
//   4. 对每个顶层 secrets[name]:从 file 或 env 读内容 → scp 到 SecretsDir/<name>(mode 0600)
//
// 注:kind=external / docker 跳过(不需要推 binary)。
func PushStack(ctx context.Context, c RemoteClient, stack *stackpkg.StackV11, opts PushOpts) error {
	if c == nil {
		return fmt.Errorf("remote client is nil")
	}
	if stack == nil {
		return fmt.Errorf("stack is nil")
	}
	if opts.BinDir == "" {
		opts.BinDir = "/usr/local/bin"
	}
	if opts.ConfigDir == "" {
		opts.ConfigDir = "~/.wau/configs"
	}
	if opts.SecretsDir == "" {
		opts.SecretsDir = "/run/secrets"
	}

	// 0. 准备远端目录
	for _, dir := range []string{opts.BinDir, opts.ConfigDir, opts.SecretsDir} {
		if opts.DryRun {
			fmt.Printf("  [dry-run] mkdir -p %s:%s\n", c.Host(), dir)
			continue
		}
		if err := c.MkdirAll(ctx, dir); err != nil {
			return fmt.Errorf("mkdir remote %s: %w", dir, err)
		}
	}

	// 1. 推每个 binary kind 的 service
	lookup := stackpkg.DefaultLookup()
	for name, svc := range stack.Services {
		if svc.Kind == stackpkg.KindExternal {
			continue
		}
		if svc.Kind == stackpkg.KindDocker {
			// reserved v1.1.x,跳过
			fmt.Printf("  ⚠ service %s: kind=docker reserved, skipping binary push\n", name)
			continue
		}

		// binary name 解析:binary field 优先,否则 command[0]
		binaryName := svc.Binary
		if binaryName == "" && len(svc.Command) > 0 {
			binaryName = svc.Command[0]
		}
		if binaryName == "" {
			continue // 没 binary 也不报错(后续 up 时会处理)
		}

		// 本地解析
		binPath, lookupErr := lookup.Resolve(binaryName)
		if lookupErr != nil {
			return fmt.Errorf("resolve binary %q for service %q: %w",
				binaryName, name, lookupErr)
		}

		dst := filepath.Join(opts.BinDir, binaryName)
		if opts.DryRun {
			fmt.Printf("  [dry-run] scp %s -> %s:%s\n", binPath, c.Host(), dst)
			continue
		}
		if err := c.ScpFile(ctx, binPath, dst, 0o755); err != nil {
			return fmt.Errorf("push binary %s: %w", binaryName, err)
		}
		fmt.Printf("  ✓ pushed %s -> %s:%s\n", binaryName, c.Host(), dst)
	}

	// 2. 推顶层 configs(map[string]string, file content)
	if err := pushConfigMap(ctx, c, stack.Configs, opts.ConfigDir, opts.DryRun); err != nil {
		return err
	}

	// 3. 推顶层 secrets
	if err := pushSecrets(ctx, c, stack.Secrets, opts.SecretsDir, opts.DryRun); err != nil {
		return err
	}

	return nil
}

// pushConfigMap 把 configs[name]=content 写到 tmp 文件 → scp 到 ConfigDir/name。
func pushConfigMap(ctx context.Context, c RemoteClient, configs map[string]string, dir string, dryRun bool) error {
	if len(configs) == 0 {
		return nil
	}
	for name, content := range configs {
		tmp, err := os.CreateTemp("", "wau-config-*.tmp")
		if err != nil {
			return fmt.Errorf("tmp file for config %s: %w", name, err)
		}
		if _, err := tmp.WriteString(content); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return fmt.Errorf("write tmp %s: %w", tmp.Name(), err)
		}
		tmp.Close()

		dst := filepath.Join(dir, name)
		if dryRun {
			fmt.Printf("  [dry-run] scp config %s -> %s:%s\n", tmp.Name(), c.Host(), dst)
			os.Remove(tmp.Name())
			continue
		}
		if err := c.ScpFile(ctx, tmp.Name(), dst, 0o644); err != nil {
			os.Remove(tmp.Name())
			return fmt.Errorf("push config %s: %w", name, err)
		}
		os.Remove(tmp.Name())
		fmt.Printf("  ✓ pushed config %s -> %s:%s\n", name, c.Host(), dst)
	}
	return nil
}

// pushSecrets 从 file 或 env 读 secret 内容,写到 tmp → scp 到 SecretsDir/<key>(mode 0600)。
func pushSecrets(ctx context.Context, c RemoteClient, secrets map[string]stackpkg.SecretSpec, dir string, dryRun bool) error {
	if len(secrets) == 0 {
		return nil
	}
	for key, spec := range secrets {
		var content []byte
		var err error
		switch {
		case spec.File != "":
			content, err = os.ReadFile(spec.File)
			if err != nil {
				return fmt.Errorf("read secret file %s: %w", spec.File, err)
			}
		case spec.Env != "":
			content = []byte(os.Getenv(spec.Env))
			if len(content) == 0 {
				return fmt.Errorf("secret env %s is empty", spec.Env)
			}
		default:
			return fmt.Errorf("secret %s has no file or env source", key)
		}

		tmp, err := os.CreateTemp("", "wau-secret-*.tmp")
		if err != nil {
			return fmt.Errorf("tmp file for secret %s: %w", key, err)
		}
		if _, err := tmp.Write(content); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return fmt.Errorf("write secret tmp: %w", err)
		}
		tmp.Close()

		dst := filepath.Join(dir, key)
		if dryRun {
			fmt.Printf("  [dry-run] scp secret %s -> %s:%s (mode 0600)\n", tmp.Name(), c.Host(), dst)
			os.Remove(tmp.Name())
			continue
		}
		if err := c.ScpFile(ctx, tmp.Name(), dst, 0o600); err != nil {
			os.Remove(tmp.Name())
			return fmt.Errorf("push secret %s: %w", key, err)
		}
		os.Remove(tmp.Name())
		fmt.Printf("  ✓ pushed secret %s -> %s:%s (mode 0600)\n", key, c.Host(), dst)
	}
	return nil
}