# Tasks: D3 LLM Gateway S/A 重切

**Change ID:** devrix-d3-sa-refine
**Demand ID:** DM-20260614-016
**Status:** S3-Gate Cleared（S3 阶段产物待 Phase B 启动）
**S3-Gate Verdict:** R1+R2+R3 全部闭合（详见 `review-r1.md` / `review-r2.md` / `review-r3.md`）
**Phases:** v1.0 Registry（Phase A-C）→ v1.1 Traceability（Phase D-E）→ v2.0 Structure（Phase F-G）

> **不估时**（playbook 原则 + OpenSpec S2 阶段约束）。任务按 Phase 排列；同一 Phase 内任务可并行。Phase 间有显式依赖。

---

## Phase A — 文档澄清（S1 → S2 → S3-Gate，已完成）

| ID | Task | 依赖 | 状态 |
|----|------|------|------|
| A1 | 创建 `openspec/changes/devrix-d3-sa-refine/` 目录 | — | ✅ |
| A2 | 写 `demand.md` v0.1（4 轴 Review + 5+1 S 切法 + Decision D1/D2/D3） | — | ✅ |
| A3 | 用户评审 3 个 Decision（R1） | A2 | ✅ APPROVED |
| A4 | 更新 `demand.md` v0.2 状态到 S2_Clarified + 写入 R1 7 项澄清 | A3 | ✅ |
| A5 | 写 `proposal.md`（D + S 切法，不含 A/F/T） | A4 | ✅ |
| A6 | 写 `review-r1.md`（Review R1 决议记录） | A5 | ✅ |
| A7 | 写 `tasks.md`（Phase A 末尾骨架） | A5 | ✅ |
| A8 | 写 `review-r2.md`（5 结构层命题 + 4 OQ + 3 分歧） | A6 | ✅ |
| A9 | 写 `review-r3.md`（4 运行层命题 + 6 NQ + Owner 自裁决） | A8 | ✅ |
| A10 | R2 §6 接力接口 12 项 + R3 §1~4 命题 4 个 + R3 §5 NQ 6 个全部闭合 | A8 + A9 | ✅ |
| A11 | `demand.md` v0.3 状态推进到 S3_Design_Gate_Cleared | A10 | ✅ |

> Phase A 产物：`demand.md` v0.3 + `proposal.md` v0.1 + `review-r1.md` v0.1 + `review-r2.md` v0.1 + `review-r3.md` v0.1 + `tasks.md`（本文件）

---

## Phase B — v1.0 Registry 重排

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| B1 | 写 `devrix/scripts/check_t_aliases.py`（alias 覆盖率校验脚本） | — | scripts/ |
| B2 | 重排 `openspec/specs/d3-llm-gateway/a-registry.md`（5+1 S） | A11 | a-registry.md v3.0.0 |
| B3 | 重排 `openspec/specs/d3-llm-gateway/f-registry.md`（每 S 重新分组） | B2 | f-registry.md v3.0.0 |
| B4 | 重排 `openspec/specs/d3-llm-gateway/t-registry.md`（新 S/A/T ID + `<!-- Mechanism: -->` 注释） | B3 | t-registry.md v3.0.0 |
| B5 | 写 `t-registry.md §Legacy Archive`（26 条 alias） | B4 | t-registry.md §Legacy |
| B6 | 重排 `openspec/specs/d3-llm-gateway/span-registry.md`（按 S 分组） | B4 | span-registry.md v3.0.0 |
| B7 | 重写 `openspec/specs/d3-llm-gateway/spec.md`（5+1 S + North Star 5 承诺 + R2 灰区声明） | B4 | spec.md v3.0.0 |
| B8 | 重写 `openspec/specs/d3-llm-gateway/design.md`（A + F 编排 + 物理映射 + R2 §4.3 拆分粒度 + R3 命题 A~D 衍生） | B2 + B3 | design.md v3.0.0 |
| B9 | 同步 `openspec/specs/architecture/layering.md §D3`（5+1 S） | B2 | layering.md v3.7.0 |
| B10 | 补 `openspec/specs/architecture/code-layout.md §4` D3 scenario-slug 注册表 | B7 | code-layout.md v1.4.0 |
| B11 | 新建 `openspec/specs/architecture/cross-domain-boundaries.md`（D3 vs D2/D5/D6 边界 + R2 命题 E 灰区声明） | A11 | cross-domain-boundaries.md v1.0.0 |
| B12 | `go build ./...`（v1.0 无代码变更，应保持绿） | B2–B11 | 全绿 |
| B13 | 跑 `scripts/check_t_aliases.py`（26 条 alias 100% 覆盖） | B5 | 校验通过 |
| B14 | `grep -r "D3-S[1-7]" openspec/specs/d3-llm-gateway/` 一致性检查 | B2–B11 | 无失同步 |

