# wau-cli

[English](README.md) | [中文](README.zh-CN.md)

> **Official command-line client for [WAU-core-kernel](https://github.com/wau/core-kernel)** — the WAU OS-level control plane (analogous to `apt` / `kubectl` / `docker` / `aws-cli`).

[![Version](https://img.shields.io/badge/version-v1.0.1-blue)](https://github.com/XploreAlpha/wau-cli/releases/tag/v1.0.1)
[![Release](https://img.shields.io/badge/release-Iris-orange)](CHANGELOG.md)
[![Next](https://img.shields.io/badge/next-v1.1.0_Jade-yellow)](CHANGELOG.md)
[![Go](https://img.shields.io/badge/go-1.23+-00ADD8)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-green)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](https://github.com/XploreAlpha/wau-cli)

**Current Release**: v1.0.1 **"Iris"** — OS CLI 化 GA (2026-08-24) 🌷
**Next Release**: v1.1.0 **"Jade"** — manual-test ready (2026-08-24)

See [CHANGELOG.md](CHANGELOG.md) for version history.

---

## 📑 Table of Contents

1. [What is wau-cli](#1-what-is-wau-cli)
2. [Features](#2-features)
3. [Installation](#3-installation)
4. [Quick Start](#4-quick-start)
5. [Commands](#5-commands)
6. [Output Formats](#6-output-formats)
7. [Configuration](#7-configuration)
8. [Roadmap](#8-roadmap)
9. [Development](#9-development)
10. [License](#10-license)

---

## 1. What is wau-cli

`wau-cli` provides a `kubectl` / `docker` experience for managing WAU services — agents, tasks, kernels, and the 9-service local stack. It is **the single entry point** for the WAU OS, spanning the **Application**, **Developer**, **System**, and **Network** layers. One binary, zero runtime dependencies, D60-additive design.

---

## 2. Features

| Feature | Description |
|---------|-------------|
| 🩺 **Health check** | Quickly verify kernel health status |
| 🤖 **Agent management** | List, get, register, deregister agents |
| 📋 **Task management** | Submit, query, list tasks |
| ⚙️ **Config management** | Initialize, validate, show configuration |
| 📊 **Multiple output formats** | Table, JSON, YAML, CSV |
| 🔐 **RBAC support** | Multiple role levels |
| 🚀 **Single binary** | No runtime dependencies |
| 🐳 **Stack lifecycle** | `wau stack up/down/logs/restart` — docker-compose-like |
| 🔑 **Auth (JWT 4-claim)** | `wau auth login/whoami/logout` |
| 📈 **Cluster overview** | `wau cluster status/agents` |

---

## 3. Installation

### 3.1 From source

```bash
git clone https://github.com/wau/wau-cli
cd wau-cli
go build -o wau ./cmd/wau
mv wau /usr/local/bin/
```

### 3.2 Pre-built binaries (coming soon)

```bash
# macOS
brew install wau

# Linux
curl -fsSL https://wau.dev/install.sh | sh
```

---

## 4. Quick Start

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
wau task submit "Help me check the weather"

# 6. Get task details
wau task get task_1700000000
```

---

## 5. Commands

### 5.1 `wau stack` — Local Stack Lifecycle

> ⭐ v1.0.0 First Cut (2026-08-20)

Manage the local WAU stack — bring services up, tear them down, inspect status, and restart. Equivalent to `docker compose` / `kubectl` for a single-node WAU deployment.

```bash
# Architecture visualization
wau stack up --dry-run                # Print 10-service startup plan
wau stack up --demo --dry-run         # Same, explicit demo profile

# Real startup (requires 9 binaries installed to ~/.wau/bin)
wau stack up --demo                   # Start 9 services + wait for health check
wau stack up --profile minimal --detach  # Only redis+core+registry, detached

# Status
wau stack ls                          # table output
wau stack ls -o json                  # JSON for scripts
wau stack ls -o yaml                  # YAML
wau stack status                      # alias for ls
wau stack ps                          # alias for ls

# Shutdown
wau stack down                        # SIGTERM → 5s → SIGKILL
wau stack down --all                  # Stop + clear runtime state
wau stack down --force                # Force kill (even failed services)

# Custom stack file
wau stack up --file /path/to/wau-stack.yml
```

**Built-in default 9-service stack** (per 19-repo architecture):
`redis + wau-core + registry + wau-store + wau-intent + wau-profile + wau-llm-router + wau-edge + wau-channel + wau-agent`

**Profiles**:
- `demo` — All 9 services (visa demo profile)
- `minimal` — Only redis + wau-core + registry (debug)

---

### 5.2 `wau log` / `wau stack logs` — Logs

> ⭐ v1.0.1 P4.1 (2026-08-24)

Show recent or follow logs for stack services. Equivalent to `docker logs` / `kubectl logs` / `journalctl -u`.

```bash
# Single service
wau log wau-core                  # Last 50 lines
wau log wau-core --follow         # tail -F
wau log wau-core --lines 200      # Last 200 lines (0 = all)
wau log wau-core --grep ERROR     # Regex filter
wau log wau-core --since 5m       # Last 5 minutes
wau log wau-core --no-color       # Disable color

# Multi-service fanout (color prefix per service)
wau stack logs                    # All loggable services in parallel
wau stack logs wau-core           # Single service (same as `wau log`)
wau stack logs --follow           # All tail -F
wau stack logs --grep "ERROR|panic"
```

**Flags**: `--follow/-f` · `--lines/-n` · `--grep` · `--since` · `--no-color`

---

### 5.3 `wau stack init-configs` — Config Bootstrap

> ⭐ v1.0.1 P4.2 + P4.5 (2026-08-24)

Write embedded service config templates to `~/.wau/configs/`. Solves the "missing configs/*.yaml" onboarding blocker.

```bash
# Default: write all 4 service configs
wau stack init-configs

# Single service
wau stack init-configs --service wau-store
wau stack init-configs --service wau-llm-router   # alias: --service router

# Custom output dir
wau stack init-configs --output-dir /etc/wau/configs

# Overwrite existing
wau stack init-configs --force

# Plan only
wau stack init-configs --dry-run

# Env substitution (visa demo / local dev)
wau stack init-configs --envsubst --force
```

**4 embedded services**:

| Service | Config filename | Purpose |
|---------|-----------------|---------|
| `wau-store` | `store.yaml` | Storage (postgres + redis + admin) |
| `wau-llm-router` | `router.yaml` | LLM routing (Thompson sampling) |
| `wau-edge` | `edge.yaml` | Edge gateway (WS / OpenAI compat) |
| `wau-channel` | `channel.yaml` | IM channels (Telegram / Discord / Slack / Feishu / DingTalk / QQ / Email / Webhook) |

**Flags**: `--service` · `--output-dir` (default `~/.wau/configs`) · `--force` · `--dry-run` · `--envsubst`

**Behavior**:
- File exists → skip by default (提示 `--force`)
- `--force` → overwrite
- `--dry-run` → only print
- Atomic write (`.tmp` + `rename`, no partial files)
- **No env expansion by default** (deployment script replaces `$VAR` per D55 SOP)
- **`--envsubst`** → expand `$VAR` via `os.ExpandEnv` (visa demo / local dev)

**Typical workflow**:
- Local dev / visa demo → use `--envsubst` (fast)
- Production deployment → no `--envsubst` (let `wau-deploy` script replace)

---

### 5.4 `wau stack restart` — Restart

> ⭐ v1.0.1 P4.4 (2026-08-24)

Restart services — convenience combination of `down <svc>` + `up <svc>`. Analogous to `docker compose restart` / `kubectl rollout restart`.

```bash
wau stack restart                       # Full stack (topo reverse down + forward up)
wau stack restart wau-core              # Single service
wau stack restart wau-core wau-router   # Multiple services
wau stack restart --wait-max 120s       # Custom health probe timeout
wau stack restart --file my.yml         # Custom stack file
wau stack reload wau-edge               # alias works the same
```

**Flags**: `--file` · `--profile` · `--wait-max` (default 60s)

**Exit codes**:
- `0` — All succeeded
- `1` — Some service start failed
- `2` — All started but health check failed

---

### 5.5 `wau auth` — Authentication

> ⭐ v1.0.1 P4.3 (2026-08-24)

Manage WAU user authentication. Equivalent to `docker login` / `npm login` / `kubectl auth whoami`.

```bash
# Login (interactive)
wau auth login
# Username: alice
# Password: ********

# Login (non-interactive, for scripts)
wau auth login --user alice --password s3cret

# Custom endpoint
wau auth login --endpoint http://localhost:18400

# Don't persist (testing)
wau auth login --no-store

# Current user
wau auth whoami
# User:        alice
# Expires:     2026-08-25 14:30:00 +08:00 (in 24h)
# Endpoint:    http://localhost:18400
# Token:       eyJhbGciOiJIUzI1NiIs...

# Logout (delete ~/.wau/credentials)
wau auth logout
```

**Subcommands**: `login` · `logout` · `whoami`

**Flags (login)**: `--user` · `--password` · `--endpoint` · `--no-store`

**Credential storage** `~/.wau/credentials` (mode 0600), JWT 4-claim format (per D66=B): `sub` / `exp` / `iat` / `role`

---

### 5.6 `wau cluster` — Cluster Overview

> ⭐ v1.0.1 P4.6 (2026-08-24)

Cluster overview — composes `/health` + `/kernel/info` + `/registry/agents` into a unified view. Analogous to `kubectl cluster-info` / `docker system info`.

```bash
# Cluster status (3 endpoints concurrent)
wau cluster status                              # Local kernel
wau cluster status --addr http://43.134.126.126:18400  # Remote server
wau cluster status --json                       # JSON output

# Cluster agent list
wau cluster agents
wau cluster agents --skill multi_agent
wau cluster agents --status online --json
```

**Flags**:
- `status` — `--json` / `--timeout` (default 10s per endpoint)
- `agents` — `--json` / `--page` / `--page-size` / `--skill` / `--status` / `--search`

**Exit codes**:
- `0` — All 3 endpoints OK
- `1` — All 3 failed (kernel unreachable)
- `2` — Partial (at least 1 OK, others failed)

---

### 5.7 Standard Commands

#### `wau health` / `wau kernel`

```bash
wau health                  # Simple check
wau health --wait           # Wait for healthy
wau health --wait --timeout 60s
wau health --addr http://43.134.126.126:18400   # Remote

wau kernel info             # Detailed info
wau kernel version          # Version
```

#### `wau agent`

```bash
wau agent list                              # List all
wau agent list --page 2 --page-size 50      # Pagination
wau agent list --skill medical              # Filter by skill
wau agent list --status online              # Filter by status
wau agent list --search fox                 # Search by name
wau agent get fox                           # Get details
wau agent score fox                         # Get score
wau agent register --name fox --url ...       # Register
wau agent deregister fox                    # Deregister

# L5 package manager (apt-like)
wau agent search medical --universe medical        # apt search
wau agent install fox-medical                       # apt install
wau agent install fox-medical --version=1.2.3      # Pin version
wau agent update                                    # Update all
wau agent update fox-medical                        # Update one
wau agent uninstall fox-medical                     # Uninstall
wau agent uninstall fox-medical --purge             # Full delete
wau agent login                                      # Login registry
wau agent publish --from ./weather-bot              # Publish
```

#### `wau task`

```bash
wau task submit "Help me check the weather"   # Submit task
wau task get task_1700000000                  # Get details
wau task list                                 # List recent
```

#### `wau node` / `wau peer` (libp2p-style)

```bash
wau node ls                              # List online nodes
wau node ls -o json                      # JSON for scripts
wau node ls --addr http://43.134.126.126:18400   # Remote
wau node info fox-medical                # Detailed status for one node
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
wau config init                  # Create config file
wau config validate              # Validate
wau config show                  # Show current
wau config show -o json          # JSON format
```

---

## 6. Output Formats

All list/get commands support multiple output formats:

```bash
wau agent list -o table    # Default: human-readable table
wau agent list -o json     # JSON (for scripts)
wau agent list -o yaml     # YAML
wau agent list -o csv      # CSV (for spreadsheets)
```

---

## 7. Configuration

### 7.1 Config file location

Searched in order:
1. `--config` flag value
2. `./config.yaml` (current directory)
3. `~/.wau/config.yaml`

### 7.2 Example (`~/.wau/config.yaml`)

```yaml
kernel:
  addr: "http://43.134.126.126:18400"   # Production server
  role: "external_agent"
  timeout: 30s

output:
  format: "table"
  color: true

logging:
  level: "info"
```

### 7.3 Remote access via `--addr`

All L5 commands support `--addr` flag to override `kernel.addr` temporarily.

```bash
wau agent search medical --addr http://43.134.126.126:18401 --universe medical
```

### 7.4 RBAC Roles

| Role | Description | Permissions |
|------|-------------|-------------|
| `kernel_core` | Kernel internal use | All operations |
| `trusted_agent` | Trusted internal agent (TrustScore ≥ 0.7) | Schedule, read-only |
| `external_agent` | External agent (default) | Submit only |

```bash
wau --role kernel_core agent list
wau --role trusted_agent task submit "..."
```

---

## 8. Roadmap

`wau-cli` uses **SemVer + codename** dual-track versioning, inspired by [HashiCorp](https://github.com/hashicorp) (Terraform / Vault / Consul).

### 8.1 Current Release

```
v1.0.1 "Iris" 🌷 — OS CLI 化 GA (2026-08-24)
```

### 8.2 Naming Rules

- **Alphabetical order**: A → B → C → ...
- **Theme**: Nature / animals / minerals
- **Format**: Single word, capitalized first letter

### 8.3 Version History

| Version | Codename | Theme | Status |
|---------|----------|-------|--------|
| v0.1.0 | Genesis | Genesis | MVP released |
| v0.9.0 | Acorn | Acorn | Pre-OS CLI |
| v1.0.0 | Phoenix | Phoenix | Pre-GA |
| **v1.0.1** | **Iris** | **Iris** | **OS CLI 化 GA ✅ (2026-08-24)** |
| **v1.1.0** | **Jade** | **Jade** | **manual-test ready ✅ (2026-08-24)** |
| v1.1.x | (TBD) | (TBD) | Post-GA patches |
| v1.2.0 | (TBD) | (TBD) | ISO image release |
| v1.3.0 | (TBD) | (TBD) | K8s Helm/Operator |
| v2.0.0 | Horizon | Horizon | Orchestration |

### 8.4 Display Format

```bash
$ wau version
wau-cli v1.0.1 "Iris"
Official CLI for WAU-core-kernel
```

### 8.5 Git Tag Convention

```bash
# Standard SemVer
git tag -a v1.0.1 -m "v1.0.1 'Iris' - OS CLI 化 GA"

# Pre-release
git tag -a v1.1.0-rc1 -m "v1.1.0-rc1 'Jade' - Release candidate 1"
```

See [CHANGELOG.md](CHANGELOG.md) for detailed version history.

---

## 9. Development

### 9.1 Build

```bash
go build -o wau ./cmd/wau
```

### 9.2 Test

```bash
go test ./...
```

### 9.3 Project Structure

```
wau-cli/
├── cmd/
│   └── wau/                # Entry point
├── internal/
│   ├── cmd/                # Command implementations
│   │   ├── agent/          # `wau agent ...`
│   │   ├── task/           # `wau task ...`
│   │   ├── stack/          # `wau stack ...`
│   │   └── config/         # `wau config ...`
│   ├── client/             # HTTP client for kernel API
│   ├── output/             # Output formatters
│   └── config/             # Config loader
├── go.mod
└── README.md
```

### 9.4 Tech Stack

- [Cobra](https://github.com/spf13/cobra) — CLI framework
- [Viper](https://github.com/spf13/viper) — Config management
- [tablewriter](https://github.com/olekukonko/tablewriter) — Table output
- [yaml.v3](https://gopkg.in/yaml.v3) — YAML parsing

### 9.5 Version Compatibility

| wau-cli | WAU-core-kernel |
|---------|-----------------|
| v1.0.1 "Iris" | v0.5.1+ |
| v1.1.0 "Jade" | v0.7.0+ |

---

## 10. License

Apache 2.0 — see [LICENSE](LICENSE)

---

## 🔗 Related Projects

`wau-cli` is **the only** tool that talks to all 14 WAU repos. Each subcommand wraps a corresponding repo's HTTP/gRPC client.

- [WAU-core-kernel](https://github.com/wau/core-kernel) — Core service
- [wau-registry](https://github.com/wau/registry) — Agent registry
- [wau-circuit](https://github.com/wau/circuit) — Circuit breaker
- [wau-intent](https://github.com/wau/intent) — Intent parser
- [wau-scheduler](https://github.com/wau/scheduler) — Scheduler
- [wau-cli](https://github.com/wau/wau-cli) — This project