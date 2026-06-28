package adapters

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T: D1-RF-T09 — channel/adapters production code must not import D7 implementation packages.
//
// hardening/ is the cross-cutting Discipline Keeper (span helpers only,
// no D7 implementation logic). It's the bridge between observability/
// and the D7 orchestrator — D1 is allowed to import it because the
// hardening package is intentionally package-agnostic (no D7 types in
// signatures, no D7-specific logic). Excluding hardening would force D1
// to inline span emission code, defeating the purpose of the central
// helper layer.
func TestD1ChannelAdapters_NoForbiddenImports(t *testing.T) {
	root := "."
	if _, err := os.Stat("internal/layers/communication/channel/adapters"); err == nil {
		root = "internal/layers/communication/channel/adapters"
	}
	forbidden := []string{
		"github.com/devrix/devrix/internal/layers/orchestration/",
	}
	allowedPrefixes := []string{
		"github.com/devrix/devrix/internal/layers/orchestration/hardening",
	}
	fset := token.NewFileSet()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			allowed := false
			for _, ok := range allowedPrefixes {
				if p == ok || strings.HasPrefix(p, ok) {
					allowed = true
					break
				}
			}
			if allowed {
				continue
			}
			for _, ban := range forbidden {
				if p == ban || strings.HasPrefix(p, ban) {
					t.Errorf("%s imports forbidden %q", path, p)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}
