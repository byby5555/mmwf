---
type: atom
title: Hermes Agent blocks self-protective commands
lesson: >-
  When an agent's command execution is routed through itself, self-destructive
  commands must be detoured to avoid self-blocking rules.
atom_type: strategy_angle
source_hash: f079dca1f3c409b8
source_path: /root/.gbrain/corpus/session-e65b75c1.txt
extracted_at: '2026-07-19T02:03:29.393Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  WARNING agent.tool_executor: Tool terminal returned error (0.00s): {"output":
  "", "exit_code": 1, "error": "Blocked: cannot restart or stop
virality_score: 55
emotional_register: sobering
---

Trying to restart the Hermes gateway service from within its own subprocess triggers an automatic block. The agent's tool executor returns error 'Blocked: cannot restart or stop', showing that a self-preservation rule prevents destructive operations from being executed via the gateway's own execution chain.
