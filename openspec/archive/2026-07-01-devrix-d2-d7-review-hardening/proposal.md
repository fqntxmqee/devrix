# Proposal: D2+D7 代码审查硬化

**Change ID:** `devrix-d2-d7-review-hardening`  
**Demand ID:** DM-20260630-013  
**Status:** Archived (S7) — PR #361 (P1-A + P1-B) + PR #362 (P2)  
**Created:** 2026-06-30  
**Archived:** 2026-07-01

---

## 1. Problem Statement

2026-06-30 D7+D2 架构审查发现 **14 条 P0/P1 + 10 条 P2** 问题（经源码核验后登记）。核心风险簇：

1. **安全 fail-open**：`edit_file` plan 绕过、symlink 路径穿越、sandbox/AST 降级放行
2. **并发隔离缺失**：全局单例 executor 可变 `Emit`；Wave `OnRelease` hook 累积
3. **压缩功能性断裂**：async autocompact 无写回闭环 → 静默上下文丢失
4. **错误吞没普遍**：`EnsureGoal`、`SetRoundPhase`、`resolve.go`、JSONL Load 等 `_ = err`
5. **规约债务**：ExpectedReturn 散文、战术 prompt 残留 Go 源码

## 2. Proposed Solution

分 **三阶段** 交付，每阶段独立 PR + 质量门：

| Phase | 主题 | 需求条目 | 预期 PR 规模 |
|-------|------|----------|-------------|
| **P0** | 安全 + 并发 Critical | RH-D7-01/02, RH-D2-01/02/03/04 | ≤400 行 × 2 PR |
| **P1** | 稳定性 + fail-closed + 错误可观测 | RH-D7-03~09/14, RH-D2-05~10 | ≤400 行 × 2–3 PR |
| **P2** | 规约清理 + 可维护性 backlog | RH-D7-10~13, RH-D2-11/12 | ≤400 行 × 1 PR |

### P0 技术要点

| 组件 | 方案 |
|------|------|
| **PerInvocationEmit** | `ItemPipelineRunner.Run(ctx, ..., emitFn)` 参数传递；移除共享 struct 字段写入 |
| **OnReleaseOnce** | `WorkerPool` 构造期注册一次；`dispatchLoop` 禁止重复 `OnRelease` |
| **PlanModeWriteParity** | `edit_file` 在 `resolveWorkspacePath` 后调用 `EnforcePlanModeWrite` |
| **SymlinkContainment** | `resolveWorkspacePath` → `EvalSymlinks` + realpath ⊆ workDir |
| **AutocompactWriteback** | 实现 `CompressionEventSink` 写回 `SessionContext`；或 `async.enabled=false` 直至闭环 |

## 3. Scope

### In Scope

- `openspec/changes/devrix-d2-d7-review-hardening/specs/` delta（D2 + D7）
- 24 条 T 测试点登记 + 实现
- 生产路径 silent swallow 清理（审查清单内）

### Out of Scope

- D7 `llmgateway` 具体包 → 接口注入重构
- God 文件物理拆分（仅 backlog 登记）
- 否定审查项（worker panic double-complete）

## 4. Success Criteria

- [ ] P0 五项 demand AC 全部 PASS
- [ ] `go test -race ./internal/layers/orchestration/... ./internal/layers/contextengine/...` 全绿
- [ ] `scripts/lint-d1-imports.sh` + layer-lint PASS
- [ ] t-registry 24 条 T 点 IMPLEMENTED
- [ ] spec.md lite-mode 契约段 + CHANGELOG 一行（S6 门禁）

## 5. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| PerInvocationEmit 改动面大 | Med | 先改 `Run` 签名 + bootstrap wire；保留 deprecated setter 一版 |
| Symlink 误杀合法 worktree | Med | 仅拒绝 realpath 逃逸；文档化 symlink 策略 |
| 禁用 async 影响长会话 | Low | feature flag + 指标 `autocompact_writeback_pending` |
| P1 范围膨胀 | Med | 严格按 phase PR；超 400 行拆 PR |

## 6. Phasing

```
P0-A (D2 security)     → edit_file + symlink + autocompact writeback
P0-B (D7 concurrency)  → PerInvocationEmit + OnReleaseOnce
P1-A (D7 errors)       → EnsureGoal + resolve + SetRoundPhase + TurnState
P1-B (D2 enforce)      → fail-closed sandbox + bashAST + audit redaction
P1-C (D2 compression)  → memory mutex + microcompact + ctx lifecycle
P2 (hygiene)           → ExpectedReturn schema + tactical prompt i18n + JSONL strict
```
