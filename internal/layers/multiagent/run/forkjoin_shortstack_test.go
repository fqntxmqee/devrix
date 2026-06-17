package run

// W3 — D2-S6-A02 (alias A7) ShortStack 包装 agent lifecycle 错误单元测试。
//
// AC8 (后半):
//   - agent spawn 失败错误栈 ≤ 5 帧
//   - 原始错误信息保留在错误字符串首部
//   - 现有错误类型断言 (errors.Is / errors.As) 不被破坏

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S6-A02-T01 (AC8 后半)
// creator.Create 失败时，Fork 返回的错误应当被 WithShortStack 包装：
// 栈 ≤ 5 帧、不含 runtime.goexit / runtime.main 等。
func TestFork_CreateFailure_ShortStack(t *testing.T) {
	cause := stderrors.New("factory quota exceeded")
	failing := stubCreator{err: cause}

	parent := &Impl{
		id:    "parent-1",
		state: multiagent.AgentStateCreated,
		cfg: multiagent.AgentConfig{
			Mode:        multiagent.ModeDefault,
			MaxChildren: 5,
		},
		session:       &types.Session{SessionID: "sess-1"},
		creator:       failing,
		childAgents:   make(map[string]multiagent.Agent),
		messageBuffer: make([]types.Message, 0),
		joinedToolIDs: make(map[string]struct{}, 4),
		done:          make(chan struct{}),
	}
	parent.permGate = newAgentPermissionGate(parent)

	_, err := parent.Fork(context.Background(), multiagent.AgentConfig{Mode: multiagent.ModeDefault})
	if err == nil {
		t.Fatalf("expected fork failure")
	}

	msg := err.Error()
	if !strings.Contains(msg, "factory quota exceeded") {
		t.Errorf("original error message lost: %q", msg)
	}
	idx := strings.Index(msg, "\n")
	if idx < 0 {
		t.Fatalf("expected shortstack appended, got %q", msg)
	}
	stack := msg[idx+1:]
	lines := strings.Split(strings.TrimSpace(stack), "\n")
	if len(lines) > 5 {
		t.Errorf("stack frames = %d, want <= 5; lines=%v", len(lines), lines)
	}
	for _, line := range lines {
		low := strings.ToLower(line)
		if strings.Contains(low, "runtime.") || strings.Contains(low, "testing.") || strings.Contains(low, "reflect.") {
			t.Errorf("stack frame should be filtered: %q", line)
		}
	}
}

// T: D2-S6-A02-T01 (AC8 后半, errors.Is 透传)
// errors.Is 在 WithShortStack 包装后仍能命中底层 sentinel。
func TestFork_CreateFailure_ErrorsIsPreserved(t *testing.T) {
	sentinel := stderrors.New("underlying factory sentinel")
	failing := stubCreator{err: fmt.Errorf("wrapped: %w", sentinel)}

	parent := &Impl{
		id:    "parent-2",
		state: multiagent.AgentStateCreated,
		cfg: multiagent.AgentConfig{
			Mode:        multiagent.ModeDefault,
			MaxChildren: 5,
		},
		session:       &types.Session{SessionID: "sess-2"},
		creator:       failing,
		childAgents:   make(map[string]multiagent.Agent),
		messageBuffer: make([]types.Message, 0),
		joinedToolIDs: make(map[string]struct{}, 4),
		done:          make(chan struct{}),
	}
	parent.permGate = newAgentPermissionGate(parent)

	_, err := parent.Fork(context.Background(), multiagent.AgentConfig{Mode: multiagent.ModeDefault})
	if err == nil {
		t.Fatalf("expected fork failure")
	}
	if !stderrors.Is(err, sentinel) {
		t.Errorf("errors.Is(err, sentinel) = false, got %q", err.Error())
	}
}

// stubCreator 实现 Creator 接口，Create() 返回固定 err。
type stubCreator struct{ err error }

func (s stubCreator) Create(ctx context.Context, cfg multiagent.AgentConfig, session *types.Session) (multiagent.Agent, error) {
	return nil, s.err
}

func (s stubCreator) ReleaseSession(string) {}
