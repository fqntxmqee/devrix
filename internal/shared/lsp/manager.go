package lsp

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultMaxServers — 最多同时保活的 LSP server 数。
const DefaultMaxServers = 4

// Manager 维护"按 languageID+workspace 路由"的 LSP server 池，LRU 淘汰。
type Manager struct {
	cap      int
	launcher SandboxLauncher
	initTimeout time.Duration
	requestTimeout time.Duration

	mu      sync.Mutex
	clients *list.List
	byKey   map[string]*list.Element
}

// NewManager 构造 manager，cap 传 0 用 DefaultMaxServers。
func NewManager(launcher SandboxLauncher, cap int, initTimeout, requestTimeout time.Duration) *Manager {
	if cap <= 0 {
		cap = DefaultMaxServers
	}
	if initTimeout <= 0 {
		initTimeout = 30 * time.Second
	}
	if requestTimeout <= 0 {
		requestTimeout = 10 * time.Second
	}
	return &Manager{
		cap:      cap,
		launcher: launcher,
		initTimeout: initTimeout,
		requestTimeout: requestTimeout,
		clients:  list.New(),
		byKey:    make(map[string]*list.Element),
	}
}

// Acquire 拿一个 client，按 languageID 路由。
// servers 列表按声明顺序查 file_pattern，找到第一个匹配的 server 启动。
func (m *Manager) Acquire(ctx context.Context, file string, servers []ServerConfig) (Client, error) {
	if m.launcher == nil {
		return nil, fmt.Errorf("lsp: launcher is nil")
	}
	cfg, err := pickServer(file, servers)
	if err != nil {
		return nil, err
	}
	rootURI := fileRootURI(file)
	key := cfg.LanguageID + "|" + rootURI

	m.mu.Lock()
	if elem, ok := m.byKey[key]; ok {
		m.clients.MoveToFront(elem)
		c := elem.Value.(*managedClient).client
		m.mu.Unlock()
		return c, nil
	}
	m.mu.Unlock()

	// 启动新 client（释放锁 → I/O）
	c, err := NewClient(ctx, cfg, m.launcher, rootURI, m.initTimeout)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// 双重检查
	if elem, ok := m.byKey[key]; ok {
		m.clients.MoveToFront(elem)
		existing := elem.Value.(*managedClient).client
		_ = c.Close() // 关闭新启的，保留旧的
		return existing, nil
	}
	elem := m.clients.PushFront(&managedClient{client: c, key: key})
	m.byKey[key] = elem
	for m.clients.Len() > m.cap {
		oldest := m.clients.Back()
		if oldest == nil {
			break
		}
		mc := oldest.Value.(*managedClient)
		_ = mc.client.Close()
		m.clients.Remove(oldest)
		delete(m.byKey, mc.key)
	}
	return c, nil
}

// Shutdown 关闭所有 client。
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var firstErr error
	for e := m.clients.Front(); e != nil; e = e.Next() {
		if err := e.Value.(*managedClient).client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	m.clients.Init()
	m.byKey = make(map[string]*list.Element)
	return firstErr
}

// Len 返回当前活跃 client 数。
func (m *Manager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clients.Len()
}

type managedClient struct {
	client Client
	key    string
}

// pickServer 按 file 扩展名匹配 server 配置。
func pickServer(file string, servers []ServerConfig) (ServerConfig, error) {
	if len(servers) == 0 {
		return ServerConfig{}, errors.New("lsp: no servers configured")
	}
	base := filepath.Base(file)
	for _, s := range servers {
		for _, pat := range s.FilePattern {
			matched, err := filepath.Match(pat, base)
			if err != nil {
				continue
			}
			if matched {
				return s, nil
			}
		}
	}
	return ServerConfig{}, fmt.Errorf("lsp: no server matches file %s", file)
}

// fileRootURI 简单地把 file 所在目录转成 file:// URI。
func fileRootURI(file string) string {
	dir := filepath.Dir(file)
	dir = filepath.Clean(dir)
	abs := dir
	if !filepath.IsAbs(abs) {
		if a, err := filepath.Abs(dir); err == nil {
			abs = a
		}
	}
	// 转 file:// URL（仅 POSIX 路径，Windows 由 clawcode 视情况扩展）
	return "file://" + filepath.ToSlash(abs)
}

// EnsureLauncher 兜底 launcher；不传则用 ExecLauncher。
func EnsureLauncher(l SandboxLauncher) SandboxLauncher {
	if l != nil {
		return l
	}
	return ExecLauncher{}
}

// LanguageForFile 根据扩展名猜测 languageID（用于 DidOpen）。
func LanguageForFile(file string) string {
	ext := strings.ToLower(filepath.Ext(file))
	switch ext {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	}
	return ""
}
