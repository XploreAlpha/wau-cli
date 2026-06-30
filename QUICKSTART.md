# wau-cli 15 分钟跑通

> 目标:本机装 wau-cli + 跑通 5 个常用子命令,验证能跟 WAU 产品体系 6+ 仓交互。

## 前置

- Go 1.21+(编译)+ 单 binary(发布版直接下载)
- 默认连 localhost:18402(WAU-core-kernel)、localhost:18460(wau-trust) 等

## 步骤

### 1. 安装(本机)

```bash
# 方式 A:从源码
cd ~/project/wau-cli
make build && cp bin/wau-cli /usr/local/bin/

# 方式 B:下载 v0.9.0 release
curl -sL https://github.com/XploreAlpha/wau-cli/releases/download/v0.9.0/wau-cli-linux-amd64 -o /usr/local/bin/wau-cli
chmod +x /usr/local/bin/wau-cli
```

### 2. 配置

```bash
mkdir -p ~/.wau
cat > ~/.wau/cli.yaml <<EOF
endpoints:
  kernel: 127.0.0.1:18402
  trust: 127.0.0.1:18460
  profile: 127.0.0.1:18480
  intent: 127.0.0.1:18490
EOF
```

### 3. 跑 5 个常用子命令

```bash
# 1. health check
wau health

# 2. 签发 trust token
wau trust issue --tenant acme --scope read --ttl 1h

# 3. 查 profile
wau profile get --tenant acme --user u-001

# 4. intent 分类
wau intent classify --text "今天天气怎么样?"

# 5. 看 wau-edge 状态(经由 kernel)
wau edge list
```

预期:每个命令都返回 JSON 数据,exit code 0

### 4. autocompletion(可选)

```bash
# bash
echo 'source <(wau completion bash)' >> ~/.bashrc
# zsh
echo 'source <(wau completion zsh)' >> ~/.zshrc
```

## 下一步

- [DEPLOY.md](DEPLOY.md) — 多 WAU 环境切换
- [ARCHITECTURE.md](ARCHITECTURE.md) — 子命令路由
- [README.md](README.md) — 全子命令列表
