# 测试规范

**版本:** 1.1.0
**状态:** Active
**所属阶段:** S4、S5
**最后更新:** 2026-06-26
**引用规范:** `openspec/specs/testing-framework/spec.md`、`openspec/specs/testing-quality/spec.md`

---

## 1. 规范层级

本规范是项目级测试入口。详细规则定义在以下权威文档中：

| 规范 | 路径 | 内容 |
|------|------|------|
| 测试框架规范 | `openspec/specs/testing-framework/spec.md` | 测试金字塔、目录结构、Mock 规则、T 层追溯 |
| **域分段测试** | `openspec/specs/testing-framework/domain-segmentation.md` | D2/D3/D4 build tag、分段脚本、Stage 矩阵 |
| 测试质量规范 | `openspec/specs/testing-quality/spec.md` | 边界条件、Mock 滥用治理、断言深度 |

**规则：** `testing-framework/spec.md` 是测试规范的权威来源（SoT）。本文件不重复其内容。

---

## 2. 测试金字塔速查

| 层级 | 位置 | Build Tag | 运行命令 | 超时 | 阻断 |
|------|------|-----------|---------|------|------|
| 单元 | `internal/**/*_test.go` | 无 | `./scripts/test-unit.sh` | < 2min | PR 门禁 |
| 集成 | `tests/integration/` | `integration` + 域 tag | `./scripts/test-integration.sh` | < 10min | 合入前 |
| E2E | `tests/e2e/` | `smoke` + 域 tag | `./scripts/test-e2e.sh` | < 5min | 合入前 |
| 验收 | `tests/acceptance/p0/` | `acceptance` + 域 tag | `./scripts/test-acceptance.sh` | < 5min | S5 |
| 性能 | `tests/performance/` | `performance` + 域 tag | — | — | 可选 |

### 2.1 域分段（D2 / D3 / D4）

按架构域独立跑测试（方案 B：build tag 第二轴）：

| 命令 | 范围 |
|------|------|
| `./scripts/test-domain.sh d2` | Context Engine 单元 + 集成 + 验收 + 性能 |
| `./scripts/test-domain.sh d3` | LLM Gateway 单元 + 集成 |
| `./scripts/test-domain.sh d3 --live` | 追加真实 LLM API 集成 |
| `./scripts/test-domain.sh d4` | Multi-Agent 单元 + 集成 + 验收 + E2E |

详见 `openspec/specs/testing-framework/domain-segmentation.md`。

**变更影响：** 修改 `contextengine/` → 至少 `./scripts/test-domain.sh d2`；`llmgateway/` → `d3`；`multiagent/` → `d4`。

---

## 3. T 层追溯要求

### 3.1 注册

S2 阶段在 T 层注册表预登记（状态：PLANNED）。根索引 `openspec/t-registry.md`，各域明细 `openspec/specs/d{N}-*/t-registry.md`。编号使用 DSAFT T 层标准格式。

### 3.2 代码标注

```go
// T: D4-S1-A01-T01
// Domain: D4
// Stage: s0_unit
func TestAgent_Run_NormalFlow(t *testing.T) { ... }
```

域 tag 与 Stage 约定见 `openspec/specs/testing-framework/domain-segmentation.md` §4。

### 3.3 验收标准

| 优先级 | S5 要求 |
|--------|--------|
| P0 | 100% PASS（阻断交付） |
| P1 | 必须执行，失败记例外 |
| P2 | 尽力执行 |

### 3.4 结论用词（避免与归档门禁混淆）

| 层级 | 字段 | 合法值 | 说明 |
|------|------|--------|------|
| **单测 / AC / T 行** | 状态列 | `PASS` · `FAIL` · `SKIP` · `DESIGN` | 具体检查项或测试用例结果 |
| **验收报告终态** | frontmatter `verdict` | `ACCEPTED` · `PARTIAL` · `REJECTED` | Change 级 S5 结论 |

规则：
- S5 完成且可进入 S6-交付：`verdict: ACCEPTED`，且所有 P0 T / AC 为 **PASS**（或已在 `proposal.md` Out of Scope 中声明 defer 并标 `DESIGN`）。
- 热修/分阶段交付：`verdict: PARTIAL` 须写明 defer 项与 follow-up Change；S6-归档前须在报告中说明 PARTIAL 依据。
- `REJECTED`：回到 S4 修复，不得归档。

`archiving.md` 归档门禁检查的是报告 **verdict**（`ACCEPTED` 或文档化的 `PARTIAL`），不是单行测试的 `PASS`。

---

## 4. 覆盖率

