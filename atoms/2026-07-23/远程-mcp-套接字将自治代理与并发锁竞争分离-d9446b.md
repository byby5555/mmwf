---
type: atom
title: 远程 MCP 套接字将自治代理与并发锁竞争分离
lesson: 在迁移到支持并发的后端之前，不要尝试修复文件级锁竞争。
atom_type: strategy_angle
source_hash: 76cfb68a6fbff8fc
source_path: /root/.gbrain/corpus/session-d5c08a3b.txt
extracted_at: '2026-07-23T02:00:29.774Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  After migration, autopilot and gbrain serve can run concurrently — Postgres
  handles the concurrency natively.
virality_score: 60
emotional_register: practical
---

从自托管大脑迁移到托管的 Supabase 引擎，可以解除 autopilot 和 gbrain serve MCP 服务器之间的单写锁竞争。迁移后，两个进程可以独立连接到同一个持久数据库，使用 Postgres 的原生并发支持，而不是等待文件系统锁。
