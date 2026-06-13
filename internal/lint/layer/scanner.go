// Package layer implements the devrix layer-lint static-analysis scanner.
//
// The scanner walks Go source files in internal/layers/, identifies the layer
// (D1..D6) each package belongs to, and reports every D{N}->D{M} import where
// N > M (i.e. a higher layer reaching down to a lower one). The default
// dependency direction is strictly upward (D1 is the lowest).
//
// T: CROSS-A01-T01  (layer-lint detects reverse D{N}→D{N} imports)
// T: CROSS-A01-T02  (CI gate uses this scanner to block violations)
package layer

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// Layer is the canonical identifier of a domain layer (D1..D6).
type Layer string

// Canonical layer IDs.
const (
	D1 Layer = "D1" // Communication
	D2 Layer = "D2" // Context Engine
	D3 Layer = "D3" // LLM Gateway
	D4 Layer = "D4" // Multi-Agent
	D5 Layer = "D5" // Observability
	D6 Layer = "D6" // Evolution
)

// Matrix is the set of forbidden layer→layer edges. Higher-numbered layers
// must never import lower-numbered ones.
type Matrix struct {
	forbidden map[edge]struct{}
}

type edge struct{ from, to Layer }

// DefaultMatrix returns the canonical matrix where D{N} → D{M} (N>M) is
// forbidden. The internal/shared package is always permitted (it is
// domain-neutral by design — see DM-20260611-002).
func DefaultMatrix() *Matrix {
	m := &Matrix{forbidden: map[edge]struct{}{}}
	layers := []Layer{D1, D2, D3, D4, D5, D6}
	for i, hi := range layers {
		for j, lo := range layers {
			if i > j {
				m.forbidden[edge{hi, lo}] = struct{}{}
			}
		}
	}
	return m
}

// IsForbidden reports whether from→to is a violation.
func (m *Matrix) IsForbidden(from, to Layer) bool {
	_, ok := m.forbidden[edge{Layer(from), Layer(to)}]
	return ok
}

// PackageRef is a discovered package: its on-disk file key, derived layer,
// and the absolute import paths it depends on.
type PackageRef struct {
	File    string   // synthetic key e.g. "internal/layers/contextengine/engine.go"
	Layer   Layer    // best-effort classification
	Imports []string // raw import paths
}

// Violation is a single forbidden import edge discovered by the scanner.
type Violation struct {
	From   Layer  `json:"from"`
	To     Layer  `json:"to"`
	File   string `json:"file"`
	Import string `json:"import"`
}

// ParseImportGraphFromSources walks the given Go sources, groups them into
// "packages" by directory, and returns one PackageRef per directory with the
// union of all imports. This indirection lets tests exercise the scanner
// without touching the filesystem.
//
// Exposed under the unexported name parseImportGraphFromSources; the public
// entry point is ParseImportGraph which walks a real directory tree.
func parseImportGraphFromSources(files map[string]string) ([]*PackageRef, error) {
	byDir := map[string]*PackageRef{}
	for path, src := range files {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		dir := dirOf(path)
		ref, ok := byDir[dir]
		if !ok {
			ref = &PackageRef{File: dir, Layer: classifyDir(dir)}
			byDir[dir] = ref
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
		if err != nil {
			return nil, err
		}
		for _, imp := range f.Imports {
			val := strings.Trim(imp.Path.Value, `"`)
			ref.Imports = append(ref.Imports, val)
		}
	}
	out := make([]*PackageRef, 0, len(byDir))
	for _, ref := range byDir {
		sort.Strings(ref.Imports)
		out = append(out, ref)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out, nil
}

// ParseImportGraph walks a directory tree and produces PackageRefs. Used by
// the devrix-layer-lint CLI.
func ParseImportGraph(root string) ([]*PackageRef, error) {
	files, err := walkGoFiles(root)
	if err != nil {
		return nil, err
	}
	return parseImportGraphFromSources(files)
}

// ScanPackages runs the matrix against a package graph and returns every
// violation, sorted by file then by (from,to).
func ScanPackages(pkgs []*PackageRef, m *Matrix) []Violation {
	var out []Violation
	for _, p := range pkgs {
		if p.Layer == "" {
			continue
		}
		for _, imp := range p.Imports {
			target := classifyImport(imp)
			if target == "" {
				continue
			}
			if m.IsForbidden(p.Layer, target) {
				out = append(out, Violation{
					From:   p.Layer,
					To:     target,
					File:   p.File,
					Import: imp,
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

// FormatText returns a human-readable listing of violations.
func FormatText(vs []Violation) string {
	var b strings.Builder
	for _, v := range vs {
		b.WriteString(string(v.From))
		b.WriteString(" -> ")
		b.WriteString(string(v.To))
		b.WriteString("  ")
		b.WriteString(v.File)
		b.WriteString("  (imports ")
		b.WriteString(v.Import)
		b.WriteString(")\n")
	}
	return b.String()
}

// FormatJSON returns the canonical JSON wire form used by the CI gate.
func FormatJSON(vs []Violation) string {
	b, _ := json.Marshal(vs)
	return string(b)
}

// classifyDir assigns a layer label based on the directory path.
func classifyDir(dir string) Layer {
	dir = strings.TrimPrefix(dir, "./")
	parts := strings.Split(dir, "/")
	for i, p := range parts {
		if p != "layers" {
			continue
		}
		if i+1 >= len(parts) {
			break
		}
		switch parts[i+1] {
		case "communication":
			return D1
		case "contextengine":
			return D2
		case "llmgateway":
			return D3
		case "multiagent":
			return D4
		case "observability":
			return D5
		case "evolution":
			return D6
		}
		break
	}
	return ""
}

// classifyImport resolves a Go import path back to a layer. Returns "" when
// the import is not a devrix D{N} package (e.g. stdlib, third-party, or
// internal/shared which is domain-neutral).
func classifyImport(p string) Layer {
	idx := strings.Index(p, "/internal/layers/")
	if idx < 0 {
		return ""
	}
	rest := p[idx+len("/internal/layers/"):]
	parts := strings.Split(rest, "/")
	if len(parts) == 0 {
		return ""
	}
	switch parts[0] {
	case "communication":
		return D1
	case "contextengine":
		return D2
	case "llmgateway":
		return D3
	case "multiagent":
		return D4
	case "observability":
		return D5
	case "evolution":
		return D6
	}
	return ""
}

func dirOf(path string) string {
	ix := strings.LastIndex(path, "/")
	if ix < 0 {
		return "."
	}
	return path[:ix]
}
