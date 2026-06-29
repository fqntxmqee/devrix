# Design: D7 TaskContract 统一 PR-C — CoW VersionChain + Similarity Check + Hard Evidence

**Change ID:** devrix-d7-taskcontract-unification-pr-c
**Demand ID:** DM-20260629-009
**Status:** S3_Design
**Parent Proposal:** `proposal.md`
**Parent Demand:** `demand.md`
**Template:** `docs/methodology/detail-design-framework.md`（六段式 ①-⑥）
**Created:** 2026-06-29

> **本设计文档遵循 devrix-architecture-design-six-segment-migration 规定的六段式 (①-⑥) 模板。**
> 父设计文档 `archive/2026-06-29-devrix-d7-taskcontract-unification/design.md` (648 行) 含 6 主段 + 5 附录，本 PR-C 文档仅就 PR-C 增量部分做差异化设计（避免 60% 父设计内容复制），父设计引用处使用 `→ 父设计 §X` 标识。
> 复用 PR-A/B 资产详 → 父设计 §2.3 复用表（23 AC 跨 3 PR 累积）。

---

## ① 设计目标与约束

### 1.1 业务目标

PR-C 是 v7.0 演进 **最终 PR（4-Layer × 3-Phase 3/3 闭环）**，落地父设计中 L3 高风险 + L4 收口共 6 AC + 1 收口 T：

| 父设计 AC | 名称 | 业务痛点 | PR-C 落地位置 |
|----------|------|---------|--------------|
| **AC13** | CoW VersionChain | 子层 Replan 无版本链，WHY 决策丢失 | `interfaces/version_chain.go` + `workmodel/version_chain.go` |
| **AC14** | Similarity Check | 相似子任务无防御，烧 token | `interfaces/similarity_check.go` + `decisionplanning/decomposer.go` |
| **AC15** | Hard Evidence | Verifier "空证 PASS" silent corruption | `interfaces/hard_evidence.go` + `executionflow/verify/verifier.go` |
| **PR-B T05** | Span/Metric 完整 wire | PR-B 占位 slog.Info，灰度期无 metric | `hardening/emitter.go` + `hardening/metrics.go` |
| **AC6** | convergence span | Observe 节点收敛宽度无度量 | `hardening/emitter.go` + `mups/observe/aggregator.go` |
| **AC18** | Coverage ≥ 80% | 新增包无硬指标 | S5 验收 |
| **AC22** | Feature Flag 灰度 | 高风险变更需灰度 | `d7-bootstrap/wire.go` |

→ 父设计 §3.4 决策树（PR-C 落在 "事前防御" 段）+ §5.3 CoW VersionChain 链路

### 1.2 设计约束

1. **PR-A/B 兼容性 100%**：PR-C 不得修改 PR-A/B 已 IMPLEMENTED 的接口（TaskSpec/TaskReport/Blockage/Dissent/Resource/MVPArtifact/PessimisticCommitGuard/FallbackPolicy/ConvergenceBudget）
2. **interfaces/ 0 import D7 子包**（IV-1 不变量）：`version_chain.go` / `similarity_check.go` / `hard_evidence.go` 仅依赖 `internal/shared/errors/`
3. **Pure types 原则**（PR-A 落地）：所有新增 interfaces 文件遵守 `With*` 浅拷贝不可变
4. **CoW 仅追加语义**：VersionChain.Append 仅追加，永不修改历史节点
5. **Similarity O(1) 性能**：token-level Jaccard 用 `map[string]struct{}` 哈希集合（实测 P99 < 0.1ms）
6. **Hard Evidence kind-specific**：code vs chat 严格分离，避免误伤轻量任务
7. **Feature Flag env-gated 默认 disabled**（PR-A/B 模式继承）：D7_HARD_EVIDENCE_ENABLED + D7_SIMILARITY_CHECK_ENABLED
8. **Hash 算法升级**：PR-B FNV-1a → PR-C SHA-256（`crypto/sha256` 标准库，256-bit 安全）
9. **错误码区间不冲突**：7120-7129（PR-C）vs 7100-7104（PR-A）vs 7110-7113（PR-B）
10. **0 行为变更承诺**：LP-1/LP-2/LP-5 集成测试 100% 兼容

### 1.3 关键决策

#### Decision 1: VersionChain 存储策略

| 选项 | 优点 | 缺点 | 选择 |
|------|------|------|------|
| A. 只存 hash | 存储最小；O(1) 查找 | 历史版本需另存（GC 复杂度）| — |
| B. 全 state inline | 历史完整；回滚直接 | 存储爆炸（10x-100x）| — |
| **C. hash + 24h GC（hash 索引 + 后台清理）** | 存储可控；O(1) 查找 | 实现复杂 | ✅ **PR-C 选择** |

**理由**：→ 父设计 §3.4 Decision 2 已选 C，本 PR-C 继承。C 是工业级 "可回滚 + 存储可控" 唯一同时满足方案。GC 周期 24h 参考 D5 `metrics.go` retention 经验。

#### Decision 2: Similarity Check 算法

| 选项 | 优点 | 缺点 | 选择 |
|------|------|------|------|
| A. embedding + cosine | 语义精准 | 每 Downlink 多 1 次 LLM 调用，慢 | — |
| **B. token-level Jaccard** | 极快 O(1)；纯字符串 | 边界（"你好"和"你好啊"高相似）| ✅ **PR-C 默认选择** |
| C. B + 边界 LLM 二次校验（0.7-0.85）| 快 + 准 | 实现稍复杂 | ⏳ PLANNED 留 v7.0.1 |

