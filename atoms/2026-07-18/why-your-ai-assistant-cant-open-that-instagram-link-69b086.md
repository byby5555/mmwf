---
type: atom
title: Why your AI assistant can't open that Instagram link
lesson: >-
  When building AI agents, prioritize giving them the simplest network tool (a
  HTTP GET function) over exotic integrations — most real-world tasks start with
  a single URL.
atom_type: story_angle
source_hash: 666eb9ab13c1226e
source_path: /root/.gbrain/corpus/session-14d9d47c.txt
extracted_at: '2026-07-18T07:49:03.993Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: 当前环境中没有 bash、python 或 execute_command 类工具可用
virality_score: 78
emotional_register: shocking
---

A user pastes an Instagram Reel link and asks for metadata. The AI assistant runs search after search — DuckDuckGo, third-party scrapers, even tries to spawn a sub-agent. Nothing works. The reason is systemic: modern LLM sandboxes ship with file readers and vector stores but strip out fundamental network primitives like curl or requests. The assistant can 'think' about the problem but cannot touch the internet. This is a perfect parable for the architectural blind spot in current AI platforms — powerful reasoning paired with crippled execution.
