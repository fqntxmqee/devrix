# 测试规范

**版本:** 1.0.0
**状态:** Active
**所属阶段:** S4、S5
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
- [ ] `./scripts/test-all.sh` 通过
- [ ] 覆盖率 >= 80%
- [ ] `acceptance-report.md` 已生成，状态 PASS
- [ ] `t-registry.md` 对应条目更新为 IMPLEMENTED（根索引 + 域注册表）
- [ ] P0 T 层测试 100% PASS（阻断交付）
