# Tasks: D3 LLM Gateway v2.0 — 价值流物理路径迁移

**Change ID:** devrix-d3-sa-refine-v2.0
**Demand ID:** DM-20260614-019
**Status:** S2_Clarified（S3 阶段产物待 S2 启动）
**Phases:** S1 demand → S2 proposal（本文件）→ S3 design → S3-Gate reviews → S4 实施（Phase F）→ S5 验收（Phase G）→ S6 归档

> **不估时**（playbook 原则 + OpenSpec S2 阶段约束）。任务按 OpenSpec S1-S6 阶段排列；同阶段任务可并行。S4 阶段 F 任务有显式依赖：F2/F3/F4/F6/F7/F8 各自独立可并行；F5（breaker+retry 合并）需 F2-F4 完成；F9（contracts.go 拆分）必须所有 F 完成后；F11（layering.md 同步）需 F2-F9 全完成。

---

## S1 — 需求澄清（已完成）

| ID | Task | 状态 |
|----|------|------|
| S1.1 | 创建 `openspec/changes/devrix-d3-sa-refine-v2.0/` 目录 | ✅ |
| S1.2 | 写 `demand.md` v0.1（8 F 物理迁移 + contracts.go 拆分 + 15 AC + 6 风险） | ✅ |
| S1.3 | 父 change 状态更新（`demand-archive-index.md` line 148） | ✅ |

---

## S2 — 提案（已完成）

| ID | Task | 依赖 | 状态 |
|----|------|------|------|
| S2.1 | 写 `proposal.md` v0.1（5+1 S 切法继承 + 7 路径迁移 + re-export 桥接契约 + contracts.go 拆分方案） | S1.2 | ✅ |
| S2.2 | 写 `tasks.md` v0.1（本文件） | S1.2 | ✅ |
| S2.3 | `demand.md` 状态推进到 S2_Clarified | S2.1 | ✅ |

---

## S3 — 设计

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| S3.1 | 写 `design.md` v3.2.0（路径映射表 + 拆分步骤 C1–C5 + F 编排沿用 + T 映射沿用 + Phase F/G 风险） | S2.3 | `openspec/specs/d3-llm-gateway/design.md` v3.2.0 |
| S3.2 | 写 `code-layout.md` §4.4 D3 scenario-slug 注册表 v1.5.0（新增 stream/route/protect/budget/guard/configure slug） | S3.1 | code-layout.md v1.5.0 |
| S3.3 | 写 `layering.md §Domain Layout` D3 章节（v3.8.0 物理路径映射） | S3.1 | layering.md v3.8.0 |
| S3.4 | 更新 `cross-domain-boundaries.md`（v1.0.0 → v1.1.0，新增 v2.0 物理迁移 § 3） | S3.1 | cross-domain-boundaries.md v1.1.0 |

> S3 产物：4 个 spec 文档同步；与 v1.0/v1.1 不冲突。

---

## S3-Gate — 评审

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| G1 | 写 `review-r1.md`（Owner 评审：5+1 S 切法继承 + 路径映射 + 拆分方案） | S3.4 | review-r1.md |
| G2 | 写 `review-r2.md`（结构层：contracts.go 拆分粒度 + re-export 桥接设计） | S3.4 | review-r2.md |
| G3 | 写 `review-r3.md`（运行层：物理迁移对 T 与运行时 span/metric/config 的影响） | S3.4 | review-r3.md |
| G4 | R1+R2+R3 全部闭合 + 接力接口就位 | G1–G3 | S3-Gate Cleared |

> S3-Gate 产物：3 个 review 文件 + S3-Gate 决议（与 v1.0 R1/R2/R3 同型但更紧凑，因 v1.0 已确立设计）。

---

## S4 — 实施（Phase F 物理迁移）

### F2 — `stream/adapter/` 迁移（高风险）

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| F2.1 | 创建 `internal/layers/llmgateway/stream/adapter/` 目录 | S3-Gate | 目录 |
| F2.2 | 物理移动 `adapter/*.go`（deepseek.go / minimax.go / protocol.go / registry_test.go / 3 个 test 文件） | F2.1 | stream/adapter/ 内容 |
| F2.3 | 物理移动 `gateway/breaker_observer.go` 引用方（v1.1 observer） | F2.1 | stream/adapter/ 引用更新 |
| F2.4 | 创建 `internal/layers/llmgateway/adapter/` 桥接文件（re-export） | F2.2 | 旧路径 1 发布周期兼容 |
| F2.5 | 更新所有 `import "internal/layers/llmgateway/adapter"` → `stream/adapter` | F2.2 | D3 内部 + 1 处 bridge.go + 1 处 tests/integration |
| F2.6 | `go build ./internal/layers/llmgateway/...` 绿 | F2.2–F2.5 | 子包编译 |

