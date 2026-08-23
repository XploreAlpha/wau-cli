# wau-cli 5 分钟跑通(visa demo 友好版)

> 目标:连上 43.134.126.126 服务器上的 wau 集群 + 跑通 5 个常用子命令,验证能跟 WAU 产品体系 16 仓交互。
>
> 适用:本地有 wau-cli binary,服务器公网 IP `43.134.126.126` 已部署完整 wau stack。

---

## 前置

- wau-cli v0.9.0+(含 wau stack 子命令)
- 服务器 `43.134.126.126` 公网可达(:18400 kernel, :18401 registry 等端口开放)
- 或本机起的 wau stack:`wau stack up --demo`

---

## 路线 A:连服务器(签证 demo 推荐)

### 1. 配置指向服务器

```bash
mkdir -p ~/.wau
cat > ~/.wau/config.yaml <<EOF
kernel:
  addr: "http://43.134.126.126:18400"
  role: "external_agent"
  timeout: 30s

output:
  format: "table"
  color: true

logging:
  level: "info"
EOF
```

### 2. 跑 5 个常用子命令

```bash
# 1. health check — kernel 是否活着
wau health --addr http://43.134.126.126:18400

# 2. kernel info — 看版本 / uptime
wau kernel info --addr http://43.134.126.126:18400

# 3. 搜 L5 agent — 类 apt search
wau agent search medical --addr http://43.134.126.126:18401 --universe medical

# 4. 装一个 agent
wau agent install fox-medical --addr http://43.134.126.126:18400

# 5. 提交任务
wau task submit "帮我开感冒药" --addr http://43.134.126.126:18400
```

预期:每个命令都返回数据,exit code 0。

---

## 路线 B:本机起 stack(开发用)

```bash
# 1. 先装 9 个 binary 到 ~/.wau/bin
mkdir -p ~/.wau/bin
cd /home/inamoto888/project/WAU-core-kernel && go install ./cmd/wau-core && cp $GOBIN/wau-core ~/.wau/bin/
cd /home/inamoto888/project/wau-registry-service && go install ./cmd/registry && cp $GOBIN/registry ~/.wau/bin/
# ... 重复 7 次(wau-store / wau-intent-service / wau-profile-service / wau-llm-router / wau-edge / wau-channel / wau-agent)
# redis: apt install redis-server 或 docker run -d -p 6379:6379 redis

# 2. 加 ~/.wau/bin 到 PATH
export PATH=~/.wau/bin:$PATH

# 3. 看 plan
wau stack up --demo --dry-run

# 4. 真起(redis 必须先在)
wau stack up --demo

# 5. 看状态
wau stack ls

# 6. 关停
wau stack down --all
```

---

## 🇪🇪 Visa 5 分钟 demo 脚本(对创新委员会)

```bash
# T0:00 — 架构可视化
$ wau stack up --demo --dry-run
Plan (dry-run): 10 services in order:
  1. redis [required]
  2. wau-core [required]
  3. registry [required]
  ... (10 服务拓扑排序输出)
讲解:9 个 binary + 1 个 redis,按依赖序启

# T0:30 — 客户端能力
$ wau agent search medical --addr http://43.134.126.126:18401 --universe medical
NAME              VERSION  TRUST  AUTHOR
fox-medical       1.2.0    0.92   acme
chinese-medicine  0.5.0    0.78   univ-tcm
讲解:类 apt search,wau-registry 的 trust score

# T1:30 — 一键装
$ wau agent install fox-medical --addr http://43.134.126.126:18400
⠋ Pulling manifest.yaml... ✓
⠋ Verifying SHA256...     ✓
✓ fox-medical 1.2.0 installed
讲解:Docker sandbox + seccomp + RO fs,来自 D68 设计

# T2:30 — 端到端任务
$ wau task submit "帮我开感冒药" --addr http://43.134.126.126:18400
✓ task_1700000123 submitted
[agent=fox-medical] 中医辨证:风寒... 开方:...
讲解:从 user query → intent → registry match → agent exec

# T4:00 — 状态全景
$ wau stack ls
No services tracked. Use `wau stack up` first.
讲解:本机 runtime 是空的,因为生产部署在 server 43.134.126.126
```

---

## 下一步

- [README.md](README.md) — 全子命令 + 选项
- [ARCHITECTURE.md](ARCHITECTURE.md) — 内部架构
- [CHANGELOG.md](CHANGELOG.md) — 版本演进
- [RELEASING.md](RELEASING.md) — 发版流程
- [DEPLOY.md](DEPLOY.md) — 多环境切换
