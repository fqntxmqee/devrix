# Proposal: D5 + D6 SA Refine v2.0 — 物理路径迁移

**Change ID:** devrix-d5-d6-sa-refine-v2.0
**Demand ID:** DM-20260615-003
**阶段:** S2 Proposal
**版本:** v1.0
**父 Change:** devrix-d5-sa-refine (DM-20260615-001) + devrix-d6-sa-refine (DM-20260615-002)

---

## 1. 背景

D5/D6 SA Refine v1.0 完成了注册表重排和 Legacy 双轨，但物理 Go 源文件仍位于旧的按技术模块命名的目录中：

| 价值流 | 当前路径 | 目标路径 |
|--------|----------|----------|
| D5-S21 Instrument | tracer/ + metrics/ + logger/ + telemetry/ | observability/instrument/{tracer,metrics,logger,telemetry}/ |
| D5-S22 Export | exporter/ | observability/export/ |
| D5-S23 Diagnose | coverage/ + incident/ | observability/diagnose/{coverage,incident}/ |
| D5-S24 Configure | settings/ + runtime/ | observability/configure/{settings,runtime}/ |
| D6-S11 RunEvaluation | eval/ | evolution/evaluate/ |
| D6-S12 GuardRuntime | orchestration/ | evolution/guard/ |

## 2. 目标

v2.0 执行物理路径迁移，使目录结构与 v1.0 Canonical S 层对齐。

## 3. 非目标

- 不合并 Go 包（保留子包独立 package 声明）
- 不修改 T 层注册表
- 不清理 bridge 文件（v2.1 处理）

## 4. 方法

- git mv 移动文件
- 更新 package 声明（仅 eval→evaluate, exporter→export, orchestration→guard）
- 全局更新 import 路径
- 创建 bridge 文件保持向后兼容
- 参考 D3 SA Refine v2.0 模式

## 5. 风险

- 爆炸半径 ~133 import 路径更新 + ~106 文件移动
- Go 环境不可用，需用户自行运行 go build/test/vet 验证
- 内部交叉导入需要预更新到最终路径

## 6. Decision

| 方案 | 选择 | 理由 |
|------|------|------|
| 子包 vs 单包 | **子包** | 避免 ~94 qualifier rename，保持 Go 惯例 |
| 合并 D5+D6 vs 分开 | **合并** | 路径迁移无逻辑变更，合并减少 change 管理开销 |
| Bridge 生命周期 | **1 release** | 与 D3/D4 v2.0 模式一致 |
