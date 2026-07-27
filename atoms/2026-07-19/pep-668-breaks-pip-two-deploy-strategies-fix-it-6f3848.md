---
type: atom
title: PEP 668 breaks pip — two deploy strategies fix it
lesson: >-
  When a system policy blocks your deploy flow, document both the quick and the
  clean escape routes instead of pretending the problem doesn't exist.
atom_type: strategy_angle
source_hash: 89ea6c9b41ca6b07
source_path: /root/.gbrain/corpus/session-b3b5ff6e.txt
extracted_at: '2026-07-19T02:02:11.672Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  For Debian 12+ with PEP 668 protections, two strategies:
  --break-system-packages or create a venv in /opt/vps-monitor/.venv
virality_score: 60
emotional_register: practical
---

Modern Debian systems refuse direct pip installs under PEP 668. The vps-monitor deploy skill documents both escape hatches: force-install with --break-system-packages (fine for single-service VPS) or create a venv and symlink the systemd ExecStart to the venv's Python. Pick the first for simplicity, the second for upgrade isolation.
