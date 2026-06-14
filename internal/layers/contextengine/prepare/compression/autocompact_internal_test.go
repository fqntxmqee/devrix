package compression

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func Test_shouldAutocompact_after_snip_scenario(t *testing.T) {
	counter := &testHighCounter{}
	cfg := config.DefaultAutocompactConfig()
	cfg.Enabled = true
	cfg.MinMessagesForSummary = 4
	cfg.PreserveHeadTurns = 1
	cfg.PreserveTailTurns = 1

	var msgs []types.Message
	for i := 0; i < 12; i++ {
		role := types.MessageRoleUser
		if i%2 == 1 {
			role = types.MessageRoleAssistant
		}
		msgs = append(msgs, *types.NewMessage("m", "s", role, strings.Repeat("word ", 80)))
	}
	budget := types.DefaultTokenBudget()
	budget.CompressionTarget = 100
	current := append([]types.Message(nil), msgs...)
	current = snip(counter, current, budget.CompressionTarget, 6)

	if !shouldAutocompact(current, budget, counter, cfg) {
		t.Fatalf("expected shouldAutocompact true, msgs=%d tokens=%d", len(current), counter.CountMessages(current))
	}
	turns := splitTurns(current)
	if len(turns) <= cfg.PreserveHeadTurns+cfg.PreserveTailTurns {
		t.Fatalf("turns=%d head=%d tail=%d", len(turns), cfg.PreserveHeadTurns, cfg.PreserveTailTurns)
	}
}

type testHighCounter struct{}

func (testHighCounter) CountText(string) int              { return 600 }
func (testHighCounter) CountMessages([]types.Message) int { return 600 }
func (testHighCounter) CountWithSystemPrompt(string, []types.Message) int {
	return 600
}
func (testHighCounter) TruncateToTokens(text string, _ int) string { return text }
func (testHighCounter) EncodingForModel(string) string             { return "test" }