> Phase B 产物：4 注册表 v3.0.0 + `spec.md` v3.0.0 + `design.md` v3.0.0 + `layering.md` v3.7.0 + `code-layout.md` v1.4.0 + `cross-domain-boundaries.md` v1.0.0 + alias 校验脚本
>
> **修订说明**：B8（design.md 重写）从原"留待 S3-Gate 后另立 change"提前到 Phase B 必出。原因：S3-Gate Cleared 后 design.md 与 spec.md 同步产出，与 v1.0 + v1.1 合并发布（R1 Q5）一致；design.md 中"R2 §4.3 contracts.go 拆分粒度"作为决策占位，"实际拆分"推迟到 v2.0 Phase F。

---

## Phase C — v1.0 验证

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| C1 | `grep -r "D3-S[1-7]" openspec/specs/` 一致性检查 | B2–B10 | 无失同步 |
| C2 | 跑所有 11 条 P0 T | B4 | 11/11 绿 |
| C3 | 跑所有 26 条 T 全量 | B4 | 26/26 绿 |
| C4 | Bridge 跨域归位验证：D3 内部 f-registry 已无 F04/F05 | B3 | CROSS 段已注册 |
| C5 | `demand-archive-index.md` 末尾追加 D3 入口 | B2 | index 更新 |
| C6 | 写 `acceptance-report（v1.0）` | C1–C5 | acceptance-report.md |

> Phase C 产物：`acceptance-report（v1.0）` 状态 = **ACCEPTED**

---

## Phase D — v1.1 韧性可见性

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| D1 | 设计 metric `llm_breaker_state{provider,state}`（D3 → D5） | C6 | span-registry.md §Metrics |
| D2 | 实现 D3-S3 ProtectCall 状态切换时 emit `llm_breaker_state` | D1 | `llmgateway/protect/circuit_breaker.go`（v2.0 物理路径占位注释） |
| D3 | 复用现有 EngineEvent（`FlowStarted` / `FlowFailed`）覆盖 D3 → D7 通知 | C6 | d7-orchestration spec 引用 |
| D4 | D6 增 probe #1：Tier 解析正确性（覆盖率 ≥ 99%） | C6 | d6-evolution spec 补丁 |
| D5 | D6 增 probe #2：Breaker 状态切换次数（异常切换告警） | D2 | d6-evolution spec 补丁 |
| D6 | D6 增 probe #3：Token 预算触发率（截断 / 报错次数） | C6 | d6-evolution spec 补丁 |
| D7 | 写 `openspec/changes/devrix-d3-sa-refine-v1.1/demand.md`（v1.1 子 change） | C6 | 启动 v1.1 子 change |

> Phase D 产物：v1.1 子 change（独立 DM ID） + d6-evolution spec 补丁 + span-registry.md §Metrics

---

## Phase E — v1.1 验证

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| E1 | 4 表追溯 S → A → F → T → Span 完整性校验 | D1–D6 | 全链路一致 |
| E2 | 跨域回归：D2/D4 消费 D3 行为不变 | D3 | 行为 bit-identical |
| E3 | D5 接收 `llm_breaker_state` 写入 observability 持久化 | D2 | dashboard 验证 |
| E4 | D6 3 probe 接入并跑通 | D4–D6 | probe 绿 |
| E5 | 写 `acceptance-report（v1.1）` | E1–E4 | acceptance-report.md |

> Phase E 产物：`acceptance-report（v1.1）` 状态 = **ACCEPTED**

---

