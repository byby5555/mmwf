---
type: atom
title: Hermes 集成五运六气只用 Python 核心脚本 + gbrain 知识库
lesson: 集成第三方 Agent 技能包时，只提取核心计算逻辑和结构化知识，用自己现有的存储和推理基础设施替代原版流程编排，效果不减反增。
atom_type: strategy_angle
source_hash: ea1e3cf2f6770cf3
source_path: /root/.gbrain/corpus/session-c960a3f8.txt
extracted_at: '2026-07-19T02:02:32.636Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: RAG 知识库 JSON → 直接写进 gbrain 页面。你的 brain 自带检索，比读 JSON 文件灵活多了。
virality_score: 62
emotional_register: practical
---

一个 9MB 的五运六气 AI Agent 技能包，不需要完整移植。核心推算脚本只有几千行 Python 代码，知识库可以直接写入 Hermes 的 gbrain 知识库页面。整个推理链路变成用户提问、Hermes 调本地脚本计算、再查 gbrain 知识库做病机分析，比原版在终端里跑 routing 和 ReAct 流程更直接。
