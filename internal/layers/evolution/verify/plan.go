// Package verify — G4 实现后自动验证,对标 clawcode src/tools/VerifyPlanExecutionTool/。
//
// 工作流:
//  1. 解析 `openspec/changes/<change-id>/tasks.md` 中 `| W{N}.{M} | desc | file | done|pending |` 表格
//  2. 对每条 Done=true 的 plan item,验证 Evidence(File/Test/Command)
//  3. 输出 Report,CLI 入口 devrix verify-plan <change-id>
//
// 设计参考:openspec/changes/devrix-diagnostic-tools-parity/design.md §2.6
package verify

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// EvidenceKind — 单条 evidence 类型。
type EvidenceKind string

const (
	EvidenceFile    EvidenceKind = "file"
	EvidenceTest    EvidenceKind = "test"
	EvidenceCommand EvidenceKind = "command"
)

// Evidence — 单条 plan item 的验证依据。
//
// Kind=file 验证 Path 存在;Match 是文件内容应包含的关键 token(可选)。
// Kind=test 验证 Path 存在且包含 `func TestXxx(` 形式。
// Kind=command 验证 Path 存在(Match 为命令名)。
type Evidence struct {
	Kind  EvidenceKind `json:"kind"`
	Path  string       `json:"path"`
	Match string       `json:"match,omitempty"`
}

// PlanItem — tasks.md 中单条 W{N}.{M} 行。
type PlanItem struct {
	ID       string     `json:"id"`
	Title    string     `json:"title"`
	File     string     `json:"file,omitempty"`
	Done     bool       `json:"done"`
	Evidence []Evidence `json:"evidence,omitempty"`
}

// UnverifiedItem — 验证失败的 plan item + 原因。
type UnverifiedItem struct {
	Item   PlanItem `json:"item"`
	Reason string   `json:"reason"`
}

// Report — Verify 全量报告。
type Report struct {
	ChangeID    string            `json:"change_id"`
	Total       int               `json:"total"`
	Verified    int               `json:"verified"`
	Unverified  []UnverifiedItem  `json:"unverified,omitempty"`
	Skipped     int               `json:"skipped"` // Done=false 的 item 数
	Summary     string            `json:"summary"`
}

// Verifier — plan 验证器接口。
type Verifier interface {
	LoadPlan(taskFile string) ([]PlanItem, error)
	Verify(ctx context.Context, items []PlanItem, repoRoot string) (Report, error)
}

// FileVerifier — 默认实现:file system + 简单 content check。
type FileVerifier struct{}

// NewFileVerifier 构造默认验证器。
func NewFileVerifier() *FileVerifier { return &FileVerifier{} }

// LoadPlan 解析 tasks.md。支持的行格式:
//
//	| W1.1 | Description | path/to/file.go | done |
//	| W2.1 | Description | path/to/file.go | pending |
//
// File 列可空(任务不直接对应单文件)。Status 列大小写不敏感。
func (v *FileVerifier) LoadPlan(taskFile string) ([]PlanItem, error) {
	f, err := os.Open(taskFile)
	if err != nil {
		return nil, fmt.Errorf("verify: open tasks.md: %w", err)
	}
	defer f.Close()

	var items []PlanItem
	rowRe := regexp.MustCompile(`^\|\s*(W\d+\.\d+)\s*\|\s*([^|]*?)\s*\|\s*([^|]*?)\s*\|\s*(done|pending|)\s*\|`)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if strings.HasPrefix(line, "| ID") || strings.HasPrefix(line, "| Task") || strings.HasPrefix(line, "|------") {
			continue
		}
		m := rowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id, title, path, status := m[1], strings.TrimSpace(m[2]), strings.TrimSpace(m[3]), strings.ToLower(m[4])
		item := PlanItem{
			ID:    id,
			Title: title,
			File:  path,
			Done:  status == "done",
		}
		if path != "" {
			item.Evidence = []Evidence{{Kind: EvidenceFile, Path: path}}
			// _test.go 后缀 → 自动加 test evidence
			if strings.HasSuffix(path, "_test.go") {
				item.Evidence = append(item.Evidence, Evidence{Kind: EvidenceTest, Path: path})
			}
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("verify: scan tasks.md: %w", err)
	}
	return items, nil
}

// Verify 逐条 plan item 验证 evidence。
func (v *FileVerifier) Verify(ctx context.Context, items []PlanItem, repoRoot string) (Report, error) {
	if repoRoot == "" {
		repoRoot = "."
	}
	report := Report{Total: len(items)}
	for _, item := range items {
		if !item.Done {
			report.Skipped++
			continue
		}
		ok, reason := v.checkItem(ctx, item, repoRoot)
		if ok {
			report.Verified++
			continue
		}
		report.Unverified = append(report.Unverified, UnverifiedItem{Item: item, Reason: reason})
	}
	report.Summary = formatReport(&report)
	return report, nil
}

func (v *FileVerifier) checkItem(ctx context.Context, item PlanItem, repoRoot string) (bool, string) {
	if len(item.Evidence) == 0 {
		// Done=true 但无 file evidence:仍标 verified(如写 spec 的任务)
		return true, ""
	}
	for _, ev := range item.Evidence {
		if err := ctx.Err(); err != nil {
			return false, err.Error()
		}
		if err := v.checkEvidence(ev, repoRoot); err != nil {
			return false, fmt.Sprintf("%s: %s", ev.Kind, err.Error())
		}
	}
	return true, ""
}

func (v *FileVerifier) checkEvidence(ev Evidence, repoRoot string) error {
	absPath := ev.Path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(repoRoot, ev.Path)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("file not found: %s", ev.Path)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory: %s", ev.Path)
	}
	switch ev.Kind {
	case EvidenceFile:
		if ev.Match != "" {
			data, err := os.ReadFile(absPath)
			if err != nil {
				return err
			}
			if !strings.Contains(string(data), ev.Match) {
				return fmt.Errorf("file %s missing required token %q", ev.Path, ev.Match)
			}
		}
	case EvidenceTest:
		data, err := os.ReadFile(absPath)
		if err != nil {
			return err
		}
		if !testFuncRe.Match(data) {
			return fmt.Errorf("test file %s contains no func TestXxx(", ev.Path)
		}
	}
	return nil
}

var testFuncRe = regexp.MustCompile(`(?m)^func\s+Test\w+\s*\(`)

// FormatJSON 渲染 report 为 JSON。
func FormatJSON(r Report) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func formatReport(r *Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "verify-plan: %d total, %d verified, %d skipped, %d unverified",
		r.Total, r.Verified, r.Skipped, len(r.Unverified))
	if len(r.Unverified) > 0 {
		b.WriteString("\n")
		for _, u := range r.Unverified {
			fmt.Fprintf(&b, "  ✗ %s — %s: %s\n", u.Item.ID, u.Item.File, u.Reason)
		}
	}
	return b.String()
}
