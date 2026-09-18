
# mmwX Pro - sing-box 订阅管理系统

基于 sing-box 内核的订阅管理与服务器管理系统，支持多服务器远程管理、套餐计费、转发链、WireGuard、证书管理等功能。

> 本项目由 [妙妙屋X](https://github.com/iluobei/miaomiaowuX) 迁移而来，将 Xray 内核替换为 sing-box，并对齐 mmwx-pro 前端功能与 UI。

## 功能特性

### sing-box 服务器管理
- 多服务器管理 - 主控统一管理多台远程 sing-box 服务器
- 远程连接 - WebSocket / HTTP / Pull 三种连接模式，自动回退
- 实时流量 - 各服务器流量统计与实时速度监控
- 远程配置 - 在线管理远程服务器的 sing-box / Nginx 配置
- 入站/出站管理 - 可视化管理 sing-box 入站、出站、路由规则
- 证书管理 - ACME 自动申请/续期 SSL 证书，支持多种 DNS 提供商
- 一键部署 - 远程服务器一键安装 sing-box + Nginx + Agent
- 套餐管理 - 用户套餐与流量限额管理
- 节点同步 - 入站变更自动同步到订阅节点

### 订阅管理
- 流量监控 - 支持 sing-box 流量采集与外部订阅流量聚合统计
- 历史流量 - 30 天流量使用趋势图表
- 节点管理 - 导入个人节点或机场节点，支持批量操作
- 用户管理 - 管理员/普通用户角色区分，订阅权限管理
- 主题切换 - 支持亮色/暗色模式

### v3 加密通道（新增）
- 前端通信全加密：X25519 ECDH 握手 + HKDF-SHA256 密钥派生 + AES-256-GCM 双向加密
- 统一分发端点 `/api/v3u`（用户侧）+ `/api/v3`（管理侧），267 个 op 路由映射
- 会话过期自动重新握手，64 位序列号防重放

### 支持的客户端格式
Clash(Meta) / Surge / Loon / Quantumult X / Shadowrocket / SingBox / Stash / Surfboard / V2Ray / Egern

## 安装部署

### 方式 1：一键安装（推荐）

```bash
curl -sL https://raw.githubusercontent.com/byby5555/mmwx-pro/main/install.sh | sudo bash
```

自动检测架构、下载最新版本、创建 systemd 服务。安装完成后访问 `http://服务器IP:8080` 进入初始化向导。

更新：
```bash
curl -sL https://raw.githubusercontent.com/byby5555/mmwx-pro/main/install.sh | sudo bash -s update
```

卸载：
```bash
curl -sL https://raw.githubusercontent.com/byby5555/mmwx-pro/main/install.sh | sudo bash -s uninstall
```

卸载将停止并禁用 systemd 服务、删除程序目录及数据库，清理零残留。

### 方式 2：Docker 部署

> 默认使用 host 网络模式 — 便于 agent 反向连接、多端口场景。

```bash
docker run -d \
  --name mmwx-pro \
  --network host \
  --restart unless-stopped \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/subscribes:/app/subscribes \
  -v $(pwd)/rule_templates:/app/rule_templates \
  ghcr.io/byby5555/mmwx-pro:latest
```

#### Docker Compose

```yaml
version: '3.8'

services:
  mmwx-pro:
    image: ghcr.io/byby5555/mmwx-pro:latest
    container_name: mmwx-pro
    restart: unless-stopped
    network_mode: host
    environment:
      - PORT=8080
      - LOG_LEVEL=info
    volumes:
      - ./data:/app/data
      - ./subscribes:/app/subscribes
      - ./rule_templates:/app/rule_templates
```

### 方式 3：二进制部署

从 [Releases](https://github.com/byby5555/mmwx-pro/releases) 下载对应平台的二进制文件：

```bash
# Linux
chmod +x mmwf-linux-amd64
./mmwf-linux-amd64
```

默认端口 `8080`，访问 `http://服务器IP:8080` 进入初始化向导。

### 远程服务器部署

在主控面板添加远程服务器后，会生成一键安装命令，在远程服务器上执行即可自动安装 mmw-agent 并连接到主控。

## 架构

```
┌─────────────────────────────────────────────┐
│           mmwX Pro (主控)                    │
│                                             │
│  订阅管理 / sing-box管理 / 证书管理 / 用户管理  │
│  流量统计 / 套餐管理 / 节点同步 / v3加密通道    │
└──────────────────┬──────────────────────────┘
                   │ WebSocket / HTTP / Pull
     ┌─────────────┼─────────────┐
     ▼             ▼             ▼
┌──────────┐  ┌──────────┐  ┌──────────┐
│ Agent 1  │  │ Agent 2  │  │ Agent 3  │
│(sing-box)│  │(sing-box)│  │(sing-box)│
└──────────┘  └──────────┘  └──────────┘
```

## 配置文件

```yaml
mode: master              # master（默认）或 remote
port: "8080"              # 监听端口
# 以下为 remote 模式配置
master_server: ""         # 主控地址
remote_token: ""          # 服务器令牌
connection_mode: "auto"   # auto | websocket | http | pull
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 服务端口 | `8080` |
| `LOG_LEVEL` | 日志级别 | `info` |
| `JWT_SECRET` | 会话令牌签名密钥 | 随机生成 |
| `ALLOWED_ORIGINS` | CORS 允许来源 | `*` |

## 技术栈

- 后端：Go 1.25+ + net/http + SQLite (modernc.org/sqlite)
- 前端：React 19 + Vite 7 + TanStack Router + TailwindCSS 4 + shadcn/ui
- 加密通道：X25519 + HKDF-SHA256 + AES-256-GCM
- 单二进制部署，前端通过 Go embed 嵌入

## 从源码构建

```bash
# 交叉编译 Linux amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o build/mmwf-linux-amd64 ./cmd/server
```

## 免责声明

- 本程序仅供学习交流使用，请勿用于非法用途
- 使用本程序需遵守当地法律法规
- 作者不对使用者的任何行为承担责任

## 许可证

MIT License

## 联系方式

- 问题反馈：[GitHub Issues](https://github.com/byby5555/mmwx-pro/issues)
