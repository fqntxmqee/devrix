# S3-Gate Review: devrix-diagnostic-tools-wiring (DM-20260617-002)

**Reviewer:** self-review (solo maintainer, no second reviewer available)
**Review Date:** 2026-06-17
**Standard:** `openspec/specs/project/review-design.md` §5

---

## 1. 检查清单对照

| 检查项 | 状态 | 证据 |
|--------|------|------|
| 层归属和接口方向正确 | ✅ PASS | 接入层全部在 `internal/bootstrap/`、`internal/cli/`、tool runner；不动 `internal/layers/*` library |
| 不重复现有能力 | ✅ PASS | 复用 DM-016 14 个 Activity 节点；不新增 T 编号 |
| demand → proposal → design → specs 追溯链完整 | ✅ PASS | DM-017-002 → proposal §9 → design §0 → spec 验收对照表 |
| 所有 P0 验收标准有对应 Scenario | ✅ PASS | spec.md §验收对照: AC1-AC12 全部覆盖 |
| Happy path 和 sad path 均有 Scenario | ✅ PASS | 22 Scenarios 中含 ~8 sad path (disabled / fail / empty / 限制) |
| 回归风险已评估 | ✅ PASS | design.md §6 表格 9 项 |
| Grill Review 结论已记录 | ✅ PASS | design.md §0 |
| Review 结论明确 | ✅ **Approved with Suggestions** | 见 §2 |

---

## 2. 决议

**结论：Approved with Suggestions**

设计可进入 S4 实现。

### 2.1 Suggestions（非强制，S4 实现时评估）

| ID | Suggestion | 评估 |
|----|-----------|------|
| S-1 | `toolrunner/{verify,freefork,tracker}_tool.go` 引用 `evolution/verify`、`multiagent/freefork`、`observability/tracker` 触发 layer-lint 跨域警告 | S4 阶段实测 layer-lint；若 FAIL，迁移到 `internal/bridges/diagnostics/`（design.md §6.2 已预案） |
| S-2 | A5 WindowAnalyzer 是否同时暴露 LLM tool 还是仅 L2 CLI？ | 设计默认仅 L2（CLI）。S4 实现时若发现 LLM 频繁请求此工具，补一个 L1 暴露 |
| S-3 | tracker tick 频率 1s 写死 | S4 实现时改为可配置（DiagnosticsConfig.TrackerTickIntervalMs，默认 1000ms） |
| S-4 | A4 FaultInject 仍仅 testbuild，是否值得加 IM 注入入口？ | **本次不实现**（AC13 P2 锁定）。若后续有需求，再开 change |
| S-5 | G3 Notify consume 注入到 `assembler.go` 是否会污染 system prompt？ | 单测验证 reminder 段不进入 system prompt 主体；上限 5 events 避免长度爆炸 |

### 2.2 不通过的项（无）

无 Changes Requested / Rejected 项。

### 2.3 S3-Gate 后续动作

- [x] Grill Review 结论已记录在 design.md §0
- [x] 本 review-design.md 已写入 openspec/changes/devrix-diagnostic-tools-wiring/
- [ ] 进入 S4：tasks.md + 代码改动 + 单测
- [ ] S4 完成时运行 `go test -race ./...`、`go vet ./...`、`go build ./...`
- [ ] S4-Gate 用 code-reviewer agent 自检（替代人工 Review）

---

## 3. Reviewer 备注

本次为单人团队的 S3-Gate 自审。需求-设计-规格链路已自检，22 个 Scenarios 覆盖 P0 AC 全部，4 项 Grill Decision 已 Agreed。

设计核心是把"library 实现"与"运行期可达"两个概念区分清楚。13 项 library 在 DM-016 已就绪，本 change 只补 6 Level 接入点。

最关键的边界：**不动 DM-016 library 文件**。所有 wiring 通过 `NewXxx()` 工厂 + `SetGlobalXxx()` 单例注入。

---

**Reviewer Sign-off:** 2026-06-17
**Status:** APPROVED — proceed to S4 implementation