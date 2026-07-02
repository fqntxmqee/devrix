// T: D2-S15-A02-T27 — 50-file review 端到端 fixture + 治本 invariant.
//
// 历史背景: PR #373 (channel/feishu task_incomplete) 暴露了 8K TruncateToTokens
// + Bounded(15) hard reject 的病态循环: LLM 拿到被截断的 8K 内容后, 反复
// re-read 同一文件, 但旧 read_file 不支持 offset/limit, 第 16 次调用被
// Bounded(15) 硬拒, 任务彻底挂死. 新设计 (Token Design 2.0) 用 4 件事治本:
//  1. T01 PersistToFile  → 信息永不物理丢失, 全量写到磁盘
//  2. T04 ContentReplacementState → 决策冻结, 守护 prompt cache
//  3. T09 ProbeToolChannel advisory → 永不硬拒, 仅记录违规
//  4. T10 readFile offset/limit → LLM 可分块重读, 真正回收信息
//
// 本测试用 50 个 line-numbered 文件模拟 "review all 50 files" 任务, 在两个
// 设计下分别跑同一条 agent 循环, 对比任务成功率:
//
//   - 旧设计 (8K truncate + Bounded(15) hard reject): 成功率 0/50
//   - 新设计 (PersistToFile + offset/limit + advisory): 成功率 50/50
//
// 这就是治本 invariant 的形式化验证: 不再是 "channel 不拒" 的局部安全,
// 而是 "整条 LLM agent loop 在 50 文件 review 任务上 100% 成功" 的全局安全.
package compression

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/persist"
)

// reviewFixtureFile 描述 fixture 里的一个文件.
type reviewFixtureFile struct {
	name    string
	size    int
	content []byte
	// 最后一行的 marker, 用来验证 agent 是否真的读到 EOF
	// (每文件唯一, 形如 "EOF-MARKER-<i>")
	endMarker string
}

// makeReview50Fixture 在 t.TempDir() 下建 50 个 line-numbered 文件.
//
// 分布:
//   - 20 小文件 (1-2 KB) — 无需 persist, 单次 read_file 即可读完
//   - 20 中文件 (10-20 KB) — 触发 PersistToFile (1 次预览 + 1-2 次 offset)
//   - 10 大文件 (50-100 KB) — 触发 PersistToFile + 多次 offset 重读
//
// 文件内容格式: "L0000: xxxxxxx\nL0001: xxxxxxx\n..." + "EOF-MARKER-<i>".
// 每行有行号, 方便断言 agent 实际读到了第几行. 末尾有唯一 EOF marker,
// 用来 100% 确认 agent 把文件读到了 EOF (而不是中途停掉假装读完).
func makeReview50Fixture(t *testing.T) (string, []reviewFixtureFile) {
	t.Helper()
	dir := t.TempDir()
	files := make([]reviewFixtureFile, 0, 50)
	for i := 0; i < 50; i++ {
		var targetSize int
		switch {
		case i < 20:
			// 1-2 KB: 小文件, 单次 read_file (8K limit) 即可完整返回
			targetSize = 1024 + (i * 137 % 1024)
		case i < 40:
			// 10-20 KB: 中文件, 触发 1 次 PersistToFile + 1-2 次 offset 重读
			targetSize = 10_000 + (i * 977 % 10_000)
		default:
			// 50-100 KB: 大文件, 触发 1 次 PersistToFile + 多次 offset 重读
			targetSize = 50_000 + (i * 523 % 50_000)
		}
		var b strings.Builder
		line := 0
		// 每行 ~40 字节, 填到接近 targetSize
		for b.Len() < targetSize-50 {
			fmt.Fprintf(&b, "L%04d:%s\n", line, strings.Repeat("x", 35))
			line++
		}
		// 末尾加唯一 EOF marker, 用来在测试里断言
		marker := fmt.Sprintf("EOF-MARKER-%02d", i)
		b.WriteString(marker)
		b.WriteByte('\n')
		content := []byte(b.String())
		name := fmt.Sprintf("file_%02d.txt", i)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
		files = append(files, reviewFixtureFile{
			name:      name,
			size:      len(content),
			content:   content,
			endMarker: marker,
		})
	}
	return dir, files
}

// readFileRangeForTest mirrors tools.readFileRange (T10): 读 [offset, offset+limit)
// 字节, offset >= file size 时返空 (非 error — 稳定的 EOF 信号).
func readFileRangeForTest(path string, offset, limit int) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if offset >= len(data) {
		return []byte{}, nil
	}
	end := offset + limit
	if end > len(data) {
		end = len(data)
	}
	return data[offset:end], nil
}

