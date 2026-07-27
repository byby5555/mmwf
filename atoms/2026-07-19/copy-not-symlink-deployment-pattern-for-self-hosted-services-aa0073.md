---
type: atom
title: Copy-not-symlink deployment pattern for self-hosted services
lesson: >-
  Clone to a temp directory then copy to the final location to keep production
  isolated from git operations.
atom_type: strategy
source_hash: 3e2fd05f68e1a540
source_path: /root/.gbrain/corpus/session-4d5fa280.txt
extracted_at: '2026-07-19T02:00:32.029Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: cp -r /tmp/vps-monitor/* /opt/vps-monitor/
virality_score: 32
emotional_register: practical
---

When deploying git-based services to production, clone to /tmp then copy to /opt, don't symlink. This keeps git metadata out of the runtime directory and prevents accidental `git pull` breaking production. The systemd unit references the copied path, making the deployment immutable after setup.
