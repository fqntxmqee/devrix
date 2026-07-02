// Package decisionplanning — toCompactBlock JSONL 序列化 (DM-20260702-009 T20).
//
// toCompactBlock 转换 LLM transcript 中的单个 block (user text / assistant
// text / tool_use) 为 classifier-friendly 的 compact string, 喂给后续的
// AutoModeClassifier。设计要点:
//
//  1. 文本块 → {role: text} JSON line
//  2. tool_use 块 → {tool_name: ToAutoClassifierInput(input)} JSON line
//  3. 解析失败 / 未知工具 → fail-open: 返回原 input + emit metric
//  4. panic 隔离: 任何 panic recover, 永不挂掉分类管道
//
// 与 clawcode compactTranscriptEntry.ts 的语义一致: 一个 JSONL 行 / block。
//
// DSAFT: D7-S10-A50-T20 (DM-20260702-009 PR-C).
package decisionplanning

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// BlockType constants for TranscriptBlock.Type. Keeps the JSON wire
// format in one place; consumers (auto-classifier, transcript replayer)
// reference these symbols.
const (
	BlockTypeText   = "text"
	BlockTypeToolUse = "tool_use"
)

// TranscriptBlock is the normalized view of a single LLM transcript
// entry. The exact source varies (Anthropic messages API, OpenAI
// chat.completions, custom gateway) — toCompactBlock only reads the
// three fields below and is decoupled from the source shape.
//
// DSAFT: D7-S10-A50-T20.
type TranscriptBlock struct {
	// Type is either BlockTypeText (user / assistant text content) or
	// BlockTypeToolUse (assistant tool_use or tool_result).
	Type string
	// Role is the speaker for text blocks ("user" / "assistant"). Empty
	// for tool_use blocks.
	Role string
	// Name is the tool name for tool_use blocks. Empty for text blocks.
	Name string
	// Input is the raw JSON tool input for tool_use blocks. Empty for
	// text blocks. Preserved as a string to avoid losing formatting /
	// key ordering.
	Input string
	// Text is the text content for text blocks. Empty for tool_use
	// blocks.
	Text string
}

// autoModeMetrics is the singleton counter for auto-mode classifier
// feedback. We use a package-level singleton (mirrors the LSPMetrics
// pattern from metrics/lsp.go) — counters are cheap to allocate, but
// the hot path in toCompactBlock avoids locking.
var (
	autoModeMetricsOnce sync.Once
	autoModeMetrics     *autoModeCounters
)

type autoModeCounters struct {
	MalformedToolInput Counter
	UnknownTool         Counter
	Panic                Counter
	Empty                Counter
}

// Counter is the minimal interface we use from metrics. We don't
// import the full Counter type to keep this package's dep tree
// narrow — the metrics package's Counter has the same shape.
type Counter interface {
	Inc()
}

// ensureAutoModeMetrics initializes the package-level counters once.
// Safe to call from concurrent goroutines.
//
// DSAFT: D7-S10-A50-T20.
//
// Metric semantics (name → label → behavior):
//
//	auto_mode_malformed_tool_input_total — tool_use input bytes failed
//	  the JSON parse inside toCompactBlock (fail-open).
//	auto_mode_unknown_tool_total — tool_use block whose Name is not in
//	  the surfaceLookup map (fail-open raw input still surfaced).
//	auto_mode_to_compact_panic_total — toCompactBlock caught a panic
//	  in projection / marshalling and fell back to fail-open.
//	auto_mode_to_compact_empty_total — toCompactBlock returned an
//	  empty string (no usable output, e.g. empty text or empty input).
func ensureAutoModeMetrics() {
	autoModeMetricsOnce.Do(func() {
		autoModeMetrics = &autoModeCounters{
			MalformedToolInput: metrics.NewCounter(
				"auto_mode_malformed_tool_input_total",
				metrics.LabelMap{"kind": "tool_input"},
			),
			UnknownTool: metrics.NewCounter(
				"auto_mode_unknown_tool_total",
				metrics.LabelMap{"kind": "unknown_tool"},
			),
			Panic: metrics.NewCounter(
				"auto_mode_to_compact_panic_total",
				metrics.LabelMap{"kind": "panic"},
			),
			Empty: metrics.NewCounter(
				"auto_mode_to_compact_empty_total",
				metrics.LabelMap{"kind": "empty"},
			),
		}
	})
}

