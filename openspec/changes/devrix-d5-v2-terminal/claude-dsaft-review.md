---
demand-id: DM-20260619-006
change-id: devrix-d5-v2-terminal
review-type: DSAFT architecture review (Claude perspective)
stage: S3-Gate pre-approval (Owner 待审)
created: 2026-06-19
---

# Claude DSAFT 五层视角 Review — devrix-d5-v2-terminal

> Owner 在 S3-Gate 决议前可参考的 DSAFT 方法论对焦。本 review 不重复 `gaming-analysis.md`（MiniMax 博弈论推导）与 `design.md`（六段式 Decision），只从 DSAFT §五"精确定义"+"追溯规则"视角交叉验证需求结构是否自洽。

---

## 一、D 层（领域）：公共域 + 裁判域双轨定位

| 维度 | 现状 v2.0 | 终态 v2.1 | DSAFT 评估 |
|------|----------|----------|----------|
| 类型 | 公共域 | 公共域 + **裁判域**（双轨） | ✅ 合理 |
| 领域 SoT | 散落 spec/design/architecture | 新建 `d5-domain.md` | ✅ 修复 D 层缺失 |
| 跨域契约 | 隐式 | 新建 `d5-boundary.md` | ✅ 公共域必备 |

**DSAFT 引用：** §五.1 "D 层有自己的业务规则和 invariants" + §三 "S → D 归属"。D5 v2.0 缺 SoT = D 层骨架缺失；其他域（D1/D2/D3/D7）都有 `d{N}-domain.md`，D5 补齐是必然路径。

**潜在疑问：** 公共域与裁判域是否冲突？答：不冲突——"公共域"是 D 层在系统中的角色（被多域依赖），"裁判域"是 C3 诊断辅助承诺的隐喻（audit 属性）。建议在 `d5-domain.md` 顶部明确"公共域 = 角色定位，裁判 = 业务隐喻"。

---

## 二、S 层（场景）：技术角色 → 价值流切法 A 迁移

### 2.1 核心架构动作

| 旧 S（技术角色，v2.0） | 新 S（价值流，v2.1） | 业务语义变化 |
|----------------------|-------------------|-------------|
| D5-S1 Tracer | **D5-S21 Instrument** | Trace/Metric/Log 三合一 S（"为任意 operation 注入遥测"） |
| D5-S2 Metrics | **D5-S21**（合并） | |
| D5-S3 Logger | **D5-S21**（合并） | |
| D5-S4 Exporter | **D5-S22 Export** | |
| D5-S5 Coverage | **D5-S23 Diagnose**（合并） | 5 类诊断子能力合并 |
| D5-S6 Telemetry | **D5-S21**（合并） | |
| D5-S7 Settings | **D5-S24 Configure** | |
| D5-S8 Incident | **D5-S23**（合并） | |
| D5-S9 Runtime | **D5-S24**（合并） | |
| **无** | **D5-S0 Facade**（新增） | Init/Shutdown/Bridge/SessionGauge |

### 2.2 DSAFT 视角评估

**正面：**
- ✅ 切法 A 一致性：D3 S1-S6、D7 S1-S5、D5 S21-S24——跨域 S 编号段对齐，可比较。
- ✅ Trace/Metric/Log 合并到 S21：它们共享同一上游事件（operation 调用），按"用户目标"应归属同一 S。
- ✅ Coverage/Incident/Doctor/Tracker/FaultInject 合并到 S23：共同上游"辅助排查"，符合 DSAFT §五.2 "S 应当聚焦用户目标"。

**风险：**
- ⚠️ S23 子承诺 C3a-C3e 共 5 类诊断（A07-A10 + A06），若 A 数 > 5 建议拆 S。当前 A08-A10 + A14 共 4 个新增 A 在 S21/S23 内部，**未超阈值**。
- ⚠️ S0 边界模糊：Facade 本质是"集成壳"，可能成为"反 D"（吸纳过多非 Instrument 能力）。建议在 `d5-domain.md` 显式列出 S0 不收纳的业务能力清单（与 Out of Scope 互文）。

### 2.3 DSAFT 引用

§五.2 "S 是用户达成完整目标的业务价值流"——v2.0 的 S1-S9 是技术角色（Tracer/Metrics/Logger），不是用户目标。v2.1 的 S21-S24 是价值流（Instrument/Export/Diagnose/Configure）。

