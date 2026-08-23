package stack

import (
	"strings"
	"testing"
)

func TestParse_ValidMinimal(t *testing.T) {
	yaml := `version: "1"
stack:
  name: test
services:
  - name: a
    binary: a-bin
  - name: b
    binary: b-bin
    depends_on: [a]
`
	s, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.Services) != 2 {
		t.Fatalf("services=%d", len(s.Services))
	}
	order, err := s.TopoOrder()
	if err != nil {
		t.Fatalf("TopoOrder: %v", err)
	}
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Errorf("order=%v, want [a b]", order)
	}
}

func TestParse_DefaultStack(t *testing.T) {
	s := DefaultStack()
	if err := s.Validate(); err != nil {
		t.Fatalf("default Validate: %v", err)
	}
	if len(s.Services) != 10 {
		t.Errorf("default services=%d, want 10", len(s.Services))
	}
	order, err := s.TopoOrder()
	if err != nil {
		t.Fatalf("default TopoOrder: %v", err)
	}
	if order[0] != "redis" {
		t.Errorf("first service should be redis (no deps), got %q", order[0])
	}
	// wau-core 必须在所有依赖 wau-core 的服务之前
	wauCoreIdx := indexOf(order, "wau-core")
	for _, name := range []string{"registry", "wau-store", "wau-intent",
		"wau-profile", "wau-llm-router", "wau-edge", "wau-channel", "wau-agent"} {
		if indexOf(order, name) <= wauCoreIdx {
			t.Errorf("%s should come after wau-core in topo order", name)
		}
	}
}

func TestParse_ProfileApply(t *testing.T) {
	s := DefaultStack()
	services, err := s.ApplyProfile("minimal")
	if err != nil {
		t.Fatalf("ApplyProfile minimal: %v", err)
	}
	if len(services) != 3 {
		t.Errorf("minimal services=%d, want 3 (redis + wau-core + registry)", len(services))
	}
}

func TestParse_ProfileApply_Unknown(t *testing.T) {
	s := DefaultStack()
	_, err := s.ApplyProfile("nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("want 'not found' error, got %v", err)
	}
}

func TestParse_CircularDependency(t *testing.T) {
	yaml := `version: "1"
stack:
  name: cycle
services:
  - name: a
    binary: a-bin
    depends_on: [c]
  - name: b
    binary: b-bin
    depends_on: [a]
  - name: c
    binary: c-bin
    depends_on: [b]
`
	_, err := Parse([]byte(yaml))
	if err == nil || !strings.Contains(err.Error(), "circular") {
		t.Errorf("want circular error, got %v", err)
	}
}

func TestParse_UnknownDep(t *testing.T) {
	yaml := `version: "1"
stack:
  name: bad
services:
  - name: a
    binary: a-bin
    depends_on: [ghost]
`
	_, err := Parse([]byte(yaml))
	if err == nil || !strings.Contains(err.Error(), "unknown service") {
		t.Errorf("want unknown service error, got %v", err)
	}
}

func TestParse_BadName(t *testing.T) {
	yaml := `version: "1"
stack:
  name: bad
services:
  - name: ""
    binary: x
`
	_, err := Parse([]byte(yaml))
	if err == nil || !strings.Contains(err.Error(), "name is empty") {
		t.Errorf("want empty-name error, got %v", err)
	}
}

func TestParse_BadKind(t *testing.T) {
	yaml := `version: "1"
stack:
  name: bad
services:
  - name: a
    kind: kubernetes
    binary: x
`
	_, err := Parse([]byte(yaml))
	if err == nil || !strings.Contains(err.Error(), "kind") {
		t.Errorf("want kind error, got %v", err)
	}
}

func TestParse_ExternalRequiresHealth(t *testing.T) {
	yaml := `version: "1"
stack:
  name: bad
services:
  - name: redis
    kind: external
`
	_, err := Parse([]byte(yaml))
	if err == nil || !strings.Contains(err.Error(), "health is required") {
		t.Errorf("want health-required error, got %v", err)
	}
}

func TestParse_BadProbeType(t *testing.T) {
	yaml := `version: "1"
stack:
  name: bad
services:
  - name: a
    kind: external
    health:
      type: udp
      port: 9999
`
	_, err := Parse([]byte(yaml))
	if err == nil || !strings.Contains(err.Error(), "probe type") {
		t.Errorf("want probe-type error, got %v", err)
	}
}

func TestProbeValidate(t *testing.T) {
	tests := []struct {
		name string
		p    Probe
		ok   bool
	}{
		{"tcp-port", Probe{Type: ProbeTCP, Port: 6379}, true},
		{"tcp-addr", Probe{Type: ProbeTCP, Addr: "1.2.3.4:6379"}, true},
		{"tcp-no-addr", Probe{Type: ProbeTCP}, false},
		{"http", Probe{Type: ProbeHTTP, URL: "http://x"}, true},
		{"http-no-url", Probe{Type: ProbeHTTP}, false},
		{"exec", Probe{Type: ProbeExec, Cmd: "true"}, true},
		{"exec-no-cmd", Probe{Type: ProbeExec}, false},
		{"unknown", Probe{Type: "foo"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.p.validate()
			if (err == nil) != tc.ok {
				t.Errorf("validate() err=%v, ok=%v", err, tc.ok)
			}
		})
	}
}

func indexOf(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}
