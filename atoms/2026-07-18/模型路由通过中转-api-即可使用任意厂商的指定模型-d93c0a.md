---
type: atom
title: 模型路由通过中转 API 即可使用任意厂商的指定模型
lesson: 通过兼容 OpenAI 的中转 API 可以自由切换模型，无需绑定单一厂商。
atom_type: strategy
source_hash: 7668a58a6c1b75ee
source_path: /root/.gbrain/corpus/session-7c3bd770.txt
extracted_at: '2026-07-18T07:51:31.853Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: >-
  可以。配了 mingwang provider，base_url 和 key 都在。之前 model.default 写的是 gpt-5.5，应该是
  5.5。我改一下。
virality_score: 72
emotional_register: practical
---

不需要直接拿 Anthropic/OpenAI key，只要有一个兼容 OpenAI 格式的中转 API（如 pie-xian.com），配好 base_url 和 api_key，就能用 deepseek-v4-flash 等模型。模型名是 provider/model 格式，例如 opencode/deepseek-v4-flash。配置 model.default、model.provider，再在 providers 段加 base_url 和 api_key 就行。
