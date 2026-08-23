// Package output - progress.go
//
// 第一刀 1.1 — progress bar / spinner 工具,封装 schollz/progressbar/v3。
//
// 设计原则:
//   - D60 additive:不动现有 format.go
//   - 单实例:不在多个 goroutine 共享同一 bar(类 docker compose up 的行为)
//   - 颜色 + emoji:沿用 format.go Success/Info/Warn/Error 的 ✓ ℹ ⚠ ✗ 风格
package output

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/schollz/progressbar/v3"
)

// ProgressOption 配置 progress bar 行为。
type ProgressOption struct {
	Description string        // 描述文字(前缀)
	Total       int           // 总数,-1 = 未知(显示 spinner)
	Width       int           // bar 宽度(默认 40)
	Color       string        // 颜色名("red"/"green"/"cyan" 等)
	Writer      io.Writer     // 输出目标,默认 os.Stderr
	Throttle    time.Duration // 更新节流
}

// Progress 创建一个 progress bar。
//
//	total >= 0:进度条模式,每 Add(1) 一格
//	total <  0:spinner 模式,只显示描述 + 旋转字符
func Progress(opt ProgressOption) *progressbar.ProgressBar {
	if opt.Width == 0 {
		opt.Width = 40
	}
	if opt.Writer == nil {
		opt.Writer = os.Stderr
	}
	if opt.Throttle == 0 {
		opt.Throttle = 100 * time.Millisecond
	}
	opts := []progressbar.Option{
		progressbar.OptionSetWriter(opt.Writer),
		progressbar.OptionSetWidth(opt.Width),
		progressbar.OptionThrottle(opt.Throttle),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprintln(opt.Writer)
		}),
	}
	if opt.Color != "" {
		opts = append(opts, progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerHead:    "█",
			SaucerPadding: "░",
			BarStart:      "[",
			BarEnd:        "]",
		}))
	}
	if opt.Total < 0 {
		// spinner 模式:未知 total
		opts = append(opts, progressbar.OptionSetItsString(""))
		return progressbar.NewOptions(-1, opts...)
	}
	return progressbar.NewOptions(opt.Total, opts...)
}

// Step 输出一行状态更新(不换行,适合 "wait → done")。
func Step(desc string) {
	fmt.Fprintf(os.Stderr, "  %s ... ", desc)
}

// StepDone 完成 Step 输出,带 ✓。
func StepDone() {
	fmt.Fprintln(os.Stderr, "✓")
}

// StepFail 完成 Step 输出,带 ✗。
func StepFail(err error) {
	fmt.Fprintf(os.Stderr, "✗ (%v)\n", err)
}

// ServiceRow 输出服务状态行(用于 wau stack ls 的彩色表格)。
//
// 	status: "running" / "stopped" / "failed" / "starting"
//	takeMs: 启动耗时(仅 status=starting 时显示)
func ServiceRow(name string, httpPort, grpcPort, pid int, status string, uptime time.Duration) string {
	var icon string
	switch status {
	case "running":
		icon = "✓"
	case "starting":
		icon = "⠋"
	case "stopped":
		icon = "○"
	case "failed":
		icon = "✗"
	default:
		icon = "?"
	}
	uptimeStr := "-"
	if status == "running" || status == "starting" {
		uptimeStr = uptime.Truncate(time.Second).String()
	}
	return fmt.Sprintf("%s %-15s  http:%-5d  grpc:%-5d  pid:%-6d  %-9s  up %s",
		icon, name, httpPort, grpcPort, pid, status, uptimeStr)
}
