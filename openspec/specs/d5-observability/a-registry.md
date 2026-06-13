# D5 Observability Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D5 可观测性域 A 层活动注册表。

---

## D5-S1: Tracer

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S1-A01 | CreateSpan | A-BE | span_name, parent_ctx | span, ctx | span.{started,ended} | `observability/tracer/tracer.go` |

## D5-S2: Metrics

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S2-A01 | RecordMetric | A-BE | metric_name, value, labels | — | metric.recorded | `observability/metrics/` |

## D5-S3: Logger

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S3-A01 | LogRecord | A-BE | level, message, fields | — | log.emitted | `observability/logger/` |

## D5-S4: Exporter

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S4-A01 | ExportData | A-BE | telemetry_data | — | data.exported | `observability/exporter/` |

## D5-S5: Coverage

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S5-A01 | AssessCoverage | A-BE | target_path | coverage_report | — | `observability/coverage/` |

## D5-S6: Telemetry

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S6-A01 | CollectTelemetry | A-BE | telemetry_event | — | telemetry.collected | `observability/telemetry/` |

## D5-S7: Settings

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S7-A01 | ManageObsSettings | A-BE | config_source | obs_config | — | `observability/settings/` |

## D5-S8: Incident

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S8-A01 | DeclareIncident | A-BE | incident_spec | incident_id | incident.{declared,resolved} | `observability/incident/` |

## D5-S9: Runtime

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S9-A01 | MonitorRuntime | A-BE | runtime_metric | — | metric.recorded | `observability/runtime/` |

---

## Statistics

| Scenarios | Activities |
|-----------|------------|
| 9 | 9 |
