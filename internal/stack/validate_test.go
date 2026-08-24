package stack

import (
	"strings"
	"testing"
)

// TestValidateV11_Basic_OK — basic level 解析成功的 schema 报告 healthy。
func TestValidateV11_Basic_OK(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
release: "v1.3.4"
services:
  a:
    binary: x
    healthcheck: { tcp: "x" }
`)
	r, err := ValidateV11(yaml, ValidationBasic)
	if err != nil {
		t.Fatalf("ValidateV11: %v", err)
	}
	if r.StackID != "x" {
		t.Errorf("StackID = %q", r.StackID)
	}
	if r.Release != "v1.3.4" {
		t.Errorf("Release = %q", r.Release)
	}
	if !r.Healthy {
		t.Errorf("expected Healthy=true, got %+v", r)
	}
	if r.Errors != 0 {
		t.Errorf("Errors = %d, want 0", r.Errors)
	}
}

// TestValidateV11_ParseError — 解析失败时报告 1 error。
func TestValidateV11_ParseError(t *testing.T) {
	yaml := []byte(`version: "2.0"
stack_id: "x"
services:
  a: { binary: x }
`)
	r, err := ValidateV11(yaml, ValidationRuntime)
	if err != nil {
		t.Fatalf("ValidateV11 returned err: %v", err)
	}
	if !r.HasErrors() {
		t.Fatal("expected HasErrors()=true")
	}
	if r.Errors != 1 {
		t.Errorf("Errors = %d, want 1", r.Errors)
	}
	if r.Issues[0].Field != "parse" {
		t.Errorf("issue Field = %q, want parse", r.Issues[0].Field)
	}
}

// TestValidateV11_PortConflict — 两个 service 用同一 port 应报 error。
func TestValidateV11_PortConflict(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a:
    binary: x
    ports: ["9000:9000"]
    healthcheck: { tcp: "x" }
  b:
    binary: y
    ports: ["9000:9001"]
    healthcheck: { tcp: "y" }
`)
	r, _ := ValidateV11(yaml, ValidationRuntime)
	if !r.HasErrors() {
		t.Fatal("expected port conflict error")
	}
	found := false
	for _, iss := range r.Issues {
		if iss.Field == "ports" && iss.Severity == SeverityError {
			found = true
			if !strings.Contains(iss.Message, "9000") {
				t.Errorf("error msg %q doesn't mention 9000", iss.Message)
			}
		}
	}
	if !found {
		t.Errorf("no ports-field error in issues: %+v", r.Issues)
	}
}

// TestValidateV11_BinaryNotFound — binary 不在 path 应报 warning。
func TestValidateV11_BinaryNotFound(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  ghost-service:
    binary: this-binary-does-not-exist-anywhere-12345
    healthcheck: { tcp: "x" }
`)
	r, _ := ValidateV11(yaml, ValidationRuntime)
	if r.Warnings == 0 {
		t.Fatal("expected binary-not-found warning")
	}
	found := false
	for _, iss := range r.Issues {
		if iss.Field == "binary" && iss.Severity == SeverityWarning {
			found = true
		}
	}
	if !found {
		t.Errorf("no binary-field warning in issues: %+v", r.Issues)
	}
}

// TestValidateV11_NoHealthcheck — binary 没 healthcheck 应报 info。
func TestValidateV11_NoHealthcheck(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  no-hc:
    binary: wau-fake
`)
	r, _ := ValidateV11(yaml, ValidationRuntime)
	found := false
	for _, iss := range r.Issues {
		if iss.Field == "healthcheck" && iss.Severity == SeverityInfo {
			found = true
		}
	}
	if !found {
		t.Errorf("expected healthcheck info issue, got: %+v", r.Issues)
	}
}

// TestValidateV11_ExternalNoHealthcheck — external 没 healthcheck 不报 info。
//
// 原因:external 是用户已有服务,健康检测由 user 自己负责,无需 wau-cli 报 info。
func TestValidateV11_ExternalNoHealthcheck(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  redis:
    kind: external
`)
	r, _ := ValidateV11(yaml, ValidationRuntime)
	for _, iss := range r.Issues {
		if iss.Service == "redis" && iss.Field == "healthcheck" {
			t.Errorf("external service shouldn't have healthcheck info: %+v", iss)
		}
	}
}

// TestValidateV11_ServicesSummary — ValidationService 摘要包含每服务字段。
func TestValidateV11_ServicesSummary(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  redis:
    kind: external
    ports: ["6379:6379"]
    healthcheck: { tcp: "x" }
  wau-core:
    binary: wau-core
    ports: ["18400:18400"]
    healthcheck: { tcp: "x" }
`)
	r, _ := ValidateV11(yaml, ValidationRuntime)
	if len(r.Services) != 2 {
		t.Fatalf("Services len = %d, want 2", len(r.Services))
	}
	// 字母序:redis 在前
	if r.Services[0].Name != "redis" {
		t.Errorf("Services[0].Name = %q, want redis", r.Services[0].Name)
	}
	if !r.Services[0].HasHealthChk {
		t.Error("redis.HasHealthChk = false, want true")
	}
	if r.Services[0].PortCount != 1 {
		t.Errorf("redis.PortCount = %d, want 1", r.Services[0].PortCount)
	}
}

// TestValidationReport_String — String() 输出包含 stack summary。
func TestValidationReport_String(t *testing.T) {
	r := &ValidationReport{
		StackID: "demo",
		Release: "v1.3.4",
		Level:   ValidationRuntime,
	}
	r.addIssue(SeverityError, "redis", "ports", "boom")
	r.addIssue(SeverityWarning, "wau-core", "binary", "missing")
	r.addIssue(SeverityInfo, "wau-store", "healthcheck", "no probe")
	if r.Errors != 1 || r.Warnings != 1 || r.Infos != 1 {
		t.Errorf("counters wrong: %d/%d/%d", r.Errors, r.Warnings, r.Infos)
	}
	s := r.String()
	if !strings.Contains(s, "demo") || !strings.Contains(s, "v1.3.4") {
		t.Errorf("String() missing key info: %s", s)
	}
	if !strings.Contains(s, "Errors:") {
		t.Errorf("String() missing Errors section: %s", s)
	}
}