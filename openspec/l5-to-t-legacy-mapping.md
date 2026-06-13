# L5 → DSAFT T 遗留映射

**Status:** Active  
**Authority:** `openspec/t-registry.md`（T 层 SoT）  
**Migration script:** `scripts/migrate_l5_to_t.py`（2026-06-13）

> 本文件供 CI、文档交叉引用与历史 PR 追溯。新测试只使用 DSAFT T 编号。

## 注释格式

| 旧 | 新 |
|----|-----|
| `// Covers: L5-*` | `// T: D*-S*-A*-T*` |
| 行内 `L5-*` | 对应 `D*-S*-A*-T*` 或 `ORCH-S2-T*` |

## 自动规则（`L5-{D}-{S}-{NN}`）

与 `t-registry.md` Legacy 表一致：`D{X}-S{X}-T{NN}` 插入 A 段。特殊修正：

- `L5-4-12-01` → `D2-S12-A01-T01`（Worktree 在 D2）
- `L5-2-11-TD01` → `D2-S11-A01-TD01`
- `L5-2-11-TD03` → `D2-S11-A01-TD03`
- `L5-2-11-D6PR` → `D2-S11-A01-D6PR`

## EventBus（`L5-2-3-*` → D1-S9）

| L5 | T |
|----|---|
| L5-2-3-01 | D1-S9-A01-T01 |
| L5-2-3-02 | D1-S9-A02-T02 |
| L5-2-3-03 | D1-S9-A02-T03 |
| L5-2-3-04 | D1-S9-A02-T04 |
| L5-2-3-05 | D1-S9-A01-T05 |
| L5-2-3-06 | D1-S9-A01-T06 |
| L5-2-3-07 | D1-S9-A01-T07 |

## 跨域 / D0（`L5-0-0-*` → CROSS）

| L5 | T |
|----|---|
| L5-0-0-01 | CROSS-A01-T01 |
| L5-0-0-02 | CROSS-A02-T03 |
| L5-0-0-03 | CROSS-A01-T02 |
| L5-0-0-04 | CROSS-A01-T04 |
| L5-0-1-06 | D0-S1-A01-T06 |
| L5-0-1-07 | D0-S1-A01-T07 |

## D2 语义 ID（`L5-CTX-*` 节选）

| L5 | T | 模块 |
|----|---|------|
| L5-CTX-01/02 | D2-S3-A01-T01 | Memory |
| L5-CTX-03/04 | D2-S2-A01-T01/T02 | Compression |
| L5-CTX-05 | D2-S3-A01-T02 | Snapshot |
| L5-CTX-06 | D2-S1-A01-T01 | PEV |
| L5-CTX-34–42 | D2-S10-A01-T34–T42 | QueryLoop |
| L5-TOOL-01 | D2-S8-A01-T01 | Sandbox |
| L5-TOOL-03 | D2-S9-A03-T05 | ToolPool |

完整 `L5-CTX-*` / `L5-LLM-*` / `L5-OBS-*` / `L5-COMM-*` 见 `scripts/migrate_l5_to_t.py` 中 `SYMBOLIC` 字典。

## Orchestration（`L5-ORCH-*`）

`L5-ORCH-{NN}` → `ORCH-S2-T{NN}`（T01–T21 见 `t-registry.md` ORCH-S2 节）。

测试文件：`scheduler_orch_test.go`、`agent_tool_orch_test.go`（原 `*_l5_*`）。
