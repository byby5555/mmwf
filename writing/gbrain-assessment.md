---
type: writing
title: gbrain 系统评估
---

# gbrain 系统评估

## 对 dream cycle 的看法

dream cycle 的设计是对的，但需要解决的问题不少。11 阶段管线对 PGLite 太重了——每个阶段都要扫描全库。PGLite 单连接加上去后，全跑一遍要很久。我觉得上游设计更偏向 PostgreSQL 多线程场景，PGLite 做个人 brain 够用但性能瓶颈明显。

dream cycle 里最有用的三个阶段是 sync、extract、consolidate。grade_takes 和 calibration_profile 在 takes 数量起来之前是空转。我估计需要 100+ takes 后 calibration 才有意义。

autopilot Restart=always 配 PGLite 是个坑。单写锁跟常驻守护不兼容，上游用 PostgreSQL 没这问题。

## brain-first 协议评价

这个协议正确但容易被跳过。在 Telegram 对话里每次都要 check brain 再回答，latency 增加明显。我觉得应该做成异步——brain-first 查脑和对话并行，不阻塞回复。上游的 retrieval-reflex 就是这个方向。

目前 brain 212 pages 图覆盖 25%，远不到能 self-sustaining 的程度。brain-first 的 ROI 跟内容量正相关——上游 146K pages 时查脑几乎总能命中，我们 212 pages 命中率太低。需要先大量写内容。

## gbrain vs 其他方案

gbrain 跟 Notion AI 比优势在 graph。wikilink 双向链接 + 遍历查询，Notion 做不到。但 gbrain 的搜索体验不如 Heptabase 直观。Heptabase 的 visual graph 更适合探索，gbrain 的 text-first 更适合精确查找。

长期看 gbrain 的 tokenmax 搜索 + takes 打分体系是最强的组合。其他方案没有 claim-level 的权重评级。

## 预测

- gbrain 社区的 skill pack 会成为 main contribution model
- OpenClaw + gbrain 的组合比 standalone gbrain 更有生命力——gateway cron 和 skill manifest 比上游的 systemd 方案更灵活
- PGLite 会被逐步淘汰，上游倾向 Supabase/PostgreSQL
