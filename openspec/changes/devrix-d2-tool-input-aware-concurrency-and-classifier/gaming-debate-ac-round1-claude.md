# AC 全面性复核 — Round 1 (Claude 起草)

> 背景: S3 设计阶段自审发现现有 14 AC 覆盖"特性面"但缺"并发正确性不变量", 且 3 条 AC 文本与 R3 收敛后的缩减范围脱节。本文档提出增补方案, 供 codex + cursor 三方博弈复核。

## 一、问题定位

本 change 核心是把 `ExecuteRound` 从串行改成 **partition 并发** (T18 `partitionToolCalls`)。并发执行的正确性不变量 (保序 / 1:1 / 限流 / 取消清理) 是这类特性的头号 bug 源, 但现有 14 AC **只测了 happy path (同构 50 read_file) + 单一 bash abort 场景**, 未锁核心不变量。

### 现状覆盖矩阵

| 维度 | 现有 AC | 缺口 |
|------|---------|------|
| 方法存在性 | AC1 (IsConcurrencySafe) / AC2 (ToAutoClassifierInput) / AC8 (no silent) | — |
| batch 拆分行为 | AC3 (连续 safe 合并, 但 e2e 同构 read_file) | 交错 safe/unsafe 保序未测 |
| 性能 | AC10 (50 文件 < 串行/3) | — |
| 结果重组 | **无** | **保序 + tool_use_id 1:1 映射未锁** |
| 完整性 | **无** | **N in → N out 无丢无重未锁** |
| 资源边界 | design.md:349 假设"9 并发上限" | **限流 enforcement 无 AC** |
| 失败语义 | AC12 (bash abort 兄弟, P1) | **read-only batch 部分失败 (兄弟应完成) 无 AC** |
| 取消 | **无** | **ctx cancel → goroutine 泄漏无 AC** |
| fail-safe | AC6 (抛错保守 false) | — |

## 二、提议新增 (AC15-AC21, 均 P0)

> 关键论点: 这 7 条 **不新增实现范围** — T18 `partitionToolCalls` 代码本就必须满足这些不变量, AC 只是把"代码必须做对的事"变成可验收的测试点。实现成本 ≈ 1 个 `partition_invariants_test.go` (~6 case) + 1 个 goleak 断言 + 1 个 read_file 单测。

| ID | 标准 | 优先级 | 验证 |
|----|------|--------|------|
| AC15 | **结果保序 + id 匹配** — partition 并发执行后, tool_results 按原始 tool_call 索引重组, 每个 result 的 `tool_use_id` 与对应 call 匹配 (乱序完成不影响输出顺序) | P0 | `partition_order_test`: batch 内 3 call 故意逆序返回 → 重组后顺序 + id 正确 |
| AC16 | **N:N 无丢无重** — M 个 tool_call 输入 → 恰好 M 个 tool_result 输出, 无 drop / dup (含 safe/unsafe 混合序列) | P0 | `partition_count_test` |
| AC17 | **并发上限 enforcement** — batch 内并发受 `maxConcurrency` 上限约束 (errgroup.SetLimit / semaphore), 50 全 safe 不 spawn 50 goroutine | P0 | `partition_limit_test`: 50 safe call, 峰值活跃 goroutine ≤ 上限 |
| AC18 | **交错保序** — `[safe, unsafe, safe, safe]` 序列 → `[safe][unsafe][safe,safe]` 3 batch, 不跨 unsafe 合并两个 safe 组, 保持原序 | P0 | `partition_interleave_test` |
| AC19 | **read-only 部分失败** — read batch 中 1 个失败, 其余照常完成 + 全部 result 返回 (不 abort 兄弟, 区别于 AC12 bash abort 语义) | P0 | `partition_readonly_partial_fail_test` |
| AC20 | **ctx 取消清理** — turn ctx 中途 cancel, 在途 batch goroutine 全部退出无泄漏 | P0 | `partition_cancel_test` (goleak + -race) |
| AC21 | **read_file size 无关** — `read_file.IsConcurrencySafe` 忽略 input size, 恒 true (锁 8K anti-pattern 回归) | P0 | `read_file_surface_test`: 大/小 input 均 true |

## 三、提议对齐 (AC4/AC5/AC11 文本与 R3 缩减范围脱节)

