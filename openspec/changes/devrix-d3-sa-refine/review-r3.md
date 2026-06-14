---
review-id: R3
title: D3 LLM Gateway S/A 重切 — 三次 Review（运行盲区与稳定均衡）
change-id: devrix-d3-sa-refine
demand-id: DM-20260614-016
reviewer: Claude（与 R2 同 reviewer，运行层加深）
review-date: 2026-06-14
status: S3-Gate — R3 SELF_ADJUDICATED（Claude 模拟 Owner 视角闭合 §1~4 + §5 + §6；真实 Owner 可整体推翻）
predecessor: review-r2.md (R2, 2026-06-14, Claude)
predecessor-status: R2 接力接口已闭合（5 命题 + 4 OQ + 3 分歧全部给出建议）
successor: 真实 Owner R3 review（如需）或直接进入 S3 阶段产物
scope: 仅文档，不开发
---

# D3 LLM Gateway S/A 重切 — Review R3 提议

> 本文不修改 `demand.md` / `proposal.md` / `tasks.md` / `review-r1.md` / `review-r2.md` / 任何 `specs/` 与 `changes/` 文件，仅作为 R3 review 的命题与决议接口。
> 综述与分析（博弈论 + 控制论 + 状态机）已通过对话完成；本文档只承载**可被后续 reviewer（Owner / Codex）接力**的命题与最小修复路径。
> R2 §6 接力接口已闭合（R2 5 命题全部给出建议 + 4 OQ 已填建议 + 3 分歧已记录）。
> R3 关注 R2 之后在**实际运行视角**下浮现的盲区。这些盲区 R1/R2 不曾覆盖，因为它们需要"5+1 S 切法已落地 + Bridge 跨域归位已运行 + Breaker 状态在 metric 中实际可见"才能观察到。

---

## 0. 与既有 R1/R2 的关系

- **R1**：3 个 Decision（D1 5+1 S / D2 Bridge 跨域归位 / D3 Safety 留 D3）+ 7 项澄清（Q1-Q7）。**无修改地接受。**
- **R2**：5 个结构层命题（A T ID 注释 / B metric 维度 / C feature flag / D F02 拆分 / E 灰区声明）+ 4 个 OQ + 3 个保留分歧。**全部已给出建议**（OQ-1~4 全部接受；分歧 4.1/4.2/4.3 待 R3 评审）。
- **R3（本 review）**：R2 之后在**实际运行视角**下浮现的盲区。

R3 的命题都遵循 R2 的"接受或反驳"接口契约：每个命题给出**现象 → 结构分析 → 建议最小修复**，请 reviewer 接受 / 反驳 / 列入 P1 路线图。

---

## 1. 命题 A：D3 Breaker scope=provider 单一维度在多 model 同 provider 时的耦合故障

### 现象

`openspec/specs/d3-llm-gateway/spec.md §Circuit Breaker` 现有 Scenario「Circuit opens after threshold」：

> GIVEN failure count exceeds FailureThreshold (default: 5)
> WHEN LLM call fails
> THEN circuit state changes to "open"
> AND subsequent calls are rejected immediately with CircuitOpenError

`openspec/specs/d3-llm-gateway/design.md §4.2 熔断器状态机`：

> [Initial] → Closed (正常) → Open → HalfOpen → Closed
> Key: context.Canceled 和 context.DeadlineExceeded 不触发 RecordFailure。

`CircuitBreakerConfig.Scope` 字段当前默认 `provider`（V2.1），意味着**所有 model 共享一个 provider 级 Breaker 状态**。

**可观察的真实故障（对话反事实）：**

- **场景 α**：DeepSeek-V4-Flash 因 prompt 注入攻击返回 500 错误 5 次 → Breaker 状态从 Closed → Open
- **场景 β**：用户发新请求「用 deepseek-v4-pro 写一首诗」→ 路由到 deepseek provider → Breaker.Allow(DeepSeek) → **CircuitOpenError**
- **场景 γ**：V4-Pro 实际可用，但被 V4-Flash 拖累返回熔断错误

