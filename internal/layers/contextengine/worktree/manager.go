package worktree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devrix/devrix/internal/shared/config"
)

var slugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// Manager creates isolated filesystem sandboxes for delegated workers.
type Manager struct {
	cfg config.WorktreeConfig
}

// NewManager creates a worktree manager.
func NewManager(cfg config.WorktreeConfig) *Manager {
	return &Manager{cfg: config.NormalizeWorktreeConfig(cfg)}
}

// Enabled reports whether worktree isolation is active.
func (m *Manager) Enabled() bool {
	return m != nil && m.cfg.Enabled
}

// Enter creates or reuses a sandbox directory for sessionID+slug.
func (m *Manager) Enter(_ context.Context, sessionID, slug, _ string) (string, error) {
	if m == nil || !m.cfg.Enabled {
		return "", fmt.Errorf("worktree: disabled")
	}
	if sessionID == "" {
		return "", fmt.Errorf("worktree: session_id is required")
	}
	slug = strings.TrimSpace(slug)
	if slug == "" || !slugPattern.MatchString(slug) {
		return "", fmt.Errorf("worktree: invalid slug %q", slug)
	}
	path := filepath.Join(m.cfg.BaseDir, sessionID, slug)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("worktree: mkdir %s: %w", path, err)
	}
	return path, nil
}

// Exit removes the sandbox unless keep is true.
func (m *Manager) Exit(_ context.Context, path string, keep bool) error {
	if m == nil || path == "" || keep {
		return nil
	}
	base := filepath.Clean(m.cfg.BaseDir)
	target := filepath.Clean(path)
	if !strings.HasPrefix(target, base+string(os.PathSeparator)) && target != base {
		return fmt.Errorf("worktree: refuse to delete path outside base: %s", target)
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("worktree: remove %s: %w", target, err)
	}
	return nil
}
