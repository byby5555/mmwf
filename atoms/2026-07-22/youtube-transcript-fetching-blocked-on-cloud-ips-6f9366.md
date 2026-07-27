---
type: atom
title: YouTube transcript fetching blocked on cloud IPs
lesson: >-
  YouTube blocks cloud provider IPs from transcript APIs — oEmbed still returns
  basic metadata, but transcript extraction needs residential proxies.
atom_type: insight
source_hash: bb7c496c6f4efe80
source_path: /root/.gbrain/corpus/session-80b06936.txt
extracted_at: '2026-07-22T02:00:29.311Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  YouTube is blocking requests from your IP. This usually is due to...you are
  doing requests from an IP belonging to a cloud provider. Most IPs from cloud
  providers are blocked by YouTube.
virality_score: 40
emotional_register: sobering
---

YouTube transcript API and yt-dlp both fail from cloud-hosted VPS environments because YouTube blocks known datacenter IP ranges. The oEmbed API still works and returns basic metadata like title, author, and thumbnail, but full transcript extraction requires residential proxies or browser cookies.
