// Package tracker — D5-S23 文件诊断追踪器，对标 clawcode diagnosticTracking.ts + LSPDiagnosticRegistry。
//
// 核心能力：
//   - SnapshotBefore：编辑前缓存 linter 快照
//   - Diff：编辑后对比基线，输出新增 diagnostic（不报"消失"的）
//   - LRU 上限 500 文件（与 clawcode 一致）
//   - 异步 OnEditComplete 钩子：linter 不阻塞主路径，结果附加给下一回合 system reminder
package tracker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"container/list"
)

const (
	// DefaultCapacity — 500 文件 LRU 上限（与 clawcode DiagnosticTrackingService 一致）。
	DefaultCapacity = 500

	// LinterTimeout — 单次 linter 调用的最大执行时间。
	LinterTimeout = 10 * time.Second
)

// Diagnostic 单条诊断信息。
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column,omitempty"`
	Severity string `json:"severity"` // "error" | "warning" | "info"
	Message  string `json:"message"`
	Source   string `json:"source"` // "go-vet" | "tsc" | "shellcheck" | ...
	Code     string `json:"code,omitempty"`
}

// LinterFunc 对单个文件运行 linter 并返回 diagnostic 列表。
type LinterFunc func(ctx context.Context, file string) ([]Diagnostic, error)

// Tracker 维护"编辑前 → 编辑后"linter 状态对比。
type Tracker struct {
	mu      sync.Mutex
	cap     int
	lru     *list.List
	by      map[string]*list.Element

	// 注入的 linter 函数表（按文件扩展名路由）
	linters map[string]LinterFunc

	// 缺省 linter（无匹配时使用）
	defaultLinter LinterFunc
}

// New 构造 tracker，cap 传 0 用 DefaultCapacity。
func New(cap int) *Tracker {
	if cap <= 0 {
		cap = DefaultCapacity
	}
	t := &Tracker{
		cap:     cap,
		lru:     list.New(),
		by:      make(map[string]*list.Element),
		linters: make(map[string]LinterFunc),
	}
	// 内置默认 linter
	t.linters[".go"] = goVetLinter
	t.linters[".ts"] = tscLinter
	t.linters[".tsx"] = tscLinter
	t.linters[".js"] = tscLinter
	t.linters[".jsx"] = tscLinter
	t.linters[".sh"] = shellcheckLinter
	t.linters[".bash"] = shellcheckLinter
	return t
}

// SetLinter 注入自定义 linter；扩展名以 "." 开头（如 ".go"）。
func (t *Tracker) SetLinter(ext string, fn LinterFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.linters == nil {
		t.linters = make(map[string]LinterFunc)
	}
	t.linters[ext] = fn
}

// SnapshotBefore 在编辑前缓存文件当前的 diagnostic 列表（不阻塞失败）。
func (t *Tracker) SnapshotBefore(ctx context.Context, file string) error {
	linter := t.linterFor(file)
	if linter == nil {
		return nil
	}
	diags, err := safeLint(ctx, linter, file)
	if err != nil {
		// 静默失败，遵循 clawcode "Fail silently if IDE doesn't support diagnostics" 策略
		return nil
	}
	t.put(file, diags)
	return nil
}

// Diff 在编辑后调用，返回基线中不存在的 diagnostic（新增）。
func (t *Tracker) Diff(ctx context.Context, file string) ([]Diagnostic, error) {
	linter := t.linterFor(file)
	if linter == nil {
		return nil, nil
	}
	current, err := safeLint(ctx, linter, file)
	if err != nil {
		// 编辑后 linter 失败：返回基线（不报"消失"），符合 R2 风险缓解策略
		return nil, nil
	}
	base := t.get(file)
	t.put(file, current)

	added := make([]Diagnostic, 0)
	for _, d := range current {
		if !containsDiag(base, d) {
			added = append(added, d)
		}
	}
	return added, nil
}

// Flush 清空所有快照（重启 / 新会话）。
func (t *Tracker) Flush() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lru.Init()
	t.by = make(map[string]*list.Element)
}

