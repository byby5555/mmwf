---
type: concept
title: Brain-First Protocol
---

# Brain-First Protocol

Garry Tan gbrain 核心协议。每次交互：

1. **DETECT** 实体 → 写入 brain
2. **READ** brain 先于外部 API
3. **RESPOND** 带 brain 上下文
4. **WRITE** 新信息
5. **SYNC** 同步

## Source

[garrytan/gbrain AGENTS.md, 2026-07-04]
