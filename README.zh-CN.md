# wau-cli

[English](README.md) | [中文](README.zh-CN.md)

> **[WAU-core-kernel](https://github.com/wau/core-kernel) 的官方命令行客户端** —— WAU 操作系统级控制平面(类比 `apt` / `kubectl` / `docker` / `aws-cli`)。

[![Version](https://img.shields.io/badge/version-v1.0.1-blue)](https://github.com/XploreAlpha/wau-cli/releases/tag/v1.0.1)
[![Release](https://img.shields.io/badge/release-Iris-orange)](CHANGELOG.md)
[![Next](https://img.shields.io/badge/next-v1.1.0_Jade-yellow)](CHANGELOG.md)
[![Go](https://img.shields.io/badge/go-1.23+-00ADD8)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-green)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](https://github.com/XploreAlpha/wau-cli)

**当前版本**:v1.0.1 **"Iris"** —— OS CLI 化 GA(2026-08-24)🌷
**下一版本**:v1.1.0 **"Jade"** —— manual-test ready(2026-08-24)

版本历史见 [CHANGELOG.md](CHANGELOG.md)。

---

## 📑 目录

1. [项目定位](#1-项目定位)
2. [核心特性](#2-核心特性)
3. [安装](#3-安装)
4. [快速开始](#4-快速开始)
5. [子命令](#5-子命令)
6. [输出格式](#6-输出格式)
7. [配置](#7-配置)
8. [路线图](#8-路线图)
9. [开发与测试](#9-开发与测试)
10. [许可证](#10-许可证)

---

## 1. 项目定位

`wau-cli` 提供 `kubectl` / `docker` 风格的体验来管理 WAU 服务 —— 包括 agent、task、kernel,以及 9 服务的本地 stack。它是 **WAU 操作系统的单一入口**,横跨 **应用层**、**开发者层**、**系统层** 和 **网络层**。单一二进制,零运行时依赖,D60 additive 设计。

---

## 2. 核心特性

| 特性 | 描述 |
|------|------|
| 🩺 **健康检查** | 快速验证 kernel 健康状态 |
| 🤖 **Agent 管理** | 列出、查询、注册、注销 agent |
| 📋 **Task 管理** | 提交、查询、列出 task |
| ⚙️ **配置管理** | 初始化、验证、显示配置 |
| 📊 **多种输出格式** | 支持表格、JSON、YAML、CSV |
| 🔐 **RBAC 支持** | 多角色权限控制 |
| 🚀 **单二进制** | 无运行时依赖 |
| 🐳 **Stack 全生命周期** | `wau stack up/down/logs/restart` —— 类 docker-compose |
| 🔑 **认证(JWT 4 段声明)** | `wau auth login/whoami/logout` |
| 📈 **集群总览** | `wau cluster status/agents` |

---

## 3. 安装

### 3.1 源码安装

```bash
git clone https://github.com/wau/wau-cli
cd wau-cli
go build -o wau ./cmd/wau
mv wau /usr/local/bin/
```

### 3.2 预编译二进制(即将发布)

```bash
# macOS
brew install wau

# Linux
curl -fsSL https://wau.dev/install.sh | sh
```

---

## 4. 快速开始

```bash
# 1. 初始化配置
wau config init

# 2. 检查 kernel 健康
wau health

# 3. 列出所有在线 agent
wau agent list

# 4. 注册新 agent
wau agent register \
  --name fox-medical \
  --url http://100.125.99.209:18800 \
  --skills medical,clinical

# 5. 提交 task
wau task submit "帮我查一下天气"

# 6. 获取 task 详情
wau task get task_1700000000
```

---

## 5. 子命令

### 5.1 `wau stack` —— 本地 stack 全生命周期

> ⭐ v1.0.0 第一刀(2026-08-20)

管理本地 WAU stack —— 启停服务、检查状态、重启。类比单节点 WAU 部署的 `docker compose` / `kubectl`。

```bash
# 架构可视化
wau stack up --dry-run                # 打印 10-service 启动 plan
wau stack up --demo --dry-run         # 同上,显式 demo profile

# 真起(需 9 binary 已装到 ~/.wau/bin)
wau stack up --demo                   # 起 9 服务 + 等 health check
wau stack up --profile minimal --detach  # 只起 redis+core+registry,后台

# 看状态
wau stack ls                          # 表格输出
wau stack ls -o json                  # JSON 给脚本
wau stack ls -o yaml                  # YAML
wau stack status                      # ls 的 alias
wau stack ps                          # ls 的 alias

# 关停
wau stack down                        # SIGTERM → 5s → SIGKILL
wau stack down --all                  # 关停 + 清 runtime state
wau stack down --force                # 强杀(即使失败服务)

# 自定义 stack 文件
wau stack up --file /path/to/wau-stack.yml
```

**内置 default 9-service stack**(per 19 仓真实架构):
`redis + wau-core + registry + wau-store + wau-intent + wau-profile + wau-llm-router + wau-edge + wau-channel + wau-agent`

**Profiles**:
- `demo` —— 9 服务全起(visa 拍板 demo profile)
- `minimal` —— 只起 redis + wau-core + registry(debug 用)

---

### 5.2 `wau log` / `wau stack logs` —— 日志查看

> ⭐ v1.0.1 P4.1(2026-08-24)

显示 stack 服务的最近日志或跟踪日志。类比 `docker logs` / `kubectl logs` / `journalctl -u`。

```bash
# 单服务
wau log wau-core                  # 最后 50 行
wau log wau-core --follow         # tail -F
wau log wau-core --lines 200      # 最后 200 行(0 = 全部)
wau log wau-core --grep ERROR     # 正则过滤
wau log wau-core --since 5m       # 最近 5 分钟
wau log wau-core --no-color       # 关彩色

# 多服务 fanout(每服务带颜色前缀)
wau stack logs                    # 所有 loggable 服务并行
wau stack logs wau-core           # 单服务(同 `wau log`)
wau stack logs --follow           # 全部 tail -F
wau stack logs --grep "ERROR|panic"
```

**Flags**: `--follow/-f` · `--lines/-n` · `--grep` · `--since` · `--no-color`

---

### 5.3 `wau stack init-configs` —— 配置初始化

> ⭐ v1.0.1 P4.2 + P4.5(2026-08-24)

把内嵌的服务配置模板写到 `~/.wau/configs/`,解决"缺少 configs/*.yaml"的入门阻塞。

```bash
# 默认:写所有 4 个服务 config
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

# env 替换(visa demo / 本地 dev)
wau stack init-configs --envsubst --force
```

**4 个内嵌服务**:

| Service | 配置文件名 | 用途 |
|---------|-----------------|---------|
| `wau-store` | `store.yaml` | 存储(postgres + redis + admin) |
| `wau-llm-router` | `router.yaml` | LLM 路由(Thompson sampling) |
| `wau-edge` | `edge.yaml` | Edge 入口(WS / OpenAI compat) |
| `wau-channel` | `channel.yaml` | IM 通道(Telegram / Discord / Slack / Feishu / DingTalk / QQ / Email / Webhook) |

**Flags**: `--service` · `--output-dir`(默认 `~/.wau/configs`)· `--force` · `--dry-run` · `--envsubst`

**行为**:
- 文件已存在 → 默认 skip(提示 `--force`)
- `--force` → overwrite
- `--dry-run` → 只 print,不写
- atomic write(`.tmp` + `rename`,无 partial file)
- **默认不展开** env placeholder(如 `$WAU_STORE_PG_DSN`)—— 由部署层脚本替换(per D55 SOP)
- **`--envsubst`** —— 写文件前用 `os.ExpandEnv` 替换 `$VAR`(visa demo / 本地 dev)

**典型 workflow**:
- 本地 dev / visa demo → 用 `--envsubst`(快)
- 生产部署 → 不传 `--envsubst`,保留 `$VAR` 字面值,让 `wau-deploy` 脚本替换

---

### 5.4 `wau stack restart` —— 重启

> ⭐ v1.0.1 P4.4(2026-08-24)

重启服务 —— `down <svc>` + `up <svc>` 的便捷组合。类比 `docker compose restart` / `kubectl rollout restart`。

```bash
wau stack restart                       # 全栈(topo 反序 down + 正序 up)
wau stack restart wau-core              # 单服务
wau stack restart wau-core wau-router   # 多服务
wau stack restart --wait-max 120s       # 自定义 health probe 超时
wau stack restart --file my.yml         # 自定义 stack file
wau stack reload wau-edge               # alias 同样工作
```

**Flags**:
- `--file <path>` —— 自定义 wau-stack.yml
- `--profile <name>` —— 应用 profile 过滤
- `--wait-max <duration>` —— health probe 超时(默认 60s)

**Exit codes**:
- `0` —— 全部成功
- `1` —— 有 service start 失败(进程没起来)
- `2` —— 全部 start 成功但 health check 没通过 / 有 service stop 失败

---

### 5.5 `wau auth` —— 认证

> ⭐ v1.0.1 P4.3(2026-08-24)

管理 WAU 用户认证。类比 `docker login` / `npm login` / `kubectl auth whoami`。

```bash
# 登录(交互式)
wau auth login
# Username: alice
# Password: ********

# 登录(非交互式,脚本用)
wau auth login --user alice --password s3cret

# 自定义 endpoint
wau auth login --endpoint http://localhost:18400

# 不存盘(测试用)
wau auth login --no-store

# 当前用户
wau auth whoami
# User:        alice
# Expires:     2026-08-25 14:30:00 +08:00 (in 24h)
# Endpoint:    http://localhost:18400
# Token:       eyJhbGciOiJIUzI1NiIs...

# 登出(删 ~/.wau/credentials)
wau auth logout
```

**子命令**: `login` · `logout` · `whoami`

**Flags (login)**: `--user` · `--password` · `--endpoint` · `--no-store`

**凭证存储** `~/.wau/credentials`(mode 0600),JWT 4 段声明格式(per D66=B):`sub` / `exp` / `iat` / `role`

---

### 5.6 `wau cluster` —— 集群总览

> ⭐ v1.0.1 P4.6(2026-08-24)

集群总览 —— 把 `/health` + `/kernel/info` + `/registry/agents` 三个 endpoint 组合成统一视图。类比 `kubectl cluster-info` / `docker system info`。

```bash
# 集群状态(并发调 3 endpoint)
wau cluster status                              # 本地 kernel
wau cluster status --addr http://43.134.126.126:18400  # 远程 server
wau cluster status --json                       # JSON 输出

# 集群 agent 列表
wau cluster agents
wau cluster agents --skill multi_agent
wau cluster agents --status online --json
```

**Flags**:
- `status` —— `--json` / `--timeout`(默认 10s per endpoint)
- `agents` —— `--json` / `--page` / `--page-size` / `--skill` / `--status` / `--search`

**Exit codes**:
- `0` —— 3 endpoint 全成功
- `1` —— 3 endpoint 全 fail(kernel unreachable)
- `2` —— partial(至少 1 个 OK,其它 fail)—— 标 ⚠

---

### 5.7 标准子命令

#### `wau health` / `wau kernel`

```bash
wau health                  # 简单检查
wau health --wait           # 等到 healthy
wau health --wait --timeout 60s
wau health --addr http://43.134.126.126:18400   # 远端

wau kernel info             # 详细信息
wau kernel version          # 版本
```

#### `wau agent`

```bash
wau agent list                              # 列出所有
wau agent list --page 2 --page-size 50      # 分页
wau agent list --skill medical              # 按 skill 过滤
wau agent list --status online              # 按状态过滤
wau agent list --search fox                 # 按名字搜索
wau agent get fox                           # 详情
wau agent score fox                         # 评分
wau agent register --name fox --url ...     # 注册
wau agent deregister fox                    # 注销

# L5 包管理器(类 apt)
wau agent search medical --universe medical        # 类 apt search
wau agent install fox-medical                       # 类 apt install
wau agent install fox-medical --version=1.2.3      # 锁版本
wau agent update                                    # 全更新
wau agent update fox-medical                        # 单独更新
wau agent uninstall fox-medical                     # 卸载
wau agent uninstall fox-medical --purge             # 全删
wau agent login                                      # 登入 registry
wau agent publish --from ./weather-bot              # 发布
```

#### `wau task`

```bash
wau task submit "帮我查天气"                 # 提交 task
wau task get task_1700000000                # 详情
wau task list                               # 列出最近
```

#### `wau node` / `wau peer`(libp2p 风格)

```bash
wau node ls                              # 列出在线 node
wau node ls -o json                      # JSON 给脚本
wau node ls --addr http://43.134.126.126:18400   # 远端
wau node info fox-medical                # 单 node 详情
wau peer ls                              # Alias (libp2p-style)
```

#### `wau completion`

```bash
source <(wau completion bash)                                              # bash → ~/.bashrc
wau completion zsh > "${fpath[1]}/_wau"                                  # zsh
wau completion fish | source                                              # fish
wau completion powershell | Out-String | Invoke-Expression                # powershell
```

#### `wau config`

```bash
wau config init                  # 创建配置
wau config validate              # 验证
wau config show                  # 显示当前
wau config show -o json          # JSON 格式
```

---

## 6. 输出格式

所有 list/get 命令支持多种输出格式:

```bash
wau agent list -o table    # 默认:人类可读表格
wau agent list -o json     # JSON(给脚本)
wau agent list -o yaml     # YAML
wau agent list -o csv      # CSV(给电子表格)
```

---

## 7. 配置

### 7.1 配置文件位置

按以下顺序查找:
1. `--config` flag 值
2. `./config.yaml`(当前目录)
3. `~/.wau/config.yaml`

### 7.2 示例(`~/.wau/config.yaml`)

```yaml
kernel:
  addr: "http://43.134.126.126:18400"   # 生产服务器
  role: "external_agent"
  timeout: 30s

output:
  format: "table"
  color: true

logging:
  level: "info"
```

### 7.3 通过 `--addr` 远端访问

所有 L5 命令支持 `--addr` flag,临时覆盖 `kernel.addr`。

```bash
wau agent search medical --addr http://43.134.126.126:18401 --universe medical
```

### 7.4 RBAC 角色

| Role | 描述 | 权限 |
|------|------|------|
| `kernel_core` | 内核内部使用 | 全部操作 |
| `trusted_agent` | 可信内部 agent(TrustScore ≥ 0.7) | Schedule, read-only |
| `external_agent` | 外部 agent(默认) | 只能提交 |

```bash
wau --role kernel_core agent list
wau --role trusted_agent task submit "..."
```

---

## 8. 路线图

`wau-cli` 采用 **SemVer + 代号** 双轨版本,灵感来自 [HashiCorp](https://github.com/hashicorp)(Terraform / Vault / Consul)。

### 8.1 当前版本

```
v1.0.1 "Iris" 🌷 —— OS CLI 化 GA(2026-08-24)
```

### 8.2 命名规则

- **字母顺序**:A → B → C → ...
- **主题**:自然 / 动物 / 矿物
- **格式**:单词,首字母大写

### 8.3 版本历史

| Version | 代号 | 主题 | 状态 |
|---------|----------|------|---------|
| v0.1.0 | Genesis | 创世 | MVP released |
| v0.9.0 | Acorn | 橡子 | Pre-OS CLI |
| v1.0.0 | Phoenix | 凤凰 | Pre-GA |
| **v1.0.1** | **Iris** | **鸢尾花** | **OS CLI 化 GA ✅(2026-08-24)** |
| **v1.1.0** | **Jade** | **翡翠** | **manual-test ready ✅(2026-08-24)** |
| v1.1.x | (TBD) | (TBD) | Post-GA patches |
| v1.2.0 | (TBD) | (TBD) | ISO 镜像发布 |
| v1.3.0 | (TBD) | (TBD) | K8s Helm/Operator |
| v2.0.0 | Horizon | 地平线 | Orchestration |

### 8.4 显示格式

```bash
$ wau version
wau-cli v1.0.1 "Iris"
Official CLI for WAU-core-kernel
```

### 8.5 Git Tag 规范

```bash
# Standard SemVer
git tag -a v1.0.1 -m "v1.0.1 'Iris' - OS CLI 化 GA"

# Pre-release
git tag -a v1.1.0-rc1 -m "v1.1.0-rc1 'Jade' - Release candidate 1"
```

详细版本历史见 [CHANGELOG.md](CHANGELOG.md)。

---

## 9. 开发与测试

### 9.1 构建

```bash
go build -o wau ./cmd/wau
```

### 9.2 测试

```bash
go test ./...
```

### 9.3 项目结构

```
wau-cli/
├── cmd/
│   └── wau/                # 入口
├── internal/
│   ├── cmd/                # 子命令实现
│   │   ├── agent/          # `wau agent ...`
│   │   ├── task/           # `wau task ...`
│   │   ├── stack/          # `wau stack ...`
│   │   └── config/         # `wau config ...`
│   ├── client/             # HTTP client for kernel API
│   ├── output/             # 输出格式化
│   └── config/             # 配置加载
├── go.mod
└── README.md
```

### 9.4 技术栈

- [Cobra](https://github.com/spf13/cobra) —— CLI framework
- [Viper](https://github.com/spf13/viper) —— 配置管理
- [tablewriter](https://github.com/olekukonko/tablewriter) —— 表格输出
- [yaml.v3](https://gopkg.in/yaml.v3) —— YAML 解析

### 9.5 版本兼容性

| wau-cli | WAU-core-kernel |
|---------|-----------------|
| v1.0.1 "Iris" | v0.5.1+ |
| v1.1.0 "Jade" | v0.7.0+ |

---

## 10. 许可证

Apache 2.0 —— 见 [LICENSE](LICENSE)

---

## 🔗 相关项目

`wau-cli` 是**唯一**与 14 个 WAU 仓全部打交道的工具。每个子命令 = 1 个对应仓的 client 封装。

- [WAU-core-kernel](https://github.com/wau/core-kernel) —— 内核服务
- [wau-registry](https://github.com/wau/registry) —— Agent 注册中心
- [wau-circuit](https://github.com/wau/circuit) —— 熔断器
- [wau-intent](https://github.com/wau/intent) —— 意图解析器
- [wau-scheduler](https://github.com/wau/scheduler) —— 调度器
- [wau-cli](https://github.com/wau/wau-cli) —— 本项目