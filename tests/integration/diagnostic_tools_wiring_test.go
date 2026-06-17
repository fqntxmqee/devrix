//go:build integration

package integration

// W14 — 集成测试 + E2E IM 验证脚本 (DM-20260617-002)
//
// AC18, AC20: 全量 wiring 闭环验证
//   - A1 /doctor CLI: 7 项 check
//   - A5 /context analyze CLI: 5 类 token 拆分
//   - G4 verify_plan_execution LLM tool: 验证 tasks.md done item
//   - G5 free_fork LLM tool: factory + GlobalForker
//   - G6 query_diagnostics LLM tool: tracker tick → query
//   - A6 ErrorClassify: errors.As 仍能拿到 LLMError after WithShortStack
//   - A7 ShortStack: WithShortStack 不让 stack 包含 panic
//   - A3 Transcript: ExpireSession → jsonl 写入
//   - G3 Notify: bus publish → FormatReminder 渲染

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contextanalyze "github.com/devrix/devrix/internal/cli/context_analyze"
	doctorcli "github.com/devrix/devrix/internal/cli/doctor"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/communication/capture/transcript"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/surface"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel/notify"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// TestIntegration_A1_DoctorCLI — A1 闭环: /doctor 输出 7 项 check。
func TestIntegration_A1_DoctorCLI(t *testing.T) {
	tmp := t.TempDir()
	out := captureStdoutForTest(t, func() error {
		return doctorcli.Run([]string{"--workdir=" + tmp})
	})
	for _, c := range []string{
		"install_paths", "config_yaml_valid", "lsp_servers_reachable",
		"workdir_writable", "observability_ready", "tool_count", "transcript_dir_ok",
	} {
		if !strings.Contains(out, c) {
			t.Errorf("A1: doctor table missing %q\n%s", c, out)
		}
	}
}

// TestIntegration_A5_ContextAnalyzeCLI — A5 闭环: /context analyze 输出 5 类 token 拆分。
func TestIntegration_A5_ContextAnalyzeCLI(t *testing.T) {
	tmp := t.TempDir()
	lines := []string{
		`{"role":"system","content":"sys"}`,
		`{"role":"user","content":"hi"}`,
		`{"role":"assistant","content":"<thinking>reasoning</thinking> answer"}`,
		`{"role":"tool","content":"{}"}`,
		`{"role":"assistant","content":"<reminder>please wait</reminder>"}`,
	}
	p := filepath.Join(tmp, "messages.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := captureStdoutForTest(t, func() error {
		return contextanalyze.Run([]string{"--messages-file=" + p})
	})
	for _, c := range []string{"system", "messages", "tools", "thinking", "reminders", "total"} {
		if !strings.Contains(out, c) {
			t.Errorf("A5: context-analyze missing %q\n%s", c, out)
		}
	}
}

