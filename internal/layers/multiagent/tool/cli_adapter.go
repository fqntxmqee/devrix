package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// CLIConfig holds configuration for a CLI agent tool.
type CLIConfig struct {
	Name         string
	DisplayName  string
	Description  string
	Capabilities []string
	Role         string // LLM role description for tool decision
	Command      string
	Args         []string
	WorkDir      string
	Timeout      time.Duration
	IdleTimeout  time.Duration
	MaxSessions  int // 0 = unlimited
}

// CLISession wraps a long-running subprocess.
type CLISession struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Scanner
	createdAt  time.Time
	lastUsedAt time.Time
	mu         sync.Mutex // serializes access to this session
}

// CLIAgentTool implements AgentTool for CLI subprocesses with session management.
type CLIAgentTool struct {
	cfg      CLIConfig
	info     Info
	sessions map[string]*CLISession // keyed by D1 sessionID
	mu       sync.RWMutex
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewCLIAgentTool creates a CLI agent tool with session management.
func NewCLIAgentTool(cfg CLIConfig) *CLIAgentTool {
	info := Info{
		Name:         cfg.Name,
		DisplayName:  cfg.DisplayName,
		Description:  cfg.Description,
		Capabilities: cfg.Capabilities,
		Role:         cfg.Role,
	}
	t := &CLIAgentTool{
		cfg:      cfg,
		info:     info,
		sessions: make(map[string]*CLISession),
		stopCh:   make(chan struct{}),
	}
	if cfg.IdleTimeout > 0 {
		t.wg.Add(1)
		go t.idleSweeper()
	}
	return t
}

// Info returns the tool's identity metadata.
func (t *CLIAgentTool) Info() Info { return t.info }

// Execute sends a task to the agent tool and streams events until complete.
func (t *CLIAgentTool) Execute(ctx context.Context, sessionID string, req Request) (<-chan Event, error) {
	sess, err := t.ensureSession(ctx, sessionID, req.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.cfg.Name, err)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	// Build stdin message (stream-json protocol)
	msg := map[string]any{
		"type": "user",
		"message": map[string]string{
			"role":    "user",
			"content": req.Task,
		},
	}
	data, _ := json.Marshal(msg)

	if _, err := fmt.Fprintln(sess.stdin, string(data)); err != nil {
		return nil, fmt.Errorf("%s: write stdin: %w", t.cfg.Name, err)
	}
	sess.lastUsedAt = time.Now()

	ch := make(chan Event)
	go func() {
		defer close(ch)

		// Close stdin on context cancellation to unblock the scanner.
		go func() {
			<-ctx.Done()
			sess.stdin.Close()
		}()

		for sess.stdout.Scan() {
			line := strings.TrimSpace(sess.stdout.Text())
			if line == "" {
				continue
			}

			parsed := ParseStreamJSONLine(line)
			for _, evt := range parsed.Events {
				select {
				case ch <- evt:
				case <-ctx.Done():
					return
				}
			}
			if parsed.Done {
				return
			}
		}
		if err := sess.stdout.Err(); err != nil {
			ch <- Event{Type: "error", Content: fmt.Sprintf("stdout error: %v", err)}
		}
	}()

	return ch, nil
}

// ensureSession returns an existing session or creates a new one.
func (t *CLIAgentTool) ensureSession(ctx context.Context, sessionID string, workDir string) (*CLISession, error) {
	t.mu.RLock()
	sess, ok := t.sessions[sessionID]
	t.mu.RUnlock()
	if ok {
		return sess, nil
	}

	t.mu.RLock()
	count := len(t.sessions)
	t.mu.RUnlock()
	if t.cfg.MaxSessions > 0 && count >= t.cfg.MaxSessions {
		return nil, fmt.Errorf("max sessions reached (%d)", t.cfg.MaxSessions)
	}

	// Start new subprocess
	cmd := exec.Command(t.cfg.Command, t.cfg.Args...)
	if dir := strings.TrimSpace(workDir); dir != "" {
		cmd.Dir = dir
	} else if t.cfg.WorkDir != "" {
		cmd.Dir = t.cfg.WorkDir
	}

	if ctx != nil {
		if envVars := tracer.PropagationEnvVars(ctx); len(envVars) > 0 {
			cmd.Env = append(os.Environ(), envVars...)
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	// Drain stderr asynchronously to prevent blocking
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			slog.Warn("agent tool stderr",
				"tool", t.cfg.Name,
				"session", sessionID,
				"line", line,
			)
		}
	}()

	sess = &CLISession{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     bufio.NewScanner(stdout),
		createdAt:  time.Now(),
		lastUsedAt: time.Now(),
	}

	t.mu.Lock()
	t.sessions[sessionID] = sess
	t.mu.Unlock()

	slog.Info("agent tool session created", "tool", t.cfg.Name, "session", sessionID)
	return sess, nil
}

// CloseSession terminates a session with three-phase shutdown.
func (t *CLIAgentTool) CloseSession(sessionID string) error {
	t.mu.Lock()
	sess, ok := t.sessions[sessionID]
	if !ok {
		t.mu.Unlock()
		return nil
	}
	delete(t.sessions, sessionID)
	t.mu.Unlock()

	return t.closeSession(sess)
}

func (t *CLIAgentTool) closeSession(sess *CLISession) error {
	if sess == nil {
		return nil
	}

	// Phase 1: graceful — close stdin
	// Ignore stdin close errors — the pipe may already be closed by a concurrent
	// context cancellation in Execute().
	_ = sess.stdin.Close()

	done := make(chan error, 1)
	go func() {
		done <- sess.cmd.Wait()
	}()

	// Phase 2: wait for graceful exit
	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
	}

	// Phase 3: SIGTERM
	if err := sess.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		slog.Warn("SIGTERM", "tool", t.cfg.Name, "error", err)
	}

	select {
	case <-done:
		return nil
	case <-time.After(3 * time.Second):
	}

	// Phase 4: SIGKILL
	if err := sess.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("kill: %w", err)
	}
	<-done
	return nil
}

// CleanupBySessionID closes all sessions for the given D1 Session ID.
func (t *CLIAgentTool) CleanupBySessionID(sessionID string) {
	t.CloseSession(sessionID)
}

// Stop terminates all sessions and stops background goroutines.
func (t *CLIAgentTool) Stop() {
	t.mu.Lock()
	select {
	case <-t.stopCh:
		// already closed
		t.mu.Unlock()
		return
	default:
		close(t.stopCh)
	}
	t.mu.Unlock()
	t.wg.Wait()

	t.mu.Lock()
	sessions := make(map[string]*CLISession, len(t.sessions))
	for k, v := range t.sessions {
		sessions[k] = v
	}
	t.sessions = make(map[string]*CLISession)
	t.mu.Unlock()

	for _, sess := range sessions {
		t.closeSession(sess)
	}
}

// idleSweeper periodically reaps idle sessions.
func (t *CLIAgentTool) idleSweeper() {
	defer t.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.reapIdle()
		}
	}
}

func (t *CLIAgentTool) reapIdle() {
	t.mu.Lock()
	var idle []string
	now := time.Now()
	for id, sess := range t.sessions {
		if now.Sub(sess.lastUsedAt) > t.cfg.IdleTimeout {
			idle = append(idle, id)
		}
	}
	t.mu.Unlock()

	for _, id := range idle {
		slog.Info("reaping idle session", "tool", t.cfg.Name, "session", id)
		t.CloseSession(id)
	}
}
