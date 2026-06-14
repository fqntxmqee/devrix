---
demand-id: DM-20260614-019
title: D3 LLM Gateway v2.0 — 价值流物理路径迁移 + contracts.go 拆分
source: 父 change DM-20260614-016 v1.0 S5_Pass（v1.0 + v1.1 已闭环）Phase F 启动；按 v1.0 demand.md §6.3 v2.0 Structure 范围
priority: P1
status: S1_Open
dsaft_domain: D3
created: 2026-06-14
last-updated: 2026-06-14
review-round: R0（待 Owner）
parent: DM-20260614-016
playbook: dsaft-refactoring-playbook
---

# D3 LLM Gateway v2.0 — 价值流物理路径迁移 + contracts.go 拆分

> **本子 change 性质**：v2.0 物理结构迁移；接续 v1.0 + v1.1（均已 ACCEPTED）。v1.0 注册表已价值流化（5+1 S），v1.1 韧性可见性 + D6 probe 已落地；本期把"注册表是价值流、代码目录仍是技术模块"的中间态彻底消除。
>
> **范围（v1.0 demand.md §6.3）**：
> - 物理路径迁移到 scenario-slug 目录（`adapter/` `gateway/` `breaker/` `retry/` `token/` `safety/` `config/` → 价值流 slug）
> - `contracts.go` 按价值流拆分到各子包；根保留 re-export 一个发布周期
> - `bridges/llm/` 路径不变（跨域锚点，v1.0 R1 D2 决议）
> - `layering.md §Domain Layout` 更新 D3 章节

---

## 0. 父需求 v1.0/v1.1 落地状态

| 父 Change | DM ID | 状态 | 触发 |
|----------|-------|------|------|
| `devrix-d3-sa-refine` | DM-20260614-016 | **S5_Pass** | 父容器（v1.0 registry-only ACCEPTED 15/15 AC） |
| `devrix-d3-sa-refine-v1.1` | DM-20260614-017 | **S5_ACCEPTED + S6 归档** | 韧性可见性 + D6 3 probe + IAdapter.Protocol() + obs fail-fast（commit 3a6970b） |

**v1.0 + v1.1 已确立的不可变性承诺**（继承自 v1.0 acceptance-report.md §0）：

- **运行时 span 名**：`llm.stream` / `llm.provider.route` / `llm.circuit_breaker` / `llm.retry` / `llm.adapter.stream` —— 字面量未改
- **核心 metric 名**：`llm_requests_total` / `llm_errors_total` / `llm_latency_seconds` / `llm_breaker_state` / `llm_breaker_transitions_total`（v1.1 新增） —— 字面量未改
- **YAML 配置 key**：`llm_gateway:` / `circuit_breaker:` / `model_tiers:` / `features:`（v1.1 新增）—— 字面量未改
- **Bridge 路径**：`internal/bridges/llm/` —— 跨域锚点不变
- **D3 5+1 S 切法**：RouteModel / StreamChat / ProtectCall / BudgetTokens / GuardContent / ConfigureGateway + D3-X CROSS（Bridge + Bootstrap）—— 已稳定

---

## 1. v2.0 范围

### 1.1 In Scope（v2.0 必做）