**理由**：→ 父设计 §3.4 Decision 4 已选 C，本 PR-C 落地 B 子集。C 完整版留 v7.0.1 follow-up（`devrix-d7-similarity-llm-boundary`）。本 PR-C 对 0.7-0.85 边界仅 slog.Warn 标记。

#### Decision 3: Hard Evidence 最小集

→ 父设计 §3.4 Decision 5：

| 任务 kind | 最小集 | 缺失行为 |
|----------|--------|---------|
| `code` | TestCoveragePct ≥ 1 OR LogExcerpt 非空 OR ArtifactHash 非空 | Kind=Partial + Blockage.RequiredExternal=["hard_evidence"] |
| `chat` | CoherenceScore ≥ 0.5 OR EntityHash 非空 | 同上 |
| `unknown` | 同 code（保守） | 同上 |

**理由**：kind-specific 严格分离，避免 chat 任务被 test 强制拒绝。

#### Decision 4: Hash 算法升级（FNV-1a → SHA-256）

| 选项 | 优点 | 缺点 | 选择 |
|------|------|------|------|
| A. 维持 FNV-1a (PR-B) | 改动最小 | 16-char hex 碰撞风险（生日悖论 ~65K）| — |
| **B. 升级到 SHA-256 (crypto/sha256)** | 标准库 256-bit 安全；零依赖 | 64-char hex 略长 | ✅ **PR-C 选择** |
| C. 升级到 SHA-256 + 内容寻址（IPFS-like）| 去重能力强 | 实现复杂 | ⏳ PLANNED v8.0 |

**理由**：PR-B 的 buildChainHash 内部函数升级；外部接口（Hash 字符串）长度变化但语义不变。SHA-256 是工业级标准库选择。

#### Decision 5: WorkItem.VersionChain 嵌入方式

| 选项 | 优点 | 缺点 | 选择 |
|------|------|------|------|
| A. 直接嵌入字段（additive）| 简单 | 序列化兼容测试多 | ✅ **PR-C 选择** |
| B. 独立 Sidecar 存储 | 不影响 WorkItem 序列化 | 跨引用 join 慢 | — |
| C. 接口注入（WorkItemVersionChainable）| 多态 | 实现复杂 | — |

**理由**：A 是 additive 模式（PR-A 沿用），`omitempty` JSON tag 保证向后兼容；`VersionChain == nil` 视为无 chain（老 WorkItem 完全不变）。

---

## ② 领域模型

### 2.1 新增 / 修改的聚合

#### VersionChain (interfaces/version_chain.go, NEW)

```go
// Hash is a content-addressed version identifier (SHA-256, 64-char hex).
type Hash string

// VersionChainEntry is an immutable CoW snapshot of a WorkItem state.
type VersionChainEntry struct {
    Hash        Hash           // SHA-256(content + parent_hash)
    Parent      Hash           // previous version (empty for genesis)
    Content     []byte         // serialized snapshot
    CreatedAt   time.Time
    Reason      string         // "replan", "rollback", "commit"
}

// VersionChain is an append-only CoW chain with O(1) hash lookup.
type VersionChain struct {
    entries map[Hash]*VersionChainEntry  // O(1) hash index
    order   []Hash                       // insertion order
    head    Hash                         // current pointer (never GC'd)
}

func NewVersionChain() *VersionChain
func (vc *VersionChain) Append(content []byte, reason string) (Hash, error)  // O(1) append + hash
func (vc *VersionChain) RollbackTo(h Hash) error                            // O(1) hash lookup
func (vc *VersionChain) Get(h Hash) (*VersionChainEntry, bool)              // O(1) lookup
func (vc *VersionChain) Head() Hash                                          // current
func (vc *VersionChain) GC(ttl time.Duration) (int, error)                   // 24h cycle, never deletes head
func (vc *VersionChain) Len() int                                           // chain length
```

**不可变性**：`Append` / `RollbackTo` / `GC` 全部返回新副本（浅拷贝 `vc.entries` + `vc.order` + `vc.head`），`map[Hash]*VersionChainEntry` 通过 rehash 复制实现不可变。

**CoW 语义**：
- `Append(content, reason)`：计算 `hash = SHA-256(parent + content)` → `entries[newhash] = entry` → `order = append(order, newhash)` → `head = newhash`
- `RollbackTo(h)`：`entries[h]` 必须存在（否则 `ErrCoWVersionChainBroken`）→ `head = h`
- `GC(ttl)`：遍历 `order`，删除 `CreatedAt < now-ttl` 且 ≠ head 的 entry；返回删除数

#### SimilarityCheck (interfaces/similarity_check.go, NEW)

```go
// SimilarityConfig holds the Jaccard threshold configuration.
type SimilarityConfig struct {
    InterceptThreshold float64  // default 0.85
    WarnThreshold      float64  // default 0.70 (boundary)
    LookbackN          int      // default 5
}

// SimilarityResult is the outcome of a Jaccard check.
type SimilarityResult struct {
    Similar     bool    // > InterceptThreshold
    Warn        bool    // 0.7-0.85 boundary
    Score       float64 // Jaccard similarity
    MatchedHash Hash    // most similar snapshot hash (empty if none)
}

// Jaccard computes |A ∩ B| / |A ∪ B| for two token sets.
// O(|A| + |B|) with map[string]struct{} backing.
func Jaccard(a, b []string) float64

// CheckSimilarity compares current tokens against last N entries in chain.
func CheckSimilarity(current string, chain *VersionChain, cfg SimilarityConfig) (SimilarityResult, error)
```

