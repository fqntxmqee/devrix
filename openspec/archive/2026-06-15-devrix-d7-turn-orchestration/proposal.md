# Proposal: D7 Turn 编排上移 — D7 直调 D3，D2 Thin 上下文面

**Change ID:** devrix-d7-turn-orchestration  
**Demand ID:** DM-20260614-020  
**Status:** Archived  
**Phase Scope:** D + S + 跨域边界（A/F 在 design.md）

---

## 1. Background

### 1.0 因果链：DM-020 → DM-018（双边共识 G-01）

> **D7 Turn 编排上移不是 D7 的孤立优化，而是三个 SA Refine 的汇聚点。**

```
DM-020: D7 Turn 编排上移（D7 获得 LLM 调用权）
    │
    ├─ D7 成为"真正的 Leader"（拥有 LLM 编排的硬权力）
    │
    ├─ LLM 调用权 + Hub-Spoke 编排权 = 互补性资产
    │     D7 调完 LLM 后如需经 D4 派发 Worker → 双重边际化
    │
    └─ DM-018: D4 交出 Hub-Spoke 编排权
          ├─ D4-S10 Delegate → D4-S14 ExecuteWorker（执行面）+ D7-S2/S4 Hub-Spoke（编排面）
          ├─ D2 SubQuery Flow 发布同迁 D7（三 Spoke 统一出口）
          └─ 与 DM-009（D2 交出 LLM 调用权）对称——三个 SA Refine 的汇聚点是 D7
```

**一句话：** 这是 Stackelberg 均衡修正——把 de facto 权力收拢到 de jure Leader（D7），Follower（D2/D4）只保留域内执行比较优势。

**双边共识：** 详见 `gaming-analysis-bilateral-consensus.md` §7；G-01~G-12 全部确认；P1~P3 Owner 级议题已闭合。

---

D2 SA Refine（DM-009）将 D2 定义为 **Execution Follower**，D7 SA Refine（DM-008）将 D7 定义为 **Orchestration Leader**。但运行时与规格仍存在 **Turn 循环归属错位**：

| 域 | 规格声称 | 代码现实 |
|----|---------|---------|
| D7 | Leader；「不拥有 LLM 调用」 | 黑盒 `QueryLoopExecutor` 委托 D2 |
| D2 | Follower；S16 RunQueryLoop | `ILLMGateway` 注入 + `query.Loop` 调 D3 |
| D3 | 水平 LLM 能力 | 被 D2 间接消费，D7 不可见 Breaker |

Owner 澄清目标：**D3 不应被 D2 调用；D7 调 D2 做上下文组装，再调 D3 完成任务执行。**

---

## 2. Problem Statement

### 2.1 Leader/Follower 倒置

```
【倒置 — 现状】
D7 (Leader) ──黑盒──▶ D2 (Follower) ──直连──▶ D3 (LLM)
                         ↑
                    实际拥有 Turn 循环
```

D7 无法：
- 在 Turn 级感知 D3 Breaker 状态选择路径
- 在 FastPath 与 Wave 间统一 LLM 调度语义
- 对 SubQuery 内层循环施加与主 Turn 相同的编排策略

### 2.2 规格自相矛盾

| 文档 | 矛盾句 |
|------|--------|
| `d7-domain.md` | 「D3 不直接和 D7 交互」 |
| `d2-domain.md` | North Star 含「执行 LLM↔Tool 多轮循环」 |
| `cross-domain-boundaries.md` §2.1.2 | QueryLoop SoT = D2 |
| `d7-boundary.md` §4.1 | `✅ D2 → D3` 合法 |

### 2.3 边界泄漏点

| 泄漏路径 | 文件 | 目标态 |
|---------|------|--------|
| 主 Turn LLM | `query/adapters.go` NewLLMCaller | D7 `turn/llm.go` |
| Autocompact 摘要 | `prepare/compression/llm_summarizer.go` | D7 调 D3 |
| SubQuery 内循环 | `nested/` via QueryLoop | D7 包装 Turn |
| Bootstrap 接线 | `bootstrap/context_engine*.go` WireContextLLM | `bootstrap/turn_orchestrator.go` |

---

## 3. North Star（修订后）

### D7 — Turn Leader

**拥有单会话 Turn 循环：prepare → route → llm → tools → persist，并向 D1 流式发布 EngineEvent。**

### D2 — Context Follower

**在给定 Turn 参数下，准备上下文、执行工具策略、持久化状态——不发起 LLM 调用。**

### D3 — LLM Horizontal

**流式推理 + 韧性保护；主消费方为 D7（经 `bridges/llm`）。**

---

## 4. Capabilities 清单

