# Design: D7 子包清理热身 Sprint

**Change ID:** `d7-package-cleanup-sprint`
**Demand ID:** DM-20260625-018
**Status:** S3_Design → S3-Gate(Review) → S4_Implemented
**设计原则**: 6 S + 1 横切 (v6.0.0 域升级) + 纯物理迁移（0 函数签名变化）

---

## 1. 背景与目标

v6.0.0 域升级（DM-20260626-001/002/003/004/005/007）完成 14 S → 6 S + 1 横切的对齐，但 4 个小子包遗留：runregistry/、toolpolicy/、sessionqueue/、d7spans/。

**目标**：1 周内 3 个 PR 完成 4 个子包物理合并，0 函数签名变化，让 D7 目录结构达到理想态（11 个子目录 = 6 S + 1 横切 + 4 工具）。

## 2. 当前结构 vs 目标结构

### 2.1 当前结构（v6.0.0 follow-up 全部 S7_Archived 后）

```
orchestration/
├── runregistry/        (3 文件, 待合并)
├── toolpolicy/         (6 文件, 待合并)
├── sessionqueue/       (3 文件, 待合并)
├── d7spans/            (2 文件, 待合并)
├── decisionplanning/   (parent)
├── delegatetools/      (未动)
├── escape/             (未动)
├── executionflow/      (parent, 0 Go 文件, 仅子包)
│   ├── bridge/
│   ├── hub/
│   ├── imsink/
│   ├── verify/
│   └── workplan/
├── hardening/          (parent)
├── mups/               (未动)
├── orchtypes/          (未动)
├── sessionorchestrator/(未动)
├── wavescheduler/      (未动)
└── workmodel/          (parent)
```

### 2.2 目标结构

```
orchestration/
├── decisionplanning/    (扩至 ~10 文件, +toolpolicy 6 文件)
├── delegatetools/       (未动)
├── escape/              (未动)
├── executionflow/       (扩至父级 3+1 文件)
│   ├── bridge/          (未动)
│   ├── hub/             (未动, 删内部 import 行)
│   ├── imsink/          (未动)
│   ├── verify/          (未动)
│   ├── workplan/        (未动)
│   ├── doc.go           (NEW, 1 行注释)
│   ├── session_queue.go (NEW, from sessionqueue/)
│   ├── session_queue_test.go (NEW)
│   └── delegate_progress_test.go (NEW)
├── hardening/           (扩至 ~7 文件, +d7spans 2 文件)
├── mups/                (未动)
├── orchtypes/           (未动)
├── sessionorchestrator/ (未动)
├── wavescheduler/       (未动)
└── workmodel/           (扩至 ~45 文件, +runregistry 3 文件)
```

11 个子目录（6 S + 1 横切 + 4 工具），达到理想态。

## 3. PR 拆分策略

按"影响半径 + 隔离风险"原则：

| PR | 范围 | 内部 D7 importer | 跨域 importer | 风险 |
|----|------|-----------------|---------------|------|
| **PR-1** | runregistry → workmodel | 6 (4 in-package) | 1 bootstrap | 最低：热身 |
| **PR-2** | toolpolicy → decisionplanning | 0 | 4 (bootstrap + D2 + CLI + integration test) | 中：唯一 D2→D7 反向引用，隔离 |
| **PR-3a** | d7spans → hardening | 5 | 0 | 中：5 importer + 3 spec |
| **PR-3b** | sessionqueue → executionflow 父级 | 2 (in-package) | 4 bootstrap + 1 testutil | 中-高：executionflow 父级扁平 |

**分组理由**：
- PR-1 热身，安全独立
- PR-2 隔离，避免与 sessionqueue/d7spans 共同 quarantine
- PR-3a + 3b 合并为 PR-3，因为两者都有较多内部 D7 importer，合并 review + 验证流程更高效

## 4. 关键设计决策

### 4.1 executionflow 父级扁平（PR-3b）

**问题**：sessionqueue 合并到 executionflow/ 哪个位置？
- 方案 A：executionflow/ 父级（executionflow/ 当前 0 Go 文件）
- 方案 B：executionflow/hub/ 兄弟包

**决策**：方案 A（用户已确认）

