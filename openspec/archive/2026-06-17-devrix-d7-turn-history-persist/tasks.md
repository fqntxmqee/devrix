# Tasks: devrix-d7-turn-history-persist

**Change ID:** devrix-d7-turn-history-persist
**Demand ID:** DM-20260617-003
**Status:** DONE (hotfix 路径 — S1-S3 ceremony 跳过 per 用户指令 2026-06-17; S4-S6 文档 + S6 归档在 follow-up PR 闭环)

| Task | L4/L5 | T 点 | 状态 | 备注 |
|------|-------|------|------|------|
| `ContextEngine.AppendAndTrimMessages` 公开 API | L3-BE-CTX-01 | D2-S3-A02-T01/T02 | DONE | `internal/layers/contextengine/engine.go:AppendAndTrimMessages`, lazy-init + append + trim |
| `turn_adapter.PersistTurn` 修复 (从 stub → AppendAndTrimMessages) | L3-BE-CTX-01 | D7-S5-A04-T01 | DONE | `internal/bootstrap/turn_adapter.go:PersistTurn` (commit `133ea2b`) |
| `engine_persist_bridge_test.go` 单测 (5 case) | L5-6-1-09 | D2-S3-A02-T01/T02 | DONE | empty/existing/fresh/trim/race-safety |
| `turn_adapter_persist_test.go` 单测 (4 case) | L5-6-1-09 | D7-S5-A04-T01 | DONE | writes-to-D2/full-round/nil-engine/append-error |
| `tests/integration/d7/turn_history_persist_test.go` (3-turn E2E) | L5-6-2-03 | D7-S5-A04-T02 | DONE | 3 轮 PersistTurn → Prepare 返回全历史 |
| D7 t-registry 新增 T01/T02 | — | D7-S5-A04-T01/T02 | DONE | `openspec/specs/d7-orchestration/t-registry.md` |
| D2 t-registry 新增 T01/T02 | — | D2-S3-A02-T01/T02 | DONE | `openspec/specs/d2-context-engine/t-registry.md` |
| `go test -race ./...` 全绿 | — | — | DONE | PR #62 + PR #64 合并前 CI 绿 |
| 飞书 IM E2E 3 轮历史引用 | — | AC1 | DONE | 用户在 PR #62 验收后确认 |

## 提交记录 (hotfix 路径)

| Commit | 说明 | 关联 |
|--------|------|------|
| `133ea2b` | fix(d7): persist turn history into D2 memory (DM-20260617-003) | PR #62 |
| `1019e89` | fix(d7): inject SessionContext into tool ctx + expose free_fork schema (DM-20260617-004, related) | PR #62 |
| `6e913c4` | fix(d7): D7 tool pipeline hardening (DM-20260617-003+005+006) (#62) | PR #62 (squash merge) |
| `bbb7178` | docs(d7): DM-20260617-003 devrix-d7-turn-history-persist S6 archive (proposal/design/demand/.openspec.yaml + index + t-registry) | PR #62 (squash merge) |
| `a4581ce` | feat(tool-surface): delete 5 remaining global singletons (DM-20260617-008) (#64) | PR #64 (squash merge, 含本 change 的 sub-commit 引用) |

## P0 覆盖

- D2-S3-A02-T01 — AppendAndTrimMessages 写入 D2 内存 + budget 裁剪 ✓
- D2-S3-A02-T02 — AppendAndTrimMessages lazy-init 不存在的 sid ✓
- D7-S5-A04-T01 — turn_adapter.PersistTurn 提交 req.Messages 到 D2 内存 ✓
- D7-S5-A04-T02 — 三轮同 session 连续 PersistTurn → Prepare 返回全历史 ✓

4/4 P0 T 点全部 IMPLEMENTED, t-registry 已登记。
