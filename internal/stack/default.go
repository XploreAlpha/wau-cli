// Package stack - default.go
//
// 第一刀 1.1 — 内置 default 9-service stack(per v1.0.0 19 仓真实架构 + D88/D89)。
// 第三刀 3.1 — 修 health probe 端点(per 真实 binary 启动验证 2026-08-20):
//   - wau-core           /health   (原来 /healthz 错)
//   - registry           /health   (原来 /healthz 错)
//   - wau-profile        TCP 50062 (gRPC only,无 HTTP)
//   - wau-channel        /health   (原来缺 probe)
//   - 其他保持
//
// 这是 visa demo 的 single-node 默认 stack:1 个 redis + 8 个 wau 服务。
// 用户可以写 wau-stack.yml 覆盖其中任意字段(参看 schema)。
package stack

import "time"

// DefaultStack 返回内置 9-service default stack。
//
// 9 服务清单(按端口依赖排序):
//   1. redis         :6379   infra
//   2. wau-core      :18400  内核(http 18400 / grpc 50051 / libp2p 4001)
//   3. registry      :18401  wau-registry-service(http 18401 / grpc 50052)
//   4. wau-store     :18405  存储
//   5. wau-intent    :50053  意图解析(grpc 50053)
//   6. wau-profile   :50062  profile 服务(grpc 50062,http 8082)
//   7. wau-llm-router:18403  LLM 路由(http 18403 / grpc 18404)
//   8. wau-edge      :18402  edge 入口(http 18402)
//   9. wau-channel   :18410  通道(http 18410 / webhook 18411)
//  10. wau-agent     :19408  agent(jsonrpc 19408 / gob 19407)
//
// 端口来源:project-wau-19-repo-real-architecture-2026-07-15。
func DefaultStack() *Stack {
	probeHTTP := func(url string) *Probe {
		return &Probe{
			Type:     ProbeHTTP,
			URL:      url,
			Interval: 500 * time.Millisecond,
			Timeout:  30 * time.Second,
		}
	}
	probeTCP := func(port int) *Probe {
		return &Probe{
			Type:     ProbeTCP,
			Port:     port,
			Interval: 500 * time.Millisecond,
			Timeout:  10 * time.Second,
		}
	}

	return &Stack{
		Version: StackVersion,
		Stack: StackMeta{
			Name:    "default",
			DataDir: "~/.wau/run",
			LogDir:  "~/.wau/log",
		},
		Services: []Service{
			// 1. redis — infra(external)
			{
				Name:     "redis",
				Kind:     KindExternal,
				HTTPPort: 6379,
				Required: true,
				Health:   probeTCP(6379),
			},
			// 2. wau-core — 内核
			//    第三刀 3.1:加 WAU_NET_LIBP2P_DISABLED=true 让本机 demo 不依赖 libp2p 静态 relay
			{
				Name:      "wau-core",
				Kind:      KindBinary,
				Binary:    "wau-core",
				HTTPPort:  18400,
				GRPCPort:  50051,
				DependsOn: []string{"redis"},
				Required:  true,
				Env: map[string]string{
					"WAU_LOG_LEVEL":            "info",
					"WAU_NET_LIBP2P_DISABLED":  "true", // visa demo single-node 不需要 p2p
				},
				Health: probeHTTP("http://localhost:18400/health"),
			},
			// 3. registry — wau-registry-service
			{
				Name:      "registry",
				Kind:      KindBinary,
				Binary:    "registry",
				HTTPPort:  18401,
				GRPCPort:  50052,
				DependsOn: []string{"wau-core"},
				Required:  true,
				Health:    probeHTTP("http://localhost:18401/health"),
			},
			// 4. wau-store
			{
				Name:      "wau-store",
				Kind:      KindBinary,
				Binary:    "wau-store",
				HTTPPort:  18405,
				DependsOn: []string{"wau-core"},
				Health:    probeHTTP("http://localhost:18405/healthz"),
			},
			// 5. wau-intent
			{
				Name:      "wau-intent",
				Kind:      KindBinary,
				Binary:    "wau-intent-service",
				GRPCPort:  50053,
				DependsOn: []string{"wau-core"},
				Health:    probeHTTP("http://localhost:50053/health"),
			},
			// 6. wau-profile — gRPC only,无 HTTP endpoint → TCP probe
			{
				Name:      "wau-profile",
				Kind:      KindBinary,
				Binary:    "wau-profile-service",
				GRPCPort:  50062,
				DependsOn: []string{"wau-core"},
				Health:    probeTCP(50062),
			},
			// 7. wau-llm-router
			{
				Name:      "wau-llm-router",
				Kind:      KindBinary,
				Binary:    "wau-llm-router",
				HTTPPort:  18403,
				GRPCPort:  18404,
				DependsOn: []string{"wau-core"},
				Health:    probeHTTP("http://localhost:18403/healthz"),
			},
			// 8. wau-edge
			{
				Name:      "wau-edge",
				Kind:      KindBinary,
				Binary:    "wau-edge",
				HTTPPort:  18402,
				DependsOn: []string{"wau-core", "wau-llm-router"},
				Health:    probeHTTP("http://localhost:18402/healthz"),
			},
			// 9. wau-channel — 第三刀 3.1:加 /health probe(原来缺)
			{
				Name:        "wau-channel",
				Kind:        KindBinary,
				Binary:      "wau-channel",
				HTTPPort:    18410,
				WebhookPort: 18411,
				DependsOn:   []string{"wau-core", "registry"},
				Health:      probeHTTP("http://localhost:18410/health"),
			},
			// 10. wau-agent
			{
				Name:         "wau-agent",
				Kind:         KindBinary,
				Binary:       "wau-agent",
				GRPCPort:     19408,
				WebhookPort:  19407,
				DependsOn:    []string{"wau-core", "registry"},
				Health:       probeTCP(19408),
			},
		},
		Profiles: map[string]Profile{
			// demo profile:9 服务全起(visa 拍板决定,2026-08-20)
			"demo": {
				Services: []string{
					"redis", "wau-core", "registry", "wau-store",
					"wau-intent", "wau-profile", "wau-llm-router",
					"wau-edge", "wau-channel", "wau-agent",
				},
			},
			// minimal:只要 core + registry,debug 用
			"minimal": {
				Services: []string{"redis", "wau-core", "registry"},
			},
		},
	}
}
