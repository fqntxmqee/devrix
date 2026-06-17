// Package lsp — 手写 LSP base protocol 子集（Content-Length + JSON body）。
//
// 目标：仅实现 design.md §2.1.3 列出的 7 个 LSP method：
//   - initialize / initialized / shutdown / exit
//   - textDocument/didOpen
//   - textDocument/definition
//   - textDocument/references
//   - textDocument/prepareCallHierarchy
//   - callHierarchy/incomingCalls
//
// 双向通知（$/logTrace、window/logMessage、$/progress 等）丢弃。
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Position 0-based per LSP spec.
type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

// Range LSP range.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location 标准 LSP location。
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
	// Preview 上下文 1-3 行（devrix 扩展，便于模型消费）
	Preview string `json:"preview,omitempty"`
}

// SymbolKind LSP SymbolKind enum（与 vscode-languageserver-types 对齐）。
type SymbolKind int

const (
	SymbolKindFile          SymbolKind = 1
	SymbolKindModule        SymbolKind = 2
	SymbolKindNamespace     SymbolKind = 3
	SymbolKindPackage       SymbolKind = 4
	SymbolKindClass         SymbolKind = 5
	SymbolKindMethod        SymbolKind = 6
	SymbolKindProperty      SymbolKind = 7
	SymbolKindField         SymbolKind = 8
	SymbolKindConstructor   SymbolKind = 9
	SymbolKindEnum          SymbolKind = 10
	SymbolKindInterface     SymbolKind = 11
	SymbolKindFunction      SymbolKind = 12
	SymbolKindVariable      SymbolKind = 13
	SymbolKindConstant      SymbolKind = 14
	SymbolKindString        SymbolKind = 15
	SymbolKindNumber        SymbolKind = 16
	SymbolKindBoolean       SymbolKind = 17
	SymbolKindArray         SymbolKind = 18
)

// CallHierarchyItem LSP 调用层级节点。
type CallHierarchyItem struct {
	Name     string     `json:"name"`
	Kind     SymbolKind `json:"kind"`
	URI      string     `json:"uri"`
	Range    Range      `json:"range"`
	Selection Range     `json:"selectionRange"`
}

// CallHierarchyIncomingCall LSP 调用方信息。
type CallHierarchyIncomingCall struct {
	From       CallHierarchyItem `json:"from"`
	FromRanges []Range           `json:"fromRanges"`
}

// Client 单个 LSP server 连接抽象。
type Client interface {
	Initialize(ctx context.Context, rootURI string) error
	DidOpen(ctx context.Context, uri, languageID, text string) error
	Definition(ctx context.Context, uri string, p Position) ([]Location, error)
	References(ctx context.Context, uri string, p Position, includeDecl bool) ([]Location, error)
	PrepareCallHierarchy(ctx context.Context, uri string, p Position) ([]CallHierarchyItem, error)
	IncomingCalls(ctx context.Context, item CallHierarchyItem) ([]CallHierarchyIncomingCall, error)
	Close() error
}

// Process 抽象 D1 sandbox 启动的子进程。
type Process interface {
	Stdin() io.Writer
	Stdout() io.Reader
	Wait() error
	Kill() error
}

// SandboxLauncher 启动 LSP server 子进程。
type SandboxLauncher interface {
	Launch(ctx context.Context, argv []string, env []string) (Process, error)
}

// ExecLauncher — 简单实现：直接 exec.Command（用于 devrix 自启场景）。
type ExecLauncher struct{}

