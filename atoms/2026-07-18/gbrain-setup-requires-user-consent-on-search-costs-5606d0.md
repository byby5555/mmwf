---
type: atom
title: GBrain setup requires user consent on search costs
lesson: >-
  When building agent infrastructure, design consent points into the
  installation flow whenever operational decisions carry large cost variance.
atom_type: insight
source_hash: a4e7a4ba2c995eef
source_path: /root/.gbrain/corpus/session-90f192d2.txt
extracted_at: '2026-07-18T07:52:03.379Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  STOP — ask the user about search mode. gbrain init auto-applied a default but
  printed a 9-cell cost matrix (mode × downstream model) preceded by [AGENT]
  markers. You MUST relay the matrix to the operator and confirm their choice
  before continuing. Cost spread between corners is 25x — silent acceptance is
  the wrong default.
virality_score: 75
emotional_register: sobering
---

Setting up GBrain forces a critical decision point: the search mode cost matrix spans a 25x cost spread between conservative and aggressive modes. The agent must pause and get explicit user approval before proceeding, making cost transparency a design feature rather than an afterthought.
