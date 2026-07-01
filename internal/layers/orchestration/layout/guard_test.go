// Layout guard tests for D7 orchestration (DM-20260701-004 PR-2).
//
// T: D7-PL-T07 TestOrphanDirs — physical dirs not in code-layout.md §4.2 allow-list
// T: D7-PL-T08 TestNoResurrectRetiredDirs — retired dirs must not reappear
// T: D7-PL-T09 TestACanonicalLocationsExist — a-registry Code Locations resolve to disk
// T: D7-PL-T10 TestFCanonicalLocationsExist — f-registry Code Locations resolve to disk
// T: D7-PL-T11 TestGhostDirsInCodeLayout — code-layout.md §4.2 scenario-slugs exist on disk
// T: D7-PL-T12 TestNoRetiredTopLevelFiles — retired top-level .go files must not reappear
package layout

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// layoutRepoRoot resolves the absolute path of the devrix repository root.
func layoutRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))
}

// readScenarioSlugsFromCodeLayout extracts every `orchestration/<slug>/` path
// referenced in code-layout.md §4.2 for the D7 table.
func readScenarioSlugsFromCodeLayout(t *testing.T, repoRoot string) []string {
	t.Helper()
	path := filepath.Join(repoRoot, "openspec", "specs", "architecture", "code-layout.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read code-layout.md: %v", err)
	}
	content := string(data)
	const startMarker = "### 4.2 D7 Orchestration"
	const endMarker = "### 4.3"
	start := strings.Index(content, startMarker)
	if start < 0 {
		t.Fatalf("could not find %s in code-layout.md", startMarker)
	}
	end := strings.Index(content[start+len(startMarker):], endMarker)
	if end < 0 {
		t.Fatalf("could not find %s after §4.2 in code-layout.md", endMarker)
	}
	section := content[start : start+len(startMarker)+end]

	var slugs []string
	for _, line := range strings.Split(section, "\n") {
		if !strings.Contains(line, "orchestration/") {
			continue
		}
		idx := strings.Index(line, "orchestration/")
		rest := line[idx+len("orchestration/"):]
		end := strings.IndexAny(rest, " `|\t/")
		if end <= 0 {
			continue
		}
		candidate := rest[:end]
		candidate = strings.TrimRight(candidate, "/")
		if candidate == "" || strings.Contains(candidate, " ") {
			continue
		}
		slugs = append(slugs, candidate)
	}
	return slugs
}

func TestOrphanDirs(t *testing.T) {
	repo := layoutRepoRoot(t)
	orphans, _, err := ScanDirectories(repo)
	if err != nil {
		t.Fatalf("ScanDirectories: %v", err)
	}
	if len(orphans) > 0 {
		var lines []string
		for _, o := range orphans {
			lines = append(lines, "  - "+o.Path+" ("+o.Reason+")")
		}
		t.Fatalf("orphan directories under internal/layers/orchestration/:\n%s\n"+
			"→ register the slug in code-layout.md §4.2 or remove the directory",
			strings.Join(lines, "\n"))
	}
}

func TestNoResurrectRetiredDirs(t *testing.T) {
	repo := layoutRepoRoot(t)
	_, resurrected, err := ScanDirectories(repo)
	if err != nil {
		t.Fatalf("ScanDirectories: %v", err)
	}
	if len(resurrected) > 0 {
		var lines []string
		for _, r := range resurrected {
			lines = append(lines, "  - "+r.Path+" (retired by "+r.RetiredBy+")")
		}
		t.Fatalf("retired directories or top-level files resurrected under internal/layers/orchestration/:\n%s\n"+
			"→ these were git-rm'd; restore the migration or open a new change to re-introduce them",
			strings.Join(lines, "\n"))
	}
}

func TestACanonicalLocationsExist(t *testing.T) {
	repo := layoutRepoRoot(t)
	entries, err := ParseRegistryEntries(repo, "openspec/specs/d7-orchestration/a-registry.md")
	if err != nil {
		t.Fatalf("ParseRegistryEntries (a-registry): %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("a-registry yielded 0 entries — parser regression?")
	}
	missing := CheckRegistryLocations(repo, entries)
	if len(missing) > 0 {
		t.Fatalf("%d a-registry Code Locations missing on disk:\n%s",
			len(missing), formatMissing(missing))
	}
}

func TestFCanonicalLocationsExist(t *testing.T) {
	repo := layoutRepoRoot(t)
	entries, err := ParseRegistryEntries(repo, "openspec/specs/d7-orchestration/f-registry.md")
	if err != nil {
		t.Fatalf("ParseRegistryEntries (f-registry): %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("f-registry yielded 0 entries — parser regression?")
	}
	missing := CheckRegistryLocations(repo, entries)
	if len(missing) > 0 {
		t.Fatalf("%d f-registry Code Locations missing on disk:\n%s",
			len(missing), formatMissing(missing))
	}
}

func TestGhostDirsInCodeLayout(t *testing.T) {
	repo := layoutRepoRoot(t)
	slugs := readScenarioSlugsFromCodeLayout(t, repo)
	if len(slugs) == 0 {
		t.Fatal("code-layout.md §4.2 yielded 0 scenario-slugs — section rewrite?")
	}

	seen := map[string]bool{}
	var missing []string
	for _, slug := range slugs {
		if seen[slug] {
			continue
		}
		seen[slug] = true
		full := filepath.Join(repo, "internal", "layers", "orchestration", slug)
		if info, err := os.Stat(full); err != nil || !info.IsDir() {
			missing = append(missing, "orchestration/"+slug+" (referenced in code-layout.md §4.2 but absent)")
		}
	}
	if len(missing) > 0 {
		t.Fatalf("ghost entries in code-layout.md §4.2:\n  - %s\n"+
			"→ either create the directory or remove the line from code-layout.md",
			strings.Join(missing, "\n  - "))
	}
}

func TestNoRetiredTopLevelFiles(t *testing.T) {
	repo := layoutRepoRoot(t)
	orchDir := filepath.Join(repo, "internal", "layers", "orchestration")
	entries, err := os.ReadDir(orchDir)
	if err != nil {
		t.Fatalf("ReadDir orchestration: %v", err)
	}
	var resurrected []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if retired, ok := retiredTopLevelFiles[e.Name()]; ok {
			resurrected = append(resurrected, "  - "+e.Name()+" (retired by "+retired+")")
		}
	}
	if len(resurrected) > 0 {
		t.Fatalf("retired top-level files under internal/layers/orchestration/:\n%s\n"+
			"→ these were git-rm'd; restore the migration or open a new change to re-introduce them",
			strings.Join(resurrected, "\n"))
	}
}

func formatMissing(missing []MissingLocation) string {
	var lines []string
	for _, m := range missing {
		lines = append(lines, "  - "+m.RegistryFile+" "+m.EntryID+" → "+m.Path)
	}
	return strings.Join(lines, "\n")
}