| # | 项 | 责任域 | 输出 |
|---|----|--------|------|
| F2 | 物理迁移 `adapter/` → `stream/`（含 re-export 桥接 1 发布周期） | D3 | `internal/layers/llmgateway/stream/adapter/` |
| F3 | 物理迁移 `gateway/router.go` → `route/` | D3 | `internal/layers/llmgateway/route/router.go` |
| F4 | 物理迁移 `gateway/gateway.go` Stream 主实现 → `stream/` | D3 | `internal/layers/llmgateway/stream/gateway.go` |
| F5 | 物理迁移 `breaker/` + `retry/` → `protect/`（两机制独立 .go） | D3 | `internal/layers/llmgateway/protect/circuit_breaker.go` + `protect/retry.go` |
| F6 | 物理迁移 `token/` → `budget/` | D3 | `internal/layers/llmgateway/budget/` |
| F7 | 物理迁移 `safety/` → `guard/` | D3 | `internal/layers/llmgateway/guard/` |
| F8 | 物理迁移 `config/` + `shared/config/llmgateway.go` → `configure/`（**注意 llmgateway_features_test.go 跨包**） | D3 | `internal/layers/llmgateway/configure/`（含 shared/config 合并） |
| F9 | `contracts.go` 按价值流拆到各子包；根保留 re-export 1 发布周期 | D3 | 子包内 contracts/<value>.go + 根 `contracts.go` re-export |
| F11 | `layering.md §Domain Layout` 更新 D3 章节 | arch | layering.md v3.8.0 |
| G1–G8 | Phase G 验证 + 归档 | D3 + arch | build/test/vet 全绿 + archive + acceptance-report v2.0 |

### 1.2 Out of Scope

- v1.1 已落地的 9 F（F1–F9）行为变更：v2.0 仅迁移路径，不动行为
- `bridges/llm/` 路径：跨域锚点（R1 D2 决议保持）
- runtime span / metric / config key 字面量：v1.0 不变性承诺
- Provider 适配器重写（DeepSeekAdapter / MiniMaxAdapter 行为不变）
- v2.0 后保留的旧路径 re-export：1 发布周期，到 G4 dead code 清理
- T08 熔断持久化（仍 PLANNED，留 v2.0+）
- D6 probe #3 Token 预算触发率（v1.1 推迟到 v1.2；v2.0 物理迁移不动 probe）
- V3 计划（Anthropic/OpenAI Adapter / Rate Limiter / 负载均衡）

---

## 2. 路径映射表（v1.1 现状 → v2.0 目标）

| # | v1.1 路径 | v2.0 路径 | re-export 桥接 | 备注 |
|---|----------|----------|----------------|------|
| 1 | `internal/layers/llmgateway/adapter/` | `internal/layers/llmgateway/stream/adapter/` | `internal/layers/llmgateway/adapter/` → 转发 import | DeepSeek + MiniMax + protocol.go + 3 test |
| 2 | `internal/layers/llmgateway/gateway/router.go` | `internal/layers/llmgateway/route/router.go` | `internal/layers/llmgateway/gateway/router.go` → 转发 | F03 ResolveModelRoute |
| 3 | `internal/layers/llmgateway/gateway/gateway.go` | `internal/layers/llmgateway/stream/gateway.go` | re-export | F01 StreamChat 主实现 |
| 4 | `internal/layers/llmgateway/gateway/breaker_observer.go` | `internal/layers/llmgateway/protect/breaker_observer.go` | re-export | F07/F08 |
| 5 | `internal/layers/llmgateway/breaker/` | `internal/layers/llmgateway/protect/` | re-export | circuit_breaker + state + observer |
| 6 | `internal/layers/llmgateway/retry/` | `internal/layers/llmgateway/protect/retry.go`（合并 .go） | re-export | F04 ComputeBackoff |
| 7 | `internal/layers/llmgateway/token/` | `internal/layers/llmgateway/budget/` | re-export | F01 CountText + F03 CheckBudget |
| 8 | `internal/layers/llmgateway/safety/` | `internal/layers/llmgateway/guard/` | re-export | F01 Check + F04 LatencySink |
| 9 | `internal/layers/llmgateway/config/` | `internal/layers/llmgateway/configure/` | re-export | F01 LoadConfig + F03 Validate |
| 10 | `internal/shared/config/llmgateway.go` | `internal/layers/llmgateway/configure/shared_config.go` | re-export | F05 FeatureFlagDefaults |
| 11 | `internal/layers/llmgateway/contracts.go` | 拆到各子包 | 根 re-export | 见 §3 |