| ID | 原文本 (P0) | R3 现实 | 提议修订 |
|----|-------------|---------|----------|
| AC4 | 真 SideQuery (5s timeout) + 7 单测 allow/deny/... | **P2 interface stub** (T22', 不接真 SideQuery) | 降 P2: `AutoModeClassifier` interface 契约存在 + panic-on-unimplemented 合规 \| `classifier_stub_test` |
| AC5 | `metric_test PASS` (真触发) | **P2** (stub 编译存在, 不实际触发) | 降 P2: telemetry metric stub 编译存在 \| 编译验证 |
| AC11 | 19 工具阈值 + classifier enable + ConcurrencySafe 全可 GrowthBook | **R3 砍到 bash 1 flag** (30K→50K) | P0 缩减: 仅 bash concurrency threshold 1 个 GB flag 可运行时调, 其余默认全关 \| `growthbook_override_test` (bash flag + Production-Safety) |

## 四、影响

- **AC 总数**: 14 → 21 (7 新增 P0 + AC4/AC5 降 P2 + AC11 缩减)
- **T 点**: 新增不变量走现有 T18 (partitionToolCalls) 的测试子项, 不新增 T 编号; AC21 走 T17。可选: 新增 1 个测试文件 T 编号 (待三方定)
- **PR 映射**: AC15-AC20 归 PR-B (partition), AC21 归 PR-A, AC4/AC5/AC11 归 PR-D+E

## 五、请三方回答

1. **Q1**: AC15-AC21 每条是否是真缺口? 有无冗余可合并 (如 AC15+AC16 合成"partition 完整性")?
2. **Q2**: 我漏了哪些并发不变量? (如: batch 间串行屏障 / classifier P2 stub 的 panic 边界 / GrowthBook flag 热切换一致性?)
3. **Q3**: AC15-AC21 是否该独立 T 编号 (D2-S15-A02-T29?) 还是折进 T18 测试子项?
4. **Q4**: AC4/AC5 降 P2 后, 验收标准"stub 编译存在"是否足够, 还是要保留最小 behavior 断言?
5. **Q5**: 7 条全 P0 是否过重? 哪几条可降 P1 (如 AC17 限流 / AC20 取消)?

---

# Round 2 收敛 (Claude + Codex 两方; cursor 后端宕机待补审)

> cursor-agent 连接 api2.cursor.sh 失败 (3 次 reconnect loop), 三方暂缺 cursor 一票。用户选择 (b): 先按 Claude+Codex 落地, cursor 恢复后补审。

## Codex 复核要点

1. **AC15+AC16 合并** → 单条"partition 完整性" (N:N + 保序 + id 1:1)。采纳。
2. **新增 panic 隔离 AC** (Codex B1): 单 tool goroutine panic 不污染 batch。**强论点** (design.md:468 有 L4 wrapper 但无 AC)。采纳为 P0。
3. **AC17 限流 / AC20 取消 → P1**: 资源边界 / 优雅退出, 非语义正确性。采纳。
4. **AC4/AC5 P2 保留最小契约守护测试** (非空 panic, 断言 panic 信息含 "P2 interface, not implemented")。采纳。
5. **T 编号**: 折进 T18 (`partition_invariants_test.go`) + T17 (read_file size), 不独立 T29。采纳。
6. Codex B2 (metric 幂等) / B3 (GB flag 热切换) 建议 P1 → Claude 定 **OOS-NEW-11/12** (边际收益低, 待 cursor 翻案)。

## 最终 AC 编号映射 (写入 demand.md)

| 最终 ID | 内容 | P | 来源 |
|---------|------|---|------|
| AC15 | partition 结果完整性 (N:N + 保序 + id 1:1) | P0 | Claude AC15+16 合并 (Codex) |
| AC16 | 交错 safe/unsafe 保序拆分 | P0 | Claude AC18 |
| AC17 | read-only batch 部分失败不 abort | P0 | Claude AC19 |
| AC18 | read_file IsConcurrencySafe 忽略 size 恒 true | P0 | Claude AC21 |
| AC19 | panic 隔离 | P0 | **Codex B1 新增** |
| AC20 | 并发上限 enforcement | P1 | Claude AC17 (Codex 降级) |
| AC21 | ctx 取消 goroutine 清理 | P1 | Claude AC20 (Codex 降级) |

**总数**: 14 → 21 AC (5 P0 + 2 P1 新增)。P0 总数 14, P1 总数 4, P2 总数 3。

## 待 cursor 补审项
- AC15 合并是否过粗 (完整性 3 子命题挤一条)?
- AC20/AC21 降 P1 是否稳妥 (50 goroutine 资源风险)?
- B2/B3 是否该从 OOS 提为 P1 AC?
