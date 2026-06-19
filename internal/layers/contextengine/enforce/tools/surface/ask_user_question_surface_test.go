package surface_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/surface"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: ASK-Q-T01 — Surface spec correctness (ReadOnly, OpenWorld, NOT
// ConcurrencySafe, NOT Destructive).
func TestAskUserQuestionSurface_Tools(t *testing.T) {
	s := surface.NewAskUserQuestionSurface()
	if s.Name() != "ask_user_question" {
		t.Errorf("Name = %q, want ask_user_question", s.Name())
	}
	specs := s.Tools(context.Background(), "", "sess-a")
	if len(specs) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(specs))
	}
	sp := specs[0]
	if sp.Name != "ask_user_question" {
		t.Errorf("spec.Name = %q, want ask_user_question", sp.Name)
	}
	if !sp.ReadOnly {
		t.Errorf("must be ReadOnly=true")
	}
	if sp.Destructive {
		t.Errorf("must NOT be Destructive")
	}
	if !sp.OpenWorld {
		t.Errorf("must be OpenWorld=true (sends IM message)")
	}
	if sp.ConcurrencySafe {
		t.Errorf("must NOT be ConcurrencySafe (long-run)")
	}
	if sp.Risk != types.RiskLevelLow {
		t.Errorf("Risk = %q, want low", sp.Risk)
	}
	if s.InterruptBehavior("ask_user_question") != contracts.InterruptCancel {
		t.Errorf("InterruptBehavior must be Cancel")
	}
	if d := s.CheckPermission(context.Background(), sp, nil); d != contracts.DecisionAllow {
		t.Errorf("CheckPermission = %v, want Allow", d)
	}
}

// T: ASK-Q-T01b — RiskLevel must not claim tools owned by other surfaces.
func TestAskUserQuestionSurface_RiskLevel_OnlyOwnTool(t *testing.T) {
	s := surface.NewAskUserQuestionSurface()
	if r := s.RiskLevel("ask_user_question"); r != types.RiskLevelLow {
		t.Errorf("RiskLevel(ask_user_question) = %q, want low", r)
	}
	for _, name := range []string{"bash", "read_file", "delegate_explore", "tool_search"} {
		if r := s.RiskLevel(name); r != "" {
			t.Errorf("RiskLevel(%q) = %q, want empty (must not hijack dispatch)", name, r)
		}
	}
}

