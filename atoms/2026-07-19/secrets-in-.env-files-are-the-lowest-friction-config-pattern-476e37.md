---
type: atom
title: Secrets in .env files are the lowest-friction config pattern
lesson: >-
  Use .env files for secrets in self-hosted Python tools; systemd services can
  read them naturally from the working directory.
atom_type: insight
source_hash: 3e2fd05f68e1a540
source_path: /root/.gbrain/corpus/session-4d5fa280.txt
extracted_at: '2026-07-19T02:00:32.473Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: 配.env填token和chat_id
virality_score: 38
emotional_register: practical
---

Telegram tokens and chat IDs belong in a .env file next to the script, not hardcoded or passed as CLI args. The vps-monitor pattern — .env with TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_IDS — makes rotation trivial and keeps secrets out of git history. This is the simplest pattern that still supports systemd (which inherits the working directory's .env).