// Launch 启动 argv[0] 子进程并返回其 stdin/stdout。
func (ExecLauncher) Launch(ctx context.Context, argv []string, env []string) (Process, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("lsp: empty argv")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if len(env) > 0 {
		cmd.Env = append([]string{}, env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("lsp: start: %w", err)
	}
	return &execProcess{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

type execProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (e *execProcess) Stdin() io.Writer  { return e.stdin }
func (e *execProcess) Stdout() io.Reader { return e.stdout }
func (e *execProcess) Wait() error       { return e.cmd.Wait() }
func (e *execProcess) Kill() error       { return e.cmd.Process.Kill() }

// rpcClient 手写 JSON-RPC 客户端。
type rpcClient struct {
	name    string
	process Process
	mu      sync.Mutex // 保护 nextID
	nextID  atomic.Int64
	pending map[int64]chan *json.RawMessage
	pendMu  sync.Mutex
	closed  bool

	reader *bufio.Reader
}

func newRPCClient(name string, proc Process, r *bufio.Reader) *rpcClient {
	c := &rpcClient{
		name:    name,
		process: proc,
		reader:  r,
		pending: make(map[int64]chan *json.RawMessage),
	}
	go c.readLoop()
	return c
}

// readLoop 持续从 stdout 读消息并分发给 pending chan。
func (c *rpcClient) readLoop() {
	for {
		msg, err := readMessage(c.reader)
		if err != nil {
			c.drainPending(err)
			return
		}
		// 判断是 response 还是 notification
		var env struct {
			ID     *int64           `json:"id,omitempty"`
			Method *string          `json:"method,omitempty"`
			Result *json.RawMessage `json:"result,omitempty"`
			Error  *json.RawMessage `json:"error,omitempty"`
		}
		if err := json.Unmarshal(msg, &env); err != nil {
			continue
		}
		if env.ID != nil {
			c.pendMu.Lock()
			ch, ok := c.pending[*env.ID]
			if ok {
				delete(c.pending, *env.ID)
			}
			c.pendMu.Unlock()
			if ok {
				// 包含 result 或 error
				resp := struct {
					Result *json.RawMessage `json:"result,omitempty"`
					Error  *json.RawMessage `json:"error,omitempty"`
				}{Result: env.Result, Error: env.Error}
				ch <- mustMarshal(resp)
			}
		}
		// notification 直接丢弃
	}
}

func (c *rpcClient) drainPending(err error) {
	c.pendMu.Lock()
	defer c.pendMu.Unlock()
	for id, ch := range c.pending {
		_ = id
		ch <- mustMarshal(map[string]any{"error": err.Error()})
		delete(c.pending, id)
	}
}

// request 发送一个 JSON-RPC 请求并等待响应（带超时）。
func (c *rpcClient) request(method string, params any, timeout time.Duration) (*json.RawMessage, error) {
	if c.closed {
		return nil, fmt.Errorf("lsp: client closed")
	}
	id := c.nextID.Add(1)
	ch := make(chan *json.RawMessage, 1)
	c.pendMu.Lock()
	c.pending[id] = ch
	c.pendMu.Unlock()

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	if err := writeMessage(c.process.Stdin(), payload); err != nil {
		c.pendMu.Lock()
		delete(c.pending, id)
		c.pendMu.Unlock()
		return nil, fmt.Errorf("lsp: write: %w", err)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("lsp: response channel closed")
		}
		var out struct {
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(*resp, &out); err != nil {
			return nil, fmt.Errorf("lsp: decode: %w", err)
		}
		if out.Error != nil {
			return nil, fmt.Errorf("lsp: server error %d: %s", out.Error.Code, out.Error.Message)
		}
		return &out.Result, nil
	case <-timer.C:
		c.pendMu.Lock()
		delete(c.pending, id)
		c.pendMu.Unlock()
		return nil, fmt.Errorf("lsp: %s timeout after %s", method, timeout)
	}
}

// notify 发送一个 notification（无响应）。
func (c *rpcClient) notify(method string, params any) error {
	if c.closed {
		return fmt.Errorf("lsp: client closed")
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	return writeMessage(c.process.Stdin(), payload)
}

func (c *rpcClient) close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return c.process.Kill()
}

// readMessage 读一个 Content-Length / Content-Type 头 + body。
func readMessage(r *bufio.Reader) ([]byte, error) {
	var contentLength int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colon])
		value := strings.TrimSpace(line[colon+1:])
		if strings.EqualFold(name, "Content-Length") {
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("lsp: invalid Content-Length %q: %w", value, err)
			}
			contentLength = n
		}
	}
	if contentLength <= 0 {
		return nil, fmt.Errorf("lsp: missing or zero Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// writeMessage 写一个 Content-Length 头 + body。
func writeMessage(w io.Writer, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	header := "Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n"
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

func mustMarshal(v any) *json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		raw := json.RawMessage(`null`)
		return &raw
	}
	r := json.RawMessage(b)
	return &r
}

// ===== High-level client =====

type lspClient struct {
	name     string
	rootURI  string
	language string
	server   ServerConfig
	rpc      *rpcClient
	proc     Process
	launcher SandboxLauncher
	timeout  time.Duration
}

// ServerConfig 单个 LSP server 启动配置。
type ServerConfig struct {
	LanguageID  string
	Command     []string
	FilePattern []string
}

// NewClient 创建并启动一个 LSP server 客户端。
func NewClient(ctx context.Context, server ServerConfig, launcher SandboxLauncher, rootURI string, initTimeout time.Duration) (Client, error) {
	if launcher == nil {
		launcher = ExecLauncher{}
	}
	proc, err := launcher.Launch(ctx, server.Command, nil)
	if err != nil {
		return nil, fmt.Errorf("lsp: launch %s: %w", server.LanguageID, err)
	}
	reader := bufio.NewReader(proc.Stdout())
	rpc := newRPCClient(server.LanguageID, proc, reader)

	timeout := initTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	c := &lspClient{
		name:     server.LanguageID,
		rootURI:  rootURI,
		language: server.LanguageID,
		server:   server,
		rpc:      rpc,
		proc:     proc,
		launcher: launcher,
		timeout:  timeout,
	}
	if err := c.Initialize(ctx, rootURI); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

func (c *lspClient) Initialize(ctx context.Context, rootURI string) error {
	c.rootURI = rootURI
	params := map[string]any{
		"processId": nil,
		"rootUri":   rootURI,
		"capabilities": map[string]any{
			"workspace":   map[string]any{},
			"textDocument": map[string]any{
				"synchronization": map[string]any{
					"didSave": true,
				},
			},
		},
	}
	resp, err := c.rpc.request("initialize", params, c.timeout)
	if err != nil {
		return err
	}
	_ = resp
	// 发送 initialized 通知
	return c.rpc.notify("initialized", map[string]any{})
}

func (c *lspClient) DidOpen(ctx context.Context, uri, languageID, text string) error {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": languageID,
			"version":    1,
			"text":       text,
		},
	}
	return c.rpc.notify("textDocument/didOpen", params)
}

