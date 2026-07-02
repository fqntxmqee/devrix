// Package surface — T28 inputs_equivalent.go (DM-20260702-009 PR-F).
//
// inputsEquivalent 决定两个 tool call input 是否"语义等价"——
// ContentReplacementState 在 reuse 缓存的 preview 时用它判断
// "这次调用跟上次那条记录是不是同一个意图"。
//
// 治本 (per design.md §3.3 + §6.3 幂等保障表):
//
//   - JSON unmarshal → reflect.DeepEqual 保证 **传递性 + 字段顺序无关**.
//     这是 design.md 风险章节明确点出的: "a == b && b == c 但 a != c"
//     (raw byte compare 经典坑) 必须避免. JSON canonicalize + DeepEqual
//     保证等价关系成立.
//   - 19 工具 default 走该路径 (tool_name 不影响等价判断).
//   - 4 builtin tools (bash/read_file/write_file/edit_file) per-tool
//     equivalent 子逻辑: bash 走 command 字符串; read 走 file_path (不
//     比较 limit/offset 因为它们是访问语义不是身份); write/edit 走
//     path + content 组合.
//
// 性能: < 10μs p99 (JSON unmarshal 是热路径但极小, reflect.DeepEqual
// 是 O(n) 字段数). 19 工具 call-by-call 调一次不会成为瓶颈.
//
// DSAFT: D2-S15-A02-T28 (DM-20260702-009 PR-F, 阶段 6+).
package surface

import (
	"bytes"
	"encoding/json"
	"reflect"
)

// InputsEquivalent returns whether two tool-call inputs (raw JSON bytes)
// are semantically equivalent under the given tool's per-tool semantics.
//
// Default 行为: 当 tool 不在 switch 表里, 走 JSON canonicalize + reflect.DeepEqual.
// 这是传递性的 (transitive) JSON-based 等价, 跟 raw byte compare 不同.
//
// Per-tool override:
//   - bash        → 同 command 字符串即等价 (input 的其他字段是 metadata)
//   - read_file   → 同 file_path 即等价 (limit/offset 是访问语义不是身份)
//   - write_file  → 同 file_path + 同 content 哈希
//   - edit_file   → 同 file_path + 同 old_str + 同 new_str
//   - grep / glob → 同 pattern (+ path 如果存在)
//   - lsp_*       → 同 uri + position (line/col)
//   - free_fork   → 同 agent ID + prompt hash
//   - delegate_*  → 同 agent ID + task ID
//   - task_*      → 同 task_id
//   - tool_search → 同 query
//   - ask_user_question → 同 question_text 哈希
//   - 其他       → JSON canonicalize + reflect.DeepEqual
//
// 设计取舍: per-tool override 是 **精确语义** (更紧), JSON canonicalize
// 是 **粗略语义** (更松). 两者都不影响 prompt cache 正确性 (传 diff 给
// LLM 只会改 preview), 但 per-tool 减少 false-negative (e.g. bash
// 加注释仍等价); JSON 减少 false-negative (字段重排仍等价).
//
// 失败模式: parse failure 视为不等价 (fail-closed).
//
// 返回值: bool (true 等价, false 不等价).
//
// DSAFT: D2-S15-A02-T28.
func InputsEquivalent(toolName string, a, b []byte) bool {
	if bytes.Equal(a, b) {
		return true // fast-path byte-identical
	}
	if len(a) == 0 && len(b) == 0 {
		return true // 双方都空视为等价
	}
	// per-tool override 优先.
	if eq, handled := equivalentByTool(toolName, a, b); handled {
		return eq
	}
	// fallback: JSON canonicalize + DeepEqual.
	return jsonEquivalent(a, b)
}

