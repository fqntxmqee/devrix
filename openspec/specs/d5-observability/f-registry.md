# D5 Observability Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d5-observability/a-registry.md`

---

## Overview

D5 可观测性域 F 层功能点注册表。

---

## D5-S1-A01 CreateSpan

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S1-A01-F01 | StartSpan | F-BE | name, parent | span, ctx | `tracer/tracer.go` |
| D5-S1-A01-F02 | EndSpan | F-BE | span | — | `tracer/span.go` |
| D5-S1-A01-F03 | PropagateContext | F-BE | ctx | carrier | `tracer/propagation.go` |

## D5-S2-A01 RecordMetric

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S2-A01-F01 | IncCounter | F-BE | name, labels | — | `metrics/counter.go` |
| D5-S2-A01-F02 | RecordHistogram | F-BE | name, value, labels | — | `metrics/histogram.go` |

## D5-S3-A01 LogRecord

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S3-A01-F01 | LogAtLevel | F-BE | level, msg, fields | — | `logger/` |
| D5-S3-A01-F02 | ShutdownLogger | F-BE | — | — | `logger/` |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 3 | 7 |
