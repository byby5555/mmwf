---
type: note
title: HelloDaily 工作流配置
validate: false
ingested_at: '2026-06-30T00:29:37.364Z'
source_kind: 'mcp:put_page'
ingested_via: 'mcp:put_page'
---

# HelloDaily 工作流

HelloDaily 是 Leslie159357/HelloDaily 仓库的自动化日报项目。

## 自动调度

- **触发器**: GitHub Actions，`.github/workflows/daily.yml`
- **时间**: 每天 09:00 CST（UTC 01:00）
- **手动**: 支持 workflow_dispatch
- **权限**: contents: write（用内置 GITHUB_TOKEN 提交）

## 数据流

1. `fetch_by_volume()` — 从 `521xueweihan/HelloGitHub` repo 的 content/ 目录扒 markdown
2. `fetch_trending()` — 抓 GitHub Trending 补充
3. `translate_descriptions()` — 用 DeepSeek V3（走 SiliconFlow API）翻译 Trending 英文简介
4. `fetch_stars()` — 用 GitHub API 查所有项目 star 数
5. 去重合并 → 生成 markdown → 更新 README → commit + push

## 环境变量

- `GITHUB_TOKEN` — GitHub API 查 star 用（Actions 内置 token 即可）
- `OPENAI_API_KEY` / `OPENAI_BASE_URL` — DeepSeek 翻译用（走 SiliconFlow）

## 注意事项

- 来源期数以 `<!-- source_volume: N -->` 隐藏注释记录在 markdown 中，下次自动跳过
- 同一期用过的 HelloGitHub 卷号不会再重复使用