### 2.1 re-export 桥接策略

每个迁移包保留旧路径 1 个发布周期：

```go
// 例：internal/layers/llmgateway/adapter/iadapter.go（v2.0 桥接）
package adapter

// Deprecated: 将在 G4 dead code 清理时删除；请迁移至 stream/adapter。
import "github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"

type Adapter = stream_adapter.Adapter
// ... 透传所有公共类型与函数
```

**目标**：
- 所有 import 旧路径的代码（G2 测试全绿）→ 不立即破坏
- `golangci-lint` 启用 `depguard` 规则后，新代码禁止 import 旧路径
- 1 发布周期后 G4 物理删除旧路径

---

## 3. contracts.go 拆分（v1.0 demand.md §6.3 v2.0 Structure）

### 3.1 拆分原则（R2 §4.3 衍生）

| 原则 | 落地 |
|------|------|
| 类型归属 = 价值流 | TokenUsage → budget；CircuitState/BreakerConfig → protect；Adapter/Request/Chunk/Stream → stream；等等 |
| 接口隔离 | IAdapter、ILLMGateway、ITierResolver、IFeatureFlags 各自归属其价值流 |
| 跨价值流类型在根 | 仅保留 ErrorSentinel（如 `ErrObservabilityRequired`）与跨域常量 |
| 根 re-export | 1 发布周期，1 个 `//go:build !devrix_v2_consume` tag 控制 |

### 3.2 拆分方案（初稿）

| 子包 | 类型（v2.0 后归属） | 旧位置 |
|------|--------------------|--------|
| `stream/adapter/` | `Adapter`、`IAdapter`、`Request`、`Chunk`、`Stream`、`Protocol` | `contracts.go` L1-L120 |
| `route/` | `Tier`、`TierAlias`、`RoutingTable` | `contracts.go` L121-L180 |
| `protect/` | `CircuitState`、`BreakerConfig`、`BreakerObserver`、`RetryPolicy` | `contracts.go` L181-L260 |
| `budget/` | `TokenUsage`、`BudgetCheckResult` | `contracts.go` L261-L310 |
| `guard/` | `SafetyCheckResult`、`SafetyLevel` | `contracts.go` L311-L360 |
| `configure/` | `LLMConfig`、`LLMFeatureFlags`、`ProviderConfig` | `contracts.go` L361-L450 + `shared/config/llmgateway.go` |
| `llmgateway/` (根) | `ILLMGateway`、`ITierResolver`、`EngineEvent`（v1.1 复用契约）、SentinelError | `contracts.go` L451-末 |

### 3.3 风险

- `import "github.com/devrix/devrix/internal/layers/llmgateway"` 旧 import 在 1 发布周期内继续工作
- 测试文件 import 路径需同步更新（G5 11 P0 + 26 T 回归保证）

---

## 4. v2.0 验收标准（AC 草案）