---

## 三、A 层（活动）：增量登记 + T↔A 编号冲突修复

### 3.1 A 层规模演进

| 版本 | A 总数 | 备注 |
|------|--------|------|
| v2.0 | 18 | 散落 S1-S9 |
| v2.1 | **30**（v4.0 注册表） | 净增 12 个 |
| 增量明细 | A14 FilterDebugLog（S21）/ A03 TrackActiveSessions（S0）/ A07-A10（A23 诊断）/ 其他 | |

### 3.2 T↔A 编号冲突修复（DSAFT 必修）

v2.0 有 T 挂错 A 的现象：
- D5-S23-A03-T01 实际归属 A07（Tracker）
- 类似冲突若干

**DSAFT §三 追溯规则：** "每个 T 必须归属一个 A 或 F"——错位 = 违反追溯规则。本次 `t-registry v3.2` 校正 + `canonical_a` 列批量更新是追溯规则的形式化执行。

### 3.3 Phase B3 PLANNED T 闭合策略

41 T 中 2 个 PLANNED：
- 选项 A：实现 + 标 IMPLEMENTED
- 选项 B：保留 PLANNED + sad path 说明

**DSAFT 视角：** §二.1 稳定性梯度 "T 最稳定"——T 的对外契约语义不应变，实现状态可标 PLANNED。建议 **优先 B（保留契约），如资源允许再 A**——保护下游消费者预期。

---

## 四、F 层（功能点）：canonical_s 双轨过渡

| 维度 | v2.0 | v2.1 |
|------|------|------|
| F 总数 | 39 | ~45 |
| 组织方式 | 按 Legacy S1-S9 | 按 Canonical S0+S21-S24 |
| 双轨字段 | 无 | 新增 `canonical_s` 列 |

**DSAFT 视角：** §三 "F → A" + §十 "design → F"——F 层是 A 的内部编排单元，应与 A 共用同一 S 归属。`canonical_s` 列本质是"Legacy ID → Canonical ID"映射表，是过渡期（双轨期）的兼容性桥。

**建议：** 双轨过渡期间，layer-lint 应允许 F 的 `Legacy S` 与 `Canonical S` 同时存在并校验一致性，禁止出现"Legacy S 标 X、Canonical S 标 Y 但 X≠Y"的不一致。

---

## 五、T 层（测试点）：41 ID 不变 + canonical_s/canonical_a 校正

| 维度 | v2.0 | v2.1 |
|------|------|------|
| T 总数 | 41 | **41**（ID 不变） |
| IMPLEMENTED | 39 | 41 或 39 + 2 有 sad path |
| canonical_s 列 | 无 | 有 |
| canonical_a 列 | 无 | 有 |

**DSAFT §2.1 引用：** "T 最稳定（不变契约）"——41 ID 不变正是这条原则的体现。重构可改 S/A/F，但 T 的 WHAT（验收契约）不应变，仅 HOW（实现细节）可变。

---

## 六、跨域视角：D5 作为裁判的双向契约

### 6.1 D7 Turn 主路径 vs D5 Referee

```
gateway.message.receive [SERVER]
└── orchestration.turn.run [INTERNAL]    ← D7 owns
    └── orchestration.turn.iteration
        ├── orchestration.llm.invoke [CLIENT]
        │   └── llm.stream [CLIENT]
        └── tool.execute.single
            └── context.process            ← D2 owns (caller=d7)
                ↑
                D5 仅采集，不拥有语义
```

**DSAFT §三 应用：** 跨域 span 应有明确归属 owner。`context.process` 的"Prepare 语义"归 D2，span 名称归 D5。`d5-boundary.md` 的 §C2 HookRegistry 拦截点 = 这条规则的形式化。

### 6.2 与其他域的对齐检查

| 域 | 价值流 S 段 | D5 对齐点 |
|----|------------|----------|
| D1 | S13-S18（价值流） | gateway.message.receive 入口采集 |
| D2 | S15-S20（canonical） | context.process / compression.run / tool.execute 采集 |
| D3 | S1-S6（5+1） | llm.stream / llm.adapter.stream 采集 |
| D4 | S1-S5 | agent.run / agent.tool.call 采集 |
| D7 | S1-S5 | orchestration.* 采集（v1.1 后主路径） |
| D6 | — | eval 探针采集 |
| Shared | — | config / types |

