// Package stack - log.go
//
// 第四刀 P4.1 (v1.0.1, 2026-08-23) — `wau log` + `wau stack logs` 子命令。
//
// 类比:
//   - docker logs / docker compose logs
//   - kubectl logs
//   - journalctl -u <service>
//
// 设计原则:
//   - 不引入新 dep(--follow 走 `tail -F`,非 follow 走 stdlib read+filter)
//   - 多服务并行 fanout,每个服务带颜色前缀(类似 docker compose logs)
//   - log path 复用 process.go 的 LogPath helper,与 StartService 写入路径一致
//
// 用法:
//   wau log wau-core                        # 最后 50 行
//   wau log wau-core --follow                # tail -F
//   wau log wau-core --lines 200             # 最后 200 行
//   wau log wau-core --grep "ERROR"          # 过滤
//   wau log wau-core --since 5m              # 最近 5 分钟
//   wau log wau-core --no-color              # 关彩色
//   wau stack logs                           # 所有服务并行
//   wau stack logs wau-core                  # 单服务
package stack

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	stackpkg "github.com/wau/wau-cli/internal/stack"
)

// ANSI 8-color cycler for multi-service fanout prefix
var ansiColors = []string{
	"\033[36m", // cyan
	"\033[33m", // yellow
	"\033[35m", // magenta
	"\033[32m", // green
	"\033[34m", // blue
	"\033[31m", // red
	"\033[96m", // bright cyan
	"\033[93m", // bright yellow
}
const ansiReset = "\033[0m"

var (
	// flag holders
	flagFollow  bool
	flagLines   int
	flagGrep    string
	flagSince   time.Duration
	flagNoColor bool
)

// logOptions 解析后的 flag。
type logOptions struct {
	Follow  bool
	Lines   int
	Grep    string
	Since   time.Duration
	NoColor bool
}

func NewLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log <service>",
		Short: "Show logs for a stack service",
		Long: `Show recent or follow logs for a single stack service.

Equivalent to:
  docker logs <container>      (Docker)
  kubectl logs <pod>            (Kubernetes)
  journalctl -u <service>       (systemd, but we don't use systemd per project rule)

The log file is at ~/.wau/log/<service>.log (per stack LogDir config).

Examples:
  # Last 50 lines
  wau log wau-core

  # Follow (tail -F, Ctrl-C to exit)
  wau log wau-core --follow

  # Last 200 lines, filter ERROR
  wau log wau-core --lines 200 --grep ERROR

  # Last 5 minutes only
  wau log wau-core --since 5m

  # Disable color
  wau log wau-core --no-color`,
		Args: cobra.ExactArgs(1),
		RunE: runLog,
	}
	addLogFlags(cmd)
	return cmd
}

func NewStackLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [service]",
		Short: "Show logs for one or all stack services",
		Long: `Show recent or follow logs for stack services.

Without arguments, all services in the stack are tailed in parallel (with service-name prefix).

Examples:
  wau stack logs                  # all services, parallel
  wau stack logs wau-core         # single service
  wau stack logs --follow         # tail -F all
  wau stack logs wau-core --grep ERROR`,
		Args: cobra.MaximumNArgs(1),
		RunE: runStackLogs,
	}
	addLogFlags(cmd)
	return cmd
}

func addLogFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&flagFollow, "follow", "f", false, "follow log output (tail -F)")
	cmd.Flags().IntVarP(&flagLines, "lines", "n", 50, "number of lines to show (0 = all)")
	cmd.Flags().StringVar(&flagGrep, "grep", "", "filter lines matching regex")
	cmd.Flags().DurationVar(&flagSince, "since", 0, "only show lines since duration ago (e.g. 5m, 1h)")
	cmd.Flags().BoolVar(&flagNoColor, "no-color", false, "disable colored output")
}

func collectLogOptions() logOptions {
	return logOptions{
		Follow:  flagFollow,
		Lines:   flagLines,
		Grep:    flagGrep,
		Since:   flagSince,
		NoColor: flagNoColor,
	}
}

