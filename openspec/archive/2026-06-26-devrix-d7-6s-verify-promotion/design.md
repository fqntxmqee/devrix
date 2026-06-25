# Design: D7 verify-promotion

**Change ID:** devrix-d7-6s-verify-promotion
**Status:** S3_Design
**Priority:** P1
**Created:** 2026-06-26
**DM:** DM-20260626-005

---

## 1. 总体设计

### 1.1 目标

把 `sessionorchestrator/` 包内的 3 个 Verify-衍生文件 (`exit_reason.go` + `verdict_to_exit_reason.go` + `verdict_to_exit_reason_test.go`, 共 218 行) 物理 promote 到 `executionflow/verify/`, 恢复 S4 ExecutionFlow + Verify (Costly Signaler + Certifier) 角色与代码归属的完全对齐, 为 follow-up #6 (6s-bootstrap-slim) 提供 wire 收敛前置条件。

**0 函数签名变化**: `ExitReason` 类型不变, 14 枚举值字符串不变, `VerdictToExitReason` 函数签名 `(v workmodel.Verdict, sessionID string) ExitReason` 不变。

### 1.2 范围

| 项目 | 数值 |
|------|------|
| 迁移文件 | 3 .go (218 行) |
| package 改名 | 3 文件 (`sessionorchestrator` → `verify`) |
| 跨包引用更新 | sessionorchestrator/turn_orchestrator.go 11 处 `ExitReason*` → `verify.ExitReason*` (含 state 字段 + 6 个常量 + 2 个函数参数类型 + 1 个 type assertion) |
| 测试代码更新 | turn_orchestrator_test.go 2 处 `ExitReasonNatural` → `verify.ExitReasonNatural` |
| spec 文档同步 | 4 文档 (d7-domain v2.3.0 + design v4.3.0 + t-registry v4.5.0 + 根 v5.5.0) |
| 新 P0 T | 4 个 (D7-S4-A50-T01..T04) |

## 2. 关键决策

### Decision 1: physical migration (git mv), not create new + delete old

| 选项 | 优劣 |
|------|------|
| **git mv** (采用) | 保留 git history (`git log --follow` 可追溯), 最小化 diff, review 友好 |
| create new + delete old | 失去 history, diff 噪声, 不利于后续 review |

**理由**: pure physical migration 标准做法。git mv 是 git 原生支持的 rename detection (default similarity threshold 50%), 100% rename detection 在 small file set 上必然达成。

### Decision 2: 0 函数签名变化, 保持 `func VerdictToExitReason(v workmodel.Verdict, sessionID string) ExitReason`

| 选项 | 优劣 |
|------|------|
| **保持原签名** (采用) | 0 函数签名变化 = LP-1/LP-2/LP-5 100% 兼容安全网, 集成测试零修改 |
| 增加 sessionID 参数 (用于 Phase 5 ReputationEvidence 归属) | sessionID 已存在但 reserved 状态 (`_ = sessionID`); Phase 5 未来按需激活 |
| 拆分 DeterministicExitReason (8) + VerifyExitReason (6) | 见 proposal.md §4 方案 B, 拒绝 |

**理由**: pure physical migration 安全网是本次 promote 的核心价值。sessionID 已预留参数位, 未来 Phase 5 启用零迁移成本。

### Decision 3: sessionorchestrator/turn_orchestrator.go 8 处 ExitReason* 全部替换为 verify.ExitReason*

| 选项 | 优劣 |
|------|------|
| **全部替换** (采用) | sessionorchestrator/ 包内 0 ExitReason 残留, S2 单包封装恢复, grep 0 残留保证 future-proof |
| 在 sessionorchestrator/ 保留 type alias `type ExitReason = verify.ExitReason` | 引入 wrapper alias, 增加 0 抽象层但未来变更需要同步两处 |
| 不替换, 仍保留 `import _ "executionflow/verify"` side-effect import | 不合规: sessionorchestrator 内部使用 verify.ExitReason 必须 explicit import |

**理由**: turn_orchestrator.go 是 ExitReason 的**消费者** (state 字段类型 + 6 个常量赋值 + 2 个函数参数), 不是 ExitReason 的**定义者**; 既然 ExitReason 物理迁移到 verify/, 消费者侧引用必须改为 `verify.ExitReason*`。这是单向引用方向: SessionOrchestrator (S2) 在 Turn 终止时消费 Verify (S4) 输出的 ExitReason, 符合 v6.0.0 S 层调用方向 (S4 → S2 反馈流, S2 → S4 数据消费流, 都是合法 DAG 边)。

