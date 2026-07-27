---
type: atom
title: 短期记忆的符号化革命：Mermaid图代替日志
lesson: 短期记忆的核心矛盾是承载信息密度vs上下文窗口，符号化压缩是比截断更优雅的解。
atom_type: strategy
source_hash: c89bcd07a3ca3593
source_path: /root/.gbrain/corpus/session-18f35bb2.txt
extracted_at: '2026-07-18T07:49:19.336Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  Symbolic short-term memory offloads heavy tool logs and condenses them into
  compact Mermaid symbols, cutting token usage and improving task success.
virality_score: 68
emotional_register: practical
---

TencentDB-Agent-Memory将沉重的工具执行日志卸载到本地文件，仅在上下文中保留一张Mermaid符号画布和node_id用于回溯，这种设计在WideSearch基准上使token消耗降低61.38%，同时任务通过率提升51.52%。对于token成本敏感的Agent系统，这是一种极具吸引力的架构选择。
