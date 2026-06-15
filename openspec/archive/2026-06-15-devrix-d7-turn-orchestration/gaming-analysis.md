# Gaming Analysis: D7 Turn Leader vs D2 LLM 僭越

**Change ID:** devrix-d7-turn-orchestration  
**Demand ID:** DM-20260614-020

---

## 1. 博弈角色重划

| 域 | 旧角色 | 新角色 | 博弈机制 |
|----|--------|--------|---------|
| D7 | Mediator（但 LLM 不可见） | **Turn Leader** | 拥有 commitment device（Turn 状态机可验证） |
| D2 | Follower（实际 Agent） | **Context Follower** | 只提供 costly signals（durable persist 后才 complete） |
| D3 | 被 D2 隐藏的供应商 | **可观测供应商** | Breaker/Route 对 D7 透明 |

```
Stackelberg:
  Leader (D7) 先动：决定何时调 D2 / D3
  Follower (D2) 后动：在给定 PreparedContext 下执行工具/持久化
  D3 不参与博弈，提供机制（Breaker = 机制约束）
```

---

## 2. 激励错配（现状）

| 行为体 | 局部最优（现状） | 全局最优 |
|--------|----------------|---------|
| D2 开发者 | 在 Loop 内直连 D3，少改 D7 | D7 可编排 Breaker/路由 |
| D7 开发者 | 黑盒委托 D2，接口稳定 | Turn 可观测、可取消 |
| D3 开发者 | 服务 D2 契约即可 | D7 可消费韧性状态 |

**根因：** Turn SoT 在 D2 → D7 缺乏对 LLM 路径的 **可验证承诺**。

---

## 3. 机制设计（切法后）

| 机制 | 实现 | 防什么 |
|------|------|--------|
| Turn 状态机 | D7-S2-A06 | D2 偷偷加 LLM 调用 |
| import lint | D2-THIN-T01 | 代码层 D2→D3 回潮 |
| Prepare 无 LLM | D2-S15 CompressHint | Autocompact 绕路 |
| SubQuery 嵌套 Turn | D7 Scope=subquery | 内层循环僭越 |
| Legacy 双轨 | S16 冻结 + Archive | T 追溯断裂 |

---

## 4. 与 D4 Follower 对称

| Follower | Leader 派发 | Follower 承诺 |
|----------|------------|--------------|
| D4 ExecuteWorker | D7 Hub-Spoke | fork/run/join 机制正确 |
| D2 Context+Tools | D7 Turn | prepare/tools/persist 机制正确 |

**统一原则：** Follower 不选择 Spoke/LLM 路径；Leader 选择。

---

## 5. 信息结构

| 信号 | 迁前谁知道 | 迁后谁知道 |
|------|-----------|-----------|
| Breaker open | D3 + D2 | **D7** + D5 |
| Model tier 选择 | D2 内部 | **D7** InvokeLLM |
| Tool permission | D2 | D2（不变） |
| Turn 完成 | D2 emit complete | D7 编排后 D2 persist |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿 |
