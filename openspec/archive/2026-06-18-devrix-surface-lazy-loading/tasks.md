# Tasks: devrix-surface-lazy-loading

**Change ID:** devrix-surface-lazy-loading
**Demand ID:** DM-20260618-003

## W1: Contracts + static defer
- [ ] T26: ToolSpec.DeferLoading 字段 + ShouldDeferByDefault helper + 7 surface 填充
- [ ] ToolFilter.ShouldDefer(ctx, spec) 接口 + AcceptAllFilter default
- [ ] orthogonal_flags.go 加 ShouldDeferByDefault (delegate_*, *_background)
- [ ] BuiltinSurface.Tools() 填充 DeferLoading

## W2: PlanMode.ShouldDefer + PlanModeOpenWorldPolicy runtime defer
- [ ] T27: PlanModeOpenWorldPolicy.ShouldDefer (mode=plan_mode + OpenWorld + !allowlist → defer)
- [ ] Update PlanModeOpenWorldPolicy.ApplyWithContext 不变 (decisions 与 defer 分离)
- [ ] Tests: T27 cases (plan + openworld + allowlist match/not-match + non-plan + non-openworld)

## W3: ToolSearchSurface (新 surface)
- [ ] T28: surface.NewToolSearchSurface(allSpecs) constructor
- [ ] surface.ToolSearchSurface.Tools() 返回 tool_search 单一 spec
- [ ] surface.ToolSearchSurface.Execute(query, category) 搜索 + 返回
- [ ] surface.ToolSearchSurface.search (exact + glob + substring, top-5)
- [ ] Tests: T28 cases (exact match / glob / substring / category filter / no match)

## W4: turn_adapter.Prepare filter
- [ ] T29: Prepare 收集所有 ToolSpec → 过滤 DeferLoading=true (除 tool_search) → 过滤 ToolFilter.ShouldDefer → 下发
- [ ] Update Prepare tests (T29)
- [ ] Ensure ToolSearchSurface 已注入 a.surfaces (via BuildSurfaces)

## W5: zodgen (Go struct → JSON Schema subset)
- [ ] T30: zodgen.Schema(reflect.Type) → map[string]any
- [ ] 支持: type / properties / required / enum / description
- [ ] Tests: T30 cases (basic struct / with omitempty / with enum / nested)

## W6: anthropic stub + S4-Gate
- [ ] internal/layers/anthropic/anthropic.go stub (列出 TODO items, 无 impl)
- [ ] anthropic_test.go: 1 case (compile-only)
- [ ] go test -race ./... PASS
- [ ] go vet + gofmt + devrix-layer-lint --strict PASS

## W7: S5 验收 + S6 归档
- [ ] PR + auto-merge + delete-branch
- [ ] archive openspec/changes/devrix-surface-lazy-loading → openspec/archive/2026-06-18-devrix-surface-lazy-loading
- [ ] verify-archive.sh 全部 PASS