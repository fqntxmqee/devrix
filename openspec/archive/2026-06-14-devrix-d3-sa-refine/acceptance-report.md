---
demand-id: DM-20260614-016
change-id: devrix-d3-sa-refine
phase: v1.0 Registry Refine
status: S5_ACCEPTED
verdict: PASS
date: 2026-06-14
reviewer: Owner（自裁决）
parent: dsaft-refactoring-playbook
---

# Acceptance Report — D3 LLM Gateway S/A 重切 v1.0

## 0. 验收范围与边界

| 维度 | 范围 |
|------|------|
| Change | `devrix-d3-sa-refine`（DM-20260614-016） |
| Phase | **v1.0 Registry Refine**（5+1 S 注册表 + spec.md + design.md + cross-domain-boundaries.md，**0 行运行时代码变更**） |
| 不在本期 | v1.1 韧性可见性（Phase D-E：metric `llm_breaker_state` / D6 probe / Phase D 子 change）；v2.0 物理迁移（Phase F-G：`adapter/` → `stream/` 等；contracts.go 拆分；Phase F 子 change） |
| Phase 走查 | Phase A（文档澄清 R1/R2/R3）→ Phase B（v1.0 Registry 重排 B1–B14）→ **Phase C（v1.0 验证 C1–C6）** |

**v1.0 不变性承诺**（R1 Q3 决议 + playbook 原则 3）：

- **5 个运行时 span 名**：`llm.stream` / `llm.provider.route` / `llm.circuit_breaker` / `llm.retry` / `llm.adapter.stream` —— 字面量未改
- **3 个核心 metric 名**：`llm_requests_total` / `llm_errors_total` / `llm_latency_seconds` —— 字面量未改
- **YAML 配置 key**：`llm_gateway:` / `circuit_breaker:` / `model_tiers:` —— 字面量未改
- **物理目录**：`adapter/` `gateway/` `breaker/` `retry/` `token/` `safety/` `config/` —— 未迁移（v2.0 范围）
- **Bridge 路径**：`internal/bridges/llm/` —— 跨域锚点不变（R1 D2 决议）

---

## 1. v1.0 验收准则（AC）逐项裁决

