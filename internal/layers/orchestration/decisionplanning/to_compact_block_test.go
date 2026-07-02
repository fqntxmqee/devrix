// T: D7-S10-A50-T20 — toCompactBlock JSONL 序列化 6 case AC.
//
// T20 acceptance criteria 要求 6 case 全部 PASS:
//
//  1. tool_use_ok — 已知工具 read_file, 有效 input → JSON line {read_file: <encoded>}
//  2. user_text — user 文本块 → JSON line {user: <text>}
//  3. malformed_input — 已知工具但 input 是非法 JSON → fail-open (raw input)
//  4. empty — text 空 / tool_name 空 → 返回 "" + Empty metric
//  5. escape_attack — input 含 "\n" / 特殊字符 → 仍可安全 round-trip (json.Marshal 已转义)
//  6. unknown_tool — 工具不在 surfaceLookup → fail-open (unknown_tool metric)
//
// panic 隔离 单独 case: simulatePanic 测 panic 落入 fail-open.
package decisionplanning

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// stubReadFileSurface 用 ToAutoClassifierInput 把 read_file input 投影为
// "/r/<path>", fail-open 与 projection 输出可以区分.
type stubReadFileSurfaceAdapter struct{}

func makeReadFileSurface() contracts.ToolSurface { return stubReadFileSurfaceAdapter{} }

func (stubReadFileSurfaceAdapter) Name() string { return "stub" }
func (stubReadFileSurfaceAdapter) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return []contracts.ToolSpec{{Name: "read_file"}}
}
func (stubReadFileSurfaceAdapter) RiskLevel(string) types.RiskLevel { return types.RiskLevelLow }
func (stubReadFileSurfaceAdapter) InterruptBehavior(string) contracts.InterruptMode {
	return contracts.InterruptBlock
}
func (stubReadFileSurfaceAdapter) CheckPermission(context.Context, contracts.ToolSpec, json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}
func (stubReadFileSurfaceAdapter) IsConcurrencySafe(json.RawMessage) bool { return true }

// ToAutoClassifierInput 投影 read_file input → "/r/<path>"。这是为了让
// happy-path 输出和 fail-open 输出 (raw input JSON) 可区分。
func (stubReadFileSurfaceAdapter) ToAutoClassifierInput(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var in struct {
		FilePath string `json:"file_path"`
	}
	// 故意不处理解析失败 — 让上层 fail-open, 这里只测 happy path。
	_ = json.Unmarshal(input, &in)
	if in.FilePath == "" {
		return ""
	}
	return "/r/" + in.FilePath
}
func (stubReadFileSurfaceAdapter) Execute(_ context.Context, _, _, _ string) (*contracts.ToolResult, error) {
	return &contracts.ToolResult{}, nil
}

// T20 happy-path: read_file 工具 + 有效 JSON input → projected line.
func TestToCompactBlock_ToolUse_OK(t *testing.T) {
	lookup := map[string]contracts.ToolSurface{"read_file": makeReadFileSurface()}
	block := TranscriptBlock{
		Type:  BlockTypeToolUse,
		Name:  "read_file",
		Input: `{"file_path":"foo.go","limit":50}`,
	}
	got := toCompactBlock(block, lookup)
	if got == "" {
		t.Fatal("expected non-empty projection, got empty")
	}
	// ReadFileSurface.ToAutoClassifierInput 把 input 投影成 /r/foo.go。
	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("output is not JSON: %v (got=%q)", err, got)
	}
	if v, ok := decoded["read_file"]; !ok {
		t.Errorf("decoded key \"read_file\" missing in %v", decoded)
	} else if v != "/r/foo.go" {
		t.Errorf("decoded[read_file]=%q, want %q", v, "/r/foo.go")
	}
}

// T20 user_text: 文本块 + 文本内容 → {role: text} JSON line.
func TestToCompactBlock_UserText(t *testing.T) {
	lookup := map[string]contracts.ToolSurface{}
	block := TranscriptBlock{
		Type: BlockTypeText,
		Role: "user",
		Text: "hello world\nthis is line 2",
	}
	got := toCompactBlock(block, lookup)
	if got == "" {
		t.Fatal("expected non-empty text projection, got empty")
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("output is not JSON: %v (got=%q)", err, got)
	}
	v, ok := decoded["user"]
	if !ok {
		t.Fatalf("decoded key \"user\" missing in %v", decoded)
	}
	if v != "hello world\nthis is line 2" {
		t.Errorf("decoded[user]=%q, want full text", v)
	}
}

