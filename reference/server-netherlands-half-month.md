---
type: note
title: 服务器 — 半月抛荷兰福利机
validate: false
ingested_at: '2026-07-01T15:53:40.989Z'
source_kind: 'mcp:put_page'
ingested_via: 'mcp:put_page'
---

## 基本信息

- **名称**: 半月抛荷兰福利机
- **地区**: 荷兰
- **状态**: 运行中
- **面板**: https://shlii.io/dashboard
- **实例 ID**: u2873-8hrpovqe
- **节点**: PEER1482-mutsumi-nl

## 配置

| 项目 | 规格 |
|------|------|
| 系统 | Alpine 3.21 |
| 模式 | LXC |
| CPU | 1核 (上限15%) |
| 内存 | 128 MB |
| 硬盘 | 2 GB |
| 带宽 | 40000 Mbps (40 Gbps) 入/出 |
| 流量 | 1 TB/月 |
| 已用流量 | 388.33 MB (0.0%) |
| 实时速率 | RX 35.0 B/s / TX 22.0 B/s |
| 续费价格 | ¥0.01/月 |
| 到期时间 | 2026-07-26 |

## SSH 登录

```bash
ssh root@45.82.56.75 -p 34435
```

- **密码**: AhZ^g6E5U&SdQl@@
- **用户**: root

## 网络

| 类型 | 地址 |
|------|------|
| 内网 IPv4 | 10.10.3.170 |
| 公网 IPv4 | 45.82.56.75 |
| SSH 端口 | 34435 |

## 监控

- Komari 探针: 已安装（2026-07-02 重启修复）
- 面板: https://vps.1122275.xyz
