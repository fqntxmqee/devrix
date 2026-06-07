# Testing Framework Specification

**Capability:** testing-framework
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-07

---

## Overview

Devrix 测试框架将 OpenSpec L5 测试点、Go 测试金字塔与 S5 自动化验收串联为统一契约。
所有新增或变更 L3/L4 能力的开发 MUST 遵循本规范。

**关联文档：**

| 文档 | 路径 |
|------|------|
| L5 注册表 | `openspec/l5-registry.md` |
| 项目元数据 | `openspec/project.md` |
| 交付流程 | `specs/05-delivery-process.md`（工作区） |

---

## ADDED Requirements

### Requirement: Test Pyramid Layering

测试 MUST 按金字塔分层组织，不得将所有测试混入默认 `go test ./...` 路径。

**Priority**: P0
**Rationale**: 保证 PR 门禁快速（<2min），同时保留完整验收能力。

#### Scenario: Unit tests co-located with source

- GIVEN a new L4 function or method is implemented
- WHEN unit tests are written
- THEN test files MUST be named `{name}_test.go`
- AND MUST reside in the same package directory as the source file
- AND MUST NOT use build tags (default `go test ./internal/...` path)

#### Scenario: Integration tests separated

- GIVEN a test spans multiple packages or requires real file I/O orchestration
- WHEN the test is not a single-function unit test
- THEN it MUST be placed under `tests/integration/`
- AND MUST declare `//go:build integration` as the first line
- AND MUST be run via `./scripts/test-integration.sh`

#### Scenario: E2E smoke tests separated

- GIVEN a test validates config loading, build sanity, or cross-module startup
- WHEN the test is an end-to-end smoke check without external credentials
- THEN it MUST be placed under `tests/e2e/`
- AND MUST declare `//go:build smoke`
- AND MUST be run via `./scripts/test-e2e.sh`

#### Scenario: Acceptance tests separated

- GIVEN a test maps to an L5 P0 acceptance criterion
- WHEN the test validates a user-visible or contract-level behavior
- THEN it MUST be placed under `tests/acceptance/p0/` (or `p1/`, `p2/`)
- AND MUST declare `//go:build acceptance`
- AND MUST be run via `./scripts/test-acceptance.sh`

---

### Requirement: Directory Structure

项目 MUST 维持以下测试目录结构，不得随意新增顶层测试目录。

**Priority**: P0
**Rationale**: 统一发现路径，降低 CI 与 AI Agent 认知成本。

```
devrix/
├── internal/**/**/*_test.go       # 单元测试（同包）
├── tests/
│   ├── testutil/                  # 跨包共享 Mock / Fixture
│   ├── integration/               # //go:build integration
│   ├── e2e/                       # //go:build smoke
│   ├── acceptance/
│   │   ├── p0/                    # L5 P0 验收
│   │   ├── p1/                    # L5 P1
│   │   └── p2/                    # L5 P2
│   └── performance/               # //go:build performance（可选）
└── scripts/
    ├── test-unit.sh
    ├── test-integration.sh
    ├── test-e2e.sh
    ├── test-acceptance.sh
    └── test-all.sh
```

#### Scenario: New layer tests follow structure

- GIVEN a new architecture layer (e.g. contextengine) is introduced
- WHEN tests are added for that layer
- THEN unit tests go under `internal/layers/{layer}/`
- AND cross-package tests go under `tests/integration/` or `tests/acceptance/`
- AND MUST NOT create ad-hoc `internal/layers/{layer}/tests/` directories

---

### Requirement: Mock Placement Rules

Mock 代码 MUST NOT 出现在非 `*_test.go` 的正式源文件中。

**Priority**: P0
**Rationale**: 防止测试辅助代码污染生产 API 与二进制。

#### Scenario: Package-private mock for unit tests

- GIVEN a unit test needs to mock unexported symbols in the same package
- WHEN the mock is only used within that package's tests
- THEN the mock MUST be defined in `{name}_mock_test.go` in the same directory
- AND MUST use `package {same_package}` (not `{package}_test`)

#### Scenario: Cross-package mock in testutil

- GIVEN a mock is shared across integration or acceptance tests
- WHEN the mock implements a public interface from `internal/`
- THEN it MUST be placed in `tests/testutil/`
- AND MUST NOT create import cycles with `internal/` test files in the same package

#### Scenario: Forbidden mock in production

- GIVEN a developer creates a mock implementation
- WHEN the file is named `mock_*.go` without `_test.go` suffix
- THEN it MUST NOT be placed under `internal/`
- AND CI review MUST reject such placement

---

### Requirement: L5 Traceability

每个 L5 测试点 MUST 有可追溯的测试用例，测试代码 MUST 标注 L5 ID。

**Priority**: P0
**Rationale**: L5 是 OpenSpec S5 验收的确定性锚点。

#### Scenario: Register new L5 before implementation