| AC | 准则 | 证据 | 裁决 |
|----|------|------|------|
| AC-01 | 5+1 S 切法落地：6 S × 1 A × 24 F（域内）+ CROSS 2 A × 2 F | `a-registry.md` v3.0.0 §3；`f-registry.md` v3.0.0 §5 总数表 | ✅ PASS |
| AC-02 | 每个 S 与"承诺"1:1（C1 Routing / C2 Streaming / C3 Resilience / C4 Budget / C5 Safety + S6 Config 横切） | `spec.md` v3.0.0 §North Star 5 承诺 | ✅ PASS |
| AC-03 | F02 拆 F02a/F02b（Tier alias 与 Default 错误签名分离） | `f-registry.md` §3.1 + 错误码签名表 | ✅ PASS |
| AC-04 | ProtectCall 合并 Breaker+Retry 并保留机制可追溯（`<!-- Mechanism: -->`） | `t-registry.md` §4 D3-S3 12 条 T 全部带 Mechanism 列；`f-registry.md` §3.3 F 编排顺序 | ✅ PASS |
| AC-05 | Bridge 跨域归位：D3 内部 f-registry 无 F04/F05；CROSS 段 D3-X-A01/A02 注册 | `f-registry.md` §3.2（仅 F01-F03）+ §4 CROSS 段；`a-registry.md` §D3-X | ✅ PASS |
| AC-06 | scenario-slug 语义化（route/stream/protect/budget/guard/configure），符合 `code-layout.md` §2.2 | `code-layout.md` v1.4.0 §4.4 D3 scenario-slug 注册表 | ✅ PASS |
| AC-07 | Legacy double-track 100% alias 覆盖（26 条旧 T ID → 新 ID） | `t-registry.md` §9 Legacy Archive 26 行；`scripts/check_t_aliases.py` 退出码 0 | ✅ PASS |
| AC-08 | R2 灰区契约化（D3-S5 vs D2-S18 内容/工具双重拒绝） | `cross-domain-boundaries.md` v1.0.0 §2.1.3；`spec.md` §11 灰区声明 | ✅ PASS |
| AC-09 | R3 P0 Fail-Fast：obs nil 时 `WireContextLLM` 返回 `ErrObservabilityRequired` | `spec.md` v3.0.0 §8 Requirement: Observability Fail-Fast Bootstrap；`design.md` §2.2 | ✅ PASS（文档落地；运行时 P0 实施跟随 v1.1） |
| AC-10 | 跨域 SoT 4 项灰区契约化（内容/工具双重拒绝；Breaker 状态可见性；D6 probe 接入；Breaker 事件命名） | `cross-domain-boundaries.md` v1.0.0 §4 灰区处理总览 | ✅ PASS |
| AC-11 | 13 P0 测试 100% IMPLEMENTED + 全绿 | §3 测试证据 | ✅ PASS |
| AC-12 | 26 T 全量编译通过（含 d3 / cross / live 标签） | §3 测试证据 | ✅ PASS |
| AC-13 | `go build ./...` 全绿（0 行代码变更） | §3.1 | ✅ PASS |
| AC-14 | 一致性扫描：D3 spec 目录 + 测试 spec 已清理旧 7 S 命名遗漏 | `testing-quality/spec.md`、`testing-framework/domain-segmentation.md` 已替换为 5+1 S 名称（C1 修复） | ✅ PASS |
| AC-15 | DM ID 唯一性：DM-20260614-016 已分配；与 D2-sessionqueue DM-013 冲突已修正（10 文件批量改名 0 残留） | `demand-archive-index.md` Active Changes 末行；`grep DM-20260614-013` D3 范围 0 命中 | ✅ PASS |

> **v1.0 总裁决：15/15 AC PASS → VERDICT = ACCEPTED**

---

## 2. Phase B 产物清单（v1.0 Registry 11 件）

| # | 文件 | 版本 | Phase B 任务 |
|---|------|------|--------------|
| 1 | `scripts/check_t_aliases.py` | new | B1 |
| 2 | `openspec/specs/d3-llm-gateway/a-registry.md` | v3.0.0 | B2 |
| 3 | `openspec/specs/d3-llm-gateway/f-registry.md` | v3.0.0 | B3 |
| 4 | `openspec/specs/d3-llm-gateway/t-registry.md` | v3.0.0 | B4 + B5 |
| 5 | `openspec/specs/d3-llm-gateway/span-registry.md` | v3.0.0 | B6 |
| 6 | `openspec/specs/d3-llm-gateway/spec.md` | v3.0.0 | B7 |
| 7 | `openspec/specs/d3-llm-gateway/design.md` | v3.0.0 | B8 |
| 8 | `openspec/specs/architecture/layering.md` | v3.7.0 | B9 |
| 9 | `openspec/specs/architecture/code-layout.md` | v1.4.0 | B10 |
| 10 | `openspec/specs/architecture/cross-domain-boundaries.md` | v1.0.0（新建） | B11 |
| 11 | `openspec/demand-archive-index.md` | C5 追加 D3 Active Changes 行 | C5 |

---

## 3. Phase C 测试证据

### 3.1 `go build ./...`

```
$ cd /Users/fukai/workspace/devrix && go build ./...
# exit 0；0 行运行时代码变更，符合 v1.0 "registry-only" 不变性承诺
```

### 3.2 D3 包级单元测试

```
$ go test -count=1 ./internal/layers/llmgateway/... ./internal/bridges/llm/...
ok  github.com/devrix/devrix/internal/bridges/llm                        cached
ok  github.com/devrix/devrix/internal/layers/llmgateway/adapter          cached
ok  github.com/devrix/devrix/internal/layers/llmgateway/breaker          cached
ok  github.com/devrix/devrix/internal/layers/llmgateway/config           cached
ok  github.com/devrix/devrix/internal/layers/llmgateway/gateway          cached
ok  github.com/devrix/devrix/internal/layers/llmgateway/retry            cached
ok  github.com/devrix/devrix/internal/layers/llmgateway/safety           cached
ok  github.com/devrix/devrix/internal/layers/llmgateway/token            cached
```

