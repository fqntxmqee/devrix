# Domain Segmentation Testing (D2 / D3 / D4)

**Capability:** testing-framework / domain-segmentation
**Status:** Active
**Version:** 1.0.0
**Parent:** `openspec/specs/testing-framework/spec.md`

---

## 1. 目的

在现有测试金字塔（按**层级**切分）之上，增加按 **D 域**切分的第二维度，使 D2 Context Engine、D3 LLM Gateway、D4 Multi-Agent 可独立分段运行，而不移动单元测试位置。

---

## 2. 设计原则

| 原则 | 说明 |
|------|------|
| 单元测试不加域 tag | 仍位于 `internal/layers/{domain}/`，通过 package 路径筛选 |
| 集成/验收/性能/E2E 加域 tag | 与层级 tag **组合**使用（方案 B） |
| 全量脚本显式传齐域 tag | 避免 `-tags=integration` 单独使用时域测试被编译排除 |
| `live` 与 PR 门禁隔离 | 需 API Key 的测试单独 `live` tag，默认 CI 不传 |

---

## 3. Build Tag 注册表

### 3.1 层级 Tag（已有）

| Tag | 目录 |
|-----|------|
| `integration` | `tests/integration/` |
| `acceptance` | `tests/acceptance/` |
| `smoke` | `tests/e2e/` |
| `performance` | `tests/performance/` |

### 3.2 域 Tag（新增）

| Tag | 域 | 单元测试 package |
|-----|-----|------------------|
| `d1` | Communication | `./internal/layers/communication/...` |
| `d2` | Context Engine | `./internal/layers/contextengine/...` |
| `d3` | LLM Gateway | `./internal/layers/llmgateway/...` |
| `d4` | Multi-Agent | `./internal/layers/multiagent/...` |
| `d5` | Observability | `./internal/layers/observability/...` |
| `cross` | 跨域集成 | `tests/integration/*_gateway_*`, `context_llm_*`, `agent_integration` 等 |
| `live` | 需外部凭证 | 与 `d3` 组合用于真实 LLM API |

**规则：** 除 `live` 外，域 tag MUST 与层级 tag 组合书写，不得单独使用。

---

## 4. Build Constraint 写法

### 4.1 单域集成/验收测试

```go
//go:build integration && d2

// Covers: D2-S1-T05
// Domain: D2
// Stage: s2
func TestIntegration_VerifyCommands(t *testing.T) { ... }
```

```go
//go:build acceptance && d2

// Covers: D2-S2-T01
// Domain: D2
// Stage: s2
func TestT_CTX_Compression_Trigger(t *testing.T) { ... }
```

### 4.2 跨域测试

```go
//go:build integration && cross

// Covers: D2-S0-T02
// Domain: cross (D2+D3)
// Stage: s2
func TestIntegration_ContextLLMGateway(t *testing.T) { ... }
```

### 4.3 真实 API（不阻断 PR）

```go
//go:build integration && d3 && live

// Covers: D3-S2-A01-T03 (SSE parse error handling)
// Domain: D3
// Stage: s3_live
// Legacy alias: D3-S1-A01-T03 (旧 7 S 切法)
func TestIntegration_LLMRealAPI(t *testing.T) { ... }
```

### 4.4 注释约定

域测试 SHOULD 在 `Covers:` 下方追加：

```go
// Domain: D2 | D3 | D4 | cross
// Stage: s0_unit | s1_contract | s2_integration | s3_live | s3_e2e
```

---

## 5. 运行矩阵

### 5.1 域分段脚本

| 命令 | 说明 |
|------|------|
| `./scripts/test-domain.sh d2` | D2 单元 + D2/cross 集成 + D2 验收 + D2 性能 |
| `./scripts/test-domain.sh d3` | D3 单元 + D3 集成（不含 live） |
| `./scripts/test-domain.sh d4` | D4 单元 + D4/cross 集成 + D4 验收 + D4 E2E |
| `./scripts/test-domain.sh d2 --cover` | 同上，并输出该域 unit/integration cover 汇总 |
| `./scripts/test-domain.sh d3 --live` | 追加 D3 live 集成 |
| `./scripts/coverage-domains.sh` | 生成 D1–D5 行/块覆盖 Markdown（CI artifact） |

### 5.2 Tag 传参规则

**全量集成（默认 CI / test-integration.sh）：**

```bash
go test -tags="integration,d1,d2,d3,d4,d5,cross" ./tests/integration/...
```

**域筛选（示例 D2）：**

```bash
go test -tags="integration,d2,cross" ./tests/integration/...
```

**全量验收：**

```bash
go test -tags="acceptance,d1,d2,d4" ./tests/acceptance/p0/...
```

**D2 验收：**

```bash
go test -tags="acceptance,d2" ./tests/acceptance/p0/...
```

### 5.3 Stage 与层级/域映射

| Stage | D2 | D3 | D4 |
|-------|----|----|-----|
| s0_unit | `go test ./internal/layers/contextengine/...` | `.../llmgateway/...` | `.../multiagent/...` |
| s1_contract | 单元 + httptest mock | adapter 单测 | factory/agent 单测 |
| s2_integration | `-tags=integration,d2,cross` | `-tags=integration,d3,cross` | `-tags=integration,d4,cross` |
| s3_live/e2e | `-tags=integration,cross,live`（可选） | `--live` | `-tags=smoke,d4` |

---

## 6. 跨域归属表

| 测试文件 | Domain Tag | 说明 |
|----------|------------|------|
| `context_gateway_flow_test.go` | `cross` | D1 Gateway + D2 Engine |
| `context_llm_gateway_test.go` | `cross` | D2 + D3 |
| `context_compression_obs_test.go` | `d2` | D2 主路径，附带 D5 断言 |
| `agent_integration_test.go` | `cross` | D4 + D2 Gateway |
| `llm_observer_test.go` | `d3` | D3 主路径，附带可观测断言 |

---

## 7. 变更影响分析

修改某域代码时，MUST 至少运行对应域脚本：

| 变更路径 | 最低验证 |
|----------|----------|
| `internal/layers/contextengine/**` | `./scripts/test-domain.sh d2` |
| `internal/layers/llmgateway/**` | `./scripts/test-domain.sh d3` |
| `internal/layers/multiagent/**` | `./scripts/test-domain.sh d4` |
| `tests/integration/context_*` | `./scripts/test-domain.sh d2` |
| `internal/bridges/llm/**` | `./scripts/test-domain.sh d2` 与 `d3` |

---

## 8. 与 T 层注册表

`openspec/t-registry.md` 中 D2/D3/D4 条目 SHOULD 在 Test 位置旁维护 Stage；新增 T 层时 MUST 指定 Domain tag 并更新本节跨域表（若适用）。