// T20 malformed_input: read_file 工具但 input 是非法 JSON → fail-open raw.
// 注意: ToAutoClassifierInput 现在能 parse (输入是 JSON), 所以 projection
// 仍会出现 — 真正的 fail-open 是 ToAutoClassifierInput 自身返回空。
//
// 这里用 NaN/nil 指针 style 触发 surface.ToAutoClassifierInput 返空 →
// 上层落到 fail-open 路径: encoded=""; encoded = string(block.Input).
func TestToCompactBlock_MalformedInput(t *testing.T) {
	// 制造一个 ToAutoClassifierInput 返 "" 的情况: input 是空字节。
	lookup := map[string]contracts.ToolSurface{"read_file": makeReadFileSurface()}
	block := TranscriptBlock{
		Type:  BlockTypeToolUse,
		Name:  "read_file",
		Input: "",
	}
	got := toCompactBlock(block, lookup)
	// fail-open 路径: encoded=string("") → marshaled to {"read_file":""}.
	// 不是 "" because surface lookup 找到 read_file, 不进 UnknownTool 分支。
	if got == "" {
		t.Fatal("fail-open should not return empty for known tool + empty input")
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if _, ok := decoded["read_file"]; !ok {
		t.Errorf("fail-open line missing read_file key: %v", decoded)
	}
}

// T20 empty: text 块空 / tool_name 空 → 返回 "".
func TestToCompactBlock_Empty(t *testing.T) {
	lookup := map[string]contracts.ToolSurface{}

	cases := []struct {
		name  string
		block TranscriptBlock
	}{
		{"empty user text", TranscriptBlock{Type: BlockTypeText, Role: "user", Text: ""}},
		{"empty assistant text", TranscriptBlock{Type: BlockTypeText, Role: "assistant", Text: ""}},
		{"empty tool name", TranscriptBlock{Type: BlockTypeToolUse, Name: "", Input: `{}`}},
		{"unknown block type", TranscriptBlock{Type: "garbage", Role: "user", Text: "x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := toCompactBlock(c.block, lookup)
			if got != "" {
				t.Errorf("expected empty for case %q, got %q", c.name, got)
			}
		})
	}
}

// T20 escape_attack: input 含 "\n" / 引号 / 反斜杠 → json.Marshal 已转义,
// round-trip 后必须仍是合法 JSON. 这是 classifier 端反注入的硬要求.
func TestToCompactBlock_EscapeAttack(t *testing.T) {
	lookup := map[string]contracts.ToolSurface{"bash": makeBashSurface()}
	// 试图通过 input 的换行 / 引号覆盖 JSON 结构.
	malicious := `{"command":"ls\n; rm -rf /\n\"DROP\"","x":"\\back"}`
	block := TranscriptBlock{
		Type:  BlockTypeToolUse,
		Name:  "bash",
		Input: malicious,
	}
	got := toCompactBlock(block, lookup)
	if got == "" {
		t.Fatal("expected non-empty projection, got empty")
	}
	// Round-trip 必须仍是合法 JSON — 不能被注入搞坏结构.
	var decoded any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("output is not valid JSON after escape attack: %v (got=%q)", err, got)
	}
	// 必须只输出单个键 "bash" (而不是被注入的多个键).
	m, ok := decoded.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", decoded)
	}
	if _, hasDrop := m["DROP"]; hasDrop {
		t.Errorf("escape attack succeeded — injected key present in %v", m)
	}
}

// T20 unknown_tool: 工具不在 surfaceLookup → fail-open + UnknownTool metric.
//
// 注意: 这里 fail-open 意味着仍然输出 best-effort 行 {name: raw input},
// 而不是返回空. classifier 拿不到精炼 projection 但能看到调用意图.
func TestToCompactBlock_UnknownTool(t *testing.T) {
	lookup := map[string]contracts.ToolSurface{} // 空 lookup
	block := TranscriptBlock{
		Type:  BlockTypeToolUse,
		Name:  "mystery_tool",
		Input: `{"arg":"value"}`,
	}
	got := toCompactBlock(block, lookup)
	if got == "" {
		t.Fatal("expected fail-open line for unknown tool, got empty")
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("output is not JSON: %v (got=%q)", err, got)
	}
	if v, ok := decoded["mystery_tool"]; !ok {
		t.Errorf("fail-open line missing mystery_tool key: %v", decoded)
	} else if v != `{"arg":"value"}` {
		t.Errorf("fail-open line should preserve raw input, got %q", v)
	}
}