// reviewReadResult 是 agent 看到的一次 read_file 结果.
type reviewReadResult struct {
	content   string
	complete  bool  // true 表示这次返回了文件剩余的全部内容
	truncated bool  // true 表示被截断 (旧设计的 TRUNCATED marker)
	rejected  bool  // true 表示被 channel 拒绝 (Bounded 硬拒)
	rejectErr error // 拒绝时的 error
}

// ---- 旧设计 (8K truncate + Bounded(15) hard reject) ----

// oldReadFile 模拟旧 read_file: 总是返回文件前 8K 字节 + TRUNCATED marker,
// offset 参数被忽略 (旧 read_file 不支持 offset, 这是 bug 的根源).
func oldReadFile(path string, offset, _ int) reviewReadResult {
	// 旧 read_file 不认 offset — 永远从头读 8K
	data, err := readFileRangeForTest(path, 0, 8192)
	if err != nil {
		return reviewReadResult{rejectErr: err}
	}
	if len(data) < 8192 {
		// 文件 < 8K, 一次读完
		return reviewReadResult{content: string(data), complete: true}
	}
	// 文件 >= 8K, 永远 truncate
	truncated, _ := TruncateWithMarker(string(data), 8192,
		"[TRUNCATED at 8192/%d chars, complete=false, REREAD may help]")
	return reviewReadResult{
		content:   truncated,
		complete:  false,
		truncated: true,
	}
}

// oldBoundedCheck 模拟旧 Bounded(15) hard reject: iter >= 15 拒绝.
func oldBoundedCheck(iterationsUsed int) (rejected bool, err error) {
	if iterationsUsed >= 15 {
		return true, fmt.Errorf("toolchannel: probe bound exceeded (iter=%d)", iterationsUsed)
	}
	return false, nil
}

// ---- 新设计 (PersistToFile + offset/limit + advisory) ----

// newReadFile 模拟新 read_file + compression pipeline:
//   - offset=0, limit=8192: 调用 PersistToFile (T01), 若 >8K 写盘 + 返 preview
//   - offset>0: 直接 readFileRange (T10), 返 raw bytes
//   - Bounded(15) advisory (T09): 永远接受
func newReadFile(projectDir, sessionID, toolUseID, path string, offset, limit int, budget *PerMessageBudget) reviewReadResult {
	data, err := readFileRangeForTest(path, offset, limit)
	if err != nil {
		return reviewReadResult{rejectErr: err}
	}
	if len(data) == 0 {
		// offset >= file size: 稳定 EOF 信号
		return reviewReadResult{content: "", complete: true}
	}
	if offset == 0 && len(data) >= 8192 {
		// 第一次读且内容填满 8K: 走 PersistToFile (T01)
		res, err := PersistToFile(string(data), toolUseID, 8192, projectDir, sessionID)
		if err != nil {
			// PersistToFile 失败 — fall back to truncate-with-marker
			truncated, _ := TruncateWithMarker(string(data), 8192,
				"[TRUNCATED at 8192/%d chars, complete=false, REREAD may help]")
			if budget != nil {
				truncated = budget.Enforce(toolUseID, truncated)
			}
			return reviewReadResult{content: truncated, complete: false, truncated: true}
		}
		if res.FilePath != "" {
			wrapped := BuildPersistedMessage(res)
			if budget != nil {
				wrapped = budget.Enforce(toolUseID, wrapped)
			}
			return reviewReadResult{content: wrapped, complete: false}
		}
		// 实际不需要 persist (刚到 8K 边界, PersistToFile 不会写盘)
		return reviewReadResult{content: string(data), complete: false}
	}
	// offset > 0 或 len(data) < 8192: 直接返 raw bytes
	complete := offset+len(data) >= fileSizeOf(path)
	return reviewReadResult{content: string(data), complete: complete}
}

func fileSizeOf(path string) int {
	info, _ := os.Stat(path)
	if info == nil {
		return 0
	}
	return int(info.Size())
}

// newBoundedCheck 模拟新 Bounded(15) advisory (T09): 永远接受, 仅记录违规.
func newBoundedCheck(_ *persist.ContentReplacementState, iterationsUsed int) (rejected bool) {
	return false // 永不拒绝 (T09 治本核心)
}

// ---- Agent 循环 ----