// Len 返回当前快照数量。
func (t *Tracker) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lru.Len()
}

// 内部：linter 路由
func (t *Tracker) linterFor(file string) LinterFunc {
	ext := strings.ToLower(filepath.Ext(file))
	if linter, ok := t.linters[ext]; ok {
		return linter
	}
	return nil
}

func (t *Tracker) put(file string, diags []Diagnostic) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if elem, ok := t.by[file]; ok {
		elem.Value = snapshot{file: file, diags: diags}
		t.lru.MoveToFront(elem)
		return
	}
	elem := t.lru.PushFront(snapshot{file: file, diags: diags})
	t.by[file] = elem
	for t.lru.Len() > t.cap {
		// LRU 淘汰
		oldest := t.lru.Back()
		if oldest == nil {
			break
		}
		t.lru.Remove(oldest)
		delete(t.by, oldest.Value.(snapshot).file)
	}
}

func (t *Tracker) get(file string) []Diagnostic {
	t.mu.Lock()
	defer t.mu.Unlock()
	if elem, ok := t.by[file]; ok {
		t.lru.MoveToFront(elem)
		return elem.Value.(snapshot).diags
	}
	return nil
}

type snapshot struct {
	file  string
	diags []Diagnostic
}

func safeLint(ctx context.Context, fn LinterFunc, file string) ([]Diagnostic, error) {
	c, cancel := context.WithTimeout(ctx, LinterTimeout)
	defer cancel()
	return fn(c, file)
}

func containsDiag(list []Diagnostic, d Diagnostic) bool {
	for _, x := range list {
		if diagEqual(x, d) {
			return true
		}
	}
	return false
}

func diagEqual(a, b Diagnostic) bool {
	return a.File == b.File &&
		a.Line == b.Line &&
		a.Column == b.Column &&
		a.Severity == b.Severity &&
		a.Message == b.Message &&
		a.Source == b.Source &&
		a.Code == b.Code
}

// === 内置 linter：go vet ===

func goVetLinter(ctx context.Context, file string) ([]Diagnostic, error) {
	if _, err := exec.LookPath("go"); err != nil {
		return nil, nil // linter 不可用 → 静默返回空
	}
	// 单文件 vet 必须配合包，调用 go vet ./<dir> 不可靠；改用 go vet -json ./... 配合 grep
	// 简单策略：运行 `go vet -json <package>` 解析 <file> 相关行
	dir := filepath.Dir(file)
	pkg, err := goPackageForFile(dir)
	if err != nil {
		return nil, nil
	}
	cmd := exec.CommandContext(ctx, "go", "vet", "-json", pkg)
	cmd.Dir = filepath.Dir(file)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run() // 失败也继续，go vet 返回非零当有错

	relFile, _ := filepath.Rel(".", file)
	if relFile == "" {
		relFile = file
	}

	out := []Diagnostic{}
	scanner := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		// 尝试 JSON 解析
		var ev struct {
			Action  string
			Package string
			Posn    struct {
				Filename string
				Line     int
				Column   int
			} `json:"Posn"`
			Message string
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Posn.Filename == "" {
			continue
		}
		// 仅收集目标文件
		if !pathMatch(ev.Posn.Filename, file, relFile) {
			continue
		}
		if ev.Action != "" && ev.Action != "warning" && ev.Action != "error" {
			continue
		}
		severity := "warning"
		if ev.Action == "error" {
			severity = "error"
		}
		out = append(out, Diagnostic{
			File:     file,
			Line:     ev.Posn.Line,
			Column:   ev.Posn.Column,
			Severity: severity,
			Message:  ev.Message,
			Source:   "go-vet",
		})
	}
	return out, nil
}

func goPackageForFile(dir string) (string, error) {
	// 简化：用 ./ 相对路径
	_ = dir
	return "./...", nil
}

