---
type: atom
title: PGLite 单写锁导致 gbrain serve 与 autopilot 无法共存
lesson: 当心智模型是单写锁时，自治代理不能同时作为长期运行的进程暴露 MCP 服务器接口。
atom_type: insight
source_hash: 76cfb68a6fbff8fc
source_path: /root/.gbrain/corpus/session-d5c08a3b.txt
extracted_at: '2026-07-23T02:00:29.325Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  autopilot's ~150s cycle is just the work interval, but the PGLite lock is held
  persistently. gbrain serve never gets it.
virality_score: 65
emotional_register: sobering
---

Gbrain 的 PGLite 引擎使用持久的单写锁，autopilot 进程不断刷新该锁，导致 gbrain serve MCP 服务器永远无法获取访问权限。这并非偶然的竞争条件，而是由常驻锁持有造成的 100% 阻塞问题。
