# Design: D7 编排层 MUPS 包路径迁移

**Change ID:** `devrix-d7-mups-package-migration`
**Demand ID:** DM-20260626-002
**Status:** S3_Design → S3-Gate Approved → S4_Implemented → S5_Accepted → S7_Archived
**优先级:** P1
**前置:** devrix-d7-six-s-simplification (DM-20260626-001) S7_Archived (PR #215)
**SoT 文档:** `openspec/archive/2026-06-26-devrix-d7-six-s-simplification/acceptance-report.md` §7 后续工作

---

## 0. S3-Gate Review Conclusion (2026-06-26)

**Reviewer:** Agent self-review (单人团队)
**Method:** Standard Review per `openspec/specs/project/review-design.md` §3.2
**Conclusion:** **Approved** (with minor suggestions — see §11 Follow-ups)

### §2.1 架构决策审查

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 层归属正确 | ✅ Pass | D7 编排层 (D7-S6 MUPS Pipeline) 子树迁移，0 跨域 |
| 接口方向正确 | ✅ Pass | 0 接口变化（仅 import path） |
| 不重复造轮子 | ✅ Pass | 复用现有 `package execute` / `package learn` 声明 |
| 跨层依赖最小 | ✅ Pass | 仅 D7 域内迁移，不跨域 |
| 设计决策有记录 | ✅ Pass | Decision §8 三条全部记录 |

### §2.2 需求完整性审查

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 需求可追溯 | ✅ Pass | demand.md AC1-AC10 → proposal.md §3 → design.md §2 → spec.md 4 Requirement 全链 |
| 验收标准覆盖 | ✅ Pass | 10 AC 全部映射到 5 Scenario（AC1→T01, AC2→T02, AC3→T03, AC4→T03, AC5→T04, AC6→T04, AC7→T04, AC8→T03, AC9→T01-T04, AC10→§follow-up 备注） |
| Out of Scope 明确 | ✅ Pass | proposal.md §7 + .openspec.yaml `out_of_scope` 共 8 项 |
| DM ID 无冲突 | ✅ Pass | DM-20260626-002 (当日 002，前 001 已分配给 six-s-simplification) |

### §2.3 规格质量审查

| 检查项 | 结果 | 说明 |
|--------|------|------|
| Gherkin 格式正确 | ✅ Pass | GIVEN/WHEN/THEN/AND 完整；3 Requirement × 4 Scenario 共 7 个 |
| Happy path + sad path | ✅ Pass | happy (目录创建) + sad (零残留) + sad (build/test fail) + 边界 (LP-1/LP-2/LP-5 兼容) 全部覆盖 |
| 并发场景覆盖 | ✅ N/A | 纯目录迁移，无共享状态变更 |
| 错误路径覆盖 | ✅ Pass | 零残留 grep + 22 包 PASS + LP 路径 0 变化 |
| T 层映射完整 | ✅ Pass | 每个 Requirement 标注 T01/T02/T03/T04 |

### §2.4 风险审查

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 回归风险已评估 | ✅ Pass | design.md §6 8 项风险评估 + 等级 + 缓解 |
| 回滚方案可行 | ✅ Pass | design.md §7 git revert + 5 分钟机械式反向操作 |
| 性能影响已评估 | ✅ N/A | 0 运行时行为变化 |

**Grill Review 决策点：**
1. **为什么选 B (保留 package execute / package learn) 不选 A (改 package mups)？** — Agreed。理由：Go 包名与目录名解耦是语言特性；改 A 会导致 17 处内部 cross-file 引用 + 15 处外部 import 全改，变更面放大 2x+。
2. **为什么不在本次 PR 合并 execute + learn 为单 mups 包？** — Agreed。理由：保留 Channel vs Memory 语义分离 (Pipeline Coordinator + Memory Curator 双角色)；6 S 文档语义已对齐，物理合并会破坏语义分离。
3. **为什么不在本次 PR 改 bootstrap wire？** — Agreed。理由：14→6 wire 收口需要其他 5 个 follow-up 全部落地后才能安全收敛；本次抢先会引入与未来 PR 的冲突风险。

**结论：Approved（2026-06-26）— 进入 S4 实现阶段。**

---

## 1. Root Cause Analysis

### 1.1 直接根因

`devrix-d7-six-s-simplification` (DM-20260626-001) 在 v6.0.0 域升级中把 D7 编排层 14 S 博弈角色精简为 6 S + 1 横切，9 个 spec 文档已在 PR #215 (commit 0ce5e52) 中完整重写并 S7_Archived。但代码包路径未做迁移：

| 当前包路径 | 物理 .go 数 | 应归 S 层 |
| ---------- | ----------- | --------- |
| `internal/layers/orchestration/execute/` | 7 | S6 MUPS Pipeline |
| `internal/layers/orchestration/learn/` | 17 | S6 MUPS Pipeline |

**因果链：**
1. PR #215 重写 spec 文档，14 S → 6 S+1 横切描述与代码层产生不一致
2. spec 文档说"S6 MUPS Pipeline = Execute 4 Channel + Learn 3 通道"，但物理仍是两个独立顶层包
3. `execute/` 与 `multiagent/execute/` 同名不同域，需靠完整 import path 区分
4. bootstrap wire 节点（14 个）与 6 S 角色不对应
5. 未来所有 6 S 全部落地后还需再做一次 wire 收敛（成本叠加）

### 1.2 深层根因

**博弈角色与物理拓扑未同步演进：**
- 博弈角色（语义层）已收敛为 6 S
- 物理拓扑（Go 包目录）仍按 14 S 时期散落
- 两者演进速度不同步导致"spec 文档清晰，但代码目录混乱"

**约束（用户硬约束）：**
- 函数签名/行为/对外接口 0 变化（纯目录迁移 + import path 替换）
- `package execute` / `package learn` 声明不变
- LP-1（Bayesian reputation）/ LP-2（Memory 3 通道）/ LP-5（Cross-session traceability）路径 0 变化

---

## 2. Solution Design

### 2.1 核心方案

把 `orchestration/execute/` 和 `orchestration/learn/` 物理迁移到 `orchestration/mups/` 子树下，**包名保持不变**：

```
orchestration/                         orchestration/
├── execute/                           ├── mups/  (NEW)
│   ├── channel.go                     │   ├── execute/   ← execute/* (7 .go)
│   ├── ... (7 files)                  │   │   ├── channel.go
│   └── execute_test.go                │   │   └── execute_test.go
└── learn/                             │   └── learn/     ← learn/* (17 .go)
    ├── adaptive_prior.go              │       ├── adaptive_prior.go
    ├── ... (17 files)                 │       └── testhelpers_test.go
    └── testhelpers_test.go            ├── decisionplanning/  (不变)
                                       ├── executionflow/     (不变)
                                       ├── sessionorchestrator/ (不变)
                                       └── ... (其他 12 个目录)
```

**关键设计选择：保留 package 声明不变**

| 方案 | 优点 | 缺点 | 选择 |
|------|------|------|------|
| A. 改 `package mups` | 物理目录与包名一致 | 内部 17 处 cross-file 引用 + 15 处外部 import 全改；测试 fixture 全失效 | ❌ |
| **B. 保持 `package execute` / `package learn`** | **内部 cross-file 引用 0 改动；只改 15 处外部 import path** | **包名与目录名不一致** | ✅ |

**理由：**
- 内部 cross-file 引用（如 `learn/learning_asset.go` 引用 `learn/asset_content.go`）基于 package name 而非 import path，迁移目录不影响
- 外部调用方只关心 `internal/layers/orchestration/mups/learn` 这一个 import path，与子包声明名无关
- 最小变更面 = 0 行为变化 + 0 函数签名变化 + 0 测试 fixture 失效

### 2.2 import path 替换映射（15 处）

| 调用方文件 | import 旧 | import 新 |
| ---------- | --------- | --------- |
| `orchestration/decisionplanning/classifier.go` | `orchestration/learn"` | `orchestration/mups/learn"` |
| `orchestration/orchtypes/anomaly_detector.go` | `orchestration/learn"` | `orchestration/mups/learn"` |
| `orchestration/orchtypes/intent_quantizer.go` | `orchestration/learn"` | `orchestration/mups/learn"` |
| `orchestration/orchtypes/observe_request.go` | `orchestration/learn"` | `orchestration/mups/learn"` |
| `orchestration/orchtypes/process.go` | `orchestration/learn"` | `orchestration/mups/learn"` |
| `orchestration/orchtypes/*_test.go` (4 files) | `orchestration/learn"` | `orchestration/mups/learn"` |
| `orchestration/sessionorchestrator/orchestrator.go` | `orchestration/learn"` | `orchestration/mups/learn"` |
| `orchestration/sessionorchestrator/autoclose.go` | `orchestration/learn"` | `orchestration/mups/learn"` |
| `orchestration/sessionorchestrator/tracing.go` | `orchestration/learn"` | `orchestration/mups/learn"` |
| `orchestration/sessionorchestrator/*_test.go` (7 files) | `orchestration/learn"` | `orchestration/mups/learn"` |

**execute 包：0 外部 import**（仅 `execute_test.go` 内部引用）

### 2.3 命名风险隔离

| 同名包 | 域 | 职责 | 是否本次迁移 |
| ------ | --- | ------ | ----------- |
| `internal/layers/orchestration/execute/` | D7 | ChannelRouter + 4 Channel | ✅ 迁移到 mups/execute/ |
| `internal/layers/multiagent/execute/` | D4 | WorkerExecutor | ❌ 不动（不同域不同职责） |

**风险隔离：**
- 迁移后 `internal/layers/orchestration/mups/execute/` (D7) 与 `internal/layers/multiagent/execute/` (D4) 物理路径完全不同，零歧义
- bootstrap wire 节点 `internal/bootstrap/wire_coordinator.go` 若涉及 execute/learn 引用同步更新

### 2.4 bootstrap 接线收敛（前置优化，不在本次 scope）

| 节点 | 现状 | v6.0.0 全部 follow-up 后目标 |
| ---- | ---- | --------------------------- |
| execute 包 wire | 14 wire 节点中 1 个 | 收口到 mups 子树 1 wire |
| learn 包 wire | 14 wire 节点中 1 个 | 收口到 mups 子树 1 wire |

**说明：** 本次 PR 仅迁移目录，bootstrap wire 收敛是后续 `devrix-d7-6s-bootstrap-slim` 的 scope。

---

## 3. Key Interfaces / Types

### 3.1 接口契约（0 变化）

**`package execute` 对外暴露接口（不变）：**
- `ChannelRouter` interface — 4 PlanKind → 4 ArtifactKind 1:1 路由
- `Channel` interface — 4 Channel 实现 (Commit / Protocol / Scenario / Exploration)
- `SideEffectStatus` enum — 5 态枚举

**`package learn` 对外暴露接口（不变）：**
- `Learner` interface — 4 类 LearningAsset (skill / feedback / scheduled / reputation) 学习入口
- `Memory` interface — 3 通道 (skill / feedback / scheduled) 持久化统一入口
- `ReputationStore` interface — Bayesian reputation 更新
- `AdaptivePrior` struct — 3 层 fail-safe Bayesian 状态
- `LearningAsset` struct — 4 类资产统一表示

### 3.2 import path 变化（唯一接口变更）

```go
// 旧
import "github.com/devrix/devrix/internal/layers/orchestration/learn"

// 新
import "github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
```

```go
// 旧
import "github.com/devrix/devrix/internal/layers/orchestration/execute"

// 新（如果有人引用过）
import "github.com/devrix/devrix/internal/layers/orchestration/mups/execute"
```

**注意：** execute 包 0 外部 import，无需实际更新。

---

## 4. Data Flow

本次迁移不涉及数据流变化（纯目录 + import path 替换）。所有数据流（ChannelRoute → Execute → Verify → Learn → ReputationUpdate）保持原状。

**E2E 关键路径（验证基线，0 变化）：**
```
Plan (S5-A07) → RouteChannel (S6-A02) → ExecuteArtifact (S6-A01) → 
VerifyVerdict (S4-A06) → BuildLearningAsset (S6-A06) → UpdateReputationEvidence (S6-A07) → 
BuildAdaptivePrior (S6-A08) → MemoryPersist (S6-A09) → RunLearner (S6-A10)
```

---

## 5. File Manifest（新增/修改/删除文件清单）

### 5.1 新增目录

| 路径 | 内容 |
| ---- | ---- |
| `internal/layers/orchestration/mups/` | NEW directory（包含 2 子目录） |
| `internal/layers/orchestration/mups/execute/` | 7 .go files（git mv from orchestration/execute/） |
| `internal/layers/orchestration/mups/learn/` | 17 .go files（git mv from orchestration/learn/） |

### 5.2 修改文件（15 处 import path 替换）

| 路径 | 修改 |
| ---- | ---- |
| `internal/layers/orchestration/decisionplanning/classifier.go` | 1 import 替换 |
| `internal/layers/orchestration/orchtypes/anomaly_detector.go` | 1 import 替换 |
| `internal/layers/orchestration/orchtypes/intent_quantizer.go` | 1 import 替换 |
| `internal/layers/orchestration/orchtypes/observe_request.go` | 1 import 替换 |
| `internal/layers/orchestration/orchtypes/process.go` | 1 import 替换 |
| `internal/layers/orchestration/orchtypes/*_test.go` (4 files) | 4 import 替换 |
| `internal/layers/orchestration/sessionorchestrator/orchestrator.go` | 1 import 替换 |
| `internal/layers/orchestration/sessionorchestrator/autoclose.go` | 1 import 替换 |
| `internal/layers/orchestration/sessionorchestrator/tracing.go` | 1 import 替换 |
| `internal/layers/orchestration/sessionorchestrator/*_test.go` (7 files) | 7 import 替换 |
| `internal/bootstrap/wire_coordinator.go` | 若涉及 execute/learn 引用，同步替换（目前 0 引用） |

### 5.3 修改文档

| 路径 | 修改 |
| ---- | ---- |
| `openspec/specs/d7-orchestration/d7-domain.md` v2.0.0 | §MUPS 5 节点管道挂载章节包路径描述更新 |
| `openspec/specs/d7-orchestration/design.md` v4.0.0 | §⑦ MUPS 5-node 6 S 归类包路径描述更新 |
| `openspec/specs/d7-orchestration/t-registry.md` v4.0.0 → v4.1.0 | 新增 D7-S6-A51 4 个 P0 T（PLANNED → IMPLEMENTED 收口后 v4.1.0 → v4.2.0） |
| `openspec/t-registry.md` v5.0.0 → v5.1.0 | 新增 DM-20260626-002 增量条目 |

### 5.4 删除目录

| 路径 | 内容 |
| ---- | ---- |
| `internal/layers/orchestration/execute/` | 整目录删除（git rm after git mv） |
| `internal/layers/orchestration/learn/` | 整目录删除（git rm after git mv） |

---

## 6. Regression Risk Assessment

| 风险点 | 等级 | 缓解措施 |
| ------ | ---- | -------- |
| **15 处 import path 替换遗漏** | 中 | `grep -rl "orchestration/learn\""` 必须 0 命中；CI 单测 100% PASS 是硬门禁 |
| **执行顺序导致 broken intermediate state** | 低 | 步骤 1 = `git mv` 全部文件 + 步骤 2 = `sed` 全仓替换 import path，分两次 commit 保证每步可独立验证 |
| **测试文件 fixture 路径 hardcode** | 中 | `git grep -n "orchestration/learn" ':!*vendor*' ':!*.md' ':!openspec/changes/*'` 0 命中；测试文件按 package 路径同步更新 |
| **CI 镜像缓存导致旧路径仍编译过** | 低 | 删除旧目录后强制 re-build 验证；CI 单测 100% PASS 是硬门禁 |
| **bootstrap wire_coordinator.go 中 learn 引用漏改** | 低 | 现状 0 引用（grep 0 命中）；pre-commit 二次确认 |
| **LP-1/LP-2/LP-5 行为漂移** | 极低 | 包名不变 + 函数签名不变 + 行为不变；Phase 6 + Phase 7 集成测试覆盖回归 |
| **22 包 regression 失败** | 中 | Step 3 跑全量 `-race` 回归；失败立即 revert |
| **与其他 follow-up PR 并行冲突** | 极低 | 其他 5 个 follow-up 尚未启动，本 change 独占 |

**回归风险评估：** **低风险**（纯目录迁移 + 机械式 import path 替换，0 行为变化）。

---

## 7. Rollback Plan

**若 S5 验收失败：**

1. `git revert <merge-commit>` 单 commit revert（PR 已 squash merge）
2. 重跑 22 包 `go test -race` 验证恢复
3. 若 revert 后仍异常（极不可能）：手动 `git mv orchestration/mups/{execute,learn}/* orchestration/` + 反向 sed 替换 import path

**回滚成本：** < 5 分钟（机械式反向操作）

---

## 8. Decision Records

### Decision: package name 保持不变（不改为 `package mups`）

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 改 package mups | 物理目录与包名一致 | 内部 17 处 cross-file 引用 + 15 处外部 import 全改；测试 fixture 全失效 |
| **B. 保持 package execute / package learn** | **内部 cross-file 引用 0 改动；只改 15 处外部 import path** | **包名与目录名不一致** |

**选择:** B
**理由:** Go 包名与目录名解耦是语言特性（package declaration 不必匹配 directory name）。保持 `package execute` / `package learn` 让 0 内部改动，只改外部 import path，最小化变更面。

### Decision: 不在本次 PR 改 bootstrap wire 节点

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 本次一并改 wire 14 → 6 | 一次到位 | 与 6 S 角色一一对应需要其他 5 个 follow-up 全部落地；本次抢先动 wire 会引入与未来 PR 的冲突风险 |
| **B. 本次只动目录 + import；wire 收口留作 devrix-d7-6s-bootstrap-slim** | **本次 PR 风险最低；wire 收口在 6 S 全部落地后做** | **需要 1 个额外 PR** |

**选择:** B
**理由:** 本次 change 是 Step 2 物理目录迁移的最小化变更；wire 收口是 Step 3 角色对齐，需要其他 5 个 follow-up 全部落地后才能安全收敛。

### Decision: 不在本次 PR 合并 `execute/` 和 `learn/` 为单 `mups` 包

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 合并为单 mups 包 | 子包深度 -1 | 需要重写 24 .go 文件 + 跨文件引用 + 测试 fixture；超出"物理迁移"范畴 |
| **B. 保留 `mups/execute/` 和 `mups/learn/` 两个子包** | **保留 Channel vs Memory 语义分离** | **mups/ 下仍有 2 个子目录** |

**选择:** B
**理由:** `execute/` (Channel 4 类) 和 `learn/` (Memory 3 通道) 在 6 S 文档中是 S6 MUPS Pipeline 下的两个独立子角色（Pipeline Coordinator + Memory Curator），合并会破坏语义分离。本次只迁移目录，不重构。

---

## 9. 相关决策对照

| 决策 | 选择 | 文档化位置 |
| ---- | ---- | ---------- |
| 14 S → 6 S + 1 横切 | A. 语义精简 + 5 新 P0/P1 Span | `openspec/archive/2026-06-26-devrix-d7-six-s-simplification/design.md` |
| Step 2: 物理目录迁移 | **本次** = `execute/` + `learn/` → `mups/` | 本 design.md §2 |
| Step 3: bootstrap wire 收口 | 后续 `devrix-d7-6s-bootstrap-slim` | `openspec/archive/2026-06-26-devrix-d7-six-s-simplification/acceptance-report.md` §7 follow-up PR 列表 |
| package name 策略 | 保持 `package execute` / `package learn` | 本 design.md §8 Decision 1 |