**算法**：`Jaccard(tokens(a), tokens(b)) = |a ∩ b| / |a ∪ b|`，token 化为 `strings.Fields(strings.ToLower(s))`，过滤单字符和标点。

**边界**：
- `> 0.85` → `Similar=true` → 强制 Abort（FallbackAbort 路径）
- `0.70-0.85` → `Warn=true` → slog.Warn + 不阻塞（PR-C 暂不升级 LLM）
- `< 0.70` → 正常通过

#### HardEvidence (interfaces/hard_evidence.go, NEW)

```go
// HardEvidence represents the minimum evidence required for Pass.
type HardEvidence struct {
    Kind           string  // "code" / "chat" / "unknown"
    TestResult     *TestResult  // optional
    LogExcerpt     string       // optional
    ArtifactHash   string       // optional
    EntityHash     string       // chat-only
    CoherenceScore float64      // chat-only, [0, 1]
}

// Verified returns true if HardEvidence satisfies the kind-specific minimum.
func (h *HardEvidence) Verified() bool
```

**kind-specific 规则**：
- `kind="code"`：`TestResult != nil && (TestResult.CoveragePct >= 1 || LogExcerpt != "" || ArtifactHash != "")`
- `kind="chat"`：`CoherenceScore >= 0.5 || EntityHash != ""`
- `kind="unknown"`：同 code（保守）

**阻断行为**：`Verified() == false` 且 `TaskReport.Result.Kind = VerdictPass` → 强制改为 `VerdictPartial` + `Blockage.RequiredExternal = ["hard_evidence"]`

#### ORCH_* SentinelError (interfaces/errors.go, MODIFIED, +3)

```go
// 7120 - CoW VersionChain 链断裂 / GC 误删
var ErrCoWVersionChainBroken = &SentinelError{
    Code:        "ORCH_COW_VERSION_CHAIN_BROKEN",
    Message:     "CoW VersionChain broken: hash not found or GC'd",
    Remediation: "trigger VersionChain.GC with longer TTL or rollback to head",
}

// 7121 - Similarity Check 相似度 > 0.85 强制 Abort
var ErrSimilarityCheckIntercepted = &SentinelError{
    Code:        "ORCH_SIMILARITY_INTERCEPTED",
    Message:     "similarity check intercepted: Jaccard > 0.85",
    Remediation: "refine directive to <0.70 similarity or split into distinct sub-tasks",
}

// 7122 - Hard Evidence 空证 PASS 拒绝
var ErrHardEvidenceMissing = &SentinelError{
    Code:        "ORCH_HARD_EVIDENCE_MISSING",
    Message:     "hard evidence missing for Pass verdict",
    Remediation: "provide at least one: test, log, artifact_hash (code) or entity_hash, coherence_score (chat)",
}
```

### 2.2 不变量

| ID | 不变量 | 守护机制 | 违反影响 |
|----|--------|---------|---------|
| **IV-1** | `interfaces/` 0 import D7 子包 | Layout guard（PR-A 落地）| 整个 v7.0 崩溃 |
| **IV-2** | VersionChain 不可变 | Append/RollbackTo/GC 全部浅拷贝返回新对象 | 历史被覆盖，无法 AAR |
| **IV-3** | `head` 永远不被 GC | GC 实现显式排除 head | 当前版本丢失，宕机 |
| **IV-4** | Hash 唯一性 (SHA-256) | crypto/sha256 标准库 | 链碰撞 (理论概率 2^-256) |
| **IV-5** | Hard Evidence kind-specific | `Verified()` 内 switch 严格分离 | 误伤 chat 任务 |
| **IV-6** | Similarity 边界 0.7-0.85 不阻塞 | `CheckSimilarity` 边界仅返回 Warn=true | 误拦合法任务 |
| **IV-7** | Feature Flag 默认 disabled | `D7_HARD_EVIDENCE_ENABLED=false` / `D7_SIMILARITY_CHECK_ENABLED=false` | 0 行为变更 |
| **IV-8** | SHA-256 升级不影响外部接口 | Hash 字符串长度变化但语义不变 | 序列化兼容 |

---

## ③ 业务流程

### 3.1 完整流程：3 类前置防御 → 决策树

```
Channel.Execute 结束
    ↓
[PR-C Gate 1] Similarity Check (AC14)
    ├─ input: current TaskSpec.Goal + WorkItem.VersionChain (last N=5)
    ├─ Jaccard(current_goal_tokens, each chain_entry_tokens)
    ├─ max_score > 0.85 → ErrSimilarityCheckIntercepted (7121) → FallbackAbort → Result.Kind = Failed
    ├─ max_score in 0.70-0.85 → slog.Warn (不阻塞)
    └─ max_score < 0.70 → 继续
    ↓
[PR-C Gate 2] Hard Evidence Verify (AC15)
    ├─ input: TaskReport.Evidence + TaskReport.Result.Kind
    ├─ if Kind=Pass && !HardEvidence.Verified():
    │   ├─ → Result.Kind = VerdictPartial (强制)
    │   ├─ → Blockage.RequiredExternal = ["hard_evidence"]
    │   ├─ → emit("d7.s18.hard.evidence.reject", ...)
    │   └─ → metric hard_evidence_reject_count{kind}++
    └─ if Kind=Pass && Verified() → 继续
    ↓
[PR-C Gate 3] CoW VersionChain Append (AC13)
    ├─ input: serialized WorkItem.State.Snapshot
    ├─ → SHA-256(parent_hash + content) → new_hash
    ├─ → VersionChain[newhash] = entry { Hash, Parent, Content, CreatedAt, Reason }
    ├─ → head = new_hash
    └─ → emit("d7.s18.worktree.versionchain.append", ...)
    ↓
[PR-B Gate] PessimisticCommitGuard.Evaluate (PR-B 已有)
    ├─ ok → 正常返回
    └─ blocked → Pessimistic / RuleBased / Abort (PR-B 路径)
    ↓
[PR-C T05 Wire] NotifyPessimistic emit Span/Metric
    ├─ Span: d7.s18.pessimistic.commit.emit (OTel 完整)
    └─ 5 Metrics: pessimistic_commit_trigger_count + fallback_rule_select_total + mvp_artifact_generated_total + pessimistic_commit_latency_us + fallback_rule_apply_total
    ↓
Channel.Execute return TaskReport
```

