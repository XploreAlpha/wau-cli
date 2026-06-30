# wau-cli 部署

wau-cli 是**本地工具**,不入生产路径。

## 安装

```bash
# 1. 通过 Homebrew / scoop(未来)
# 2. 通过 `go install`
go install github.com/XploreAlpha/wau-cli/cmd/wau-cli@v0.9.0

# 3. 二进制下载
# 见 README "下载" 段
```

## 多环境切换

```bash
# 默认 ~/.wau/cli.yaml
wau --endpoint prod trust issue
wau --endpoint staging trust issue
wau --endpoint dev trust issue
```

通过 `WAU_ENDPOINT` 环境变量或 `--endpoint` flag 切换:

```bash
export WAU_ENDPOINT=prod
wau trust issue --tenant acme
```

## 凭据

```bash
# 必填
WAU_CLI_TOKEN=$WAU_CLI_TOKEN
```

**所有 token 用 `$VAR` 占位**(per [[feedback-hf-token-leak-2026-06-17]])

## 配置

| 字段 | 默认 | 说明 |
|---|---|---|
| `endpoints.kernel` | `127.0.0.1:18402` | WAU-core-kernel gRPC |
| `endpoints.trust` | `127.0.0.1:18460` | wau-trust gRPC |
| `endpoints.profile` | `127.0.0.1:18480` | wau-profile gRPC |
| `endpoints.intent` | `127.0.0.1:18490` | wau-intent gRPC |
| `token` | (env `WAU_CLI_TOKEN`)| 用户 token |

## 升级路径

- v0.9.0(Acorn)→ v0.8.0(Sprout):
  - 子命令 100% 兼容
  - 配置文件自动迁移