// TestIntegration_G4_VerifyTool — G4 闭环: tasks.md done items 验证。
func TestIntegration_G4_VerifyTool(t *testing.T) {
	tmp := t.TempDir()
	tasksMD := filepath.Join(tmp, "openspec", "changes", "demo-change", "tasks.md")
	if err := os.MkdirAll(filepath.Dir(tasksMD), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	tasks := `# Test Plan
- [x] W1.1 done — ` + "`/tmp/evidence.txt`" + `
- [x] W1.2 done — ` + "`/tmp/test_evidence_int.go`" + `
`
	if err := os.WriteFile(tasksMD, []byte(tasks), 0o644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}
	if err := os.WriteFile("/tmp/evidence.txt", []byte("ok"), 0o644); err != nil {
		t.Fatalf("write ev: %v", err)
	}
	if err := os.WriteFile("/tmp/test_evidence_int.go", []byte("package x\nfunc TestFoo(t *testing.T){}\n"), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove("/tmp/test_evidence_int.go")
	})

	// W11 phase 2c: verify_plan_execution is now exposed via surface.VerifySurface
	// (TOOL-SURFACE-1 SoT). The integration test goes through the surface
	// Execute path instead of toolrunner.RegisterVerifyTool + reg.Execute.
	s := surface.NewVerifySurface()
	res, err := s.Execute(context.Background(), "verify_plan_execution", `{"change_id":"demo-change","repo_root":"`+tmp+`"}`, "")
	if err != nil {
		t.Fatalf("surface.Execute: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("verify error: %s", res.Error)
	}
	if !strings.Contains(res.Output, `"verified"`) {
		t.Errorf("G4: missing verified field, output=%s", res.Output)
	}
}

// TestIntegration_G5_FreeForkTool — G5 闭环: free_fork tool 通过 surface 注入函数。
//
// W11 phase 2c: free_fork 的实现已从 toolrunner.freeforkRunner +
// toolrunner.SetFreeForker(global) 迁移到 surface.FreeForkSurface。FreeForkerFunc
// 现在显式传给 surface 构造函数, integration test 直接构造 surface 走 Execute 路径。
func TestIntegration_G5_FreeForkTool(t *testing.T) {
	factory := &fakeFactory{}
	forker := func(_ context.Context, parentSession string, reqs []toolrunner.FreeForkRequestDTO) ([]toolrunner.FreeForkHandleDTO, error) {
		_ = parentSession
		handles := make([]toolrunner.FreeForkHandleDTO, 0, len(reqs))
		for _, r := range reqs {
			_ = r
			// 走 factory: 复用 fakeFactory 的 Create 计数
			if _, err := factory.Create(context.Background(), multiagent.AgentConfig{}, nil); err != nil {
				return nil, err
			}
			handles = append(handles, toolrunner.FreeForkHandleDTO{
				AgentID: "agent-x",
				Name:    "stub",
			})
		}
		return handles, nil
	}

	s := surface.NewFreeForkSurface(forker)
	res, err := s.Execute(context.Background(), "free_fork", `{"parent_session":"sess-int","requests":[{"name":"r1","prompt":"p1","worktree":true}]}`, "")
	if err != nil {
		t.Fatalf("surface.Execute: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("free_fork error: %s", res.Error)
	}
	if !strings.Contains(res.Output, `"spawned_count":1`) {
		t.Errorf("G5: expected spawned_count=1, got %s", res.Output)
	}
	if factory.created != 1 {
		t.Errorf("G5: factory.create count = %d, want 1", factory.created)
	}
}

// TestIntegration_G6_QueryDiagnosticsTool — G6 闭环: tick → query。
//
// W11 phase 2a: query_diagnostics 的实现已从 toolrunner.trackerRunner +
// tracker.SetGlobalTracker 迁移到 surface.TrackerSurface。integration test
// 现在直接构造 surface 走 Execute 路径, 不再依赖任何 process-wide global。
func TestIntegration_G6_QueryDiagnosticsTool(t *testing.T) {
	tr := tracker.New(0)
	tr.SetLinter(".go", func(_ context.Context, _ string) ([]tracker.Diagnostic, error) {
		return []tracker.Diagnostic{
			{File: "a.go", Line: 1, Severity: "error", Message: "m", Source: "go-vet"},
		}, nil
	})
	tr.WatchFile("/tmp/a.go")
	added := tr.TickOnce(context.Background())
	if added != 1 {
		t.Fatalf("tick added=%d, want 1", added)
	}

	s := surface.NewTrackerSurface(tr)
	res, err := s.Execute(context.Background(), "query_diagnostics", `{}`, "")
	if err != nil {
		t.Fatalf("surface.Execute: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("query_diagnostics error: %s", res.Error)
	}
	if !strings.Contains(res.Output, `"count":1`) {
		t.Errorf("G6: expected count=1, got %s", res.Output)
	}
}

// TestIntegration_A3_TranscriptOnSessionClose — A3 闭环: ExpireSession 写 transcript。
func TestIntegration_A3_TranscriptOnSessionClose(t *testing.T) {
	tmp := t.TempDir()
	tw, err := transcript.NewWriter(filepath.Join(tmp, "transcripts"))
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	store, err := capture.NewFileSessionStore(filepath.Join(tmp, "sessions"))
	if err != nil {
		t.Fatalf("NewFileSessionStore: %v", err)
	}
	sess := types.NewSession("sess-int-3", "cli", "/tmp")
	if err := store.Create(sess); err != nil {
		t.Fatalf("Create: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, tw)
	if err := gw.ExpireSession("sess-int-3"); err != nil {
		t.Fatalf("ExpireSession: %v", err)
	}
	events, err := tw.LoadReader("sess-int-3")
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("A3: no events written")
	}
	if events[len(events)-1].Kind != "session_close" {
		t.Errorf("A3: last event kind = %q, want session_close", events[len(events)-1].Kind)
	}
}

// TestIntegration_G3_NotifyPrompt — G3 闭环: notify publish → FormatReminder 渲染。
func TestIntegration_G3_NotifyPrompt(t *testing.T) {
	prev := notify.GlobalBus()
	notify.SetGlobalBus(notify.NewInMemoryBus(8))
	t.Cleanup(func() { notify.SetGlobalBus(prev) })
	notify.GlobalBus().Publish("sess-int-3", notify.CompletionEvent{
		TaskID:  "task-int",
		Kind:    "bash",
		Summary: "did work",
	})
	out := notify.FormatReminder(notify.GlobalBus().Drain("sess-int-3"))
	if !strings.Contains(out, "<task_notifications>") {
		t.Errorf("G3: expected <task_notifications> block, got %q", out)
	}
	if !strings.Contains(out, "task-int") {
		t.Errorf("G3: expected task-int in block, got %q", out)
	}
}

// TestIntegration_A6_ErrorClassify — A6 闭环: errors.As 仍能拿到 LLMError after WithShortStack。
func TestIntegration_A6_ErrorClassify(t *testing.T) {
	bizErr := sharederrors.NewLLMAuthFailedError(errors.New("401 unauthorized"))
	wrapped := sharederrors.WithShortStack(bizErr, 3)
	var llmErr *sharederrors.LLMError
	if !errors.As(wrapped, &llmErr) {
		t.Errorf("A6: expected errors.As to extract LLMError from wrapped chain, got %T", wrapped)
	}
	if llmErr != nil && llmErr.Code != sharederrors.CodeLLMAuthFailed {
		t.Errorf("A6: LLMError code = %q, want %q", llmErr.Code, sharederrors.CodeLLMAuthFailed)
	}
}

// TestIntegration_A7_ShortStack — A7 闭环: WithShortStack 过滤 runtime/testing/reflect 帧。
func TestIntegration_A7_ShortStack(t *testing.T) {
	wrapped := sharederrors.WithShortStack(errors.New("base error"), 5)
	msg := wrapped.Error()
	if !strings.Contains(msg, "base error") {
		t.Errorf("A7: wrapped must contain base error, got %q", msg)
	}
	// 验证格式化时 stack 行不含 runtime.* / testing.*
	if strings.Contains(msg, "runtime.") || strings.Contains(msg, "testing.") {
		t.Errorf("A7: stack should filter runtime/testing frames, got %q", msg)
	}
}

// === helpers ===

func captureStdoutForTest(t *testing.T, fn func() error) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fnErr := fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if fnErr != nil {
		t.Logf("capture stdout fn returned: %v", fnErr)
	}
	return buf.String()
}

// === fake factory for G5 ===

type fakeFactory struct {
	created int
}

func (f *fakeFactory) Create(_ context.Context, _ multiagent.AgentConfig, _ *types.Session) (multiagent.Agent, error) {
	f.created++
	return &fakeAgent{id: "fake-1"}, nil
}

func (f *fakeFactory) ReleaseSession(_ string) {}

type fakeAgent struct{ id string }

func (a *fakeAgent) ID() string                     { return a.id }
func (a *fakeAgent) State() multiagent.AgentState   { return multiagent.AgentStateRunning }
func (a *fakeAgent) Config() multiagent.AgentConfig { return multiagent.AgentConfig{} }
func (a *fakeAgent) Run(_ context.Context) (*multiagent.AgentResult, error) {
	return &multiagent.AgentResult{ExitCode: 0}, nil
}
func (a *fakeAgent) Fork(_ context.Context, _ multiagent.AgentConfig) (multiagent.Agent, error) {
	return nil, nil
}
func (a *fakeAgent) Join(_ context.Context, _ multiagent.Agent) error { return nil }
func (a *fakeAgent) Terminate(_ context.Context) error                { return nil }
func (a *fakeAgent) Wait(_ context.Context) (*multiagent.AgentResult, error) {
	return &multiagent.AgentResult{ExitCode: 0}, nil
}
func (a *fakeAgent) ResolvePermission(string, bool)                  {}
func (a *fakeAgent) GetMessages() []types.Message                    { return nil }
func (a *fakeAgent) SetAgentObserver(multiagent.AgentObserver)       {}
func (a *fakeAgent) SetEngineEventSink(func(*contracts.EngineEvent)) {}