| Cap ID | Name | 域 | 类型 |
|--------|------|-----|------|
| CAP-TURN-01 | RunTurnLoop | D7-S2 | 新增 Canonical |
| CAP-TURN-02 | InvokeLLMStream | D7-S2 | 新增 Canonical |
| CAP-TURN-03 | RouteModelForTurn | D7→D3-S1 | 跨域 |
| CAP-CTX-01 | PrepareExecutionContext | D2-S15 | 保留 |
| CAP-CTX-02 | ExecuteToolRound | D2-S18 | 扩展（自 S16 拆出） |
| CAP-CTX-03 | PersistSessionState | D2-S17 | 保留 |
| CAP-CTX-04 | NestedTurnScope | D2-S19 + D7 包装 | 修订 |
| CAP-BND-01 | ForbidD2ToD3 | 架构 | import lint |
| CAP-BND-02 | LLMBridgeConsumerD7 | bridges/llm | 消费方迁移 |

---

## 5. S 层变更（Canonical + Legacy 双轨）

### 5.1 D7 扩展（S2 Session Orchestrator）

| Canonical S | 新增 A | Promise |
|-------------|--------|---------|
| D7-S2 | **A06 RunTurnLoop** | 多轮 tool_use 主编排 |
| D7-S2 | **A07 InvokeLLM** | 单次 D3 流式调用 + RouteModel |

### 5.2 D2 修订

| Canonical S | 变更 |
|-------------|------|
| D2-S15 | 不变；Out of Scope 加「LLM 调用」 |
| **D2-S16** | **Legacy 冻结**；Canonical 能力迁 D7-S2-A06 + D2-S18 |
| D2-S18 | 扩展 `ExecuteToolRound` A |
| D2-S19 | SubQuery 执行机制保留；LLM 由 D7 Turn 包装 |

### 5.3 D3 消费方变更（规格层）

| 契约 | v1.0 登记 | v2.0 代码 |
|------|----------|----------|
| `ILLMGateway` | 主消费方 **D7** | bootstrap 迁接线 |
| `ITierResolver` | D7 InvokeLLM 前调用 | 同左 |

---

## 6. R1 决议摘要（已闭合）

| OQ | 决议 |
|----|------|
| OQ1 | D2-S16 Legacy 保留；Canonical → D7-A06 + D2-S18 |
| OQ2 | v2.0 拆 `QueryLoopExecutor`；v1.0 登记三接口 |
| OQ3 | SubQuery 由 D7 包装 Turn |
| OQ4 | D7 InvokeLLM 复用 D3-S1 RouteModel |

---

## 7. 目标调用链（SoT）

```text
D1.Gateway
  └── D7.IOrchestrationEntry.ProcessMessage
        ├── [simple] D7-S2-A06 RunTurnLoop
        │     ├── D2-S15 PrepareExecutionContext
        │     ├── D3-S1 RouteModel（D7 内）
        │     ├── D3-S2 StreamChat（D7 直调 bridges/llm）
        │     ├── D2-S18 ExecuteToolRound（若有 tool_calls）
        │     └── D2-S17 PersistSessionState
        ├── [nested] D7 包装 D2-S19 SubQuery → 递归 RunTurnLoop（同链）
        └── [compress] D2 返回 CompressRequest → D7 调 D3 摘要 → D2 合并
```

**禁止边：** `D2 → D3`（任何 import / 调用）

---

## 8. 跨域文档增量

| 文档 | 动作 |
|------|------|
| `d7-orchestration/d3-boundary.md` | **新建** |
| `d7-orchestration/d7-domain.md` | 修订 North Star + 删除「D3 不交互」 |
| `d2-context-engine/d2-domain.md` | 修订 North Star；S16 Legacy |
| `d2-context-engine/d7-boundary.md` | 调用链 + 禁止 D2→D3 |
| `architecture/cross-domain-boundaries.md` | §2.1 重写 + §D7↔D3 |
| `d7/d2 a-registry.md` | A06/A07 + S18 扩展 |

---

## 9. Phase 划分

| Phase | 范围 | Go 变更 |
|-------|------|---------|
| **v1.0 Registry** | 规格 + 边界 + T 映射 | **0** |
| **v2.0 Structure** | slice a–f 代码迁移 | 是 |
| v1.1（可选） | Span `orchestration.turn.*` + D6 probe | 小 |

---

## 10. 与 DM-018 / DM-009 并行策略（因果链 G-01）

| 顺序 | Change | 理由 |
|------|--------|------|
| 1 | DM-020 v2.0 slice a–c（Turn 骨架 + FastPath） | 先建立 D7 Turn 骨架（LLM 编排权落地），再迁 Hub-Spoke |
| 2 | DM-018 v2.0 slice a（hubspoke 骨架） | 正交模块；DM-020 先行减少 bootstrap 冲突 |
| 3 | 两者 slice d–f 可并行 | 不同子包 |
| — | DM-009 v2.0 | D2 交出 LLM 调用权，与 DM-020 强耦合；三 SA Refine 汇聚 D7 |

---

## 11. Grill 检查点（S3-Gate 前）

- [ ] 每个 P0 AC 有 Gherkin Scenario（design.md）
- [ ] D2-S16 Legacy Archive 100% T 映射
- [ ] SubQuery 嵌套深度 + cancel 传播 sad path
- [ ] Autocompact 灰区不引入 D2→D3 回退

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | S2 Proposal + R1 闭合 |
| 0.2 | 2026-06-15 | 双边共识落盘：因果链 G-01 前言 + DM-018/DM-009 并行策略互引 |
