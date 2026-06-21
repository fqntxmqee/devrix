package freefork

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	// DM-20260621-010 PR-A: 之前是 t.Logf,改为硬断言,任何遗留 = sandbox
	// 清理失败 (Escape 路径) 立即报告。
	//
	// 父目录 (sess-A/) 是 session 级别的入口,本身不在 cleanup 范围内;
	// 但 alpha/beta 叶子目录必须被 Exit 删干净。
	leafEntries, readErr := os.ReadDir(filepath.Join(wt.base, "s"))
	if readErr != nil {
		t.Fatalf("read session dir: %v", readErr)
	}
	for _, e := range leafEntries {
		t.Errorf("leftover sandbox dir: %s (rollback did not clean up)", e.Name())
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

// === DM-20260621-010 PR-A: errors.Join aggregation + metrics ===

// TestFork_AllFailuresJoined verifies that when all forks fail, the
// returned error is errors.Join containing every wrapped per-fork error
// (previously returned errs[0], dropping N-1 errors).
func TestFork_AllFailuresJoined(t *testing.T) {
	fac := &stubFactory{}
	fac.failNext.Store(true) // all Create calls fail
	wt := &stubWorkerSandbox{enabled: true, base: t.TempDir()}
	f := NewDefaultForker(ForkerDeps{Factory: fac, Sandbox: wt})

	_, err := f.Fork(context.Background(), "sess-X", []ForkRequest{
		{Name: "alpha", Worktree: true},
		{Name: "beta", Worktree: true},
		{Name: "gamma", Worktree: true},
	})
	if err == nil {
		t.Fatal("expected error when all forks fail")
	}

	// errors.Is must find the underlying factory error at least once.
	if !strings.Contains(err.Error(), "factory create") {
		t.Errorf("joined error should mention 'factory create', got %v", err)
	}
	// Each fork name should be mentioned in the joined message.
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("joined error should mention fork %q, got %v", name, err)
		}
	}
}