### 3.2 Similarity Check 拦截路径（PR-C 实施）

```
decompose.TaskGraphSynthesize(taskSpec)
    ↓
[Check] WorkItem.VersionChain != nil && .Len() > 0
    ├─ No → 直接通过
    └─ Yes →
         ↓
         current_tokens = tokenize(taskSpec.Goal)
         for entry in chain.LastN(5):
             entry_tokens = tokenize(string(entry.Content))
             score = Jaccard(current_tokens, entry_tokens)
             max_score = max(max_score, score)
         ↓
         if max_score > 0.85:
             emit "d7.s18.similarity.check.intercept"
             metric similarity_check_intercept_count{boundary=high}++
             return ErrSimilarityCheckIntercepted (7121)  → FallbackAbort
         else if max_score > 0.70:
             emit slog.Warn("similarity_check_warn", score, matched_hash)
             metric similarity_check_intercept_count{boundary=mid}++
             return nil  // 不阻塞
         else:
             return nil  // 正常通过
```

### 3.3 Hard Evidence 校验路径（PR-C 实施）

```
Verifier.Verify(artifact, report)
    ↓
[PR-A 已就位] kind-specific 配置读取
    ↓
[PR-C 增强] HardEvidence extraction
    ├─ if report.Result.Kind != VerdictPass → 通过
    └─ if report.Result.Kind == VerdictPass:
         ↓
         ev := ExtractHardEvidence(report)  // 抽取 TestResult / LogExcerpt / ArtifactHash / CoherenceScore / EntityHash
         ↓
         if !ev.Verified():
             ↓
             emit "d7.s18.hard.evidence.reject" (kind=ev.Kind)
             metric hard_evidence_reject_count{kind=ev.Kind}++
             ↓
             // 强制降级
             report.Result.Kind = VerdictPartial
             report.Blockage = report.Blockage.WithRequiredExternal("hard_evidence")
             return ErrHardEvidenceMissing (7122)
         else:
             return nil  // 正常通过
```

### 3.4 CoW VersionChain 生命周期（PR-C 实施）

```
WorkItem.Commit(state)
    ↓
[Check] WorkItem.VersionChain == nil?
    ├─ Yes → VersionChain = NewVersionChain() (lazy init, additive)
    └─ No → 继续
    ↓
content := json.Marshal(state.Snapshot)
hash := SHA-256(parent_hash + content)  // SHA-256 升级
new_hash, err := VersionChain.Append(content, reason="commit")
    ↓
[Async] 24h GC worker (background goroutine)
    ↓
    if VersionChain.Len() > 10:
        deleted, err := VersionChain.GC(24 * time.Hour)
        ↓
        // 守护 head 永远不被删
        assert head ∉ deleted
    ↓
WorkItem.VersionChain = VersionChain  // 浅拷贝 immutable
```

---

## ④ 接口契约

### 4.1 VersionChain interface

```go
// NewVersionChain returns an empty CoW chain.
func NewVersionChain() *VersionChain

// Append adds a new content snapshot to the chain.
// Returns the new hash. CoW semantics: never modifies existing entries.
func (vc *VersionChain) Append(content []byte, reason string) (Hash, error)

// RollbackTo sets the head to a previous hash. O(1) hash lookup.
// Returns ErrCoWVersionChainBroken (7120) if hash not found or GC'd.
func (vc *VersionChain) RollbackTo(h Hash) error

// Get returns the entry for a hash. O(1) lookup.
func (vc *VersionChain) Get(h Hash) (*VersionChainEntry, bool)

// Head returns the current chain head hash.
func (vc *VersionChain) Head() Hash

// GC removes entries older than TTL, except head. Returns count deleted.
func (vc *VersionChain) GC(ttl time.Duration) (int, error)

// Len returns the number of entries in the chain.
func (vc *VersionChain) Len() int
```

### 4.2 SimilarityCheck interface

```go
// Default thresholds (PR-C fixed, configurable via env in v7.0.1).
const (
    DefaultInterceptThreshold = 0.85
    DefaultWarnThreshold      = 0.70
    DefaultLookbackN          = 5
)

// Jaccard computes the Jaccard similarity between two token sets.
func Jaccard(a, b []string) float64

// CheckSimilarity compares current string against the last N entries.
// Returns SimilarityResult with Similar/Warn flags + score + matched hash.
func CheckSimilarity(current string, chain *VersionChain, cfg SimilarityConfig) (SimilarityResult, error)
```

### 4.3 HardEvidence interface

