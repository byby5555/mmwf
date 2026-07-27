---
type: reference
title: Minis Memory Export
ingested_at: '2026-07-25T13:53:15.043Z'
source_kind: 'mcp:put_page'
ingested_via: 'mcp:put_page'
---

# Minis 平台记忆摘要

从用户之前在 Minis 平台的记忆导出文件中提取的关键信息。

## 去 AI 味工作流（Minis 版）

在 Minis 平台上有一套完整的 8 步骤去 AI 味工作流，是当前 thesis-style-zhuque skill 的前身：
- 步骤 0: 素材获取（read / baoyu-url-to-markdown）
- 步骤 1: AI 特征检测（ai-text-humanizer-zh detect.py）
- 步骤 2-5.5: 多种去 AI 味 skill 串联（humanizer-zh, humanize-ai-text, unclecheng-reduce-ai-perception-v2 等）
- 步骤 6: 深度润色（writing-polish）
- 步骤 7: Markdown 格式化（baoyu-format-markdown）
- 步骤 8: 质量验证（check）
- 交付物：AI 检测报告 + 改写对照 + 定稿 + 策略说明

Minis 平台的去 AI 味规则更详细（write-zh.md 有 646 行近 4 万字），包含：bold+句号节奏检测、列表去 list 化、报告腔替换、翻译腔四套件等高级规则。 [Source: Minis Memory Export, 2026-04-28]

## 英语学习背景

- 目标：备考雅思
- 英语水平：A1-A2（初学）
- 使用 App：英语天天练（ABC Zone，好未来/学而思）、懒人英语（Lanren English）
- 对这些 App 做过 MITM VIP 破解分析 [Source: Minis Memory Export, 2026-05-20/23]

## 其他项目

- MarginNote 插件开发（Note Stats 插件，WebView 版本）
- 公文写作工作流（6 步骤，official-doc-writer → official-writing → govwriter-pro → official-doc → docx-formatter → doc-format-gw）
- GitHub 账号：Leslie159357 [Source: Minis Memory Export, 2026-04-29/05-21]
