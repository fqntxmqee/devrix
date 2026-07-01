package layout

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// allowedTopLevelDirs are the scenario-slug directories the layout guard
// accepts under internal/layers/orchestration/. Mirrors code-layout.md §4.2
// v1.13.0 (DM-20260701-004 PR-1).
var allowedTopLevelDirs = map[string]bool{
	"workmodel":         true,
	"sessionorchestrator": true,
	"wavescheduler":     true,
	"executionflow":     true,
	"decisionplanning":  true,
	"plan":              true,
	"mups":              true,
	"escape":            true,
	"interfaces":        true,
	"hardening":         true,
	"orchtypes":         true,
	"delegatetools":     true,
	"layout":            true, // this package itself
}

// knownRootPrefixes maps the leading path segment of a Code Location to its
// on-disk repository root. References under `orchestration/` are stripped of
// the prefix; references under sibling roots (`bootstrap`, other layers) are
// preserved verbatim.
var knownRootPrefixes = map[string]string{
	"orchestration":  "internal/layers/orchestration",
	"bootstrap":      "internal/bootstrap",
	"communication":  "internal/layers/communication",
	"contextengine":  "internal/layers/contextengine",
	"llmgateway":     "internal/layers/llmgateway",
	"multiagent":     "internal/layers/multiagent",
	"observability":  "internal/layers/observability",
	"evolution":      "internal/layers/evolution",
}

// retiredTopLevelDirs are directories that were git-rm'd by earlier changes
// and must never resurrect under internal/layers/orchestration/.
var retiredTopLevelDirs = map[string]string{
	"coordinator": "DM-20260619-005",
	"hubspoke":    "DM-20260619-005",
	"observe":     "DM-20260626-002",
	"turn":        "DM-20260626-004",
	"milestone":   "v2.x",
	"queryloop":   "DM-20260617-001",
}

// retiredTopLevelFiles are retired singleton files at orchestration root.
var retiredTopLevelFiles = map[string]string{
	"coordinator.go": "DM-20260619-005",
	"hubspoke.go":    "DM-20260619-005",
	"turn.go":        "DM-20260626-004",
	"milestone.go":   "v2.x",
	"fastpath.go":    "DM-20260626-008",
}

// codePathRE matches the file path tokens embedded inside a registry Code
// Location cell. Captures `pkg/file.go` style references; backtick-bounded.
var codePathRE = regexp.MustCompile("`([a-zA-Z][a-zA-Z0-9_]*(?:/[a-zA-Z][a-zA-Z0-9_]*)*/[a-zA-Z][a-zA-Z0-9_]*\\.go)`")

// codePathBraceRE matches `{a,b,c}.go` brace expansion patterns inside a
// registry cell, e.g. workmodel/{work_tree,task_manager}.go.
var codePathBraceRE = regexp.MustCompile("`([a-zA-Z][a-zA-Z0-9_]*)/\\{([a-zA-Z0-9_,]+)\\}\\.go`")

// registryRowRE matches a single registry table row that starts with an
// activity identifier (D7-S*, Hardening-*, D7-X-*) and captures the Status
// column value (the cell containing ✅ / 🔶 / ⬜ etc.).
var registryRowRE = regexp.MustCompile(`^\|\s*(\*{0,2}(?:D7-S\d+|Hardening-[A-Z]\d+|D7-X-[A-Z]\d+)-A\d+(?:-[A-Z]\d+)?[A-Z0-9-]*\*{0,2})\s*\|`)

// statusCellRE extracts the glyph (✅ / 🔶 / ⬜ / ❌) from a single Status
// table cell so the test can skip non-✅ rows.
var statusCellRE = regexp.MustCompile(`✅|🔶|⬜|❌`)

// RegistrySourceFiles enumerates the registry markdown files this guard scans.
var RegistrySourceFiles = []string{
	"openspec/specs/d7-orchestration/a-registry.md",
	"openspec/specs/d7-orchestration/f-registry.md",
}

