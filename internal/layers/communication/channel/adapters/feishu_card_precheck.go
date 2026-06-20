package adapters

import (
	"fmt"
	"regexp"
)

// FeishuTableCountPrecheck enforces feishu card content limits.
//
// Specifically:
//   - `<table>` element count must be ≤ cfg.MaxTablesPerCard (default 5).
//     Feishu ErrCode 11310 rejects cards with too many tables.
//   - Total content length must be ≤ cfg.MaxCharsPerCard (default 28000).
//     Feishu hard limit is 30KB for interactive cards.
//
// Reference: openspec/changes/2026-06-20-devrix-context-budget-and-isolation/design.md §1.5
type FeishuTableCountPrecheck struct {
	cfg CardPrecheckConfig
}

// NewFeishuTableCountPrecheck constructs a FeishuTableCountPrecheck with the given config.
// Pass DefaultCardPrecheckConfig() for recommended defaults.
func NewFeishuTableCountPrecheck(cfg CardPrecheckConfig) *FeishuTableCountPrecheck {
	return &FeishuTableCountPrecheck{cfg: cfg}
}

// Name returns "feishu-table-count" for log identification.
func (p *FeishuTableCountPrecheck) Name() string {
	return "feishu-table-count"
}

// Check inspects content and returns:
//   - ErrTooManyTables wrapped with count/limit if `<table>` count exceeds cfg.MaxTablesPerCard
//   - ErrTooLong wrapped with size/limit if total length exceeds cfg.MaxCharsPerCard
//   - nil if all checks pass
func (p *FeishuTableCountPrecheck) Check(content string) error {
	// Table count check.
	tableCount := countTableTags(content)
	if tableCount > p.cfg.MaxTablesPerCard {
		return fmt.Errorf("%w: count=%d, limit=%d",
			ErrTooManyTables, tableCount, p.cfg.MaxTablesPerCard)
	}

	// Length check (soft cap; feishu hard limit is 30KB).
	if len(content) > p.cfg.MaxCharsPerCard {
		return fmt.Errorf("%w: size=%d, limit=%d",
			ErrTooLong, len(content), p.cfg.MaxCharsPerCard)
	}

	return nil
}

// tableTagRegex matches a `<table>` (or self-closing `<table/>`) tag but
// not other tags whose names happen to start with "table" (e.g.
// `<table_view>`). Anchored to a word boundary: either a `>` (open or
// close) or a `/` (self-close) immediately after "table".
var tableTagRegex = regexp.MustCompile(`<table(>|/>|[\s][^>]*>)`)

// countTableTags counts `<table>` (and self-closing `<table/>`) elements
// in content. The regex anchors to `>` or `/>` after "table" so we do
// not match `<table_view>` style false-positives.
func countTableTags(content string) int {
	if content == "" {
		return 0
	}
	return len(tableTagRegex.FindAllString(content, -1))
}