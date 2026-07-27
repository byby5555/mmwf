---
type: atom
title: 'Autopilot runs on the remote brain, not the local one'
lesson: >-
  Installing the software is step one; teaching the agent to use it at every
  decision point is the real work.
atom_type: story_angle
source_hash: 460180e8ba6feed4
source_path: /root/.gbrain/corpus/session-53df2002.txt
extracted_at: '2026-07-18T07:50:43.507Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  I installed gbrain too... but gbrain is just an MCP backend to me. I use MCP
  tools to query it, but I'm not operating the way Garry does.
virality_score: 68
emotional_register: shocking
---

This user has two gbrain installations: a local one (196 pages, PGLite) and a remote one on another VPS with the same data. The Hermes agent connects to the remote via SSH MCP but never reads the local brain. The local brain runs autopilot and gateway loop, but the agent treats it as just another MCP backend instead of its own memory layer. The story is: you can have all the infrastructure in place and still not be using it because the agent doesn't know it's supposed to.
