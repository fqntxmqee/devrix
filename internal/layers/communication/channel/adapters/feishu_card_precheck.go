package adapters

import (
	"fmt"
	"strings"
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

// countTableTags counts `<table` occurrences in content.
// We use `<table` (not `<table>`) to avoid matching `<table_view>` etc.
// This is a heuristic but adequate for feishu card JSON which only uses
// canonical `<table>` tags in cardkit schemas.
func countTableTags(content string) int {
	return strings.Count(content, "<table")
}