- GIVEN a new L4 capability is planned in `tasks.md`
- WHEN development begins
- THEN a corresponding L5 ID MUST exist in `openspec/l5-registry.md`
- AND the L5 entry MUST include Priority, L4 mapping, and test file path

#### Scenario: Annotate test with L5 ID

- GIVEN a test case covers an L5 acceptance criterion
- WHEN the test function is written
- THEN it MUST include a comment `// Covers: L5-{LAYER}-{NN}` above the function
- AND P0 L5 tests MUST reside in `tests/acceptance/p0/`

#### Scenario: Acceptance test naming

- GIVEN an acceptance test maps to L5-COMM-04
- WHEN the test function is named
- THEN it SHOULD use the pattern `TestL5_{LAYER}_{NN}_{Behavior}`

---

### Requirement: Test Naming Conventions

测试函数命名 MUST 遵循以下约定。

**Priority**: P1
**Rationale**: 统一可读性与 grep 追溯能力。

| 层级 | 命名模式 | 示例 |
|------|----------|------|
| 单元测试 | `Test{Type}_{Method}_{Condition}` | `TestPermissionManager_Request_Timeout` |
| 集成测试 | `TestIntegration_{Flow}` | `TestIntegration_SessionExpiration` |
| 验收测试 | `TestL5_{DOMAIN}_{NN}_{Behavior}` | `TestL5_COMM_Commands_Parse` |
| 性能测试 | `Benchmark{Type}_{Method}` | `BenchmarkFileSessionStore_Create` |

---

### Requirement: Test Execution Gates

不同阶段的测试执行 MUST 使用指定脚本，不得绕过。

**Priority**: P0
**Rationale**: 保证本地与 CI 行为一致。

| 阶段 | 命令 | 超时预算 | 阻断级别 |
|------|------|----------|----------|
| 日常开发 / PR | `./scripts/test-unit.sh` | < 2min | MUST PASS |
| 合入前 | `./scripts/test-integration.sh` + `./scripts/test-e2e.sh` | < 10min | MUST PASS |
| S5 验收 | `./scripts/test-acceptance.sh` | < 5min | P0 MUST PASS |
| 全量 | `./scripts/test-all.sh` | < 15min | MUST PASS |

#### Scenario: Developer completes a feature task

- GIVEN a task in `tasks.md` is marked complete
- WHEN the developer submits for review
- THEN `./scripts/test-unit.sh` MUST pass
- AND affected L5 tests MUST pass via `./scripts/test-acceptance.sh`
- AND `tasks.md` MUST list associated L5 IDs

#### Scenario: Slow tests use short timeouts in unit path

- GIVEN a test involves waiting (e.g. permission timeout)
- WHEN the wait exceeds 1 second
- THEN the test MUST NOT run in the default unit path
- AND MUST use an explicit short timeout config or be moved to `tests/integration/`

---

### Requirement: Coverage Threshold

代码覆盖率 MUST 达到项目定义的最低阈值。

**Priority**: P1
**Rationale**: 与 `project.md` 质量目标一致。

#### Scenario: Merge gate coverage check

- GIVEN a PR is ready to merge
- WHEN CI runs coverage analysis
- THEN coverage report MUST be generated and uploaded
- AND current CI minimum is 20% (baseline); target MUST reach ≥ 80% per `project.md`
- AND any new L4 package without tests MUST be rejected

#### Scenario: S5 acceptance report generation

- GIVEN a change reaches S5 acceptance
- WHEN `./scripts/gen-acceptance-report.sh --change {slug}` is executed
- THEN `acceptance-report.md` MUST be written under `openspec/changes/{slug}/`
- AND P0 L5 results MUST be included from `openspec/l5-registry.md`
- AND exit code MUST be 0 only when verdict is ACCEPTED

---

### Requirement: OpenSpec S5 Acceptance Integration

功能变更的 S5 验收 MUST 基于 L5 测试点生成 `acceptance-report.md`。

**Priority**: P0
**Rationale**: 对齐工作区交付管线 S5 阶段。

#### Scenario: Feature change acceptance

- GIVEN a change in `openspec/changes/{slug}/` reaches S5
- WHEN acceptance is executed
- THEN each P0 L5 MUST have PASS status with evidence (test name or CI link)
- AND `acceptance-report.md` MUST be generated per `specs/05-delivery-process.md` §9.2
- AND overall verdict MUST be ACCEPTED before S6 delivery

---

### Requirement: Build Tags Reference

Build tags MUST 使用以下标准值，不得自定义未登记的 tag。

**Priority**: P1
**Rationale**: 避免 CI 与脚本分叉。

| Tag | 目录 | 用途 |
|-----|------|------|
| `integration` | `tests/integration/` | 跨包集成 |
| `smoke` | `tests/e2e/` | 冒烟 |
| `acceptance` | `tests/acceptance/` | L5 验收 |
| `performance` | `tests/performance/` | 基准（可选） |
| `live` | `tests/acceptance/live/` | 需外部凭证（不阻断 PR） |

---

## MODIFIED Requirements

(None)

---

## REMOVED Requirements

(None)
