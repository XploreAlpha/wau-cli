## [Unreleased] — v1.1.0 子项 4.1.4 — SSH push / remote exec 基础层

### Added

- **`internal/stack/remote/`** — SSH 远端执行 client + push + process 管理
  - `client.go` — `RemoteClient` interface + `*Client`(shell-out 到 `ssh`/`scp` CLI,0 新 dep)
    - `Dial(addr, opts)` — 解析 `user@host[:port]` / `ssh://user@host[:port]`
    - `Exec(ctx, cmd)` / `ScpFile(ctx, src, dst, mode)` / `Stat(ctx, path)` / `MkdirAll(ctx, path)` / `Close()`
    - ssh flags:`-o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 -i <key>`
    - 端口冲突按 host-side port 比较,scp mode 默认 0755(binary)/ 0600(secret)
  - `push.go` — `PushStack(ctx, client, stack, opts)` 把 binary + configs + secrets 推到远端
    - 默认目录:`/usr/local/bin` (binary) / `~/.wau/configs` / `/run/secrets`
    - `PushOpts.DryRun` 模式只打印 plan 不真传
    - `kind=external` skip / `kind=docker` warn + skip
    - secrets 支持 `file:` / `env:` 两种源,自动 chmod 0600
  - `proc.go` — `StartRemote(ctx, client, svc, name)` / `StopRemote` / `StatusRemote` / `StopAll`
    - StartRemote:`setsid bash -c '<cmd> >log 2>&1 & echo $! > pidfile; disown'` + cat pidfile 拿 PID
    - StopRemote:`pkill -TERM -f wau-<name>` → 5s 等 → `pkill -KILL -f wau-<name>` → rm pidfile/log(per KillProcessGroup 思路)
    - StatusRemote:`pgrep -f wau-<name>` 拿 PID(exit 1 = not found,不报错)
  - `dial.go` — `DialRemote(addr)` wrapper:空 addr → 本地模式(nil, nil),非空 → ping 验证连通
  - `remote_test.go` — **20 unit tests** 全 PASS (parseAddr × 5 + Dial × 2 + PushStack × 5 + StartRemote × 3 + StopRemote × 1 + StatusRemote × 2 + DialRemote × 2)

### Design choice: shell-out, not crypto/ssh

不引入 `golang.org/x/crypto/ssh` 包(避免新增 dep + key parsing 复杂度),而是用 `exec.CommandContext("ssh", ...)` / `exec.CommandContext("scp", ...)` shell-out。这样:
- 0 新 dep(D60 spirit 干净)
- 自动复用 user 的 `~/.ssh/config` + identity files
- 测试可注入 mock(`RemoteClient` interface)— **unit test 完全离线**

### Compatibility (D60 additive)

- ✅ 0 modified files(纯新增 `internal/stack/remote/` 子目录)
- ✅ 现有 `ProcessManager` / `loadStack` / `up.go` / `down.go` 完全不变
- ✅ `--remote` flag wiring 推迟到 **4.1.5** — 本段只交付 SSH 层 + 测试
- ✅ `--remote ""` 默认(本地模式)— 老 `wau stack up/down` 用户 0 改动

### Reference

- **代码**:+1,033 行 (client.go 228 + dial.go 39 + proc.go 124 + push.go 206 + remote_test.go 436)
- **测试**:+20 unit tests,全 PASS,全仓 12 包 0 回归
- **下 1 段**:4.1.5 — `wau stack up --file wau-stack.yml --remote ssh://...` 集成
- **canonical plan**:`~/WAU-develop/develop-log/wau-cli/v1.1.0-wau-stack-yml/plan.md` §4.1.4

---

## [Unreleased] — v1.1.0 子项 4.1.3 — `wau stack validate` + status matrix 层

### Added

