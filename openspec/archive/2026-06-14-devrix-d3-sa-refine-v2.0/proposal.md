# Proposal: D3 LLM Gateway v2.0 — 价值流物理路径迁移

**Change ID:** devrix-d3-sa-refine-v2.0
**Demand ID:** DM-20260614-019
**Status:** S2_Clarified
**Parent:** DM-20260614-016 (v1.0 S5_Pass)

> **本 proposal 范围**：D + S 切法（不含 A/F/T 编排；F/T 编排在 design.md v3.2.0）。
> v1.0 R1 三项 Decision（D1 价值流 S / D2 Bridge 跨域归位 / D3 Safety 归属）已闭合，v1.0 + v1.1 已落地。本期仅作**物理路径归位**。

---

## 1. 5+1 S 价值流切法（继承 v1.0 D1 决议）

| S 段 | 价值流承诺 | v1.0 物理目录（技术词） | **v2.0 物理目录（价值流 slug）** | 路径增量 |
|------|----------|--------------------|--------------------------|---------|
| D3-S1 RouteModel | 给我 model 名/tier，我返回正确 provider+model | `gateway/router.go` | **`route/router.go`** | `route/` |
| D3-S2 StreamChat | 给我 chat 请求，我流式返回 chunk | `gateway/gateway.go` + `adapter/` | **`stream/gateway.go` + `stream/adapter/`** | `stream/` |
| D3-S3 ProtectCall | Provider 故障不阻塞我 | `breaker/` + `retry/` | **`protect/`** | `protect/` |
| D3-S4 BudgetTokens | 上下文不超 token 预算 | `token/` | **`budget/`** | `budget/` |
| D3-S5 GuardContent | 恶意内容不能穿过 gateway | `safety/` | **`guard/`** | `guard/` |
| D3-S6 ConfigureGateway | 配置加载与验证 | `config/` + `shared/config/llmgateway.go` | **`configure/`** | `configure/` |
| D3-X CROSS | Bridge 跨域 + Bootstrap | `internal/bridges/llm/` | **`internal/bridges/llm/`（不变）** | 0 |

---

## 2. 物理路径迁移总表（F2–F9 + F11）

| ID | 旧路径 | 新路径 | 桥接策略 | 风险等级 |
|----|--------|--------|---------|---------|
| F2 | `internal/layers/llmgateway/adapter/` | `internal/layers/llmgateway/stream/adapter/` | 旧路径 re-export 1 发布周期 | 高 |
| F3 | `internal/layers/llmgateway/gateway/router.go` | `internal/layers/llmgateway/route/router.go` | 旧路径 re-export 1 发布周期 | 中 |
| F4 | `internal/layers/llmgateway/gateway/gateway.go` | `internal/layers/llmgateway/stream/gateway.go` | 旧路径 re-export 1 发布周期 | 中 |
| F5 | `internal/layers/llmgateway/breaker/` + `retry/` | `internal/layers/llmgateway/protect/` | 旧路径 re-export 1 发布周期；两机制独立 .go | 高 |
| F6 | `internal/layers/llmgateway/token/` | `internal/layers/llmgateway/budget/` | 旧路径 re-export 1 发布周期 | 中 |
| F7 | `internal/layers/llmgateway/safety/` | `internal/layers/llmgateway/guard/` | 旧路径 re-export 1 发布周期 | 中 |
| F8 | `internal/layers/llmgateway/config/` + `internal/shared/config/llmgateway.go` | `internal/layers/llmgateway/configure/` | 旧路径 re-export 1 发布周期（跨包） | 中-高 |
| F9 | `internal/layers/llmgateway/contracts.go` | 拆到各子包 + 根 re-export | 根 < 200 行；1 发布周期 | 高 |
| F10 | `internal/bridges/llm/` | 不变 | 0 | 0 |
| F11 | `openspec/specs/architecture/layering.md §D3` | v3.8.0 | 文档同步 | 低 |

**总计 7 个旧技术角色词目录 → 6 个新价值流 slug 目录**（route / stream / protect / budget / guard / configure + stream/adapter 子目录）

---

## 3. re-export 桥接契约

### 3.1 桥接模板

```go
// 例：internal/layers/llmgateway/adapter/iadapter.go（v2.0 桥接文件）
package adapter

// Deprecated: 将在 v2.1 dead code 清理时删除；请迁移至
//   github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter
import stream_adapter "github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"

// 类型别名（保留原 import 路径有效）
type (
    Adapter       = stream_adapter.Adapter
    IAdapter      = stream_adapter.IAdapter
    Request       = stream_adapter.Request
    Chunk         = stream_adapter.Chunk
    Stream        = stream_adapter.Stream
    Protocol      = stream_adapter.Protocol
    StreamFactory = stream_adapter.StreamFactory
)

// 函数透传
var (
    NewDeepSeek   = stream_adapter.NewDeepSeek
    NewMiniMax    = stream_adapter.NewMiniMax
    NewMock       = stream_adapter.NewMock
    Register      = stream_adapter.Register
    Get           = stream_adapter.Get
)
```

