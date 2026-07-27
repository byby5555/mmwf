---
type: note
title: Agent Reach 安装记录
date: '2026-07-01T00:00:00.000Z'
validate: false
ingested_at: '2026-06-30T07:28:02.501Z'
source_kind: 'mcp:put_page'
ingested_via: 'mcp:put_page'
---

# Agent Reach 安装记录

- **安装时间**: 2026-06-30
- **版本**: agent-reach v1.5.0
- **状态**: 7/15 渠道可用

## 已配置渠道

- ✅ GitHub — 完整可用
- ✅ YouTube — 视频信息+字幕
- ✅ V2EX — 公开 API
- ✅ RSS/Atom — 订阅源读取
- ✅ 全网语义搜索 — Exa (免费)
- ✅ 任意网页 — Jina Reader
- ✅ B站 — 搜索 API
- ✅ **小红书 — 已登录，按需启动**

## 小红书配置

- **工具**: xiaohongshu-mcp (端口 :18060)
- **登录**: 已扫码登录，Cookie 持久化
- **使用方式**: `xhs xiaohongshu.<tool> [args]`（按需启动，用完自动关）
- **脚本**: `/usr/local/bin/xhs`

## Skill

已注册到 Hermes skills: `agent-reach`
