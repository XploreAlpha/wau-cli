// Package stack — validate.go
//
// 4.1.3 (2026-08-24, v1.1.0 子项 4.1) — wau-stack.yml v1.1 深度校验层。
//
// ParseV11 已经做了 schema / topo / depends_on 校验。ValidateV11 在此基础上做
// runtime-level 检查(binary 存在性 + port 冲突),输出结构化 ValidationReport
// 给 cmd 层用(table / json)。
//
// D60 additive:不动 ParseV11 / Validate;ValidateV11 是独立新函数,接受
// ParseV11 已经成功的 *StackV11 或失败时的 raw bytes(报 parse error 给 user)。
package stack

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ValidationLevel 校验深度(渐进披露)。
type ValidationLevel string

const (
	// ValidationBasic — schema + topo + depends_on(ParseV11 已经做的)。
	ValidationBasic ValidationLevel = "basic"

	// ValidationRuntime — basic + binary 存在性 + port 冲突(默认)。
	ValidationRuntime ValidationLevel = "runtime"
)

// ValidationSeverity issue 严重程度。
type ValidationSeverity string

const (
	SeverityError   ValidationSeverity = "error"
	SeverityWarning ValidationSeverity = "warning"
	SeverityInfo    ValidationSeverity = "info"
)

// ValidationIssue 单条 issue。
type ValidationIssue struct {
	Severity ValidationSeverity `json:"severity"`
	Service  string             `json:"service,omitempty"`
	Field    string             `json:"field,omitempty"`
	Message  string             `json:"message"`
}

// ValidationService 单服务的校验状态摘要。
type ValidationService struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind,omitempty"`
	Binary       string   `json:"binary,omitempty"`
	BinaryExists bool     `json:"binary_exists"`
	PortCount    int      `json:"port_count"`
	HasHealthChk bool     `json:"has_healthcheck"`
}

// ValidationReport 校验报告(cmd 层 table / json 输出)。
type ValidationReport struct {
	StackID   string              `json:"stack_id"`
	Release   string              `json:"release,omitempty"`
	Level     ValidationLevel     `json:"level"`
	FetchedAt time.Time           `json:"fetched_at"`
	Issues    []ValidationIssue   `json:"issues"`
	Errors    int                 `json:"errors"`
	Warnings  int                 `json:"warnings"`
	Infos     int                 `json:"infos"`
	Services  []ValidationService `json:"services,omitempty"`
	Healthy   bool                `json:"healthy"`
}

