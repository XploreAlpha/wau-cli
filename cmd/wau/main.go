// Package main is the entry point for wau-cli.
//
// wau-cli is the official command-line client for WAU-core-kernel.
// It provides a kubectl/docker-like experience for managing WAU services.
//
// Usage:
//
//	wau [command]
//
// Available commands:
//
//	agent       Manage agents
//	config      Manage wau-cli configuration
//	health      Check kernel health
//	kernel      Show kernel information
//	task        Manage tasks
//	version     Show wau-cli version
//
// Use "wau [command] --help" for more information about a command.
package main

import (
	"fmt"
	"os"

	"github.com/wau/wau-cli/internal/cmd"
)

// exitCoder 是支持自定义 exit code 的 error 接口(per [[feedback-cli-cant-push-git]]
// 节奏不动 main.go 的核心路径,只是加个 interface 探测)。
type exitCoder interface {
	ExitCode() int
}

// silentError 标识"不需要 main 重复打印"的 error(由子命令自己已打 summary)。
type silentError interface {
	SilenceError() bool
}

func main() {
	err := cmd.Execute()
	if err == nil {
		return
	}
	// 1. silent error → 不打印(子命令已自打 summary),只用 ExitCode
	if s, ok := err.(silentError); ok && s.SilenceError() {
		if e, ok := err.(exitCoder); ok {
			os.Exit(e.ExitCode())
		}
		os.Exit(1)
	}
	// 2. 普通 error → 打印 + exit 1
	fmt.Fprintln(os.Stderr, "Error:", err)
	if e, ok := err.(exitCoder); ok {
		os.Exit(e.ExitCode())
	}
	os.Exit(1)
}
