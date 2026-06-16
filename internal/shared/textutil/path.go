// Package textutil provides text and path utility functions shared across domains.
package textutil

import (
	"os"
	"path/filepath"
)

// ExpandPath expands ~ and environment variables in a file path.
func ExpandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}
	return os.ExpandEnv(path)
}
