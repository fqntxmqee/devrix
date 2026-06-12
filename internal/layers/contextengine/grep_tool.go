package contextengine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

type gFileMatch struct {
	path    string
	modTime time.Time
	count   int
	lines   []gMatchLine
}

type gMatchLine struct {
	lineNum int
	text    string
	context []string
}

type grepRunner struct{}

func newGrepRunner() *grepRunner {
	return &grepRunner{}
}

func (r *grepRunner) Name() string { return "grep" }

func (r *grepRunner) Schema() ToolSchema {
	return ToolSchema{
		Name:        "grep",
		Description: "A powerful search tool built on regex. Search file contents using regular expressions. Supports content, files_with_matches, and count output modes.",
		Parameters:  `{"type":"object","required":["pattern"],"properties":{"pattern":{"type":"string"},"path":{"type":"string"},"output_mode":{"type":"string","enum":["content","files_with_matches","count"]},"-i":{"type":"boolean","description":"Case insensitive search"},"head_limit":{"type":"integer","description":"Limit output to first N entries"},"offset":{"type":"integer","description":"Skip first N entries before applying head_limit"},"-C":{"type":"integer","description":"Context lines before and after each match"},"glob":{"type":"string","description":"Glob pattern to filter files (e.g. *.ts)"}}}`,
	}
}

func (r *grepRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *grepRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
	fields := parseToolInput(input)
	pattern := fields["pattern"]
	if pattern == "" {
		return &ToolResult{Error: "grep: pattern is required"}, nil
	}

	rawPath := fields["path"]
	if rawPath == "" {
		rawPath = workDir
	}
	searchDir, err := resolveWorkspacePath(workDir, rawPath)
	if err != nil {
		return &ToolResult{Error: fmt.Sprintf("grep: %s", err)}, nil
	}

	outputMode := fields["output_mode"]
	if outputMode == "" {
		outputMode = "files_with_matches"
	}
	caseInsensitive := fields["-i"] == "true" || fields["-i"] == "1"
	headLimit := parseInt(fields["head_limit"], 250)
	offset := parseInt(fields["offset"], 0)
	contextLines := parseInt(fields["-C"], 0)
	globFilter := fields["glob"]

	start := time.Now()

	// Compile regex
	reStr := pattern
	if caseInsensitive {
		reStr = "(?i)" + reStr
	}
	re, err := regexp.Compile(reStr)
	if err != nil {
		return &ToolResult{Error: fmt.Sprintf("grep: invalid regex %q: %s", pattern, err)}, nil
	}

	// VCS dirs to skip
	vcsDirs := map[string]bool{".git": true, ".svn": true, ".hg": true, ".bzr": true}

	var fileMatches []gFileMatch
	const maxFileSize = 1 << 20 // 1MB

	walkErr := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && vcsDirs[info.Name()] {
			return filepath.SkipDir
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		if info.Size() > maxFileSize {
			return nil
		}

		// Apply glob filter if specified
		if globFilter != "" {
			rel, _ := filepath.Rel(searchDir, path)
			m, err := filepath.Match(globFilter, rel)
			if err != nil || !m {
				if err != nil {
					m = strings.Contains(rel, globFilter)
				}
				if !m {
					return nil
				}
			}
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(data), "\n")

		if re.Match(data) {
			if outputMode == "count" {
				count := len(re.FindAll(data, -1))
				fileMatches = append(fileMatches, gFileMatch{path: path, modTime: info.ModTime(), count: count})
				return nil
			}

			// Find matching lines
			var matchingLines []gMatchLine
			for i, line := range lines {
				if re.MatchString(line) {
					var ctx []string
					if contextLines > 0 {
						ctxStart := i - contextLines
						if ctxStart < 0 {
							ctxStart = 0
						}
						ctxEnd := i + contextLines + 1
						if ctxEnd > len(lines) {
							ctxEnd = len(lines)
						}
						for _, cl := range lines[ctxStart:ctxEnd] {
							ctx = append(ctx, cl)
						}
					}
					matchingLines = append(matchingLines, gMatchLine{lineNum: i + 1, text: line, context: ctx})
				}
			}
			if len(matchingLines) > 0 {
				fileMatches = append(fileMatches, gFileMatch{path: path, modTime: info.ModTime(), lines: matchingLines})
			}
		}
		return nil
	})
	if walkErr != nil {
		return &ToolResult{Error: fmt.Sprintf("grep: %s", walkErr)}, nil
	}

	durationMs := time.Since(start).Milliseconds()

	// Sort by modification time (newest first)
	sort.Slice(fileMatches, func(i, j int) bool {
		if !fileMatches[i].modTime.Equal(fileMatches[j].modTime) {
			return fileMatches[i].modTime.After(fileMatches[j].modTime)
		}
		return fileMatches[i].path < fileMatches[j].path
	})

	switch outputMode {
	case "content":
		var lines []string
		totalLines := 0
		for _, fm := range fileMatches {
			rel, _ := filepath.Rel(workDir, fm.path)
			for _, ml := range fm.lines {
				if totalLines >= offset {
					if headLimit <= 0 || len(lines) < headLimit {
						lines = append(lines, fmt.Sprintf("%s:%d:%s", rel, ml.lineNum, ml.text))
					}
				}
				totalLines++
			}
		}
		content := strings.Join(lines, "\n")
		appliedLimit := headLimit
		if totalLines <= headLimit {
			appliedLimit = 0
		}
		out, _ := json.Marshal(map[string]any{
			"mode":          "content",
			"numFiles":      len(fileMatches),
			"filenames":     grepFilePaths(workDir, fileMatches),
			"content":       content,
			"numLines":      len(lines),
			"appliedLimit":  appliedLimit,
			"appliedOffset": offset,
			"durationMs":    durationMs,
		})
		return &ToolResult{Output: string(out)}, nil

	case "count":
		totalMatches := 0
		for _, fm := range fileMatches {
			totalMatches += fm.count
		}
		out, _ := json.Marshal(map[string]any{
			"mode":          "count",
			"numFiles":      len(fileMatches),
			"filenames":     grepFilePaths(workDir, fileMatches),
			"numMatches":    totalMatches,
			"appliedLimit":  headLimit,
			"appliedOffset": offset,
			"durationMs":    durationMs,
		})
		return &ToolResult{Output: string(out)}, nil

	default: // files_with_matches
		filenames := grepFilePaths(workDir, fileMatches)
		applied := len(filenames)
		if offset > 0 && offset < len(filenames) {
			filenames = filenames[offset:]
		}
		if headLimit > 0 && len(filenames) > headLimit {
			filenames = filenames[:headLimit]
			applied = headLimit
		} else {
			applied = 0
		}
		out, _ := json.Marshal(map[string]any{
			"mode":          "files_with_matches",
			"numFiles":      len(filenames),
			"filenames":     filenames,
			"appliedLimit":  applied,
			"appliedOffset": offset,
			"durationMs":    durationMs,
		})
		return &ToolResult{Output: string(out)}, nil
	}
}

func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return defaultVal
	}
	return n
}

func grepFilePaths(workDir string, matches []gFileMatch) []string {
	out := make([]string, len(matches))
	for i, fm := range matches {
		rel, _ := filepath.Rel(workDir, fm.path)
		out[i] = rel
	}
	return out
}
