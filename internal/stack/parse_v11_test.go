package stack

import (
	"strings"
	"testing"
)

// TestParseV11_ValidMinimal — 单 service 最小有效 schema。
func TestParseV11_ValidMinimal(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "wau-prod-001"
services:
  wau-core:
    binary: wau-core-kernel
    args: ["--config", "/etc/wau/kernel.yaml"]
    healthcheck:
      tcp: "localhost:18400"
      interval: 5s
      timeout: 30s
`)
	s, err := ParseV11(yaml)
	if err != nil {
		t.Fatalf("ParseV11: %v", err)
	}
	if s.Version != "1.1" {
		t.Errorf("Version = %q, want 1.1", s.Version)
	}
	if s.StackID != "wau-prod-001" {
		t.Errorf("StackID = %q, want wau-prod-001", s.StackID)
	}
	if len(s.Services) != 1 {
		t.Errorf("Services len = %d, want 1", len(s.Services))
	}
	if _, ok := s.Services["wau-core"]; !ok {
		t.Error("services[wau-core] not found")
	}
}

// TestParseV11_FullSchema — 全字段 + 多种 service kind + profiles。
func TestParseV11_FullSchema(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "wau-prod-full"
domain: "wau.example.com"
release: "v1.3.4"
data_dir: "/var/lib/wau"
services:
  redis:
    kind: external
    healthcheck:
      tcp: "localhost:6379"
      interval: 2s
  wau-core-kernel:
    kind: binary
    binary: wau-core-kernel
    args: ["--config", "/etc/wau/kernel.yaml"]
    ports: ["18400:18400"]
    env:
      WAU_REGISTRY_URL: "http://wau-registry:18401"
    depends_on: ["redis", "wau-registry"]
    healthcheck:
      grpc: "wau.HealthService/Check"
      interval: 10s
      timeout: 30s
      retries: 3
    required: true
  wau-registry:
    kind: binary
    command: ["wau-registry", "--port", "18401"]
    depends_on: ["redis"]
    healthcheck:
      http: "http://localhost:18401/health"
volumes:
  wau-store-data:
    driver: local
    path: /var/lib/wau/store
networks:
  wau-internal:
    driver: bridge
configs:
  kernel.toml: |
    [server]
    grpc_port = 18400
secrets:
  WAU_JWT_SHARED_SECRET:
    file: /run/secrets/wau-jwt.hex
profiles:
  minimal:
    services: ["redis", "wau-core-kernel"]
`)
	s, err := ParseV11(yaml)
	if err != nil {
		t.Fatalf("ParseV11: %v", err)
	}
	if s.Domain != "wau.example.com" {
		t.Errorf("Domain = %q", s.Domain)
	}
	if s.Release != "v1.3.4" {
		t.Errorf("Release = %q", s.Release)
	}
	if _, ok := s.Volumes["wau-store-data"]; !ok {
		t.Error("volumes[wau-store-data] not found")
	}
	if _, ok := s.Secrets["WAU_JWT_SHARED_SECRET"]; !ok {
		t.Error("secrets[WAU_JWT_SHARED_SECRET] not found")
	}
	if _, ok := s.Profiles["minimal"]; !ok {
		t.Error("profiles[minimal] not found")
	}

	// wau-core-kernel has 2 deps; wau-registry has 1
	wck := s.Services["wau-core-kernel"]
	if len(wck.DependsOn) != 2 {
		t.Errorf("wau-core-kernel.DependsOn len = %d, want 2", len(wck.DependsOn))
	}
	if wck.Healthcheck == nil || wck.Healthcheck.GRPC != "wau.HealthService/Check" {
		t.Error("wau-core-kernel healthcheck.grpc not parsed")
	}
}

