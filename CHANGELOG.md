## [v0.9.0] - 2026-07-02 (v0.9.0 GA)

### Highlights

- v0.9.0 同步发版 + 文档补全(4 新文档 + CHANGELOG)
- 详见 GA 收口报告:~/WAU-develop/develop-log/kernel/v0.9.0/wrapup/2026-07-02-PROGRESS-v0.9.0-GA-CLOSURE.md

### Compatibility

- API 100% 保留
- 4 SDK 同步 v1.2.0

## [Unreleased] — v1.0.0 "Phoenix" M11 P4.5 ⭐L5 包管理器 CLI (2026-07-10, per D72/D73/D74)

### Added

- **5 新 subcommand** under `wau agent`:
  - `wau agent install <name>`     装 agent(类比 apt install / npm install)
  - `wau agent uninstall <name>`   卸 agent(`--purge` 全删)
  - `wau agent update [<name>]`    更新 agent(无 name = 全更新,`--version` 锁版)
  - `wau agent search <query>`     搜 wau-registry(`--universe` / `--limit` 过滤)
  - `wau agent login`              登入拿 token(落 `~/.wau/credentials` per D74)
- **L5 HTTP client** `internal/client/l5.go`(~150 LoC):5 method + 5 request/response struct
- **wau agent help 文档**:`agent.go` `Long` 字段加 5 新 subcommand 描述

### Compatibility (D60 additive)

- 老 6 subcommand(list/get/register/deregister/score/publish)0 改
- 老 client `internal/client/*.go` 0 改
- 走 WAU-core-kernel `/v1/l5/*` HTTP API(单仓 +0 整合 per D72)

### Reference

- D72 A 拍板 (wau-toolkit v2.0 OS-level, 仓数 +0):[stage1/01-D66-D74-9-decisions-summary#七](https://github.com/wau-network/WAU-develop/blob/main/develop-log/kernel/v1.0.0/stage1/01-D66-D74-9-decisions-summary.md)
- 设计 doc:[stage1/04-wau-toolkit-v2.0-OS-level-design.md](https://github.com/wau-network/WAU-develop/blob/main/develop-log/kernel/v1.0.0/stage1/04-wau-toolkit-v2.0-OS-level-design.md)

# Changelog

All notable changes to wau-cli will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Release Naming Convention

Each minor version has a codename following alphabetical order (A → B → C → ...).
Codenames use natural/animals/minerals themes, inspired by HashiCorp's naming style.

| Version | Codename   | Theme          |
|---------|------------|----------------|
| v0.1.0  | Genesis    | 创世           |
| v0.2.0  | Coral      | 珊瑚           |
| v0.3.0  | Dolphin    | 海豚           |
| v0.4.0  | Emerald    | 翡翠           |
| v0.5.0  | Falcon     | 猎鹰           |
| v0.6.0  | Falcon...  | ...            |
| v1.0.0  | Phoenix    | 凤凰（GA）     |
| v1.1.0  | Granite    | 花岗岩         |
| v2.0.0  | Horizon    | 地平线         |

---

## [v0.1.0] "Genesis" - 2026-06-02

### 🎉 First MVP Release

This is the initial MVP release of wau-cli, the official command-line client for WAU-core-kernel.

### Added

- **Core commands**:
  - `wau health` - Health check (with `--wait` support for CI/CD)
  - `wau kernel info` - Show kernel information
  - `wau kernel version` - Show kernel version
  - `wau version` - Show wau-cli version

- **Agent management** (`wau agent`):
  - `list` - List agents (with pagination, filtering, search)
  - `get` - Get agent details
  - `register` - Register a new agent
  - `deregister` - Remove an agent
  - `score` - Get 15-dimension agent score

- **Task management** (`wau task`):
  - `submit` - Submit a new task
  - `get` - Get task details
  - `list` - List recent tasks (placeholder)

- **Config management** (`wau config`):
  - `init` - Generate default config file
  - `validate` - Validate configuration
  - `show` - Show current configuration

- **Output formats**: Table, JSON, YAML, CSV

- **RBAC support**: kernel_core, trusted_agent, external_agent roles

- **Configuration**: Viper-based with YAML/ENV/flags support

### Tech Stack

- [Cobra](https://github.com/spf13/cobra) v1.10.2
- [Viper](https://github.com/spf13/viper) v1.21.0
- [tablewriter](https://github.com/olekukonko/tablewriter) v1.1.4
- [yaml.v3](https://gopkg.in/yaml.v3) v3.0.1

### Compatibility

- **WAU-core-kernel**: v0.2.0+
- **Go**: 1.22+

---

## [Unreleased]

### Planned for v0.2.0 "Coral"

- [ ] Shell autocompletion (bash/zsh/fish)
- [ ] Installation script (curl one-liner)
- [ ] Homebrew formula
- [ ] Improved error messages
- [ ] More robust retry logic
- [ ] Connection pooling

### Planned for v0.3.0 "Dolphin"

- [ ] OpenAPI 3.0 spec generation
- [ ] Python SDK
- [ ] TypeScript SDK
- [ ] `wau task watch` real-time monitoring
- [ ] Better task list endpoint integration

### Planned for v0.4.0 "Emerald"

- [ ] TUI mode (Bubble Tea)
- [ ] Interactive search (`/`)
- [ ] Multi-column sorting
- [ ] Offline cache

### Planned for v1.0.0 "Phoenix" - GA

- [ ] gRPC protocol support
- [ ] Plugin framework
- [ ] Rust SDK
- [ ] Java SDK
- [ ] Stable API guarantee

---

## Version History

| Version | Codename  | Date       | Status   |
|---------|-----------|------------|----------|
| v0.1.0  | Genesis   | 2026-06-02 | ✅ Current |
| v0.2.0  | Coral     | TBD        | 📋 Planned |
| v0.3.0  | Dolphin   | TBD        | 📋 Planned |
| v1.0.0  | Phoenix   | TBD        | 🎯 GA Goal |

---

[unreleased]: https://github.com/wau/wau-cli/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/wau/wau-cli/releases/tag/v0.1.0