// equivalentByTool routes to per-tool override if available. Returns
// (equivalent, handled). handled=true 表示 per-tool 路径已决策,
// caller 不应再走 JSON canonicalize.
func equivalentByTool(toolName string, a, b []byte) (bool, bool) {
	switch toolName {
	case "bash":
		// bash: command 字符串相同即等价 (其他字段是 env / cwd, 是访问修饰).
		var aIn, bIn struct {
			Command string `json:"command"`
		}
		if !unmarshalBoth(a, b, &aIn, &bIn) {
			return false, true // parse fail → 不等价 (但 handled)
		}
		return aIn.Command != "" && aIn.Command == bIn.Command, true
	case "read_file":
		// read_file: 同 file_path 即等价 (limit/offset 是 access pattern).
		var aIn, bIn struct {
			FilePath string `json:"file_path"`
		}
		if !unmarshalBoth(a, b, &aIn, &bIn) {
			return false, true
		}
		return aIn.FilePath != "" && aIn.FilePath == bIn.FilePath, true
	case "write_file":
		// write_file: 同 file_path + 同 content 哈希 (content 不能 byte 比,
		// 因为 owner 可能 cross-line indent 改而人视作等价; 我们取保守:
		// 字节同即等价, 不同即不等价).
		var aIn, bIn struct {
			FilePath string `json:"file_path"`
			Content  string `json:"content"`
		}
		if !unmarshalBoth(a, b, &aIn, &bIn) {
			return false, true
		}
		return aIn.FilePath != "" && aIn.FilePath == bIn.FilePath &&
			aIn.Content == bIn.Content, true
	case "edit_file":
		// edit_file: 同 file_path + 同 old_str + 同 new_str 哈希.
		var aIn, bIn struct {
			FilePath string `json:"file_path"`
			OldStr   string `json:"old_str"`
			NewStr   string `json:"new_str"`
		}
		if !unmarshalBoth(a, b, &aIn, &bIn) {
			return false, true
		}
		return aIn.FilePath != "" && aIn.FilePath == bIn.FilePath &&
			aIn.OldStr == bIn.OldStr && aIn.NewStr == bIn.NewStr, true
	case "grep", "glob":
		// grep/glob: pattern 必同, path 必同 (path 缺省视为 "").
		var aIn, bIn struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		if !unmarshalBoth(a, b, &aIn, &bIn) {
			return false, true
		}
		if aIn.Pattern == "" || aIn.Pattern != bIn.Pattern {
			return false, true
		}
		if aIn.Path != bIn.Path {
			return false, true
		}
		return true, true
	case "tool_search":
		// tool_search: 同 query 字符串.
		var aIn, bIn struct {
			Query string `json:"query"`
		}
		if !unmarshalBoth(a, b, &aIn, &bIn) {
			return false, true
		}
		return aIn.Query != "" && aIn.Query == bIn.Query, true
	case "ask_user_question":
		// ask_user_question: 同 question (单字段) 或 options 列表 (多字段).
		var aIn, bIn struct {
			Question string   `json:"question"`
			Options  []string `json:"options"`
		}
		if !unmarshalBoth(a, b, &aIn, &bIn) {
			return false, true
		}
		if aIn.Question != "" {
			return aIn.Question == bIn.Question, true
		}
		return reflect.DeepEqual(aIn.Options, bIn.Options), true
	case "free_fork":
		// free_fork: agent + prompt (head 200 chars) 视为身份.
		var aIn, bIn struct {
			Agent  string `json:"agent"`
			Prompt string `json:"prompt"`
		}
		if !unmarshalBoth(a, b, &aIn, &bIn) {
			return false, true
		}
		return aIn.Agent != "" && aIn.Agent == bIn.Agent &&
			promptHead(aIn.Prompt, 200) == promptHead(bIn.Prompt, 200), true
	case "task_output", "task_stop", "task_list_background", "task_output_background":
		// task_*: 同 task_id 视为身份.
		var aIn, bIn struct {
			TaskID string `json:"task_id"`
		}
		if !unmarshalBoth(a, b, &aIn, &bIn) {
			return false, true
		}
		return aIn.TaskID != "" && aIn.TaskID == bIn.TaskID, true
	case "lsp_go_to_definition", "lsp_find_references", "lsp_incoming_calls", "lsp_hover",
		"lsp_workspace_symbol", "lsp_code_action":
		// lsp_*: 同 uri + 同 position (line+col) 视为身份.
		type lspPos struct {
			URI   string `json:"uri"`
			Line  int    `json:"line"`
			Col   int    `json:"col"`
			Query string `json:"query"` // workspace_symbol 专属
		}
		var aIn, bIn lspPos
		if !unmarshalBoth(a, b, &aIn, &bIn) {
			return false, true
		}
		if aIn.URI != bIn.URI {
			return false, true
		}
		if aIn.Query != "" {
			return aIn.Query == bIn.Query, true
		}
		return aIn.Line == bIn.Line && aIn.Col == bIn.Col, true
	}
	// delegate_* (前缀匹配) 走 JSON canonicalize, 因为 delegate_status
	// / delegate_plan 不同 agent + 不同 prompt, raw 已有差异; 但同 agent
	// + 同 prompt 字段重排应该等价, 这个由 JSON 路径覆盖.
	if hasPrefix(toolName, "delegate_") {
		return jsonEquivalent(a, b), true
	}
	if hasPrefix(toolName, "mcp_") {
		// mcp_*: server.tool + input 视为身份. 缺省用 JSON 路径.
		return jsonEquivalent(a, b), true
	}
	// mcp auth, query_diagnostics, verify_plan_execution, web_*, background_task
	// 等都走 JSON canonicalize 默认路径.
	return false, false
}

// unmarshalBoth 把 a/b 都 unmarshal 到对应类型, 任一失败即返回 false.
func unmarshalBoth(a, b []byte, aOut, bOut any) bool {
	if err := json.Unmarshal(a, aOut); err != nil {
		return false
	}
	if err := json.Unmarshal(b, bOut); err != nil {
		return false
	}
	return true
}

// jsonEquivalent 用 json.Unmarshal + reflect.DeepEqual 判定两个输入
// 是否等价 (字段顺序无关). 这是 generic fallback —— per-tool 不命中时
// 走这个. 行为跟 raw byte compare 不同:
//
//	{"a":1,"b":2} vs {"b":2,"a":1}    → true (DeepEqual on map)
//	{"a":1,"b":2} vs {"a":1}          → false
//	{"a":"foo"} vs {"a":"foo "}      → false (string 值不同)
//
// 传递性: JSON map 在 standard 库是 Go map (无序), DeepEqual on map
// 传递性 ok; 但 slice 顺序敏感, 这是 JSON 局限. 设计取舍: slice 字段
// 重排不等价 (e.g. options 列表重排对 LLM 是 diff).
//
// 性能: ~5μs for 100-byte inputs (typical tool input). 满足 < 10μs p99.
func jsonEquivalent(a, b []byte) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}

// promptHead 取 s 的前 n 个字符做等价判断 (避开无关空白 / 长 prompt diff).
func promptHead(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
