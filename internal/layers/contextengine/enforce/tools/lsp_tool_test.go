package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/lsp"
	"github.com/devrix/devrix/internal/shared/types"
)

// TestLSP_DisabledByDefault — LSPConfig.Enabled=false 时返回明确错误。
func TestLSP_DisabledByDefault(t *testing.T) {
	r := newLSPRunner(&LSPConfig{Enabled: false})
	res, err := r.Execute(context.Background(), "/tmp", `{"operation":"definition","file_path":"a.go","line":1,"character":1}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Error, "disabled") {
		t.Fatalf("expected disabled error, got %q", res.Error)
	}
}

// TestLSP_NoServersConfigured — 无 server 配置时返回明确错误。
func TestLSP_NoServersConfigured(t *testing.T) {
	r := newLSPRunner(&LSPConfig{Enabled: true, Servers: nil})
	res, _ := r.Execute(context.Background(), "/tmp", `{"operation":"definition","file_path":"a.go","line":1,"character":1}`)
	if !strings.Contains(res.Error, "no servers") {
		t.Fatalf("expected 'no servers configured' error, got %q", res.Error)
	}
}

// TestLSP_BadJSON — 输入 JSON 解析失败。
func TestLSP_BadJSON(t *testing.T) {
	r := newLSPRunner(&LSPConfig{Enabled: true, Servers: []lsp.ServerConfig{
		{LanguageID: "go", Command: []string{"gopls"}, FilePattern: []string{"*.go"}},
	}})
	res, _ := r.Execute(context.Background(), "/tmp", `not json`)
	if !strings.Contains(res.Error, "parse") {
		t.Fatalf("expected parse error, got %q", res.Error)
	}
}

// TestLSP_MissingFields — 必填字段缺失。
func TestLSP_MissingFields(t *testing.T) {
	r := newLSPRunner(&LSPConfig{Enabled: true, Servers: []lsp.ServerConfig{
		{LanguageID: "go", Command: []string{"gopls"}, FilePattern: []string{"*.go"}},
	}})
	cases := []string{
		`{}`,
		`{"operation":"definition"}`,
		`{"operation":"definition","file_path":"a.go"}`,
		`{"operation":"definition","file_path":"a.go","line":0,"character":1}`,
	}
	for _, c := range cases {
		res, _ := r.Execute(context.Background(), "/tmp", c)
		if res.Error == "" {
			t.Fatalf("expected error for %q, got success", c)
		}
	}
}

// TestLSP_FileNotFound — 文件不存在。
func TestLSP_FileNotFound(t *testing.T) {
	r := newLSPRunner(&LSPConfig{Enabled: true, Servers: []lsp.ServerConfig{
		{LanguageID: "go", Command: []string{"gopls"}, FilePattern: []string{"*.go"}},
	}})
	res, _ := r.Execute(context.Background(), "/tmp", `{"operation":"definition","file_path":"nonexistent_12345.go","line":1,"character":1}`)
	if !strings.Contains(res.Error, "not found") {
		t.Fatalf("expected 'not found' error, got %q", res.Error)
	}
}

// TestLSP_SchemaExposesOperations — Schema 含 5 个合法 operation
// (DM-20260618-007 D2-S4-A01 扩展: + hover, + workspace_symbol).
func TestLSP_SchemaExposesOperations(t *testing.T) {
	r := newLSPRunner(&LSPConfig{Enabled: true})
	schema := r.Schema()
	if schema.Name != "lsp" {
		t.Fatalf("expected name=lsp, got %q", schema.Name)
	}
	var params struct {
		Properties struct {
			Operation struct {
				Enum []string `json:"enum"`
			} `json:"operation"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(schema.Parameters), &params); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"definition":       true,
		"references":       true,
		"incoming_calls":   true,
		"hover":            true, // D2-S4-A01-F04
		"workspace_symbol": true, // D2-S4-A01-F05
	}
	for _, op := range params.Properties.Operation.Enum {
		if !want[op] {
			t.Errorf("unexpected op: %s", op)
		}
		delete(want, op)
	}
	if len(want) > 0 {
		t.Fatalf("missing ops: %v", want)
	}
}

// TestLSP_RiskLevelLow — 风险等级为 Low（只读）。
func TestLSP_RiskLevelLow(t *testing.T) {
	r := newLSPRunner(&LSPConfig{Enabled: true})
	if lvl := r.RiskLevel(); lvl != types.RiskLevelLow {
		t.Fatalf("expected low risk, got %v", lvl)
	}
}

// TestFormatLocations — Location 列表格式化。
func TestFormatLocations(t *testing.T) {
	locs := []lsp.Location{
		{URI: "file:///tmp/foo.go", Range: lsp.Range{
			Start: lsp.Position{Line: 4, Character: 2},
		}, Preview: "func Bar()"},
	}
	out := formatLocations(locs, "/tmp")
	if !strings.Contains(out, "foo.go") {
		t.Errorf("output missing foo.go: %q", out)
	}
	if !strings.Contains(out, "5:3") { // 1-based: 4+1, 2+1
		t.Errorf("output missing 1-based pos: %q", out)
	}
}

// TestFormatIncomingCalls — IncomingCalls 格式化。
func TestFormatIncomingCalls(t *testing.T) {
	calls := []lsp.CallHierarchyIncomingCall{
		{From: lsp.CallHierarchyItem{
			Name: "CallerFunc", Kind: lsp.SymbolKindFunction, URI: "file:///tmp/caller.go",
			Range: lsp.Range{Start: lsp.Position{Line: 9, Character: 0}},
		}},
	}
	out := formatIncomingCalls(calls, "/tmp")
	if !strings.Contains(out, "CallerFunc") {
		t.Errorf("output missing CallerFunc: %q", out)
	}
	if !strings.Contains(out, "function") {
		t.Errorf("output missing 'function' kind: %q", out)
	}
	if !strings.Contains(out, "caller.go") {
		t.Errorf("output missing caller.go: %q", out)
	}
}

// TestSymbolKindName — SymbolKind → string。
func TestSymbolKindName(t *testing.T) {
	cases := map[lsp.SymbolKind]string{
		lsp.SymbolKindFunction:    "function",
		lsp.SymbolKindMethod:      "method",
		lsp.SymbolKindClass:       "class",
		lsp.SymbolKindInterface:   "interface",
		lsp.SymbolKindVariable:    "variable",
		lsp.SymbolKindField:       "field",
		lsp.SymbolKindConstructor: "constructor",
		lsp.SymbolKindArray:       "symbol",
	}
	for k, want := range cases {
		if got := symbolKindName(k); got != want {
			t.Errorf("kind %d: expected %s, got %s", k, want, got)
		}
	}
}
