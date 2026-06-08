---
demand-id: DM-20260608-003
title: 上下文引擎 V4（Autocompact 异步 + Snappy 压缩）
source: Context Engine V3 性能 Review
priority: P1
status: S7_ARCHIVED
l1-domain: contextengine
created: 2026-06-08
---

# 上下文引擎 V4

## 1. 原始描述

> Autocompact 步骤 6 同步调用 LLM 摘要，默认 10s 超时阻塞 PEV 主循环；会话快照以原始 JSON 持久化，长会话占用过高。

## 2. 澄清记录

### Q1: 异步摘要如何与主循环协调？

**A**: 同步返回 head + 占位摘要 + tail；后台 goroutine 完成后通过 `OnAutocompactComplete` 通知 Observer。 — 2026-06-08

### Q2: 多次触发如何处理？

**A**: 同 session 新任务取消旧 goroutine，仅最新 token 的结果写入。 — 2026-06-08

### Q3: 快照压缩兼容性？

**A**: 魔数 `\xfe\x53` + Snappy；无魔数走 legacy JSON；小于 threshold 不压缩。 — 2026-06-08

## 3. L1–L5 映射草案

| 层级 | 资产 |
|------|------|
| L1 | contextengine |
| L3 | 上下文压缩活动 |
| L4 | L4-CTX-COMPRESS, L4-CTX-MEMORY |
| L5 | L5-CTX-31 ~ L5-CTX-33 |

## 4. 验收标准

- P1：Autocompact 占位 <50ms 返回；异步完成/失败可观测
- P2：Snappy 压缩显著减小快照体积且兼容旧格式
