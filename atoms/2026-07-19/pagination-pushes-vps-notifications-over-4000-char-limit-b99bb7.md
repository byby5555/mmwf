---
type: atom
title: Pagination pushes VPS notifications over 4000 char limit
lesson: >-
  If your output hits platform limits, paginate early rather than truncating or
  skipping items.
atom_type: strategy
source_hash: dda7f3e6d19f59a9
source_path: /root/.gbrain/corpus/session-a22c51d5.txt
extracted_at: '2026-07-19T02:01:31.457Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  def _paginate(items, notify_fn, page_size=5): ... sent += 1 (implementation of
  paginated Telegram notifications)
virality_score: 60
emotional_register: practical
---

When sending VPS inventory updates via Telegram, a naive flat list exceeds the 4096-character message limit. The solution implemented here is auto-pagination: split items into pages of 5, add a page counter, and send each page as a separate message. This keeps notifications readable, avoids truncation, and mirrors what top-tier VPS monitor bots do.
