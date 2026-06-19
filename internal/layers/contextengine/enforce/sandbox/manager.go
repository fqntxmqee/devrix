package sandbox

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

// Manager creates isolated filesystem sandboxes for delegated workers (D2-S18).
// This is a mkdir sandbox, not git worktree or D7 WorkTree task model.
type Manager struct {
	cfg config.SandboxConfig
}

func NewManager(cfg config.SandboxConfig) *Manager {
	return &Manager{cfg: config.NormalizeSandboxConfig(cfg)}
}

func (m *Manager) Enabled() bool {
	return m != nil && m.cfg.Enabled
}

func (m *Manager) Enter(_ context.Context, sessionID, slug, _ string) (string, error) {
	if m == nil || !m.cfg.Enabled {
		return "", fmt.Errorf("sandbox: disabled")
	}
	if sessionID == "" {
		return "", fmt.Errorf("sandbox: session_id is required")
	}
	slug = strings.TrimSpace(slug)
	if slug == "" || !slugPattern.MatchString(slug) {
		return "", fmt.Errorf("sandbox: invalid slug %q", slug)
	}
	path := filepath.Join(m.cfg.BaseDir, sessionID, slug)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("sandbox: mkdir %s: %w", path, err)
	}
	return path, nil
}

func (m *Manager) Exit(_ context.Context, path string, keep bool) error {
	if m == nil || path == "" || keep {
		return nil
	}
	base := filepath.Clean(m.cfg.BaseDir)
	target := filepath.Clean(path)
	if !strings.HasPrefix(target, base+string(os.PathSeparator)) && target != base {
		return fmt.Errorf("sandbox: refuse to delete path outside base: %s", target)
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("sandbox: remove %s: %w", target, err)
	}
	return nil
}