| 阶段 | 阈值 | 说明 |
|------|------|------|
| 当前 CI 门禁 | 20% | 最低基线，不通过则阻断 |
| S5 验收 | >= 80% | 目标值 |
| 新模块 | 80%+ | 新 package 禁止无测试合并 |

覆盖率提升计划：
1. 短期：CI 阈值提升至 50%
2. 中期：CI 阈值提升至 80%

---

## 5. 并发测试

涉及共享状态的代码必须通过 race 检测：

```bash
go test -race ./internal/layers/multiagent/...
```

---

## 6. 项目特有测试模式

### 6.1 Table-Driven Tests

```go
func TestAgent_StateTransition(t *testing.T) {
    tests := []struct {
        name    string
        from    State
        to      State
        wantErr bool
    }{
        {"created to running", StateCreated, StateRunning, false},
        {"terminated to running", StateTerminated, StateRunning, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

### 6.2 测试辅助函数

跨包共享的 mock/fixture 放在 `tests/testutil/`。

---

## 7. 检查清单

S4 提测前：
- [ ] 所有 P0 T 层测试已编写（编号格式 `D{X}-S{X}-A{XX}-T{XX}`）
- [ ] 测试代码标注了 T 层编号
- [ ] `./scripts/test-unit.sh` 通过
- [ ] 受影响域 `./scripts/test-domain.sh d{N}` 通过（D2/D3/D4 变更时）
- [ ] 新代码有对应的 `_test.go`
- [ ] 并发代码通过 `-race` 检测
- [ ] 测试文件无 `t.Skip`（除非有注释说明原因）

S5 验收前：
- [ ] `./scripts/test-all.sh` 通过（**全量**；PR CI 仅跑 `unit tests` smoke，不等同 S5）
- [ ] 覆盖率 >= 80%
- [ ] `acceptance-report.md` 已生成，frontmatter **`verdict: ACCEPTED`**（或文档化 `PARTIAL`）
- [ ] `t-registry.md` 对应条目更新为 IMPLEMENTED（根索引 + 域注册表）
- [ ] P0 T 层测试 100% PASS（阻断交付）

---

## 8. acceptance-report.md 模板

S5 产出，路径：`openspec/changes/<change-id>/acceptance-report.md`。

可手工编写，或用 `./scripts/gen-acceptance-report.sh --change <change-id>` 生成骨架后补全。

```markdown
---
demand-id: DM-YYYYMMDD-NNN
change-id: devrix-{module-name}
title: <Change 标题> — 验收报告
executor: <执行人 / Agent>
environment: local | staging | production
date: YYYY-MM-DD
verdict: ACCEPTED | PARTIAL | REJECTED
---

# 验收报告：<标题>

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| Demand ID | DM-YYYYMMDD-NNN |
| Change ID | devrix-{module-name} |
| PR | #NNN |
| 总体结论 | **ACCEPTED**（或 PARTIAL / REJECTED） |

### 验证命令与结果

| Check | Command | Result |
|-------|---------|--------|
| 单元测试 | `./scripts/test-unit.sh` | PASS / FAIL |
| 全量测试 | `./scripts/test-all.sh` | PASS / FAIL |
| 覆盖率 | `go test ./internal/... -coverprofile=...` | XX% |
| 域分段（如适用） | `./scripts/test-domain.sh d{N}` | PASS / FAIL |

## 2. T 层验收矩阵

| T ID | 描述 | 优先级 | 状态 | 证据 |
|------|------|--------|------|------|
| D{X}-S{X}-A{XX}-T{XX} | ... | P0 | PASS | 测试文件:行 / PR commit |

**P0 T 通过率:** N/N = 100%

## 3. AC 验收对照

| AC | 描述 | 优先级 | 状态 | 证据 |
|----|------|--------|------|------|
| AC1 | ... | P0 | PASS | ... |

## 4. 领域文档同步

| 路径 | 是否更新 | 说明 |
|------|----------|------|
| `openspec/specs/d{N}-*/spec.md` | 是/否 | ... |
| `openspec/specs/d{N}-*/t-registry.md` | 是/否 | PLANNED → IMPLEMENTED |

## 5. 边界与遗留（PARTIAL 时必填）

- defer 项、follow-up Change ID、风险说明
```

**verdict 判定：**
- **ACCEPTED** — P0 T 100% PASS，P0 AC 满足，`test-all.sh` 绿，覆盖率 ≥ 80%
- **PARTIAL** — 范围已在 proposal Out of Scope 声明的 defer；须列 follow-up
- **REJECTED** — 未达 P0 门禁，需回 S4
