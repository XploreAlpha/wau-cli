package defaults

import (
	"strings"
	"testing"

	"github.com/wau/wau-cli/internal/stack"
)

// TestDefaultStackYAML_NotEmpty — embed bytes 非空。
func TestDefaultStackYAML_NotEmpty(t *testing.T) {
	if len(DefaultStackYAMLBytes()) == 0 {
		t.Fatal("DefaultStackYAML is empty (embed failed)")
	}
}

// TestDefaultStackYAML_Version11 — version = "1.1"。
func TestDefaultStackYAML_Version11(t *testing.T) {
	if !strings.Contains(string(DefaultStackYAMLBytes()), `version: "1.1"`) {
		t.Error(`default YAML doesn't contain version: "1.1"`)
	}
}

// TestDefaultStackYAML_ReleaseIsV134 — release pin = "v1.3.4"。
func TestDefaultStackYAML_ReleaseIsV134(t *testing.T) {
	s, err := stack.ParseV11(DefaultStackYAMLBytes())
	if err != nil {
		t.Fatalf("ParseV11: %v", err)
	}
	if s.Release != "v1.3.4" {
		t.Errorf("Release = %q, want v1.3.4 (Jade alignment per D92)", s.Release)
	}
}

// TestDefaultStackYAML_AllServices — 10 服务都在(1 redis external + 9 wau binary)。
func TestDefaultStackYAML_AllServices(t *testing.T) {
	s, err := stack.ParseV11(DefaultStackYAMLBytes())
	if err != nil {
		t.Fatalf("ParseV11: %v", err)
	}
	if len(s.Services) != 10 {
		t.Errorf("services count = %d, want 10", len(s.Services))
	}
	want := []string{
		"redis", "wau-core", "registry", "wau-store", "wau-intent",
		"wau-profile", "wau-llm-router", "wau-edge", "wau-channel", "wau-agent",
	}
	for _, name := range want {
		if _, ok := s.Services[name]; !ok {
			t.Errorf("service %q missing from default stack", name)
		}
	}
}

// TestDefaultStackYAML_ParseableV11 — embed YAML 通过 ParseV11 schema 校验。
func TestDefaultStackYAML_ParseableV11(t *testing.T) {
	if _, err := stack.ParseV11(DefaultStackYAMLBytes()); err != nil {
		t.Fatalf("ParseV11 validation: %v", err)
	}
}

// TestDefaultStackYAML_Profiles — demo + minimal profile 都定义。
func TestDefaultStackYAML_Profiles(t *testing.T) {
	s, _ := stack.ParseV11(DefaultStackYAMLBytes())
	if _, ok := s.Profiles["demo"]; !ok {
		t.Error("demo profile missing")
	}
	if _, ok := s.Profiles["minimal"]; !ok {
		t.Error("minimal profile missing")
	}
}

// TestDefaultStackYAML_TopoOrder — 默认 stack 拓扑无环 + 10 服务全排出。
func TestDefaultStackYAML_TopoOrder(t *testing.T) {
	s, err := stack.ParseV11(DefaultStackYAMLBytes())
	if err != nil {
		t.Fatalf("ParseV11: %v", err)
	}
	order, err := s.TopoOrder()
	if err != nil {
		t.Fatalf("TopoOrder: %v", err)
	}
	if len(order) != 10 {
		t.Errorf("TopoOrder len = %d, want 10", len(order))
	}
	// redis 必须第一个(无 dep)
	if order[0] != "redis" {
		t.Errorf("TopoOrder[0] = %q, want redis", order[0])
	}
}