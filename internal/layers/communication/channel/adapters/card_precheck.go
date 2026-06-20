// Package adapters — Card content precheck abstraction.
//
// Context: see openspec/changes/2026-06-20-devrix-context-budget-and-isolation/.
//
// Feishu card API rejects cards whose `<table>` element count exceeds an
// internal limit (ErrCode 11310). Pre-check before sending so we can fall back
// to a plain-text path instead of retrying into a silent failure loop.
//
// DSAFT: D1-S5-A07 (SendOutboundCard) extension.
package adapters

import (
	"errors"
	"fmt"
)

// CardContentPrecheck inspects card content before it's sent to a channel.
//
// Implementations are channel-specific: feishu checks table count, future
// channels (lark / wechat) may check different limits.
type CardContentPrecheck interface {
	// Name returns a stable identifier for logs.
	Name() string

	// Check returns nil if content passes, or a wrapped error otherwise.
	// Common wrapped errors: ErrTooManyTables, ErrTooLong.
	Check(content string) error
}

// CardPrecheckConfig holds tunable limits.
type CardPrecheckConfig struct {
	// MaxTablesPerCard is the maximum `<table>` element count per card.
	// Feishu rejects cards with more (ErrCode 11310); default 5 is conservative.
	MaxTablesPerCard int

	// MaxCharsPerCard is a soft cap on total card JSON size.
	// Feishu hard limit is 30KB for interactive cards; default 28000.
	MaxCharsPerCard int
}

// DefaultCardPrecheckConfig returns the recommended default config.
//
// Conservative defaults aligned with feishu API limits observed 2026-06-20
// (sess_1781916669178_3000 / D5 spans task).
func DefaultCardPrecheckConfig() CardPrecheckConfig {
	return CardPrecheckConfig{
		MaxTablesPerCard: 5,
		MaxCharsPerCard:  28000,
	}
}

// ErrTooManyTables signals the card has too many `<table>` elements.
var ErrTooManyTables = errors.New("card content has too many tables")

// ErrTooLong signals the card content exceeds the soft character limit.
var ErrTooLong = errors.New("card content too long")

// FormatPrecheckError formats an error for logs.
func FormatPrecheckError(p CardContentPrecheck, content string, err error) string {
	return fmt.Sprintf("card precheck failed: precheck=%s error=%v content_len=%d",
		p.Name(), err, len(content))
}