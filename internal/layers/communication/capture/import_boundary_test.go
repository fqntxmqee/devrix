package capture

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T: D1-RF-T01 — D1 capture production code must not import D4/D7 implementation packages.
func TestD1Capture_NoForbiddenImports(t *testing.T) {
	root := "."
	if _, err := os.Stat("internal/layers/communication/capture"); err == nil {
		root = "internal/layers/communication/capture"
	}
	forbidden := []string{
		"github.com/devrix/devrix/internal/layers/multiagent",
		"github.com/devrix/devrix/internal/layers/orchestration/",
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
