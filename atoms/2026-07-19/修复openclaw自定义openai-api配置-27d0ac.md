---
type: atom
title: 修复OpenClaw自定义OpenAI API配置
lesson: OpenClaw可以通过OpenAI格式兼容层接入任意自定义API，关键是设置正确的baseUrl和模型名，避免使用不支持的provider插件。
atom_type: strategy
source_hash: ee36eae663860262
source_path: /root/.gbrain/corpus/session-9e20102f.txt
extracted_at: '2026-07-19T02:01:04.743Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  搞定了！配置总结：provider: openai, baseUrl: https://opencode.ai/zen/go/v1, model:
  openai/deepseek-v4-flash
virality_score: 65
emotional_register: practical
---

OpenClaw原生支持通过OpenAI格式兼容层接入自定义API，关键是在配置中设置正确的baseUrl和模型名。本例中用户需要接入opencode.ai的deepseek-v4-flash模型，最终配置为provider: openai, baseUrl: https://opencode.ai/zen/go/v1, model: openai/deepseek-v4-flash。OpenClaw的config命令提供了openclaw config set --baseUrl来设置自定义API地址。