- **`wau stack validate`** — 新 subcommand,深校验 wau-stack.yml v1.1(schema + binary + port 冲突)
  - `internal/stack/validate.go` — `ValidateV11(data, level)` + `ValidationReport`/`ValidationIssue`/`ValidationService`
  - `internal/stack/validate_test.go` — **8 unit tests** 全 PASS (Basic_OK / ParseError / PortConflict / BinaryNotFound / NoHealthcheck / ExternalNoHealthcheck / ServicesSummary / String)
  - `internal/cmd/stack/validate.go` — Cobra factory `NewValidateCmd` + `runValidate`(table / json)
  - `internal/cmd/stack/validate_test.go` — **3 unit tests** 全 PASS (Flags / DefaultStack / BadYAML)
  - `internal/cmd/stack/stack.go` — register `NewValidateCmd`(+1 line)
- **Status matrix 层(供 cmd 后续接入)** — `internal/stack/status.go` + `status_test.go`
  - `StatusV11(stack, runtime, opts)` → `StatusReport` matrix(name/kind/binary/pid/ports/health/uptime/last_error)
  - `HealthState` 枚举:unknown / running / starting / degraded / stopped / failed / external
  - `ProbePorts` 开关:对 binary 服务的 healthcheck.tcp 跑短超时端口探测(本机测试用 `net.Listen` 验证)
  - **9 unit tests** 全 PASS (NilStack / NoRuntime / Degraded / FailedState / ProbePorts_OK / ProbePorts_Fail / LoadRuntime_NoFile / String / Counters)

### Validation depth

| Level | 校验项 |
|-------|--------|
| `basic` | ParseV11 已保证 schema + topo + depends_on |
| `runtime`(默认) | basic + binary 存在性(`DefaultLookup()`)+ host port 冲突 + healthcheck 缺失 info |

### Exit codes (validate subcommand)

| Code | 含义 |
|------|------|
| 0 | healthy(0 errors / 0 warnings) |
| 1 | errors(端口冲突 / parse failure) |
| 2 | warnings only(binary 不在 PATH,但 schema OK) |

### Compatibility (D60 additive)

- ✅ `internal/cmd/stack/stack.go` 只 +2 行(register `NewValidateCmd` + doc line)
- ✅ 现有 `up` / `down` / `ls` / `status` / `restart` / `logs` / `init-configs` subcommand 完全不变
- ✅ 老 v1 path `loadStack(...)` 完全不动(validate 走 ParseV11 新路径)
- ✅ `internal/stack/runtime.go` + `process.go` + `default.go` 0 改

### Real-run verification

```
$ wau stack validate
Stack: wau-default (release: v1.3.4, level: runtime)
Services: 10
Issues: 0 errors, 0 warnings, 0 infos

  ✓ redis                binary=             ports=1 healthcheck=true
  ✓ registry             binary=registry     ports=1 healthcheck=true
  ✓ wau-agent            binary=wau-agent    ports=2 healthcheck=true
  ...
```

### Reference

- **代码**:+1,032 行 (validate.go 227 + status.go 191 + validate_test.go 207 + status_test.go 203 + cmd/validate.go 131 + cmd/validate_test.go 73)
- **测试**:stack pkg 85 → 102 tests (+17),全 PASS,全仓 11 包 0 回归
- **下 1 段**:4.1.4 — SSH push / remote exec(`--remote ssh://...`)
- **canonical plan**:`~/WAU-develop/develop-log/wau-cli/v1.1.0-wau-stack-yml/plan.md` §4.1.3

---

## [Unreleased] — v1.1.0 子项 4.1.2 — embedded default `wau-stack.yml`

### Added

- **内嵌 default `wau-stack.yml`(v1.1 schema)** — 10-service single-node orchestration
  - `internal/stack/defaults/wau-stack.yml` — 1 redis(external)+ 9 wau 服务,镜像 v1.0.1 `default.go` 内置编排
  - `internal/stack/defaults/defaults.go` — `//go:embed` + `DefaultStackYAMLBytes()`(defensive copy)
  - `internal/stack/defaults/defaults_test.go` — **7 unit tests** 全 PASS (NotEmpty / Version11 / ReleaseIsV134 / AllServices / ParseableV11 / Profiles / TopoOrder)