// TestParseV11_BadVersion — version 不等于 1.1。
func TestParseV11_BadVersion(t *testing.T) {
	yaml := []byte(`
version: "2.0"
stack_id: "x"
services:
  a: { binary: x }
`)
	_, err := ParseV11(yaml)
	if err == nil {
		t.Fatal("expected error for bad version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("error %q doesn't mention version", err)
	}
}

// TestParseV11_MissingStackID — stack_id 必填。
func TestParseV11_MissingStackID(t *testing.T) {
	yaml := []byte(`
version: "1.1"
services:
  a: { binary: x }
`)
	_, err := ParseV11(yaml)
	if err == nil {
		t.Fatal("expected error for missing stack_id")
	}
	if !strings.Contains(err.Error(), "stack_id") {
		t.Errorf("error %q doesn't mention stack_id", err)
	}
}

// TestParseV11_UnknownDep — service depends on 不存在的服务。
func TestParseV11_UnknownDep(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a:
    binary: x
    depends_on: ["ghost"]
`)
	_, err := ParseV11(yaml)
	if err == nil {
		t.Fatal("expected error for unknown dep")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error %q doesn't mention ghost", err)
	}
}

// TestParseV11_SelfDep — service depends on 自己。
func TestParseV11_SelfDep(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a:
    binary: x
    depends_on: ["a"]
`)
	_, err := ParseV11(yaml)
	if err == nil {
		t.Fatal("expected error for self-dep")
	}
}

// TestParseV11_Cycle — a→b→a。
func TestParseV11_Cycle(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a:
    binary: x
    depends_on: ["b"]
  b:
    binary: y
    depends_on: ["a"]
`)
	_, err := ParseV11(yaml)
	if err == nil {
		t.Fatal("expected error for circular dep")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error %q doesn't mention circular", err)
	}
}

// TestParseV11_Healthcheck_OK_GRPCOnly — grpc 单一 OK。
func TestParseV11_Healthcheck_OK_GRPCOnly(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a:
    binary: x
    healthcheck:
      grpc: "wau.HealthService/Check"
`)
	_, err := ParseV11(yaml)
	if err != nil {
		t.Fatalf("ParseV11: %v", err)
	}
}

// TestParseV11_Healthcheck_TwoTypes — 同时填 grpc + http 应失败。
func TestParseV11_Healthcheck_TwoTypes(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a:
    binary: x
    healthcheck:
      grpc: "x"
      http: "http://x"
`)
	_, err := ParseV11(yaml)
	if err == nil {
		t.Fatal("expected error for two healthcheck types")
	}
}

// TestParseV11_Healthcheck_None — 健康探针全空应失败。
func TestParseV11_Healthcheck_None(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a:
    binary: x
    healthcheck:
      interval: 5s
`)
	_, err := ParseV11(yaml)
	if err == nil {
		t.Fatal("expected error for empty healthcheck")
	}
}

// TestParseV11_DockerKindReserved — kind=docker 解析时拒绝。
func TestParseV11_DockerKindReserved(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a:
    kind: docker
    image: "redis:7"
`)
	_, err := ParseV11(yaml)
	if err == nil {
		t.Fatal("expected error for kind=docker")
	}
	if !strings.Contains(err.Error(), "docker") {
		t.Errorf("error %q doesn't mention docker", err)
	}
}

// TestParseV11_ImageReserved — image 字段解析时拒绝(留 schema 位)。
func TestParseV11_ImageReserved(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a:
    binary: x
    image: "redis:7"
`)
	_, err := ParseV11(yaml)
	if err == nil {
		t.Fatal("expected error for image field")
	}
}

// TestParseV11_PlacementReserved — placement.host 解析时拒绝。
func TestParseV11_PlacementReserved(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a:
    binary: x
    placement:
      host: "node-2.example.com"
`)
	_, err := ParseV11(yaml)
	if err == nil {
		t.Fatal("expected error for placement.host")
	}
}

// TestParseV11_BinaryKindNeedsBinary — kind=binary 必须 binary 或 command 至少一个。
func TestParseV11_BinaryKindNeedsBinary(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a:
    kind: binary
`)
	_, err := ParseV11(yaml)
	if err == nil {
		t.Fatal("expected error for empty binary kind")
	}
}

// TestParseV11_ApplyProfile — profile 过滤 + depends_on 闭包。
func TestParseV11_ApplyProfile(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  redis:
    kind: external
    healthcheck: { tcp: "localhost:6379" }
  wau-core:
    binary: x
    depends_on: ["redis"]
  wau-registry:
    binary: y
    depends_on: ["redis"]
  wau-agent:
    binary: z
    depends_on: ["wau-core", "wau-registry"]
profiles:
  minimal:
    services: ["wau-agent"]
`)
	s, err := ParseV11(yaml)
	if err != nil {
		t.Fatalf("ParseV11: %v", err)
	}
	got, err := s.ApplyProfile("minimal")
	if err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}
	want := []string{"redis", "wau-core", "wau-registry", "wau-agent"}
	if !equalStrings(got, want) {
		t.Errorf("ApplyProfile(minimal) = %v, want %v (with depends_on closure)", got, want)
	}
}

// TestParseV11_ApplyProfile_NotFound — profile 名不存在。
func TestParseV11_ApplyProfile_NotFound(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a: { binary: x }
`)
	s, _ := ParseV11(yaml)
	_, err := s.ApplyProfile("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
}

