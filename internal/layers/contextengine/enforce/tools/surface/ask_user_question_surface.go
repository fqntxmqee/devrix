// Package surface — ask_user_question surface (DM-20260618-006).
//
// ask_user_question exposes the AskUserQuestion tool, the LLM's
// interactive Q&A primitive. Mirrors clawcode's AskUserQuestionTool
// (Claude Code v2.1.88) for the question schema (1-4 questions, 2-4
// options each, multiSelect) but with one important devrix difference:
//
//	devrix runs as a long-lived IM bot, so the tool does NOT block
//	waiting for a UI click. It sends the question as a formatted
//	outbound IM message and returns immediately. The user's reply
//	arrives as the next inbound message; the LLM picks it up on the
//	next turn.
//
// This is a non-blocking design — sufficient for the IM gateway's
// fire-and-respond loop. A future hotfix can introduce true blocking
// (event-bus rendezvous) if a CLI / REPL adapter is added later.
package surface

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// AskUserQuestionSender is the bridge between the surface and the
// communication layer. The bootstrap wires this to a function that
// delivers the formatted question to the user via the gateway's
// RouteOutbound. nil means "no IM sender wired" — the tool still
// returns a result so the LLM can see what it asked.
//
// DM-20260618-006: parallels BackgroundTaskToolsDeps — a global
// configuration hook installed once at engine bootstrap.
type AskUserQuestionSender func(ctx context.Context, sessionID, formattedQuestion string) error

// globalAskUserQuestionSender is the process-wide sender. Set by
// bootstrap via SetAskUserQuestionSender.
var (
	globalAskUserQuestionSenderMu sync.RWMutex
	globalAskUserQuestionSender   AskUserQuestionSender
)

// SetAskUserQuestionSender installs the global sender. Pass nil to
// disable the wiring (the tool will still execute but will not push
// the question to the user).
func SetAskUserQuestionSender(s AskUserQuestionSender) {
	globalAskUserQuestionSenderMu.Lock()
	globalAskUserQuestionSender = s
	globalAskUserQuestionSenderMu.Unlock()
}

// AskUserQuestionSurface exposes the ask_user_question tool. The
// surface is stateless — the sender is held in a process-global so
// callers don't have to thread it through the engine composition.
type AskUserQuestionSurface struct{}

// NewAskUserQuestionSurface returns a stateless ask surface.
func NewAskUserQuestionSurface() *AskUserQuestionSurface { return &AskUserQuestionSurface{} }

// Name implements contracts.ToolSurface.
func (s *AskUserQuestionSurface) Name() string { return "ask_user_question" }

// Tools implements contracts.ToolSurface.
func (s *AskUserQuestionSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	rOnly, dest, openW, concSafe := OrthogonalFlagFor("ask_user_question")
	return []contracts.ToolSpec{{
		Name:            "ask_user_question",
		Description:     "Ask the user one to four multiple-choice questions. The question is delivered as a formatted IM message with numbered options; the user can reply with a number (e.g. '1') or with the option label. Their reply arrives on the next turn. Use this when the LLM is genuinely uncertain (e.g. ambiguous user request, tool choice, design trade-off). Do NOT use it for trivial clarifications — just ask in plain text.",
		Parameters: `{
			"type": "object",
			"required": ["questions"],
			"properties": {
				"questions": {
					"type": "array",
					"minItems": 1,
					"maxItems": 4,
					"items": {
						"type": "object",
						"required": ["question", "options"],
						"properties": {
							"question": {"type": "string", "description": "The question to ask. End with a question mark."},
							"header": {"type": "string", "maxLength": 12, "description": "Short chip label (max 12 chars)"},
							"options": {
								"type": "array",
								"minItems": 2,
								"maxItems": 4,
								"items": {
									"type": "object",
									"required": ["label", "description"],
									"properties": {
										"label": {"type": "string", "description": "1-5 word option label"},
										"description": {"type": "string", "description": "Explanation of the option"}
									}
								}
							},
							"multi_select": {"type": "boolean", "default": false}
						}
					}
				}
			}
		}`,
		Risk:            types.RiskLevelLow,
		ReadOnly:        rOnly,
		Destructive:     dest,
		OpenWorld:       openW,
		ConcurrencySafe: concSafe,
	}}
}

// InterruptBehavior implements contracts.ToolSurface. Long-run — the
// sender may wait for a write to the IM gateway. Cancels on ctx.Done().
func (s *AskUserQuestionSurface) InterruptBehavior(_ string) contracts.InterruptMode {
	return contracts.InterruptCancel
}

// RiskLevel implements contracts.ToolSurface.
func (s *AskUserQuestionSurface) RiskLevel(name string) types.RiskLevel {
	if name == "ask_user_question" {
		return types.RiskLevelLow
	}
	return ""
}

// CheckPermission implements contracts.ToolSurface. Pure interactive
// Q&A — always Allow.
func (s *AskUserQuestionSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}

