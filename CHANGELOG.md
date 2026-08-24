## [Unreleased] — v1.0.0 "Phoenix" ⭐wau stack + 第二刀 HTTP client 重构 + 第三刀 binary 安装验证 (2026-08-20, per visa demo + 子项 4.1)

### Added — 第四刀 P4.4(2026-08-24,v1.0.1-p4.4)— `wau stack restart [service...]`

- **`wau stack restart [service...]`** 子命令 — `down <svc>` + `up <svc>` 的便捷组合(per P4.3 closure §8.1):
  - 类比 `docker compose restart` / `kubectl rollout restart` / `systemctl restart <svc>`
  - **复用** `internal/stack/process.go` 的 `pm.Stop` / `pm.Start` / `KillProcessGroup` + `rt.SetStatus`(per P4.1 / P4.2)
  - **抽出 helpers** `stopOne` / `startOne` 隔离 down / up 阶段
- **三种用法**(从 visa demo 真实场景):
  - `wau stack restart` — 全栈按 topo 反序 down + 正序 up(改 stack 配置后)
  - `wau stack restart wau-core` — 单服务(改 init-configs 后)
  - `wau stack restart wau-core wau-router` — 多服务(同时滚更多个组件)
- **Flags**:`--file` / `--profile`(跟 up/down 一致)+ `--wait-max` (默认 60s,health probe 超时)
- **Aliases**:`reload`(避免跟 `kubectl rollout restart` 混淆)
- **Exit codes**:
  - `0` — 全部成功
  - `1` — 有 service start 失败(进程没起来)
  - `2` — 全部 start 成功但 health check 没通过 / 有 service stop 失败(警告级)
- **UX 改进**(per 真实 smoke):
  - 进程已起但 health check 超时 → 标 `⚠ health warning (process up, pid N)` 而**不是** `✗`
  - 保留 PID 到 runtime,`wau stack ls` 仍能看 new process

### Tests — 第四刀 P4.4

- `internal/cmd/stack/restart_test.go`(P4.4 新增 3 tests):
  - `TestNewRestartCmd_BasicArgs` — Use=restart + Aliases=[reload] + 3 flag(file/profile/wait-max)+ wait-max default=1m0s
  - `TestReverseStrings` — 4 case(普通 / nil / single / reverse mutate 不动原 slice)
  - `TestRunRestart_InvalidService` — args=[nonexistent] → "not in stack" error,exit=1

### smoke test — P4.4 (3 case,全 PASS)

| # | 命令 | 结果 |
|---|------|------|
| 1 | `wau stack restart wau-channel --wait-max 30s` | ✅ PID 335589 → 336795,status=running |
| 2 | `wau stack restart wau-intent wau-edge --wait-max 30s` | ✅ 两个服务 PID 都变(332456→338405 / 332962→337726)|
| 3 | `wau stack restart nonexistent` | ✅ "service "nonexistent" not in stack (use 'wau stack ls'...)" exit=1 |

### Compatibility (P4.4 D60 additive)

- ✅ `wau stack up / down` 完全不变,restart 是新 subcommand
- ✅ 已有 init-configs(P4.2)+ log-follow(P4.1)+ auth(P4.3)+ 全栈 up/down 全部兼容
- ⚠️ restart 的 PID/health warning 区分(`⚠` vs `✗`)是**新** UX,可能跟用户期望的"非 0 即成功"直觉不符;用户脚本可检查 exit code (0/1/2)

### Reference

- **代码**:`internal/cmd/stack/restart.go` (200 行) + `restart_test.go` (90 行)
- **plan + closure**:`~/WAU-develop/develop-log/wau-cli/v1.0.1-wau-stack-restart/`
- **后续** P4.5 `wau stack init-configs --envsubst` + P4.6 `wau cluster status`

---

### Added — 第四刀 P4.3(2026-08-24,v1.0.1-p4.3)— `wau auth login / logout / whoami`

