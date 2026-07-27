---
type: atom
title: Cron-based GitHub sync for personal brain and agent config
lesson: >-
  Schedule automated syncs with silent-fail resilience to eliminate manual
  backup burden.
atom_type: strategy
source_hash: 4e21df35d28d0e5e
source_path: /root/.gbrain/corpus/session-3a1aab49.txt
extracted_at: '2026-07-18T07:49:57.709Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  每天凌晨4点同步 brain 仓库和 myagent 仓库到 GitHub。  运行 cd /root/brain && gbrain sync
  2>/dev/null || true … 失败不报错，下次自动重试。
virality_score: 25
emotional_register: practical
---

This cron job runs daily at 4 AM to sync two repositories to GitHub: the brain knowledge base (via gbrain sync and git commit/push) and the myagent configuration directory (SOUL.md and config.yaml). Failures are silent — the job skips errors and retries the next day without alerting the user. This ensures zero-maintenance off-site backup of personal knowledge and agent identity.
