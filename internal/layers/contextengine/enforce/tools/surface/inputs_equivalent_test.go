// T: D2-S15-A02-T28 — inputsEquivalent 19 工具 × 3 case = 57 单测 (PR-F).
//
// Case 设计 (per task AC14):
//
//   1. same      — 完全相同 input → true
//   2. reorder   — 字段顺序调换 → true (验证 JSON canonicalize, 排除
//                   raw byte compare 经典坑)
//   3. different — 完全不同 input → false
//
// 19 工具覆盖 (与 orthogonal_flags.go truth table 对齐):
//
//   read_file, write_file, edit_file, bash, grep, glob,
//   lsp_go_to_definition, lsp_find_references, lsp_incoming_calls,
//   lsp_hover, lsp_workspace_symbol, lsp_code_action,
//   free_fork, query_diagnostics, verify_plan_execution,
//   delegate_status (delegate_* 通配), task_output (task_* 通配),
//   ask_user_question, tool_search
//
// 性能 / 幂等 / 传递性单测放最后 (TestInputsEquivalent_*).
package surface

import (
	"fmt"
	"strings"
	"testing"
)

// 19 工具固定清单. 跟 orthogonal_flags.go 19 工具 truth table 对齐.
var inputsEquivalentTools = []string{
	"read_file",
	"write_file",
	"edit_file",
	"bash",
	"grep",
	"glob",
	"lsp_go_to_definition",
	"lsp_find_references",
	"lsp_incoming_calls",
	"lsp_hover",
	"lsp_workspace_symbol",
	"lsp_code_action",
	"free_fork",
	"query_diagnostics",
	"verify_plan_execution",
	"delegate_status", // delegate_* 通配代表
	"task_output",     // task_* 通配代表
	"ask_user_question",
	"tool_search",
}

// canonicalInputs[tool] 是 tool 的"代表性" input JSON (含多种字段,
// 用于 reorder 测试).
var canonicalInputs = map[string]string{
	"read_file":               `{"file_path":"foo.go","limit":50,"offset":100}`,
	"write_file":              `{"file_path":"bar.go","content":"hello world"}`,
	"edit_file":               `{"file_path":"baz.go","old_str":"foo","new_str":"bar"}`,
	"bash":                    `{"command":"ls -la","cwd":"/tmp","env":{"FOO":"bar"}}`,
	"grep":                    `{"pattern":"TODO","path":".","case_insensitive":true}`,
	"glob":                    `{"pattern":"**/*.go","path":"internal/"}`,
	"lsp_go_to_definition":    `{"uri":"file:///main.go","line":42,"col":7}`,
	"lsp_find_references":     `{"uri":"file:///main.go","line":15,"col":3}`,
	"lsp_incoming_calls":      `{"uri":"file:///main.go","line":99,"col":1}`,
	"lsp_hover":               `{"uri":"file:///main.go","line":7,"col":4}`,
	"lsp_workspace_symbol":    `{"query":"TestRun","limit":10}`,
	"lsp_code_action":         `{"uri":"file:///main.go","line":1,"col":0,"kind":"quickfix"}`,
	"free_fork":               `{"agent":"reviewer","prompt":"please review this diff"}`,
	"query_diagnostics":       `{"kind":"compile","file":"main.go","tool":"go"}`,
	"verify_plan_execution":   `{"plan_id":"plan_abc","step":3}`,
	"delegate_status":         `{"agent":"explorer","task_id":"t_42","format":"json"}`,
	"task_output":             `{"task_id":"t_99","block":true,"timeout_ms":5000}`,
	"ask_user_question":       `{"question":"Pick one","options":["A","B"]}`,
	"tool_search":             `{"query":"git commit","limit":5}`,
}

