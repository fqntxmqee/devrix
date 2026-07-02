// Package persist — T28 inputsEquivalent 联动 bridge (DM-20260702-009 PR-F).
//
// ContentReplacementState 在 cache invalidation 时需要判断 "这次 input
// 跟之前记录的 input 是不是同一个意图". 这是 inputsEquivalent 的主 consumer.
//
// 依赖方向 (intra-domain, 无 boundary, design.md §4.4):
//
//	persist → enforce/tools/surface.InputsEquivalent
//
// bridge 设计: 不让 persist 自己实现等价语义 (避免 drift), 而是 thin
// wrapper 转发到 surface.InputsEquivalent (the 19-tool canonical).
//
// 防御: nil / 退化输入直接走 raw byte compare (保留旧行为, 不可
// 引起性能回归). 这是为了 hot path 在 inputsEquivalent 在某些路径
// 不可达时不退化.
//
// DSAFT: D2-S15-A02-T28 (DM-20260702-009 PR-F).
package persist

import (
	"bytes"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/surface"
)

// InputsEquivalentBridge is the persist-side wrapper for the surface
// inputsEquivalent 19-tool logic. Returns true when two tool-call inputs
// are semantically equivalent for the purpose of caching the LLM-visible
// replacement string (i.e. "did the user want the same thing?").
//
// Tool-name routing: bash / read_file / write_file / edit_file / grep /
// glob / lsp_* / free_fork / task_* / ask_user_question / tool_search
// follow the per-tool canonical (see surface.InputsEquivalent). All other
// names fall back to JSON canonicalize + reflect.DeepEqual.
//
// empty / nil inputs: a == "" && b == "" → true; else fall-through to
// surface.InputsEquivalent (which returns false for nil/non-empty pairs).
func InputsEquivalentBridge(toolName string, a, b []byte) bool {
	if bytes.Equal(a, b) {
		return true
	}
	return surface.InputsEquivalent(toolName, a, b)
}