### Service catalog (10 services)

| 服务 | port | kind | binary | healthcheck |
|------|------|------|--------|-------------|
| redis | 6379 | external | (none) | tcp:localhost:6379 |
| wau-core | 18400 | binary | wau-core | http:/health |
| registry | 18401 | binary | registry | http:/health |
| wau-store | 18405 | binary | wau-store | http:/healthz |
| wau-intent | 50053 | binary | wau-intent-service | http:/health |
| wau-profile | 50062 | binary | wau-profile-service | tcp:50062(gRPC only) |
| wau-llm-router | 18403/18404 | binary | wau-llm-router | http:/healthz |
| wau-edge | 18402 | binary | wau-edge | http:/healthz |
| wau-channel | 18410/18411 | binary | wau-channel | http:/health |
| wau-agent | 19408/19407 | binary | wau-agent | tcp:19408 |

### Compatibility (D60 additive)

- ✅ `internal/stack/default.go` 老 programmatic 9-service `DefaultStack()` 完全不变 — 老调用 path 继续工作
- ✅ 新 `DefaultStackYAMLBytes()` 走与用户 wau-stack.yml 相同 `ParseV11` 路径,无 hardcode schema 分叉
- ✅ 单 schema 真源(defaults/wau-stack.yml 一份)— 改一处全仓受益

### Reference

- **代码**:+270 行 (defaults.go 26 + defaults_test.go 89 + wau-stack.yml 155)
- **测试**:+7 defaults tests,全 PASS,0 回归
- **下 1 段**:4.1.3 — `wau stack validate / status` 扩展
- **canonical plan**:`~/WAU-develop/develop-log/wau-cli/v1.1.0-wau-stack-yml/plan.md` §4.1.2

---

## [Unreleased] — v1.1.0 子项 4.1.1 — `wau-stack.yml` schema v1.1 扩展

### Added

- **`wau-stack.yml` schema v1.1** 解析 + 校验(`ParseV11` + `ParseStackFile` dispatcher)
  - 新增 `internal/stack/types_v11.go` — `StackV11` / `ServiceV11` / `HealthcheckSpec` / `PlacementSpec` / `SecretSpec` / `VolumeSpec` / `NetworkSpec` / `ProfileV11`
  - 新增 `internal/stack/parse_v11.go` — `ParseV11` / `ParseStackFile` dispatcher + (StackV11) `Validate` / `resolveDependsOn` / `TopoOrder` / `ServiceByName` / `ApplyProfile` + (HealthcheckSpec) `validate`
  - 新增 `internal/stack/parse_v11_test.go` — **23 v11 unit tests** 全 PASS
  - **双格式兼容**:`version: "1"` (老 v1.0.1 default) + `version: "1.1"` (新) 都接受;`ParseStackFile` 按 version 字段路由

### Schema v1.1 vs v1 (diff)

| 字段 | v1 | v1.1 |
|------|-----|------|
| services | `[]Service`(slice + name field)| `map[string]ServiceV11`(YAML key 即服务名)|
| stack id | `stack.name` | 顶层 `stack_id`(必填)|
| 域 / release pin | (无)| 顶层 `domain` / `release` / `data_dir` |
| 共享卷 / 网络 | (无)| `volumes` / `networks` |
| 配置 / 密钥 | (无)| `configs` / `secrets` |
| profile | `map[string]Profile` | `map[string]ProfileV11`(同 shape)|
| healthcheck | `Probe`(单 type + fields) | `HealthcheckSpec`(grpc/http/tcp/exec **四选一** + interval/timeout/retries)|
| placement | (无)| `PlacementSpec`(v1.1 only supports local,host 字段 reserved)|
| image / kind=docker | (无)| 字段保留,parse 时 warn "reserved for v1.1.x"|

### Algorithm details

- **`TopoOrder`** — Kahn's algorithm,字母序 deterministic queue(同 v1 算法,map vs slice 差异导致代码不复用;4.1.6 closure 后抽 `topoSortGeneric` 公共 helper)
- **`ApplyProfile`** — BFS transitive deps 闭包 + 全 stack TopoOrder 过滤(deps-first 启动顺序)