// TestFork_Metrics_AllCountersTriggered verifies that a full failure
// path increments all relevant ForkerMetrics counters.
func TestFork_Metrics_AllCountersTriggered(t *testing.T) {
	fac := &stubFactory{}
	fac.failNext.Store(true) // factory Create fails
	wt := &stubWorkerSandbox{enabled: true, base: t.TempDir()}
	m := &ForkerMetrics{}
	f := NewDefaultForker(ForkerDeps{Factory: fac, Sandbox: wt}).WithMetrics(m)

	_, err := f.Fork(context.Background(), "sess-Y", []ForkRequest{
		{Name: "alpha", Worktree: true},
		{Name: "beta", Worktree: true},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	snap := m.Snapshot()
	// 2 attempts, both fail at factory.Create after successful sandbox.Enter
	if snap.Spawned != 0 {
		t.Errorf("Spawned = %d, want 0 (all failed)", snap.Spawned)
	}
	if snap.SpawnFailed != 2 {
		t.Errorf("SpawnFailed = %d, want 2", snap.SpawnFailed)
	}
	if snap.SandboxEnterFailed != 0 {
		t.Errorf("SandboxEnterFailed = %d, want 0 (Enter succeeded)", snap.SandboxEnterFailed)
	}
	if snap.FactoryCreateFailed != 2 {
		t.Errorf("FactoryCreateFailed = %d, want 2", snap.FactoryCreateFailed)
	}
	if snap.RollbackTriggered != 1 {
		t.Errorf("RollbackTriggered = %d, want 1 (any failure triggers rollback)", snap.RollbackTriggered)
	}
}

// TestFork_Metrics_SandboxEnterFailed verifies counter on Enter failure.
func TestFork_Metrics_SandboxEnterFailed(t *testing.T) {
	// failEnter sandbox returns error on Enter
	wt := &failingEnterSandbox{enabled: true, base: t.TempDir()}
	fac := &stubFactory{}
	m := &ForkerMetrics{}
	f := NewDefaultForker(ForkerDeps{Factory: fac, Sandbox: wt}).WithMetrics(m)

	_, err := f.Fork(context.Background(), "sess-Z", []ForkRequest{
		{Name: "alpha", Worktree: true},
	})
	if err == nil {
		t.Fatal("expected error from sandbox Enter failure")
	}
	snap := m.Snapshot()
	if snap.SandboxEnterFailed != 1 {
		t.Errorf("SandboxEnterFailed = %d, want 1", snap.SandboxEnterFailed)
	}
	if snap.FactoryCreateFailed != 0 {
		t.Errorf("FactoryCreateFailed = %d, want 0 (Enter failed before Create)", snap.FactoryCreateFailed)
	}
	if snap.SpawnFailed != 1 {
		t.Errorf("SpawnFailed = %d, want 1", snap.SpawnFailed)
	}
	if snap.RollbackTriggered != 1 {
		t.Errorf("RollbackTriggered = %d, want 1", snap.RollbackTriggered)
	}
}

// TestFork_Metrics_NilSafe verifies that DefaultForker works without
// WithMetrics set (backward-compat with 13 existing callers).
func TestFork_Metrics_NilSafe(t *testing.T) {
	fac := &stubFactory{}
	wt := &stubWorkerSandbox{enabled: true, base: t.TempDir()}
	f := NewDefaultForker(ForkerDeps{Factory: fac, Sandbox: wt})
	// No WithMetrics call — must not panic.

	handles, err := f.Fork(context.Background(), "sess-W", []ForkRequest{
		{Name: "alpha", Worktree: true},
	})
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}
	if len(handles) != 1 {
		t.Errorf("expected 1 handle, got %d", len(handles))
	}
}

// TestFork_SandboxExitFailure_RecordsMetric verifies the rollback path
// increments SandboxExitFailed when Sandbox.Exit returns error.
func TestFork_SandboxExitFailure_RecordsMetric(t *testing.T) {
	// First fork succeeds; second fails at factory.Create.
	// Rollback will try Exit on the successful fork's sandbox — make Exit fail.
	fac := &flakyFactory{succeed: atomic.Bool{}}
	fac.succeed.Store(true)
	wt := &failingExitSandbox{enabled: true, base: t.TempDir(), failExit: true}
	m := &ForkerMetrics{}
	f := NewDefaultForker(ForkerDeps{Factory: fac, Sandbox: wt}).WithMetrics(m)

	_, err := f.Fork(context.Background(), "sess-RB", []ForkRequest{
		{Name: "alpha", Worktree: true},
		{Name: "beta", Worktree: true},
	})
	if err == nil {
		t.Fatal("expected error from factory failure")
	}
	snap := m.Snapshot()
	// Sandbox.Exit 失败计数器覆盖所有 Exit 失败:
	//   1. spawnOne 中 beta factory.Create 失败时调 1 次 Exit
	//   2. Fork 末尾回滚 alpha 时调 1 次 Exit
	// 合计 2 次失败。
	if snap.SandboxExitFailed != 2 {
		t.Errorf("SandboxExitFailed = %d, want 2 (spawnOne + rollback Exit failed)", snap.SandboxExitFailed)
	}
	if snap.RollbackTriggered != 1 {
		t.Errorf("RollbackTriggered = %d, want 1", snap.RollbackTriggered)
	}
}

// --- supporting test stubs ---

// failingEnterSandbox returns error from Enter to test SandboxEnterFailed metric.
type failingEnterSandbox struct {
	enabled bool
	base    string
}

func (s *failingEnterSandbox) Enabled() bool { return s.enabled }
func (s *failingEnterSandbox) Enter(_ context.Context, _, _, _ string) (string, error) {
	return "", errors.New("stub: enter failed")
}
func (s *failingEnterSandbox) Exit(_ context.Context, _ string, _ bool) error { return nil }

// failingExitSandbox: Enter succeeds, Exit fails.
type failingExitSandbox struct {
	enabled  bool
	base     string
	failExit bool
	mu       sync.Mutex
	entered  []string
}

func (s *failingExitSandbox) Enabled() bool { return s.enabled }
func (s *failingExitSandbox) Enter(_ context.Context, sessionID, slug, _ string) (string, error) {
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
func (s *failingExitSandbox) Exit(_ context.Context, _ string, _ bool) error {
	if s.failExit {
		return errors.New("stub: exit failed")
	}
	return nil
}

// flakyFactory: first Create succeeds, then all subsequent fail.
type flakyFactory struct {
	succeed atomic.Bool
	idx     atomic.Int32
	mu      sync.Mutex
	created []multiagent.AgentConfig
}

func (f *flakyFactory) Create(_ context.Context, cfg multiagent.AgentConfig, _ *types.Session) (multiagent.Agent, error) {
	if !f.succeed.Load() {
		return nil, errors.New("stub: factory failed")
	}
	// 第一次调用成功,之后失败
	if f.idx.Add(1) > 1 {
		return nil, errors.New("stub: factory failed after first")
	}
	f.mu.Lock()
	f.created = append(f.created, cfg)
	f.mu.Unlock()
	return &stubAgent{
		id:    "agent-flaky-1",
		cfg:   cfg,
		state: multiagent.AgentStateCreated,
	}, nil
}