// T: ASK-Q-T02 — validation: empty questions array.
func TestAskUserQuestionSurface_Execute_EmptyQuestions(t *testing.T) {
	s := surface.NewAskUserQuestionSurface()
	ctx := toolrunner.WithToolSessionID(context.Background(), "sess-v1")
	res, err := s.Execute(ctx, "ask_user_question", `{"questions":[]}`, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Error == "" {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(res.Error, "at least 1 question") {
		t.Errorf("unexpected error: %s", res.Error)
	}
}

// T: ASK-Q-T03 — validation: too many questions (>4).
func TestAskUserQuestionSurface_Execute_TooManyQuestions(t *testing.T) {
	s := surface.NewAskUserQuestionSurface()
	in := `{"questions":[
		{"question":"q1","options":[{"label":"a","description":"x"},{"label":"b","description":"y"}]},
		{"question":"q2","options":[{"label":"a","description":"x"},{"label":"b","description":"y"}]},
		{"question":"q3","options":[{"label":"a","description":"x"},{"label":"b","description":"y"}]},
		{"question":"q4","options":[{"label":"a","description":"x"},{"label":"b","description":"y"}]},
		{"question":"q5","options":[{"label":"a","description":"x"},{"label":"b","description":"y"}]}
	]}`
	res, _ := s.Execute(context.Background(), "ask_user_question", in, "")
	if res.Error == "" {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(res.Error, "at most 4 questions") {
		t.Errorf("unexpected error: %s", res.Error)
	}
}

// T: ASK-Q-T04 — validation: option label required.
func TestAskUserQuestionSurface_Execute_OptionLabelRequired(t *testing.T) {
	s := surface.NewAskUserQuestionSurface()
	in := `{"questions":[
		{"question":"q1","options":[
			{"label":"","description":"x"},
			{"label":"b","description":"y"}
		]}
	]}`
	res, _ := s.Execute(context.Background(), "ask_user_question", in, "")
	if res.Error == "" {
		t.Fatalf("expected validation error")
	}
}

// T: ASK-Q-T05 — validation: duplicate labels rejected.
func TestAskUserQuestionSurface_Execute_DuplicateLabels(t *testing.T) {
	s := surface.NewAskUserQuestionSurface()
	in := `{"questions":[
		{"question":"q1","options":[
			{"label":"same","description":"x"},
			{"label":"same","description":"y"}
		]}
	]}`
	res, _ := s.Execute(context.Background(), "ask_user_question", in, "")
	if res.Error == "" {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(res.Error, "duplicated") {
		t.Errorf("unexpected error: %s", res.Error)
	}
}

// T: ASK-Q-T06 — validation: header length cap (max 12).
func TestAskUserQuestionSurface_Execute_HeaderCap(t *testing.T) {
	s := surface.NewAskUserQuestionSurface()
	in := `{"questions":[
		{"question":"q1","header":"this is way too long","options":[
			{"label":"a","description":"x"},{"label":"b","description":"y"}
		]}
	]}`
	res, _ := s.Execute(context.Background(), "ask_user_question", in, "")
	if res.Error == "" {
		t.Fatalf("expected header-cap error")
	}
	if !strings.Contains(res.Error, "header") {
		t.Errorf("unexpected error: %s", res.Error)
	}
}

// T: ASK-Q-T07 — happy path: 1 question, 3 options, sender installed.
func TestAskUserQuestionSurface_Execute_HappyPath(t *testing.T) {
	var (
		mu          sync.Mutex
		gotSession  string
		gotText     string
		sendCount   int
	)
	surface.SetAskUserQuestionSender(func(_ context.Context, sessionID, text string) error {
		mu.Lock()
		defer mu.Unlock()
		gotSession = sessionID
		gotText = text
		sendCount++
		return nil
	})
	t.Cleanup(func() { surface.SetAskUserQuestionSender(nil) })

	s := surface.NewAskUserQuestionSurface()
	ctx := toolrunner.WithToolSessionID(context.Background(), "sess-h1")
	in := `{"questions":[
		{"question":"你要查的是文件诊断还是工具调用历史？","header":"工具选择","options":[
			{"label":"文件诊断","description":"query_diagnostics (linter 报错)"},
			{"label":"工具调用历史","description":"数 sc.Messages 里的 tool_calls"},
			{"label":"两个都要","description":"同时给两个结果"}
		]}
	]}`
	res, err := s.Execute(ctx, "ask_user_question", in, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}

	mu.Lock()
	defer mu.Unlock()
	if sendCount != 1 {
		t.Errorf("sender called %d times, want 1", sendCount)
	}
	if gotSession != "sess-h1" {
		t.Errorf("sessionID = %q, want sess-h1", gotSession)
	}
	if !strings.Contains(gotText, "1. 文件诊断") {
		t.Errorf("formatted text missing '1. 文件诊断':\n%s", gotText)
	}
	if !strings.Contains(gotText, "3. 两个都要") {
		t.Errorf("formatted text missing '3. 两个都要':\n%s", gotText)
	}
	if !strings.Contains(gotText, "其他") {
		t.Errorf("formatted text missing '其他' auto-Other hint:\n%s", gotText)
	}

	var out struct {
		Delivered    bool   `json:"delivered"`
		Hint         string `json:"hint"`
		QuestionText string `json:"question_text"`
	}
	if err := json.Unmarshal([]byte(res.Output), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.Delivered {
		t.Errorf("Delivered = false, want true")
	}
	if !strings.Contains(out.Hint, "next user message") {
		t.Errorf("Hint = %q, want it to mention next user message", out.Hint)
	}
	if out.QuestionText == "" {
		t.Errorf("QuestionText empty in response")
	}
}

// T: ASK-Q-T08 — no sender wired: still returns success with
// Delivered=false (graceful degradation).
func TestAskUserQuestionSurface_Execute_NoSender(t *testing.T) {
	surface.SetAskUserQuestionSender(nil)
	s := surface.NewAskUserQuestionSurface()
	ctx := toolrunner.WithToolSessionID(context.Background(), "sess-h2")
	in := `{"questions":[
		{"question":"q1","options":[
			{"label":"a","description":"x"},
			{"label":"b","description":"y"}
		]}
	]}`
	res, _ := s.Execute(ctx, "ask_user_question", in, "")
	if res.Error != "" {
		t.Fatalf("expected graceful no-op, got error: %s", res.Error)
	}
	var out struct {
		Delivered bool `json:"delivered"`
	}
	_ = json.Unmarshal([]byte(res.Output), &out)
	if out.Delivered {
		t.Errorf("Delivered = true, want false (no sender)")
	}
}

// T: ASK-Q-T09 — sender error path surfaces as ToolResult.Error.
func TestAskUserQuestionSurface_Execute_SenderError(t *testing.T) {
	surface.SetAskUserQuestionSender(func(_ context.Context, _, _ string) error {
		return errSentinelForTest
	})
	t.Cleanup(func() { surface.SetAskUserQuestionSender(nil) })

	s := surface.NewAskUserQuestionSurface()
	ctx := toolrunner.WithToolSessionID(context.Background(), "sess-h3")
	in := `{"questions":[
		{"question":"q1","options":[
			{"label":"a","description":"x"},
			{"label":"b","description":"y"}
		]}
	]}`
	res, _ := s.Execute(ctx, "ask_user_question", in, "")
	if res.Error == "" {
		t.Fatalf("expected error from sender failure")
	}
	if !strings.Contains(res.Error, "send failed") {
		t.Errorf("unexpected error: %s", res.Error)
	}
}

// T: ASK-Q-T10 — multi-question formatted output is well-structured.
func TestAskUserQuestionSurface_RenderMultiple(t *testing.T) {
	// We test renderQuestionsForIM indirectly via the Execute path
	// since the function is unexported. The fixture mirrors what
	// the IM gateway will receive.
	surface.SetAskUserQuestionSender(func(_ context.Context, _, text string) error {
		if !strings.Contains(text, "【Header A】") {
			t.Errorf("missing header chip:\n%s", text)
		}
		if !strings.Contains(text, "(可多选)") {
			t.Errorf("missing multiSelect marker:\n%s", text)
		}
		return nil
	})
	t.Cleanup(func() { surface.SetAskUserQuestionSender(nil) })

	s := surface.NewAskUserQuestionSurface()
	ctx := toolrunner.WithToolSessionID(context.Background(), "sess-h4")
	in := `{"questions":[
		{"question":"q1","header":"Header A","options":[
			{"label":"a","description":"x"},
			{"label":"b","description":"y"}
		]},
		{"question":"q2","header":"Header B","options":[
			{"label":"c","description":"z"},
			{"label":"d","description":"w"}
		],"multi_select":true}
	]}`
	res, _ := s.Execute(ctx, "ask_user_question", in, "")
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
}

// T: ASK-Q-T11 — invalid JSON input returns a clear error.
func TestAskUserQuestionSurface_Execute_InvalidJSON(t *testing.T) {
	s := surface.NewAskUserQuestionSurface()
	res, _ := s.Execute(context.Background(), "ask_user_question", `not json`, "")
	if res.Error == "" {
		t.Fatalf("expected error")
	}
	if !strings.Contains(res.Error, "invalid input JSON") {
		t.Errorf("unexpected error: %s", res.Error)
	}
}

// T: ASK-Q-T12 — Concurrent SetAskUserQuestionSender / currentAskSender
// are race-free (run with -race).
func TestAskUserQuestionSurface_ConcurrentSetGet(t *testing.T) {
	const N = 100
	var wg sync.WaitGroup
	wg.Add(2 * N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			surface.SetAskUserQuestionSender(func(_ context.Context, _, _ string) error {
				return nil
			})
		}(i)
		go func() {
			defer wg.Done()
			_ = surface.NewAskUserQuestionSurface() // touch surface; sender is global
		}()
	}
	wg.Wait()
}

type sentinelErr struct{ msg string }

func (e *sentinelErr) Error() string { return e.msg }

var errSentinelForTest = &sentinelErr{msg: "mock sender failure"}