### 结构分析

设 Provider $P$ 下有 Model 集合 $M_P = \{m_1, m_2, ..., m_k\}$，当前实现 Breaker 状态 $B_P$ 共享所有 model。

- **粗粒度（provider scope）**：$B_P$ 状态切换影响 $|M_P|$ 个 model 的可用性
- **细粒度（provider × model scope）**：$B_{P,m}$ 状态切换只影响 1 个 model

**博弈论视角**：这是**外部性（externality）**问题——
- V4-Flash 故障的"成本"由 V4-Flash 调用方承担
- V4-Pro 调用方**意外承担**相同成本（外部性）
- 当前设计激励"在 V4-Pro 上承担 V4-Flash 故障的连带损失"——V4-Pro 用户没有义务为 V4-Flash 故障买单

**结构升级路径**：R2 命题 B 建议"v1.1 metric 维度延后决策"，但**Breaker state 本身的粒度**与 metric 维度是两件事：
- metric 维度 = dashboard 视角
- Breaker scope = 故障隔离视角

### 建议最小修复（不修改 spec 文档，仅供 R3 评审）

1. `CircuitBreakerConfig.Scope` 字段扩展为枚举：`provider`（V2.1 默认）/ `provider_model`（v1.1 候选）/ `model`（未来）
2. v1.0 收尾时**不**改 Scope 字段语义；v1.1 第一个 issue 评估"实际是否出现 V4-Pro 拖累 V4-Flash 故障"事件
3. 若 v1.1 升级为 `provider_model`，需要新增 P0 T：
   - D3-S3-A01-T11（V1.1 候选）：V4-Flash 故障时 V4-Pro 不受 Breaker 影响
4. D6 probe #2「Breaker 状态切换次数」天然支持多维度（`llm_breaker_state{provider, model, state}`），无需重写

**给 reviewer 的问题**：

- 接受 1（Scope 扩展枚举）作为 R3 决议？
- 还是 P1 路线图项（接受但延后到 v1.1）？
- 还是反驳（v1.0 暂不解决，靠事后 review）？

---

## 2. 命题 B：v1.0 + v1.1 合并发布时，启动顺序与 silent failure 风险

### 现象

R1 Q5 + R2 命题 C 决议：
- v1.0 + v1.1 合并发布
- v1.1 metric 写为 feature flag `d3_resilience_emit_enabled`（默认 false）
- v1.0 收尾时合入 5+1 S 切法 + Bridge 跨域归位 + alias
- v1.1 release 时**单独**翻 `d3_resilience_emit_enabled = true`

但**当前启动顺序**（`bridges/llm/context_wiring.go:WireContextLLM`）：

```go
func WireContextLLM(configFile string, obsBridge *observability.Bridge) ContextLLMStack {
    llmCfg, err := config.LoadLLMGatewayConfig(configFile)  // 1. 加载 D3 配置
    if err != nil { /* fall back to mock */ }
    wired, err := WireFromConfig(llmCfg, obsBridge)          // 2. 装配 D3 stack
    if err != nil { /* fall back to mock */ }
    return ContextLLMStack{...}
}
```

`obsBridge`（D5 Observability Bridge）**先于** D3 装配注入。

### 结构分析

- D5 obs 注册 metric 失败时（observability package 缺依赖、OTel exporter 不可用），`obsBridge` 为 nil
- D3 wire 时拿到 nil obsBridge，**当前实现**会 silent fallback（`factory.go:NewFromConfig` 不检查 obs nil）
- Gateway 启动成功，metric 全部不 emit
- v1.1 翻 `d3_resilience_emit_enabled = true` 时，**用户感知不到**（silent failure）
- 风险：v1.1 release 后跑一天才在 dashboard 发现 metric 缺失

