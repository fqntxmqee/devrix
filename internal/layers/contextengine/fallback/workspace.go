package fallback

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// ScanWorkspace walks workDir and builds WorkspaceContext metadata.
func ScanWorkspace(workDir string, cfg config.HarnessPrefetchConfig) (types.WorkspaceContext, error) {
	ws := types.WorkspaceContext{
		WorkDir:   workDir,
		ScannedAt: time.Now(),
	}
	if workDir == "" {
		return ws, nil
	}
	info, err := os.Stat(workDir)
	if err != nil {
		return ws, err
	}
	if !info.IsDir() {
		return ws, nil
	}

	maxDepth := cfg.MaxWalkDepth
	if maxDepth <= 0 {
		maxDepth = 4
	}

	agentsPaths := []string{"AGENTS.md", ".devrix/AGENTS.md"}
	for _, rel := range agentsPaths {
		if _, err := os.Stat(filepath.Join(workDir, rel)); err == nil {
			ws.AgentsMDPresent = true
			break
		}
	}

	rootSet := make(map[string]struct{})
	err = filepath.WalkDir(workDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if path != workDir {
				rel, _ := filepath.Rel(workDir, path)
				depth := strings.Count(rel, string(os.PathSeparator)) + 1
				if depth > maxDepth {
					return filepath.SkipDir
				}
			}
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".go") {
			ws.GoFileCount++
			root := topLevelDir(workDir, path)
			rootSet[root] = struct{}{}
			if strings.HasSuffix(name, "_test.go") {
				ws.TestFileCount++
			}
		}
		return nil
	})
	if err != nil {
		return ws, err
	}
	for root := range rootSet {
		ws.SourceRoots = append(ws.SourceRoots, root)
	}
	return ws, nil
}

func topLevelDir(workDir, filePath string) string {
	rel, err := filepath.Rel(workDir, filePath)
	if err != nil || rel == "." {
		return "."
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) == 1 {
		return "."
	}
	return parts[0]
}

// CheckGuards validates runtime prerequisites for harness bootstrap.
func CheckGuards(workDir string) error {
	if workDir == "" {
		return nil
	}
	info, err := os.Stat(workDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return os.ErrNotExist
	}
	testFile := filepath.Join(workDir, ".devrix_harness_guard")
	f, err := os.Create(testFile)
	if err != nil {
		return err
	}
	_ = f.Close()
	return os.Remove(testFile)
}
