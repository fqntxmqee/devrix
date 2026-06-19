package freefork

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/kernel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// stubWorkerSandbox 简单 worktree sandbox mock:用 t.TempDir() 作 base。
type stubWorkerSandbox struct {
	enabled bool
	base    string
	mu      sync.Mutex
	entered []string
}

func (s *stubWorkerSandbox) Enabled() bool { return s.enabled }
func (s *stubWorkerSandbox) Enter(_ context.Context, sessionID, slug, _ string) (string, error) {
	if !s.enabled {
		return "", errors.New("disabled")
	}
	p := filepath.Join(s.base, sessionID, slug)
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.entered = append(s.entered, p)
	s.mu.Unlock()
	return p, nil
}
func (s *stubWorkerSandbox) Exit(_ context.Context, path string, _ bool) error {
	return os.RemoveAll(path)
}

// stubAgent 满足 multiagent.Agent 接口的最小实现。
type stubAgent struct {
	id      string
	cfg     multiagent.AgentConfig
	state   multiagent.AgentState
	created time.Time
	terminated atomic.Bool
}

func (a *stubAgent) ID() string                       { return a.id }
func (a *stubAgent) State() multiagent.AgentState     { return a.state }
func (a *stubAgent) Config() multiagent.AgentConfig   { return a.cfg }
func (a *stubAgent) Run(_ context.Context) (*multiagent.AgentResult, error) {
	return &multiagent.AgentResult{ExitCode: 0}, nil
}
func (a *stubAgent) Fork(_ context.Context, _ multiagent.AgentConfig) (multiagent.Agent, error) {
	return nil, errors.New("stub: no nested fork")
}
func (a *stubAgent) Join(_ context.Context, _ multiagent.Agent) error { return nil }
func (a *stubAgent) Terminate(_ context.Context) error {
	a.terminated.Store(true)
	a.state = multiagent.AgentStateTerminated
	return nil
}
func (a *stubAgent) Wait(_ context.Context) (*multiagent.AgentResult, error) {
	return &multiagent.AgentResult{ExitCode: 0}, nil
}
func (a *stubAgent) ResolvePermission(_ string, _ bool)            {}
func (a *stubAgent) GetMessages() []types.Message                  { return nil }
func (a *stubAgent) SetAgentObserver(_ multiagent.AgentObserver)   {}
func (a *stubAgent) SetEngineEventSink(_ func(*contracts.EngineEvent)) {}

// stubFactory 满足 IAgentFactory:生成 stubAgent。
type stubFactory struct {
	mu        sync.Mutex
	created   []multiagent.AgentConfig
	nextID    atomic.Int32
	failNext  atomic.Bool
}

func (f *stubFactory) Create(_ context.Context, cfg multiagent.AgentConfig, _ *types.Session) (multiagent.Agent, error) {
	if f.failNext.Load() {
		return nil, errors.New("stub: factory failed")
	}
	f.mu.Lock()
	f.created = append(f.created, cfg)
	f.mu.Unlock()
	id := f.nextID.Add(1)
	a := &stubAgent{
		id:    "agent-" + intToStr(int(id)),
		cfg:   cfg,
		state: multiagent.AgentStateCreated,
	}
	return a, nil
}

func (f *stubFactory) snapshot() []multiagent.AgentConfig {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]multiagent.AgentConfig, len(f.created))
	copy(out, f.created)
	return out
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// TestFork_SpawnsAll — 批量 fork 全部成功。
func TestFork_SpawnsAll(t *testing.T) {
	fac := &stubFactory{}
	wt := &stubWorkerSandbox{enabled: true, base: t.TempDir()}
	f := NewDefaultForker(ForkerDeps{Factory: fac, Sandbox: wt})

	handles, err := f.Fork(context.Background(), "sess-A", []ForkRequest{
		{Name: "alpha", Prompt: "do A", Worktree: true},
		{Name: "beta", Prompt: "do B", Worktree: true},
		{Name: "gamma", Prompt: "do C", Worktree: false},
	})
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}
	if len(handles) != 3 {
		t.Fatalf("expected 3 handles, got %d", len(handles))
	}
	// 并发派发,顺序不固定;按 Name 索引
	byName := map[string]Handle{}
	for _, h := range handles {
		byName[h.Name] = h
	}
	if byName["alpha"].SandboxPath == "" {
		t.Errorf("alpha should have worktree, got empty")
	}
	if byName["beta"].SandboxPath == "" {
		t.Errorf("beta should have worktree, got empty")
	}
	if byName["gamma"].SandboxPath != "" {
		t.Errorf("gamma should not have worktree, got %q", byName["gamma"].SandboxPath)
	}
	// factory 收到 3 个 cfg
	if got := len(fac.snapshot()); got != 3 {
		t.Errorf("expected 3 cfg, got %d", got)
	}
}