D5 的 S21 Instrument 必须能 hook 到**所有 6 个域的 operation**——这是公共域 + 裁判域的双重责任。

---

## 七、Phase 划分 DSAFT 评估

### 7.1 Phase 顺序合理性

```
Phase A (docs) ──S3-Gate──► Phase B1 ──► Phase B2 ──► Phase B3 ──► Phase C
```

| Phase | DSAFT 风险 | 建议 |
|-------|----------|------|
| A (docs-only) | 低——纯规格对齐 | ✅ 适合 S3-Gate 后立即执行 |
| B1 (根目录归位) | 中——package 声明 + 全仓 import | ✅ 与 A 可并行 |
| B2 (bridge 删除 9 包) | 高——一次性删 9 包 + 全仓 import grep | ⚠️ 参考 D2 v2.2 closure 的成功经验（物理迁移后留 1 release shim） |
| B3 (PLANNED T 闭合) | 低——T 状态调整 | ✅ |
| C (验收 + S7) | 低——标准流程 | ✅ |

### 7.2 Phase B2 bridge 删除的具体建议

D2 v2.2 closure 教训：物理迁移 + 留 1 release shim 是安全路径。本次 B2 一次性删 9 包：

- ✅ 已有 D2 v2.2 成熟先例（DM-20260619-007）
- ⚠️ 建议先在 `bridge.go` 内 re-export 1 个 release，再 B2 删除——若 Owner 接受，可降风险
- ⚠️ `grep observability/tracer` 全仓 0 命中（除 archive/docs）的检查应在 CI 自动化，避免回归

### 7.3 AC-A7 `grep query.loop` 应进 CI

当前 AC-A7 只在 Phase A Gate 验收"grep query.loop 仅 RETIRED/Legacy"。建议把这条规则写入 layer-lint，让 CI 强制执行——否则 S7 归档后回归难查。

---

## 八、对 Owner S3-Gate 决议的 5 条具体建议

| # | 建议 | 理由 |
|---|------|------|
| 1 | 接受 S3 设计，按 Phase A 顺序推进 | docs-only 风险低，是后续 B/C 的前置 |
| 2 | Phase B2 拆为 2 步：B2a 改 bridge.go import 到 canonical、B2b 删 9 包（中间留 1 release shim） | 降低 9 包删除的爆炸半径 |
| 3 | AC-A7 `grep query.loop` 写入 layer-lint（CI 强制） | 防止 S7 归档后回归 |
| 4 | `t-registry v3.2` 的 canonical_s/canonical_a 校正必须与 a-registry/f-registry 一次性同步 | 避免三表不一致引发 lint 错误 |
| 5 | `d5-domain.md` 顶部明示 S0 边界（不收纳清单），防止 S0 演化成"反 D" | S0 Facade 易吸纳过多非 Instrument 能力 |

---

## 九、OpenSpec 阶段映射

| 阶段 | DSAFT 产出 | 本 change 状态 |
|------|-----------|--------------|
| proposal | D + S | ✅ S0+S21-S24 已定 |
| specs | A | ✅ a-registry v4.0（30 A） |
| design | F + A↔F | ✅ f-registry v3.0（~45 F） |
| tasks | F 实现任务 | ✅ tasks.md（Phase A/B/C） |
| verify | T | ✅ t-registry v3.2（41 T） |
| **Gate** | 跨层一致性 | ⏳ Owner 待审 |

---

## 十、参考引用

- DSAFT 方法论：`docs/methodology/dsaft-methodology.md` §五 + §三 + §二.1 + §十
- D7 v2.1 structure closure（DM-20260619-005）作为本次参考先例：`openspec/archive/2026-06-19-devrix-d7-v2-structure/`
- D2 v2.2 closure（DM-20260619-007）：bridge 删除的成熟路径参考
- `gaming-analysis.md`（MiniMax 博弈论推导）
- `design.md`（六段式 Decision）
- `d5-requirements-clarifications.md`（Grill Review §6 六个问题）

---

*Claude DSAFT Review v1.0 — 2026-06-19*