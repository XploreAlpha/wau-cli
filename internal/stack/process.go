// Package stack - process.go
//
// 第一刀 1.1 — 本地 process 启停管理 + 二进制路径解析。
//
// 设计原则:
//   - Hybrid 路径解析(per 2026-08-20 visa demo 拍板):
//     1. ~/.wau/bin/<binary>(如存在)
//     2. $GOBIN/<binary>(如设置)
//     3. which <binary>($PATH)
//     4. 失败 → 报清晰错误(带 go install hint)
//   - 启动:os/exec,日志重定向到 ~/.wau/log/<stack>/<service>.log
//   - 停止:SIGTERM,5s 不响应升级 SIGKILL
//   - PID 文件:~/.wau/run/<stack>/<service>.pid
//   - State 文件:~/.wau/run/<stack>.json(整个 stack 的服务状态)
package stack

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// BinaryLookup 二进制路径解析策略。
type BinaryLookup struct {
	Home      string // ~/.wau/bin
	GOBIN     string // $GOBIN
	ExtraDirs []string
}

// DefaultLookup 默认查找策略。
func DefaultLookup() BinaryLookup {
	home, _ := os.UserHomeDir()
	gobin := os.Getenv("GOBIN")
	var extra []string
	if path := os.Getenv("PATH"); path != "" {
		extra = strings.Split(path, string(os.PathListSeparator))
	}
	return BinaryLookup{
		Home:      filepath.Join(home, ".wau", "bin"),
		GOBIN:     gobin,
		ExtraDirs: extra,
	}
}