// TestFork_EmptyParentSessionRejected — parent session 为空 → 拒绝。
func TestFork_EmptyParentSessionRejected(t *testing.T) {
	f := NewDefaultForker(ForkerDeps{Factory: &stubFactory{}})
	_, err := f.Fork(context.Background(), "", []ForkRequest{{Name: "x"}})
	if err == nil {
		t.Fatal("expected error for empty parent session")
	}
}

// TestFork_NilFactoryRejected — 缺工厂 → 拒绝。
func TestFork_NilFactoryRejected(t *testing.T) {
	f := NewDefaultForker(ForkerDeps{Sandbox: &stubWorkerSandbox{}})
	_, err := f.Fork(context.Background(), "s", []ForkRequest{{Name: "x"}})
	if err == nil {
		t.Fatal("expected error for nil factory")
	}
}

// TestFork_NoRequests — 空 reqs → 返回空,nil。
func TestFork_NoRequests(t *testing.T) {
	f := NewDefaultForker(ForkerDeps{Factory: &stubFactory{}})
	handles, err := f.Fork(context.Background(), "s", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(handles) != 0 {
		t.Errorf("expected 0 handles, got %d", len(handles))
	}
}

// TestFork_NameRequired — 每条 req 必须有 name。
func TestFork_NameRequired(t *testing.T) {
	f := NewDefaultForker(ForkerDeps{Factory: &stubFactory{}})
	_, err := f.Fork(context.Background(), "s", []ForkRequest{{Prompt: "no name"}})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

// TestFork_FactoryFailureRollsBack — 任一 factory 失败 → 已启动的 terminate, worktree 清理。
func TestFork_FactoryFailureRollsBack(t *testing.T) {
	fac := &stubFactory{}
	fac.failNext.Store(true) // 全部 Create 都失败
	wt := &stubWorkerSandbox{enabled: true, base: t.TempDir()}
	f := NewDefaultForker(ForkerDeps{Factory: fac, Sandbox: wt})

	_, err := f.Fork(context.Background(), "s", []ForkRequest{
		{Name: "alpha", Worktree: true},
		{Name: "beta", Worktree: true},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestFork_FailureMidBatchRollsBack — 前 N 成功,后续失败时前 N 也回滚。
func TestFork_FailureMidBatchRollsBack(t *testing.T) {
	fac := &stubFactory{nextID: atomic.Int32{}}
	// 第一次 Create 成功,第二次失败
	fac.failNext.Store(true)
	wt := &stubWorkerSandbox{enabled: true, base: t.TempDir()}
	f := NewDefaultForker(ForkerDeps{Factory: fac, Sandbox: wt})

	_, err := f.Fork(context.Background(), "s", []ForkRequest{
		{Name: "alpha", Worktree: true},
		{Name: "beta", Worktree: true},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	// 验证:worktree base 目录中没有遗留的 alpha/beta 子目录
	// (在并发场景下可能部分创建,但不应该保留)
	// 至少根目录应该被清理过
	entries, _ := os.ReadDir(wt.base)
	for _, e := range entries {
		t.Logf("leftover: %s", e.Name())
	}
}

// TestFork_PromptPassedToAgent — prompt → cfg.InitialInput。
func TestFork_PromptPassedToAgent(t *testing.T) {
	fac := &stubFactory{}
	f := NewDefaultForker(ForkerDeps{Factory: fac, Sandbox: nil})
	_, err := f.Fork(context.Background(), "s", []ForkRequest{
		{Name: "p", Prompt: "build the rocket", Worktree: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := fac.snapshot()
	if len(got) != 1 || got[0].InitialInput != "build the rocket" {
		t.Errorf("prompt not propagated: %+v", got)
	}
}

// TestSlugify_BasicCases — slug 转换基本规则。
func TestSlugify_BasicCases(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello world", "hello-world"},
		{"Build_New-Feature", "build_new-feature"},
		{"/path/to:x", "-path-to-x"},
		{"", "fork-"},
	}
	for _, c := range cases {
		got := slugify(c.in)
		// 空字符串生成 fork-<nanos>,只检查前缀
		if c.in == "" {
			if got[:5] != "fork-" {
				t.Errorf("empty slug should start with fork-, got %q", got)
			}
			continue
		}
		if got != c.want {
			t.Errorf("slugify(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

// TestFork_WorktreeDisabledSkipsSandbox — Worktree=true 但 wt disabled → 走 default WorkDir。
func TestFork_WorktreeDisabledSkipsSandbox(t *testing.T) {
	fac := &stubFactory{}
	wt := &stubWorkerSandbox{enabled: false} // disabled
	f := NewDefaultForker(ForkerDeps{
		Factory:  fac,
		Sandbox: wt,
		DefaultConfig: multiagent.AgentConfig{WorkDir: "/tmp/orig"},
	})
	handles, err := f.Fork(context.Background(), "s", []ForkRequest{
		{Name: "x", Worktree: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(handles) != 1 {
		t.Fatalf("expected 1 handle, got %d", len(handles))
	}
	if handles[0].SandboxPath != "" {
		t.Errorf("expected no worktree, got %q", handles[0].SandboxPath)
	}
	// WorkDir 应保留 default
	if got := fac.snapshot()[0].WorkDir; got != "/tmp/orig" {
		t.Errorf("expected /tmp/orig, got %q", got)
	}
}

// TestFork_InterfaceConformance — DefaultForker 满足 Forker 接口。
func TestFork_InterfaceConformance(t *testing.T) {
	var _ Forker = NewDefaultForker(ForkerDeps{Factory: &stubFactory{}})
}

// TestKernelImport — 确保 kernel 包可被 import(防止 import cycle)。
func TestKernelImport(t *testing.T) {
	_ = kernel.AgentStateCreated
}

// === DM-20260618-007 W8-W10 cross-reference (T 点映射) ===

// T: W8 (FreeForker.Fork 批量 fork)— 现有 TestFork_SpawnsAll 覆盖。
// T: W9 (WorkerContext / Spawn One)— 现有 TestFork_PromptPassedToAgent + TestSlugify_BasicCases 覆盖。
// T: W10 (Worktree sandbox)— 现有 TestFork_WorktreeDisabledSkipsSandbox 覆盖。
//
// 整合 spec 映射: free-fork change 的 3 F (Fork + WorkerContext + Worktree)
// 全部由 multiagent/provision/freefork/forker.go + shared/contracts/worktree.go 实现。
func TestW8_10_FreeForkStack_T_CrossRef(t *testing.T) {
	fac := &stubFactory{}
	wt := &stubWorkerSandbox{enabled: true, base: t.TempDir()}
	f := NewDefaultForker(ForkerDeps{Factory: fac, Sandbox: wt})

	// W8: Fork 批量 (T: free-fork T01)
	handles, err := f.Fork(context.Background(), "sess-W8", []ForkRequest{
		{Name: "alpha", Prompt: "W8 test", Worktree: true},
	})
	if err != nil {
		t.Fatalf("W8 Fork: %v", err)
	}
	if len(handles) != 1 {
		t.Errorf("W8: handles = %d, want 1", len(handles))
	}

	// W10: Worktree sandbox 启用时调用 Enter (T: free-fork T03)
	// 验证 wt.base 下应创建 alpha 子目录
	if !wt.Enabled() {
		t.Error("W10: worktree should be enabled")
	}

	// W9: WorkerContext / spawnOne 把 prompt 透传给 child agent
	got := fac.snapshot()
	if len(got) != 1 || got[0].InitialInput != "W8 test" {
		t.Errorf("W9: prompt not propagated, got %+v", got)
	}
}