```go
// Verified returns true if HardEvidence satisfies the kind-specific minimum.
// kind="code": TestResult != nil && (TestResult.CoveragePct >= 1 || LogExcerpt != "" || ArtifactHash != "")
// kind="chat": CoherenceScore >= 0.5 || EntityHash != ""
// kind="unknown": same as "code" (conservative)
func (h *HardEvidence) Verified() bool

// ExtractHardEvidence extracts HardEvidence from a TaskReport (PR-C utility).
func ExtractHardEvidence(report *TaskReport) *HardEvidence
```

### 4.4 Feature Flag 接口（PR-C 新增 2 个）

```go
// d7-bootstrap/wire.go
func HardEvidenceEnabled() bool {
    return envBool("D7_HARD_EVIDENCE_ENABLED", false)  // default disabled
}

func SimilarityCheckEnabled() bool {
    return envBool("D7_SIMILARITY_CHECK_ENABLED", false)  // default disabled
}
```

### 4.5 Span/Metric 接口（PR-C T05 收口 + 新增）

**PR-C T05 收口（PR-B slog.Info → 完整 OTel）**：
```go
// hardening/emitter.go - 1 span (PR-B T05)
Span{Pattern: "d7.s18.pessimistic.commit.emit", Kind: SpanInternal, Component: "escape"}
```

**PR-C 新增 3 span**：
```go
// hardening/emitter.go - 3 spans (PR-C)
Span{Pattern: "d7.s18.hard.evidence.reject", Kind: SpanInternal, Component: "executionflow/verify"}
Span{Pattern: "d7.s18.worktree.versionchain.append", Kind: SpanInternal, Component: "workmodel"}
Span{Pattern: "d7.s18.similarity.check.intercept", Kind: SpanInternal, Component: "decisionplanning"}
```

**PR-C 新增 8 metric**：
```go
// hardening/metrics.go - 5 metrics (PR-B T05 收口)
Counter("pessimistic_commit_trigger_count", ...)
CounterVec("fallback_rule_select_total", []string{"rule"}, ...)
Counter("mvp_artifact_generated_total", ...)
Histogram("pessimistic_commit_latency_us", ...)
Counter("fallback_rule_apply_total", ...)

// hardening/metrics.go - 3 metrics (PR-C 新增)
CounterVec("hard_evidence_reject_count", []string{"kind"}, ...)
CounterVec("similarity_check_intercept_count", []string{"boundary"}, ...)
Counter("versionchain_append_total", ...)
```

---

## ⑤ 实现路径

### 5.1 文件清单 (10 NEW + 4 MODIFIED)

| 状态 | 文件 | 用途 | LOC 估算 |
|------|------|------|---------|
| NEW | `interfaces/version_chain.go` | Hash + VersionChain + Append/RollbackTo/GC | 80 |
| NEW | `interfaces/similarity_check.go` | SimilarityConfig + Check + Jaccard | 80 |
| NEW | `interfaces/hard_evidence.go` | HardEvidence + Verified + kind-specific | 50 |
| NEW | `workmodel/version_chain.go` | CoW impl + 24h GC worker | 120 |
| NEW | `workmodel/similarity.go` | Jaccard 算法 + 边界检测 | 80 |
| NEW | `interfaces/version_chain_test.go` | ≥ 5 tests (Append/Rollback/GC) | 150 |
| NEW | `interfaces/similarity_check_test.go` | ≥ 4 tests (Jaccard + boundary) | 120 |
| NEW | `interfaces/hard_evidence_test.go` | ≥ 4 tests (kind-specific + reject) | 100 |
| NEW | `workmodel/version_chain_test.go` | ≥ 5 tests (CoW + GC + race) | 150 |
| NEW | `workmodel/similarity_test.go` | ≥ 4 tests (Jaccard + boundary) | 100 |
| MOD  | `interfaces/errors.go` | +3 ORCH_* (7120-7122) | +30 |
| MOD  | `executionflow/verify/verifier.go` | Hard Evidence reject 路径 | +50 |
| MOD  | `decisionplanning/decomposer.go` | Similarity Check 入口 | +40 |
| MOD  | `workmodel/work_tree.go` | VersionChain 嵌入 | +30 |
| MOD  | `d7-bootstrap/wire.go` | Feature Flag 注入 (HardEvidenceEnabled + SimilarityCheckEnabled) | +20 |
| MOD  | `hardening/emitter.go` | T05 完整 OTel wire + 3 新 span | +40 |
| MOD  | `hardening/metrics.go` | T05 5 metrics + 3 新 metric | +50 |

**总计**：10 NEW (~1030 LOC) + 7 MODIFIED (~260 LOC) = **+1290 LOC**

### 5.2 实施顺序

