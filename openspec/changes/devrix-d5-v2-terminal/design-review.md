# S3-Gate Design Review: D5 v2.1 Terminal

**Change ID:** devrix-d5-v2-terminal
**Demand ID:** DM-20260619-006
**Review Type:** S3-Gate Grill Review（review-design.md §3.1）
**Review Date:** 2026-06-19
**Phase A PR:** [#118](https://github.com/fqntxmqee/devrix/pull/118) — `docs/d5-v2-terminal-spec`
**结论:** **Approved** — 可进入 S4（前提：Owner 确认本 Review）

---

## 1. 审查范围

### Phase A 实际产出（PR #118 已提交）

| 类别 | 文件 | 路径 | 版本 |
|------|------|------|------|
| **新建** | d5-domain.md | `openspec/specs/d5-observability/` | v1.1.0 |
| **新建** | d5-boundary.md | 同上 | v1.0.0 |
| **新建** | observability-guide.md | 同上 | v1.0.0 |
| **新建** | terminal-state-guide.md | 同上 | v1.0.0 |
| **新建** | dsaft-architecture.md | 同上 | v1.0.0 |
| **升级** | spec.md | 同上 | v2.0.0 → **v3.0.0** |
| **升级** | design.md | 同上 | v2.0.0 → **v3.0.0** |
| **升级** | a-registry.md | 同上 | v3.0.0 → **v4.0.0** ⬅️ 代码锚点 |
| **升级** | f-registry.md | 同上 | v2.0.0 → **v3.0.0** |
| **升级** | t-registry.md | 同上 | v3.1.0 → **v3.2.0** ⬅️ 代码锚点 |
| **升级** | coverage.md | 同上 | — |
| **升级** | span-registry.md | 同上 | v2.0.0 → **v3.0.0** |
| **升级** | layer-delta.md | 同上 | v3.0.0 → **v4.0.0** |
| **升级** | cross-domain-boundaries.md | `openspec/specs/architecture/` | v1.6.0 → **v1.7.0** |
| **升级** | code-layout.md | 同上 | v1.11.0 → **v1.12.0** |
| **升级** | code-map.md | `docs/architecture/` | — |
| **参考** | demand.md, design.md, tasks.md, gaming-analysis.md | `openspec/changes/devrix-d5-v2-terminal/` | 已同步 |

**总计：** 5 新建 + 11 升级 + 4 change 参考 = 20 文件

---

## 2. 架构决策审查（review-design.md §2.1）

| # | Decision | 选择 | 替代方案 | 落盘位置 | 结论 |
|---|---------|------|---------|----------|------|
| Q1 | 重构范围 | **C** — 文档终态 + S23 + bridge 删除 | A: 仅文档 / B: 全部重做 | `demand.md` §2 | **Agreed** |
| Q2 | S23 是否新 S25 | **否** — C3a–C3e 子承诺 | 新增 S25 | `d5-domain.md` §S23 子承诺 | **Agreed** |
| Q3 | DebugFilter 归属 | **S21**（A14 FilterDebugLog） | S23 / S0 | `a-registry.md` v4.0 S21-A14 | **Agreed** |
| Q4 | SessionBridge 归属 | **S0**（A03 TrackActiveSessions） | S23 / S21 | `a-registry.md` v4.0 S0-A03 | **Agreed** |
| Q5 | bridge 删除时机 | **本轮**（Phase B2a+B2b） | v2.2 推迟 | `design.md` §8 | **Agreed** |
| Q6 | North Star | d5-domain.md §North Star | — | `d5-domain.md` | **Agreed** |
| G10 | Phase A 代码锚点 | **≥1**（a-registry + t-registry 双锚点） | 0（纯文档） | `a-registry.md` v4.0 + `t-registry.md` v3.2 | **Agreed** |
| G11 | B2a↔B2b 之间不留 shim | **不留 release 间隔** | 留一个 release shim | `design.md` §8 | **Agreed** |

**结论：8/8 Agreed，无 Revised 或 Deferred。**

---

## 3. 需求完整性审查（review-design.md §2.2）

### 追溯链：demand → proposal → design → specs

| 环节 | 文件 | 状态 |
|------|------|------|
| S1 需求 | `demand.md` | ✅ DM-20260619-006，P0 优先级 |
| S2 提案 | `proposal.md`（内嵌于 design.md §2） | ✅ 六项 Owner 决议 |
| S3 设计 | `design.md` v3.0（specs 目录） | ✅ 10 节完整 |
| S3 specs | 16 个 spec 文件 | ✅ 全部升级到 v2.1 Terminal |

### AC 逐条覆盖（Phase A 9 条 + Phase B 8 条）

#### Phase A — 规格终态

| AC | 描述 | 优先级 | 覆盖文件 | 状态 |
|----|------|--------|----------|------|
| AC-A1 | d5-domain.md（Tl;DR + North Star + 完备性边界 + 博弈论玩家表 + 时间属性矩阵 + S23 子承诺 + S25 触发条件 + 举证责任 + Terminal 冻结 + Bridge 时间线 + 阅读优先级） | P0 | `d5-domain.md` v1.1.0 | ✅ |
| AC-A2 | spec.md v3.0（S21–S24 主表；D7 Turn 主路径；query.loop 仅 RETIRED） | P0 | `spec.md` v3.0.0 | ✅ |
| AC-A3 | observability-guide.md（D7 Turn Trace 树 + Span↔T P0 矩阵 + Runbook + Coverage 多维指标 + WARN metric 聚合 + 成功指标双轨） | P0 | `observability-guide.md` v1.0.0 | ✅ |
| AC-A4 | terminal-state-guide.md + dsaft-architecture.md Stub | P1 | 两文件均已创建 | ✅ |
| AC-A5 | d5-boundary.md + cross-domain-boundaries.md D5 段（含 D5→D6 证据移交规则） | P0 | `d5-boundary.md` v1.0.0 + `cross-domain-boundaries.md` v1.7.0 §4 | ✅ |
| AC-A6 | a-registry v4.0 + f-registry v3.0 + design.md v3.0（含 S23 硬边界 + Bridge 拆步 + legacy_harness 退役） | P0 | 三文件均已升级 | ✅ |
| AC-A7 | code-layout.md §4.6 diagnose 子目录完整；grep query.loop 仅 Legacy/RETIRED | P0 | `code-layout.md` v1.12.0；`grep` 验证通过 | ✅ |
| AC-A8 | **Phase A ≥1 代码锚点**（a-registry Code Location 或 t-registry canonical 列） | P0 | **2 个锚点**：a-registry v4.0 + t-registry v3.2 | ✅ |
| AC-A9 | 所有新建 spec 含阅读优先级标注（MUST/SHOULD/REFERENCE） | P1 | `d5-domain.md` §规格文档索引 | ✅ |

#### Phase B — 代码清债（S4 阶段验证，本 Review 仅确认设计）

| AC | 描述 | 优先级 | 设计落盘位置 | 设计状态 |
|----|------|--------|-------------|----------|
| AC-B1 | B2a: bridge.go import 改 instrument/* 直连 | P0 | `design.md` §8 + `tasks.md` Phase B2a | ✅ 设计就绪 |
| AC-B2 | B2b: 删除 9 bridge 包，grep 旧路径 = 0 | P0 | `design.md` §8 + §9 CI 规则 | ✅ 设计就绪 |
| AC-B3 | CI bridge 防回归规则 | P0 | `design.md` §9 | ✅ 设计就绪 |
| AC-B4 | genai_tokens.go → instrument/metrics/；llm_log.go → diagnose/incident/ | P1 | `tasks.md` Phase B1 | ✅ 设计就绪 |
| AC-B5 | t-registry canonical_s/a 校正 + 3 PLANNED T 闭合 | P1 | `t-registry.md` v3.2（Phase A 已部分完成） | ✅ T 层已校正 |
| AC-B6 | 41/41 T IMPLEMENTED | P0 | `t-registry.md` v3.2 Statistics | ✅ Phase A 已达 41/41 |
| AC-B7 | go test -race + layer-lint + obs integration 全绿 | P0 | `tasks.md` Phase C1 | ✅ 验收项明确 |
| AC-B8 | legacy_harness DEPRECATED + 退役计划 | P1 | `design.md` §10 + `f-registry.md` | ✅ 设计就绪 |

**AC 覆盖结论：17/17 AC 有明确落盘。Phase A 9 条全部在 PR #118 中实现。Phase B 8 条设计已就绪，S4 执行。**

---

## 4. 规格质量审查（review-design.md §2.3）

| 检查项 | 结果 | 证据 |
|--------|------|------|
| Gherkin 格式正确 | ✅ | `spec.md` v3.0 含 GIVEN/WHEN/THEN Scenario |
| Happy path + sad path | ✅ | Graceful Degradation、Doctor fail、Incident export fail |
| 并发场景覆盖 | ✅ | RecordHit 100 goroutine（D5-S23-A01-T03） |
| 错误路径覆盖 | ✅ | unknown operation WARN、exporter 失败 degraded |
| T 层映射完整 | ✅ | `t-registry.md` v3.2：41 T + canonical_s + canonical_a 全量填充 |
| 跨域契约明确 | ✅ | `d5-boundary.md` + `cross-domain-boundaries.md` §4 |

### query.loop 退役验证（AC-A7 Gate）

```bash
$ grep -r "query.loop" openspec/specs/d5-observability/ --include="*.md" | grep -v "RETIRED\|REMOVED\|Legacy\|legacy\|退役\|追溯"
# 预期：0 命中（除 RETIRED/Legacy 节外无活跃引用）
```

**关键文件检查：**
- `spec.md`: query.loop 仅在 RETIRED 节 + "REMOVED" 标注
- `design.md`: 主路径为 D7 Turn；query.loop 仅 history 提及
- `coverage.md`: 已退役 Operation 列表含 query.loop.*
- `span-registry.md`: query.loop 标 ~~strikethrough~~ RETIRED
- `layer-delta.md`: query.loop 在 REMOVED/RETIRED 节

---

## 5. 风险审查（review-design.md §2.4）

| 风险 | 等级 | 缓解 | 落盘位置 |
|------|------|------|----------|
| 文档与代码短期不一致 | 低 | Phase A 先合并；spec 即权威 | PR #118 docs-only |
| bridge 删除编译破坏 | 中 | B2a→B2b 两步；grep + CI 防回归 | `design.md` §8–§9 |
| S23 子承诺继续膨胀 | 低 | 硬边界 ≤7 + S25 触发条件 + 举证责任 | `d5-domain.md` §S23 硬边界 |
| 博弈论对焦改决议 | 低 | 20 共识项已落盘；Decision 在 design.md | `gaming-analysis.md` §20 |
| 回退风险（bridge 删除后） | 低 | 序贯级联先例（D6→D7→D5）；CI 防回归规则；git revert 可逆 | `design.md` §8 |

**回滚方案：**
- Phase A: docs-only，git revert 即可
- Phase B2a: git revert（未删包，安全）
- Phase B2b: git revert 恢复 9 bridge 包（文件在 git history 中）

---

## 6. Grill Review 记录（review-design.md §3.1）

### 6.1 逐决策遍历

| 决策点 | 问"为什么选 A 不选 B？" | 结论 |
|--------|------------------------|------|
| 范围 C vs A/B | B（全部重做）风险高且 v2.0 物理迁移已完成，重做无边际收益 | **Agreed** |
| S23 子承诺 vs 新 S | 新 S 会增加 S 号段碎片化，C3a–C3e 语义已清晰 | **Agreed** |
| DebugFilter → S21 | DebugFilter 是 logger handler chain，属遥测生成非诊断 | **Agreed** |
| SessionBridge → S0 | SessionBridge 是横切集成（Facade），非诊断 | **Agreed** |
| bridge 本轮删 vs 推迟 | D6 已验证安全；推迟 = 道德风险（永不移） | **Agreed** |
| 不留 shim | shim = B2b 被无限期推迟的期权，违反"burn the ships" | **Agreed** |
| 代码锚点 ≥1 | v1.0 纯文档承诺强度为零（cheap talk 教训） | **Agreed** |
| T ID 不变 | T 层宪法：41 T ID 字符串不可 renumber | **Agreed** |

### 6.2 逐依赖确认

| 依赖 | Change ID | 状态 |
|------|-----------|------|
| D6 bridge 删除先例 | v2.0.1 | ✅ 已完成 |
| D7 v2 Structure 终态模式参考 | DM-20260619-005 | ✅ 已完成 |
| D2 v2.2 closure | DM-20260619-007 | ✅ 平行级联（已完成） |
| QueryLoop REMOVED | DM-20260618-010 | ✅ 代码已删 |
| 诊断工具链 | DM-20260616~18 | ✅ IMPLEMENTED |
| v1.0 Registry | DM-20260615-001 | ✅ ACCEPTED |
| v2.0 物理迁移 | DM-20260615-003 | ✅ ACCEPTED |

### 6.3 逐 Scenario 推演

**Scenario 1: 新人读 D5 文档**

```
入口: d5-domain.md (MUST) → spec.md (MUST) → d5-boundary.md (MUST)
  → observability-guide.md (SHOULD) → design.md (REFERENCE)
```

验证: 阅读优先级标注完整（`d5-domain.md` §规格文档索引）。✅

**Scenario 2: SRE on-call 排障**

```
报警 → observability-guide.md Runbook → D7 Turn Trace 树定位
  → Span↔T P0 矩阵 → Jaeger trace_id 交叉验证
```

验证: `observability-guide.md` 含 Runbook + Span↔T P0 矩阵。✅

**Scenario 3: 开发者新增 Span**

```
names.go 加 Op* → registry.go 加 OperationMeta → registry_test.go 加 expected
  → coverage.md §新增 Operation 流程
```

验证: `coverage.md` + `spec.md` §Operation Registry 流程完整。✅

**Scenario 4: Phase B 执行 bridge 删除**

```
B2a: bridge.go import 改 instrument/* → go build 通过
B2b: rm -rf 9 bridge 目录 → grep 0 命中 → CI 防回归规则激活
```

验证: `design.md` §8 拆步策略 + §9 CI 规则。✅

---

## 7. 代码锚点验证（AC-A8）

### 锚点 1: a-registry.md v4.0 — Code Location 校正

| 检查项 | 结果 |
|--------|------|
| 32 Activity 全部有 Code Location | ✅ |
| 旧 bridge 路径（tracer/tracer.go → instrument/tracer/tracer.go） | ✅ 全量校正 |
| 新增 A14/A03/A07/A09/A10 有 Code Location | ✅ |
| Code Location 路径与实际文件存在一致 | ✅（已验证 `find` 输出） |

**示例校正：**
- `tracer/tracer.go` → `instrument/tracer/tracer.go`
- `metrics/meter.go` → `instrument/metrics/meter.go`
- `exporter/console.go` → `export/console.go`
- `coverage/coverage.go` → `diagnose/coverage/coverage.go`
- `runtime/path_resolver.go` → `configure/runtime/path_resolver.go`

### 锚点 2: t-registry.md v3.2 — canonical_s/a 校正

| 检查项 | 结果 |
|--------|------|
| canonical_a 列全量填充（43 T） | ✅ |
| canonical_s 校正 A08→S21（DebugFilter） | ✅ D5-S23-A08-T01/T02 |
| canonical_s 校正 A06→S0（SessionBridge） | ✅ D5-S23-A06-T01 |
| canonical_a 校正 Doctor T→A10 | ✅ D5-S23-A03-T01/T02 |
| canonical_a 校正 DebugFilter T→A14 | ✅ D5-S23-A08-T01/T02 |
| canonical_a 校正 SessionBridge T→A03 | ✅ D5-S23-A06-T01 |
| 3 PLANNED T 全部闭合 → 41/41 IMPLEMENTED | ✅ |

---

## 8. 检查清单（review-design.md §5）

- [x] 层归属和接口方向正确 — D5 公共域，Bridge 注入，无反向依赖
- [x] 不重复现有能力 — bridge 删除复用 instrument 包
- [x] demand → proposal → design → specs 追溯链完整
- [x] 所有 P0 验收标准有对应 Scenario — 17 AC 全覆盖
- [x] Happy path 和 sad path 均有 Scenario — Graceful Degradation 路径
- [x] 回归风险已评估 — §5 风险表 + 回滚方案
- [x] Grill Review 结论已记录 — §6 逐决策/逐依赖/逐 Scenario
- [x] Review 结论明确 — **Approved**（见 §10）

---

## 9. Suggestions（非阻塞）

1. **SUG-1 [Low]:** `observability-guide.md` Runbook 的 on-call 排障动线可在首次真实 incident 后迭代优化
2. **SUG-2 [Low]:** `d5-domain.md` §时间属性矩阵中"事后×强承诺"的 Incident export 报警阈值待 SRE 确认
3. **SUG-3 [Low]:** Phase B 启动前建议再跑一次 `grep query.loop` 确认代码侧也已清理（文档侧已确认）
4. **SUG-4 [Info]:** `gaming-analysis.md` OQ-1（S23 子承诺上限 7）的 Schelling 点选择可在 3 个月后复盘实际子承诺数

---

## 10. Gate Verdict

| 维度 | 评估 |
|------|------|
| 设计完整性 | **Pass** — 5 新建 + 11 升级 = 16 spec 文件，DSAFT 五层全绿 |
| 决策可辩护性 | **Pass** — 8/8 Decision Agreed，每项有替代方案分析 |
| 需求覆盖度 | **Pass** — 17/17 AC 落盘（9 Phase A + 8 Phase B 设计就绪） |
| 代码锚点 | **Pass** — 双锚点（a-registry v4.0 + t-registry v3.2），超过 ≥1 要求 |
| 规格质量 | **Pass** — Gherkin 完整，T 层映射 41/41，跨域契约明确 |
| 风险评估 | **Pass** — 5 风险 + 缓解 + 回滚方案 |
| **总体** | **Approved** |

---

## 11. S3-Gate Sign-off

| 角色 | 状态 | 日期 | 备注 |
|------|------|------|------|
| **设计作者** | Submitted | 2026-06-19 | Phase A 完成，PR #118 已提交 |
| **S3-Gate Reviewer** | Approved | 2026-06-19 | 本 Review — 无阻塞问题 |
| **Owner（最终决策）** | ✅ **Confirmed** | 2026-06-19 | PR #118 解除 Draft → Auto-merge |

---

## 12. 后续行动

| 步骤 | 行动 | 阻塞项 |
|------|------|--------|
| 1 | ✅ Owner 确认本 Review | 2026-06-19 |
| 2 | 🔄 PR #118 解除 Draft → Auto-merge | 执行中 |
| 3 | `grep query.loop` 代码侧最终确认 | 步骤 2 |
| 4 | **Phase B 启动**（B2a → B2b → B1 → B3） | Owner 指令 |
| 5 | Phase C 验收归档 | Phase B 完成 |
