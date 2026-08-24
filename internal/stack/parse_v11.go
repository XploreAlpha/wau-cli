// Package stack — parse_v11.go
//
// 4.1.1 (2026-08-24, v1.1.0 子项 4.1) — wau-stack.yml v1.1 YAML 解析 + 校验。
//
// 设计原则:
//   - D60 additive:不动 parse.go / types.go,新增 ParseV11 + ParseStackFile dispatcher
//   - 服务发现走 map[string]ServiceV11 key(而不是 slice + name field)
//   - depends_on circle detection 在 Parse 阶段就报告(避免 stack up 时才崩)
//   - image / placement.host 字段保留 schema,parse 时 warn 但不报错(v1.1.x 后续 wire)
package stack

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseV11 从 bytes 解析 stack v1.1 YAML。
func ParseV11(data []byte) (*StackV11, error) {
	var s StackV11
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse stack v1.1 YAML: %w", err)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if err := s.resolveDependsOn(); err != nil {
		return nil, err
	}
	if _, err := s.TopoOrder(); err != nil {
		return nil, err
	}
	return &s, nil
}

// ParseStackFile dispatcher — 按 YAML `version` 字段路由到 v1 或 v1.1。
//
// 返回 any 兼容两种 schema。call-site 需 type assert:
//
//	switch s := result.(type) {
//	case *stack.Stack:     ... // v1
//	case *stack.StackV11:  ... // v1.1
//	}
func ParseStackFile(data []byte) (any, error) {
	var probe struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("parse stack YAML version probe: %w", err)
	}
	switch probe.Version {
	case "", StackVersion:
		return Parse(data)
	case StackVersionV11:
		return ParseV11(data)
	default:
		return nil, fmt.Errorf("unsupported stack version %q (supported: %q, %q)",
			probe.Version, StackVersion, StackVersionV11)
	}
}

// Validate 校验 v1.1 stack 配置(不依赖外部 IO)。
func (s *StackV11) Validate() error {
	var errs []string
	if s.Version != StackVersionV11 {
		errs = append(errs, fmt.Sprintf("version %q != supported %q", s.Version, StackVersionV11))
	}
	if strings.TrimSpace(s.StackID) == "" {
		errs = append(errs, "stack_id is required")
	}
	for name, svc := range s.Services {
		switch svc.Kind {
		case "":
			// 默认 binary
		case KindBinary, KindExternal:
			// ok(KindDocker 预留 v1.1.x)
		case KindDocker:
			errs = append(errs, fmt.Sprintf("services[%s].kind=docker reserved for v1.1.x, use kind=binary", name))
		default:
			errs = append(errs, fmt.Sprintf("services[%s].kind %q unknown", name, svc.Kind))
		}
		if svc.Kind == KindBinary && svc.Binary == "" && len(svc.Command) == 0 {
			errs = append(errs, fmt.Sprintf("services[%s] kind=binary needs binary or command", name))
		}
		if svc.Image != "" {
			errs = append(errs, fmt.Sprintf("services[%s].image reserved for v1.1.x (parse-only, no effect)", name))
		}
		if svc.Placement != nil && svc.Placement.Host != "" {
			// 解析但不报错,留 schema 位置
			errs = append(errs, fmt.Sprintf("services[%s].placement.host=%q reserved for v1.1.x (v1.1 only supports local)", name, svc.Placement.Host))
		}
		if svc.Healthcheck != nil {
			if err := svc.Healthcheck.validate(); err != nil {
				errs = append(errs, fmt.Sprintf("services[%s].healthcheck: %v", name, err))
			}
		}
	}
	if len(errs) > 0 {
		return errors.New("stack v1.1 validation failed: " + strings.Join(errs, "; "))
	}
	return nil
}

// validate v1.1 健康探针:grpc/http/tcp/exec 四选一。
func (h *HealthcheckSpec) validate() error {
	n := 0
	if h.GRPC != "" {
		n++
	}
	if h.HTTP != "" {
		n++
	}
	if h.TCP != "" {
		n++
	}
	if h.Exec != "" {
		n++
	}
	if n == 0 {
		return errors.New("needs one of: grpc / http / tcp / exec")
	}
	if n > 1 {
		return errors.New("must specify exactly one of: grpc / http / tcp / exec")
	}
	return nil
}

