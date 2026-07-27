---
type: note
title: GBrain Integrations
---

# GBrain Integrations

These integrations require API keys to enable. Set them up when ready.

## Available Integrations

### Infrastructure
- [ ] **credential-gateway** — Gmail + Calendar OAuth (ClawVisor or direct Google)
- [ ] **ngrok-tunnel** — Fixed public URL for MCP

### Senses (Data Inputs)
- [ ] **email-to-brain** — Gmail → brain pages (needs credential-gateway)
- [ ] **x-to-brain** — Twitter timeline + mentions (needs X API Bearer Token, $200/mo)
- [ ] **meeting-sync** — Circleback transcripts (free for 10 meetings/mo)
- [ ] **calendar-to-brain** — Google Calendar events (needs credential-gateway)

### Reflexes
- [x] **retrieval-reflex** — Configured

## Cron Jobs (Active)
- [x] Dream Cycle: 2 AM daily
- [x] Live Sync: every 15 min
- [x] Morning Briefing: 8 AM daily
- [x] Auto-Update Check: 9 AM daily
- [x] Weekly Health: Mon 6 AM