// ParseRegistryEntries reads a single registry markdown file and returns
// every entry whose Code Location cell is parseable.
func ParseRegistryEntries(repoRoot, relativePath string) ([]RegistryEntry, error) {
	full := filepath.Join(repoRoot, relativePath)
	f, err := os.Open(full)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", relativePath, err)
	}
	defer f.Close()

	var entries []RegistryEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		m := registryRowRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id := strings.Trim(m[1], "*")

		status := "⬜"
		if sm := statusCellRE.FindString(line); sm != "" {
			status = sm
		}

		locs := extractCodeLocations(line)
		for _, loc := range locs {
			entries = append(entries, RegistryEntry{
				ID:           id,
				CodeLocation: loc,
				SourceFile:   relativePath,
				Status:       status,
				IsCanonical:  status == "✅",
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", relativePath, err)
	}
	return entries, nil
}

// extractCodeLocations pulls every `pkg/file.go` reference (with brace
// expansion) out of one registry row.
func extractCodeLocations(line string) []CodeLocation {
	var out []CodeLocation

	for _, m := range codePathBraceRE.FindAllStringSubmatch(line, -1) {
		pkg := m[1]
		files := strings.Split(m[2], ",")
		for _, file := range files {
			out = append(out, newCodeLocation(pkg+"/"+file+".go"))
		}
	}

	for _, m := range codePathRE.FindAllStringSubmatch(line, -1) {
		raw := m[1]
		if !strings.HasSuffix(raw, ".go") {
			continue
		}
		out = append(out, newCodeLocation(raw))
	}

	return out
}

// ResolveCodeLocation populates the Root / Package / File fields on a
// CodeLocation given its raw form. The first path segment names the root
// (`orchestration`, `bootstrap`, other D{N} layer roots); the remainder is
// the package + file inside that root.
func ResolveCodeLocation(loc CodeLocation) CodeLocation {
	raw := loc.Raw()
	if raw == "" {
		return loc
	}
	parts := strings.Split(raw, "/")
	if len(parts) < 2 {
		return loc
	}

	root, ok := knownRootPrefixes[parts[0]]
	if !ok {
		return loc
	}
	loc.Root = root

	tail := parts[1:]
	if last := tail[len(tail)-1]; strings.Contains(last, ":") {
		idx := strings.Index(last, ":")
		loc.Func = last[idx+1:]
		tail[len(tail)-1] = last[:idx]
	}
	loc.File = tail[len(tail)-1]
	if len(tail) >= 2 {
		loc.Package = strings.Join(tail[:len(tail)-1], "/")
	}
	return loc
}

// ScanDirectories walks internal/layers/orchestration/ and reports orphan
// directories and resurrected retired entries.
func ScanDirectories(repoRoot string) (orphans []OrphanDirViolation, resurrected []ResurrectViolation, err error) {
	orchDir := filepath.Join(repoRoot, "internal", "layers", "orchestration")
	entries, err := os.ReadDir(orchDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read orchestration dir: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() {
			if retired, ok := retiredTopLevelFiles[name]; ok {
				resurrected = append(resurrected, ResurrectViolation{
					Path:      filepath.Join("internal/layers/orchestration", name),
					RetiredBy: retired,
				})
			}
			continue
		}
		if retired, ok := retiredTopLevelDirs[name]; ok {
			resurrected = append(resurrected, ResurrectViolation{
				Path:      filepath.Join("internal/layers/orchestration", name),
				RetiredBy: retired,
			})
			continue
		}
		if !allowedTopLevelDirs[name] {
			orphans = append(orphans, OrphanDirViolation{
				Path:   filepath.Join("internal/layers/orchestration", name),
				Reason: "not in code-layout.md §4.2 allow-list",
			})
		}
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].Path < orphans[j].Path })
	sort.Slice(resurrected, func(i, j int) bool { return resurrected[i].Path < resurrected[j].Path })
	return orphans, resurrected, nil
}

// CheckRegistryLocations verifies that every canonical (Status ✅) registry
// entry's physical .go file exists on disk. Rows with partial / planned /
// deprecated statuses (🔶 / ⬜ / ❌) are skipped — those document historical
// intent, not current canonical wiring.
func CheckRegistryLocations(repoRoot string, entries []RegistryEntry) []MissingLocation {
	var missing []MissingLocation
	for _, e := range entries {
		if !e.IsCanonical {
			continue
		}
		resolved := ResolveCodeLocation(e.CodeLocation)
		path := resolved.RelativePath()
		if path == "" {
			continue
		}
		full := filepath.Join(repoRoot, path)
		if _, err := os.Stat(full); err != nil {
			missing = append(missing, MissingLocation{
				RegistryFile: e.SourceFile,
				EntryID:      e.ID,
				Path:         path,
			})
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].RegistryFile != missing[j].RegistryFile {
			return missing[i].RegistryFile < missing[j].RegistryFile
		}
		return missing[i].Path < missing[j].Path
	})
	return missing
}

// GuardScan runs every layout check and returns the aggregated report.
func GuardScan(repoRoot, changeID string) (GuardReport, error) {
	report := GuardReport{
		ChangeID:  changeID,
		ScannedAt: time.Now(),
	}

	orphans, resurrected, err := ScanDirectories(repoRoot)
	if err != nil {
		return report, err
	}
	report.OrphanDirs = orphans
	report.ResurrectedDirs = resurrected

	var allEntries []RegistryEntry
	for _, src := range RegistrySourceFiles {
		entries, err := ParseRegistryEntries(repoRoot, src)
		if err != nil {
			return report, err
		}
		allEntries = append(allEntries, entries...)
	}
	report.MissingLocations = CheckRegistryLocations(repoRoot, allEntries)

	return report, nil
}