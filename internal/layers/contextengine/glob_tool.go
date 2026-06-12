package contextengine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

type globRunner struct{}

func newGlobRunner() *globRunner {
	return &globRunner{}
}

func (r *globRunner) Name() string { return "glob" }

func (r *globRunner) Schema() ToolSchema {
	return ToolSchema{
		Name:        "glob",
		Description: "Fast file pattern matching tool. Supports glob patterns like \"**/*.js\" or \"src/**/*.ts\". Returns matching file paths sorted by modification time.",
		Parameters:  `{"type":"object","required":["pattern"],"properties":{"pattern":{"type":"string"},"path":{"type":"string"}}}`,
	}
}

func (r *globRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *globRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
	fields := parseToolInput(input)
	pattern := fields["pattern"]
	if pattern == "" {
		return &ToolResult{Error: "glob: pattern is required"}, nil
	}
	rawPath := fields["path"]
	if rawPath == "" {
		rawPath = workDir
	}

	searchDir, err := resolveWorkspacePath(workDir, rawPath)
	if err != nil {
		return &ToolResult{Error: fmt.Sprintf("glob: %s", err)}, nil
	}

	start := time.Now()
	const maxResults = 100

	type match struct {
		path    string
		modTime time.Time
	}
	var matches []match

	walkErr := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible
		}
		if len(matches) >= maxResults {
			return filepath.SkipAll
		}

		rel, err := filepath.Rel(searchDir, path)
		if err != nil {
			return nil
		}
		if rel == "." {
			return nil
		}

		matched, err := filepath.Match(pattern, rel)
		if err != nil {
			matched = strings.Contains(rel, pattern)
		}
		if matched {
			matches = append(matches, match{path: path, modTime: info.ModTime()})
		}
		return nil
	})
	if walkErr != nil && len(matches) == 0 {
		return &ToolResult{Error: fmt.Sprintf("glob: %s", walkErr)}, nil
	}

	sort.Slice(matches, func(i, j int) bool {
		if !matches[i].modTime.Equal(matches[j].modTime) {
			return matches[i].modTime.After(matches[j].modTime)
		}
		return matches[i].path < matches[j].path
	})

	durationMs := time.Since(start).Milliseconds()
	filenames := make([]string, len(matches))
	for i, m := range matches {
		rel, _ := filepath.Rel(workDir, m.path)
		filenames[i] = rel
	}
	truncated := len(filenames) >= maxResults

	out, _ := json.Marshal(map[string]any{
		"filenames":  filenames,
		"durationMs": durationMs,
		"numFiles":   len(filenames),
		"truncated":  truncated,
	})
	return &ToolResult{Output: string(out)}, nil
}
