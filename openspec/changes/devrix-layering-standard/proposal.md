# Proposal: Devrix 分层 ID 规范标准化

**Change ID:** devrix-layering-standard
**Demand ID:** DM-20260608-005
**Status:** S0_Deferred
**Author:** Devrix Team
**Date:** 2026-06-08

> **搁置原因 (2026-06-08)：** D-S-A-F-T 方案与当前实际使用的 L1-L2 格式（l5-registry.md 中的 `L5-{L1}-{L2}-{NN}`）存在冲突。当前 L1-L2 方案已能满足需要，且与目录结构天然对应。待项目规模增长、需求追溯成为明确痛点时再重新评估。

---

## 1. Background

Devrix 作为一个多智能体开发助手，当前缺乏统一的 ID 规范来系统化地标识和管理业务能力。随着项目复杂度增长，急需一套完整的 ID 分层体系来实现：

- **标识**: 唯一标识所有业务能力（需求、功能、测试）
- **追溯**: 建立从领域到测试点的完整链路
- **导航**: 快速定位代码、文档和测试用例

## 2. Problem Statement

### 2.1 当前问题

| 问题域 | 现状 | 影响 |
|--------|------|------|
| ID 体系缺失 | 无统一 ID 规范 | PR/Issue 难以定位 |
| 追溯链路断 | 测试与需求无映射 | 回归风险高 |
| 协作效率低 | 多人协作缺乏导航 | 定位成本高 |

### 2.2 备选方案对比

| 维度 | L1-L2 (技术分层) | D-S-A-F-T (业务分层) |
|------|-----------------|---------------------|
| 分层依据 | 技术架构 | 业务语义 |
| ID 语义 | `L5-{领域}-{模块}-{序号}` | `{领域}-{场景}-{活动}-{功能}-{测试}` |
| 可追溯性 | 单向追溯 | 双向追溯 |
| ID 长度 | 短 (8-10 字符) | 中 (17-19 字符) |
| 适用场景 | 小团队 | 中大型团队 |

**选择**: 推荐 D-S-A-F-T 五层分层，原因：
1. 多智能体场景需要清晰的业务分层
2. 需求可追溯：PRD Issue → 功能 → 测试用例完整链路
3. 长期维护：语义清晰的 ID 更易理解和协作

## 3. Proposed Solution

### 3.1 D-S-A-F-T 五层结构

```
{D}-{S}-{A}-{F}-{T}
  │   │   │   │   └── T: 测试点 (Test Point)     01-99
  │   │   │   └────── F: 功能点 (Feature Point)   01-99
  │   │   └────────── A: 业务活动 (Activity)        01-99
  │   └────────────── S: 场景 (Scenario)           01-99
  └────────────────── D: 领域 (Domain)            01-99
```

### 3.2 六层领域定义

| ID | 领域 | 英文 | 描述 |
|----|------|------|------|
| D01 | 通信域 | COMM | 用户交互、消息路由、会话管理 |
| D02 | 上下文域 | CTX | 上下文压缩、PEV 引擎、记忆管理 |
| D03 | LLM 网关域 | LLM | 模型适配、熔断、重试、Token 管理 |
| D04 | 多智能体域 | AGENT | Agent 生命周期、Fork/Join、权限管道 |
| D05 | 可观测域 | OBS | 链路追踪、指标、日志 |
| D06 | 演化域 | EVO | 版本管理、配置热更新 |

### 3.3 示例

**完整 ID:**
```
D02-S02-A03-F03-T01
│  │   │   │   │   └── 验证超时处理测试
│  │   │   │   └────── 命令验证执行器
│  │   │   └────────── 结果验证活动
│  │   └────────────── PEV 执行场景
│  └────────────────── 上下文领域
```

**实际代码映射:**
```go
// D01-S04-A01-F01: 创建新会话
// D02-S01-A02-F06-T01: Autocompact 降级测试
func (s *SessionManager) Create(...) error {
    // D01-S04-A01-F01-T01: 验证输入
    if req.ProjectID == "" {
        return ErrInvalidProject
    }
    // ...
}
```

### 3.4 目录结构映射

```
internal/
├── layers/
│   ├── communication/           # D01
│   │   ├── gateway/           # D01-S04
│   │   │   └── session.go    # D01-S04-A01-F01
│   │   └── adapters/         # D01-S01
│   │       └── feishu.go     # D01-S01-A02-F01
│   │
│   ├── contextengine/         # D02
│   │   ├── compression/     # D02-S01
│   │   │   ├── pipeline.go  # D02-S01-A02-F01~F07
│   │   │   └── autocompact.go  # D02-S01-A02-F06
│   │   ├── pev/            # D02-S02
│   │   │   └── verify_runner.go  # D02-S02-A03-F03
│   │   └── memory/        # D02-S03
│   │
│   ├── llmgateway/          # D03
│   │   ├── breaker/       # D03-S02
│   │   └── adapter/       # D03-S01
│   │
│   └── observability/      # D05
│       ├── tracer/        # D05-S01
│       └── metrics/       # D05-S02
│
└── tests/
    ├── integration/        # D*-S*-A*-F*-T* integration
    ├── acceptance/        # D*-S*-A*-F*-T* acceptance
    └── performance/        # D*-S*-A*-F*-T* performance
```

## 4. Success Metrics

| Metric | Target |
|--------|--------|
| 规范文档发布 | `specs/architecture/layering-standard.md` |
| 核心领域 ID 映射 | D01-D06 完成 |
| 新功能 ID 覆盖率 | 100% (新功能) |
| 现有代码迁移 | 按需，不强制 |

## 5. 实施计划

### 5.1 阶段划分

| Phase | 内容 | 输出 |
|-------|------|------|
| **P1: 规范制定** | 完善 D-S-A-F-T 规范 | `layering-standard.md` |
| **P2: 核心映射** | D01-D06 ID 分配 | ID 映射表 |
| **P3: 新功能试点** | 新增功能使用 ID | 代码示例 |
| **P4: 工具支持** | ID 生成/验证脚本 | `scripts/` |

### 5.2 不改动部分

- 生产代码逻辑
- 测试逻辑
- 配置文件结构
- CI/CD 流程

## 6. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| ID 长度增加维护成本 | 低 | 仅标注，不强制转换现有代码 |
| 规范与现有代码不一致 | 中 | 从新功能开始执行 |
| 规范变更导致 ID 失效 | 低 | 版本化规范，新增 ID 不重复使用 |

## 7. 评审待确认

- [ ] D-S-A-F-T 是否满足所有业务场景？
- [ ] ID 长度 (17-19 字符) 是否可接受？
- [ ] 现有代码迁移策略？
- [ ] 测试文件命名规范选择 (分离 vs 内联)?
