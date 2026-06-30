# wau-cli 架构

## 模块拆分

```
wau-cli/
├── cmd/wau-cli/main.go         # cobra root
├── internal/
│   ├── config/                  # cli.yaml 加载 + endpoint 切换
│   ├── client/                  # 复用同仓 client 包
│   │   ├── kernel/              # → WAU-core-kernel gRPC
│   │   ├── trust/               # → wau-trust
│   │   ├── profile/             # → wau-profile
│   │   ├── intent/              # → wau-intent
│   │   ├── registry/            # → wau-registry-service
│   │   ├── circuit/             # → wau-circuit
│   │   └── scheduler/           # → wau-scheduler
│   └── cmd/                     # 各子命令实现
└── README.md / QUICKSTART.md / DEPLOY.md / ARCHITECTURE.md / CHANGELOG.md
```

## 数据流

```
用户输入 `wau trust issue --tenant acme`
    ↓ cobra 解析 → cmd.IssueCmd
    ↓ client.trust.IssueToken(...)
    ↓ gRPC :18460
wau-trust 返回 token
    ↓
格式化输出(JSON / 表 / 文本)
```

## 关键决策

| 决策 | 内容 |
|---|---|
| **跨 14 仓** | wau-cli 是唯一跟所有仓都打交道的工具 |
| **不存业务逻辑** | 只做 client 封装 + 命令路由 |
| **cobra** | 行业标准 Go CLI 框架 |

## 接口边界

- **入**:CLI 命令 + 配置文件
- **出**:JSON / 格式化输出
- **依赖**:14 仓的 gRPC client(直接连)
- **被依赖**:开发者(本地使用,无上游)

## 性能预算

| 指标 | 目标 |
|---|---|
| 命令启动 | < 50 ms(冷启动)|
| 命令启动 | < 5 ms(热启动 + autocompletion cache)|
| 单 RPC 等待 | < 100 ms |

## 跟其他仓的关系

- **上游(本地)**:开发者
- **下游**:14 仓全部(WAU-core-kernel / wau-trust / wau-profile / wau-intent / wau-registry / wau-circuit / wau-scheduler + 3 Sidecar + 4 SDK 间接)
- **不嵌 runtime**:每个调用是独立 gRPC