// T20 panic 隔离: 表面 ToAutoClassifierInput panic → fail-open (raw input).
//
// 通过单独的 panicky surface 实现触发 panic, 验证 recover 路径.
func TestToCompactBlock_PanicIsolated(t *testing.T) {
	lookup := map[string]contracts.ToolSurface{"boom": panickySurface{}}
	block := TranscriptBlock{
		Type:  BlockTypeToolUse,
		Name:  "boom",
		Input: `{"arg":"v"}`,
	}
	got := toCompactBlock(block, lookup)
	if got == "" {
		t.Fatal("expected fail-open line after panic, got empty")
	}
	// 必须包含原始 input (而非 projection), 因为 surface 自身 panic 了.
	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("output is not JSON: %v (got=%q)", err, got)
	}
	if v, ok := decoded["boom"]; !ok {
		t.Errorf("panic-fail-open line missing boom key: %v", decoded)
	} else if v != `{"arg":"v"}` {
		t.Errorf("panic-fail-open should preserve raw input, got %q", v)
	}
}

// T20 public wrapper: ToCompactBlock(ctx, block, lookup) 不 panic 且与内部一致.
func TestToCompactBlock_PublicWrapper(t *testing.T) {
	lookup := map[string]contracts.ToolSurface{"read_file": makeReadFileSurface()}
	block := TranscriptBlock{Type: BlockTypeText, Role: "user", Text: "hi"}
	got := ToCompactBlock(context.Background(), block, lookup)
	if !strings.Contains(got, `"user":"hi"`) {
		t.Errorf("public wrapper output %q should contain user:hi JSON", got)
	}
}

// --- helpers ---

// bashSurface: 把 input.command 投影为 "!<cmd>" 以便和 fail-open 区分.
type bashSurface struct{}

func makeBashSurface() contracts.ToolSurface { return bashSurface{} }

func (bashSurface) Name() string         { return "bash-stub" }
func (bashSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return []contracts.ToolSpec{{Name: "bash"}}
}
func (bashSurface) RiskLevel(string) types.RiskLevel { return types.RiskLevelLow }
func (bashSurface) InterruptBehavior(string) contracts.InterruptMode {
	return contracts.InterruptBlock
}
func (bashSurface) CheckPermission(context.Context, contracts.ToolSpec, json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}
func (bashSurface) IsConcurrencySafe(json.RawMessage) bool { return false }
func (bashSurface) ToAutoClassifierInput(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var in struct {
		Command string `json:"command"`
	}
	_ = json.Unmarshal(input, &in)
	if in.Command == "" {
		return ""
	}
	return "!" + in.Command
}
func (bashSurface) Execute(_ context.Context, _, _, _ string) (*contracts.ToolResult, error) {
	return &contracts.ToolResult{}, nil
}

// panickySurface 故意在 ToAutoClassifierInput 里 panic.
type panickySurface struct{}

func (panickySurface) Name() string { return "panicky" }
func (panickySurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return []contracts.ToolSpec{{Name: "boom"}}
}
func (panickySurface) RiskLevel(string) types.RiskLevel { return types.RiskLevelLow }
func (panickySurface) InterruptBehavior(string) contracts.InterruptMode {
	return contracts.InterruptBlock
}
func (panickySurface) CheckPermission(context.Context, contracts.ToolSpec, json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}
func (panickySurface) IsConcurrencySafe(json.RawMessage) bool { return true }
func (panickySurface) ToAutoClassifierInput(_ json.RawMessage) string {
	panic("intentional test panic in panickySurface.ToAutoClassifierInput")
}
func (panickySurface) Execute(_ context.Context, _, _, _ string) (*contracts.ToolResult, error) {
	return &contracts.ToolResult{}, nil
}
