# Acceptance Report: devrix-d7-loop-first-routing

**Change ID:** devrix-d7-loop-first-routing
**Demand ID:** DM-20260616-002
**Result:** ACCEPTED
**Date:** 2026-06-16
**Reviewer:** devrix-team (Agent S5)

## Summary

Clawcode 对齐的 Loop-First 路由：默认 `routing_mode=loop_first`，ingress 仅 Skip/Command/Turn；Wave/Plan 由 Turn 内 `delegate_wave` / `enter_plan_mode` tool 门控；EngineEvent 单投递消除飞书重复回复。

## L5 验收

| L5 ID | 描述 | 验证方式 | 状态 |
|-------|------|----------|------|
| D7-S2-L5-01 | 问候 Turn 不触发 Wave | `d7_loop_first_test.go` + integration stub | ✅ PASS |
| D7-S2-L5-02 | delegate_wave 门控 Wave | `turn_tools_test.go` + `d7_loop_first_test.go` | ✅ PASS |
| D7-S2-L5-03 | Slash 命令零 LLM | `d7_orthogonal_dispatch_test.go` | ✅ PASS |
| D7-S2-L5-04 | EngineEvent 单投递 | unit tests + capture agent_route guard | ✅ PASS |
| D7-S2-L5-05 | enter_plan_mode tool | `turn_tools_test.go` | ✅ PASS |
| D7-S2-L5-06 | rule_orchestrate 回滚 | `routing_test.go` + legacy integration tests | ✅ PASS |

## 门禁验证

| 检查项 | 命令 | 结果 |
|--------|------|------|
| Coordinator 单元测试 | `go test -race ./internal/layers/orchestration/coordinator/...` | ✅ PASS |
| Capture / Turn / Bootstrap | `go test -race ./internal/layers/communication/capture/ ./internal/layers/orchestration/turn/ ./internal/bootstrap/` | ✅ PASS |
| D7 集成测试 | `go test -tags='integration d7' -race ./tests/integration/d7/...` | ✅ PASS |
| 构建 | `go build ./...` | ✅ PASS |

## 领域文档同步

| 文档 | 路径 | 状态 |
|------|------|------|
| spec.md | `openspec/specs/d7-orchestration/spec.md` v3.4.0 | ✅ 已同步 |
| a-registry.md | `openspec/specs/d7-orchestration/a-registry.md` v3.6.0 | ✅ 已同步 |
| t-registry.md | `openspec/specs/d7-orchestration/t-registry.md` v3.2.0 | ✅ L5-01..06 登记 |

## 真机备注 (L5-01)

飞书「你好」场景由 integration stub 覆盖（`TestIntegration_D7LoopFirst_GreetingNoWave`）；部署后可通过 `./scripts/devrix.sh restart` 真机复验。

## Conclusion

**ACCEPTED.** 全部 P0 L5 通过，测试全绿，领域文档已同步，准备 S7 归档。
