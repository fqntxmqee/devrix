# Design: Devrix Testing Framework

**Change ID:** devrix-test-framework
**Demand ID:** DM-20260607-008
**Status:** Delivered

---

## 1. 架构决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 单元测试位置 | 同包 `*_test.go` | Go 惯例，PR 门禁快速 |
| 集成/E2E/验收 | `tests/` + build tag | 与默认 `go test ./internal/...` 隔离 |
| Mock 策略 | 禁止正式包 `mock_*.go` | 避免生产包污染 |
| L5 追溯 | `openspec/l5-registry.md` + `// Covers: L5-*` | 支撑 S5 自动化验收 |
| SoT | `openspec/specs/testing-framework/spec.md` | 后续变更必须遵守 |

## 2. 目录结构

```
devrix/
├── internal/**/*_test.go          # 单元测试（无 build tag）
├── tests/
│   ├── testutil/                  # 跨包 Mock / 辅助
│   ├── integration/               # -tags=integration
│   ├── e2e/                       # -tags=e2e
│   └── acceptance/p0/             # -tags=acceptance
└── scripts/
    ├── test-unit.sh
    ├── test-integration.sh
    ├── test-e2e.sh
    ├── test-acceptance.sh
    ├── test-all.sh
    └── gen-acceptance-report.sh
```

## 3. CI 门禁

`.github/workflows/ci.yml` 流水线：unit → integration/e2e/acceptance gate → coverage (≥80%)。

## 4. 关联规范

归档后 SoT 保留在 `openspec/specs/testing-framework/spec.md`；本归档包内 `specs/testing-framework/spec.md` 为交付快照。
