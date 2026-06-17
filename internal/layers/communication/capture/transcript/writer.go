// Package transcript — A3 会话转录持久化,对标 clawcode --continue 行为。
//
// 每 session 一个 .jsonl 文件,每行一条 Event。
// Writer 串行追加(全局 mutex 序列化 Append 调用),保证单文件行序。
//
// 设计参考:openspec/changes/devrix-diagnostic-tools-parity/design.md §2.11
package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Event 单条转录事件。
type Event struct {
	Time time.Time `json:"t"`
	Kind string    `json:"kind"` // user|assistant|tool_call|tool_result|system
	Role string    `json:"role,omitempty"`
	Body string    `json:"body"`
}

// Writer JSONL 追加器,落盘到 <dir>/<sessionID>.jsonl。
type Writer struct {
	dir string
	mu  sync.Mutex
}

// NewWriter 构造 Writer。dir 不存在时自动创建。
func NewWriter(dir string) (*Writer, error) {
	if dir == "" {
		return nil, fmt.Errorf("transcript: dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("transcript: mkdir %s: %w", dir, err)
	}
	return &Writer{dir: dir}, nil
}

// Append 追加一条 event 到 <sessionID>.jsonl。
// time 为零值时填 time.Now();Kind 必须非空。
func (w *Writer) Append(sessionID string, ev Event) error {
	if w == nil {
		return fmt.Errorf("transcript: nil writer")
	}
	if sessionID == "" {
		return fmt.Errorf("transcript: sessionID is required")
	}
	if ev.Kind == "" {
		return fmt.Errorf("transcript: event kind is required")
	}
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	path := filepath.Join(w.dir, sanitize(sessionID)+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("transcript: open %s: %w", path, err)
	}
	defer f.Close()
	// 行分隔:NDJSON
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("transcript: marshal: %w", err)
	}
	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("transcript: write: %w", err)
	}
	return nil
}

// Dir 报告落盘目录(只读)。
func (w *Writer) Dir() string {
	if w == nil {
		return ""
	}
	return w.dir
}

// LoadReader 读出 <sessionID>.jsonl 全部事件(供 --continue 重建 context)。
func (w *Writer) LoadReader(sessionID string) ([]Event, error) {
	if w == nil {
		return nil, fmt.Errorf("transcript: nil writer")
	}
	path := filepath.Join(w.dir, sanitize(sessionID)+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			return nil, fmt.Errorf("transcript: bad line: %w", err)
		}
		out = append(out, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ListSessions 列出 dir 下所有 session 文件名(去扩展名)。
// 按 mtime 倒序(最近优先)。
func (w *Writer) ListSessions() ([]string, error) {
	if w == nil {
		return nil, fmt.Errorf("transcript: nil writer")
	}
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return nil, err
	}
	type item struct {
		name    string
		modTime time.Time
	}
	var items []item
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, item{name: strings.TrimSuffix(name, ".jsonl"), modTime: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].modTime.After(items[j].modTime)
	})
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.name
	}
	return out, nil
}

// sanitize 防止 path traversal:仅保留 [A-Za-z0-9._-]。
func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "session"
	}
	return string(out)
}