func pathMatch(candidate, file, rel string) bool {
	candBase := filepath.Base(candidate)
	if candBase == filepath.Base(file) {
		return true
	}
	if strings.HasSuffix(candidate, rel) {
		return true
	}
	return false
}

// === 内置 linter：tsc --noEmit ===

var tscPathRE = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\):\s+(error|warning|info)\s+(TS\d+):\s*(.+)$`)

func tscLinter(ctx context.Context, file string) ([]Diagnostic, error) {
	if _, err := exec.LookPath("tsc"); err != nil {
		return nil, nil
	}
	dir := filepath.Dir(file)
	cmd := exec.CommandContext(ctx, "tsc", "--noEmit", "--pretty", "false")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()

	out := []Diagnostic{}
	for _, line := range append(strings.Split(stdout.String(), "\n"), strings.Split(stderr.String(), "\n")...) {
		line = strings.TrimSpace(line)
		matches := tscPathRE.FindStringSubmatch(line)
		if len(matches) != 6 {
			continue
		}
		filePath := matches[1]
		if !strings.HasSuffix(filePath, filepath.Base(file)) {
			continue
		}
		// 解析行号
		var lineNum, col int
		_, _ = fmt.Sscanf(matches[2], "%d", &lineNum)
		_, _ = fmt.Sscanf(matches[3], "%d", &col)
		out = append(out, Diagnostic{
			File:     file,
			Line:     lineNum,
			Column:   col,
			Severity: matches[4],
			Message:  matches[6],
			Source:   "tsc",
			Code:     matches[5],
		})
	}
	return out, nil
}

// === 内置 linter：shellcheck ===

type shellcheckEntry struct {
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Source  string `json:"source"`
	Code    string `json:"code"`
}

func shellcheckLinter(ctx context.Context, file string) ([]Diagnostic, error) {
	if _, err := exec.LookPath("shellcheck"); err != nil {
		return nil, nil
	}
	cmd := exec.CommandContext(ctx, "shellcheck", "-f", "json", file)
	cmd.Dir = filepath.Dir(file)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()

	if stdout.Len() == 0 {
		return nil, nil
	}
	var entries []shellcheckEntry
	if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
		return nil, nil
	}
	out := make([]Diagnostic, 0, len(entries))
	for _, e := range entries {
		sev := e.Level
		if sev == "info" {
			sev = "info"
		}
		out = append(out, Diagnostic{
			File:     file,
			Line:     e.Line,
			Column:   e.Column,
			Severity: sev,
			Message:  e.Message,
			Source:   "shellcheck",
			Code:     e.Code,
		})
	}
	return out, nil
}

// === 辅助：AppendToReminder 把 diagnostic 列表格式化为 LLM 可消费的 system reminder block。 ===
func AppendToReminder(existing string, diags []Diagnostic) string {
	if len(diags) == 0 {
		return existing
	}
	var b strings.Builder
	b.WriteString(existing)
	if existing != "" {
		b.WriteString("\n")
	}
	b.WriteString("<file_diagnostics>\n")
	for _, d := range diags {
		fmt.Fprintf(&b, "  %s [%s] %s:%d:%d — %s\n",
			severityIcon(d.Severity), d.Source, filepath.Base(d.File), d.Line, d.Column, d.Message)
	}
	b.WriteString("</file_diagnostics>\n")
	return b.String()
}

func severityIcon(sev string) string {
	switch sev {
	case "error":
		return "✖"
	case "warning":
		return "⚠"
	case "info":
		return "ℹ"
	}
	return "•"
}

// EnsureLintToolAvailable checks if the linter binary is on PATH. 用于 Doctor.
func EnsureLintToolAvailable(ext string) error {
	binaries := map[string]string{
		".go": "go",
		".ts": "tsc",
		".tsx": "tsc",
		".sh": "shellcheck",
	}
	bin, ok := binaries[ext]
	if !ok {
		return nil
	}
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("linter %q not found in PATH", bin)
	}
	return nil
}

// keep os import for future expansion (read file content if needed)
var _ = os.Stat