// QuestionOption mirrors a single multiple-choice option.
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// Question mirrors a single question block.
type Question struct {
	Question    string           `json:"question"`
	Header      string           `json:"header,omitempty"`
	Options     []QuestionOption `json:"options"`
	MultiSelect bool             `json:"multi_select,omitempty"`
}

// askUserQuestionInput is the validated input to Execute.
type askUserQuestionInput struct {
	Questions []Question `json:"questions"`
}

// askUserQuestionOutput is the JSON the tool returns to the LLM.
type askUserQuestionOutput struct {
	Delivered    bool   `json:"delivered"`
	SentAt       string `json:"sent_at"`
	QuestionText string `json:"question_text"`
	// Echo of the validated questions so the LLM can confirm what it
	// asked without re-parsing its own input.
	Questions []Question `json:"questions"`
	// Hint shown to the LLM so it knows how to interpret the user's
	// next reply.
	Hint string `json:"hint"`
}

const askUserQuestionHint = "Question sent. The user can reply with the option number (e.g. '1'), the option label, or free text. Their reply will be in the next user message; pick up from there."

// Execute implements contracts.ToolSurface. Validates the questions,
// renders them as a formatted IM message, sends via the global sender,
// and returns the JSON result to the LLM.
func (s *AskUserQuestionSurface) Execute(ctx context.Context, _, input, _ string) (*contracts.ToolResult, error) {
	var in askUserQuestionInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return &contracts.ToolResult{Error: fmt.Sprintf("ask_user_question: invalid input JSON: %s", err.Error())}, nil
	}
	if err := validateQuestions(in.Questions); err != nil {
		return &contracts.ToolResult{Error: fmt.Sprintf("ask_user_question: %s", err.Error())}, nil
	}

	sessionID := toolrunner.ToolSessionIDFromContext(ctx)

	formatted := renderQuestionsForIM(in.Questions)

	sender := currentAskSender()
	delivered := false
	if sender != nil && sessionID != "" {
		sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := sender(sendCtx, sessionID, formatted); err != nil {
			return &contracts.ToolResult{Error: fmt.Sprintf("ask_user_question: send failed: %s", err.Error())}, nil
		}
		delivered = true
	}

	out := askUserQuestionOutput{
		Delivered:    delivered,
		SentAt:       time.Now().UTC().Format(time.RFC3339),
		QuestionText: formatted,
		Questions:    in.Questions,
		Hint:         askUserQuestionHint,
	}
	bz, _ := json.Marshal(out)
	return &contracts.ToolResult{Output: string(bz)}, nil
}

func currentAskSender() AskUserQuestionSender {
	globalAskUserQuestionSenderMu.RLock()
	s := globalAskUserQuestionSender
	globalAskUserQuestionSenderMu.RUnlock()
	return s
}

func validateQuestions(qs []Question) error {
	if len(qs) == 0 {
		return fmt.Errorf("at least 1 question is required")
	}
	if len(qs) > 4 {
		return fmt.Errorf("at most 4 questions allowed (got %d)", len(qs))
	}
	for i, q := range qs {
		if strings.TrimSpace(q.Question) == "" {
			return fmt.Errorf("questions[%d].question is required", i)
		}
		if len(q.Options) < 2 {
			return fmt.Errorf("questions[%d].options must have at least 2 entries (got %d)", i, len(q.Options))
		}
		if len(q.Options) > 4 {
			return fmt.Errorf("questions[%d].options must have at most 4 entries (got %d)", i, len(q.Options))
		}
		seen := map[string]bool{}
		for j, o := range q.Options {
			if strings.TrimSpace(o.Label) == "" {
				return fmt.Errorf("questions[%d].options[%d].label is required", i, j)
			}
			if seen[o.Label] {
				return fmt.Errorf("questions[%d].options[%d].label %q is duplicated", i, j, o.Label)
			}
			seen[o.Label] = true
		}
		if len(q.Header) > 12 {
			return fmt.Errorf("questions[%d].header exceeds 12 chars (got %d)", i, len(q.Header))
		}
	}
	return nil
}

// renderQuestionsForIM formats the questions as a plain-text IM message.
// Numbered options, "Other" hint at the end (auto-provided by clawcode's
// UI; we surface it explicitly because IM has no auto-Other).
func renderQuestionsForIM(qs []Question) string {
	var b strings.Builder
	for i, q := range qs {
		if q.Header != "" {
			fmt.Fprintf(&b, "【%s】\n", q.Header)
		}
		b.WriteString(q.Question)
		if q.MultiSelect {
			b.WriteString(" (可多选)")
		}
		b.WriteString("\n")
		for j, o := range q.Options {
			fmt.Fprintf(&b, "  %d. %s — %s\n", j+1, o.Label, o.Description)
		}
		b.WriteString("  其他. 直接回复你的想法\n")
		if i < len(qs)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n回复序号 (例如 1) 或选项文字即可。")
	return b.String()
}

// askSessionIDKey is no longer used — the surface reads the session id
// via toolrunner.ToolSessionIDFromContext, the same path every other
// surface uses. Kept as a private type for future cross-package handoff
// experiments; safe to remove if the design stays.
type askSessionIDKey struct{}