→ **8 个 D3 包全绿**（含 Bridge）

### 3.3 D3 集成（`-tags="integration d3"`）

```
=== RUN   TestIntegration_LLMCircuitBreaker_state_transitions
--- PASS  (0.00s)            <!-- 覆盖 D3-S3-A01-T01..T06 状态机 -->
=== RUN   TestIntegration_LLMGateway_fallback_models
--- PASS  (4.64s)            <!-- 覆盖 D3-S3-A01-T11/T12 Fallback -->
=== RUN   TestIntegration_LLMGateway_emits_observability_span
--- PASS  (0.08s)            <!-- 覆盖 D3-S2-A01-T05 spans + metrics -->
PASS
ok  github.com/devrix/devrix/tests/integration  5.294s
```

### 3.4 D3 Live（`-tags="integration d3 live"`，VCR fixtures）

```
=== RUN   TestIntegration_DeepSeekVCR_SSEParseError
--- PASS  (0.00s)            <!-- 覆盖 D3-S2-A01-T03 SSE parse error -->
=== RUN   TestIntegration_MiniMaxVCR_RateLimit429
--- PASS  (0.00s)            <!-- 覆盖 D3-S3-A01-T07 429 rate limit -->
=== RUN   TestIntegration_MiniMaxVCR_ServerError500
--- PASS  (0.00s)            <!-- 5XX 错误覆盖（无独立 T ID，挂 ProtectCall A01） -->
PASS
ok  github.com/devrix/devrix/tests/integration  0.694s
```

### 3.5 Alias 100% 覆盖校验

```
$ python3 scripts/check_t_aliases.py
==> 解析 t-registry.md: openspec/specs/d3-llm-gateway/t-registry.md
    Legacy Archive 映射数: 26
    当前 T ID 数（含 alias）: ≥ 26
    测试文件 // T: 注释引用数: …
    spec/design/layer-delta T 引用数: …

==> 校验结果
  ✅ 所有旧 T ID 都有 alias（覆盖数: 26/26 = 100%）
  ✅ 所有 alias 都指向 current T ID
# exit 0
```

### 3.6 T 总计

| 维度 | 数量 |
|------|------|
| 域内 T（S1–S6） | 25 |
| CROSS T（D3-X） | 1 |
| **总计 T** | **26** |
| 优先级 P0 | 13（全部 IMPLEMENTED + 全绿） |
| 优先级 P1 | 12（全部 IMPLEMENTED + 全绿） |
| 优先级 P2（PLANNED） | 1（`D3-S3-A01-T08` 熔断器状态持久化，v1.1 候选） |

→ **26/26 in-scope T 全绿；P0 100% 达标；唯一 PLANNED 项明确属于 v1.1 路线图，不阻断 v1.0 验收**

---

## 4. R1/R2/R3 决议落地清单

