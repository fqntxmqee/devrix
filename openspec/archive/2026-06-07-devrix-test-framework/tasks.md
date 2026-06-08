# Tasks: devrix-test-framework

**Change ID:** devrix-test-framework
**Status:** Completed

---

## Milestone 1: 目录与代码迁移

- [x] **T1**: 创建 `tests/{testutil,integration,e2e,acceptance/p0}/`
  - L4: L4-BE-DEVRIX-TEST
  - L5: —

- [x] **T2**: 迁移 `integration_test.go` → `tests/integration/`
  - L4: L4-BE-DEVRIX-TEST
  - L5: L5-COMM-01, L5-COMM-03, L5-COMM-07

- [x] **T3**: 移除正式包中的 `mock_gateway.go`、`mock_feishu.go`
  - L4: L4-BE-DEVRIX-TEST
  - L5: —

- [x] **T4**: 添加 `scripts/test-{unit,integration,e2e,acceptance,all}.sh`
  - L4: L4-BE-DEVRIX-TEST
  - L5: —

## Milestone 2: 规范沉淀

- [x] **T5**: 编写 `openspec/specs/testing-framework/spec.md`
  - L4: L4-BE-DEVRIX-TEST
  - L5: —

- [x] **T6**: 编写 `openspec/l5-registry.md`
  - L4: L4-BE-DEVRIX-TEST
  - L5: L5-COMM-01 ~ L5-COMM-23

- [x] **T7**: 添加 Cursor 规则 `08-testing-framework.mdc`
  - L4: L4-BE-DEVRIX-TEST
  - L5: —

- [x] **T8**: 更新 `openspec/project.md` 测试章节
  - L4: L4-BE-DEVRIX-TEST
  - L5: —

## Milestone 3: CI 与 S5 自动化

- [x] **T9**: 添加 `.github/workflows/ci.yml`
  - L4: L4-BE-DEVRIX-TEST
  - Jobs: unit → gate (integration/e2e/acceptance) → coverage (≥80%)

- [x] **T10**: 实现 `scripts/gen-acceptance-report.sh`
  - L4: L4-BE-DEVRIX-TEST
  - L5: S5 全流程
  - 用法: `./scripts/gen-acceptance-report.sh --change {slug}`