// resolveDependsOn 把每个 service.DependsOn 校验为已知服务名。
func (s *StackV11) resolveDependsOn() error {
	for name, svc := range s.Services {
		for _, dep := range svc.DependsOn {
			if dep == name {
				return fmt.Errorf("service %q depends on itself", name)
			}
			if _, ok := s.Services[dep]; !ok {
				return fmt.Errorf("service %q depends on unknown service %q", name, dep)
			}
		}
	}
	return nil
}

// TopoOrder 返回按 depends_on 拓扑排序后的服务名列表(无环保证)。
//
// 算法:Kahn's algorithm,字母序确定性。环检测:Kahn 完成后 len(order) != n 即有环,报具体循环节点。
//
// 注:与 v1 的 (Stack)TopoOrder 是相同算法,map vs slice 差异导致代码不复用。
// 后续可抽 topoSortGeneric 公共 helper,4.1.6 closure 后再 refactor。
func (s *StackV11) TopoOrder() ([]string, error) {
	names := make([]string, 0, len(s.Services))
	for n := range s.Services {
		names = append(names, n)
	}
	sort.Strings(names)

	inDeg := make(map[string]int, len(names))
	adj := make(map[string][]string, len(names))
	for _, n := range names {
		inDeg[n] = 0
	}
	for name, svc := range s.Services {
		for _, dep := range svc.DependsOn {
			adj[dep] = append(adj[dep], name)
			inDeg[name]++
		}
	}

	// Kahn:字母序 deterministic queue
	var queue []string
	for _, n := range names {
		if inDeg[n] == 0 {
			queue = append(queue, n)
		}
	}
	sort.Strings(queue)

	order := make([]string, 0, len(names))
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		order = append(order, u)
		neighbors := append([]string(nil), adj[u]...)
		sort.Strings(neighbors)
		for _, v := range neighbors {
			inDeg[v]--
			if inDeg[v] == 0 {
				idx := sort.SearchStrings(queue, v)
				queue = append(queue, "")
				copy(queue[idx+1:], queue[idx:])
				queue[idx] = v
			}
		}
	}
	if len(order) != len(names) {
		var cycle []string
		for _, n := range names {
			if inDeg[n] > 0 {
				cycle = append(cycle, n)
			}
		}
		sort.Strings(cycle)
		return nil, fmt.Errorf("circular dependency detected involving: %v", cycle)
	}
	return order, nil
}

// ServiceByName 按名字查找 v1.1 service。
func (s *StackV11) ServiceByName(name string) (*ServiceV11, bool) {
	svc, ok := s.Services[name]
	if !ok {
		return nil, false
	}
	return &svc, true
}

// ApplyProfile 返回 profile 过滤后的服务名列表(topo 序,带 depends_on 闭包)。
//
// 先对全 stack 跑 TopoOrder(保证 deps 在被依赖项之前启动),再保留 wanted 闭包内的服务。
// 空 profileName 返回全部服务的 topo 序。
func (s *StackV11) ApplyProfile(profileName string) ([]string, error) {
	full, err := s.TopoOrder()
	if err != nil {
		return nil, err
	}
	if profileName == "" {
		return full, nil
	}
	p, ok := s.Profiles[profileName]
	if !ok {
		keys := make([]string, 0, len(s.Profiles))
		for k := range s.Profiles {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return nil, fmt.Errorf("profile %q not found (available: %v)", profileName, keys)
	}
	wanted := make(map[string]bool, len(p.Services))
	queue := make([]string, 0, len(p.Services))
	for _, n := range p.Services {
		if !wanted[n] {
			wanted[n] = true
			queue = append(queue, n)
		}
	}
	// depends_on 闭包(BFS 跟 transitive deps)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		svc, ok := s.Services[name]
		if !ok {
			continue
		}
		for _, dep := range svc.DependsOn {
			if !wanted[dep] {
				wanted[dep] = true
				queue = append(queue, dep)
			}
		}
	}
	out := make([]string, 0, len(wanted))
	for _, n := range full {
		if wanted[n] {
			out = append(out, n)
		}
	}
	return out, nil
}