### F3 — `route/` 迁移（中风险）

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| F3.1 | 创建 `internal/layers/llmgateway/route/` | F2.* | 目录 |
| F3.2 | 物理移动 `gateway/router.go` | F3.1 | route/router.go |
| F3.3 | 创建 `gateway/router.go` 桥接（re-export） | F3.2 | 旧路径 1 发布周期 |
| F3.4 | 更新所有 `import "internal/layers/llmgateway/gateway/router"` → `route/router` | F3.2 | 引用方更新 |
| F3.5 | `go build` 绿 | F3.4 | 子包编译 |

### F4 — `stream/gateway.go` 主实现迁移（中风险）

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| F4.1 | 物理移动 `gateway/gateway.go` → `stream/gateway.go` | F3.* | stream/gateway.go |
| F4.2 | 创建 `gateway/gateway.go` 桥接 | F4.1 | 旧路径 |
| F4.3 | 更新所有 import + 测试文件 | F4.1 | 引用方 |
| F4.4 | `go build` 绿 | F4.3 | 子包编译 |

### F5 — `protect/` 合并迁移（高风险）

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| F5.1 | 创建 `internal/layers/llmgateway/protect/` | F4.* | 目录 |
| F5.2 | 物理移动 `breaker/*.go` + `retry/*.go`（circuit_breaker.go / state.go / observer.go / retry.go / retry_jitter.go） | F5.1 | protect/ 内容 |
| F5.3 | 创建 `breaker/` 桥接 + `retry/` 桥接 | F5.2 | 旧路径 |
| F5.4 | `protect/breaker_observer.go`（v1.1 observer）保持独立 .go | F5.2 | F 编排下 2 个 F 保留 |
| F5.5 | 更新所有 import + 完整 P0 回归（Breaker 4 T + Retry 2 T） | F5.2 | 11 P0 T 全绿 |
| F5.6 | `go build` + `go test` 绿 | F5.5 | 子包编译测试 |

### F6 — `budget/` 迁移（中风险）

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| F6.1 | 创建 `internal/layers/llmgateway/budget/` | F5.* | 目录 |
| F6.2 | 物理移动 `token/*.go`（counter.go / counter_test.go） | F6.1 | budget/ 内容 |
| F6.3 | 创建 `token/` 桥接 | F6.2 | 旧路径 |
| F6.4 | 更新 import + Token P0 回归（T01/T02/T03） | F6.2 | 测试绿 |
| F6.5 | `go build` + `go test` 绿 | F6.4 | 子包编译测试 |

### F7 — `guard/` 迁移（中风险）

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| F7.1 | 创建 `internal/layers/llmgateway/guard/` | F6.* | 目录 |
| F7.2 | 物理移动 `safety/*.go`（filter.go / patterns.go / filter_test.go） | F7.1 | guard/ 内容 |
| F7.3 | 创建 `safety/` 桥接 | F7.2 | 旧路径 |
| F7.4 | 更新 import + Safety P0 回归（T01 critical + v1.1 T03 latency） | F7.2 | 测试绿 |
| F7.5 | `go build` + `go test` 绿 | F7.4 | 子包编译测试 |

### F8 — `configure/` 迁移（跨包，中-高风险）

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| F8.1 | 创建 `internal/layers/llmgateway/configure/` | F7.* | 目录 |
| F8.2 | 物理移动 `config/*.go`（loader.go / loader_test.go） | F8.1 | configure/ 内容 |
| F8.3 | 物理移动 `shared/config/llmgateway.go` → `configure/shared_config.go` | F8.1 | configure/ 内容 |
| F8.4 | 物理移动 `shared/config/llmgateway_features_test.go` | F8.3 | configure/ 测试 |
| F8.5 | 创建 `config/` 桥接 + `shared/config/llmgateway*.go` 桥接 | F8.2–F8.4 | 旧路径 |
| F8.6 | 更新 import（跨包：shared/config/llmgateway.go → llmgateway/configure/shared_config.go） | F8.3 | 跨包引用 |
| F8.7 | F9 v1.1 T02（feature flag 8 组合）回归 | F8.4 | 测试绿 |
| F8.8 | `go build` + `go test` 绿 | F8.7 | 子包编译测试 |

