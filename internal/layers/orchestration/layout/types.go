package layout

import "time"

// CodeLocation is the `<root>/<pkg>/<file>.go[:Func]` triple recorded in
// a-registry.md / f-registry.md Code Location columns. Root is the on-disk
// directory containing the package (e.g. `internal/layers/orchestration`,
// `internal/bootstrap`).
type CodeLocation struct {
	Root    string
	Package string
	File    string
	Func    string
	raw     string
}

func newCodeLocation(raw string) CodeLocation {
	return CodeLocation{raw: raw}
}

// Raw returns the original string as it appears in the registry column.
func (c CodeLocation) Raw() string { return c.raw }

// RelativePath returns the path relative to the repository root, or "" if
// the location cannot be resolved from raw text.
func (c CodeLocation) RelativePath() string {
	if c.Root == "" || c.File == "" {
		return ""
	}
	if c.Package == "" {
		return c.Root + "/" + c.File
	}
	return c.Root + "/" + c.Package + "/" + c.File
}

// RegistryEntry is one row extracted from a-registry.md or f-registry.md.
type RegistryEntry struct {
	ID           string
	CodeLocation CodeLocation
	SourceFile   string
	Status       string // ✅ / 🔶 / ⬜ / ❌
	IsCanonical  bool   // true when Status == ✅
}

// OrphanDirViolation reports a directory that exists under
// orchestration/ but is not present in code-layout.md §4.2.
type OrphanDirViolation struct {
	Path   string
	Reason string
}

// ResurrectViolation reports a retired directory or top-level file that
// reappeared in orchestration/.
type ResurrectViolation struct {
	Path    string
	RetiredBy string
}

// MissingLocation reports a Code Location registered in a/f-registry but
// whose physical .go file does not exist on disk.
type MissingLocation struct {
	RegistryFile string
	EntryID      string
	Path         string
}

// GuardReport aggregates every violation produced by a GuardScan.
type GuardReport struct {
	ChangeID         string
	ScannedAt        time.Time
	OrphanDirs       []OrphanDirViolation
	ResurrectedDirs  []ResurrectViolation
	MissingLocations []MissingLocation
}

// HasViolations reports whether the report contains any failure-causing entries.
func (g GuardReport) HasViolations() bool {
	return len(g.OrphanDirs) > 0 || len(g.ResurrectedDirs) > 0 || len(g.MissingLocations) > 0
}

// Total counts all violations across categories.
func (g GuardReport) Total() int {
	return len(g.OrphanDirs) + len(g.ResurrectedDirs) + len(g.MissingLocations)
}