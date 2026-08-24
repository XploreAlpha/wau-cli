// Package stack — up_v11.go
//
// 4.1.5 (2026-08-24, v1.1.0 子项 4.1) — `wau stack up` v1.1 schema 路径 + remote SSH push/exec。
//
// D60 additive:不动 runUp 现有 v1 路径;新文件 up_v11.go 加 runUpV11 + dispatcher + 转换 helper。
// 触发条件:upFile 给的 YAML 是 v1.1(version: "1.1"),否则走老 v1 路径。
package stack

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	stackpkg "github.com/wau/wau-cli/internal/stack"
	"github.com/wau/wau-cli/internal/stack/defaults"
	"github.com/wau/wau-cli/internal/stack/remote"
)

var upRemote string // --remote flag(4.1.5)

// isV11YAML 探查文件 version 字段,返回 true 如果是 v1.1。
// 用于 up.go runUp 顶部 dispatcher。
func isV11YAML(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var probe struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Version == stackpkg.StackVersionV11
}

// runUpV11V11Entry 入口(per 4.1.5)。
//
// 流程:
//   1. 加载 YAML(--file 或 defaults/wau-stack.yml embed)
//   2. ApplyProfile 过滤服务名
//   3. dry-run 路径打印 plan 后 return
//   4. remote path(若 --remote):
//      a. DialRemote 连接
//      b. PushStack 推 binary + configs + secrets
//      c. 对每个 service 调 StartRemote(按 topo 序)
//   5. local path:对每个 service 转 v1 Service → 复用 ProcessManager.Start
//   6. 写 runtime state 到 ~/.wau/run/<stack_id>.json
func runUpV11(cmd *cobra.Command, args []string) error {
	// 0. profile 互斥
	profile := upProfile
	if upDemo {
		if profile != "" {
			return fmt.Errorf("--demo and --profile are mutually exclusive")
		}
		profile = "demo"
	}

	// 1. 加载 YAML
	var data []byte
	if upFile != "" {
		read, err := os.ReadFile(upFile)
		if err != nil {
			return fmt.Errorf("read --file %s: %w", upFile, err)
		}
		data = read
	} else {
		data = defaults.DefaultStackYAMLBytes()
	}
	stack, err := stackpkg.ParseV11(data)
	if err != nil {
		return err
	}

	// 2. 应用 profile → topo 服务名列表
	serviceNames, err := stack.ApplyProfile(profile)
	if err != nil {
		return err
	}

	// 3. dry-run 路径
	if upDryRun {
		fmt.Fprintf(cmd.OutOrStdout(),
			"Plan (dry-run, v1.1, stack_id=%q, release=%q, profile=%q, remote=%q):\n",
			stack.StackID, stack.Release, profile, upRemote)
		fmt.Fprintf(cmd.OutOrStdout(), "  %d services in topo order:\n", len(serviceNames))
		for i, name := range serviceNames {
			svc, _ := stack.ServiceByName(name)
			markers := ""
			if svc != nil {
				if svc.Required {
					markers += " [required]"
				}
				if svc.Image != "" {
					markers += " [image-reserved]"
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s%s\n", i+1, name, markers)
		}
		if upRemote != "" {
			fmt.Fprintf(cmd.OutOrStdout(),
				"\n[remote=%s] PushStack would push binaries + configs + secrets.\n",
				upRemote)
			fmt.Fprintln(cmd.OutOrStdout(), "[remote] StartRemote would invoke each service in topo order.")
		}
		return nil
	}

	// 4. remote 路径:DialRemote + PushStack
	var rc remote.RemoteClient
	if upRemote != "" {
		c, err := remote.DialRemote(upRemote)
		if err != nil {
			return fmt.Errorf("dial remote %s: %w", upRemote, err)
		}
		defer c.Close()
		rc = c
		fmt.Fprintf(cmd.OutOrStdout(), "→ Pushing binaries + configs + secrets to %s...\n", rc.Host())
		if err := remote.PushStack(cmd.Context(), rc, stack, remote.PushOpts{}); err != nil {
			return fmt.Errorf("push stack to remote: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "  push complete")
	}

	// 5. 解析数据目录 + runtime(从 stack.StackID)
	dataDir, logDir, err := resolveStackDirsV11(stack)
	if err != nil {
		return err
	}
	rt, err := stackpkg.LoadRuntime(dataDir, stack.StackID)
	if err != nil {
		return fmt.Errorf("load runtime: %w", err)
	}
	rt.DataDir = dataDir
	rt.LogDir = logDir

	// 6. 按 topo 顺序 start
	pm := stackpkg.NewProcessManager()
	results := make(map[string]error)
	startedAt := time.Now()

	modeStr := "local"
	if rc != nil {
		modeStr = "remote"
	}
	fmt.Fprintf(cmd.OutOrStdout(),
		"\nBringing up stack %q v1.1 (%d services, mode=%s, release=%q)...\n\n",
		stack.StackID, len(serviceNames), modeStr, stack.Release)

	for _, name := range serviceNames {
		svc, _ := stack.ServiceByName(name)
		if svc == nil {
			continue
		}

		// external skip
		if svc.Kind == stackpkg.KindExternal {
			fmt.Fprintf(cmd.OutOrStdout(), "  ⠋ %-15s ... external (skipped)\n", name)
			_ = rt.SetStatus(name, "external", 0, nil)
			continue
		}
		// docker kind / image 字段 reserved
		if svc.Image != "" {
			fmt.Fprintf(cmd.OutOrStdout(),
				"  ⚠ %-15s image=%q reserved for v1.1.x, skipped\n", name, svc.Image)
			continue
		}

		// 已 running 跳过
		if existing, ok := rt.Services[name]; ok && existing.Status == "running" && stackpkg.IsAlive(existing.PID) {
			fmt.Fprintf(cmd.OutOrStdout(),
				"  ✓ %-15s already running (pid %d)\n", name, existing.PID)
			continue
		}

		// start
		fmt.Fprintf(cmd.OutOrStdout(), "  ⠋ %-15s ... ", name)
		startT := time.Now()
		ctx := context.Background()

		if rc != nil {
			// remote 路径
			pid, err := remote.StartRemote(ctx, rc, *svc, name)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "✗ remote (%v)\n", err)
				results[name] = err
				_ = rt.SetStatus(name, "failed", 0, nil)
				if rt.Services[name] != nil {
					rt.Services[name].LastError = err.Error()
					_ = rt.Save()
				}
				if svc.Required {
					return fmt.Errorf("required service %q remote start failed: %w", name, err)
				}
				continue
			}
			_ = rt.SetStatus(name, "running", pid, map[string]interface{}{
				"binary":  svc.Binary,
				"remote":  rc.Host(),
				"logFile": fmt.Sprintf("/tmp/wau-%s.log", name),
			})
			took := time.Since(startT).Truncate(100 * time.Millisecond)
			fmt.Fprintf(cmd.OutOrStdout(), "✓ remote pid %d (%.1fs)\n", pid, took.Seconds())
		} else {
			// local 路径:转 v1 Service → pm.Start
			v1Svc := convertToV1Service(name, svc)
			pid, err := pm.Start(ctx, v1Svc, logDir)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "✗ (%v)\n", err)
				results[name] = err
				_ = rt.SetStatus(name, "failed", 0, nil)
				if rt.Services[name] != nil {
					rt.Services[name].LastError = err.Error()
					_ = rt.Save()
				}
				if svc.Required {
					return fmt.Errorf("required service %q failed: %w", name, err)
				}
				continue
			}
			_ = stackpkg.WritePIDFile(stackpkg.PidFilePath(dataDir, name), pid)
			httpPort, grpcPort := extractPortsFromList(svc.Ports)
			_ = rt.SetStatus(name, "running", pid, map[string]interface{}{
				"binary":     svc.Binary,
				"binaryPath": pm.Lookup.Home,
				"logFile":    fmt.Sprintf("%s/%s.log", logDir, name),
				"httpPort":   httpPort,
				"grpcPort":   grpcPort,
			})

			// 健康探针(本机)
			if !upDetach && svc.Healthcheck != nil {
				probe := healthToProbe(svc.Healthcheck)
				runner, perr := stackpkg.NewProbeRunner(probe, name)
				if perr != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "✗ (probe init: %v)\n", perr)
					continue
				}
				probeCtx, cancel := context.WithTimeout(ctx, upWaitMax)
				result := runner.Run(probeCtx)
				cancel()
				if !result.OK {
					fmt.Fprintf(cmd.OutOrStdout(), "✗ health: %v\n", result.Err)
					results[name] = result.Err
					if svc.Required {
						return fmt.Errorf("required service %q health check failed: %w", name, result.Err)
					}
					continue
				}
			}
			took := time.Since(startT).Truncate(100 * time.Millisecond)
			fmt.Fprintf(cmd.OutOrStdout(), "✓ (%.1fs)\n", took.Seconds())
		}
	}

	// 7. 汇总
	total := time.Since(startedAt).Truncate(100 * time.Millisecond)
	fmt.Fprintf(cmd.OutOrStdout(),
		"\n✓ Stack %q v1.1 up. %d services, took %s.\n",
		stack.StackID, len(serviceNames), total)
	if len(results) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(),
			"⚠ %d service(s) had issues (optional-only; required would have aborted): %v\n",
			len(results), results)
		return upV11ExitError(2)
	}
	return nil
}