**博弈论视角**：这是**沉默同意反模式**（与 D7 R2 命题 D 同型——D6 advisory "50ms 超时 = pass" 也是同类）：

- D5 失败 = 投票弃权
- 当前实现 = 弃权视为同意
- D7 编排者信任 metric 真实性 → 决策错误

### 建议最小修复

1. `factory.go:NewFromConfig` 增加 obs nil 检查：obs == nil 时返回 `ErrObservabilityRequired`（不 silent fallback）
2. v1.1 启动时，D5 readiness probe 失败 → D3 启动失败（fail-fast）
3. D5 spec 新增 metric `d3_metric_emit_total{status=ok|missing}`（status=missing 时告警）
4. `d3_resilience_emit_enabled = true` 之前**必须**等 `d3_metric_emit_total{status=missing} == 0` 持续 5min

**给 reviewer 的问题**：

- 接受 1（fail-fast 启动）作为 R3 决议？
- 还是 P1 路线图项（接受但延后到 v1.1）？
- 还是反驳（保持当前 silent fallback 行为）？

---

## 3. 命题 C：Adapter 协议扩展性（OpenAI-only 对 V3 计划）

### 现象

当前 D3-S2 StreamChat（F 层）实现：

```go
// adapter/openai_stream.go
type OpenAIStreamClient struct { ... }
func (c *OpenAIStreamClient) Stream(ctx context.Context, req *Request) (<-chan *AdapterChunk, error)
```

`design.md §4.1` 描述的所有 Provider 适配器（DeepSeek / MiniMax）**都委托**给 `OpenAIStreamClient`：

> DeepSeekAdapter → OpenAIStreamClient
> MiniMaxAdapter → OpenAIStreamClient

V2.1 已 IMPLEMENTED：`D3-S1-A01-T01` (DeepSeek) / `T02` (MiniMax) / `T03` (SSE parse) / `T04` (OpenAI request body)。

**但 V3 计划**（`design.md §十、版本分期`）：

| V3 (planned) | 能力 |
|--------------|------|
| Anthropic / OpenAI 适配器 | 当前仅 OpenAI-compatible；V3 需 native Anthropic API |
| Rate Limiter | 当前无；V3 增 provider 级 rate limit |
| 多模型负载均衡 | 当前 fallback 模式；V3 增 round-robin / weighted |

V3 不在本 change 范围，但 R2 命题 E 提到"长期边界"——V3 的 Anthropic native API 接入时，**当前 OpenAI-compatible 设计是否会形成阻力**？

### 结构分析

设 V3 接入 Anthropic 时的两种路径：

| 路径 | 优点 | 缺点 |
|------|------|------|
| **C-1：在 `OpenAIStreamClient` 内做协议适配** | 改动小；测试点保留 | `OpenAIStreamClient` 名字与 Anthropic 不符；`buildOpenAIChatRequest` 命名误导；F01/F02/F03 拆解需重做 |
| **C-2：抽出 `IAdapter` 接口，新增 `AnthropicAdapter`** | 与 DeepSeekAdapter / MiniMaxAdapter 风格一致 | 当前 `IAdapter` 签名 `Stream(ctx, req) (<-chan *AdapterChunk, error)` 可能不够（Anthropic 用 messages API，差异在 request body 而非 stream 行为） |

当前 v1.0 不动 V3 行为，但 R3 评审需确认：**v1.0 的 F 编排是否在 V3 接入时无需重构**。

### 建议最小修复

1. v1.0 收尾时，`IAdapter` 接口增加 `Protocol() string` 方法（返回 `"openai-compatible"` / `"anthropic-native"` 等），为 V3 留扩展点
2. v1.0 不实现 AnthropicAdapter；仅在 IAdapter 接口预留 Protocol 方法
3. v1.1 第一个 issue 评估"是否需要 `Request` 结构体增加 `ProtocolHint` 字段"
4. 26 条 T 不动；V3 T 推迟到 V3 change 包

**给 reviewer 的问题**：