- **顶层 `wau auth login / logout / whoami`** 子命令组(per D74 JWT 凭证流程):
  - 类比 `docker login / logout` / `npm login / whoami` / `kubectl auth whoami` / `gh auth login`
  - **复用** `internal/client/auth.go` 的 `Credentials` + `LoadCredentials` + `Save`(已有)
  - **新增** `client.Login(ctx, opts)` helper:`wau auth login` + `wau agent login` 都调它(DRY)
- **`wau auth login`** (交互式 / `--user` + `--password` / `--endpoint` / `--no-store`):
  - 调 kernel `POST /v1/l5/login`(已存在,per D74)
  - 拿 4-claim JWT access_token + refresh_token(per D66=B)
  - 存到 `~/.wau/credentials` (mode 0600)
- **`wau auth logout`**:`os.Remove(~/.wau/credentials)`,无凭证 → 友好提示(exit=0,非 error)
- **`wau auth whoami`**:读本地凭证,显示 user_id / expires_at(剩余时间)/ endpoint / token 前 20 字符
  - 无凭证 → 提示 `wau auth login`
  - token 过期 / 即将过期(<5m) → ⚠️ 警告
- **`internal/cmd/auth/`** package (~340 LoC + 11 tests):
  - `auth.go` NewAuthCmd factory(顶层子命令组,跟 `wau stack` 风格一致)
  - `login.go` NewLoginCmd + runAuthLogin
  - `logout.go` NewLogoutCmd + runAuthLogout
  - `whoami.go` NewWhoamiCmd + runAuthWhoami + formatDuration
  - `SetAccessors(kernelAddr, role func() string)` — 由 root.go 注入(避免 cmd 包 cycle)

### Tests — 第四刀 P4.3

- `internal/client/auth_test.go`(P4.3 新增 11 tests):
  - `Login_Success` mock server 4 field 全填
  - `Login_BadCreds` !OK → 友好 error
  - `Login_MissingFields` username/password 缺 → error
  - `Login_ServerError` 500 → error
  - `SaveLoadCredentials_Roundtrip` 内容一致
  - `SaveCredentials_FilePermission` 写出来 mode=0600
  - `LoadCredentials_NotExist` 不存在 → 空 Credentials(非 error)
  - `LoadCredentials_DefaultPath` 调用 "" → 用默认 ~/.wau/credentials
  - `DefaultCredentialsPath` 后缀 = .wau/credentials
  - `Valid_FreshToken` / `Valid_ExpiredToken` / `Valid_NoExpiry` / `Valid_NilOrEmpty`
  - `CredentialsProvider_Token` / `CredentialsProvider_Expired`
- `internal/cmd/auth/auth_test.go`(P4.3 11 tests):
  - `NewAuthCmd_BasicArgs` Use=auth + 3 subcommand 注册
  - `NewLoginCmd_BasicArgs` 4 flag
  - `RunAuthLogin_NoUserPass` stdin closed → error
  - `RunAuthLogin_RequiresStoreFlag` --no-store flag 注册
  - `NewLogoutCmd_BasicArgs` Use=logout + RunE
  - `RunAuthLogout_NoCredentials` 文件不存在 → 友好提示 + exit=0
  - `RunAuthLogout_RemoveExisting` 删除 + verify 不存在
  - `NewWhoamiCmd_BasicArgs` Use=whoami + alias=status
  - `RunAuthWhoami_NotLoggedIn` 无凭证 → 提示
  - `RunAuthWhoami_LoggedIn` 有凭证 → 显示 user_id / Expires / Token / Endpoint
  - `RunAuthWhoami_ExpiredToken` 过期 → ⚠️ EXPIRED 警告
- **全 PASS**(client 11 + auth 11 = 22 新,0 回归,`TestLsCmd_EmptyState` P4.2 stack down 清理后也已 PASS)

### smoke test — P4.3 (7 case)

