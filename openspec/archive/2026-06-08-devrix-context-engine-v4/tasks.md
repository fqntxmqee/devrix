# Tasks: devrix-context-engine-v4

**Change ID:** devrix-context-engine-v4
**Parent:** devrix-context-engine-v3
**Status:** S4 Development
**Based on:** design.md, `specs/context-engine/spec.md`

---

## Milestone 1: Autocompact 异步化（P1）

### Definition of Done
- [x] Autocompact 占位摘要 < 50ms 返回
- [x] 异步 LLM 摘要完成后通知 Observer
- [x] 多次触发去重（cancel 旧的）
- [x] Shutdown 取消所有 pending goroutine

### Tasks

- [x] **T1**: 新增 `compression/async_compact.go` AsyncAutocompacter
  - 异步 goroutine 管理 + WaitGroup
  - pending map（token → cancel）去重
  - Shutdown 超时清理
  - L5: L5-CTX-31, L5-CTX-33
  - Estimate: 3h
  - Dependencies: None

- [x] **T2**: 修改 `compression/autocompact.go` runAutocompact
  - 同步返回占位摘要（head + placeholder + tail）
  - 异步触发 startAsync
  - Observer 接口新增 OnAutocompactComplete
  - L5: L5-CTX-31
  - Estimate: 2h
  - Dependencies: T1

---

## Milestone 2: Snappy 快照压缩（P2）

### Definition of Done
- [x] Snappy 压缩启用后快照体积显著减小
- [x] 旧格式快照正常读取
- [x] 压缩阈值配置生效

### Tasks

- [x] **T3**: 修改 `snapshot/store.go` Serialize/Deserialize
  - Serialize: snappy.Encode + 魔数头部
  - Deserialize: 魔数检测 + snappy.Decode / legacy JSON fallback
  - SnapshotConfig 新增 Compression + CompressionThreshold
  - L5: L5-CTX-32
  - Estimate: 2h
  - Dependencies: None

- [x] **T4**: 更新 `go.mod` + 配置文档
  - 添加 `github.com/golang/snappy` 依赖
  - 添加 `github.com/google/uuid` 依赖（异步去重 token）
  - YAML 配置示例更新
  - Estimate: 0.5h
  - Dependencies: None

---

## Milestone 3: Test（P0）

### Definition of Done
- [x] 3 个新 L5 测试点全部 IMPLEMENTED
- [x] 现有压缩/快照测试回归通过
- [x] `-race` 异步测试零警告

### Tasks

- [x] **T5**: 编写性能测试
  - `async_compact_test.go`: 异步执行 + 去重 + Shutdown
  - `store_test.go`: Snappy 压缩/解压 + 兼容性 + 阈值
  - `compression/pipeline_test.go`: 异步集成
  - L5: L5-CTX-31, -32, -33
  - Estimate: 4h
  - Dependencies: T1-T4

---

## 任务统计

| Milestone | 任务数 | 预估 |
|-----------|--------|------|
| M1 Async Compact | 2 | 5h |
| M2 Snappy | 2 | 2.5h |
| M3 Test | 1 | 4h |
| **合计** | **5** | **~11.5h** |

---

## 依赖关系图

```
T1 ── T2 ──┐
            ├── T5
T3 ────────┤
            │
T4 ────────┘
```

## 执行顺序建议

1. **并行**: T1, T3, T4
2. **串行**: T2 → (等待 T1)
3. **最终**: T5 → (等待 T2, T3, T4)