| AC | 标准 |
|----|------|
| AC-01 | 8 个新价值流子目录（stream/ stream/adapter/ route/ protect/ budget/ guard/ configure/）存在；旧技术角色词目录（adapter/ gateway/ breaker/ retry/ token/ safety/ config/）存在但仅含 re-export |
| AC-02 | `go build ./...` 全绿（包含旧路径 re-export 兼容） |
| AC-03 | `go test -race ./internal/layers/llmgateway/... ./internal/bridges/llm/...` 全绿（11 P0 T + 26 T 总） |
| AC-04 | `go vet ./...` 无新增警告（v1.0 11 P0 T 全部 IMPLEMENTED 状态保持） |
| AC-05 | `scripts/check_t_aliases.py` 退出码 0（26 条 alias 100% 覆盖；v1.1 9 新 T 仍 IMPLEMENTED） |
| AC-06 | `grep -r "D3-S[1-7]" openspec/specs/` 无新增失同步 |
| AC-07 | runtime span / metric / YAML config key 字面量未改（v1.0 不变性承诺） |
| AC-08 | `internal/bridges/llm/` 路径不变（跨域锚点） |
| AC-09 | `contracts.go` 拆分后，根文件 < 200 行（仅 ILLMGateway/ITierResolver/EngineEvent/SentinelError + re-export） |
| AC-10 | v1.0 灰区声明（D3-S5 vs D2-S18）仍有效（cross-domain-boundaries.md §2.1.3 无需变更） |
| AC-11 | 7 个新场景 slug 全部注册到 `code-layout.md §4.4` D3 注册表 |
| AC-12 | `layering.md §Domain Layout` D3 章节更新为 v3.8.0（物理路径映射） |
| AC-13 | `demand-archive-index.md` 标记 `devrix-d3-sa-refine-v2.0` ACCEPTED |
| AC-14 | v2.0 子 change 归档到 `openspec/archive/2026-MM-DD-devrix-d3-sa-refine-v2.0/` |
| AC-15 | 父 change `devrix-d3-sa-refine` 状态从 S5_Pass 推进到 S7_Archived（v1.0 + v1.1 + v2.0 全部完成） |

---

## 5. 实施阶段

| Phase | 内容 | 依赖 | 交付物 |
|-------|------|------|--------|
| S1 → S2 | 写本 demand.md + proposal.md | v1.0 S5_Pass | OpenSpec S1 → S2 推进 |
| S3 | 写 design.md v3.2.0（路径映射 + 拆分方案 + 风险） | S2 | design.md |
| S3-Gate | review-r1/r2/r3 | S3 | S3-Gate Cleared |
| S4 | Phase F 实施（F2–F9 + F11） | S3-Gate | 8 个子目录 + re-export + contracts.go 拆分 + layering.md |
| S5 | Phase G 验证（G1–G8） | S4 | acceptance-report v2.0 |
| S6 | 归档 v2.0 子 change + 父 change 推进到 S7 | S5 | archive + demand-archive-index 更新 |

---

## 6. 风险与缓解

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| 物理迁移破坏 v2.1 IMPLEMENTED 状态 | 中 | 高 | re-export 桥接 1 发布周期 + 完整 P0 回归（G5） |
| contracts.go 拆分影响所有 consumer | 中 | 高 | re-export + `depguard` 拦截新代码 + G5 11 P0 T 全绿 |
| 26 条 T ID 跨物理目录的测试文件 import 不同步 | 中 | 中 | 一次完成 F2–F8 + alias 校验脚本 + 同步更新所有 test 文件 |
| 旧路径 dead code 未清（G4 推迟） | 低 | 低 | 1 发布周期后 G4 强制清理；过渡期 `// Deprecated:` 注释 |
| F5 breaker/retry 合并冲突 v1.1 韧性可见性 | 低 | 中 | F5 仅物理合并（独立 .go），circuit_breaker/observer 保持，F 编排下 2 个 F 各自 IMPLEMENTED |
| v1.0 + v1.1 已归档后，父 change S7 推进时序 | 低 | 低 | S6 v2.0 归档时同步 S7 父 change 状态（S7 归档协议） |

---

## 7. 关联与依赖

| 关联 | 影响 |
|------|------|
| DM-20260614-016 v1.0 S5_Pass | 父需求已闭环；本子 change 是 v1.0 计划内 F-G 阶段 |
| DM-20260614-017 v1.1 ACCEPTED | 兄弟子 change；本 v2.0 物理迁移**仅动路径，不动 v1.1 行为** |
| dsaft-refactoring-playbook | 首次 D 域 v2.0 物理迁移样板；输出供 D2/D4/D5 v2.0 参考 |
| v1.0 acceptance-report §11 风险表 | 物理迁移高风险已识别（v2.0 继承缓解策略） |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：8 F 路径迁移 + contracts.go 拆分 + 15 AC + 6 风险；S1_Open |
