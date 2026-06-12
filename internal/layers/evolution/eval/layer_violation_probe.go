// Package eval — LayerViolationProbe (D6-S3).
//
// The probe runs the devrix layer-lint scanner over internal/layers/ at
// probe-execution time, counts the resulting D{N}→D{M} (N>M) violations,
// and emits a 0-1 score: 1.0 = zero violations, 0.5 = one violation, ≤ 0
// for two or more. The probe is registered automatically in init().
//
// Covers: L5-0-0-04  (D6 LayerViolationProbe registered and reports a score)
package eval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/devrix/devrix/internal/lint/layer"
)

func init() {
	RegisterProbe(&LayerViolationProbe{})
}

// LayerViolationProbe measures the number of reverse D{N}→D{M} imports that
// the layer-lint scanner finds in the devrix repository.
type LayerViolationProbe struct{}

// ID is the stable identifier used by the eval runner and the eval dataset
// format. Do NOT rename without updating eval items that reference it.
func (p *LayerViolationProbe) ID() string { return "layer_violation" }

// Run executes the layer-lint scanner. The "root" field of item.Input is
// honoured when present; otherwise the probe walks "internal/layers"
// relative to the repository root (auto-detected by walking up until it
// finds go.mod). Falls back to the current working directory + "internal/layers".
func (p *LayerViolationProbe) Run(ctx context.Context, item EvalItem, _ Judge) (*DomainScore, error) {
	root := "internal/layers"
	if r, ok := item.Input["root"].(string); ok && r != "" {
		root = r
	}
	if !filepath.IsAbs(root) {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
	}
	if _, err := os.Stat(root); err != nil {
		// Try resolving "internal/layers" relative to the repo root (where
		// go.mod lives). This keeps the probe location-independent.
		if repoRoot, ferr := findRepoRoot(); ferr == nil {
			candidate := filepath.Join(repoRoot, "internal", "layers")
			if _, serr := os.Stat(candidate); serr == nil {
				root = candidate
			} else {
				return nil, fmt.Errorf("layer_violation probe: cannot stat root %s: %w", root, err)
			}
		} else {
			return nil, fmt.Errorf("layer_violation probe: cannot stat root %s: %w", root, err)
		}
	}

	pkgs, err := layer.ParseImportGraph(root)
	if err != nil {
		return nil, fmt.Errorf("layer_violation probe: scan: %w", err)
	}
	violations := layer.ScanPackages(pkgs, layer.DefaultMatrix())
	count := len(violations)
	score := p.scoreFromViolations(count)

	details := map[string]float64{
		"reverse_import_count":         float64(count),
		"score":                        score,
		"runtime.layer_violation_refs": float64(count),
	}
	buckets := map[string]float64{}
	if item.Bucket != "" {
		buckets[item.Bucket] = score
	}
	return &DomainScore{
		Domain:    "d6",
		Dimension: p.ID(),
		Score:     score,
		Details:   details,
		Buckets:   buckets,
	}, nil
}

// scoreFromViolations encodes the policy the probe enforces:
//
//	0 violations → 1.0, 1 violation → 0.5, 2+ → 0.0.
//
// Centralised so tests can pin the formula.
func (p *LayerViolationProbe) scoreFromViolations(count int) float64 {
	switch {
	case count <= 0:
		return 1.0
	case count == 1:
		return 0.5
	default:
		return 0.0
	}
}

// findRepoRoot walks up from the current working directory until it finds
// go.mod, returning the directory that contains it. Returns an error if
// the search reaches the filesystem root.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