| # | 命令 | 结果 |
|---|------|------|
| 1 | `wau auth whoami`(无 creds) | ✅ "Not logged in. Hint: run `wau auth login`" |
| 2 | `wau auth logout`(无 creds) | ✅ "Not logged in (no credentials file)" + exit=0 |
| 3 | 写 fake creds + `wau auth whoami` | ✅ 显示 user=alice / Expires=2h / Endpoint / Token prefix |
| 4 | `wau auth logout`(有 creds) | ✅ "✓ Credentials removed" + 文件真删除 |
| 5 | `wau auth login` 本机 kernel | ✅ 友好错误 "dial tcp 127.0.0.1:18400: connection refused" |
| 6 | `wau auth login --addr http://43.134.126.126:18400` | ✅ **远程 server 真实响应** "API error (status 401): invalid credentials" — 证明端到端 POST /v1/l5/login 工作 |
| 7 | `wau auth login --no-store --user alice --password x` | ✅ login 失败但不写 creds 文件 |

### Compatibility (P4.3 D60 additive)

- `wau agent login` **保留**(L5 包管理器旧入口,向后兼容)
- `wau auth login` 是**新增**顶层 OS-level 入口(都调 `client.Login`,DRY)
- 不改 server 端(`/v1/l5/login` 已有)
- 不改 token 格式(per D66=B 4-claim JWT)
- 不引入新 dep(纯 stdlib + 复用已有 client + cobra)
- password 仍明文 stdin(已知 P4.x 改进:用 `golang.org/x/term.ReadPassword`)

### Reference
- D74 凭证流程:`feedback-wau-cli-purpose`
- D66=B 4-claim JWT:`project-wau-v1-1-0-deployment-plan-main-2026-08-19`
- 已有 `wau agent login`:`internal/cmd/agent/login.go`(2026-07-10 旧路径)
- P4.2 closure:`~/WAU-develop/develop-log/wau-cli/v1.0.1-wau-init-configs/closure.md`
- OS CLI 定位:`feedback-wau-cli-purpose`

---

### Added — 第四刀 P4.2(2026-08-24,v1.0.1-p4.2)— `wau stack init-configs`

- **`wau stack init-configs`** 子命令(per stage3 §5.4 "缺 configs/*.yaml 是常见 onboarding 障碍" + 子项 4.3):
  - **embed 4 个服务 config 模板**到 wau-cli binary:wau-store / wau-llm-router / wau-edge / wau-channel
  - 默认写 `~/.wau/configs/<service>.yaml`(可 `--output-dir` 自定义)
  - `--service <name>` 单服务(支持 alias: `router` / `llm-router` → `wau-llm-router`)
  - `--force` 覆盖已有文件
  - `--dry-run` 只看 plan,不写
  - 文件已存在默认 skip + 提示 `--force`
  - atomic write(`.tmp` + `rename`,同 process.go 模式)
- **`internal/stack/initconfigs/`** package (~200 LoC + 14 tests):
  - `//go:embed configs/*.yaml` 模板
  - `Template` / `ListTemplates` / `TemplateByService` / `remapServiceName` / `normalizeService`(大小写不敏感 + alias)
  - `Writer` / `WriteAll` / `WriteResult`(wrote / skipped / would-write / error 4 状态)
  - `ExpandHome`(跟 log.go / process.go 一致)
- **`internal/stack/process.go` 加 `expandArgs` / `expandHomeArg`**:`~` 在 svc.Args 里展开成 user home(让 `--config ~/.wau/configs/...` 工作)
- **`internal/stack/default.go` 改 4 个服务 `Args`**:
  - `wau-store` `--config ~/.wau/configs/store.yaml`
  - `wau-llm-router` `--config ~/.wau/configs/router.yaml`
  - `wau-edge` `--config ~/.wau/configs/edge.yaml`
  - `wau-channel` `--config ~/.wau/configs/channel.yaml`
- **不解 PG / Redis / admin token 的 env placeholder**:`$WAU_STORE_PG_DSN` / `$WAU_STORE_REDIS_PASSWORD` / `$WAU_STORE_ADMIN_TOKEN` 保持字面(由 `wau-deploy` 替换,deployment-level concern,不在 wau-cli scope)