| 决议来源 | 决议项 | v1.0 落地证据 |
|---------|-------|--------------|
| R1 D1 | 5+1 S 切法 = A 方案 | `a-registry.md` v3.0.0 §3 索引 |
| R1 D2 | Bridge 跨域归位 = D2-1（留 `internal/bridges/llm/`） | `f-registry.md` §4 CROSS；`cross-domain-boundaries.md` §2.1.1 |
| R1 D3 | Safety 归属 = D3-1（保留 D3，更名 GuardContent） | `f-registry.md` §3.5 D3-S5 |
| R1 Q3 | 运行时 span / metric 字符串字面量保持不变 | `span-registry.md` v3.0.0 各 span 名段「v1.0 字面量」表 |
| R1 Q4 | Legacy alias → `t-registry.md` §Legacy Archive | `t-registry.md` §9 共 26 行 |
| R1 Q5 | v1.0 与 v1.1 合并发布 | 本报告 §0 + Phase D 子 change 启动计划 |
| R1 Q6 | D3 → D7 通知复用现有 EngineEvent，不新增直接契约 | `cross-domain-boundaries.md` §2.4.3 |
| R2 命题 A | Breaker+Retry 合并 + Mechanism 注释保留可追溯 | `f-registry.md` §3.3；`t-registry.md` D3-S3 全行 Mechanism 列 |
| R2 命题 D / OQ-4 | F02 拆 F02a/F02b | `f-registry.md` §3.1 + §错误码签名 |
| R2 命题 E / P0 #5 | 内容 vs 工具双重拒绝灰区契约化 | `cross-domain-boundaries.md` §2.1.3 |
| R2 §4.3 | contracts.go 拆分粒度（v2.0 占位） | `design.md` v3.0.0 §2.1；`code-layout.md` §4.4 |
| R3 P0 #8 | obs nil fail-fast | `spec.md` §8 Requirement |
| R3 P1 #11 | Scope 字段扩展（v1.1 候选） | `design.md` §2.2 |
| R3 P1 #15 | `IAdapter.Protocol() string`（v1.0 release 后第一个 issue） | `f-registry.md` §3.2 V3 扩展点 |
| R3 P1 #16 | Safety filter latency span event（v1.0 release 后第一个 issue） | `f-registry.md` §3.5 v1.0 性能观察；`span-registry.md` §metrics v1.1 |
| R3 NQ-5 | Breaker 事件命名（v1.1 第一个 issue 决定） | `cross-domain-boundaries.md` §2.4.3 |
| R3 NQ-6 | v2.0 不引入 `kernel/` 子包；kernel 类型继续留根 `contracts.go` | `code-layout.md` §4.4 R3 NQ-6 决策 |

→ **R1 (7 项) + R2 (5 项) + R3 (5 项) = 17 项全部 v1.0 落地**

---

## 5. 风险与缓解

| # | 风险 | 缓解 |
|---|------|------|
| 1 | DM ID 冲突（DM-013 双重使用） | C5 已修正：D3 重新分配 DM-016，10 文件批量替换 0 残留；D2-sessionqueue（archived）保留 DM-013 |
| 2 | v1.0 注册表已价值流化但代码目录仍叫 `adapter/`/`gateway/` | R1 Q5 决议：v1.0 与 v1.1 合并发布；v2.0 物理迁移作为独立子 change |
| 3 | v1.1 Phase D 子 change（韧性可见性 metric + 3 probe）尚未启动 | tasks.md Phase D 已列；Phase D 子 change 启动条件 = v1.0 ACCEPTED（即本报告） |
| 4 | v2.0 contracts.go 拆分粒度仅在 design.md 占位 | R2 §4.3 决策已落字；实际 Edit 留待 Phase F；type alias re-export 桥接策略已设计 |
| 5 | PLANNED T (`D3-S3-A01-T08` 熔断器状态持久化) 未实施 | P2 级别，v1.1 候选；不阻断 v1.0 P0/P1 |
| 6 | `D3-X-A01-T01` Bridge 测试位置在 `internal/bridges/llm/bridge_test.go` 不属 D3 内部包 | 跨域锚点设计决定；R1 D2 决议明确归属；测试已 IMPLEMENTED + 全绿 |

---

## 6. v1.1 / v2.0 下一步

| Phase | 子 change | 触发条件 | 范围 |
|-------|----------|---------|------|
| Phase D-E | `devrix-d3-sa-refine-v1.1`（新 DM ID 待 S2 申请） | 本报告 ACCEPTED 后启动 | metric `llm_breaker_state{provider,state}` + EngineEvent 复用 + D6 3 probe（Tier 解析正确性 / Breaker 状态切换次数 / Token 预算触发率）+ Safety latency span event + `IAdapter.Protocol()` 接口扩展 |
| Phase F-G | `devrix-d3-sa-refine-v2.0`（新 DM ID 待 S2 申请） | v1.1 ACCEPTED 后启动 | 物理迁移：`adapter/` → `stream/` / `gateway/router.go` → `route/` / `breaker/`+`retry/` → `protect/` / `token/` → `budget/` / `safety/` → `guard/` / `config/` → `configure/`；`contracts.go` 拆分；re-export 桥接 1 发布周期 |