// Resolve 解析 binary 路径。
//
// 顺序:~/.wau/bin → $GOBIN → $PATH(逐个目录)。
// 找到可执行文件返回绝对路径,找不到返回带 hint 的清晰错误。
func (b BinaryLookup) Resolve(name string) (string, error) {
	candidates := []string{}
	if b.Home != "" {
		candidates = append(candidates, filepath.Join(b.Home, name))
	}
	if b.GOBIN != "" {
		candidates = append(candidates, filepath.Join(b.GOBIN, name))
	}
	for _, d := range b.ExtraDirs {
		candidates = append(candidates, filepath.Join(d, name))
	}
	for _, p := range candidates {
		if isExecutable(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("binary %q not found (looked in %d paths); try: go install ./cmd/%s",
		name, len(candidates), name)
}

// isExecutable 检查路径是否存在且可执行。
func isExecutable(p string) bool {
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	mode := info.Mode()
	return mode&0o111 != 0
}

// ServiceState 单个服务的运行时状态(persisted to JSON)。
type ServiceState struct {
	Name      string    `json:"name"`
	Binary    string    `json:"binary,omitempty"`
	BinaryPath string   `json:"binary_path,omitempty"`
	PID       int       `json:"pid,omitempty"`
	Status    string    `json:"status"` // running | stopped | failed | starting
	StartedAt time.Time `json:"started_at,omitempty"`
	StoppedAt time.Time `json:"stopped_at,omitempty"`
	LogFile   string    `json:"log_file,omitempty"`
	LastError string    `json:"last_error,omitempty"`
	HTTPPort  int       `json:"http_port,omitempty"`
	GRPCPort  int       `json:"grpc_port,omitempty"`
	Instances int       `json:"instances,omitempty"`
}

// Runtime stack 运行时状态(整个 stack 的聚合)。
type Runtime struct {
	Name      string                  `json:"name"`
	DataDir   string                  `json:"data_dir"`
	LogDir    string                  `json:"log_dir"`
	Services  map[string]*ServiceState `json:"services"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// LoadRuntime 从 dataDir/<name>.json 加载 runtime 状态;文件不存在返回空 Runtime(非错误)。
func LoadRuntime(dataDir, name string) (*Runtime, error) {
	path := filepath.Join(dataDir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Runtime{
				Name:     name,
				DataDir:  dataDir,
				Services: make(map[string]*ServiceState),
			}, nil
		}
		return nil, fmt.Errorf("read runtime file %s: %w", path, err)
	}
	var rt Runtime
	if err := json.Unmarshal(data, &rt); err != nil {
		return nil, fmt.Errorf("parse runtime file %s: %w", path, err)
	}
	if rt.Services == nil {
		rt.Services = make(map[string]*ServiceState)
	}
	return &rt, nil
}

// Save 持久化 runtime 状态。
func (r *Runtime) Save() error {
	r.UpdatedAt = time.Now()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = r.UpdatedAt
	}
	if err := os.MkdirAll(r.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data_dir %s: %w", r.DataDir, err)
	}
	path := filepath.Join(r.DataDir, r.Name+".json")
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal runtime: %w", err)
	}
	// 原子写:先写 .tmp 再 rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write runtime tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename runtime: %w", err)
	}
	return nil
}

// SetStatus 设置服务状态并 Save。
//
// extra 可携带 metadata(binary / binaryPath / logFile / httpPort / grpcPort),
// 用于 service 首次启动时持久化端口和 binary 信息;后续调用可传 nil。
func (r *Runtime) SetStatus(name, status string, pid int, extra map[string]interface{}) error {
	s, ok := r.Services[name]
	if !ok {
		s = &ServiceState{Name: name}
		r.Services[name] = s
	}
	// 应用 extra(binary / ports / log file)
	if extra != nil {
		if v, ok := extra["binary"].(string); ok {
			s.Binary = v
		}
		if v, ok := extra["binaryPath"].(string); ok {
			s.BinaryPath = v
		}
		if v, ok := extra["logFile"].(string); ok {
			s.LogFile = v
		}
		if v, ok := extra["httpPort"].(int); ok {
			s.HTTPPort = v
		}
		if v, ok := extra["grpcPort"].(int); ok {
			s.GRPCPort = v
		}
	}
	s.Status = status
	if pid > 0 {
		s.PID = pid
	}
	switch status {
	case "running":
		s.StartedAt = time.Now()
		s.LastError = ""
	case "stopped", "failed":
		s.StoppedAt = time.Now()
	}
	return r.Save()
}

// Remove 从 runtime 移除一个服务。
func (r *Runtime) Remove(name string) error {
	delete(r.Services, name)
	return r.Save()
}

// ProcessManager 启停 services 的 manager。
type ProcessManager struct {
	Lookup BinaryLookup
}

// NewProcessManager 默认 lookup。
func NewProcessManager() *ProcessManager {
	return &ProcessManager{Lookup: DefaultLookup()}
}

// Start 启动单个 service。
//
// 流程:
//   1. resolve binary path
//   2. open log file(~/.wau/log/<stack>/<service>.log, append mode)
//   3. fork process (setsid 脱离父进程)
//   4. 写 PID 文件
//   5. 返回 PID(不等健康)
func (pm *ProcessManager) Start(ctx context.Context, svc *Service, logDir string) (int, error) {
	if svc.Kind != KindBinary {
		return 0, fmt.Errorf("service %q kind=%s not supported by local process manager", svc.Name, svc.Kind)
	}
	binPath, err := pm.Lookup.Resolve(svc.Binary)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return 0, fmt.Errorf("create log_dir %s: %w", logDir, err)
	}
	logPath := filepath.Join(logDir, svc.Name+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open log file %s: %w", logPath, err)
	}
	// header:启动分隔线
	fmt.Fprintf(logFile, "\n──── %s started at %s ────\n",
		svc.Name, time.Now().Format(time.RFC3339))

	cmd := exec.CommandContext(ctx, binPath, svc.Args...)
	cmd.Env = os.Environ()
	for k, v := range svc.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // 独立 session,父进程退出不影响
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return 0, fmt.Errorf("start %s: %w", svc.Name, err)
	}
	// logFile 不关:子进程还引用它,关闭会触发 eof
	// 但需要保留引用防止 GC
	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()
	return cmd.Process.Pid, nil
}

// Stop 停止单个 service(按 PID)。
//
// 流程:
//   1. 检查 PID 是否存活(es signal 0)
//   2. SIGTERM,等 5s
//   3. 还活着 → SIGKILL
func (pm *ProcessManager) Stop(ctx context.Context, pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	// 先 SIGTERM
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return fmt.Errorf("SIGTERM pid %d: %w", pid, err)
	}
	// 等最多 5s
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			if errors.Is(err, os.ErrProcessDone) {
				return nil
			}
			// 其他错误也认为已退出
			return nil
		}
	}
	// 升级 SIGKILL
	if err := proc.Signal(syscall.SIGKILL); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("SIGKILL pid %d: %w", pid, err)
	}
	return nil
}

// KillProcessGroup 杀整个进程组(防止子进程遗留)。
func KillProcessGroup(pid int) error {
	if pid <= 0 {
		return nil
	}
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		// 进程已退,忽略
		return nil
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

// IsAlive 检查 PID 是否还活着。
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// PidFilePath 返回 PID 文件路径。
func PidFilePath(dataDir, serviceName string) string {
	return filepath.Join(dataDir, serviceName+".pid")
}

// ReadPIDFile 读 PID 文件。
func ReadPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid file %s: %w", path, err)
	}
	return pid, nil
}

// WritePIDFile 写 PID 文件。
func WritePIDFile(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}

// TailLog 返回 service log 文件最后 n 行(用于 wau service logs)。
func TailLog(logPath string, n int) (string, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// 简单实现:读全部,返回最后 n 行(对 visa demo 足够;后续可换 ring buffer)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n") + "\n", nil
}