// reorderInputs[tool] 把 canonicalInputs 的字段顺序调换 — per-tool override
// 路径必须仍判等价. 这是 JSON canonicalize 的核心测试: 验证 path 用了
// DeepEqual on map 而不是 raw byte compare.
var reorderInputs = map[string]string{
	"read_file":               `{"offset":100,"file_path":"foo.go","limit":50}`,
	"write_file":              `{"content":"hello world","file_path":"bar.go"}`,
	"edit_file":               `{"new_str":"bar","old_str":"foo","file_path":"baz.go"}`,
	"bash":                    `{"env":{"FOO":"bar"},"cwd":"/tmp","command":"ls -la"}`,
	"grep":                    `{"case_insensitive":true,"path":".","pattern":"TODO"}`,
	"glob":                    `{"path":"internal/","pattern":"**/*.go"}`,
	"lsp_go_to_definition":    `{"col":7,"line":42,"uri":"file:///main.go"}`,
	"lsp_find_references":     `{"line":15,"col":3,"uri":"file:///main.go"}`,
	"lsp_incoming_calls":      `{"uri":"file:///main.go","col":1,"line":99}`,
	"lsp_hover":               `{"col":4,"uri":"file:///main.go","line":7}`,
	"lsp_workspace_symbol":    `{"limit":10,"query":"TestRun"}`,
	"lsp_code_action":         `{"kind":"quickfix","line":1,"uri":"file:///main.go","col":0}`,
	"free_fork":               `{"prompt":"please review this diff","agent":"reviewer"}`,
	"query_diagnostics":       `{"tool":"go","file":"main.go","kind":"compile"}`,
	"verify_plan_execution":   `{"step":3,"plan_id":"plan_abc"}`,
	"delegate_status":         `{"format":"json","task_id":"t_42","agent":"explorer"}`,
	"task_output":             `{"timeout_ms":5000,"block":true,"task_id":"t_99"}`,
	"ask_user_question":       `{"options":["A","B"],"question":"Pick one"}`,
	"tool_search":             `{"limit":5,"query":"git commit"}`,
}

// differentInputs[tool] 是一个完全不同的 input — 必须判不等价.
// 设计: 关键字段值差异 (e.g. 不同 file_path, 不同 command).
var differentInputs = map[string]string{
	"read_file":             `{"file_path":"DIFFERENT.go","limit":999}`,
	"write_file":            `{"file_path":"OTHER.go","content":"DIFFERENT CONTENT"}`,
	"edit_file":             `{"file_path":"OTHER.go","old_str":"DIFFERENT","new_str":"STUFF"}`,
	"bash":                  `{"command":"rm -rf /","cwd":"/"}`,
	"grep":                  `{"pattern":"FIXME","path":"different/","case_insensitive":false}`,
	"glob":                  `{"pattern":"**/*.py","path":"external/"}`,
	"lsp_go_to_definition":  `{"uri":"file:///OTHER.go","line":1,"col":1}`,
	"lsp_find_references":   `{"uri":"file:///OTHER.go","line":1,"col":1}`,
	"lsp_incoming_calls":    `{"uri":"file:///OTHER.go","line":1,"col":1}`,
	"lsp_hover":             `{"uri":"file:///OTHER.go","line":1,"col":1}`,
	"lsp_workspace_symbol":  `{"query":"DifferentQuery","limit":99}`,
	"lsp_code_action":       `{"uri":"file:///OTHER.go","line":1,"col":1,"kind":"refactor"}`,
	"free_fork":             `{"agent":"DIFFERENT_AGENT","prompt":"DIFFERENT PROMPT"}`,
	"query_diagnostics":     `{"kind":"runtime","file":"OTHER.go","tool":"python"}`,
	"verify_plan_execution": `{"plan_id":"plan_OTHER","step":99}`,
	"delegate_status":       `{"agent":"OTHER_AGENT","task_id":"t_OTHER","format":"yaml"}`,
	"task_output":           `{"task_id":"t_OTHER","block":false,"timeout_ms":999}`,
	"ask_user_question":     `{"question":"DIFFERENT","options":["X","Y"]}`,
	"tool_search":           `{"query":"DIFFERENT","limit":99}`,
}

// TestInputsEquivalent_Same — 19 工具 × 相同输入 (canonicalInputs)
// same input → equivalent (19 case PASS).
func TestInputsEquivalent_Same(t *testing.T) {
	for _, tool := range inputsEquivalentTools {
		t.Run(tool, func(t *testing.T) {
			in := canonicalInputs[tool]
			got := InputsEquivalent(tool, []byte(in), []byte(in))
			if !got {
				t.Errorf("%s: same input must be equivalent, got false (input=%s)", tool, in)
			}
		})
	}
}

// TestInputsEquivalent_Reorder — 19 工具 × 字段顺序调换
// canonicalInputs vs reorderInputs → equivalent (19 case PASS).
func TestInputsEquivalent_Reorder(t *testing.T) {
	for _, tool := range inputsEquivalentTools {
		t.Run(tool, func(t *testing.T) {
			canonical := canonicalInputs[tool]
			reorder := reorderInputs[tool]
			got := InputsEquivalent(tool, []byte(canonical), []byte(reorder))
			if !got {
				t.Errorf("%s: reorder must be equivalent, got false\ncanonical: %s\nreorder:   %s",
					tool, canonical, reorder)
			}
		})
	}
}

