package prompt

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type agentsDocument struct {
	path    string
	content string
}

type agentsContextCache struct {
	entries map[string]string
}

var globalAgentsContextCache = &agentsContextCache{entries: make(map[string]string)}

// ClearAgentsCache drops cached merged AGENTS.md for all work directories.
func ClearAgentsCache() {
	globalAgentsContextCache.entries = make(map[string]string)
}

func (c *agentsContextCache) get(workDir string) (string, bool) {
	v, ok := c.entries[workDir]
	return v, ok
}

func (c *agentsContextCache) set(workDir, content string) {
	if c.entries == nil {
		c.entries = make(map[string]string)
	}
	c.entries[workDir] = content
}

func (l *Loader) discoverAgentsDocuments(workDir string) []agentsDocument {
	cfg := l.cfg.Normalized()
	var docs []agentsDocument

	if globalPath := expandTildePath(cfg.UserGlobal); globalPath != "" {
		if doc, ok := l.readAgentsFile(globalPath, cfg.IncludeEnabled()); ok {
			docs = append(docs, doc)
		}
	}

	dirs := []string{workDir}
	if cfg.WalkUpEnabled() && workDir != "" {
		dirs = ancestorDirs(workDir)
	} else if workDir != "" {
		if abs, err := filepath.Abs(workDir); err == nil {
			dirs = []string{abs}
		}
	}

	loadSources := sourcesLoadOrder(cfg.Sources)
	for _, dir := range dirs {
		if ruleDocs := l.loadRulesDocuments(dir, cfg.RulesGlob, cfg.IncludeEnabled()); len(ruleDocs) > 0 {
			docs = append(docs, ruleDocs...)
		}
		for _, src := range loadSources {
			path := src
			if !filepath.IsAbs(src) {
				path = filepath.Join(dir, src)
			}
			if doc, ok := l.readAgentsFile(path, cfg.IncludeEnabled()); ok {
				docs = append(docs, doc)
			}
		}
	}
	return docs
}

func (l *Loader) loadAgentsContext(workDir string) string {
	if v, ok := globalAgentsContextCache.get(workDir); ok {
		return v
	}
	cfg := l.cfg.Normalized()
	docs := l.discoverAgentsDocuments(workDir)
	merged := mergeAgentsDocuments(docs)
	if merged == "" {
		merged = cfg.Fallback
	}
	if cfg.MaxChars > 0 && len(merged) > cfg.MaxChars {
		merged = merged[:cfg.MaxChars] + "\n... (AGENTS context truncated) ..."
	}
	globalAgentsContextCache.set(workDir, merged)
	return merged
}

func (l *Loader) readAgentsFile(path string, enableInclude bool) (agentsDocument, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return agentsDocument{}, false
	}
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return agentsDocument{}, false
	}
	content := expandAgentsIncludes(raw, path, enableInclude, nil, 0)
	if strings.TrimSpace(content) == "" {
		return agentsDocument{}, false
	}
	return agentsDocument{path: path, content: content}, true
}

func (l *Loader) loadRulesDocuments(dir, rulesGlob string, enableInclude bool) []agentsDocument {
	if rulesGlob == "" {
		return nil
	}
	pattern := rulesGlob
	if !filepath.IsAbs(rulesGlob) {
		pattern = filepath.Join(dir, rulesGlob)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}
	sort.Strings(matches)
	out := make([]agentsDocument, 0, len(matches))
	for _, path := range matches {
		if doc, ok := l.readAgentsFile(path, enableInclude); ok {
			out = append(out, doc)
		}
	}
	return out
}

func mergeAgentsDocuments(docs []agentsDocument) string {
	parts := make([]string, 0, len(docs))
	for _, d := range docs {
		if strings.TrimSpace(d.content) == "" {
			continue
		}
		parts = append(parts, d.content)
	}
	return strings.Join(parts, "\n\n")
}

// sourcesLoadOrder returns sources from lowest to highest priority within a directory.
func sourcesLoadOrder(sources []string) []string {
	if len(sources) <= 1 {
		return sources
	}
	out := make([]string, len(sources))
	for i, s := range sources {
		out[len(sources)-1-i] = s
	}
	return out
}

func ancestorDirs(workDir string) []string {
	abs, err := filepath.Abs(workDir)
	if err != nil || abs == "" {
		return nil
	}
	var chain []string
	for {
		chain = append(chain, abs)
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

func expandTildePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		if path == "~" {
			return home
		}
		return filepath.Clean(filepath.Join(home, strings.TrimPrefix(path, "~/")))
	}
	return filepath.Clean(path)
}