// Package stack — types_v11.go
//
// 4.1.1 (2026-08-24, v1.1.0 子项 4.1) — `wau-stack.yml` v1.1 schema 类型。
//
// Diff from v1:
//   - services 是 map[string]ServiceV11(YAML key 直接是服务名,不用再写 name field)
//   - 新增 stack_id / domain / release / volumes / networks / configs / secrets / profiles
//   - service.command / service.image / service.healthcheck.grpc / service.placement
//
// D60 additive:`types.go` v1 类型 0 改,新增 v1.1 类型并存在不同文件名。
package stack

import "time"

// StackVersionV11 v1.1 schema 版本号。
const StackVersionV11 = "1.1"

// StackV11 wau-stack.yml v1.1 顶层结构。
type StackV11 struct {
	Version  string                 `yaml:"version"`              // 必须 = "1.1"
	StackID  string                 `yaml:"stack_id"`             // 必填,集群内唯一
	Domain   string                 `yaml:"domain"`               // 内部 DNS / TLS SNI
	Release  string                 `yaml:"release"`              // pin 到 wau 版本(如 "v1.3.4")
	DataDir  string                 `yaml:"data_dir"`             // 持久化根(`~` 展开)
	Services map[string]ServiceV11  `yaml:"services"`             // 服务定义(key 是服务名)
	Volumes  map[string]VolumeSpec  `yaml:"volumes,omitempty"`    // 共享卷
	Networks map[string]NetworkSpec `yaml:"networks,omitempty"`   // 共享网络
	Configs  map[string]string      `yaml:"configs,omitempty"`    // name → file content
	Secrets  map[string]SecretSpec  `yaml:"secrets,omitempty"`    // 密钥定义
	Profiles map[string]ProfileV11  `yaml:"profiles,omitempty"`   // 子集 profile
}

// ServiceV11 v1.1 单个服务定义。
//
// 注意:image 字段当前版本解析时 warn "Docker kind not in v1.1",留作 schema 预留。
type ServiceV11 struct {
	Kind        ServiceKind      `yaml:"kind,omitempty"`         // binary / external(docker 预留)
	Command     []string         `yaml:"command,omitempty"`      // entrypoint(可覆盖 binary 默认)
	Args        []string         `yaml:"args,omitempty"`         // 传给 command 的参数
	Ports       []string         `yaml:"ports,omitempty"`        // "18400:18400"
	Env         map[string]string `yaml:"env,omitempty"`
	DependsOn   []string         `yaml:"depends_on,omitempty"`
	Healthcheck *HealthcheckSpec `yaml:"healthcheck,omitempty"`
	Placement   *PlacementSpec   `yaml:"placement,omitempty"`   // v1.1 = local only(字段预留)
	Required    bool             `yaml:"required,omitempty"`     // 起不来是否 abort up
	Image       string           `yaml:"image,omitempty"`        // reserved, 解析时 warn
	Binary      string           `yaml:"binary,omitempty"`       // kind=binary 时与 command 二选一
}

// HealthcheckSpec v1.1 健康探针(单一类型,grpc/http/tcp/exec 四选一)。
type HealthcheckSpec struct {
	GRPC     string        `yaml:"grpc,omitempty"`     // "wau.HealthService/Check"
	HTTP     string        `yaml:"http,omitempty"`     // url
	TCP      string        `yaml:"tcp,omitempty"`      // "host:port"
	Exec     string        `yaml:"exec,omitempty"`     // command
	Interval time.Duration `yaml:"interval,omitempty"`
	Timeout  time.Duration `yaml:"timeout,omitempty"`
	Retries  int           `yaml:"retries,omitempty"`
}

// PlacementSpec v1.1.x 预留字段 — 服务放置位置。
// 当前 v1.1 = local only,host 字段解析但不实际生效。
type PlacementSpec struct {
	Host string `yaml:"host,omitempty"` // reserved for v1.1.x
}

// SecretSpec 密钥定义 — 从文件读或从 env 读。
type SecretSpec struct {
	File string `yaml:"file,omitempty"` // /run/secrets/wau-jwt.hex
	Env  string `yaml:"env,omitempty"`  // WAU_JWT_SHARED_SECRET
}

// VolumeSpec 共享卷定义。
type VolumeSpec struct {
	Driver string `yaml:"driver,omitempty"` // "local"(默认)
	Path   string `yaml:"path,omitempty"`   // host 路径
}

// NetworkSpec 共享网络定义。
type NetworkSpec struct {
	Driver string `yaml:"driver,omitempty"` // "bridge"(默认)
	Subnet string `yaml:"subnet,omitempty"` // v1.1.x 预留
}

// ProfileV11 v1.1 子集 profile。
type ProfileV11 struct {
	Services []string `yaml:"services"`
}