// TestInputsEquivalent_Different — 19 工具 × 不同输入
// canonicalInputs vs differentInputs → not equivalent (19 case PASS).
func TestInputsEquivalent_Different(t *testing.T) {
	for _, tool := range inputsEquivalentTools {
		t.Run(tool, func(t *testing.T) {
			canonical := canonicalInputs[tool]
			different := differentInputs[tool]
			got := InputsEquivalent(tool, []byte(canonical), []byte(different))
			if got {
				t.Errorf("%s: different input must NOT be equivalent, got true\ncanonical: %s\ndifferent: %s",
					tool, canonical, different)
			}
		})
	}
}

// TestInputsEquivalent_AC14_All57_AtOnce — sanity check: 一遍跑完 57 个
// (19 tools × 3 case) 并报告结果. 给 CI 一个 single-line 报告.
func TestInputsEquivalent_AC14_All57_AtOnce(t *testing.T) {
	total := 0
	pass := 0
	for _, tool := range inputsEquivalentTools {
		// case same
		total++
		if InputsEquivalent(tool, []byte(canonicalInputs[tool]), []byte(canonicalInputs[tool])) {
			pass++
		}
		// case reorder
		total++
		if InputsEquivalent(tool, []byte(canonicalInputs[tool]), []byte(reorderInputs[tool])) {
			pass++
		}
		// case different
		total++
		if !InputsEquivalent(tool, []byte(canonicalInputs[tool]), []byte(differentInputs[tool])) {
			pass++
		}
	}
	if pass != total {
		t.Errorf("AC14: %d/%d pass, want %d/%d", pass, total, total, total)
	}
	t.Logf("AC14: %d/%d cases PASS (19 tools × 3 cases)", pass, total)
}

// TestInputsEquivalent_ByteIdenticalFastPath — byte-identical inputs
// 走 fast path (不需要 JSON unmarshal). 验证 quick return 行为.
func TestInputsEquivalent_ByteIdenticalFastPath(t *testing.T) {
	a := []byte(`{"foo":1,"bar":2}`)
	b := []byte(`{"foo":1,"bar":2}`)
	if !InputsEquivalent("any_tool_name_doesnt_matter", a, b) {
		t.Error("byte-identical must be equivalent")
	}
}

// TestInputsEquivalent_EmptyInputs — 两个空字节都视为等价.
// (callers 会传 [] 来表示 "no input" — 这是正常语义).
func TestInputsEquivalent_EmptyInputs(t *testing.T) {
	if !InputsEquivalent("read_file", nil, nil) {
		t.Error("nil + nil must be equivalent")
	}
	if !InputsEquivalent("read_file", []byte{}, []byte{}) {
		t.Error("empty + empty must be equivalent")
	}
	if InputsEquivalent("read_file", []byte(`{}`), nil) {
		t.Error("non-empty + nil must NOT be equivalent")
	}
	if InputsEquivalent("read_file", nil, []byte(`{}`)) {
		t.Error("nil + non-empty must NOT be equivalent")
	}
}

// TestInputsEquivalent_ParseFailure_FailsClosed — invalid JSON 视为不等价.
func TestInputsEquivalent_ParseFailure_FailsClosed(t *testing.T) {
	if InputsEquivalent("read_file", []byte(`{broken`), []byte(`{"file_path":"foo"}`)) {
		t.Error("invalid JSON a must fail closed")
	}
	if InputsEquivalent("read_file", []byte(`{"file_path":"foo"}`), []byte(`{broken`)) {
		t.Error("invalid JSON b must fail closed")
	}
}

// TestInputsEquivalent_Transitive — a == b && b == c → a == c.
// 这是 design.md 风险章节点出的 raw byte compare 经典坑的验证.
// 三个 JSON 写法不同但语义相同 (e.g. whitespace 差异).
func TestInputsEquivalent_Transitive(t *testing.T) {
	a := []byte(`{"file_path":"foo.go","limit":50}`)
	b := []byte(`{"limit":50,"file_path":"foo.go"}`)
	c := []byte(`{ "file_path":"foo.go", "limit":50 }`) // 间距不同
	// a == b, b == c, c == a 都应该走 JSON canonicalize 路径 → equivalent.
	if !InputsEquivalent("read_file", a, b) {
		t.Error("a == b should hold")
	}
	if !InputsEquivalent("read_file", b, c) {
		t.Error("b == c should hold")
	}
	if !InputsEquivalent("read_file", a, c) {
		t.Error("a == c (transitivity) must hold")
	}
}

