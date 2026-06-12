// Filesystem helper for the layer-lint scanner.
package layer

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// walkGoFiles walks root recursively and returns every .go file as
// {relativePath: content}. It silently skips vendor and .git directories.
func walkGoFiles(root string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == ".git" || strings.HasPrefix(name, ".") {
				if path != root {
					return fs.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		out[rel] = string(data)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