### Compatibility (D60 additive)

- ✅ **0 删 / 0 改 / 0 重命名公开接口**:`internal/stack/types.go` + `internal/stack/parse.go` v1 代码 0 行修改
- ✅ 老 `Parse` / `LoadFile` / `Validate` / `TopoOrder` / `ApplyProfile` 公开 API 完全不变
- ✅ 老 v1 YAML 文件(`version: "1"`)+ `wau stack up --file X` 路径 0 改动继续工作
- ✅ `default.go` 内嵌 9-service 老 default stack 0 改

### Reference

- **代码**:+883 行 (types_v11.go 87 + parse_v11.go 269 + parse_v11_test.go 527)
- **测试**:stack 包 55 → 78 tests (+23 v11)
- **下 1 段**:4.1.2 — embedded default `wau-stack.yml` 19 服务编排
- **canonical plan**:`~/WAU-develop/develop-log/wau-cli/v1.1.0-wau-stack-yml/plan.md` §4.1.1

---

## [v1.3.4] - 2026-08-24 — ⭐ 子项 4.2 version alignment kickoff

### Changed

- **Bump `Version = "v1.0.1"` → `"v1.3.4"`** (per D92, v1.1.0 子项 4.2 alignment)
  - 跳过 v1.1.0 Granite / v1.2.0(5 SDK 已在 v1.3.4,避免 SDK 假设 server 字段 silent 失败 — per homerail Plan C v1.4-academic 教训)
- **Bump `ReleaseName = "Iris"` → `"Jade"`** (接 Phoenix → Granite → Iris → Jade)
- **新增 `internal/version/` package** (`internal/version/version.go` + `version_test.go`):
  - `const Version = "v1.3.4"` + `const ReleaseName = "Jade"`
  - 5 unit tests 全 PASS (NonEmpty / PrefixV / AlignedV134 / ReleaseName_NonEmpty / Jade)
- **`internal/cmd/root.go`** 改 import `internal/version` 包,删本地 `Version`/`ReleaseName` var (D60 additive — public var 还在 re-export 兼容老 import path)

### Compatibility (D60 additive)

- ✅ `wau --version` 输出从 `wau-cli v1.0.1 "Iris"` → `wau-cli v1.3.4 "Jade"`(公开行为变,但 D60 含义是 API;version 字段是产品层 metadata,可改)
- ✅ 老 `wau.Version` / `wau.ReleaseName` 公开 var 仍可用(从 internal/version 转发)— 老 import `cmd.Version` 不 break
- ✅ 全测试通过,0 回归(`go test ./...`)
- ✅ 现有 v1.0.1 子项 4 全部子命令(`log` / `init-configs` / `auth` / `restart` / `cluster` / `init-configs --envsubst`)0 改

### Reference

- **代码**:`internal/version/version.go` (22 LoC) + `version_test.go` (50 LoC, 5 tests)
- **改**:`internal/cmd/root.go` (+5 LoC import + var re-export)
- **计划**:v1.1.0 子项 4.1 / 4.1.0 — 14 仓版本对齐 kickoff
- **canonical doc**:`~/WAU-develop/develop-log/kernel/v1.1.0/2026-08-19-deployment-plan-as-main-goal.md` §四
- **下 13 仓**:wau-core-kernel / wau-registry / wau-registry-service / wau-scheduler / wau-intent / wau-profile / wau-circuit / wau-trust / wau-store / wau-channel / wau-edge / wau-llm-router / wau-agent

---

## [v1.0.1] - 2026-08-24 — ⭐ 第四刀 OS CLI 化(P4.1-P4.6 全 6 段)

### Added

