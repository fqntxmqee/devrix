package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

// fakeProcess 模拟一个 LSP server 进程：echo 给定响应到 stdout，可写 stdin。
type fakeProcess struct {
	stdin  *pipe
	stdout *pipe
	closed bool
}

func newFakeProcess() *fakeProcess {
	return &fakeProcess{
		stdin:  newPipe(),
		stdout: newPipe(),
	}
}

func (f *fakeProcess) Stdin() io.Writer  { return f.stdin }
func (f *fakeProcess) Stdout() io.Reader { return f.stdout }
func (f *fakeProcess) Wait() error       { return nil }
func (f *fakeProcess) Kill() error       { f.closed = true; return nil }

// simple in-memory pipe
type pipe struct {
	mu     sync.Mutex
	buf    []byte
	cond   *sync.Cond
	closed bool
}

func newPipe() *pipe {
	p := &pipe{}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *pipe) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return 0, errors.New("write to closed pipe")
	}
	p.buf = append(p.buf, b...)
	p.cond.Broadcast()
	return len(b), nil
}

func (p *pipe) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.buf) == 0 && !p.closed {
		p.cond.Wait()
	}
	if len(p.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(b, p.buf)
	p.buf = p.buf[n:]
	return n, nil
}

// fakeLauncher 返回预制的 fakeProcess，并可选地 echo 一些响应。
type fakeLauncher struct {
	mu     sync.Mutex
	procs  []*fakeProcess
	// respondMap: method -> canned response JSON
	respondMap map[string]json.RawMessage
}

func newFakeLauncher() *fakeLauncher {
	return &fakeLauncher{respondMap: make(map[string]json.RawMessage)}
}

func (f *fakeLauncher) Launch(ctx context.Context, argv []string, env []string) (Process, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	proc := newFakeProcess()
	f.procs = append(f.procs, proc)
	// 启动 echo goroutine
	go f.handle(proc)
	return proc, nil
}

func (f *fakeLauncher) handle(proc *fakeProcess) {
	reader := bufio.NewReader(proc.stdin)
	for {
		body, err := readMessageFromReader(reader)
		if err != nil {
			return
		}
		var req struct {
			ID     int64  `json:"id"`
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &req)
		var resp []byte
		if req.Method == "initialize" {
			resp = mustResp(req.ID, map[string]any{
				"capabilities": map[string]any{},
			})
		} else if v, ok := f.respondMap[req.Method]; ok {
			// allow override
			resp = mustRespRaw(req.ID, v)
		} else {
			resp = mustResp(req.ID, nil)
		}
		_ = writeMessage(proc.stdout, json.RawMessage(resp))
	}
}

func readMessageFromReader(r *bufio.Reader) ([]byte, error) {
	return readMessage(r)
}

func mustResp(id int64, result any) []byte {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	return b
}

func mustRespRaw(id int64, raw json.RawMessage) []byte {
	wrapped := map[string]json.RawMessage{
		"jsonrpc": json.RawMessage(`"2.0"`),
		"id":      json.RawMessage(jsonNumber(id)),
		"result":  raw,
	}
	b, _ := json.Marshal(wrapped)
	return b
}

func jsonNumber(n int64) string {
	bb, _ := json.Marshal(n)
	return string(bb)
}

// TestManager_AcquireFirstCall — 第一次 Acquire 启动新 client。
func TestManager_AcquireFirstCall(t *testing.T) {
	launcher := newFakeLauncher()
	mgr := NewManager(launcher, 2, 1*time.Second, 1*time.Second)
	servers := []ServerConfig{
		{LanguageID: "go", Command: []string{"fake"}, FilePattern: []string{"*.go"}},
	}
	client, err := mgr.Acquire(context.Background(), "/tmp/foo.go", servers)
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
	if mgr.Len() != 1 {
		t.Fatalf("expected 1 active client, got %d", mgr.Len())
	}
}

// TestManager_AcquireReusesSameLang — 同一 langID 多次 Acquire 复用 client。
func TestManager_AcquireReusesSameLang(t *testing.T) {
	launcher := newFakeLauncher()
	mgr := NewManager(launcher, 2, 1*time.Second, 1*time.Second)
	servers := []ServerConfig{
		{LanguageID: "go", Command: []string{"fake"}, FilePattern: []string{"*.go"}},
	}
	c1, _ := mgr.Acquire(context.Background(), "/tmp/foo.go", servers)
	c2, _ := mgr.Acquire(context.Background(), "/tmp/bar.go", servers)
	if c1 != c2 {
		t.Fatal("expected same client for same language+root")
	}
	if mgr.Len() != 1 {
		t.Fatalf("expected 1 client, got %d", mgr.Len())
	}
}

