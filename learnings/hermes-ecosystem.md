---
type: note
title: Hermes 生态学习笔记
created: '2026-07-27T00:00:00.000Z'
tags:
  - ecosystem
  - hermes
  - learning
  - summary
---

# Hermes 生态学习笔记

汇总2026年7月26-27日对 Hermes 生态相关项目的一字不漏阅读学习。

## 阅读清单

| 项目 | 作者 | 规模 | 关键收获 |
|------|------|------|---------|
| Hermes 官方文档 | Nous Research | 完整 llms-full.txt | 架构了解，prompt assembly，memory 设计 |
| hermesagent.org.cn | 社区中文化 | 中文文档 | 国内镜像+微信群+中国模型配置 |
| Hermes 白皮书 | 鲲鹏Talk | 16册30万字 | 四层温度记忆模型，九大变现路径 |
| jwangkun/hermes-agent-guide | 鲲鹏Talk | 16册，885K字 | 最系统的中文 Hermes 讲解 |
| awesome-claude-code | hesreallyhim | 51K stars | Claude Code 资源索引 |
| claw-code | ultraworkers | 195K stars | 事件驱动+故障自愈，全 AI 自主维护 |
| lazycodex | code-yeongyu | 3K stars | Codex 安装器，OmO 封装 |
| gajae-code | Yeachan-Heo | 2.2K stars，4867文件 | 外挂式 harness，详尽 AGENTS.md |
| gbrain | garrytan | 27.1K stars | YC 总裁开源的个人知识库系统 |

## 关键设计模式

### Gajae-Code AGENTS.md 规范
来源: gajae-code/AGENTS.md (148行)
- 代码规范: `#private` 代替 `private`，`Promise.withResolvers()` 代替 `new Promise`
- 测试规范: "测外部可观察的契约，不测占位符"
- 命令规范: `bun check` 代替 `npx tsc`
- 角色 agent 的 bashAllowedPrefixes 白名单设计
- 工作流路由: 直接执行 > deep-interview > ralplan > ultragoal > team

### Claw Code 事件驱动设计
来源: claw-code
- LaneEvent 系统，36 种事件变体
- EventProvenance 区分 live/test/healthcheck/replay
- 故障自愈: 7 种恢复配方，一次自动重试后升级到人
- Worker 状态机: 8 个状态

### ERRATA Debug 方法
来源: gajae-code/docs/ERRATA-GPT5-HARMONY.md
- 分析 105 万次调用
- 精确 ppm 泄漏率
- 按工具分布归类
- 自增反馈循环分析

### ADR 文档文化
来源: gajae-code/docs/adr-*
- 每条 ADR: 决策 → Drivers → 备选 → 选这个的原因 → 后果
- 20-30行一条，结构固定

## 供应链关系

```
oh-my-codex (工作流层)
    → oh-my-openagent/OmO (60K⭐ 核心引擎)
        → LazyCodex (Codex 安装器, 3K⭐)
        → Gajae-Code (独立外挂 harness, 2.2K⭐)
            → Claw Code (Rust 实现, 195K⭐)

Hermes Agent (Nous Research) ← 当前平台
    + gbrain (YC Garry Tan) ← 知识库系统
```

[Source: Telegram conversation with user, 2026-07-26~27]