// TestParseV11_TopoOrder_3Nodes — 三服务拓扑排序字母序确定性。
func TestParseV11_TopoOrder_3Nodes(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  zeta:
    binary: z
    depends_on: ["alpha", "beta"]
  beta:
    binary: b
    depends_on: ["alpha"]
  alpha:
    binary: a
`)
	s, err := ParseV11(yaml)
	if err != nil {
		t.Fatalf("ParseV11: %v", err)
	}
	order, err := s.TopoOrder()
	if err != nil {
		t.Fatalf("TopoOrder: %v", err)
	}
	want := []string{"alpha", "beta", "zeta"}
	if !equalStrings(order, want) {
		t.Errorf("TopoOrder = %v, want %v", order, want)
	}
}

// TestParseStackFile_Dispatcher_V1 — version=1 走 v1 路径。
func TestParseStackFile_Dispatcher_V1(t *testing.T) {
	yaml := []byte(`
version: "1"
stack:
  name: default
services:
  - name: a
    binary: x
`)
	got, err := ParseStackFile(yaml)
	if err != nil {
		t.Fatalf("ParseStackFile: %v", err)
	}
	if _, ok := got.(*Stack); !ok {
		t.Errorf("dispatcher returned %T, want *Stack (v1)", got)
	}
}

// TestParseStackFile_Dispatcher_V11 — version=1.1 走 v1.1 路径。
func TestParseStackFile_Dispatcher_V11(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a:
    binary: x
`)
	got, err := ParseStackFile(yaml)
	if err != nil {
		t.Fatalf("ParseStackFile: %v", err)
	}
	if _, ok := got.(*StackV11); !ok {
		t.Errorf("dispatcher returned %T, want *StackV11", got)
	}
}

// TestParseStackFile_Dispatcher_Unsupported — 不支持的版本号。
func TestParseStackFile_Dispatcher_Unsupported(t *testing.T) {
	yaml := []byte(`version: "99"`)
	_, err := ParseStackFile(yaml)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error %q doesn't mention unsupported", err)
	}
}

// TestParseStackFile_Dispatcher_EmptyVersion — 空 version 走 v1 路径(Parse 校验通过)。
//
// 注:ParseStackFile 把空 version 路由到 Parse(v1)。Parse 要求 version="1",
// 所以 YAML 里要带显式 version: "1"。这测的是 dispatcher routing,
// 不是 Parse 兼容空 version(那是 Parse 的事,D60 不动)。
func TestParseStackFile_Dispatcher_EmptyVersion(t *testing.T) {
	yaml := []byte(`
version: "1"
stack:
  name: default
services:
  - name: a
    binary: x
`)
	got, err := ParseStackFile(yaml)
	if err != nil {
		t.Fatalf("ParseStackFile: %v", err)
	}
	if _, ok := got.(*Stack); !ok {
		t.Errorf("dispatcher returned %T for v1 YAML, want *Stack", got)
	}
}

// TestServiceByName_V11_OK — 找到。
func TestServiceByName_V11_OK(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  redis: { kind: external, healthcheck: { tcp: "x" } }
  wau-core: { binary: x }
`)
	s, _ := ParseV11(yaml)
	svc, ok := s.ServiceByName("wau-core")
	if !ok {
		t.Fatal("ServiceByName(wau-core) not found")
	}
	if svc.Binary != "x" {
		t.Errorf("Binary = %q, want x", svc.Binary)
	}
}

// TestServiceByName_V11_NotFound — 找不到。
func TestServiceByName_V11_NotFound(t *testing.T) {
	yaml := []byte(`
version: "1.1"
stack_id: "x"
services:
  a: { binary: x }
`)
	s, _ := ParseV11(yaml)
	_, ok := s.ServiceByName("ghost")
	if ok {
		t.Fatal("expected not found for ghost")
	}
}

// helpers
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}