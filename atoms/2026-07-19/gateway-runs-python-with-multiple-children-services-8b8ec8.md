---
type: atom
title: Gateway runs Python with multiple children services
lesson: >-
  Build agents as process supervisors that bundle code intelligence, remote
  agents, and state recording into a single supervised hierarchy.
atom_type: insight
source_hash: f079dca1f3c409b8
source_path: /root/.gbrain/corpus/session-e65b75c1.txt
extracted_at: '2026-07-19T02:03:29.541Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  ├─155283 /usr/local/lib/hermes-agent/venv/bin/python -m hermes_cli.main
  gateway run

  ├─189405 node /root/.hermes/lsp/bin/yaml-language-server --stdio

  ├─223508 /usr/bin/sshpass...
virality_score: 70
emotional_register: practical
---

The Hermes gateway process includes not just the Python main loop, but also a YAML language server, an SSH tunnel to a remote server running gbrain serve, and a shell-based snapshot script. This reveals a composable architecture where a single gateway coordinates LSP, remote execution, and state capture.