### 3.2 兼容性约束

| 维度 | 行为 |
|------|------|
| 旧路径 import | 1 发布周期内继续工作（G2 测试全绿即合规） |
| 旧路径修改 | G4 dead code 清理前禁止（避免外部依赖未迁移） |
| 新代码 import 旧路径 | `depguard` 规则在 v2.0.1 起启用，警告但允许；v2.1 起硬失败 |
| 桥接文件头注释 | `// Deprecated:` + 替代路径（`go doc` 与 IDE 可识别） |

### 3.3 桥接有效期

- v2.0 release（含）：旧路径 + 新路径并存
- v2.0 + 1 release：旧路径物理删除（G4）
- 桥接文件自删除检测：`go vet` 启用 `unused` 检查；外部代码 0 引用时提前删除

---

## 4. contracts.go 拆分方案（F9）

### 4.1 拆分原则（继承 v1.0 R2 §4.3）

1. **类型归属价值流**：与某价值流 S 强相关的类型迁入该子包
2. **接口隔离**：每个接口归属其主服务的价值流
3. **根最小化**：根 `contracts.go` 仅保留跨域契约与 SentinelError
4. **桥接兼容**：根 re-export 1 发布周期

### 4.2 拆分映射

| 子包 | 迁入类型（v2.0） | 来源 |
|------|---------------|------|
| `stream/adapter/` | `Adapter` `IAdapter` `Request` `Chunk` `Stream` `Protocol` `StreamFactory` | `contracts.go` + 新增 v1.1 `Protocol()` |
| `stream/` | `ChatRequest` `ChatResponse` `SSEEvent` | `contracts.go` |
| `route/` | `Tier` `TierAlias` `RoutingTable` `RoutingError` | `contracts.go` |
| `protect/` | `CircuitState` `BreakerConfig` `BreakerObserver` `RetryPolicy` `BackoffStrategy` | `contracts.go` + v1.1 observer |
| `budget/` | `TokenUsage` `BudgetCheckResult` `Tokenizer` | `contracts.go` |
| `guard/` | `SafetyCheckResult` `SafetyLevel` `SafetyPattern` | `contracts.go` |
| `configure/` | `LLMConfig` `LLMFeatureFlags` `ProviderConfig` `TierConfig` | `contracts.go` + `shared/config/llmgateway.go` |
| `llmgateway/`（根） | `ILLMGateway` `ITierResolver` `IEngineEvent`（v1.1 复用契约） `SentinelError`（`ErrObservabilityRequired` 等） | `contracts.go` 保留部分 |

### 4.3 拆分步骤

| 步骤 | 操作 | 验证 |
|------|------|------|
| C1 | 在各子包创建 `contracts.go`（子包内） | 子包编译通过 |
| C2 | 从根 `contracts.go` 移除已迁类型，添加 `// Deprecated:` 类型别名指向新位置 | 根 < 200 行；`go build` 全绿 |
| C3 | 同步更新所有 `import` 路径（自底向上：子包 → 子包） | `goimports` 通过 |
| C4 | 旧 import 路径兼容（re-export 类型别名） | G2 测试全绿 |
| C5 | v1.1 + v1.0 现有 26 T + 9 v1.1 T 测试 import 同步 | G5 11 P0 T + 26 T 回归 |

---

## 5. 与 v1.0 R1 Decision 的一致性

| v1.0 Decision | v2.0 落地 |
|--------------|----------|
| **D1** 价值流 S 切法（5+1 S） | ✅ v2.0 物理路径与 S 1:1 对齐 |
| **D2** Bridge 跨域归位（`internal/bridges/llm/`） | ✅ F10 保持 |
| **D3** Safety 留 D3 | ✅ F7 物理迁移到 `guard/`（语义化） |

**v1.1 韧性可见性 + D6 探针**：v2.0 **仅动路径**，不动行为；F1/F2/F3 metric + EngineEvent 全部在 F2/F5 迁移后保持。

---

## 6. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 物理迁移破坏 IMPLEMENTED 状态 | re-export 桥接 + G5 11 P0 T + 26 T 回归 |
| contracts.go 拆分影响 consumer | 根 re-export 1 发布周期 + `goimports` 自动修复 |
| 测试 import 路径遗漏 | 一次完成 + `goimports` + `go vet` 校验 |
| F5 breaker/retry 合并破坏 F 编排 | 独立 .go（circuit_breaker.go / retry.go），F 编排下 2 个 F 各自保留 |
| 旧路径 dead code 残留 | 1 发布周期 + G4 强制清理 |

---

## 7. 不在 S2 阶段产物范围

- **F 编排**：见 design.md v3.2.0 §3
- **T 映射**：见 design.md v3.2.0 §4（沿用 v1.0 + v1.1 全部 T，仅 import 路径同步）
- **评审结论**：S3 阶段产物（review-r1/r2/r3.md）

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：5+1 S 价值流切法 + 7 路径迁移 + contracts.go 拆分 + 桥接契约；S2_Clarified |
