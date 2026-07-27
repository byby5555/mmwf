---
type: atom
title: 对接自定义API需要匹配正确的模型名前缀
lesson: 对接非标准OpenAI兼容API时，优先使用对应provider的环境变量（如DEEPSEEK_BASE_URL），而不是硬编码baseUrl到配置文件中。
atom_type: insight
source_hash: ee36eae663860262
source_path: /root/.gbrain/corpus/session-9e20102f.txt
extracted_at: '2026-07-19T02:01:05.036Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: '你说得对，opencode.ai 就是 OpenAI 格式。问题是 OpenClaw 发请求时模型名带了 openai: 前缀，API 不认。'
virality_score: 70
emotional_register: practical
---

配置OpenClaw接入自定义OpenAI兼容API时，模型名必须匹配目标API的命名规范。opencode.ai要求模型名为纯字符串'deepseek-v4-flash'，而OpenClaw的provider前缀（如openai:）会导致API请求失败。最终通过DEEPSEEK_BASE_URL环境变量和deepseek插件成功对接，证明环境变量方式是可靠的配置方法。
