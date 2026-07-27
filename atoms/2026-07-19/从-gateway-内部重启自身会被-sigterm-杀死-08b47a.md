---
type: atom
title: 从 gateway 内部重启自身会被 SIGTERM 杀死
lesson: 自托管服务的优雅重启策略必须考虑进程树信号传播——同一进程内的自重启命令永远无法完成，必须委托给外部管理者。
atom_type: insight
source_hash: bea03f6a3f3abd70
source_path: /root/.gbrain/corpus/session-ef4c835e.txt
extracted_at: '2026-07-19T02:03:52.568Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  Blocked: cannot restart or stop the gateway from inside the gateway process.
  The gateway would kill this command before it could complete (SIGTERM
  propagates to child processes).
virality_score: 72
emotional_register: practical
---

尝试在 Hermes gateway 运行中的终端里执行 restart 或 stop 命令会导致失败，因为 gateway 进程会将 SIGTERM 传播到子进程，从而杀死正在执行重启命令的 shell。唯一的办法是从外部 shell 或通过子代理（subagent）异步执行重启。
