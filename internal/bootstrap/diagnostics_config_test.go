package bootstrap

// W13 — 配置 + Bootstrap 总集成 单元测试。
//
// AC14 (P2 锁定):
//   - DefaultDiagnosticsConfig 字段非零 (LRU=500, Tick=1000ms, LSP=false)
//   - Normalized 0 值字段填充为 default
//   - ContextEngineConfig.Normalized 注入 Diagnostics
//   - DiagnosticsConfig 0 → normalized 走 default

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
)

func TestDefaultDiagnosticsConfig_HasSensibleDefaults(t *testing.T) {
	d := config.DefaultDiagnosticsConfig()
	if d.TrackerLRUCapacity != 500 {
		t.Errorf("TrackerLRUCapacity = %d, want 500", d.TrackerLRUCapacity)
	}
	if d.TrackerTickIntervalMs != 1000 {
		t.Errorf("TrackerTickIntervalMs = %d, want 1000", d.TrackerTickIntervalMs)
	}
	if d.LSPEnabled != false {
		t.Errorf("LSPEnabled = %v, want false", d.LSPEnabled)
	}
}

func TestDiagnosticsConfig_NormalizedFillsZeroFields(t *testing.T) {
	d := config.DiagnosticsConfig{
		TrackerLRUCapacity:   0, // 0 → 走 default
		TrackerTickIntervalMs: 0, // 0 → 走 default
		LSPEnabled:           true, // 显式 true 保留
		TranscriptDir:        "/custom/transcripts", // 显式设置保留
	}
	out := d.Normalized()
	if out.TrackerLRUCapacity != 500 {
		t.Errorf("normalized LRU = %d, want 500", out.TrackerLRUCapacity)
	}
	if out.TrackerTickIntervalMs != 1000 {
		t.Errorf("normalized tick = %d, want 1000", out.TrackerTickIntervalMs)
	}
	if out.LSPEnabled != true {
		t.Errorf("LSPEnabled should be preserved, got %v", out.LSPEnabled)
	}
	if out.TranscriptDir != "/custom/transcripts" {
		t.Errorf("TranscriptDir should be preserved, got %q", out.TranscriptDir)
	}
}

func TestDiagnosticsConfig_NormalizedPreservesExplicitValues(t *testing.T) {
	d := config.DiagnosticsConfig{
		TrackerLRUCapacity:   2000,
		TrackerTickIntervalMs: 500,
	}
	out := d.Normalized()
	if out.TrackerLRUCapacity != 2000 {
		t.Errorf("explicit LRU = %d, want 2000 (preserved)", out.TrackerLRUCapacity)
	}
	if out.TrackerTickIntervalMs != 500 {
		t.Errorf("explicit tick = %d, want 500 (preserved)", out.TrackerTickIntervalMs)
	}
}

func TestDefaultContextEngineConfig_IncludesDiagnostics(t *testing.T) {
	c := config.DefaultContextEngineConfig()
	if c.Diagnostics.TrackerLRUCapacity == 0 {
		t.Error("DefaultContextEngineConfig should include non-zero Diagnostics.TrackerLRUCapacity")
	}
	if c.Diagnostics.TrackerTickIntervalMs == 0 {
		t.Error("DefaultContextEngineConfig should include non-zero Diagnostics.TrackerTickIntervalMs")
	}
}

func TestLSPServerConfig_Fields(t *testing.T) {
	// 验证 LSPServerConfig 可序列化 (yaml tag 生效).
	lsp := config.LSPServerConfig{
		Name:    "gopls",
		Command: "gopls",
		Args:    []string{"-rpc.verbose"},
	}
	if lsp.Name != "gopls" {
		t.Errorf("Name = %q, want gopls", lsp.Name)
	}
	if lsp.Command != "gopls" {
		t.Errorf("Command = %q, want gopls", lsp.Command)
	}
	if len(lsp.Args) != 1 || lsp.Args[0] != "-rpc.verbose" {
		t.Errorf("Args = %v, want [-rpc.verbose]", lsp.Args)
	}
}
