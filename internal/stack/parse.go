// Package stack - parse.go
//
// 第一刀 1.1 — wau-stack.yml YAML 解析 + 校验 + 依赖图拓扑排序。
//
// 设计原则:
//   - YAML schema 跟 pkg/stackfile/default.go 内置 default 保持 byte-equal
//   - 解析失败要给清晰错误(列具体字段 + 行号)
//   - depends_on circle detection 必须在拓扑排序前做
package stack

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFile 从 path 加载 stack YAML。
func LoadFile(path string) (*Stack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read stack file %s: %w", path, err)
	}
	return Parse(data)
}

// Parse 从 bytes 解析 stack YAML。
func Parse(data []byte) (*Stack, error) {
	var s Stack
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse stack YAML: %w", err)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if err := s.resolveDependsOn(); err != nil {
		return nil, err
	}
	// fail-fast:cycle detection 在 Parse 阶段就报告(避免 stack up 时才崩)
	if _, err := s.TopoOrder(); err != nil {
		return nil, err
	}
	return &s, nil
}

// Validate 校验 stack 配置合法性(不依赖外部 IO)。
func (s *Stack) Validate() error {
	var errs []string
	if s.Version != StackVersion {
		errs = append(errs, fmt.Sprintf("version %q != supported %q", s.Version, StackVersion))
	}
	if strings.TrimSpace(s.Stack.Name) == "" {
		errs = append(errs, "stack.name is required")
	}
	names := make(map[string]bool, len(s.Services))
	for i, svc := range s.Services {
		if svc.Name == "" {
			errs = append(errs, fmt.Sprintf("services[%d].name is empty", i))
			continue
		}
		if names[svc.Name] {
			errs = append(errs, fmt.Sprintf("duplicate service name %q", svc.Name))
		}
		names[svc.Name] = true
		switch svc.Kind {
		case "":
			// 默认 binary
		case KindBinary, KindDocker, KindExternal:
			// ok
		default:
			errs = append(errs, fmt.Sprintf("services[%s].kind %q unknown", svc.Name, svc.Kind))
		}
		if svc.Kind == KindBinary && svc.Binary == "" {
			errs = append(errs, fmt.Sprintf("services[%s].binary is required for kind=binary", svc.Name))
		}
		if svc.Kind == KindExternal && svc.Health == nil {
			errs = append(errs, fmt.Sprintf("services[%s].health is required for kind=external", svc.Name))
		}
		if svc.Health != nil {
			if err := svc.Health.validate(); err != nil {
				errs = append(errs, fmt.Sprintf("services[%s].health: %v", svc.Name, err))
			}
		}
	}
	if len(errs) > 0 {
		return errors.New("stack validation failed: " + strings.Join(errs, "; "))
	}
	return nil
}

// validate 校验健康探针定义。
func (p *Probe) validate() error {
	switch p.Type {
	case ProbeTCP:
		if p.Port == 0 && p.Addr == "" {
			return errors.New("tcp probe needs port or addr")
		}
	case ProbeHTTP:
		if p.URL == "" {
			return errors.New("http probe needs url")
		}
	case ProbeExec:
		if p.Cmd == "" {
			return errors.New("exec probe needs cmd")
		}
	default:
		return fmt.Errorf("probe type %q unknown", p.Type)
	}
	return nil
}

// resolveDependsOn 把每个 service.DependsOn 校验为已知服务名,不存在则报错。
func (s *Stack) resolveDependsOn() error {
	known := make(map[string]bool, len(s.Services))
	for _, svc := range s.Services {
		known[svc.Name] = true
	}
	for _, svc := range s.Services {
		for _, dep := range svc.DependsOn {
			if !known[dep] {
				return fmt.Errorf("service %q depends on unknown service %q", svc.Name, dep)
			}
			if dep == svc.Name {
				return fmt.Errorf("service %q depends on itself", svc.Name)
			}
		}
	}
	return nil
}

// TopoOrder 返回按 depends_on 拓扑排序后的 service 名字列表(无环保证)。
//
// 算法:经典 Kahn's algorithm。环检测:Kahn 完成后 len(order) != n 即有环,报具体循环路径。
func (s *Stack) TopoOrder() ([]string, error) {
	n := len(s.Services)
	inDeg := make(map[string]int, n)
	adj := make(map[string][]string, n)
	names := make([]string, 0, n)
	for _, svc := range s.Services {
		names = append(names, svc.Name)
		if _, ok := inDeg[svc.Name]; !ok {
			inDeg[svc.Name] = 0
		}
	}
	sort.Strings(names) // 保证 determinism
	for _, svc := range s.Services {
		for _, dep := range svc.DependsOn {
			adj[dep] = append(adj[dep], svc.Name)
			inDeg[svc.Name]++
		}
	}

	// Kahn:用 sorted queue 保证 determinism
	var queue []string
	for _, name := range names {
		if inDeg[name] == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	order := make([]string, 0, n)
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		order = append(order, u)
		// 按字母序处理 neighbors
		neighbors := append([]string(nil), adj[u]...)
		sort.Strings(neighbors)
		for _, v := range neighbors {
			inDeg[v]--
			if inDeg[v] == 0 {
				// 二分插入保持 queue 字母序
				idx := sort.SearchStrings(queue, v)
				queue = append(queue, "")
				copy(queue[idx+1:], queue[idx:])
				queue[idx] = v
			}
		}
	}
	if len(order) != n {
		// 找剩余节点的环路径
		var cycle []string
		for _, name := range names {
			if inDeg[name] > 0 {
				cycle = append(cycle, name)
			}
		}
		sort.Strings(cycle)
		return nil, fmt.Errorf("circular dependency detected involving: %v", cycle)
	}
	return order, nil
}

// ServiceByName 按名字查找 service。
func (s *Stack) ServiceByName(name string) (*Service, bool) {
	for i := range s.Services {
		if s.Services[i].Name == name {
			return &s.Services[i], true
		}
	}
	return nil, false
}

// ApplyProfile 返回 profile 过滤后的 service 子集(保持原顺序)。
func (s *Stack) ApplyProfile(profileName string) ([]Service, error) {
	if profileName == "" {
		return s.Services, nil
	}
	p, ok := s.Profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("profile %q not found (available: %v)", profileName, profileKeys(s.Profiles))
	}
	wanted := make(map[string]bool, len(p.Services))
	for _, n := range p.Services {
		wanted[n] = true
	}
	// 补 depends_on 闭包:即使 profile 没显式列,depends_on 也得带上
	for _, svc := range s.Services {
		if wanted[svc.Name] {
			for _, dep := range svc.DependsOn {
				wanted[dep] = true
			}
		}
	}
	var out []Service
	for _, svc := range s.Services {
		if wanted[svc.Name] {
			out = append(out, svc)
		}
	}
	return out, nil
}

func profileKeys(m map[string]Profile) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
