# wau-cli

[![Version](https://img.shields.io/badge/version-v0.1.0--dev-blue)](https://github.com/wau/wau-cli)
[![Release](https://img.shields.io/badge/release-Genesis-orange)](CHANGELOG.md)
[![Go](https://img.shields.io/badge/go-1.23+-00ADD8)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-green)](LICENSE)

> Official command-line client for [WAU-core-kernel](https://github.com/wau/core-kernel).

`wau-cli` provides a `kubectl`/`docker`-like experience for managing WAU services, including agents, tasks, and kernel information.

**Current Release**: v0.1.0 **"Genesis"** — First MVP 🎉

See [CHANGELOG.md](CHANGELOG.md) for the version naming convention and history.

---

## ✨ Features

- 🩺 **Health check** - Quickly verify kernel health status
- 🤖 **Agent management** - List, get, register, deregister agents
- 📋 **Task management** - Submit, query tasks
- ⚙️ **Config management** - Initialize, validate, show configuration
- 📊 **Multiple output formats** - Table, JSON, YAML, CSV
- 🔐 **RBAC support** - Multiple role levels
- 🚀 **Single binary** - No runtime dependencies

---

## 📦 Installation

### From source

```bash
git clone https://github.com/wau/wau-cli
cd wau-cli
go build -o wau ./cmd/wau
mv wau /usr/local/bin/
```

### (Coming soon) Pre-built binaries

```bash
# macOS
brew install wau

# Linux
curl -fsSL https://wau.dev/install.sh | sh
```

---

## 🚀 Quick Start

```bash
# 1. Initialize configuration
wau config init

# 2. Check kernel health
wau health

# 3. List all online agents
wau agent list

# 4. Register a new agent
wau agent register \
  --name fox-medical \
  --url http://100.125.99.209:18800 \
  --skills medical,clinical

# 5. Submit a task
wau task submit "帮我查一下天气"

# 6. Get task details
wau task get task_1700000000
```

---

## 📚 Commands

### `wau stack` ⭐ v1.0.0 第一刀 (2026-08-20)

Manage the local WAU stack — bring services up, tear them down, and inspect status.

Equivalent to `docker compose` / `kubectl` for a single-node WAU deployment.

```bash
# 架构可视化(签证 demo 必杀技)
wau stack up --dry-run                # 打印 10-service 启动 plan
wau stack up --demo --dry-run         # 同上,显式 demo profile

# 真起(需本机装 9 个 binary 到 ~/.wau/bin)
wau stack up --demo                   # 起 9 服务 + 等 health check
wau stack up --profile minimal --detach  # 只起 redis+core+registry,后台

# 看状态
wau stack ls                          # table 输出
wau stack ls -o json                  # JSON 给脚本
wau stack ls -o yaml                  # YAML
wau stack status                      # alias for ls
wau stack ps                          # alias for ls

# 关停
wau stack down                        # SIGTERM → 5s → SIGKILL
wau stack down --all                  # 关停 + 清 runtime state
wau stack down --force                # 强杀(即使失败服务)

# 自定义 stack 文件
wau stack up --file /path/to/wau-stack.yml

# 看日志(第四刀 P4.1,2026-08-24)
wau log wau-core                          # 最后 50 行(顶层 alias)
wau log wau-core --follow                 # tail -F,Ctrl-C 退出
wau log wau-core --lines 200 --grep ERROR # 最后 200 行 + 过滤
wau log wau-core --since 5m               # 最近 5 分钟
wau stack logs                            # 所有服务并行 fanout
wau stack logs wau-core --no-color        # 单服务,关彩色
```

**内置 default 9-service stack**(per 19 仓真实架构):
`redis + wau-core + registry + wau-store + wau-intent + wau-profile + wau-llm-router + wau-edge + wau-channel + wau-agent`

**Profiles**:
- `demo` — 9 服务全起(visa 拍板,2026-08-20)
- `minimal` — 只起 redis + wau-core + registry(debug 用)

### `wau log` / `wau stack logs` ⭐ v1.0.1 P4.1 (2026-08-24)

Show recent or follow logs for stack services. Equivalent to `docker logs` / `kubectl logs` / `journalctl -u`.

```bash
# 单服务
wau log wau-core                  # 最后 50 行
wau log wau-core --follow         # tail -F
wau log wau-core --lines 200      # 最后 200 行(0 = 全部)
wau log wau-core --grep ERROR     # 正则过滤(grep regex)
wau log wau-core --since 5m       # 最近 5 分钟(解析 RFC3339Nano ts)
wau log wau-core --no-color       # 关彩色

# 多服务 fanout(每服务带颜色前缀)
wau stack logs                    # 所有 loggable 服务并行
wau stack logs wau-core           # 单服务(同 `wau log`)
wau stack logs --follow           # 全部 tail -F
wau stack logs --grep "ERROR|panic"
```

**Flags**: `--follow/-f` · `--lines/-n` · `--grep` · `--since` · `--no-color`

**实现**:
- `--follow` 走 POSIX `tail -F`(处理 rotate/truncate,无需新 dep;Linux/macOS)
- 非 follow 走 stdlib read+filter+tail-N
- 多服务 fanout 用 `sync.WaitGroup` + `SafeWriter`(mutex 保护,多 goroutine 写 stdout 不交错)
- 颜色:8-color cycler(cyan/yellow/magenta/green/blue/red/bright cyan/bright yellow),`--no-color` 关
- `redis` external 服务在 fanout 中自动 skip(无 log file)

### `wau stack init-configs` ⭐ v1.0.1 P4.2 (2026-08-24)

Write embedded service config templates to `~/.wau/configs/`. Solves stage3's "missing configs/*.yaml" onboarding blocker.

```bash
# 默认:写所有 4 个服务 config 到 ~/.wau/configs/
wau stack init-configs

# 单服务
wau stack init-configs --service wau-store
wau stack init-configs --service wau-llm-router   # alias: --service router

# 自定义输出目录
wau stack init-configs --output-dir /etc/wau/configs

# 覆盖已有文件
wau stack init-configs --force

# 只看 plan,不写
wau stack init-configs --dry-run
```

**4 个服务**(embed 在 wau-cli binary):
| Service | Config filename | Purpose |
|---------|-----------------|---------|
| `wau-store` | `store.yaml` | 存储(postgres + redis + admin) |
| `wau-llm-router` | `router.yaml` | LLM 路由(thompson / newapi sidecar) |
| `wau-edge` | `edge.yaml` | Edge 入口(WS / OpenAI compat / dashboard / newapi) |
| `wau-channel` | `channel.yaml` | 通道(telegram / discord / slack / feishu / dingtalk / qq / email / webhook) |

**Flags**: `--service` · `--output-dir` (default `~/.wau/configs`) · `--force` · `--dry-run`

**行为**:
- 文件已存在 → 默认 skip(提示 `--force`)
- `--force` → overwrite
- `--dry-run` → 只 print,不写
- atomic write(`.tmp` + `rename`,无 partial file)
- **不展开** env placeholder(如 `$WAU_STORE_PG_DSN`)— 由部署层脚本替换

**前提**:跑完后 `wau stack up --demo` 才能完整启动 wau-store / wau-llm-router / wau-edge / wau-channel(否则还是 stage3 的 "config not found" 报错)。

### `wau health`

Check kernel health.

```bash
wau health                  # Simple check
wau health --wait           # Wait for healthy
wau health --wait --timeout 60s
wau health --addr http://43.134.126.126:18400   # 远端
```

### `wau kernel`

Show kernel information.

```bash
wau kernel info             # Show detailed info
wau kernel version          # Show version
```

### `wau agent`

Manage agents.

```bash
wau agent list                              # List all agents
wau agent list --page 2 --page-size 50      # Pagination
wau agent list --skill medical              # Filter by skill
wau agent list --status online              # Filter by status
wau agent list --search fox                 # Search by name
wau agent get fox                           # Get agent details
wau agent score fox                         # Get agent score
wau agent register --name fox --url ...     # Register new agent
wau agent deregister fox                    # Remove agent

# L5 包管理器(per D72/D73/D74,2026-07-10)— 类 apt
wau agent search medical --universe medical        # 类 apt search
wau agent install fox-medical                       # 类 apt install
wau agent install fox-medical --version=1.2.3      # 锁版本
wau agent update                                    # 全更新
wau agent update fox-medical                        # 单独更新
wau agent uninstall fox-medical                     # 卸载
wau agent uninstall fox-medical --purge             # 全删
wau agent login                                      # 登入 wau-registry
wau agent publish --from ./weather-bot              # 发布到 registry
```

### `wau task`

Manage tasks.

```bash
wau task submit "帮我查天气"                 # Submit task
wau task get task_1700000000                # Get task details
wau task list                               # List recent tasks
```

### `wau node` ⭐ 第二刀 (2026-08-20 visa demo 验证)

List and inspect WAU network nodes. In WAU, a "node" = a registered agent instance (each `agent_name` registered via `/registry/agents/register`).

```bash
wau node ls                              # List all online nodes (table)
wau node ls -o json                      # JSON for scripts
wau node ls --addr http://43.134.126.126:18400   # remote
wau node info fox-medical                # Detailed status for one node
wau peer ls                              # Alias (libp2p-style)
```

**Tolerant decoder**: server may return either `{"agents":[...]}` or raw `[...]` — client auto-detects.

### `wau completion` ⭐ 第二刀

Generate shell completion script.

```bash
source <(wau completion bash)                                              # bash → ~/.bashrc
wau completion zsh > "${fpath[1]}/_wau"                                  # zsh
wau completion fish | source                                              # fish
wau completion powershell | Out-String | Invoke-Expression                # powershell
```

### `wau config`

Manage wau-cli configuration.

```bash
wau config init                  # Create config file
wau config validate              # Validate config
wau config show                  # Show current config
wau config show -o json          # JSON format
```

---

## 🎨 Output Formats

All list/get commands support multiple output formats:

```bash
wau agent list -o table    # Default: human-readable table
wau agent list -o json     # JSON (for scripts)
wau agent list -o yaml     # YAML
wau agent list -o csv      # CSV (for spreadsheets)
```

---

## 🔐 RBAC Roles

Specify role with `--role` flag:

| Role | Description | Permissions |
|------|-------------|-------------|
| `kernel_core` | Kernel internal use | All operations |
| `trusted_agent` | Trusted internal agent (TrustScore >= 0.7) | Schedule, read-only |
| `external_agent` | External agent (default) | Submit only |

```bash
wau --role kernel_core agent list
wau --role trusted_agent task submit "..."
```

---

## ⚙️ Configuration

Config file location (searched in order):
1. `--config` flag value
2. `./config.yaml` (current directory)
3. `~/.wau/config.yaml`

Example (`~/.wau/config.yaml`):

```yaml
kernel:
  addr: "http://43.134.126.126:18400"   # 生产服务器(per 2026-08-20 visa demo)
  role: "external_agent"
  timeout: 30s

output:
  format: "table"
  color: true

logging:
  level: "info"
```

**远端访问**:所有 L5 命令支持 `--addr` flag,临时覆盖 `kernel.addr`(per wau-cli root.go L77)。
示例:`wau agent search medical --addr http://43.134.126.126:18401 --universe medical`

---

## 🛠 Development

### Build

```bash
go build -o wau ./cmd/wau
```

### Test

```bash
go test ./...
```

### Project structure

```
wau-cli/
├── cmd/
│   └── wau/                # Entry point
├── internal/
│   ├── cmd/                # Command implementations
│   │   ├── agent/          # `wau agent ...`
│   │   ├── task/           # `wau task ...`
│   │   └── config/         # `wau config ...`
│   ├── client/             # HTTP client for kernel API
│   ├── output/             # Output formatters
│   └── config/             # Config loader
├── go.mod
└── README.md
```

### Tech stack

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Config management
- [tablewriter](https://github.com/olekukonko/tablewriter) - Table output
- [yaml.v3](https://gopkg.in/yaml.v3) - YAML parsing

---

## 📋 Version Compatibility

| wau-cli | WAU-core-kernel |
|---------|-----------------|
| v0.1.0-dev | v0.2.0+ |

---

## 🏷 Version Naming

`wau-cli` uses **SemVer + codename** dual-track versioning, inspired by [HashiCorp](https://github.com/hashicorp) (Terraform/Vault/Consul).

### Current Release

```
v0.1.0 "Genesis" 🎉
```

### Naming Rules

- **Alphabetical order**: Each minor version → one letter (A → B → C → ...)
- **Theme**: Nature / animals / minerals
- **Format**: Single word, capitalized first letter
- **Source**: Inspired by HashiCorp and Ubuntu naming traditions

### Roadmap

| Version | Codename | Theme | Goal |
|---------|----------|-------|------|
| **v0.1.0** | **Genesis** | 创世 | **MVP (current)** ✅ |
| v0.2.0 | Coral | 珊瑚 | Basic features improvement |
| v0.3.0 | Dolphin | 海豚 | Multi-language SDK |
| v0.4.0 | Emerald | 翡翠 | Client maturity |
| v0.5.0 | Falcon | 猎鹰 | TUI + real-time |
| **v1.0.0** | **Phoenix** | 凤凰 | **GA release** 🎯 |
| v1.1.0 | Granite | 花岗岩 | Stability |
| v2.0.0 | Horizon | 地平线 | Orchestration |

### Display Format

```bash
$ wau version
wau-cli v0.1.0 "Genesis"
Official CLI for WAU-core-kernel
```

### Git Tag Convention

```bash
# Standard SemVer
git tag -a v0.1.0 -m "v0.1.0 'Genesis' - MVP release"

# Pre-release
git tag -a v0.2.0-rc1 -m "v0.2.0-rc1 'Coral' - Release candidate 1"
git tag -a v0.2.0-beta1 -m "v0.2.0-beta1 'Coral' - Beta"
```

See [CHANGELOG.md](CHANGELOG.md) for detailed version history.

---

## 📄 License

Apache 2.0 - see [LICENSE](LICENSE)

---

## 🔗 Related Projects

- [WAU-core-kernel](https://github.com/wau/core-kernel) - Core service
- [wau-registry](https://github.com/wau/registry) - Agent registry
- [wau-circuit](https://github.com/wau/circuit) - Circuit breaker
- [wau-intent](https://github.com/wau/intent) - Intent parser
- [wau-scheduler](https://github.com/wau/scheduler) - Scheduler

---

## v0.9.0 "Acorn" 收口段(2026-09-15 GA)

上文详细介绍了 wau-cli 设计 + 子命令 + 与 WAU 产品体系的关系。本段为 v0.9.0 GA 增量补充。

### 角色

| OS 类比 | CLI / DevTool(开发工具)|
|---|---|
| 部署 | 单 binary,本地装,不入生产路径 |
| 通信 | HTTP / gRPC client,调 WAU 产品体系所有仓 |
| 状态 | v0.8.0 GA 已发(2026-07-13)|

### v0.9.0 子命令新增

- `wau trust issue/verify/revoke` — wau-trust 调试
- `wau profile get/upsert` — wau-profile 调试
- `wau intent classify` — wau-intent 调试
- `wau registry register/list` — wau-registry-service 调试
- `wau circuit run` — wau-circuit 跑 circuit 描述
- `wau scheduler submit/stats` — wau-scheduler 调试

### 跨产品体系

`wau-cli` 是**唯一**跟 14 仓全部打交道的工具。每个子命令 = 1 个对应仓的 client 封装。

### v0.9.0 "Acorn" 5 份核心文档

| # | 文件 | 内容 |
|---|---|---|
| 1 | [README.md](README.md)(本文件)| 仓入口 + 子命令总览 + v0.9.0 收口段 |
| 2 | [QUICKSTART.md](QUICKSTART.md) | 15 分钟跑通 5 个常用子命令 |
| 3 | [DEPLOY.md](DEPLOY.md) | 本地装 + 配置 + autocompletion |
| 4 | [ARCHITECTURE.md](ARCHITECTURE.md) | 子命令路由 + client 复用 |
| 5 | [CHANGELOG.md](CHANGELOG.md) | v0.8.0 + v0.9.0 倒序(126 行已存在)|

### 历史锚点

- v0.8.0 GA([[project-v0.8.0-GA-2026-07-13]])