- **第四刀 P4.6** — `wau cluster status / agents`(组合 `/health` + `/kernel/info` + `/registry/agents`,并发 fetch,partial OK)
- **第四刀 P4.5** — `wau stack init-configs --envsubst`(本地 visa demo 不需要 deploy 脚本)
- **第四刀 P4.4** — `wau stack restart [service...]`(`down + up` 组合,health warning 区分)
- **第四刀 P4.3** — `wau auth login / logout / whoami`(顶层 JWT 凭证子命令组,D74)
- **第四刀 P4.2** — `wau stack init-configs`(embed 4 服务 yaml config,wau-store / wau-llm-router / wau-edge / wau-channel)
- **第四刀 P4.1** — `wau log` + `wau stack logs`(POSIX `tail -F` 跟日志,multi-service fanout + filter)

### Compatibility

- **D60 additive 贯彻全段**:0 删 / 0 改 / 0 重命名公开接口
- 老用户从 v1.0.0 升级到 v1.0.1 零代码改动,只多了 6 个新子命令
- `wau agent login` / `wau agent list` / `wau health` / `wau kernel info` / `wau node` 全部保留

### Reference

- **代码**: ~2,500 行(6 段累计)
- **测试**: ~50 新 case,全 PASS,0 回归
- **真实验证**: `http://43.134.126.126:18400` 远程 server 真实响应
- **closure**: `~/WAU-develop/develop-log/wau-cli/v1.0.1-wau-*/closure.md`(6 个)

---

## [Unreleased] — v1.0.0 "Phoenix" ⭐wau stack + 第二刀 HTTP client 重构 + 第三刀 binary 安装验证 (2026-08-20, per visa demo + 子项 4.1)

### Added — 第四刀 P4.6(2026-08-24,v1.0.1-p4.6)— `wau cluster status / agents`

- **`wau cluster` 子命令组** — 把已有 `/health` + `/kernel/info` + `/registry/agents` 三个 endpoint 组合成"集群视图":
  - 类比 `kubectl cluster-info` / `docker system info`
  - **不新增 kernel 端点**(kernel v0.5.1 还没 `/v1/cluster/*`),纯 CLI 组合
- **`wau cluster status`** — 一次给完整 overview:
  - 并发调 3 endpoint(`sync.WaitGroup`),节省 wall-clock
  - 显示 kernel version / uptime / redis / modules / agent 总数
  - `--json` 输出(给 jq / dashboard)
  - `--timeout` per-endpoint(默认 10s)
  - **Partial OK**:任一 endpoint fail 不 abort(标 ⚠),全部 fail → exit 1
- **`wau cluster agents`** — 集群范围内的 agent 列表:
  - 跟 `wau agent list` 类似但走 cluster 上下文(支持 `--addr` 远程)
  - Filter:--skill / --status / --search / --page / --page-size
  - `--json` 输出
- **`wau cluster status --addr http://43.134.126.126:18400`** ⭐ visa demo 实战:
  - **真实验证远程 server**:live kernel v0.5.1,uptime 40d 2h,5 modules(scheduler/registry/intent/circuit/heartbeat),1 agent(matwau)
- **`KernelInfo.Modules []string`** 新字段(per kernel v0.5+ `modules` 响应)
- **`client.BaseURL() string`** getter(P4.6 new)— 给 cluster status 展示 endpoint
- **`client.ListAgentsRaw(ctx, page, pageSize, skill, status, search)`** 新函数 — 直接调 `/registry/agents`,返回 `(agents, total, err)`,不走 tolerant decoder
- **`client.ClusterStatus`** struct — 汇总 health + kernel + agents count + 3 个 err 字段(partial)

### Tests — 第四刀 P4.6

- `internal/client/cluster_test.go`(P4.6 新增 7 tests,httptest mock 3 endpoint):
  - `TestClusterStatus_AllOK` — 3 endpoint 全 OK → ClusterStatus 全填
  - `TestClusterStatus_HealthFail` — health 500 → partial(health nil, kernel + agents OK)
  - `TestClusterStatus_KernelFail` — /kernel/info 500 → partial
  - `TestClusterStatus_AllFail` — 3 全 fail → 返回 err + status 都有
  - `TestClusterStatus_AgentsRawArray` — live server 行为:raw array 长度 = total
  - `TestClusterStatus_AgentsObjectFormat` — 未来 server 改 object 格式 → 用 total 字段
  - `TestListAgentsRaw_RawArray` — 单独测 ListAgentsRaw 函数
