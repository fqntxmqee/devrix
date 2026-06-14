package prompt

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxIncludeDepth = 16

var includeLinePattern = regexp.MustCompile(`^\s*@\S+\s*$`)

// expandAgentsIncludes resolves @include lines in AGENTS.md content (ClawCode @ notation).
func expandAgentsIncludes(content, baseFile string, enableInclude bool, seen map[string]struct{}, depth int) string {
	if !enableInclude || strings.TrimSpace(content) == "" {
		return strings.TrimSpace(content)
	}
	if depth > maxIncludeDepth {
		return strings.TrimSpace(content)
	}
	if seen == nil {
		seen = make(map[string]struct{})
	}
	baseDir := filepath.Dir(baseFile)
	var out strings.Builder
	inCodeBlock := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		if inCodeBlock {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		m := includeLinePattern.FindStringSubmatch(line)
		if m == nil {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		incPath := resolveIncludePath(line, baseDir)
		if incPath == "" {
			continue
		}
		if _, ok := seen[incPath]; ok {
			continue
		}
		data, err := os.ReadFile(incPath)
		if err != nil {
			continue
		}
		seen[incPath] = struct{}{}
		expanded := expandAgentsIncludes(string(data), incPath, enableInclude, seen, depth+1)
		if expanded != "" {
			if out.Len() > 0 {
				out.WriteByte('\n')
			}
			out.WriteString(expanded)
			out.WriteByte('\n')
		}
	}
	return strings.TrimSpace(out.String())
}

func resolveIncludePath(rawLine, baseDir string) string {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rawLine), "@"))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "~/") || raw == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		if raw == "~" {
			return home
		}
		return filepath.Clean(filepath.Join(home, strings.TrimPrefix(raw, "~/")))
	}
	if strings.HasPrefix(raw, "/") {
		return filepath.Clean(raw)
	}
	if strings.HasPrefix(raw, "./") {
		return filepath.Clean(filepath.Join(baseDir, strings.TrimPrefix(raw, "./")))
	}
	return filepath.Clean(filepath.Join(baseDir, raw))
}