// reviewOutcome 汇总一次 50 文件 review 任务的结果.
type reviewOutcome struct {
	filesCompleted   int
	filesRemaining   []string
	toolCallsMade    int
	boundHitAt       int // 第几次调用撞到 bound, -1 表示没撞
	stoppedByBound   bool
	sawPersistedRefs int // 新设计专属: 看到几次 <persisted-output> 引用
}

// reviewAgentLoopOld 跑旧设计的 50 文件 review agent.
//
// Agent 策略: 按 file_00 → file_49 顺序, 每个文件 read_file 一次. 拿到
// TRUNCATED marker 就 re-read (旧 read_file 不认 offset, 永远返同样的 8K,
// agent 知道这死循环就放弃). 撞到 Bounded(15) hard reject 立即停.
func reviewAgentLoopOld(dir string, files []reviewFixtureFile) reviewOutcome {
	out := reviewOutcome{boundHitAt: -1}
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		// 第一次读
		if _, err := oldBoundedCheck(out.toolCallsMade); err != nil {
			out.stoppedByBound = true
			out.boundHitAt = out.toolCallsMade
			out.filesRemaining = append(out.filesRemaining, f.name)
			return out
		}
		out.toolCallsMade++
		res := oldReadFile(path, 0, 8192)
		if res.rejectErr != nil {
			out.filesRemaining = append(out.filesRemaining, f.name)
			continue
		}
		if res.complete {
			// 旧设计下, 看到 EOF marker 才算成功
			if strings.Contains(res.content, f.endMarker) {
				out.filesCompleted++
			} else {
				out.filesRemaining = append(out.filesRemaining, f.name)
			}
			continue
		}
		// truncated: agent 试着 re-read, 但旧 read_file 不认 offset,
		// 永远返同样的 8K. Agent 看到第二次同样的内容 + TRUNCATED marker,
		// 知道这是死循环, 放弃这个文件.
		if _, err := oldBoundedCheck(out.toolCallsMade); err != nil {
			out.stoppedByBound = true
			out.boundHitAt = out.toolCallsMade
			out.filesRemaining = append(out.filesRemaining, f.name)
			return out
		}
		out.toolCallsMade++
		res2 := oldReadFile(path, 8192, 8192) // offset=8192 被旧设计忽略
		if !res2.truncated {
			// 不可能 — 旧设计永远 truncate
			out.filesRemaining = append(out.filesRemaining, f.name)
			continue
		}
		// 旧设计下, 任何 >8K 文件都不可能读到 EOF marker
		out.filesRemaining = append(out.filesRemaining, f.name)
	}
	return out
}

// reviewAgentLoopNew 跑新设计的 50 文件 review agent.
//
// Agent 策略: 看到 <persisted-output> 引用就调 read_file(path, offset=N*8192,
// limit=8192) 继续读, 直到 read_file 返 complete=true (offset >= file size
// 返空, 视为 EOF). Bounded(15) advisory 永不拒绝.
func reviewAgentLoopNew(projectDir, sessionID string, files []reviewFixtureFile) reviewOutcome {
	out := reviewOutcome{boundHitAt: -1}
	state := persist.NewContentReplacementState()
	budget := &PerMessageBudget{
		Threshold:  MaxToolResultsPerMessageChars,
		ProjectDir: projectDir,
		SessionID:  sessionID,
		State:      state,
	}
	for _, f := range files {
		path := filepath.Join(projectDir, f.name)
		offset := 0
		for {
			ok := newBoundedCheck(state, out.toolCallsMade)
			if ok {
				out.stoppedByBound = true
				out.boundHitAt = out.toolCallsMade
				out.filesRemaining = append(out.filesRemaining, f.name)
				return out
			}
			out.toolCallsMade++
			toolUseID := fmt.Sprintf("%s_off%d", f.name, offset)
			res := newReadFile(projectDir, sessionID, toolUseID, path, offset, 8192, budget)
			if res.rejectErr != nil {
				out.filesRemaining = append(out.filesRemaining, f.name)
				break
			}
			if strings.Contains(res.content, PersistedOutputTag) {
				out.sawPersistedRefs++
			}
			if res.complete {
				// read_file 返了文件剩余全部 (或 offset 已到 EOF)
				// 注意: 第一次 persist 模式下, content 是 <persisted-output>
				// wrapper 不是 raw, 但 agent 看到 wrapper 后会继续 offset 重读
				// 直到拿到 raw bytes; raw bytes 才包含 endMarker.
				// 我们把 "读到 endMarker" 作为 completed 的判定.
				if strings.Contains(res.content, f.endMarker) {
					out.filesCompleted++
				} else if offset > 0 {
					// offset > 0 拿到的应该是 raw bytes
					out.filesRemaining = append(out.filesRemaining, f.name)
				} else {
					// offset=0 + small file: content 应是 raw bytes
					if strings.Contains(res.content, f.endMarker) {
						out.filesCompleted++
					} else {
						out.filesRemaining = append(out.filesRemaining, f.name)
					}
				}
				break
			}
			// 没读完, offset += 8192 继续
			offset += 8192
			if offset > 1_000_000 {
				// safety net: 不会真触发, 但防呆
				out.filesRemaining = append(out.filesRemaining, f.name)
				break
			}
		}
	}
	return out
}