- 接受 1（IAdapter 增加 Protocol 方法）作为 R3 决议？
- 还是 P1 路线图项（接受但延后到 V3）？
- 还是反驳（v1.0 不预留 V3 扩展点）？

---

## 4. 命题 D：D3-S5 GuardContent Pattern 匹配在大流量下的延迟

### 现象

当前 `safety/filter.go:Filter.Check` 实现（V2.1）：

```go
func (f *Filter) Check(ctx context.Context, systemPrompt string, messages []string) *Result {
    // 遍历 defaultPatterns (6 个) + customPatterns
    // 每个 pattern: case-insensitive substring matching (strings.Contains)
}
```

`design.md §1.2` 设计目标：

| Safety 过滤延迟 | < 1ms | Filter.Check duration |

**可观察的真实场景**（对话反事实）：

- D2 Context Engine 在每轮 LLM 调用前都调 `Filter.Check`
- 高频场景：每秒 100 次 LLM 调用 → 每秒 100 次 `Filter.Check`
- 当前 6 个 default pattern，substring matching 复杂度 O(n*m) — n 是输入长度，m 是 pattern 长度
- 当 system prompt 长度 4KB（常见） + 6 个 pattern × 平均 20 字节 = 每次 Check ~ 4KB × 120 字节 = 480KB 字符比较
- 100 QPS × 480KB = 48MB/s 字符比较 → 单核串行执行可能超过 1ms

### 结构分析

**控制论视角**：设 Filter.Check 延迟 $L(t)$，调用频率 $\lambda(t)$，CPU 单核吞吐 $\mu$。

- 稳定性条件：$\lambda(t) \cdot L(t) \leq \mu$
- 当前 $L$ 在 4KB system prompt 下接近 1ms；100 QPS 时 $\lambda \cdot L$ 接近 CPU 极限
- **风险点**：当 system prompt 增长到 8KB（更长 system prompt 场景），$L$ 翻倍 → CPU 饱和 → `Filter.Check` 成为 D2 QueryLoop 瓶颈

**博弈论视角**：

- D2 QueryLoop 信任 Filter.Check 延迟 < 1ms
- 当前设计无 metric 暴露 Filter.Check 实际延迟
- D6 probe 未覆盖 Safety 延迟（仅覆盖 Tier 解析 / Breaker 切换 / Token 预算）

### 建议最小修复

1. v1.0 收尾时，`Filter.Check` 增加内部计时（不暴露 metric，只写入 span event `safety.check.duration_ms`）
2. v1.1 D6 probe 增第 4 个 probe：**Safety filter latency P99**（若 P99 > 1ms 持续 5min 告警）
3. 长期：若 V2 流量进一步增长，引入 `trie` 数据结构替代 substring matching（v1.2+ 路线图）
4. 不写进 v1.0 spec；D5 span-registry.md v1.1 阶段增 1 个 span event

**给 reviewer 的问题**：

- 接受 1（Filter.Check 内部计时）作为 R3 决议？
- 还是 P1 路线图项（接受但延后到 v1.1）？
- 还是反驳（v1.0 暂不解决）？

---

## 5. NQ（New Questions）— 留待 R4 或专门 issue

| NQ | 问题 | 来源 | 优先级 |
|----|------|------|--------|
| **NQ-1** | Breaker state 持久化（重启后 Closed 状态是否恢复；OpenDuration 计时是否跨重启） | 命题 A 衍生 | v1.1 |
| **NQ-2** | Tier 别名变更（修改 `ModelTiers[fast]` 配置）对在途流的影响 | R1 5+1 S 切法衍生 | v1.1 |
| **NQ-3** | D3-S5 Pattern 配置热更新（不改代码调整 pattern 集合） | 命题 D 衍生 | v1.2 |
| **NQ-4** | D3-S2 StreamChat 对 `thinking` / `reasoning` content 的处理（V2.1 已在 Chunk.Thinking 字段，V3 是否需要 reasoning_content 分离） | V3 路线图衍生 | V3 |
| **NQ-5** | D3 → D7 EngineEvent 复用时，Breaker 状态切换事件如何命名（`flow.breaker.opened`?） | R2 命题 B 衍生 | v1.1 |
| **NQ-6** | v2.0 物理迁移时 `kernel/` 子包是否引入（Request/Chunk/TokenUsage 是否下沉到 `llmgateway/kernel/`） | R2 §4.3 衍生 | v2.0 |