**理由**：
1. 与其他 3 个合并方向一致（runregistry→workmodel 父级、toolpolicy→decisionplanning 父级、d7spans→hardening 父级）
2. 符合 6 S + 1 横切对齐精神（executionflow 是 S4，sessionqueue 是 S4 内的子能力，不应拆给 hub）
3. 新增 `executionflow/doc.go` 1 行注释说明，缓解"父级有 Go 文件"的可读性问题

### 4.2 in-package import 删行（PR-1, PR-3b）

**问题**：workmodel 内部 4 个 importer 合并后变 in-package，sessionqueue 2 个 importer 同理。

**决策**：**直接删除 import 行**，不要保留 `import ".../workmodel"` 这种自引用。

**理由**：
1. Go 语言规范禁止包内 self-import
2. 保留会导致编译失败

**风险**：误保留 → 编译失败 → 立即发现（go build 0 错误是 PR 验证硬条件）

### 4.3 跨域 D2→D7 反向引用（PR-2）

**问题**：`internal/layers/contextengine/enforce/contracts.go:14` 注释中引用 `orchestration/toolpolicy` 路径。

**决策**：注释路径同步替换为 `orchestration/decisionplanning`。

**理由**：
1. 注释描述路径与实际包路径一致
2. 避免后续 reader 困惑

**特殊**：这是 D2 → D7 反向引用，唯一例外。其他跨域引用都是 D7 → D7 + bootstrap → D7。

## 5. CI 资源影响

### 5.1 `.github/CODEOWNERS`

runregistry/ 是唯一在 CODEOWNERS 中单独列出的子包。合并后删除该行。

### 5.2 `scripts/audit-property-rights.sh`

第 26 行有兜底分支 `*orchestration/runregistry/*`（workmodel 之外的特例）。合并后该兜底分支无意义，删除。

**验证**：`bash scripts/audit-property-rights.sh` 应返回 0 violations。

### 5.3 `scripts/verify-archive.sh`

不影响。该脚本检查 OpenSpec 归档完整性，与代码包名无关。

## 6. Spec 同步清单

| Spec | 路径 | 同步内容 |
|------|------|---------|
| D7 t-registry | `openspec/specs/d7-orchestration/t-registry.md` | runregistry/spawn_test.go → workmodel/registry_test.go（PR-1）；d7spans 引用 → hardening（PR-3a） |
| D7 design | `openspec/specs/d7-orchestration/design.md:759` | d7spans.SetBridge → hardening.SetBridge（PR-3a） |
| D7 d7-domain | `openspec/specs/d7-orchestration/d7-domain.md:112` | 同上 |
| D7 observability-guide | `openspec/specs/d7-orchestration/observability-guide.md:172` | sessionqueue → executionflow（PR-3b） |
| D7 clarifications | `openspec/specs/d7-orchestration/d7-requirements-clarifications.md:56` | 目录列表（PR-3b） |
| D7 design S5 验收 | `openspec/specs/d7-orchestration/d7-domain.md` v2.2.0 → v2.3.0 | 目录结构图（S5 阶段） |
| 全局 code-layout | `openspec/specs/architecture/code-layout.md:101,103,156,209` | 多处子包路径替换 |
| D2 spec | `openspec/specs/d2-context-engine/spec.md:1197` | 路径替换（PR-2） |
| D2 a-registry | `openspec/specs/d2-context-engine/a-registry.md:73` | 路径替换（PR-2） |
| D2 t-registry | `openspec/specs/d2-context-engine/t-registry.md` 多处 | 路径替换（PR-2） |
| 根 t-registry | `openspec/t-registry.md` v5.4.0 → v5.5.0 | DM-20260625-018 增量条目（S5 阶段） |

## 7. 端到端验证（3 PR 全部合入后）

1. `go build ./...` 0 错误
2. `go vet ./...` 0 警告
3. `go test -race ./internal/layers/orchestration/...` 22/22 包 PASS
4. `go test -race ./internal/layers/contextengine/... ./internal/bootstrap/... ./internal/cli/... ./tests/integration/... ./tests/testutil/...` 全部 PASS
5. `bash scripts/verify-archive.sh` 12/12 PASS
6. `bash scripts/audit-property-rights.sh` 0 violations
7. `ls internal/layers/orchestration/` 不包含 runregistry、toolpolicy、sessionqueue、d7spans
8. `go build -o bin/devrix ./cmd/devrix && restart`，飞书端发消息主路径正常
9. 3 PR 全部 auto-merge
10. S6 归档完成