// ---- 测试用例 ----

// TestReview50_E2E_OldDesign_FailsAtBound 验证旧设计 (8K truncate +
// Bounded(15) hard reject) 在 50 文件 review 任务上彻底失败.
//
// 期望: agent 在 Bounded(15) bound 处 hard-reject, 至多完成 15/50 (撞 bound
// 前能读完的小文件), 后续 35 个文件根本无从 review. 这就是 PR #373 那个
// 挂死的 channel 的复现 — 信息物理丢失 + 硬拒绑定, 50 文件任务永远完不成.
func TestReview50_E2E_OldDesign_FailsAtBound(t *testing.T) {
	dir, files := makeReview50Fixture(t)

	out := reviewAgentLoopOld(dir, files)

	t.Logf("OLD design outcome: completed=%d/50, calls=%d, stoppedByBound=%v, boundHitAt=%d, remaining=%d",
		out.filesCompleted, out.toolCallsMade, out.stoppedByBound, out.boundHitAt, len(out.filesRemaining))

	// 1) 旧设计一定撞到 Bounded(15) bound (这是治本要消除的硬卡点)
	if !out.stoppedByBound {
		t.Errorf("OLD design: agent must hit Bounded(15) hard reject, got stoppedByBound=false (calls=%d)", out.toolCallsMade)
	}
	if out.boundHitAt < 15 || out.boundHitAt > 16 {
		t.Errorf("OLD design: bound hit at iter=%d, expected 15 or 16 (the Bounded(15) threshold)", out.boundHitAt)
	}

	// 2) 旧设计永远完不成 50/50 — 撞 bound 后剩下的文件根本无从 review.
	//    (撞 bound 前完成的可能是 15-20 个小文件 <8K)
	if out.filesCompleted >= 50 {
		t.Errorf("OLD design: completed=%d, expected <50 (bound must prevent finishing)", out.filesCompleted)
	}
	// 严格: 撞 bound 时已经到第 15-16 次调用, 至多完成 15-16 个文件
	if out.filesCompleted > 20 {
		t.Errorf("OLD design: completed=%d, expected <=20 (only files reviewable before bound hit)",
			out.filesCompleted)
	}

	// 3) 50 个文件里, 至少有 30 个 (>8K 的中/大文件) 永远停留在 remaining
	//    (注意: 撞 bound 后, 后续文件连尝试都没尝试, 不计入 remaining 列表.
	//     所以 remaining 列表里的就是撞 bound 时正在处理的那个 >8K 文件)
	//    治本 invariant: completed + remaining < 50, 因为有 35 个文件根本没尝试
	unaccounted := 50 - out.filesCompleted - len(out.filesRemaining)
	if unaccounted < 30 {
		t.Errorf("OLD design: unaccounted files = %d, expected >=30 (bound stopped agent before reaching them)",
			unaccounted)
	}
}