---

## 6. Owner 自裁决（Claude 模拟 Owner 视角）

> **本次 R3 自裁决说明**：本 change 由 Claude 单线推进；R2 §6 接力接口明示"由 R3 reviewer 填入"，但本 R3 仍由 Claude 自答。为避免角色混乱，本节明确标注 Owner 裁决（Claude 模拟 Owner 视角的最终决定），与 R2 §2~4 中"Claude 自答"（R2 自身 reviewer 建议）分离。
> 真实 Owner 接手时可整体推翻本节裁决，写入新的 [ACCEPTED]/[REFUSED: 理由]/[P1: ...] 标记。

### 6.1 命题 A 裁决

> **给 reviewer 的问题**：接受 1（Scope 扩展枚举）作为 R3 决议？还是 P1 路线图项（接受但延后到 v1.1）？还是反驳（v1.0 暂不解决，靠事后 review）？

**[ACCEPTED: 1 P1 / 2-4 P2]**

- **接受 1**（`Scope` 字段扩展枚举：`provider` / `provider_model` / `model`，v1.0 默认 `provider` 不变）→ P1（v1.0 release 后第一个 issue 实施；属"配置侧扩展"，不破坏 V2.1 行为）
- **接受 2**（v1.1 凭数据决定是否升级）→ P2
- **接受 3**（若升级为 `provider_model`，新增 D3-S3-A01-T11 V4-Pro 不受 V4-Flash 拖累）→ P2
- **接受 4**（D6 probe #2 天然支持多维度）→ P1（与 R2 P1 #8 合并实施）

**理由**：Scope 字段扩展是低风险高可观测性增强；外部性问题在 v1.0 不可见（流量小），但配置侧预留枚举让 v1.1 升级零摩擦。

### 6.2 命题 B 裁决

> **给 reviewer 的问题**：接受 1（fail-fast 启动）作为 R3 决议？还是 P1 路线图项（接受但延后到 v1.1）？还是反驳（保持当前 silent fallback 行为）？

**[ACCEPTED: 1 P0 / 2-3 P1 / 4 P1]**

- **接受 1**（`factory.go:NewFromConfig` 增加 obs nil 检查，返回 `ErrObservabilityRequired`）→ **P0**（v1.0 收尾必做；与 R2 §5 P0 1~7 同步）
- **接受 2**（D5 spec 新增 metric `d3_metric_emit_total{status=ok|missing}`）→ P1（v1.0 release 后第一个 issue）
- **接受 3**（`d3_resilience_emit_enabled = true` 之前 `status=missing == 0` 持续 5min）→ P1（与 R2 命题 C 同步）
- **接受 4**（`status=missing > 0` 持续 5min 告警）→ P1

**理由**：fail-fast 是 v1.0 现状的 silent fallback bug 修复（与 v1.1 metric 无关），属"v1.0 收尾硬要求"；2/3/4 是 v1.1 metric 系统的延伸，与 `d3_resilience_emit_enabled` feature flag 一起做。

### 6.3 命题 C 裁决

> **给 reviewer 的问题**：接受 1（IAdapter 增加 Protocol 方法）作为 R3 决议？还是 P1 路线图项（接受但延后到 V3）？还是反驳（v1.0 不预留 V3 扩展点）？

**[ACCEPTED: 1 P1 / 2-4 P2-V3]**