// convertToV1Service 把 v1.1 ServiceV11 转成 v1 Service(给 ProcessManager.Start 用)。
//
// 字段映射:
//   - name / kind / binary / args / env / depends_on / required → 直传
//   - command 合并到 args(command 作为 entrypoint,args 作为参数)
//   - ports 解析:第一个 → http_port, 第二个 → grpc_port(per default.go 约定)
//   - healthcheck → Probe
func convertToV1Service(name string, svc *stackpkg.ServiceV11) *stackpkg.Service {
	mergedArgs := append([]string(nil), svc.Args...)
	mergedArgs = append(mergedArgs, svc.Command...)
	httpPort, grpcPort := extractPortsFromList(svc.Ports)
	var probe *stackpkg.Probe
	if svc.Healthcheck != nil {
		probe = healthToProbe(svc.Healthcheck)
	}
	return &stackpkg.Service{
		Name:      name,
		Kind:      svc.Kind,
		Binary:    svc.Binary,
		Args:      mergedArgs,
		Env:       svc.Env,
		HTTPPort:  httpPort,
		GRPCPort:  grpcPort,
		DependsOn: svc.DependsOn,
		Required:  svc.Required,
		Health:    probe,
	}
}

// extractPortsFromList 从 ["18400:18400"] / ["18403:18403", "18404:18404"] 抽出端口号。
// 第一个 = http_port, 第二个 = grpc_port(per default.go 9-service 约定)。
// 单端口元素只有 http_port。
func extractPortsFromList(ports []string) (http, grpc int) {
	for i, p := range ports {
		// "host[:container]" → 取 host 部分
		host := extractHostPortFromSpec(p)
		n := atoiSafe(host)
		switch i {
		case 0:
			http = n
		case 1:
			grpc = n
		}
	}
	return
}

