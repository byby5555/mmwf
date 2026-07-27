---
type: atom
title: Youtube-transcript-api 被服务器 IP 拦截时无法获取字幕
lesson: 媒体下载类工具要预留多条回退路线，不能依赖单一种子或者单台服务器的出口 IP。
atom_type: strategy_angle
source_hash: 71ffcf7e0b180c9f
source_path: /root/.gbrain/corpus/session-a091c2e8.txt
extracted_at: '2026-07-19T02:01:16.431Z'
extracted_by: extract_atoms-v0.41.2.1
source_quote: "全部试过了，都不行 \U0001F605 本机 IP 被 YouTube 封了；意大利机内存太小跑不动；荷兰机 musl libc 不兼容 yt-dlp"
virality_score: 55
emotional_register: sobering
---

YouTube 对大量 IP 段（尤其是机房 IP）实施了严格的字幕接口封禁，导致 yt-dlp 和 youtube-transcript-api 都无法获取字幕。绕过方式包括：从手机浏览器直接打开第三方字幕聚合站复制文本、用家庭宽带 IP 做代理爬取，或者直接搜视频关键词找文字资料代替。硬怼换 IP 通常徒劳，因为机房的 ASN 本身已经被拉黑。
