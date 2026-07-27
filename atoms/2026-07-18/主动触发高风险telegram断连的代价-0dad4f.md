---
type: atom
title: 主动触发高风险：Telegram断连的代价
lesson: 在维护高可用实时系统时，把'不要手动触发'作为一条铁律写进文档。
atom_type: strategy_angle
source_hash: 4a5ebcb6b4ece0a5
source_path: /root/.gbrain/corpus/session-52b79e44.txt
extracted_at: '2026-07-18T07:50:34.841Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: 手动跑会断连 Telegram，所以不去碰它。
virality_score: 30
emotional_register: practical
---

当自动化管道涉及到实时通讯服务（如Telegram bot）时，手动触发任何一个环节都可能造成连接中断。这位用户明确知道手动跑梦循环会导致机器人下线，因此刻意只依赖系统定时任务。这种'线上不动手'的原则适用于所有需要持续在线的系统。