// TestManager_LRUEvictsOldest — cap=2 满后再次 Acquire 触发 LRU 淘汰。
func TestManager_LRUEvictsOldest(t *testing.T) {
	launcher := newFakeLauncher()
	mgr := NewManager(launcher, 2, 1*time.Second, 1*time.Second)
	servers := []ServerConfig{
		{LanguageID: "go", Command: []string{"fake"}, FilePattern: []string{"*.go"}},
		{LanguageID: "ts", Command: []string{"fake"}, FilePattern: []string{"*.ts"}},
		{LanguageID: "py", Command: []string{"fake"}, FilePattern: []string{"*.py"}},
	}
	_, _ = mgr.Acquire(context.Background(), "/tmp/a.go", servers)
	_, _ = mgr.Acquire(context.Background(), "/tmp/a.ts", servers)
	_, _ = mgr.Acquire(context.Background(), "/tmp/a.py", servers) // 触发淘汰
	if mgr.Len() != 2 {
		t.Fatalf("expected len=2, got %d", mgr.Len())
	}
}

// TestManager_ShutdownClosesAll — Shutdown 关闭所有 client。
func TestManager_ShutdownClosesAll(t *testing.T) {
	launcher := newFakeLauncher()
	mgr := NewManager(launcher, 2, 1*time.Second, 1*time.Second)
	servers := []ServerConfig{
		{LanguageID: "go", Command: []string{"fake"}, FilePattern: []string{"*.go"}},
	}
	_, _ = mgr.Acquire(context.Background(), "/tmp/a.go", servers)
	if err := mgr.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if mgr.Len() != 0 {
		t.Fatalf("expected 0 after Shutdown, got %d", mgr.Len())
	}
}

// TestManager_NoMatchingServer — 文件无匹配 server → 错误。
func TestManager_NoMatchingServer(t *testing.T) {
	launcher := newFakeLauncher()
	mgr := NewManager(launcher, 2, 1*time.Second, 1*time.Second)
	servers := []ServerConfig{
		{LanguageID: "go", Command: []string{"fake"}, FilePattern: []string{"*.go"}},
	}
	_, err := mgr.Acquire(context.Background(), "/tmp/foo.rs", servers)
	if err == nil {
		t.Fatal("expected error for unmatched file type")
	}
}

// TestPickServer_FirstMatch — pickServer 返回第一个匹配的 server。
func TestPickServer_FirstMatch(t *testing.T) {
	servers := []ServerConfig{
		{LanguageID: "ts", Command: []string{"x"}, FilePattern: []string{"*.ts"}},
		{LanguageID: "tsx", Command: []string{"y"}, FilePattern: []string{"*.tsx"}},
	}
	got, err := pickServer("main.tsx", servers)
	if err != nil {
		t.Fatal(err)
	}
	if got.LanguageID != "tsx" {
		t.Fatalf("expected tsx, got %s", got.LanguageID)
	}
}

// TestLanguageForFile — 常见扩展名识别。
func TestLanguageForFile(t *testing.T) {
	cases := map[string]string{
		"a.go":  "go",
		"a.ts":  "typescript",
		"a.tsx": "typescriptreact",
		"a.py":  "python",
		"a.rs":  "rust",
		"a.unknown": "",
	}
	for f, want := range cases {
		if got := LanguageForFile(f); got != want {
			t.Errorf("%s: expected %q, got %q", f, want, got)
		}
	}
}

// TestFileRootURI — 转换 file:// URL。
func TestFileRootURI(t *testing.T) {
	uri := fileRootURI("/tmp/foo/bar.go")
	if uri != "file:///tmp/foo" {
		t.Fatalf("unexpected uri: %q", uri)
	}
}

// TestRPC_RequestResponse — RPC request/response 流正常。
func TestRPC_RequestResponse(t *testing.T) {
	proc := newFakeProcess()
	reader := bufio.NewReader(proc.stdout)
	rpc := newRPCClient("test", proc, reader)

	go func() {
		// 等客户端发送请求，再回响应
		body, err := readMessageFromReader(bufio.NewReader(proc.stdin))
		if err != nil {
			t.Errorf("read err: %v", err)
			return
		}
		_ = body
		// 解析 id
		var req struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(body, &req)
		resp := mustResp(req.ID, "hello")
		_ = writeMessage(proc.stdout, json.RawMessage(resp))
	}()

	resp, err := rpc.request("ping", nil, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if string(*resp) != `"hello"` {
		t.Fatalf("expected hello, got %s", string(*resp))
	}
}

// TestRPC_Timeout — 慢响应触发超时。
func TestRPC_Timeout(t *testing.T) {
	proc := newFakeProcess()
	reader := bufio.NewReader(proc.stdout)
	rpc := newRPCClient("test", proc, reader)
	// 不读 stdin → 永不响应
	_ = proc
	_, err := rpc.request("ping", nil, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
