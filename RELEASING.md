# Releasing — wau-cli

> **版本管理 + 发版流程** — 适用于 WAU 仓的版本号规范、tag 命名、CHANGELOG 模板、发布 checklist。

## 1. 版本号规范(SemVer)

WAU 仓采用 [Semantic Versioning 2.0.0](https://semver.org/) 三段式 `MAJOR.MINOR.PATCH`:

| 段 | 含义 | 触发场景 |
|---|---|---|
| **MAJOR** | 不兼容 API 变更 | 删公开接口 / 改老契约 / 重命名(WAU v1.0.0 之后才能有 MAJOR) |
| **MINOR** | 兼容性新增功能 | 新 SDK 方法 / 新平台 adapter / 新 RPC / 新增 endpoint(本期主流) |
| **PATCH** | 兼容性 bug fix | 修测试 flake / 修文档 / 改依赖小版本号 |

**预发版后缀**: `-alpha.N` / `-beta.N` / `-rc.N`(例:`v1.3.0-rc.1`)

**当前最新 tag**: `v0.9.0`

## 2. Tag 命名规范

- **格式**: `v<MAJOR>.<MINOR>.<PATCH>[-预发版后缀]`
- **示例**: `v1.2.0` / `v1.3.0-rc.1` / `v2.0.0-alpha.3`
- **annotated tag**: `git tag -a v1.3.0 -m "v1.3.0 GA"`(必带 -a,触发 GitHub Releases)
- **推送**: `git push origin v1.3.0`(单 tag 推,不批量)
- **删除**: `git tag -d v1.3.0 && git push origin :refs/tags/v1.3.0`(本地 + remote 同步删)

## 3. CHANGELOG.md 模板

每次发版前,`CHANGELOG.md` 顶部插一段(用 `## [vX.Y.Z] - YYYY-MM-DD` 锚定):

```markdown
## [vX.Y.Z] - 2026-MM-DD

### Added
- 新功能 1(per W? ? 子项)
- 新功能 2

### Changed
- 兼容性变更(如有)

### Fixed
- bug fix 1

### Compatibility
- 公开契约 D60 additive: 0 改 / 0 删 / 0 重命名
- 老 SDK 用户无需改动

---
```

**老 CHANGELOG**(Unreleased section)同时回填进 vX.Y.Z 段并加发布日期。

## 4. 发版 checklist(per maintainer)

| # | 步骤 | 命令 / 操作 | 验证 |
|---|---|---|---|
| 1 | 跑测试 + lint | `go test ./...` / `go vet ./...` / `gofmt -l .`(or python/typescript/rust/java equivalent) | 0 fail / 0 vet / 0 fmt diff |
| 2 | 更新 CHANGELOG | 顶部插 vX.Y.Z 段(见 §3 模板) | git diff CHANGELOG.md |
| 3 | 更新 README 版本号 | `[![Version](...vX.Y.Z...)]` badge | 视觉确认 |
| 4 | commit 发版准备 | `git commit -am "chore(release): prepare vX.Y.Z"` | 1 commit |
| 5 | 打 annotated tag | `git tag -a vX.Y.Z -m "vX.Y.Z GA"` | `git tag -l 'vX.Y.Z'` |
| 6 | 推送 commit + tag | `git push origin main && git push origin vX.Y.Z` | GitHub Actions 跑 CI |
| 7 | 等 CI 通过 | (GitHub Actions 自动跑) | ✅ 绿 |
| 8 | GitHub Release | 浏览器 / `gh release create vX.Y.Z --notes "..."` | release page 公开 |
| 9 | 通知 | (per 项目 channel) | Slack / Discord |

## 5. 跨仓同步发版

WAU 仓**全部走 `vX.Y.Z` 一致**(SDK 4 仓 + 核心 6 仓 = 10 仓 SoT):
- **主 SoT**: `WAU-core-kernel` 版本号
- **同步从属**: `wau-channel` / `wau-edge` / `wau-llm-router` / `wau-agent` / `wau-store` / `wau-registry` / `wau-go-sdk` / `wau-python-sdk` / `wau-typescript-sdk` / `wau-rust-sdk`
- **同步机制**: kernel vX.Y.Z → 其他 9 仓当周内完成 SDK + adapter 升级 → 各仓打同号 tag

**v1.0.0 GA 同步**: `WAU-core-kernel v1.0.0` + 6 核心仓 v1.0.0 + 4 SDK v1.0.0(per [[stage2/2026-07-04-v1.0.0-W1-W15-detailed-schedule]] W15 GA)。

## 6. 紧急 hotfix(紧急 patch)

紧急修复不走 PR 流程(per `feedback-no-branches-until-1.0.0` 单分支硬约束),走"main 直接 commit + tag vX.Y.Z+1":

```bash
# 1. 修 hotfix
git commit -am "fix(urgent): ..."

# 2. 打 patch tag
git tag -a vX.Y.Z+1 -m "vX.Y.Z+1 hotfix"

# 3. 推送
git push origin main && git push origin vX.Y.Z+1
```

## 7. 关联

- [CHANGELOG.md](./CHANGELOG.md) — 版本变更历史
- [README.md](./README.md) — 项目主页 + 版本 badge
- [WAU 仓列表](https://github.com/XploreAlpha) — 跨仓同步参考
- WAU Whitepaper § Governance — 版本策略
- `feedback-no-branches-until-1.0.0` — 单分支硬约束(v1.0.0 GA 前无 feature 分支)