## Phase F — v2.0 物理迁移

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| F1 | 写 `openspec/changes/devrix-d3-sa-refine-v2.0/demand.md`（v2.0 子 change） | E5 | 启动 v2.0 子 change |
| F2 | 物理迁移 `adapter/` → `stream/`（含 re-export 桥接） | F1 | 物理目录 |
| F3 | 物理迁移 `gateway/router.go` → `route/` | F1 | 物理目录 |
| F4 | 物理迁移 `gateway/gateway.go` Stream 主实现 → `stream/` | F1 | 物理目录 |
| F5 | 物理迁移 `breaker/` + `retry/` → `protect/` | F1 | 物理目录 |
| F6 | 物理迁移 `token/` → `budget/` | F1 | 物理目录 |
| F7 | 物理迁移 `safety/` → `guard/` | F1 | 物理目录 |
| F8 | 物理迁移 `config/` + `shared/config/llmgateway.go` → `configure/` | F1 | 物理目录 |
| F9 | `contracts.go` 拆分到各子包；根保留 re-export | F2–F8 | contracts.go 拆分 |
| F10 | `bridges/llm/` 路径不变（跨域锚点） | — | 保持 |
| F11 | `layering.md §Domain Layout` 更新 D3 章节 | F2–F8 | layering.md v3.8.0 |

> Phase F 产物：物理目录与价值流 S 1:1 对齐 + `layering.md` v3.8.0

---

## Phase G — v2.0 验证 + 归档

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| G1 | `go build ./...` 全绿 | F2–F9 | 全绿 |
| G2 | `go test ./internal/layers/llmgateway/... ./internal/bridges/llm/...` 全绿 | F2–F9 | 全绿 |
| G3 | `go vet` 无新增警告 | F2–F9 | 0 新增 |
| G4 | 旧路径 dead code 清理（1 发布周期后） | G1 | 旧路径删除 |
| G5 | 完整 P0 回归 | G2 | 11/11 绿 |
| G6 | change 包归档到 `openspec/archive/2026-MM-DD-devrix-d3-sa-refine/` | G1–G5 | archive 目录 |
| G7 | `demand-archive-index.md` 标记 D3 价值流化完成 | G6 | index 更新 |
| G8 | 写 `acceptance-report（v2.0）` + S7 归档报告 | G6 | 归档报告 |

> Phase G 产物：v2.0 acceptance-report 状态 = **ACCEPTED** + archive 完成

---

## 任务依赖图

```
Phase A (已完成)
   ↓
Phase B (v1.0 Registry)
   ├─ B1 (脚本)
   ├─ B2..B10 (注册表重排, 串行)
   └─ B11, B12 (校验)
   ↓
Phase C (v1.0 验证)
   ↓
Phase D (v1.1 韧性可见性, 启动子 change)
   ↓
Phase E (v1.1 验证)
   ↓
Phase F (v2.0 物理迁移, 启动子 change)
   ↓
Phase G (v2.0 验证 + 归档)
```

---

## 后续子 change 计划

| 子 change | DM ID | 范围 | 启动时机 |
|----------|------|------|---------|
| `devrix-d3-sa-refine-v1.1` | DM-YYYYMMDD-NNN（待 S3 申请） | D3 → D5/D7 韧性状态 emit + D6 3 probe | Phase D 启动 |
| `devrix-d3-sa-refine-v2.0` | DM-YYYYMMDD-NNN（待 S3 申请） | 物理路径迁移 + contracts.go 拆分 | Phase F 启动 |

> **v1.0 子 change 命名**：`devrix-d3-sa-refine`（当前 change 既是 v1.0 容器，也是文档澄清 change；v1.0 Registry 变更直接在本 change 完成）

---

## 校验脚本占位（Phase B 写）

```python
# devrix/scripts/check_t_aliases.py
# 用途：校验 t-registry.md §Legacy Archive 100% 覆盖
# 依赖：openspec/specs/d3-llm-gateway/{a,f,t}-registry.md
# 退出码：0 覆盖率=100%；1 否则
# 实现：解析 t-registry.md，提取旧 ID → 新 ID 映射，扫描 spec.md/ 设计稿/ 测试文件，
#       确认每个旧 ID 都能追溯到新 ID；输出未覆盖列表。
```

> 脚本实现细节留待 Phase B 启动时写，本 tasks.md 仅占位。

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：Phase A-G 任务分解骨架 + 依赖图 + 后续子 change 计划 |