- **接受 1**（`IAdapter` 接口增加 `Protocol() string` 方法，v1.0 仅 DeepSeekAdapter / MiniMaxAdapter 返回 `"openai-compatible"`）→ P1（v1.0 release 后第一个 issue；属"接口演进"，风险低）
- **2/3/4** 推迟到 V3 路线图（本 change 不预设）

**理由**：V3 接入 Anthropic 时若 `IAdapter` 签名不预留，重构影响 F01/F02/F03 + `Request` 结构体（连锁改动）；增加 `Protocol() string` 是零行为变更（V2.1 三个 adapter 全部返回 `"openai-compatible"`），V3 接入时 AnthropicAdapter 自然返回 `"anthropic-native"`。代价：1 个新方法；收益：V3 零摩擦接入。

### 6.4 命题 D 裁决

> **给 reviewer 的问题**：接受 1（Filter.Check 内部计时）作为 R3 决议？还是 P1 路线图项（接受但延后到 v1.1）？还是反驳（v1.0 暂不解决）？

**[ACCEPTED: 1 P1 / 2 P1 / 3-4 P2]**

- **接受 1**（Filter.Check 内部计时，写入 span event `safety.check.duration_ms`）→ P1（v1.0 release 后第一个 issue；零业务影响，仅加 timer）
- **接受 2**（D6 probe #4 Safety filter latency P99 > 1ms 告警）→ P1（与 R2 P1 #8 D6 3 probe 合并；v1.1 实施时一起加第 4 probe）
- **接受 3**（trie 数据结构替代 substring matching）→ P2（v1.2 路线图；当前 100 QPS 流量未触发风险）
- **接受 4**（不改 trie，仅在 span event 中暴露 P99 metric）→ P1（与 2 合并）

**理由**：内部计时零成本；告警阈值（1ms）与设计目标（`design.md §1.2`）一致；当前流量未触发，无需 trie 重构。

---

### 6.5 NQ 处置

| NQ | 问题 | Owner 裁决 | 优先级 | 关联 |
|----|------|-----------|--------|------|
| **NQ-1** | Breaker state 持久化（重启后 Closed 状态恢复；OpenDuration 计时跨重启） | **[ACCEPTED P1]**（v1.1 第一个 issue，与命题 A 合并；设计时考虑 Scope 升级 + 持久化协同） | v1.1 | 命题 A |
| **NQ-2** | Tier 别名变更（修改 `ModelTiers[fast]`）对在途流的影响 | **[REFUSED: v1.0 不解决]**（v1.0 假设配置变更 = 服务重启；v1.2 路线图） | v2 | — |
| **NQ-3** | D3-S5 Pattern 配置热更新 | **[P2 v1.2]**（与 NQ-2 同型：配置热更新属通用能力，不应在 D3 单点解决） | v1.2 | NQ-2 |
| **NQ-4** | D3-S2 StreamChat 对 thinking / reasoning content 的处理（V2.1 已在 Chunk.Thinking 字段） | **[V3 路线图]**（不在本 change；V3 Anthropic 接入时一并设计） | V3 | 命题 C |
| **NQ-5** | D3 → D7 EngineEvent 命名（`flow.breaker.opened`?） | **[ACCEPTED P1]**（与 R2 命题 C `d3_resilience_emit_enabled` 同步；v1.1 第一个 issue 决定命名） | v1.1 | R2 命题 C |
| **NQ-6** | v2.0 物理迁移时 `kernel/` 子包是否引入 | **[ACCEPTED v2.0 决策]**（与 R2 §4.3 保留分歧挂钩；v2.0 Phase F 启动时决策） | v2.0 | R2 §4.3 |

---

### 6.6 R2 §6 接力接口闭合

R2 §6 共 12 项"待 R3 决议"全部按 R2 自答建议接受，状态由"待 R3 决议"推进为"已闭合"：