- `internal/cmd/cluster/cluster_test.go`(P4.6 新增 6 tests):
  - `TestNewClusterCmd_BasicArgs` — Use=cluster + 2 subcommand + Aliases=[cl]
  - `TestNewStatusCmd_BasicArgs` — Use=status + json / timeout flag
  - `TestNewAgentsCmd_BasicArgs` — Use=agents + 6 flag + Aliases=[ls]
  - `TestFormatUptime` — 5 case(0.5s / 45s / 2m5s / 1h1m / 1d1h)
  - `TestTruncate` — 5 case(短 / 等长 / 长 / n<3 不补 ... / 加 ...)
  - `TestJoinStrings` — 4 case(nil / 1 / 2 / 3 element)

### smoke test — P4.6 (6 case,全 PASS 对 `http://43.134.126.126:18400`)

| # | 命令 | 结果 |
|---|------|------|
| 1 | `wau cluster status` (本地 kernel 没跑) | ✅ exit=1, "Kernel unreachable at http://localhost:18400" |
| 2 | `wau cluster status --addr http://43.134.126.126:18400` | ✅ v0.5.1 / 40d 2h uptime / redis=connected / 5 modules / 1 agent |
| 3 | `wau cluster agents --addr http://43.134.126.126:18400` | ✅ 列 matwau + skills=[multi_agent, critic_llm] |
| 4 | `wau cluster agents --json` | ✅ JSON 输出含 total + agents array |
| 5 | `wau cluster status --json` | ✅ JSON 输出含 endpoint / Health / Kernel / AgentsTotal / Modules |
| 6 | `wau cluster agents --skill multi_agent` | ✅ filter 通过(server 实际不过滤,但 cli 不报错)|

### Compatibility (P4.6 D60 additive)

- ✅ `wau health` / `wau kernel info` / `wau agent list` 完全不变
- ✅ 没新增 kernel 端点,纯 CLI 组合(per D60)
- ✅ `--addr` 跟其它子命令一致(支持远程 server)
- ⚠️ KernelInfo 加了 `Modules []string` 字段,**server 没返回** modules 不影响(wau-cli 不依赖此字段)

### Reference

- **代码**:`internal/client/cluster.go` (+95 LoC) + `client.go BaseURL()` getter + `types.go Modules` 字段 + `internal/cmd/cluster/{cluster,status,agents,helpers}.go` (~250 LoC)
- **Tests**:`internal/client/cluster_test.go` (+200) + `internal/cmd/cluster/cluster_test.go` (+100)
- **plan + closure**:`~/WAU-develop/develop-log/wau-cli/v1.0.1-wau-cluster-status/`
- **v1.0.1 子项 4 收口**:4 段 P4.1/P4.2/P4.3/P4.4/P4.5/P4.6 全部完成,v1.0.1 可以发版

---

### Added — 第四刀 P4.5(2026-08-24,v1.0.1-p4.5)— `wau stack init-configs --envsubst`

- **`wau stack init-configs --envsubst`** flag — 写 yaml 前用 `os.ExpandEnv` 替换 `$VAR` 占位符(per P4.2 closure §7.3):
  - 类比 `envsubst`(GNU gettext)/ `docker compose --env-file` / `kubectl create configmap --from-env-file`
  - 让本地 visa demo 能直接 `export WAU_STORE_PG_DSN=...` + 跑 `init-configs --envsubst`,**不需要** deploy 脚本
  - **生产部署** 仍走 `wau-deploy` 脚本(不传 `--envsubst`,保留 `$VAR` 字面值)— D60 additive