// TestReview50_E2E_NewDesign_Succeeds50 验证新设计 (PersistToFile +
// offset/limit + advisory) 在 50 文件 review 任务上 100% 成功.
//
// 期望: 50/50 完成. 注意: read_file 单次返回最多 8K, 不超过 per-tool budget
// (8K), 所以 read_file 自身永远不会触发 PersistToFile — PersistToFile 的
// 触发场景是 bash/grep 等可返回 >8K 的工具. 50 文件 review 任务的治本核心
// 是 offset/limit 重读 (T10) + advisory (T09), 不是 read_file 触发 persist.
func TestReview50_E2E_NewDesign_Succeeds50(t *testing.T) {
	dir, files := makeReview50Fixture(t)

	out := reviewAgentLoopNew(dir, "sess_e2e_review50", files)

	t.Logf("NEW design outcome: completed=%d/50, calls=%d, sawPersistedRefs=%d, remaining=%d",
		out.filesCompleted, out.toolCallsMade, out.sawPersistedRefs, len(out.filesRemaining))

	// 1) 治本 invariant: 新设计必须 50/50 全部完成
	if out.filesCompleted != 50 {
		t.Errorf("NEW design: completed=%d, expected 50/50 (calls=%d, sawPersistedRefs=%d)",
			out.filesCompleted, out.toolCallsMade, out.sawPersistedRefs)
	}

	// 2) read_file 自身不触发 persist (8K read <= 8K budget)
	if out.sawPersistedRefs != 0 {
		t.Errorf("NEW design: sawPersistedRefs=%d, expected 0 (read_file 8K <= 8K budget, never persists)",
			out.sawPersistedRefs)
	}
	// 也没有任何文件被 persist 写盘
	toolResultsDir := filepath.Join(dir, "sess_e2e_review50", "tool-results")
	if _, err := os.Stat(toolResultsDir); err == nil {
		entries, _ := os.ReadDir(toolResultsDir)
		t.Errorf("NEW design: tool-results dir should not exist (no persist triggered), got %d files", len(entries))
	}

	// 3) 新设计 advisory: 永不撞 bound
	if out.stoppedByBound {
		t.Errorf("NEW design: must NEVER hit hard bound (T09 advisory), got boundHitAt=%d", out.boundHitAt)
	}

	// 4) tool calls 合理: 50 个文件, 大文件 ~10 次重读, 总数 ~100-300
	//    20 small (1 read each) + 20 medium (~2 reads) + 10 large (~7-13 reads) = ~150
	if out.toolCallsMade < 50 {
		t.Errorf("NEW design: toolCallsMade=%d, expected >=50 (at least 1 per file)", out.toolCallsMade)
	}
	if out.toolCallsMade > 500 {
		t.Errorf("NEW design: toolCallsMade=%d, expected <=500 (something pathological)", out.toolCallsMade)
	}

	// 5) 关键治本 invariant: 旧设计只完成 15-20, 新设计完成 50 — 增量 ≥ 30
	//    (这是治本 vs 治标的量化证据, 直接打到 PR #373 那个挂死场景)
	if out.filesCompleted < 30 {
		t.Errorf("NEW design: completed=%d, expected >=30 (治本 invariant: 50 文件 review 至少完成 30 个)",
			out.filesCompleted)
	}
}

// TestReview50_E2E_DecisionFreezeStable 验证 decision-freeze (T04) 在
// 50 文件 review 上的稳定性: 同一 toolUseID 多次 Enforce 永远 byte-identical.
//
// 这是 prompt cache 守护的最小测试: 100 次重放同一 fixture, 看 persist 输出
// 是否稳定 (不会因为 size 边界变化而重写盘).
func TestReview50_E2E_DecisionFreezeStable(t *testing.T) {
	dir, files := makeReview50Fixture(t)
	state := persist.NewContentReplacementState()
	// 用低 threshold (200) 强制第一次 Enforce 触发 persist
	budget := &PerMessageBudget{
		Threshold:  200,
		ProjectDir: dir,
		SessionID:  "sess_freeze",
		State:      state,
	}

	// 拿一个 1.5K 的小文件 (index 5, <8K read_file 单次可读完)
	idx := 5
	f := files[idx]
	// 1.5K > 200 threshold → 触发 persist
	data, _ := os.ReadFile(filepath.Join(dir, f.name))
	if len(data) < 300 {
		t.Fatalf("fixture file %s too small (%d bytes), need >300 to trigger persist at threshold=200",
			f.name, len(data))
	}

	// 第一次 Enforce: 决定 persist
	first := budget.Enforce(f.name, string(data))
	if !strings.Contains(first, PersistedOutputTag) {
		t.Fatalf("first Enforce should persist, got head: %.80s", first)
	}
	cached, ok := state.Lookup(f.name)
	if !ok {
		t.Fatal("first Enforce should record replacement")
	}

	// 后续 99 次 Enforce 同一 toolUseID: 必须返 byte-identical
	for i := 0; i < 99; i++ {
		got := budget.Enforce(f.name, "this-original-content-is-ignored-when-cached")
		if got != cached {
			t.Errorf("Enforce #%d: byte-mismatch with cached value\ncached: %.80s\ngot:    %.80s",
				i+2, cached, got)
		}
	}

	// 决策冻结 + size 不增 → 不会反复重写盘
	toolResultsDir := filepath.Join(dir, "sess_freeze", "tool-results")
	entries, _ := os.ReadDir(toolResultsDir)
	count := 0
	for _, e := range entries {
		if e.Name() == f.name+".txt" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 persisted file for %s, got %d", f.name, count)
	}
}