### Decision 4: ExitReason 14 枚举值全部保留在 verify/ (含 8 deterministic + 6 verify-derived)

| 选项 | 优劣 |
|------|------|
| **14 枚举值全部留 verify/** (采用) | 单一 SoT, 类型完整, 8 deterministic 常量 SessionOrchestrator 仍可消费 (只是从 verify/ 包 import) |
| 8 deterministic 拆出 → sessionorchestrator/ + 6 verify-derived 留 verify/ | 类型 union, 大幅扩大 scope (见 proposal.md §4 方案 B) |

**理由**: ExitReason 是**类型系统**层面的概念, 不是**职责归属**层面的概念。8 deterministic + 6 verify-derived 在 verifier-driven decision tree 里属于同一个"Turn 终止原因"枚举空间; 强行拆分会破坏 LP-1 (Bayesian reputation) / LP-2 (5 节点管道) 集成测试中 `Metadata["exit_reason"]` 字符串值的稳定性。

### Decision 5: T 编号使用 `D7-S4-A50` (S4 ExecutionFlow + Verify 角色)

| 选项 | 优劣 |
|------|------|
| **D7-S4-A50** (采用) | S4 ExecutionFlow + Verify 角色下, 与现有 S4-A47 (system.anomaly_detect) 同序列, 语义对齐 |
| D7-S2-A51 | S2 SessionOrchestrator 角色下, 但本次 promote 是把 ExitReason 移出 S2, 编号放 S2 反而违和 |
| D7-S10-A50 | S10 是 v6.0.0 之前的旧 14 S 编号, v6.0.0 已重归类为 6 S, 不再使用 |

**理由**: T 编号应反映"哪个 S 层拥有此测试点", S4 ExecutionFlow + Verify 才是 exit_reason.go 的新归属。

## 3. 实施步骤

### Step 1: spec 同步 (前置, 0.5h)

**前置原因**: spec 文档必须在代码实现前先更新, 否则 S3-Gate review 时文档与代码不同步会 block review。

修改 4 文件:
1. `openspec/specs/d7-orchestration/d7-domain.md` v2.2.0 → v2.3.0 + 新增 v2.3.0 changelog row
2. `openspec/specs/d7-orchestration/design.md` v4.2.0 → v4.3.0 + 新增 v4.3.0 changelog row
3. `openspec/specs/d7-orchestration/t-registry.md` v4.4.0 → v4.5.0 + 新增 D7-S4-A50-T01..T04 + 域 t-registry 218 IMPLEMENTED → 222 IMPLEMENTED
4. `openspec/t-registry.md` (root) v5.4.0 → v5.5.0 + DM-20260626-005 增量条目 + 总 P0 350 → 354

### Step 2: 物理迁移 (1h)

```bash
cd /Users/fukai/workspace/devrix

# git mv 3 文件
git mv internal/layers/orchestration/sessionorchestrator/exit_reason.go \
       internal/layers/orchestration/executionflow/verify/exit_reason.go

git mv internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason.go \
       internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason.go

git mv internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason_test.go \
       internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason_test.go

# sed package 改名 (3 文件)
sed -i '' '1s|package sessionorchestrator|package verify|' \
  internal/layers/orchestration/executionflow/verify/exit_reason.go \
  internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason.go \
  internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason_test.go
```

### Step 3: 跨包引用更新 (1h)

**sessionorchestrator/turn_orchestrator.go**:

```bash
# 加 import "executionflow/verify"
sed -i '' 's|"github.com/devrix/devrix/internal/layers/orchestration/executionflow/bridge"|&\n\t"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"|' \
  internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go

# 8 处 ExitReason* → verify.ExitReason*
# 注: 变量名 exitReason (小写) 是 state 字段, 类型 ExitReason (大写) 是 import 引用, sed 区分
sed -i '' -E 's|\bExitReason\b|verify.ExitReason|g; s|\bExitReason([A-Z][a-zA-Z]*)\b|verify.ExitReason\1|g' \
  internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go
```

替换预期:
- `ExitReason ExitReason` → `verify.ExitReason verify.ExitReason` (state 字段类型) ✓
- `st.exitReason = ExitReasonMaxTurns` → `st.exitReason = verify.ExitReasonMaxTurns` (常量赋值) ✓
- `if exitReason == ExitReasonMaxTurns` → `if exitReason == verify.ExitReasonMaxTurns` (类型断言) ✓
- `func resolveFinalText(... exitReason ExitReason, ...)` → `func resolveFinalText(... exitReason verify.ExitReason, ...)` (参数类型) ✓
- `metadataKeyExitReason: string(exitReason)` → 不变 (string conversion, 0 类型引用) ✓

**sessionorchestrator/turn_orchestrator_test.go**:

```bash
# 2 处 ExitReasonNatural → verify.ExitReasonNatural (test 字段)
sed -i '' 's|ExitReasonNatural|verify.ExitReasonNatural|g' \
  internal/layers/orchestration/sessionorchestrator/turn_orchestrator_test.go
```

### Step 4: 验证 (1.5h)

```bash
# 编译 + lint + race 测试
go build ./...
go vet ./...
go test -race -count=1 ./internal/layers/orchestration/...  # 22/22 PASS 预期

# 集成测试 (LP-1/2/5 兼容验证)
go test -race -count=1 ./tests/integration/d7/...  # 现有 baseline PASS 预期

# baseline-stability 验证
git diff -- internal/layers/orchestration/sessionorchestrator/autoclose.go \
              internal/layers/orchestration/hardening/ \
              internal/layers/orchestration/escape/circuit_breaker.go
# 预期: 空 (本次 promote 不影响这些文件)

# sessionorchestrator/ 包内 ExitReason 残留验证
grep -rn "ExitReason[^a-zA-Z]" internal/layers/orchestration/sessionorchestrator/*.go \
  | grep -v "// "  # 排除注释
# 预期: 0 命中 (所有引用已替换为 verify.ExitReason*)

# orchestration/turn/ 0 残留验证 (DM-20260626-004 T04 baseline 保持)
ls internal/layers/orchestration/turn/ 2>&1
# 预期: No such file or directory (turn/ 已合并到 sessionorchestrator/)
```

### Step 5: S4-Gate (PR + auto-merge, 0.5h)

```bash
git checkout -b feat/devrix-d7-6s-verify-promotion
git add -A
git commit -m "refactor(d7): exit_reason + verdict_to_exit_reason promote sessionorchestrator/ → executionflow/verify/ (DM-20260626-005)"
git push -u origin feat/devrix-d7-6s-verify-promotion
gh pr create --title "refactor(d7): exit_reason promote to executionflow/verify/ (DM-20260626-005)" --body "..."
gh pr merge <PR#> --auto --squash --delete-branch
```

### Step 6: S5 验收报告 (1h)

创建 `openspec/changes/devrix-d7-6s-verify-promotion/acceptance-report.md`, 13 AC × 9 sections 验证矩阵:
1. §1 摘要 (3 文件 promote, 0 函数签名变化)
2. §2 T 层验证 (4 P0 T 全 IMPLEMENTED)
3. §3 22 包 baseline 回归 (22/22 PASS)
4. §4 LP-1/2/5 集成测试兼容
5. §5 hardening/escape/autoclose 0 变化 baseline 保持
6. §6 cross-package import DAG 验证 (单向: sessionorchestrator → verify)
7. §7 sessionorchestrator/ 包内 ExitReason 残留 0 验证
8. §8 spec 同步 (4 文档 version bump)
9. §9 PR + auto-merge 落地证据

### Step 7: S6 归档 (1h)

```bash
mkdir -p openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/specs/d7-orchestration/
git mv openspec/changes/devrix-d7-6s-verify-promotion/{demand,proposal,design,tasks,acceptance-report}.md \
       openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/
git mv openspec/changes/devrix-d7-6s-verify-promotion/specs/d7-orchestration/spec.md \
       openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/specs/d7-orchestration/
git mv openspec/changes/devrix-d7-6s-verify-promotion/.openspec.yaml \
       openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/

# .openspec.yaml status: s2_proposal → s7_archived, 4 P0 T PLANNED → IMPLEMENTED
# demand-archive-index.md 新增 DM-20260626-005 行
# rm -rf openspec/changes/devrix-d7-6s-verify-promotion/

bash scripts/verify-archive.sh devrix-d7-6s-verify-promotion  # 12/12 PASS 预期
```

## 4. 跨包引用矩阵

Promote 前后对比:

| 文件 | promote 前 package | promote 后 package | 含 ExitReason 引用? |
|------|--------------------|--------------------|--------------------|
| `executionflow/verify/exit_reason.go` | sessionorchestrator | **verify** | ✅ 定义 |
| `executionflow/verify/verdict_to_exit_reason.go` | sessionorchestrator | **verify** | ✅ 定义 + 引用 |
| `executionflow/verify/verdict_to_exit_reason_test.go` | sessionorchestrator | **verify** | ✅ 引用 |
| `sessionorchestrator/turn_orchestrator.go` | sessionorchestrator | sessionorchestrator | ✅ 11 处 (改 verify.ExitReason*) |
| `sessionorchestrator/turn_orchestrator_test.go` | sessionorchestrator | sessionorchestrator | ✅ 2 处 (改 verify.ExitReason*) |
| `executionflow/verify/anomaly.go` | (已存在) verify | verify | ❌ (0 引用) |

跨包 DAG 边:
- `sessionorchestrator/turn_orchestrator.go` → `executionflow/verify` (新增, 11 处 ExitReason 引用)
- `executionflow/verify/exit_reason.go` → (无 cross-package)
- `executionflow/verify/verdict_to_exit_reason.go` → `orchtypes` + `workmodel` (无 sessionorchestrator, 无 cycle)

## 5. 风险缓解

| 风险 | 等级 | 缓解 |
|------|------|------|
| 跨包 import cycle | Low | `executionflow/verify/` 已存在且 0 sessionorchestrator dep; promote 后单向 `sessionorchestrator → verify` |
| 11 处 ExitReason* 替换的 sed 边界 (避免误替换 exitReason 变量名) | Medium | sed `\bExitReason\b` 单词边界 + `\bExitReason([A-Z])\b` 大写后缀限定, exitReason 变量名小写不命中 |
| LP-1/LP-2/LP-5 路径行为变化 | Low | 0 函数签名变化, 14 ExitReason 字符串值不变 |
| `executionflow/verify/` 包行数膨胀 (现有 2 文件 ~200 行 + promote 3 文件 218 行 = ~418 行) | Low | 未超 layering.md 800 行上限; 仍在合理范围 |
| sessionorchestrator/autoclose.go + hardening/ + escape/circuit_breaker.go 误改 | Low | 本次 scope 仅 5 个 .go 文件 (3 promote + 2 turn_orchestrator*), git diff 限定到具体文件可 100% 保证 baseline stability |

## 6. 关键文件清单

### 修改文件 (5 个 .go)

- `internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go` (11 处 ExitReason* → verify.ExitReason* + 加 import)
- `internal/layers/orchestration/sessionorchestrator/turn_orchestrator_test.go` (2 处 ExitReasonNatural → verify.ExitReasonNatural)

### 迁移文件 (3 个 .go git mv)

- `internal/layers/orchestration/sessionorchestrator/exit_reason.go` → `internal/layers/orchestration/executionflow/verify/exit_reason.go`
- `internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason.go` → `internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason.go`
- `internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason_test.go` → `internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason_test.go`

### 文档同步 (4 个 .md)

- `openspec/specs/d7-orchestration/d7-domain.md` (v2.2.0 → v2.3.0)
- `openspec/specs/d7-orchestration/design.md` (v4.2.0 → v4.3.0)
- `openspec/specs/d7-orchestration/t-registry.md` (v4.4.0 → v4.5.0)
- `openspec/t-registry.md` (root) (v5.4.0 → v5.5.0)

### 新增文件 (S6 archive)

- `openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/demand.md`
- `openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/proposal.md`
- `openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/design.md`
- `openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/tasks.md`
- `openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/acceptance-report.md`
- `openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/.openspec.yaml`
- `openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/specs/d7-orchestration/spec.md`

## 7. 验证矩阵

| 维度 | 验证方式 | 预期 |
|------|----------|------|
| 编译 | `go build ./...` | 0 错误 |
| Lint | `go vet ./...` | 0 警告 |
| 单元测试 | `go test -race -count=1 ./internal/layers/orchestration/...` | 22/22 PASS, 0 race |
| 集成测试 | `go test -race -count=1 ./tests/integration/d7/...` | baseline 持平, LP-1/2/5 100% 兼容 |
| Baseline stability | `git diff` autoclose.go + hardening/ + escape/circuit_breaker.go | 空 diff |
| 残留验证 | `grep -rn "ExitReason[^a-zA-Z]" sessionorchestrator/*.go` | 0 命中 |
| 跨包 DAG | `go list -deps ./internal/layers/orchestration/executionflow/verify` | 不含 sessionorchestrator |
| Spec sync | grep d7-domain v2.3.0 + design v4.3.0 + t-registry v4.5.0 + 根 v5.5.0 | 全部命中 |
| Archive | `verify-archive.sh devrix-d7-6s-verify-promotion` | 12/12 PASS |