// runLog handles `wau log <service>`.
func runLog(cmd *cobra.Command, args []string) error {
	serviceName := args[0]
	s, err := loadStack("", "")
	if err != nil {
		return err
	}
	if !hasService(s, serviceName) {
		return fmt.Errorf("service %q not found in default stack (available: %s)",
			serviceName, serviceNames(s))
	}

	opts := collectLogOptions()
	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	return showServiceLog(ctx, cmd.OutOrStdout(), s.Stack.LogDir, serviceName, opts, 0)
}

// runStackLogs handles `wau stack logs [service]`.
func runStackLogs(cmd *cobra.Command, args []string) error {
	profileName := ""
	// loadStack 不需要 profile(只看 service names 是否存在)
	s, err := loadStack("", profileName)
	if err != nil {
		return err
	}

	opts := collectLogOptions()
	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if len(args) == 1 {
		serviceName := args[0]
		if !hasService(s, serviceName) {
			return fmt.Errorf("service %q not found in default stack (available: %s)",
				serviceName, serviceNames(s))
		}
		return showServiceLog(ctx, cmd.OutOrStdout(), s.Stack.LogDir, serviceName, opts, 0)
	}

	// 无 service 名 = 所有服务 fanout
	services := make([]string, 0, len(s.Services))
	for _, svc := range s.Services {
		// redis 是 external,无 log file;skip
		if svc.Kind == stackpkg.KindExternal {
			continue
		}
		services = append(services, svc.Name)
	}
	if len(services) == 0 {
		return fmt.Errorf("no loggable services in stack (only redis/external found)")
	}
	return fanoutLogs(ctx, cmd.OutOrStdout(), s.Stack.LogDir, services, opts)
}

// hasService 检查 stack 是否含指定 service。
func hasService(s *stackpkg.Stack, name string) bool {
	for _, svc := range s.Services {
		if svc.Name == name {
			return true
		}
	}
	return false
}

// serviceNames 列出所有服务名(用于错误信息)。
func serviceNames(s *stackpkg.Stack) []string {
	out := make([]string, 0, len(s.Services))
	for _, svc := range s.Services {
		out = append(out, svc.Name)
	}
	return out
}

// showServiceLog 输出单服务日志。
func showServiceLog(ctx context.Context, w io.Writer, logDir, service string, opts logOptions, colorIdx int) error {
	logPath := stackpkg.LogPath(expandHome(logDir), service)
	if _, err := os.Stat(logPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no log file at %s — run `wau stack up` first", logPath)
		}
		return err
	}

	filter, err := buildFilter(opts)
	if err != nil {
		return err
	}

	prefix := colorPrefix(service, colorIdx, opts.NoColor)
	prefixed := newPrefixedWriter(w, prefix)

	if opts.Follow {
		sw := stackpkg.NewSafeWriter(prefixed)
		return stackpkg.FollowLog(ctx, logPath, sw, filter)
	}

	// 非 follow:读整个文件 → 过滤 → 取最后 N 行
	return readAndPrint(ctx, logPath, opts.Lines, filter, prefixed)
}

