// Package initconfigs manages embedded service config templates.
//
// P4.2 (v1.0.1, 2026-08-24) — 提供:
//   - 4 个服务的最小可启动 config 模板(embed 到 binary,无需运行时依赖)
//   - Writer:写到用户指定目录(~/.wau/configs 默认),支持 --force / --dry-run
//   - 4 个服务:wau-store / wau-llm-router / wau-edge / wau-channel
//
// 设计原则:
//   - 4 个 yaml 从各服务的 source 仓 configs/ 同步(手写,见文件头注释)
//   - 不修改服务 source(D60 additive)
//   - opt-in:用户显式跑 `wau stack init-configs`,不自动跑
//   - idempotent:文件已存在默认 skip,--force 覆盖
package initconfigs

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed configs/*.yaml
var embeddedConfigs embed.FS

// Service config 模板元数据。
type Template struct {
	Service  string // "wau-store"
	Filename string // "store.yaml"
	Path     string // embed FS 里的路径:configs/store.yaml
	Contents []byte // 模板内容
}

// ListTemplates 列出所有 embed 的模板(按 Service 名排序)。
func ListTemplates() ([]Template, error) {
	entries, err := embedConfigs()
	if err != nil {
		return nil, err
	}
	out := make([]Template, 0, len(entries))
	for _, e := range entries {
		out = append(out, e)
	}
	return out, nil
}

// TemplateByService 按 service 名查单个模板(不区分大小写)。
//
// service 名映射:
//   - "wau-store"          → store.yaml
//   - "wau-llm-router"     → router.yaml
//   - "wau-edge"           → edge.yaml
//   - "wau-channel"        → channel.yaml
func TemplateByService(service string) (Template, error) {
	all, err := ListTemplates()
	if err != nil {
		return Template{}, err
	}
	s := normalizeService(service)
	for _, t := range all {
		if t.Service == s {
			return t, nil
		}
	}
	return Template{}, fmt.Errorf("no template for service %q (available: %s)",
		service, availableServices(all))
}

// normalizeService 把用户输入归一化成 canonical 名。
//
// 接受:"wau-store" / "store" / "WAU-STORE"  → "wau-store"
// 例外 alias:"router" / "llm-router" → "wau-llm-router"(因 source 文件叫 router.yaml)
func normalizeService(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// 特殊 alias(无 wau- 前缀也直接归一)
	switch s {
	case "router", "llm-router":
		return "wau-llm-router"
	}
	if !strings.HasPrefix(s, "wau-") {
		s = "wau-" + s
	}
	return s
}

func availableServices(ts []Template) string {
	names := make([]string, len(ts))
	for i, t := range ts {
		names[i] = t.Service
	}
	return strings.Join(names, ", ")
}

// embedConfigs 从 embed FS 读所有 .yaml + 按 service 名映射。
func embedConfigs() ([]Template, error) {
	files, err := fs.ReadDir(embeddedConfigs, "configs")
	if err != nil {
		return nil, fmt.Errorf("read embed configs dir: %w", err)
	}
	out := make([]Template, 0, len(files))
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".yaml") {
			continue
		}
		filename := f.Name()                  // "store.yaml"
		stem := strings.TrimSuffix(filename, ".yaml") // "store"
		service := "wau-" + stem              // "wau-store"
		// 例外:wau-llm-router 的源文件叫 router.yaml → "wau-router"
		// 但 canonical service 名是 "wau-llm-router",我们需要映射
		service = remapServiceName(service)

		contents, err := embeddedConfigs.ReadFile("configs/" + filename)
		if err != nil {
			return nil, fmt.Errorf("read embed %s: %w", filename, err)
		}
		out = append(out, Template{
			Service:  service,
			Filename: filename,
			Path:     "configs/" + filename,
			Contents: contents,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("no embedded yaml configs found")
	}
	return out, nil
}

// remapServiceName 把从 filename 推导出的 service 名映射到 canonical 名。
func remapServiceName(s string) string {
	switch s {
	case "wau-router":
		return "wau-llm-router"
	default:
		return s
	}
}

// Writer 把 templates 写到 OutputDir 的 writer。
type Writer struct {
	OutputDir string // 目标目录(~/.wau/configs 或自定义)
	Force     bool   // 覆盖已有文件
	DryRun    bool   // 只 print,不实际写
}

// WriteResult 单个 template 的写入结果。
type WriteResult struct {
	Service  string
	Filepath string
	Status   string // "wrote" / "skipped" / "would-write" / "error"
	Size     int    // bytes written (or 0 if skipped/error)
	Err      error
}

// WriteAll 批量写多个 templates。返回结果列表(按输入顺序)。
func (w *Writer) WriteAll(templates []Template) []WriteResult {
	out := make([]WriteResult, 0, len(templates))
	for _, t := range templates {
		out = append(out, w.Write(t))
	}
	return out
}

// Write 单个 template 的写入逻辑。
//
// 流程:
//   - DryRun → 直接返回 "would-write"
//   - 文件存在 + !Force → "skipped"
//   - 否则 mkdir -p OutputDir + atomic write(tmp + rename)
func (w *Writer) Write(t Template) WriteResult {
	dir := ExpandHome(w.OutputDir)
	filepath := filepath.Join(dir, t.Filename)
	res := WriteResult{
		Service:  t.Service,
		Filepath: filepath,
	}

	if w.DryRun {
		res.Status = "would-write"
		res.Size = len(t.Contents)
		return res
	}

	// 检查文件是否存在
	if _, err := os.Stat(filepath); err == nil {
		if !w.Force {
			res.Status = "skipped"
			return res
		}
	} else if !os.IsNotExist(err) {
		res.Status = "error"
		res.Err = fmt.Errorf("stat %s: %w", filepath, err)
		return res
	}

	// mkdir -p
	if err := os.MkdirAll(dir, 0o755); err != nil {
		res.Status = "error"
		res.Err = fmt.Errorf("mkdir %s: %w", dir, err)
		return res
	}

	// atomic write:写 .tmp 再 rename
	tmpPath := filepath + ".tmp"
	if err := os.WriteFile(tmpPath, t.Contents, 0o644); err != nil {
		res.Status = "error"
		res.Err = fmt.Errorf("write tmp %s: %w", tmpPath, err)
		return res
	}
	if err := os.Rename(tmpPath, filepath); err != nil {
		res.Status = "error"
		res.Err = fmt.Errorf("rename %s → %s: %w", tmpPath, filepath, err)
		return res
	}

	res.Status = "wrote"
	res.Size = len(t.Contents)
	return res
}

// ExpandHome 展开 ~ 到 user home dir(跟 internal/cmd/stack/log.go 一致)。
func ExpandHome(p string) string {
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