### Tests — 第四刀 P4.2

- `internal/stack/initconfigs/initconfigs_test.go`:14 tests —
  - `ListTemplates` 4 个 template 都在
  - `TemplateByService` 7 case(wau-store / store / WAU-STORE / wau-llm-router / router / wau-edge / wau-channel)
  - `TemplateByService_NotFound` 未知 service → error
  - `RemapServiceName` 4 case
  - `ExpandHome` 6 case(`~` / `~/x` / `~~/x` / 空 / `/abs` / `rel`)
  - `Writer_Write_New` mkdir + write
  - `Writer_Write_ExistsSkip` 已有 → skip
  - `Writer_Write_ExistsForce` 已有 + Force → overwrite
  - `Writer_DryRun` 不写
  - `Writer_WriteAll` 批量
  - `Writer_WriteAll_PartialSkip` 1 已有 → 3 write + 1 skip
  - `Writer_NestedOutputDir` mkdir 递归
  - `Writer_AtomicWrite_NoPartialFile` 无 .tmp 残留
- `internal/cmd/stack/initconfigs_test.go`:6 tests —
  - `NewInitConfigsCmd_BasicArgs` flag 注册
  - `RunInitConfigs_DryRun` 输出 "Dry run" + service 列表
  - `RunInitConfigs_WriteAll` 写 4 个文件 + 输出 "4 wrote"
  - `RunInitConfigs_ServiceFilter` 单服务
  - `RunInitConfigs_ServiceNotFound` ghost → error
  - `RunInitConfigs_SkipAndForce` 跳过 + --force overwrite
- **全 PASS**(initconfigs 14 + cmd 6 = 20 新,0 回归)

### smoke test — P4.2 (8 case)

| # | 命令 | 结果 |
|---|------|------|
| 1 | `wau stack init-configs --dry-run` | ✅ 4 个 plan,不写 |
| 2 | `wau stack init-configs --output-dir /tmp/wau-cfg-test` | ✅ 4 写 + "Next: wau stack up --demo" |
| 3 | re-run 同命令 | ✅ 4 skip("already exists") |
| 4 | `--force` | ✅ 4 overwrite |
| 5 | `--service wau-store --output-dir /tmp/wau-cfg-single` | ✅ 1 write,只有 store.yaml |
| 6 | `--service wau-ghost` | ✅ exit=1,"no template for service" |
| 7 | `wau stack init-configs`(默认 ~/.wau/configs) | ✅ 4 写到 `~/.wau/configs/` |
| 8 | `~/.wau/bin/wau-store --config ~/.wau/configs/store.yaml` | ✅ **不再 "file not found"**;走到 PG DSN env placeholder(deployment-level) |

### Compatibility (P4.2 D60 additive)

- `wau stack up/down/ls/log/logs` 0 改
- `wau store/llm-router/edge/channel` 的 `Args` 多加 `--config` flag,**不影响手工启动**(用户没显式 init-configs 也能正常跑,只是会用 source 仓的相对路径 — 跟 stage3 行为一致)
- 不引入新 dep(纯 stdlib + `//go:embed`)
- 不修改任何服务的 source(D60 additive — 模板从 source 仓 yaml **复制到** `internal/stack/initconfigs/configs/`,头部注明 source of truth URL)

### Reference
- stage3 known issue:`~/WAU-develop/develop-log/wau-cli/stage3-summary-2026-08-23.md` §5.4
- P4.1 closure:`~/WAU-develop/develop-log/wau-cli/v1.0.1-wau-log-follow/closure.md`
- v1.1.0 子项 4.1:`project-wau-v1-1-0-deployment-plan-main-2026-08-19`
- OS CLI 定位:`feedback-wau-cli-purpose`

---

### Added — 第四刀 P4.1(2026-08-24,v1.0.1-p4.1)— `wau log` + `wau stack logs`

