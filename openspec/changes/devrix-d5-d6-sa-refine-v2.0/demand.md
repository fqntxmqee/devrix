# Demand: D5 + D6 SA Refine v2.0 — 物理路径迁移

**Demand ID:** DM-20260615-003
**Phase:** S3 Design
**Parent:** devrix-d5-sa-refine (DM-20260615-001) + devrix-d6-sa-refine (DM-20260615-002)

## 价值承诺

| # | 承诺 | 消费者 | 验证方式 |
|---|------|--------|----------|
| C1 | D5 目录结构反映 4+1 价值流 S 层 | 所有开发者 | `go build ./...` 全绿 + 目录结构匹配 |
| C2 | D6 目录结构消除命名冲突 | 运维/D7 | `evolution/guard/` 与 `orchestration/` 无冲突 |
| C3 | 11 bridge 文件保证向后兼容 | 所有 importer | 旧 import 路径仍可编译 |
| C4 | 零 T 层测试退化 | QA | P0 T 100% PASS |

## 非功能需求

- 零包引用重命名（除 D6 eval→evaluate, exporter→export, orchestration→guard）
- bridge 文件标记 `// Deprecated:`，1 release 后删除
- 不得新增 import cycle
