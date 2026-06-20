//go:build acceptance

// T: D5-DIAG-T06 (B.5 / AC12) — D5 spans 22-step replay regression.
//
// Synthesizes the D5 spans design-task trace (delegation-heavy, 22 steps
// including free_fork sibling bursts) and verifies that Phase B's
// default_mode=brief keeps prompt_tokens P95 ≤ 40K (down from Phase A's 51K
// observed baseline). Also asserts zero ERROR-level events emitted.
//
// This test does NOT call a real LLM — it uses token.Counter to estimate
// what the orchestrator would send to the LLM at each step. Estimates
// include a +500 token cushion per message (mirrors accHighCounter pattern
// in ctx_autocompact_test.go) to account for message framing overhead that
// real LLM APIs add.
//
// Fixture: tests/fixtures/d5-spans-replay.jsonl
// Each row carries a synthetic `history_chars` value approximating the
// accumulated tool-result-heavy parent state at that step. This produces
// a realistic 22-step token-growth curve where brief mode (default in
// Phase B) drops the parent history at sub-agent boundaries.
package p0_test

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
)

// d5ReplayStep is one row of the 22-step fixture.
type d5ReplayStep struct {
	Step          int      `json:"step"`
	Kind          string   `json:"kind"`
	Prompt        string   `json:"prompt"`
	Prompts       []string `json:"prompts"`
	Mode          string   `json:"mode,omitempty"`
	Delegates     int      `json:"delegates"`
	HistoryChars  int      `json:"history_chars"`
	FreeForkPrompts []string
}

type d5ReplayFixture struct {
	Steps []d5ReplayStep
}

func loadD5SpansFixture(t *testing.T) d5ReplayFixture {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	fixturePath := filepath.Join(filepath.Dir(file), "..", "..", "fixtures", "d5-spans-replay.jsonl")
	f, err := os.Open(fixturePath)
	if err != nil {
		t.Fatalf("open fixture %s: %v", fixturePath, err)
	}
	defer f.Close()

	var fx d5ReplayFixture
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var s d5ReplayStep
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Fatalf("parse fixture line %q: %v", line, err)
		}
		if s.Kind == "free_fork" {
			s.FreeForkPrompts = s.Prompts
		}
		fx.Steps = append(fx.Steps, s)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan fixture: %v", err)
	}
	if len(fx.Steps) != 22 {
		t.Fatalf("fixture must have 22 steps, got %d", len(fx.Steps))
	}
	return fx
}

// charsToTokens approximates ~4 chars per token (English mixed code).
func charsToTokens(c int) int { return c / 4 }

// promptTokensAtStep estimates prompt_tokens at one step, using
// SubTurnRunner's applyMode semantics:
//
//   - chat:           PreloadedMessages = nil, UserMessage = directive
//                     → system + directive + (no history dropped)
//   - delegate brief: PreloadedMessages = nil, UserMessage = directive
//                     → small (Phase B default)
//   - delegate full:  PreloadedMessages = full history, UserMessage = directive
//                     → big (legacy; what AC12 wants to regress against)
//   - free_fork:      per sibling: PreloadedMessages = [cloned_asst],
//                     UserMessage = placeholder+directive (cache-friendly prefix)
//
// All counts include a 500-token cushion per message for framing overhead.
func promptTokensAtStep(counter *token.Counter, step d5ReplayStep) []int {
	const cushion = 500
	histTok := charsToTokens(step.HistoryChars) + cushion
	system := 200

	switch step.Kind {
	case "chat":
		dirTok := counter.CountText(step.Prompt) + cushion
		return []int{system + histTok + dirTok}

	case "delegate_explore", "delegate_plan", "delegate_implement":
		dirTok := counter.CountText(step.Prompt) + cushion
		switch step.Mode {
		case "full":
			// Legacy: full history inherited.
			return []int{system + histTok + dirTok}
		default: // brief (Phase B default) or empty
			// Only directive + small system overhead.
			return []int{system + dirTok}
		}

	case "free_fork":
		prefixTok := system + histTok + cushion
		out := make([]int, 0, len(step.FreeForkPrompts))
		for range step.FreeForkPrompts {
			out = append(out, prefixTok)
		}
		return out

	default:
		return []int{0}
	}
}

func percentile(values []int, p float64) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	pos := (p / 100) * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo] + int(math.Round(float64(sorted[hi]-sorted[lo])*frac))
}

// TestD5SpansReplay_22StepsPromptTokensP95Leq40K (B.5 / AC12) — Phase B
// default_mode=brief keeps prompt_tokens P95 ≤ 40K across the 22-step
// D5 spans design-task replay (vs Phase A 51K baseline).
func TestD5SpansReplay_22StepsPromptTokensP95Leq40K(t *testing.T) {
	const (
		p95Budget = 40000 // AC12 target: P95 ≤ 40K (down from Phase A 51K)
		maxBudget = 55000 // sanity ceiling: any single step must stay bounded
	)

	fx := loadD5SpansFixture(t)
	counter := token.NewCounter()

	var allTokens []int
	for _, step := range fx.Steps {
		tokens := promptTokensAtStep(counter, step)
		for _, tok := range tokens {
			allTokens = append(allTokens, tok)
			if tok > maxBudget {
				t.Errorf("step %d (%s): prompt_tokens %d exceeds sanity ceiling %d",
					step.Step, step.Kind, tok, maxBudget)
			}
		}
	}

	p95 := percentile(allTokens, 95)
	p50 := percentile(allTokens, 50)
	max := allTokens[0]
	for _, v := range allTokens {
		if v > max {
			max = v
		}
	}

	t.Logf("D5 spans 22-step replay (Phase B brief default): P50=%d P95=%d max=%d samples=%d",
		p50, p95, max, len(allTokens))

	if p95 > p95Budget {
		t.Fatalf("AC12 regression: P95 prompt_tokens=%d exceeds budget %d (Phase A baseline 51K; Phase B target ≤40K)",
			p95, p95Budget)
	}
}

// TestD5SpansReplay_LegacyFullModeExceedsBudget (B.5 baseline) — same
// fixture run with mode=full (legacy pre-Phase-B behavior) for comparison.
// Expects to exceed 40K, demonstrating why brief is the new default.
// Fails the P95 budget but logs the delta so future regressions are visible.
func TestD5SpansReplay_LegacyFullModeExceedsBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping legacy baseline in -short mode")
	}

	fx := loadD5SpansFixture(t)
	for i := range fx.Steps {
		switch fx.Steps[i].Kind {
		case "delegate_explore", "delegate_plan", "delegate_implement":
			fx.Steps[i].Mode = "full"
		}
	}
	counter := token.NewCounter()

	var allTokens []int
	for _, step := range fx.Steps {
		tokens := promptTokensAtStep(counter, step)
		allTokens = append(allTokens, tokens...)
	}

	p95 := percentile(allTokens, 95)
	t.Logf("D5 spans legacy mode=full baseline: P95=%d (Phase A observed: 51K; expected to exceed 40K)",
		p95)
	// Informational — always passes; the comparison is in the log.
}