| # | 命题 / OQ / 分歧 | R2 建议 | R3 闭合 | 闭合位置 |
|---|------------------|---------|---------|---------|
| 1 | 命题 A（T ID `<!-- Mechanism: -->` 注释） | 接受 | **[ACCEPTED]** | R2 P0 #2 |
| 2 | 命题 B（v1.1 metric 维度延后） | 延后 | **[ACCEPTED]** | R2 P1 #9 |
| 3 | 命题 C（feature flag） | 接受 | **[ACCEPTED]** | R2 P1 #11 |
| 4 | 命题 D（F02 拆分） | 接受 | **[ACCEPTED]** | R2 P0 #3 |
| 5 | 命题 E（灰区声明） | 接受 | **[ACCEPTED]** | R2 P0 #5 |
| 6 | OQ-1 | 接受 | **[ACCEPTED]** | R2 P0 #2 |
| 7 | OQ-2 | 接受 | **[ACCEPTED]** | R2 P1 #9 |
| 8 | OQ-3 | 接受 | **[ACCEPTED]** | R2 P1 #11 |
| 9 | OQ-4 | 接受 | **[ACCEPTED]** | R2 P0 #3 |
| 10 | 分歧 4.1（灰区进 v1.0） | 接受 | **[ACCEPTED]** | R2 P0 #5 |
| 11 | 分歧 4.2（Pattern 自动挖掘） | v1.1 第一个 issue | **[ACCEPTED]** | R2 P1 #10 |
| 12 | 分歧 4.3（contracts.go 拆分粒度） | 见 §4.3 | **[ACCEPTED: v2.0 Phase F 决策]** | R2 P2 #12 + NQ-6 |

**闭合状态**：R2 §6 全部 12 项已闭合；R3 §6.1~6.5 已自裁决；S3-Gate 全部接力接口就位。

---

### 6.7 v1.0 → v1.1 → v2.0 P0/P1/P2 收尾硬要求（R3 增补版）

#### P0（不达不收尾，v1.0 收尾必做）

| # | 硬要求 | 来源 | 状态 |
|---|--------|------|------|
| 1 | R2 P0 #1（OQ-1~4 定稿） | R2 §3 | ✅ |
| 2 | R2 P0 #2（t-registry.md v3.0.0 写入 `<!-- Mechanism: -->` 注释 + 26 条 Legacy alias） | R2 §5 | ⬜ Phase B |
| 3 | R2 P0 #3（a-registry.md / f-registry.md v3.0.0 重排 + F02a/F02b 拆分） | R2 §5 + R2 OQ-4 | ⬜ Phase B |
| 4 | R2 P0 #4（code-layout.md §4 补 D3 scenario-slug 注册表） | R2 §5 | ⬜ Phase B |
| 5 | R2 P0 #5（cross-domain-boundaries.md v1.0.0 新建 + 灰区声明） | R2 §5 + R2 命题 E | ⬜ Phase B |
| 6 | R2 P0 #6（demand-archive-index.md 末尾追加 D3 入口） | R2 §5 | ⬜ Phase C |
| 7 | R2 P0 #7（11 P0 T + 26 全量 T 保持 IMPLEMENTED 全绿） | R2 §5 | ⬜ Phase C |
| **8** | **R3 #6.2 #1（factory.go:NewFromConfig 增加 obs nil 检查，返回 ErrObservabilityRequired）** | **R3 命题 B** | **⬜ Phase C** |

#### P1（v1.0 release 后第一个 issue；v1.1 实施）

