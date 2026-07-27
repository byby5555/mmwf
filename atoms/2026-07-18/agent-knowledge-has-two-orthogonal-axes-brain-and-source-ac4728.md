---
type: atom
title: 'Agent knowledge has two orthogonal axes: brain and source'
lesson: >-
  When designing multi-tenant memory systems, separate the storage axis from the
  content axis explicitly to avoid silent routing failures.
atom_type: framework
source_hash: a4e7a4ba2c995eef
source_path: /root/.gbrain/corpus/session-90f192d2.txt
extracted_at: '2026-07-18T07:52:03.683Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  GBrain knowledge is organized along two orthogonal axes. Users AND agents must
  understand both, or queries misroute silently. Brain — WHICH DATABASE. Source
  — WHICH REPO INSIDE THE DATABASE.
virality_score: 70
emotional_register: inspiring
---

GBrain organizes knowledge along two independent axes: 'brain' specifies which database, and 'source' specifies which repo inside that database. Every query must resolve both or routing silently misroutes. This separation enables team deployments where each person gets a scoped slice of company memory.
