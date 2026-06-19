# D5 Observability Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Draft — devrix-d5-v2-terminal（S3）
**Version:** 3.0.0
**Last Updated:** 2026-06-19
**Parent:** `a-registry.md` v4.0（本 change 草案）
**Change:** devrix-d5-v2-terminal — 增 `canonical_s` 列 + 路径更新 + 诊断 F

> S7 归档后替换 `openspec/specs/d5-observability/f-registry.md`。

---

## Overview

- **Legacy F ID**（D5-S1-A01-F01 等）冻结追溯，不 renumber
- **canonical_s** 列指向 Canonical S21–S24
- **Code Location** 使用 v2.0 物理路径

---

## D5-S21-A01 CreateSpan（canonical_s: S21）

| F ID | Name | canonical_s | Code Location |
|------|------|-------------|---------------|
| D5-S1-A01-F01 | StartSpan | S21 | `instrument/tracer/tracer.go` (Start) |
| D5-S1-A01-F02 | ApplySampler | S21 | `instrument/tracer/sampler.go` |
| D5-S1-A01-F03 | WarnUnknownOp | S21 | `instrument/tracer/tracer.go` |

## D5-S21-A02 EndSpan

| F ID | Name | canonical_s | Code Location |
|------|------|-------------|---------------|
| D5-S1-A02-F01 | FinalizeSpan | S21 | `instrument/tracer/span.go` |
| D5-S1-A02-F02 | ExportSpan | S21 | `instrument/tracer/export.go` |

## D5-S21-A03 PropagateContext

| F ID | Name | canonical_s | Code Location |
|------|------|-------------|---------------|
| D5-S1-A03-F01 | InjectTraceparent | S21 | `instrument/tracer/propagation.go` |
| D5-S1-A03-F02 | ExtractTraceparent | S21 | `instrument/tracer/propagation.go` |
| D5-S1-A03-F03 | SetBaggage | S21 | `instrument/tracer/baggage.go` |
| D5-S1-A03-F04 | PropagateToSubprocess | S21 | `instrument/tracer/propagation_env.go` |

## D5-S21-A14 FilterDebugLog（新增）

| F ID | Name | canonical_s | Code Location |
|------|------|-------------|---------------|
| D5-S21-A14-F01 | MatchCategory | S21 | `instrument/logger/debugfilter/filter.go` |
| D5-S21-A14-F02 | PassthroughNonDebug | S21 | `instrument/logger/debugfilter/filter.go` |

## D5-S23-A07 TrackFileDiagnostics（新增）

| F ID | Name | canonical_s | Code Location |
|------|------|-------------|---------------|
| D5-S23-A07-F01 | SnapshotBefore | S23 | `diagnose/tracker/tracker.go` |
| D5-S23-A07-F02 | DiffAfterEdit | S23 | `diagnose/tracker/tracker.go` |
| D5-S23-A07-F03 | LRUDedup | S23 | `diagnose/tracker/tracker.go` |
| D5-S23-A07-F04 | AsyncLinterTick | S23 | `diagnose/tracker/tracker.go` |

## D5-S23-A09 InjectFault（新增）

| F ID | Name | canonical_s | Code Location |
|------|------|-------------|---------------|
| D5-S23-A09-F01 | ParseFaultEnv | S23 | `diagnose/faultinject/injector.go` |
| D5-S23-A09-F02 | ApplyHook | S23 | `diagnose/faultinject/injector.go` |
| D5-S23-A09-F03 | ProdNoOpStub | S23 | `diagnose/faultinject/injector_prod.go` |

## D5-S23-A10 RunDoctorChecks（新增）

| F ID | Name | canonical_s | Code Location |
|------|------|-------------|---------------|
| D5-S23-A10-F01 | RunAllChecks | S23 | `diagnose/doctor/doctor.go` |
| D5-S23-A10-F02 | ReportHealthy | S23 | `diagnose/doctor/doctor.go` |
| D5-S23-A10-F03 | ReportMissingLSP | S23 | `diagnose/doctor/doctor.go` |

## D5-S0-A03 TrackActiveSessions（新增）

| F ID | Name | canonical_s | Code Location |
|------|------|-------------|---------------|
| D5-S0-A03-F01 | IncActiveSessions | S0 | `bridge.go` (SessionBridge) |
| D5-S0-A03-F02 | DecActiveSessions | S0 | `bridge.go` (SessionBridge) |

---

## Legacy F 索引（路径已更新，ID 不变）

其余 Legacy F（S1–S9 编号）见 `openspec/specs/d5-observability/f-registry.md` v2.0.0；S7 归档时 **仅更新 Code Location 列** 与 **canonical_s 列**，不 renumber F ID。

| Legacy 前缀 | canonical_s | 新路径前缀 |
|-------------|-------------|------------|
| D5-S1-* | S21 | `instrument/tracer/` |
| D5-S2-* | S21 | `instrument/metrics/` |
| D5-S3-* | S21 | `instrument/logger/` |
| D5-S4-* | S22 | `export/` |
| D5-S5-* | S23 | `diagnose/coverage/` |
| D5-S6-* | S21 | `instrument/telemetry/` |
| D5-S8-* | S23 | `diagnose/incident/` |
| D5-S9-* | S24 | `configure/runtime/` |

---

## Statistics

| 新增 F（本 change） | 合计 F（终态目标） |
|--------------------|-------------------|
| +14（诊断 + Filter + Session） | ~45（Legacy 39 + 新增 14 − 重叠统计见归档复核） |

## Revision History

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.0.0 | 2026-06-14 | Legacy 模块 F |
| **3.0.0** | **2026-06-19** | canonical_s + 诊断 F + 路径 |