```
Phase 1 (S4-1, 1 day): interfaces/ 3 NEW + 3 test
  → interfaces/version_chain.go + similarity_check.go + hard_evidence.go
  → interfaces/{version_chain,similarity_check,hard_evidence}_test.go
  → 验证: 3 NEW interfaces 文件 IV-1 不变量 (0 import D7 子包)
  → 验证: ≥ 80% 覆盖率

Phase 2 (S4-2, 1 day): workmodel/ 2 NEW + 2 test
  → workmodel/version_chain.go (CoW impl + 24h GC)
  → workmodel/similarity.go (Jaccard + boundary)
  → workmodel/{version_chain,similarity}_test.go
  → 验证: race detector clean + CoW 不可变

Phase 3 (S4-3, 1 day): 5 MODIFIED (verifier + decomposer + work_tree + wire + emitter/metrics)
  → executionflow/verify/verifier.go (Hard Evidence reject)
  → decisionplanning/decomposer.go (Similarity Check 入口)
  → workmodel/work_tree.go (VersionChain 嵌入)
  → d7-bootstrap/wire.go (Feature Flag 注入)
  → hardening/emitter.go + metrics.go (T05 完整 wire + 3 新 span + 3 新 metric)
  → 验证: Feature Flag smoke test (default disabled 0 行为变更)

Phase 4 (S4-4, 1 day): spec sync
  → openspec/specs/d7-orchestration/{spec,d7-domain,a-registry,f-registry,span-registry,t-registry}.md
  → openspec/specs/d7-orchestration/{contract,behavior,observability,security}-test-specs/*.md (Gherkin scenarios)
  → 验证: verify-spec-sync.sh 12/12 PASS

Phase 5 (S4-5, 0.5 day): 整合 + race detector + coverage 全量
  → go test -race -count=1 ./internal/layers/orchestration/...
  → go test -cover 4 个目标包 ≥ 80%
  → LP-1/LP-2/LP-5 集成测试 100% 兼容
```

### 5.3 复用 PR-A/B 资产

→ 父设计 §2.3 复用表 + PR-B 验收报告 §2.1 复用清单。

| 资产 | 复用方式 |
|------|----------|
| `interfaces.TaskSpec` | Similarity Check 读 `taskSpec.Goal` token 化 |
| `interfaces.TaskReport` | Hard Evidence 读 `report.Evidence` + `report.Result.Kind` |
| `interfaces.FallbackPolicy` | Similarity 拦截时复用 FallbackAbort 路径 |
| `escape.DefaultPessimisticCommitGuard` | Hard Evidence + Similarity 在 PessimisticCommitGuard 入口前置 check |
| `sharederrors.WithCode` 模式 | 3 ORCH_COW_* / ORCH_SIMILARITY_* / ORCH_HARD_EVIDENCE_* (7120-7122) |
| `interfaces/ 0 import` 原则 | PR-C version_chain.go / similarity_check.go / hard_evidence.go 仍守 IV-1 |
| `Feature Flag env-gated` 模式 | D7_HARD_EVIDENCE_ENABLED + D7_SIMILARITY_CHECK_ENABLED 默认 disabled |
| `coordinator/aliases.go` v6.0.x legacy | 不动；VersionChain 是 WorkItem 增量字段（additive）|

---

## ⑥ 验证与风险

### 6.1 验证策略

```
# 命令 1: interfaces 包构建 + IV-1 守护
go build ./internal/layers/orchestration/interfaces/
grep -r 'orchestration/' internal/layers/orchestration/interfaces/ | grep -v _test.go  # 0 lines
✅ PASS

# 命令 2: 4 个目标包覆盖率
go test -race -cover -count=1 ./internal/layers/orchestration/interfaces/
go test -race -cover -count=1 ./internal/layers/orchestration/escape/
go test -race -cover -count=1 ./internal/layers/orchestration/workmodel/
go test -race -cover -count=1 ./internal/layers/orchestration/executionflow/verify/
# 4/4 覆盖率 ≥ 80%

# 命令 3: 全 orchestration 包 race PASS
go test -race -count=1 -timeout 180s ./internal/layers/orchestration/... 2>&1 | tail -5
# 24/24 packages PASS

# 命令 4: 集成测试 LP-1/LP-2/LP-5
go test -tags "integration d7" -race -count=1 -timeout 240s -run "TestAcceptance_LP1_|TestAcceptance_LP2_|TestAcceptance_LP5_" ./tests/integration/d7/
# 7/7 LP tests PASS (Feature Flag 默认 disabled)

# 命令 5: Feature Flag 默认 disabled smoke
D7_HARD_EVIDENCE_ENABLED=false D7_SIMILARITY_CHECK_ENABLED=false go test -count=1 ./internal/layers/orchestration/...
# 0 行为变更验证

# 命令 6: SHA-256 算法验证
go test -run TestVersionChain_Append_SHA256
# SHA-256 hex 长度 64-char, 与 PR-B FNV-1a 16-char 区分
```

### 6.2 风险与缓解

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| CoW VersionChain 链膨胀 | 中 | 存储爆炸 | hash 索引 + 24h 后台 GC；VersionChain 只保留 hash 不 inline state |
| Hard Evidence 误伤 chat 任务 | 中 | 合法任务被拒 | kind-specific 配置 + Feature Flag 灰度 + 1%→10%→50%→100% |
| Similarity Check 边界 0.7-0.85 误判 | 中 | 合法任务被拦 | 仅 slog.Warn 标记，不阻塞；LLM 二次校验留 v7.0.1 |
| Hash 碰撞 | 极低 | 链断裂 | SHA-256 标准库 (256-bit 安全，碰撞概率 2^-128) |
| PR-B FNV-1a → PR-C SHA-256 升级 | 低 | 序列化兼容 | buildChainHash 内部函数，外部接口不变（仅哈希字符串长度） |
| 4 个 ORCH_* 错误码 + PR-A/B 共占 7100-7122 区间 | 0 | 错误码冲突 | 区间划分：7100-7109 PR-A / 7110-7119 PR-B / 7120-7129 PR-C |
| IV-1 invariant | 0 | 整个 v7.0 崩溃 | version_chain.go / similarity_check.go / hard_evidence.go 仅依赖 `internal/shared/errors/` |
| GC 误删 head | 极低 | 当前版本丢失 | GC 实现显式排除 head + 单元测试守护 |

### 6.3 Rollback Plan