func extractHostPortFromSpec(spec string) string {
	for i, c := range spec {
		if c == ':' {
			return spec[:i]
		}
	}
	return spec
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// toProbe 把 v1.1 HealthcheckSpec 转成 v1 Probe(给 NewProbeRunner 用)。
//
// 支持 tcp / http / exec。grpc 在 v1 ProcessManager 里没独立类型,这里降级为 tcp probe。
func healthToProbe(h *stackpkg.HealthcheckSpec) *stackpkg.Probe {
	if h == nil {
		return nil
	}
	interval := h.Interval
	if interval == 0 {
		interval = 500 * time.Millisecond
	}
	timeout := h.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	probe := &stackpkg.Probe{
		Interval: interval,
		Timeout:  timeout,
	}
	switch {
	case h.TCP != "":
		probe.Type = stackpkg.ProbeTCP
		probe.Addr = h.TCP
	case h.HTTP != "":
		probe.Type = stackpkg.ProbeHTTP
		probe.URL = h.HTTP
	case h.GRPC != "":
		// 简化:grpc health 当 tcp probe(addr 由 grpc 字段解析)
		probe.Type = stackpkg.ProbeTCP
		probe.Addr = h.GRPC
	case h.Exec != "":
		probe.Type = stackpkg.ProbeExec
		probe.Cmd = h.Exec
	}
	return probe
}

// resolveStackDirsV11 从 v1.1 Stack 解析数据/日志目录(沿用 v1 的 ~ 展开 + 默认值)。
func resolveStackDirsV11(stack *stackpkg.StackV11) (dataDir, logDir string, err error) {
	home, herr := os.UserHomeDir()
	if herr != nil {
		home = "/tmp"
	}
	dataDir = stack.DataDir
	if dataDir == "" {
		dataDir = home + "/.wau/run"
	} else if len(dataDir) >= 2 && dataDir[:2] == "~/" {
		dataDir = home + "/" + dataDir[2:]
	}
	logDir = home + "/.wau/log"
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create data_dir %s: %w", dataDir, err)
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create log_dir %s: %w", logDir, err)
	}
	return dataDir, logDir, nil
}

// upV11ExitError 实现 cobra ExitCode(返回非 0)。
type upV11ExitError int

func (e upV11ExitError) Error() string { return fmt.Sprintf("up v1.1 exit code %d", int(e)) }
func (upV11ExitError) ExitCode() int   { return 2 }
