package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/devrix/devrix/internal/shared/types"
)

type editFileRunner struct {
	cfg *toolExecConfig
}

func newEditFileRunner(cfg *toolExecConfig) *editFileRunner {
	return &editFileRunner{cfg: cfg}
}

func (r *editFileRunner) Name() string { return "edit_file" }

func (r *editFileRunner) Schema() ToolSchema {
	return ToolSchema{
		Name:        "edit_file",
		Description: "Edit a file by replacing exact text. Use old_string to match the text to replace, and new_string for the replacement text. Prefer this over write_file for targeted edits.",
		Parameters:  `{"type":"object","required":["file_path","old_string","new_string"],"properties":{"file_path":{"type":"string"},"old_string":{"type":"string"},"new_string":{"type":"string"},"replace_all":{"type":"boolean","description":"Replace all occurrences instead of just the first"}}}`,
	}
}

func (r *editFileRunner) RiskLevel() types.RiskLevel { return types.RiskLevelMedium }

func (r *editFileRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
	_ = ctx
	fields := ParseToolInput(input)
	filePath := firstNonEmpty(fields, "file_path", "path", "file")
	oldString := fields["old_string"]
	newString := fields["new_string"]
	replaceAll := fields["replace_all"] == "true" || fields["replace_all"] == "1"

	if filePath == "" {
		return &ToolResult{Error: "edit_file: file_path is required"}, nil
	}
	if oldString == "" {
		return &ToolResult{Error: "edit_file: old_string is required"}, nil
	}

	// Resolve path
	target, err := resolveWorkspacePath(workDir, filePath)
	if err != nil {
		return &ToolResult{Error: fmt.Sprintf("edit_file: %s", err)}, nil
	}

	// RH-D2-01 (DM-20260630-013): edit_file must enforce the same plan-mode
	// write gate as write_file. Without this parity, plan mode could rewrite
	// non-plan files via targeted edits.
	if denied := EnforcePlanModeWrite(ctx, target); denied != nil {
		return denied, nil
	}

	// Read file
	data, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{Error: fmt.Sprintf("edit_file: file does not exist: %s", filePath)}, nil
		}
		return &ToolResult{Error: fmt.Sprintf("edit_file: %s", err)}, nil
	}
	content := string(data)

	// old_string == new_string check
	if oldString == newString {
		return &ToolResult{Error: "edit_file: old_string and new_string are identical — no change to make"}, nil
	}

	// Find actual match with quote normalization
	actualOld := findActualString(content, oldString)
	if actualOld == "" {
		return &ToolResult{Error: fmt.Sprintf("edit_file: could not find exact match for old_string in %s. The text may not exist or may differ in whitespace/quoting.", filePath)}, nil
	}

	// Check uniqueness
	if !replaceAll {
		count := strings.Count(content, actualOld)
		if count > 1 {
			return &ToolResult{Error: fmt.Sprintf("edit_file: found %d matches. Use replace_all=true to replace all, or provide more context to make old_string unique.", count)}, nil
		}
	}

	// Apply replacement
	var newContent string
	if replaceAll {
		newContent = strings.ReplaceAll(content, actualOld, newString)
	} else {
		newContent = strings.Replace(content, actualOld, newString, 1)
	}

	if newContent == content {
		return &ToolResult{Error: "edit_file: replacement produced no changes"}, nil
	}

	// Write file
	fileMode := os.FileMode(0o644)
	if info, err := os.Stat(target); err == nil {
		fileMode = info.Mode()
	}
	if err := os.WriteFile(target, []byte(newContent), fileMode); err != nil {
		return &ToolResult{Error: fmt.Sprintf("edit_file: write failed: %s", err)}, nil
	}

	// Count changed lines
	oldLines := strings.Split(content, "\n")
	newLines := strings.Split(newContent, "\n")
	diffLines := 0
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}
	for i := 0; i < maxLen; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine != newLine {
			diffLines++
		}
	}

	out, _ := json.Marshal(map[string]any{
		"filePath":      filePath,
		"oldString":     actualOld,
		"newString":     newString,
		"didReplaceAll": replaceAll,
		"diffLines":     diffLines,
	})
	return &ToolResult{Output: string(out)}, nil
}

// findActualString tries exact match first, then normalizes quotes and retries.
func findActualString(content, search string) string {
	if strings.Contains(content, search) {
		return search
	}
	normalized := normalizeQuotes(search)
	if normalized == search {
		return ""
	}
	// Search for normalized version in content
	idx := strings.Index(content, normalized)
	if idx >= 0 {
		return content[idx : idx+len(normalized)]
	}
	return ""
}

// normalizeQuotes converts curly/smart quotes to straight quotes.
func normalizeQuotes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '‘', '’': // ’‘
			b.WriteByte('\'')
		case '“', '”': // ""
			b.WriteByte('"')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