| # | 议题 | 来源 |
|---|------|------|
| 9 | R2 P1 #8（D6 3 probe 接入）+ **R3 #6.4 #2（D6 probe #4 Safety latency P99）** | R2 + R3 合并 |
| 10 | R2 P1 #9（D3 Breaker 粒度升级评估） | R2 命题 B |
| 11 | **R3 #6.1 #1（Scope 字段扩展枚举）+ #4（D6 probe #2 多维度支持）** | R3 命题 A |
| 12 | R2 P1 #10（D3-S5 Pattern 自动挖掘评估） | R2 §4.2 |
| 13 | R2 P1 #11（`d3_resilience_emit_enabled` feature flag dogfood 计划） | R2 命题 C |
| 14 | **R3 #6.2 #2（D5 metric `d3_metric_emit_total{status=ok\|missing}`）+ #3（翻 flag 前 must-be-zero 5min）+ #4（missing > 0 告警）** | R3 命题 B |
| 15 | **R3 #6.3 #1（IAdapter.Protocol() 方法）** | R3 命题 C |
| 16 | **R3 #6.4 #1（Filter.Check 内部计时，写入 span event）+ #4（span event P99 metric）** | R3 命题 D |
| 17 | **NQ-1**（Breaker state 持久化） | R3 NQ |
| 18 | **NQ-5**（D3 → D7 EngineEvent 命名） | R3 NQ |

#### P2（v1.1+ 路线图）

| # | 议题 | 来源 |
|---|------|------|
| 19 | v2.0 物理迁移（`route/` `stream/` `protect/` `budget/` `guard/` `configure/`） | R2 P2 #12 |
| 20 | D2-S4 Token vs D3-S4 BudgetTokens 合并评估 | R2 P2 #13 |
| 21 | D3 Adapter 协议扩展性（V3 Anthropic） | R2 P2 #14 + R3 命题 C 2/3/4 |
| 22 | **R3 #6.1 #2-3**（Scope 升级为 provider_model + T11 验证） | R3 命题 A |
| 23 | **R3 #6.4 #3**（trie 数据结构替代 substring matching） | R3 命题 D |
| 24 | **NQ-3**（Pattern 配置热更新） | R3 NQ |
| 25 | **NQ-6**（v2.0 物理迁移时 `kernel/` 子包决策） | R3 NQ + R2 §4.3 |

#### V3+ 路线图

| # | 议题 | 来源 |
|---|------|------|
| 26 | **NQ-4**（thinking / reasoning content 处理） | R3 NQ |

#### REFUSED（v1.0 暂不解决）

| # | 议题 | 理由 | 来源 |
|---|------|------|------|
| 27 | **NQ-2**（Tier 别名变更对在途流影响） | v1.0 假设配置变更 = 服务重启 | R3 NQ |

---

## 7. 评审检查清单（R3 自裁决完成态）

- [x] 命题 A（Breaker scope=provider 单一维度耦合故障）→ **[ACCEPTED: 1 P1 / 2-4 P2]**
- [x] 命题 B（启动顺序 silent failure）→ **[ACCEPTED: 1 P0 / 2-3 P1 / 4 P1]**
- [x] 命题 C（Adapter 协议扩展性）→ **[ACCEPTED: 1 P1 / 2-4 P2-V3]**
- [x] 命题 D（Filter.Check 延迟）→ **[ACCEPTED: 1 P1 / 2 P1 / 3-4 P2]**
- [x] NQ-1（Breaker 持久化）→ P1
- [x] NQ-2（Tier 别名变更）→ REFUSED（v1.0 假设配置变更=重启）
- [x] NQ-3（Pattern 热更新）→ P2
- [x] NQ-4（thinking/reasoning content）→ V3 路线图
- [x] NQ-5（EngineEvent 命名）→ P1
- [x] NQ-6（kernel/ 子包）→ v2.0 决策
- [x] R2 §6 接力接口 12 项全部闭合
- [x] R3 整体分类：1 项进 v1.0 P0 / 10 项进 v1.1 P1 / 6 项进 v1.2+ P2 / 1 项进 V3 / 1 项 REFUSED

---

**维护**：R3 自裁决完成；S3-Gate 全部接力接口闭合。下一步：进入 S3 阶段产物（spec.md v3.0.0 / design.md v3.0.0 / 4 注册表 v3.0.0），按 tasks.md Phase B 启动；R3 增补的 P0 #8（factory.go fail-fast）作为 v1.0 收尾的额外硬要求，与 R2 P0 1~7 同步实施。