### F9 — `contracts.go` 拆分（高风险，必须 F2-F8 完成后）

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| F9.1 | 在各子包创建 `contracts.go`（子包内） | F2-F8 | 子包 contracts |
| F9.2 | 从根 `contracts.go` 移除已迁类型，添加 `// Deprecated:` 类型别名指向新位置 | F9.1 | 根 < 200 行 |
| F9.3 | 同步更新所有 `import` 路径（自底向上：子包 → 子包） | F9.1 | `goimports` 通过 |
| F9.4 | 旧 import 路径兼容（re-export 类型别名） | F9.2 | G2 测试绿 |
| F9.5 | v1.0 + v1.1 26 T + 9 T 测试 import 同步 | F9.3 | G5 11 P0 + 26 T 回归 |
| F9.6 | `go build ./...` 全绿 | F9.5 | 整体编译 |

### F10 — `bridges/llm/` 不变

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| F10.1 | 验证 `internal/bridges/llm/` 路径与 import 未变 | F9.6 | 跨域锚点稳定 |

### F11 — `layering.md` v3.8.0（文档同步）

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| F11.1 | 更新 `layering.md §Domain Layout` D3 章节，6 个 slug 物理路径 | F9.6 | layering.md v3.8.0 |
| F11.2 | 更新 `code-layout.md §4.4` D3 scenario-slug 注册表 v1.5.0 | F11.1 | code-layout.md v1.5.0 |

---

## S5 — 验收（Phase G 验证）

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| G1 | `go build ./...` 全绿 | F11.2 | 全绿 |
| G2 | `go test -race ./internal/layers/llmgateway/... ./internal/bridges/llm/...` 全绿 | G1 | 全绿 |
| G3 | `go vet ./...` 无新增警告 | G1 | 0 新增 |
| G4 | 旧路径 dead code 清理（**v2.0 + 1 release 实施；本期留 TODO**） | G1 | TODO 注释 + `depguard` 规则 |
| G5 | 完整 P0 回归（11 P0 T 100% 绿；v1.1 9 新 T 仍 IMPLEMENTED） | G2 | 11/11 绿 |
| G6 | `scripts/check_t_aliases.py` 退出码 0（26 alias 100% 覆盖） | G2 | 校验通过 |
| G7 | `grep -r "D3-S[1-7]" openspec/specs/` 无新增失同步 | G1 | 无失同步 |
| G8 | runtime span / metric / YAML config key 字面量未改（v1.0 不变性承诺） | G1 | 全绿 |
| G9 | `internal/bridges/llm/` 路径不变 | G1 | 路径稳定 |
| G10 | `contracts.go` 拆分后，根 < 200 行 | F9.6 | < 200 行 |
| G11 | `code-layout.md §4.4` 7 个新 slug 注册 | F11.2 | 注册完整 |
| G12 | `layering.md` v3.8.0 D3 章节更新 | F11.1 | 更新完成 |
| G13 | 写 `acceptance-report（v2.0）`（15 AC 全部裁决） | G1–G12 | acceptance-report.md |

---

## S6 — 归档

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| Arch.1 | `openspec/changes/devrix-d3-sa-refine-v2.0/` → `openspec/archive/2026-06-14-devrix-d3-sa-refine-v2.0/` | G13 | archive 目录 |
| Arch.2 | 更新 `openspec/demand-archive-index.md` 主表：DM-20260614-019 → ACCEPTED + archive 路径 + 底部归档说明 | Arch.1 | index 更新 |
| Arch.3 | 更新 `openspec/demand-archive-index.md` Active Changes 表：移除 v2.0 行（已归档） | Arch.2 | index 同步 |
| Arch.4 | 推进父 change `devrix-d3-sa-refine` 状态：S5_Pass → S7_Archived（v1.0 + v1.1 + v2.0 全部完成） | Arch.3 | 父 change 归档协议 |
| Arch.5 | 提交 commit + push origin master（v2.0 物理迁移） | Arch.1 | commit |
| Arch.6 | 写 `acceptance-report（v2.0）` 同步 S6 状态 | Arch.1 | 报告完成 |

---

## 任务依赖图

```
S1 demand ✅ ── S2 proposal ✅ ── S3 design ── S3-Gate reviews
                                                     │
                                                     ▼
              F2 stream/adapter ──┐
              F3 route             │
              F4 stream/gateway    ├── 并行
              F6 budget            │
              F7 guard             │
              F8 configure         │
                                  │
              F5 protect (高风险合并) ── 需 F2-F4 完成
                                  │
                                  ▼
              F9 contracts.go 拆分 ── 需 F2-F8 全完成
                                  │
                                  ▼
              F10 bridges/llm 不变
                                  │
              F11 layering.md + code-layout.md
                                  │
                                  ▼
              Phase G 验证（G1-G13）── G13 acceptance-report v2.0
                                  │
                                  ▼
              Phase Arch 归档（Arch.1-6）── push origin master
```

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：S1-S6 全阶段任务分解；F2-F11 + G1-G13 + Arch.1-6 全部就位 |
