# Claude 博弈论分析 — DM-20260702-009

**日期:** 2026-07-02
**作者:** Claude (MiniMax-M3)
**立场:** 治本派 + 借鉴派平衡
**核心态度:** 2 个治本接受, 4 项 tech-debt 收口 OK, 但**默认全关的 GrowthBook + 借鉴 clawcode `inputsEquivalent` + LLM SideQuery auto-mode 是过度工程**, 需博弈裁剪

---

## 博弈点 1: per-input 函数 vs 字段化 metadata

**Claude 立场:** **per-input 函数 (clawcode 路线)**, 但要加 fail-safe 机制

- 字段化 (`ToolSpec.IsConcurrencySafePerInputMode enum`) 退化成"配置描述", 表达力不够 (bash 的 isReadOnly 是 command 解析问题, 不是配置问题)
- 函数化是唯一能处理 bash read-only detection / read_file size 判断 / write target path 互斥的方案
- **风险**: 抛错上抛 → 整个 turn 崩溃。AC6 fail-safe 强制要求 catch + false, 接受
- **额外建议**: 加 `IsConcurrencySafePanics` metric, 当 catch 到 panic 时记 1 次, **不要让 fail-safe 变成静默 bug 容器**

## 博弈点 2: auto-mode classifier 是否必要

**Claude 立场:** **P0 实施但默认关闭**, P1 之后根据实战数据决定是否打开

- VerifyContract 4 元组是 ground truth, **不可替换**
- auto-mode 是 **intermediate defense**, 在静态规则后、VerifyContract 前的"二次机会"
- 风险: LLM 幻觉, fail-open 默认 (5s timeout 后 allow) 不安全
- **关键设计**: SideQuery 必须用独立 LLM (避免主 LLM 自我审查), 5s timeout 硬上限, 默认 **OFF** (Production-Safety)
- **疑问**: 谁触发? 自动 (每个 tool_call)? 还是用户开关? 需求写"auto-mode", 但 P0 全关矛盾 — 需博弈

## 博弈点 3: tech-debt 收口策略

**Claude 立场:** **4 项一起收, 但 PR-F 单独立项**

- TD-STE-01 (混合批次并发) 跟 PR-B 强绑定, **必须 PR-B 一起**
- TD-STE-02 (Bash sibling abort) 单独 PR-F, 影响 Bash tool 语义
- TD-STE-03 (discard on fallback) 单独 PR-F, 依赖 TD-QL-03 已 CLOSED
- TD-STE-06 (ConcurrencySafe 注册表) 跟 PR-A 一起, 19 工具默认实现
- **理由**: PR-A/B 治本, PR-F 收债, 6 PR 划分清晰

## 博弈点 4: PR 拆分粒度

**Claude 立场:** **接受 6 PR, 但 PR-E 跟 PR-D 合并, 5 PR 即可**

- PR-D (classifier 集成) + PR-E (测试 + telemetry + e2e) **本质同一 PR**, 拆开会拉长回归期
- 5 PR 更紧凑: A interface / B partition / C ToAutoClassifier / D classifier+test+e2e / F GrowthBook+abort+discard+inputsEquivalent
- 反对意见: 6 PR 利于 review, 但 devrix 现状 (Hotfix 模式 + 用户验收) 5 PR 足够

## 博弈点 5: GrowthBook + inputsEquivalent 借鉴是否过度

**Claude 立场:** **GrowthBook 接受 (T25), inputsEquivalent (T28) 降级 P2 或 P3**

- GrowthBook runtime override 是 P0 (AC11), 但**默认全关 + Production-Safety** 的话, 实际是**死代码**。建议: GrowthBook **降 P2**, P0 阶段不接, 后续根据实际运营需要再启用
- `inputsEquivalent(a, b []byte) bool` (AC14 P2): 19 工具 × 3 case = 57 单测, **重复工作大, 价值小**。devrix ContentReplacementState (T04 已落地) 已经能感知内容变化, **inputsEquivalent 是它的弱化版**。建议**降 P3** 或直接删

## 借鉴 vs 保留创新 — 评估清单

| devrix 创新 | clawcode 借鉴 | Claude 评价 |
|------------|--------------|------------|
| EmissionClass 4 类路由 | 无 | ✅ 架构性, 保留 |
| VerifyContract 4 元组 | 无 (VerifyContract 是 devrix 独有) | ✅ 第一道安全, 保留 |
| MUPS 5 节点 | 无 | ✅ 保留 |
| Learn FeedbackMemory | 无 (clawcode 无 Learn 节点) | ✅ 保留 |
| LTL-Lite L4-L6 | 无 | ✅ 保留 |
| Token Design 2.0 | 无 | ✅ 保留 |
| ConvergenceContract 4 control plane | 无 | ✅ 保留 |
| IsConcurrencySafe 函数 | per-input 函数 | ✅ 接受 |
| partitionToolCalls | clawcode 真实做法 | ✅ 接受 |
| ToAutoClassifierInput | clawcode 真实做法 | ✅ 接受 |
| AutoModeClassifier | yoloClassifier | ✅ 接受, 默认关 |
| toCompactBlock JSONL | clawcode 真实做法 | ✅ 接受 |
| Bash sibling abort | clawcode 真实做法 | ✅ 接受 |
| StreamingToolExecutor.Discard() | 弱相关 (clawcode 无 discard) | ✅ 接受, TD-QL-03 已 CLOSED |
| **inputsEquivalent** | clawcode 35 字段 | ⚠️ 弱需求, 建议降 P3 |
| **GrowthBook override** | clawcode 无 (devrix 内部需求) | ⚠️ 默认全关是死代码, 建议降 P2 |

## 关键风险 (高 → 低)

1. **auto-mode classifier 默认关 = 死代码, 但 P0 强制实施** — 浪费工时
2. **Bash isReadOnly 误判** — 误把 `bash -c "ls; rm -rf"` 标并发, 缓解: 必须 parse 整个 command tree
3. **SideQuery LLM 不可用** — fail-open 不安全, 但 hard timeout 后 fail-open 是 clawcode 做法
4. **partitionToolCalls 改造破坏现有并发行为** — AC1 强制 19 工具默认保持 v2, 接受

## 共识诉求

- **三方共识**点: per-input 函数 + auto-mode classifier 默认关 + 6 PR 拆分 + 4 tech-debt 收口
- **争议**点: GrowthBook (P0 vs P2) + inputsEquivalent (P2 vs P3) + PR-D/E 是否合并 + auto-mode 谁触发
- **待用户裁决**: GrowthBook 降 P2? inputsEquivalent 删或降 P3? auto-mode 触发模式?