// fanoutLogs 多服务并行输出(每个服务带独立颜色前缀)。
func fanoutLogs(ctx context.Context, w io.Writer, logDir string, services []string, opts logOptions) error {
	filter, err := buildFilter(opts)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(services))
	for i, svc := range services {
		i, svc := i, svc
		logPath := stackpkg.LogPath(expandHome(logDir), svc)
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			fmt.Fprintf(w, "[%s] (no log yet)\n", svc)
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			prefix := colorPrefix(svc, i, opts.NoColor)
			prefixed := newPrefixedWriter(w, prefix)

			if opts.Follow {
				sw := stackpkg.NewSafeWriter(prefixed)
				if err := stackpkg.FollowLog(ctx, logPath, sw, filter); err != nil {
					errs <- fmt.Errorf("[%s] %w", svc, err)
				}
			} else {
				if err := readAndPrint(ctx, logPath, opts.Lines, filter, prefixed); err != nil {
					errs <- fmt.Errorf("[%s] %w", svc, err)
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// readAndPrint 读 log path 整个文件,过滤后取最后 N 行输出。
//
// 不开 follow,一次性 read+filter+tail。
func readAndPrint(ctx context.Context, logPath string, n int, filter func(string) bool, w io.Writer) error {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return err
	}
	lines := bytes.Split(data, []byte("\n"))

	var out [][]byte
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		s := string(line)
		if filter != nil && !filter(s) {
			continue
		}
		out = append(out, line)
	}

	// 取最后 N 行
	if n > 0 && len(out) > n {
		out = out[len(out)-n:]
	}

	for _, line := range out {
		if _, err := w.Write(line); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
		// 检查 ctx cancel
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

// buildFilter 组合 grep + since 过滤器(都是 nil 则不过滤)。
func buildFilter(opts logOptions) (func(string) bool, error) {
	var grepRe *regexp.Regexp
	if opts.Grep != "" {
		re, err := regexp.Compile(opts.Grep)
		if err != nil {
			return nil, fmt.Errorf("invalid --grep regex %q: %w", opts.Grep, err)
		}
		grepRe = re
	}

	sinceCutoff := time.Time{}
	if opts.Since > 0 {
		sinceCutoff = time.Now().Add(-opts.Since)
	}

	if grepRe == nil && sinceCutoff.IsZero() {
		return nil, nil
	}
	return func(line string) bool {
		if grepRe != nil && !grepRe.MatchString(line) {
			return false
		}
		if !sinceCutoff.IsZero() {
			ts, ok := parseLogTimestamp(line)
			if !ok {
				return true // 无时间戳行不丢(向后兼容)
			}
			if ts.Before(sinceCutoff) {
				return false
			}
		}
		return true
	}, nil
}

// parseLogTimestamp 解析 wau 服务日志里的 ISO 8601 前缀(2026-08-23T22:18:05 ...)。
func parseLogTimestamp(line string) (time.Time, bool) {
	// 找第一个空格之前的 token
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			candidate := line[:i]
			// 尝试 RFC3339Nano
			if t, err := time.Parse(time.RFC3339Nano, candidate); err == nil {
				return t, true
			}
			if t, err := time.Parse("2006-01-02T15:04:05", candidate); err == nil {
				return t, true
			}
			return time.Time{}, false
		}
	}
	return time.Time{}, false
}

// prefixedWriter 给每行加 [service] 前缀(并可选着色)。
type prefixedWriter struct {
	w      io.Writer
	prefix string
	mu     sync.Mutex
}

func newPrefixedWriter(w io.Writer, prefix string) *prefixedWriter {
	return &prefixedWriter{w: w, prefix: prefix}
}

func (pw *prefixedWriter) Write(p []byte) (int, error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	// 处理 bufio.Scanner 行(line 已不带 \n)
	s := string(p)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	// 整行写,prefix 只加一次
	out := pw.prefix + s
	n, err := pw.w.Write([]byte(out))
	// 报告写入了 len(p),不是带 prefix 的长度(让 caller 觉得只写了自己的 byte)
	if err == nil && n >= len(p) {
		return len(p), nil
	}
	return n, err
}

// colorPrefix 构造服务名前缀(可选着色)。
func colorPrefix(service string, idx int, noColor bool) string {
	if noColor {
		return "[" + service + "] "
	}
	color := ansiColors[idx%len(ansiColors)]
	return color + "[" + service + "]" + ansiReset + " "
}

// expandHome 展开路径里的 ~ (跟 stack/types.go 里的 expandHome 行为一致)。
func expandHome(p string) string {
	if p == "" {
		return p
	}
	if p == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// 编译期保证 bufio.Scanner / bytes.Split 等被使用(cobra 之外)
var _ = bufio.NewScanner