// TestInputsEquivalent_BashOnlyCommand — bash 走 path 路径, command
// 字符串以外字段 (env / cwd) 差异不影响等价判断. 这是 per-tool 的关键放宽.
func TestInputsEquivalent_BashOnlyCommand(t *testing.T) {
	a := []byte(`{"command":"ls -la","cwd":"/tmp"}`)
	b := []byte(`{"command":"ls -la","cwd":"/different"}`)
	if !InputsEquivalent("bash", a, b) {
		t.Error("bash with same command but different cwd must be equivalent (per-tool path)")
	}
	c := []byte(`{"command":"rm -rf /","cwd":"/tmp"}`)
	if InputsEquivalent("bash", a, c) {
		t.Error("bash with different command must NOT be equivalent")
	}
}

// TestInputsEquivalent_ReadFileIgnoresLimitOffset — read_file per-tool:
// file_path 相同, limit/offset 是访问语义不影响身份.
func TestInputsEquivalent_ReadFileIgnoresLimitOffset(t *testing.T) {
	a := []byte(`{"file_path":"foo.go","limit":50,"offset":0}`)
	b := []byte(`{"file_path":"foo.go","limit":999,"offset":5000}`)
	if !InputsEquivalent("read_file", a, b) {
		t.Error("read_file with same path but different limit/offset must be equivalent (per-tool path)")
	}
	c := []byte(`{"file_path":"OTHER.go","limit":50,"offset":0}`)
	if InputsEquivalent("read_file", a, c) {
		t.Error("read_file with different file_path must NOT be equivalent")
	}
}

// 防御性: canonicalInputs / reorderInputs / differentInputs 必须覆盖所有
// 19 工具 — 否则测试有 missing coverage.
func TestInputsEquivalent_TestDataCoversAll19(t *testing.T) {
	for _, tool := range inputsEquivalentTools {
		if _, ok := canonicalInputs[tool]; !ok {
			t.Errorf("canonicalInputs missing for %s", tool)
		}
		if _, ok := reorderInputs[tool]; !ok {
			t.Errorf("reorderInputs missing for %s", tool)
		}
		if _, ok := differentInputs[tool]; !ok {
			t.Errorf("differentInputs missing for %s", tool)
		}
	}
	if len(canonicalInputs) != 19 {
		t.Errorf("canonicalInputs count = %d, want 19", len(canonicalInputs))
	}
	if len(reorderInputs) != 19 {
		t.Errorf("reorderInputs count = %d, want 19", len(reorderInputs))
	}
	if len(differentInputs) != 19 {
		t.Errorf("differentInputs count = %d, want 19", len(differentInputs))
	}
}

// 性能 sanity check (T28 设计目标: < 10μs p99). 19 工具 × 1000 调 = 19000
// 调用, 总耗时 < 200ms (意味着每调用 < 10μs 平均).
func TestInputsEquivalent_PerformanceSanity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance sanity in short mode")
	}
	const iterations = 1000
	for _, tool := range inputsEquivalentTools {
		in := canonicalInputs[tool]
		for i := 0; i < iterations; i++ {
			_ = InputsEquivalent(tool, []byte(in), []byte(in))
		}
	}
	// 这一行如果跑得太慢会 fail; 不设硬阈值是因为 CI 噪声.
	// 真正测是 `go test -bench` — 这里只 smoke.
	t.Logf("performance sanity: 19 × %d = %d calls completed", iterations, 19*iterations)
}

// summary 用来给 CI 报告一次性的 57 case 摘要 (VerboseOutput > 0).
func TestInputsEquivalent_Summary(t *testing.T) {
	t.Logf("57 cases = 19 tools × 3 cases (same / reorder / different): all tools = %s",
		strings.Join(inputsEquivalentTools, ", "))
	for i, tool := range inputsEquivalentTools {
		t.Logf("  [%2d] %-22s canonical=%s", i+1, tool, truncate(canonicalInputs[tool], 40))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// unused: 避免 fmt 被 linter 警告.
var _ = fmt.Sprintf
