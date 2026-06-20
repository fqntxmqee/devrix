package adapters

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/kernel"
)

func TestCardPrecheckConfig_Default(t *testing.T) {
	cfg := DefaultCardPrecheckConfig()
	if cfg.MaxTablesPerCard != 5 {
		t.Errorf("MaxTablesPerCard: got %d, want 5", cfg.MaxTablesPerCard)
	}
	if cfg.MaxCharsPerCard != 28000 {
		t.Errorf("MaxCharsPerCard: got %d, want 28000", cfg.MaxCharsPerCard)
	}
}

func TestCountTableTags(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"empty", "", 0},
		{"no table", "plain text only", 0},
		{"single table", "<table>...</table>", 1},
		{"multiple tables", "<table>a</table><table>b</table>", 2},
		{"nested tag prefix", "<table_view>", 0}, // substring should NOT match
		{"attribute", `tag="table"`, 0},
		{"markdown table syntax", "| col1 | col2 |\n|------|------|", 0}, // pipe table is not <table>
		{"mixed", "before <table>x</table> between <table>y</table> after", 2},
		{"self-closing", "<table/>", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countTableTags(tt.content)
			if got != tt.want {
				t.Errorf("countTableTags(%q): got %d, want %d", tt.content, got, tt.want)
			}
		})
	}
}

func TestFeishuTableCountPrecheck_BelowLimit(t *testing.T) {
	p := &FeishuTableCountPrecheck{cfg: DefaultCardPrecheckConfig()}

	content := "<table>a</table><table>b</table>"
	if err := p.Check(content); err != nil {
		t.Errorf("expected nil for 2 tables, got %v", err)
	}
}

func TestFeishuTableCountPrecheck_AtLimit(t *testing.T) {
	p := &FeishuTableCountPrecheck{cfg: DefaultCardPrecheckConfig()}

	var sb strings.Builder
	for i := 0; i < 5; i++ {
		sb.WriteString("<table>r</table>")
	}
	content := sb.String()

	if err := p.Check(content); err != nil {
		t.Errorf("expected nil at limit (5 tables), got %v", err)
	}
}

func TestFeishuTableCountPrecheck_AboveLimit(t *testing.T) {
	p := &FeishuTableCountPrecheck{cfg: DefaultCardPrecheckConfig()}

	var sb strings.Builder
	for i := 0; i < 6; i++ {
		sb.WriteString("<table>r</table>")
	}
	content := sb.String()

	err := p.Check(content)
	if err == nil {
		t.Fatal("expected error for 6 tables, got nil")
	}
	if !errors.Is(err, ErrTooManyTables) {
		t.Errorf("expected ErrTooManyTables wrapped, got %v", err)
	}
}

func TestFeishuTableCountPrecheck_TooLong(t *testing.T) {
	cfg := CardPrecheckConfig{MaxTablesPerCard: 100, MaxCharsPerCard: 100}
	p := &FeishuTableCountPrecheck{cfg: cfg}

	longContent := strings.Repeat("x", 200)
	err := p.Check(longContent)
	if err == nil {
		t.Fatal("expected error for too-long content, got nil")
	}
	if !errors.Is(err, ErrTooLong) {
		t.Errorf("expected ErrTooLong wrapped, got %v", err)
	}
}

func TestFeishuTableCountPrecheck_Name(t *testing.T) {
	p := &FeishuTableCountPrecheck{cfg: DefaultCardPrecheckConfig()}
	if p.Name() != "feishu-table-count" {
		t.Errorf("Name: got %q, want %q", p.Name(), "feishu-table-count")
	}
}

func TestCardFallbackText_HeaderAndMarkdown(t *testing.T) {
	card := kernel.NewCard().
		Title("My Card", "blue").
		Markdown("hello world").
		Build()

	out := cardFallbackText(card, ErrTooManyTables)

	if !strings.Contains(out, "My Card") {
		t.Errorf("expected header in output, got %q", out)
	}
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected markdown content in output, got %q", out)
	}
	if !strings.Contains(out, "card auto-flattened") {
		t.Errorf("expected auto-flattened marker in output, got %q", out)
	}
	if !strings.Contains(out, "too many tables") {
		t.Errorf("expected precheck error in output, got %q", out)
	}
}

func TestCardFallbackText_FlattensPipeTable(t *testing.T) {
	pipes := "| col1 | col2 |\n|------|------|\n| a    | b    |"
	card := kernel.NewCard().Markdown(pipes).Build()

	out := cardFallbackText(card, nil)

	// flattenMarkdownTablesForFeishu should remove the separator row.
	if strings.Contains(out, "|------|") {
		t.Errorf("expected separator row to be flattened, got %q", out)
	}
	if !strings.Contains(out, "col1") || !strings.Contains(out, "a") {
		t.Errorf("expected row data preserved, got %q", out)
	}
}

func TestCardFallbackText_NilCard(t *testing.T) {
	out := cardFallbackText(nil, fmt.Errorf("boom"))
	if !strings.Contains(out, "boom") {
		t.Errorf("expected precheck error in output even with nil card, got %q", out)
	}
}