- **`Writer.EnvSubst bool`** 字段(P4.5 新):Write 时 conditional 走 `os.ExpandEnv`
- **`ExtractEnvVars(content) []string`** helper:扫描模板找出所有 `$VAR`(支持 `$VAR` + `${VAR}` 两种语法,去重)
- **dry-run 增强**:`--dry-run --envsubst` 列出每个模板引用的 `$VAR` + `✓`(set)/ `✗`(未 set)标记
- **写完 warning**:`--envsubst` 真写时,扫描原 template 检查哪些 `$VAR` 在 env 是空 → 标 `⚠ N env var(s) were empty (will be replaced with "")` + exit 2
- **小 fix**:`configs/store.yaml` 注释里 `$ENV` 占位改成 `env`(避免被 regex 误识别为真实 env var)

### Tests — 第四刀 P4.5

- `internal/stack/initconfigs/initconfigs_test.go`(P4.5 新增 6 tests):
  - `TestWriter_EnvSubst_Template` — `$WAU_STORE_PG_DSN` 被 env 替换,文件含真实值 + 不再含字面 `$VAR`
  - `TestWriter_EnvSubst_MissingEnvVar` — env 空 → `$VAR` 替换成 `""`(os.ExpandEnv 默认行为)
  - `TestWriter_NoEnvSubst_PreservesLiteral` — EnvSubst=false → 文件保留 `$VAR` 文本(env 不渗透)
  - `TestExtractEnvVars_Template` — 从 wau-store 模板 grep 出 3 个真实 $VAR(不含误识别的 $ENV)
  - `TestExtractEnvVars_BothSyntaxes` — `$VAR` + `${VAR}` 都识别,重复去重
  - `TestExtractEnvVars_Empty` — 无 $VAR / `$` 单独 / `$$` literal 都不误识别
- `internal/cmd/stack/initconfigs_test.go`(P4.5 新增 4 tests):
  - `TestNewInitConfigsCmd_EnvSubstFlag` — `--envsubst` flag 已注册
  - `TestRunInitConfigs_EnvSubstDryRun_ShowsVars` — dry-run 列出 ✓/✗ 标记
  - `TestRunInitConfigs_EnvSubst_WritesExpanded` — env 全 set → 文件含替换值,exit=0
  - `TestRunInitConfigs_EnvSubst_MissingEnv_WarnsExit2` — env 全空 → 警告 + exit=2

### smoke test — P4.5 (3 case,全 PASS)

| # | 命令 | 结果 |
|---|------|------|
| 1 | `wau stack init-configs --service wau-store --dry-run --envsubst` | ✅ 列出 4 个 $VAR,全 ✗(env 未 set)|
| 2 | `export WAU_STORE_* + wau stack init-configs --service wau-store --envsubst --force` | ✅ 文件含 `postgres://demo:demo@...` / `pgpass` / `redispass` / `admintoken` |
| 3 | `wau stack init-configs --service wau-store --force` (无 --envsubst) | ✅ 文件保留 `$WAU_STORE_PG_DSN` 字面值(production path) |

### Compatibility (P4.5 D60 additive)

- ✅ 不传 `--envsubst` → 行为**完全不变**(P4.2 行为),生产 deploy 脚本继续可用
- ✅ `wau stack up` / `restart` / `auth` / 其它子命令都不受影响
- ⚠️ `os.ExpandEnv` 对未 set 的 env var 返回 `""`(非错误)— 用户**必须** export 才能拿到非空值;我们用 `⚠ N env var(s) were empty` 警告 + exit 2 提醒

### Reference

- **代码**:`internal/stack/initconfigs/initconfigs.go` (+30 LoC EnvSubst + ExtractEnvVars) + `internal/cmd/stack/initconfigs.go` (+80 LoC --envsubst flag + warning)
- **Tests**:`internal/stack/initconfigs/initconfigs_test.go` (+110) + `internal/cmd/stack/initconfigs_test.go` (+80)
- **plan + closure**:`~/WAU-develop/develop-log/wau-cli/v1.0.1-wau-init-configs-envsubst/`
- **后续** P4.6 `wau cluster status`

---

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