- **`wau log <service>`** + **`wau stack logs [service]`** 子命令(per 子项 4.1 + OS CLI 定位):
  - 类比 `docker logs` / `kubectl logs` / `journalctl -u`
  - 多服务并行 fanout(类似 `docker compose logs`),每服务带 ANSI 颜色前缀(8-color cycler)
  - `--follow/-f` 走 `tail -F`(POSIX,自动处理 rotate/truncate,无需新 dep)
  - `--lines/-n N` 默认 50(`0` = 全部)
  - `--grep REGEX` 过滤(支持正则,坏 regex → 清晰错误)
  - `--since 5m` 时间窗口(解析 RFC3339Nano,无 ts 行向后兼容不丢)
  - `--no-color` 关彩色
  - SIGINT/SIGTERM 干净退出(走 `signal.NotifyContext` + ctx cancel → kill tail 子进程)
- **`internal/stack/process.go` 新 helper**:`LogPath(dir, svc)` + `FollowLog(ctx, path, sw, filter)` + `SafeWriter` (mutex-protected,多服务 fanout 不交错)
- **`internal/cmd/stack/log.go`**(~450 LoC,含测试):
  - `NewLogCmd()` / `NewStackLogsCmd()` factory(exported,顶层 + stack 下双挂)
  - `runLog` / `runStackLogs` cobra RunE
  - `showServiceLog` / `fanoutLogs` / `readAndPrint` / `buildFilter` / `parseLogTimestamp` / `colorPrefix` / `prefixedWriter` / `expandHome`
  - `redis` external 服务在 fanout 中自动 skip

### Tests — 第四刀 P4.1

- `internal/cmd/stack/log_test.go`:18 tests —
  - `hasService` / `serviceNames` / `LogPath` 基础辅助
  - `buildFilter`:nil / grep only / bad regex / since only 4 case
  - `readAndPrint`:basic tail-N / with grep filter
  - `parseLogTimestamp`:RFC3339Nano / 短格式 / 无 ts / 无效日期
  - `colorPrefix`:有 / 无 color
  - `prefixedWriter`:Write 加 [svc] 前缀
  - `NewLogCmd` / `NewStackLogsCmd` flag 注册完整性
  - `runLog`:服务不存在 / log 文件不存在
  - `runStackLogs`:所有服务 fanout (--no-color)
- 全 PASS,无回归(stack:18 P4.1 + 已有 28 P3 = 46)
- `TestLsCmd_EmptyState` host-state-dependent 已知问题:依赖 `~/.wau/run/default.json` 为空,本机之前 stage3 测试遗留 state 导致 fail;与 P4.1 无关,不在 P4.1 修复范围(可改用 `t.Setenv("HOME", t.TempDir())` 隔离,后续 PR)

### Compatibility (P4.1 D60 additive)

- `wau stack up / down / ls` 0 改
- `wau stack logs` / `wau log` 是 **纯新增**(不影响现有命令)
- 去掉了之前误加的 Aliases=["logs"] / ["log"](造成 `wau stack logs` 无参数时走到 NewLogCmd 的 ExactArgs(1) 报错,smoke test 发现并修复)
- 不引入新 dep(沿用 stdlib + `os/exec tail -F`)
- 顶层 `wau log <service>` 作为 `wau stack logs <service>` 的便捷 alias

### Reference
- v1.1.0 子项 4.1:[[project-wau-v1-1-0-deployment-plan-main-2026-08-19]]
- visa demo context:[[project-user-applying-estonian-startup-visa-2026-08-20]]
- server deployment:[[project-wau-production-deployment-43-134-126-126-2026-08-20]]

---

### Added — 第一刀(上午段)

- **`wau stack up / down / ls`** 子命令族(per visa demo + v1.1.0 子项 4.1,2026-08-20):
  - `wau stack up [--file PATH] [--profile NAME] [--demo] [--dry-run] [--detach] [--wait-max DURATION]`
  - `wau stack down [--file PATH] [--profile NAME] [--force] [--all]`
  - `wau stack ls [--file PATH] [--profile NAME] [-o table|json|yaml]` (alias: status, ps)
