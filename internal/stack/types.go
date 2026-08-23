// Package stack provides local WAU stack orchestration: parse wau-stack.yml,
// start/stop services, run health probes, and persist runtime state.
//
// 第一刀 1.1 — wau stack up/down/status(per visa demo + 子项 4.1,2026-08-20)。
//
// 设计原则:
//   - D60 additive:不动现有 client/manifest/output 任何文件
//   - Cobra pattern:沿用 internal/cmd/agent/agent.go 的 NewCmd() 工厂
//   - Output pattern:沿用 output.Print(table/json/yaml/csv)
//   - State persistence:~/.wau/run/<stack>.json + ~/.wau/log/<stack>/*.log
package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StackVersion 当前 schema 版本号。
const StackVersion = "1"

// ProbeType 健康探针类型。
type ProbeType string

const (
	ProbeTCP   ProbeType = "tcp"   // TCP 端口可连
	ProbeHTTP  ProbeType = "http"  // HTTP GET 2xx/3xx
	ProbeExec  ProbeType = "exec"  // exec 命令 exit 0
)

// ServiceKind 服务来源类型。
type ServiceKind string

const (
	KindBinary   ServiceKind = "binary"   // 本地 binary(go install 后的产物)
	KindDocker   ServiceKind = "docker"   // docker run(预留,v1.1 后续)
	KindExternal ServiceKind = "external" // 已存在的服务(redis/mysql 等)
)

// Probe 健康探针定义。
type Probe struct {
	Type     ProbeType      `yaml:"type" json:"type"`
	Addr     string         `yaml:"addr,omitempty" json:"addr,omitempty"`         // tcp
	Port     int            `yaml:"port,omitempty" json:"port,omitempty"`         // tcp
	URL      string         `yaml:"url,omitempty" json:"url,omitempty"`           // http
	Cmd      string         `yaml:"cmd,omitempty" json:"cmd,omitempty"`           // exec
	Args     []string       `yaml:"args,omitempty" json:"args,omitempty"`         // exec
	Interval time.Duration  `yaml:"interval,omitempty" json:"interval,omitempty"` // 轮询间隔
	Timeout  time.Duration  `yaml:"timeout,omitempty" json:"timeout,omitempty"`   // 总超时
}

// Service 单个服务定义。
type Service struct {
	Name         string            `yaml:"name" json:"name"`
	Kind         ServiceKind       `yaml:"kind" json:"kind"`
	Binary       string            `yaml:"binary,omitempty" json:"binary,omitempty"`
	Args         []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env          map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	HTTPPort     int               `yaml:"http_port,omitempty" json:"http_port,omitempty"`
	GRPCPort     int               `yaml:"grpc_port,omitempty" json:"grpc_port,omitempty"`
	WebhookPort  int               `yaml:"webhook_port,omitempty" json:"webhook_port,omitempty"`
	DependsOn    []string          `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Required     bool              `yaml:"required,omitempty" json:"required,omitempty"` // 起不来是否 abort up
	Health       *Probe            `yaml:"health,omitempty" json:"health,omitempty"`
	Instances    int               `yaml:"instances,omitempty" json:"instances,omitempty"` // 多实例(预留,默认 1)
}

// Stack 完整 stack 配置。
type Stack struct {
	Version    string             `yaml:"version" json:"version"`
	Stack      StackMeta          `yaml:"stack" json:"stack"`
	Services   []Service          `yaml:"services" json:"services"`
	Profiles   map[string]Profile `yaml:"profiles,omitempty" json:"profiles,omitempty"`
}

// StackMeta stack 元信息。
type StackMeta struct {
	Name    string `yaml:"name" json:"name"`
	DataDir string `yaml:"data_dir,omitempty" json:"data_dir,omitempty"`
	LogDir  string `yaml:"log_dir,omitempty" json:"log_dir,omitempty"`
}

// Profile 子集 profile(用于 wau up --demo 等)。
type Profile struct {
	Services []string `yaml:"services" json:"services"`
}

// DefaultDataDir 默认数据目录(~/.wau/run)。
func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/wau/run"
	}
	return filepath.Join(home, ".wau", "run")
}

// DefaultLogDir 默认日志目录(~/.wau/log)。
func DefaultLogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/wau/log"
	}
	return filepath.Join(home, ".wau", "log")
}

// ResolvedDirs 解析 stack.DataDir / LogDir 里的 ~。
func (s *Stack) ResolvedDirs() (dataDir, logDir string, err error) {
	dataDir = expandHome(s.Stack.DataDir)
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}
	logDir = expandHome(s.Stack.LogDir)
	if logDir == "" {
		logDir = DefaultLogDir()
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create data_dir %s: %w", dataDir, err)
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create log_dir %s: %w", logDir, err)
	}
	return dataDir, logDir, nil
}

// expandHome 展开 ~ 为 home dir。
func expandHome(p string) string {
	if p == "" {
		return ""
	}
	if len(p) >= 2 && p[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return home
	}
	return p
}