// toCompactBlock converts one TranscriptBlock into a single
// JSONL-compatible string (one JSON object + \n).
//
// Returns "" when the block cannot be projected at all (unknown
// tool, empty text). Empty returns are filtered upstream by the
// classifier; the metric `auto_mode_to_compact_empty_total` lets
// dashboards distinguish "intentional skip" from "bug".
//
// The function is panic-safe — any panic in the per-tool projection
// path is recovered and treated as fail-open (raw input).
//
// DSAFT: D7-S10-A50-T20.
func toCompactBlock(
	block TranscriptBlock,
	surfaceLookup map[string]contracts.ToolSurface,
) (out string) {
	ensureAutoModeMetrics()
	defer func() {
		if r := recover(); r != nil {
			autoModeMetrics.Panic.Inc()
			out = failOpenLine(block)
		}
	}()

	switch block.Type {
	case BlockTypeToolUse:
		if block.Name == "" {
			autoModeMetrics.Empty.Inc()
			return ""
		}
		surface, ok := surfaceLookup[block.Name]
		if !ok || surface == nil {
			autoModeMetrics.UnknownTool.Inc()
			// Fail-open: include the raw input so the classifier at
			// least sees the call attempt (best-effort signal).
			return failOpenLine(block)
		}
		encoded := surface.ToAutoClassifierInput([]byte(block.Input))
		if encoded == "" {
			// Surface returns "" for default-surfaced tools (P2 stub).
			// Per design.md §②P2 fail-open, we still emit the line so
			// the classifier sees the tool invocation.
			encoded = string(block.Input)
		}
		bz, err := json.Marshal(map[string]string{block.Name: encoded})
		if err != nil {
			autoModeMetrics.MalformedToolInput.Inc()
			return failOpenLine(block)
		}
		return string(bz)
	case BlockTypeText:
		text := block.Text
		if text == "" {
			autoModeMetrics.Empty.Inc()
			return ""
		}
		role := block.Role
		if role == "" {
			role = "user"
		}
		bz, err := json.Marshal(map[string]string{role: text})
		if err != nil {
			return ""
		}
		return string(bz)
	default:
		// Unknown block type — skip silently (forward compat).
		return ""
	}
}

// failOpenLine returns the best-effort JSON line for a block whose
// proper projection failed. For tool_use, it's {name: raw input}; for
// text, it's {role: text}; for unknown types, it's empty.
func failOpenLine(block TranscriptBlock) string {
	switch block.Type {
	case BlockTypeToolUse:
		bz, err := json.Marshal(map[string]string{block.Name: block.Input})
		if err != nil {
			return ""
		}
		return string(bz)
	case BlockTypeText:
		role := block.Role
		if role == "" {
			role = "user"
		}
		bz, err := json.Marshal(map[string]string{role: block.Text})
		if err != nil {
			return ""
		}
		return string(bz)
	}
	return ""
}

// ToCompactBlock is the public wrapper for toCompactBlock. Pass a
// context for future cancellation hooks; today the context is unused
// but having it on the signature matches the rest of the AutoMode
// pipeline.
//
// DSAFT: D7-S10-A50-T20.
func ToCompactBlock(
	ctx context.Context,
	block TranscriptBlock,
	surfaceLookup map[string]contracts.ToolSurface,
) string {
	_ = ctx // reserved for future ctx-cancel propagation
	return toCompactBlock(block, surfaceLookup)
}

// toCompactBlockString is a convenience for tests: takes a JSON
// TranscriptBlock payload and returns the compact line. Returns "" +
// non-nil error on malformed JSON.
func toCompactBlockFromJSON(raw []byte, surfaceLookup map[string]contracts.ToolSurface) (string, error) {
	var block TranscriptBlock
	if err := json.Unmarshal(raw, &block); err != nil {
		return "", fmt.Errorf("toCompactBlockFromJSON: %w", err)
	}
	return toCompactBlock(block, surfaceLookup), nil
}