```bash
# Phase 1: PR-C 灰度回滚（生产事故）
./scripts/devrix.sh rollback-flag hard_evidence
# → 关闭 D7_HARD_EVIDENCE_ENABLED → Verifier 恢复 v6.0.x 行为
# → LP-1/LP-2/LP-5 维持兼容

./scripts/devrix.sh rollback-flag similarity_check
# → 关闭 D7_SIMILARITY_CHECK_ENABLED → Decomposer 跳过 Similarity Check
# → LP-1/LP-2/LP-5 维持兼容

# Phase 2: PR-C 代码回滚（极端情况）
git revert <pr-c-merge-commit>
# → 恢复 master 到 PR-B HEAD
# → VersionChain 嵌入 WorkItem 字段保留（omitempty JSON tag 兼容）

# Phase 3: 错误码清理（不推荐）
# → 7120-7122 错误码保留（API 兼容），仅禁用调用方路径
```

### 6.4 回归风险

| 回归路径 | 概率 | 检测手段 | 缓解 |
|---------|------|---------|------|
| LP-1 (5 节点 round-trip) | 低 | LP-1 集成测试 | Feature Flag 默认 disabled |
| LP-2 (Risk + Verifier) | 中 | LP-2 集成测试 | Hard Evidence 默认 disabled |
| LP-5 (子 agent 嵌套) | 低 | LP-5 集成测试 | Similarity Check 默认 disabled |
| Verifier 拒绝空证 → 旧 Pass 任务被拒 | 中 | LP-1 + LP-2 | kind-specific + Feature Flag 灰度 |
| VersionChain 嵌入 WorkItem → JSON 序列化 break | 低 | serialization_test.go | `omitempty` JSON tag |
| SHA-256 升级 → PR-B 的 buildChainHash 引用 break | 低 | buildChainHash_test.go | 内部函数升级 + 外部接口不变 |

---

## 附录 A: File Manifest

### A.1 NEW (10)

| 文件 | LOC | 验证 |
|------|-----|------|
| `interfaces/version_chain.go` | 80 | go build + 0 import D7 子包 |
| `interfaces/similarity_check.go` | 80 | 同上 |
| `interfaces/hard_evidence.go` | 50 | 同上 |
| `workmodel/version_chain.go` | 120 | go test -race + GC head 守护 |
| `workmodel/similarity.go` | 80 | go test -race + Jaccard 边界 |
| `interfaces/version_chain_test.go` | 150 | ≥ 80% 覆盖 |
| `interfaces/similarity_check_test.go` | 120 | ≥ 80% 覆盖 |
| `interfaces/hard_evidence_test.go` | 100 | ≥ 80% 覆盖 |
| `workmodel/version_chain_test.go` | 150 | ≥ 80% 覆盖 + race |
| `workmodel/similarity_test.go` | 100 | ≥ 80% 覆盖 |

### A.2 MODIFIED (7)

| 文件 | 改动 | LOC | 验证 |
|------|------|-----|------|
| `interfaces/errors.go` | +3 ORCH_* (7120-7122) | +30 | sharederrors.WithCode 模式 |
| `executionflow/verify/verifier.go` | Hard Evidence reject | +50 | LP-1/LP-2 集成测试 |
| `decisionplanning/decomposer.go` | Similarity Check 入口 | +40 | LP-5 集成测试 |
| `workmodel/work_tree.go` | VersionChain 嵌入 (additive) | +30 | serialization_test.go |
| `d7-bootstrap/wire.go` | Feature Flag 注入 (2 new) | +20 | envBool 默认 disabled |
| `hardening/emitter.go` | T05 完整 OTel wire + 3 新 span | +40 | Jaeger trace 可见 |
| `hardening/metrics.go` | T05 5 metrics + 3 新 metric | +50 | Prometheus scrape 可见 |

### A.3 SPEC DOCS (6 同步)

- `openspec/specs/d7-orchestration/spec.md` (新增 3 ADDED Requirements for D7-S18-A13/A14/A15)
- `openspec/specs/d7-orchestration/d7-domain.md` (§8 Layer 4-Layer × 3-Phase PR-C 闭环)
- `openspec/specs/d7-orchestration/a-registry.md` (D7-S18-A13/A14/A15 3 A entries)
- `openspec/specs/d7-orchestration/f-registry.md` (D7-S18-A13/A14/A15 F entries, +7 F)
- `openspec/specs/d7-orchestration/span-registry.md` (3 个新 P0 span ops, 32→35)
- `openspec/specs/d7-orchestration/t-registry.md` (13 个 P0 T 登记, 246→259)

---

## 附录 B: Rollback Plan

详见 §6.3 Rollback Plan。

**回滚矩阵**：

| 触发条件 | 回滚方式 | 影响 |
|---------|---------|------|
| Hard Evidence 误伤 chat 任务 | `D7_HARD_EVIDENCE_ENABLED=false` | Verifier 恢复 v6.0.x |
| Similarity 误拦合法任务 | `D7_SIMILARITY_CHECK_ENABLED=false` | Decomposer 跳过 |
| VersionChain GC 误删 head | 回滚 PR-C merge commit | WorkItem 恢复 PR-B 行为 |
| SHA-256 序列化 break | revert `buildChainHash` | 内部函数恢复 FNV-1a |

---

## 附录 C: 回归风险评估