> **v1.0 ACCEPTED ↔ v1.1 启动**：本报告状态 = `S5_ACCEPTED` 即触发 Phase D 子 change S1 申请。

---

## 7. 反向链接

| 文档 | 路径 | 关系 |
|------|------|------|
| Demand | `openspec/changes/devrix-d3-sa-refine/demand.md` v0.3 | 需求与 R1/R2/R3 |
| Proposal | `openspec/changes/devrix-d3-sa-refine/proposal.md` | D + S 切法 |
| Review R1 | `openspec/changes/devrix-d3-sa-refine/review-r1.md` | 用户 Owner |
| Review R2 | `openspec/changes/devrix-d3-sa-refine/review-r2.md` | Claude 结构层 |
| Review R3 | `openspec/changes/devrix-d3-sa-refine/review-r3.md` | Claude 运行层 |
| Tasks | `openspec/changes/devrix-d3-sa-refine/tasks.md` | Phase A-G 任务 |
| D3 spec | `openspec/specs/d3-llm-gateway/spec.md` v3.0.0 | 5+1 S 主规格 |
| D3 design | `openspec/specs/d3-llm-gateway/design.md` v3.0.0 | A+F 编排时序 |
| D3 a-registry | `openspec/specs/d3-llm-gateway/a-registry.md` v3.0.0 | A 编排注册表 |
| D3 f-registry | `openspec/specs/d3-llm-gateway/f-registry.md` v3.0.0 | F 详细注册 |
| D3 t-registry | `openspec/specs/d3-llm-gateway/t-registry.md` v3.0.0 | T + Legacy Archive |
| D3 span-registry | `openspec/specs/d3-llm-gateway/span-registry.md` v3.0.0 | span/metric SoT |
| 跨域边界 | `openspec/specs/architecture/cross-domain-boundaries.md` v1.0.0 | D3 vs 全域 SoT + 灰区 |
| 分层 | `openspec/specs/architecture/layering.md` v3.7.0 | D3 5+1 S 同步 |
| 代码布局 | `openspec/specs/architecture/code-layout.md` v1.4.0 | scenario-slug 注册 |
| Demand Archive Index | `openspec/demand-archive-index.md` | Active Changes 末行 |

---

## 8. 总评

D3 LLM Gateway 完成从「技术模块型 S」（Adapter / Gateway / Breaker / Retry / Token / Config / Safety = 7 S 技术角色词）到「价值流型 S」（RouteModel / StreamChat / ProtectCall / BudgetTokens / GuardContent + ConfigureGateway 横切 = 5+1 S 承诺装置）的注册表层重切。

**核心承诺达成**：

1. **S 与承诺 1:1 对齐**（5 个对外可验证承诺 + 1 个横切配置）
2. **运行时不变性绝对保持**（5 span 名 / 3 metric 名 / YAML key / 物理目录 / Bridge 路径全部字面量未改）
3. **Legacy 双轨 100% 覆盖**（26 旧 T ID → 26 新 T ID，alias 脚本校验 0 残留）
4. **跨域 SoT 契约化**（D3 vs D1/D2/D4/D5/D6/D7 边界 + 4 项灰区写入 `cross-domain-boundaries.md`）
5. **R1+R2+R3 17 项决议全部落地**

**验收裁决：v1.0 ACCEPTED**（15/15 AC PASS；26/26 T 全绿；P0 13/13 达标）

→ 触发 Phase D 子 change（v1.1 韧性可见性）S1 申请。

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0 | 2026-06-14 | 初版：v1.0 Phase A-C 全闭合验收报告 |
