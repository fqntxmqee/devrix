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
func TestD1ChannelAdapters_NoForbiddenImports(t *testing.T) {
	root := "."
	if _, err := os.Stat("internal/layers/communication/channel/adapters"); err == nil {
		root = "internal/layers/communication/channel/adapters"
	}
	forbidden := []string{
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