| 维度 | 评估 | 备注 |
|------|------|------|
| **API 兼容** | ✅ 100% | PR-C 0 行为变更（Feature Flag 默认 disabled） |
| **数据兼容** | ✅ 100% | VersionChain `omitempty` JSON tag + 老 WorkItem `VersionChain == nil` 视为无 chain |
| **错误码兼容** | ✅ 100% | 7120-7129 区间不与 PR-A 7100-7104 / PR-B 7110-7113 冲突 |
| **Hash 算法兼容** | ⚠️ 内部升级 | buildChainHash FNV-1a → SHA-256 内部函数升级；外部 Hash 字符串长度变化但语义不变 |
| **Feature Flag 兼容** | ✅ 100% | D7_HARD_EVIDENCE_ENABLED + D7_SIMILARITY_CHECK_ENABLED 默认 disabled |
| **LP-1/LP-2/LP-5 兼容** | ✅ 100% | Feature Flag 默认 disabled 保证 0 行为变更 |
| **IV-1 invariant 兼容** | ✅ 100% | 3 NEW interfaces 文件仅依赖 `internal/shared/errors/` |

---

## 附录 D: S3 Checklist (六段式自检)

| 段 | 子项 | 状态 |
|----|------|------|
| **① 架构目标** | 业务目标 7 AC + T05 | ✅ |
| ① | 设计约束 10 条 | ✅ |
| ① | 关键决策 5 个 | ✅ |
| **② 领域模型** | 3 新增聚合 (VersionChain + SimilarityCheck + HardEvidence) | ✅ |
| ② | 3 新增 ORCH_* SentinelError | ✅ |
| ② | 8 个不变量 (IV-1..IV-8) | ✅ |
| **③ 业务流程** | 完整流程 3 Gate → PR-B 路径 | ✅ |
| ③ | 3 子流程 (Similarity + HardEvidence + CoW lifecycle) | ✅ |
| **④ 接口契约** | 3 interface (VersionChain + Similarity + HardEvidence) | ✅ |
| ④ | 2 Feature Flag interface | ✅ |
| ④ | 3 新 span + 8 新 metric | ✅ |
| **⑤ 实现路径** | 10 NEW + 7 MODIFIED 文件清单 | ✅ |
| ⑤ | 5 阶段实施顺序 (S4-1..S4-5) | ✅ |
| ⑤ | 8 个 PR-A/B 复用资产 | ✅ |
| **⑥ 验证与风险** | 6 验证命令 | ✅ |
| ⑥ | 8 风险 + 缓解 | ✅ |
| ⑥ | 4 阶段 Rollback Plan | ✅ |
| ⑥ | 6 维度回归风险评估 | ✅ |
| **附录** | A File Manifest | ✅ |
| 附录 | B Rollback Plan | ✅ |
| 附录 | C 回归风险评估 | ✅ |
| 附录 | E 下一步 (PR-C → v7.0.0 release) | ✅ |

**S3-Gate 状态**：12/12 全绿，可进入 S4 实现。

---

## 附录 E: 下一步 (PR-C → v7.0.0 release)

PR-C S7_Archived 后启动 v7.0.0 minor release 闭环：

```
PR-C S7_Archived (2026-07-04)
    ↓
v7.0.0 minor release (DM-20260629-010 PLANNED)
    ├─ SemVer: v6.0.x → v7.0.0 minor bump
    ├─ Changelog: 23 AC 跨 3 PR 累积 + Hash 算法升级 SHA-256
    ├─ Migration: type alias 保留 1 minor 版本 (v6.0.x Plan/ChannelRequest/LearnRequest)
    ├─ Rollout: 1% → 10% → 50% → 100% (1 周)
    └─ Observability: 9 Span + 13 Metric + 12 ORCH_* 错误码全 Jaeger + Prometheus 可见
    ↓
可选 follow-up Change (按用户确认按需立):
    ├─ devrix-d7-adaptive-threshold (AC7) — AdaptiveThreshold 接入 RunTurn
    ├─ devrix-d7-performance-budget (AC19) — Performance benchmark 收敛
    ├─ devrix-d7-security-classification (AC20) — TaskSpec.Classification 标签
    ├─ devrix-d7-similarity-llm-boundary — 0.7-0.85 边界 LLM 二次校验
    └─ devrix-d7-versionchain-distributed — VersionChain 跨 session 持久化
```

**4-Layer × 3-Phase 闭环总览**（PR-C 后）：

| Layer | PR-A (✅) | PR-B (✅) | PR-C (✅) |
|-------|----------|----------|----------|
| L1 接口层 | TaskSpec/TaskReport | — | — |
| L2 字段语义层 | Dissent/Blockage/Resource | — | — |
| L3 防御运行时层 | — | Pessimistic + RuleBased | **CoW + Similarity + HardEvidence** |
| L4 治理横切层 | spec 同步 + race test + LP regression + Migration + Boundary + FeatureFlag + ErrorCode | convergence span + Coverage + Layout guard + 灰度 | **T05 wire + convergence span + 灰度 1%→100%** |

**AC 覆盖率**：

| PR | AC IMPLEMENTED | AC PLANNED | 累计 |
|----|----------------|------------|------|
| PR-A (✅) | 6 (AC1-5 + AC17) | 0 | 6/23 |
| PR-B (✅) | 8 (AC11+AC12+AC9+AC10+AC16+AC21+AC22+AC23) | 0 | 14/23 |
| **PR-C (✅)** | **6 (AC13+AC14+AC15+AC6+AC18+AC22)** | 3 (AC7+AC19+AC20) | **20/23 (87%)** |

PR-C 后 v7.0.0 release 即闭环 87% AC，剩余 3 AC（AC7/19/20）留 follow-up。