// ValidateV11 校验 v1.1 stack YAML,返回结构化报告。
//
// 行为:
//   - 解析失败:返回 report (1 error) + nil(让 cmd 层用 exit code 1 退出,不 panic)
//   - 解析成功:按 level 跑深度校验
//   - ValidationBasic:ParseV11 已保证 schema/topo,这里只 set StackID/Release
//   - ValidationRuntime:+ binary 存在性(binary kind)+ port 冲突检测
func ValidateV11(data []byte, level ValidationLevel) (*ValidationReport, error) {
	report := &ValidationReport{
		Level:     level,
		FetchedAt: time.Now(),
	}
	parsed, err := ParseV11(data)
	if err != nil {
		report.addIssue(SeverityError, "", "parse", fmt.Sprintf("parse failed: %v", err))
		return report, nil
	}
	report.StackID = parsed.StackID
	report.Release = parsed.Release

	if level == ValidationBasic {
		// ParseV11 已经做了 — 但 health = true 表示没有 issues。
		report.Healthy = true
		return report, nil
	}

	// Runtime level:binary + port check
	lookup := DefaultLookup()
	seenPorts := make(map[string]string) // port spec -> service name

	// 字母序遍历保证 deterministic
	names := make([]string, 0, len(parsed.Services))
	for n := range parsed.Services {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		svc := parsed.Services[name]
		vs := ValidationService{
			Name:         name,
			Kind:         string(svc.Kind),
			Binary:       svc.Binary,
			PortCount:    len(svc.Ports),
			HasHealthChk: svc.Healthcheck != nil,
		}

		// binary existence(kind=binary 才查)
		if svc.Kind == "" || svc.Kind == KindBinary {
			binaryName := svc.Binary
			if binaryName == "" && len(svc.Command) > 0 {
				binaryName = svc.Command[0]
			}
			if binaryName != "" {
				if _, lookupErr := lookup.Resolve(binaryName); lookupErr != nil {
					report.addIssue(SeverityWarning, name, "binary",
						fmt.Sprintf("binary %q not found on host (will fail at up): %v",
							binaryName, lookupErr))
				} else {
					vs.BinaryExists = true
				}
			}
		}

		// port 冲突检测(按 host-side port 比较,"host:container" / 单port 都支持)
		for _, portSpec := range svc.Ports {
			hostPort := extractHostPort(portSpec)
			if hostPort == "" {
				continue // 解析失败的 port spec 跳过(避免假阳性)
			}
			if owner, exists := seenPorts[hostPort]; exists && owner != name {
				report.addIssue(SeverityError, name, "ports",
					fmt.Sprintf("host port %s already used by service %q (spec: %q)",
						hostPort, owner, portSpec))
			} else {
				seenPorts[hostPort] = name
			}
		}

		// healthcheck 缺失(对 binary kind 是 warning,对 external 是 error)
		if svc.Healthcheck == nil && svc.Kind != KindExternal {
			report.addIssue(SeverityInfo, name, "healthcheck",
				"no healthcheck defined; up will skip readiness wait")
		}

		report.Services = append(report.Services, vs)
	}

	report.Healthy = report.Errors == 0
	return report, nil
}

// addIssue 加一条 issue + 维护 severity counter。
func (r *ValidationReport) addIssue(sev ValidationSeverity, svc, field, msg string) {
	r.Issues = append(r.Issues, ValidationIssue{
		Severity: sev, Service: svc, Field: field, Message: msg,
	})
	switch sev {
	case SeverityError:
		r.Errors++
	case SeverityWarning:
		r.Warnings++
	case SeverityInfo:
		r.Infos++
	}
}

// HasErrors — report 是否有 error severity(给 cmd 层 exit code 用)。
func (r *ValidationReport) HasErrors() bool { return r.Errors > 0 }

// String — 人类可读 summary(给 cmd 层 table 输出用)。
func (r *ValidationReport) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Stack: %s (release: %s, level: %s)\n", r.StackID, r.Release, r.Level)
	fmt.Fprintf(&b, "Services: %d\n", len(r.Services))
	fmt.Fprintf(&b, "Issues: %d errors, %d warnings, %d infos\n", r.Errors, r.Warnings, r.Infos)
	if r.HasErrors() {
		b.WriteString("\nErrors:\n")
		for _, iss := range r.Issues {
			if iss.Severity == SeverityError {
				fmt.Fprintf(&b, "  ✗ [%s] %s: %s\n", iss.Field, iss.Service, iss.Message)
			}
		}
	}
	if r.Warnings > 0 {
		b.WriteString("\nWarnings:\n")
		for _, iss := range r.Issues {
			if iss.Severity == SeverityWarning {
				fmt.Fprintf(&b, "  ⚠ [%s] %s: %s\n", iss.Field, iss.Service, iss.Message)
			}
		}
	}
	return b.String()
}

// extractHostPort 从 "host[:container]" port spec 抽出 host port 字符串。
//
// 支持格式:"9000" / "9000:9000" / "127.0.0.1:9000:9000"。
// 不能解析返回 ""。
func extractHostPort(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ""
	}
	parts := strings.Split(spec, ":")
	switch len(parts) {
	case 1:
		return parts[0]
	case 2:
		// "host:container" — 取第一段
		return parts[0]
	case 3:
		// "ip:host:container" — 取中间段
		return parts[1]
	}
	return ""
}