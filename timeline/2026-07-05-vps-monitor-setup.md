---
type: timeline
title: 2026 07 05 Vps Monitor Setup
date: '2026-07-05T00:00:00.000Z'
---

# VPS Monitor Setup (2026-07-05)

## Summary
- Deployed VPS monitor on Toncyzcy (107.150.7.152) based on shali10/vps-monitor
- Currently monitoring: site_e (czl.net, 446 providers), site_a (pending token), site_let (CF blocked placeholder)
- NodeSeek RSS monitor removed per request
- Systemd service running: vps-monitor (A+E+LET)
- Telegram push via bot chat_id=660182906

## Configuration
- SITE_E_API_URL: https://vps-monitor.czl.net/api/public/filter
- Poll intervals: site_e=180s, site_let=180s
- Telegram bot integrated for notifications