func (c *lspClient) Definition(ctx context.Context, uri string, p Position) ([]Location, error) {
	resp, err := c.rpc.request("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     p,
	}, c.timeout)
	if err != nil {
		return nil, err
	}
	var out []Location
	if err := json.Unmarshal(*resp, &out); err != nil {
		// 可能是单个 location 不是数组
		var single Location
		if err := json.Unmarshal(*resp, &single); err != nil {
			return nil, fmt.Errorf("lsp: decode definition: %w", err)
		}
		out = []Location{single}
	}
	return out, nil
}

func (c *lspClient) References(ctx context.Context, uri string, p Position, includeDecl bool) ([]Location, error) {
	resp, err := c.rpc.request("textDocument/references", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     p,
		"context":      map[string]any{"includeDeclaration": includeDecl},
	}, c.timeout)
	if err != nil {
		return nil, err
	}
	var out []Location
	if err := json.Unmarshal(*resp, &out); err != nil {
		return nil, fmt.Errorf("lsp: decode references: %w", err)
	}
	return out, nil
}

func (c *lspClient) PrepareCallHierarchy(ctx context.Context, uri string, p Position) ([]CallHierarchyItem, error) {
	resp, err := c.rpc.request("textDocument/prepareCallHierarchy", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     p,
	}, c.timeout)
	if err != nil {
		return nil, err
	}
	var out []CallHierarchyItem
	if err := json.Unmarshal(*resp, &out); err != nil {
		return nil, fmt.Errorf("lsp: decode call hierarchy: %w", err)
	}
	return out, nil
}

func (c *lspClient) IncomingCalls(ctx context.Context, item CallHierarchyItem) ([]CallHierarchyIncomingCall, error) {
	resp, err := c.rpc.request("callHierarchy/incomingCalls", map[string]any{
		"item": item,
	}, c.timeout)
	if err != nil {
		return nil, err
	}
	var out []CallHierarchyIncomingCall
	if err := json.Unmarshal(*resp, &out); err != nil {
		return nil, fmt.Errorf("lsp: decode incoming calls: %w", err)
	}
	return out, nil
}

func (c *lspClient) Close() error {
	return c.rpc.close()
}