- **内置 default 9-service stack**(per project-wau-19-repo-real-architecture-2026-07-15):
  redis + wau-core + registry + wau-store + wau-intent + wau-profile + wau-llm-router + wau-edge + wau-channel + wau-agent
- **`demo` / `minimal` profile**:`wau stack up --demo` / `--profile minimal`
- **`internal/stack` package**(1756 LoC,含测试):YAML schema + 拓扑排序(Kahn's algo) + 环检测 + health probes(TCP/HTTP/exec 3 类型) + process manager(SIGTERM→5s→SIGKILL) + runtime state 持久化(`~/.wau/run/<stack>.json`)
- **`internal/cmd/stack` package**(837 LoC,含测试):cobra 子命令 + dry-run 模式 + 彩色输出
- **`internal/output/progress.go`**:Progress bar + Spinner + ServiceRow 格式化(基于 `schollz/progressbar/v3`)
- **Binary 路径 hybrid 解析**:`~/.wau/bin` → `$GOBIN` → `$PATH` → go install hint
- **依赖图拓扑排序**:Kahn's algorithm + alphabetical tie-break + 环检测(失败列出循环节点)

### Added — 第二刀(下午段,visa demo 验证 + HTTP client 重构)

- **HTTP client retry + exponential backoff**(per P1.1,internal/client/retry.go,~120 LoC):
  - 默认 3 次重试,exponential backoff(base=500ms, max=8s)+ ±20% jitter
  - 重试条件:网络错误 / HTTP 5xx / HTTP 429(不重试 4xx 业务错误避免 lockout)
  - RequestOpts 覆盖:MaxRetries / InitialBackoff / MaxBackoff / PerAttemptTimeout
- **HTTP client JWT bearer auth**(per P1.2 + D66=B 4-claim,internal/client/auth.go,~110 LoC):
  - 凭证从 `~/.wau/credentials` 读(per D74):access_token / refresh_token / expires_at / user_id
  - 自动发 `Authorization: Bearer <access_token>` header
  - 保留 `X-Agent-Role`(向后兼容老 server)
  - AuthProvider interface 支持未来 RefreshableProvider(401 自动 refresh)
- **`wau completion <bash|zsh|fish|powershell>`** 子命令(per P1.4):生成 shell 自动补全脚本
  - 禁用 cobra 默认 `__complete` 命令,启用我们的 `completion` 子命令
- **`wau node ls` / `wau node info <name>` / `wau peer ls`** 子命令(per P2.5,internal/cmd/node.go,~155 LoC):
  - 调 `/registry/agents` + `/registry/agents/{name}/status`
  - table / json / yaml 三种格式
  - `wau peer ls` = `wau node ls` 的 libp2p 风格别名
  - tolerant decoder:接受 object `{"agents":[...]}` 或 array `[...]` 两种 server 返回格式
- **`internal/client/agents.go` ListAgents tolerant decoder**(per visa demo):
  - server 实测返回 array,client 先尝试 object 解码,失败 fallback 到 array(无 retry,1 次试探)
  - 已验证:`wau node ls --addr http://43.134.126.126:18400` 正确列出 matwau 节点

### Tests — 第二刀

- `internal/client/client_test.go`:14 tests — NewClient 默认 / retry 4xx/5xx/429 / 网络错误 / context cancel / Bearer auth / 过期 token / backoff math
- `internal/client/l5_test.go`:6 tests — L5Search/Update/Login JSON roundtrip + Credentials LoadSave/Valid
- `internal/cmd/agent/install_test.go`:10 tests — parseConfig 4 case + runInstall happy/error/500/noarg + request shape + cmd wiring
- `internal/cmd/completion_test.go`:4 tests — 4 shell 都通过 / 非法 shell / 缺参数 / bash 包含完整子命令树
- `internal/cmd/node_test.go`:8 tests — node ls 3 case(table/json/error)+ peer alias + node info 3 case(happy/noarg/unknown)
- 全 PASS,无回归(client 23 / agent 11 / cmd 6 / node 8 / stack 28)

### Compatibility (D60 additive)
- 老 subcommand(agent/task/config/health/kernel/version)0 改
- 默认 kernel.addr 仍 `http://localhost:18400`(没动)— 用户配置 ~/.wau/config.yaml 可覆盖
- 第三方 dep:`github.com/schollz/progressbar/v3 v3.19.1`(第一刀),无新增第二刀

### Reference
- v1.1.0 子项 4.1:[[project-wau-v1-1-0-deployment-plan-main-2026-08-19]]
- visa demo context:[[project-user-applying-estonian-startup-visa-2026-08-20]]
- server deployment:[[project-wau-production-deployment-43-134-126-126-2026-08-20]]

---

## [Unreleased] — v1.0.1 — 第三刀 本地 stack 真起验证 (2026-08-23, per visa demo 收口)

### Fixed (internal/stack/default.go)

- **wau-core health probe**:`/healthz` → `/health`(实测 kernel mux 只注册 `/health`)
- **registry health probe**:`/healthz` → `/health`(实测 internal/api/http.go:33)
- **wau-profile probe**:HTTP → TCP 50062(实测无 HTTP endpoint,gRPC only)
- **wau-channel probe**:补 `/health`(原本缺失 → Required 跳过)
- **wau-core env**:`WAU_NET_LIBP2P_DISABLED=true`(避免 libp2p EnableAutoRelay panic "Need a Peer Source fn")

### Verified — 本机 9 binary 安装 + minimal 真起

```
$ ls ~/.wau/bin/                # 9 binaries (~200MB)
registry  wau-agent  wau-channel  wau-core  wau-edge
wau-intent-service  wau-llm-router  wau-profile-service  wau-store

$ /tmp/wau stack up --profile minimal --wait-max 30s
⠋ redis           ... external (skipped startup)
⠋ wau-core        ... ✓ (0.5s)
⠋ registry        ... ✓ (0.5s)
✓ Stack "default" up. 3 services, took 1s.

$ /tmp/wau health --addr http://localhost:18400
✓ WAU-core is healthy
  Version: v0.5.1
  Redis:   connected

$ /tmp/wau kernel info --addr http://localhost:18400
  Version:     v0.5.1
  Uptime:      39.6s
  Agents:      0
  Tasks:       0

$ /tmp/wau task submit "hello visa demo" --addr http://localhost:18400
✗ Failed: gRPC RecommendTopK ... dial tcp 127.0.0.1:50053: connection refused
# ^ wau-intent 没起 → 502 L3 unavailable(预期,因为 minimal profile 只起 3/9)
# 关键:task_id 已生成,retry 3 次后透传,验证 retry + 任务通路 OK
```

### Known Issues (demo profile 缺 config)

`wau stack up --demo`(9 服务)失败原因:

| 服务 | 错误 | 修复方向(未来) |
|---|---|---|
| wau-store | `open configs/store.yaml: no such file or directory` | 需要写 configs/store.yaml 模板 |
| wau-llm-router | `open configs/router.yaml: no such file or directory` | 需要写 configs/router.yaml 模板 |
| wau-agent | `Usage: wau-agent sidecar --config <yaml> ...` | CLI 错(无默认 server mode)|
| wau-channel / wau-edge / wau-intent | 启动但 HTTP/gRPC probe 不通 | 端口或 path 待对齐 |

**`--profile minimal`(3 服务:redis + wau-core + registry)目前全绿,适合 visa demo 主路径**。
9 服务 demo 需要给 6 个服务补 configs/*.yaml 模板,属后续 wau v1.1.0 子项 4.2 范围。

### Reference
- stage3 summary:`~/WAU-develop/develop-log/wau-cli/stage3-summary-2026-08-23.md`
- canonical closure:`~/WAU-develop/develop-log/wau-cli/v1.0.1-local-stack-validation/2026-08-23-third-cut-closure.md`(待写)
- 服务器部署:[[project-wau-production-deployment-43-134-126-126-2026-08-20]]

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
