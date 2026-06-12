// Package session provides tmux + ttyd session management for pflow.
// It creates managed tmux sessions and exposes them as web terminals via ttyd.
package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Session represents a managed terminal session.
type Session struct {
	Name     string `json:"name"`
	WorkDir  string `json:"work_dir"`
	TtydPort int    `json:"ttyd_port"`
	TtydURL  string `json:"ttyd_url"`

	cmd     *exec.Cmd
	started time.Time
}

// Manager manages tmux sessions and their ttyd web terminal processes.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session // keyed by sanitized name
	basePort int
	host     string // host for constructing ttyd URLs
}

// NewManager creates a new session Manager.
func NewManager(basePort int, host string) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		basePort: basePort,
		host:     host,
	}
}

// Start creates a tmux session (if not exists) and starts ttyd to expose it.
// The session name is derived from the workDir's base name.
func (m *Manager) Start(workDir string) (*Session, error) {
	// Validate dependencies
	if err := checkDeps(); err != nil {
		return nil, err
	}

	// Resolve and validate workDir
	absDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve path %q: %w", workDir, err)
	}
	if info, err := os.Stat(absDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("directory %q does not exist or is not a directory", absDir)
	}

	// Derive a unique session name from the workDir
	name := sanitizeName(filepath.Base(absDir))
	name = m.uniqueName(name)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we already have a session for this workDir
	for _, s := range m.sessions {
		if s.WorkDir == absDir {
			if m.isAlive(s) {
				return s, nil
			}
			// Session is dead, clean it up
			m.cleanupLocked(s)
			break
		}
	}

	// 1. Ensure tmux session exists
	if err := ensureTmux(name, absDir); err != nil {
		return nil, err
	}

	// 2. Allocate port
	port := m.allocatePort()

	// 3. Start ttyd
	cmd, err := startTtyd(port, name, m.host)
	if err != nil {
		// Clean up tmux session on failure
		exec.Command("tmux", "kill-session", "-t", name).Run()
		return nil, fmt.Errorf("failed to start ttyd: %w", err)
	}

	sess := &Session{
		Name:     name,
		WorkDir:  absDir,
		TtydPort: port,
		TtydURL:  fmt.Sprintf("http://%s:%d", m.host, port),
		cmd:      cmd,
		started:  time.Now(),
	}
	m.sessions[name] = sess

	return sess, nil
}

// StartExisting attaches ttyd to an already-running tmux session without
// creating a new one. This is used when the frontend discovers a pflow-managed
// tmux session via lookup and wants to open a web terminal to it.
func (m *Manager) StartExisting(tmuxName, workDir string) (*Session, error) {
	// Validate dependencies
	if err := checkDeps(); err != nil {
		return nil, err
	}

	// Verify the tmux session exists
	if !tmuxSessionExists(tmuxName) {
		return nil, fmt.Errorf("tmux session %q does not exist", tmuxName)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we already have ttyd running for this session
	if sess, ok := m.sessions[tmuxName]; ok {
		if m.isAlive(sess) {
			return sess, nil
		}
		m.cleanupLocked(sess)
		delete(m.sessions, tmuxName)
	}

	// Resolve workDir
	absDir := workDir
	if absDir == "" {
		var err error
		absDir, err = filepath.Abs(".")
		if err != nil {
			return nil, fmt.Errorf("cannot resolve work_dir: %w", err)
		}
	} else {
		var err error
		absDir, err = filepath.Abs(workDir)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve path %q: %w", workDir, err)
		}
	}

	// Allocate port and start ttyd
	port := m.allocatePort()
	cmd, err := startTtyd(port, tmuxName, m.host)
	if err != nil {
		return nil, fmt.Errorf("failed to start ttyd: %w", err)
	}

	sess := &Session{
		Name:     tmuxName,
		WorkDir:  absDir,
		TtydPort: port,
		TtydURL:  fmt.Sprintf("http://%s:%d", m.host, port),
		cmd:      cmd,
		started:  time.Now(),
	}
	m.sessions[tmuxName] = sess

	return sess, nil
}

// Stop terminates the ttyd process and optionally kills the tmux session.
func (m *Manager) Stop(name string, killTmux bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[name]
	if !ok {
		return fmt.Errorf("session %q not found", name)
	}

	m.cleanupLocked(sess)
	delete(m.sessions, name)

	if killTmux {
		exec.Command("tmux", "kill-session", "-t", name).Run()
	}

	return nil
}

// List returns all currently managed sessions.
func (m *Manager) List() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Filter out dead sessions
	var alive []*Session
	for name, s := range m.sessions {
		if m.isAlive(s) {
			alive = append(alive, s)
		} else {
			delete(m.sessions, name)
		}
	}
	return alive
}

// Get returns a session by name.
func (m *Manager) Get(name string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[name]
	if s != nil && !m.isAlive(s) {
		delete(m.sessions, name)
		return nil
	}
	return s
}

// ── internal helpers ──────────────────────────────────────────────

func (m *Manager) isAlive(s *Session) bool {
	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}
	// Check if ttyd process is still running
	// Process.Signal(nil) is a no-op on Unix; it checks if the process exists.
	return s.cmd.Process.Signal(os.Signal(nil)) == nil
}

func (m *Manager) cleanupLocked(s *Session) {
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		// Best-effort wait to avoid zombie processes
		_ = s.cmd.Wait()
	}
}

func (m *Manager) allocatePort() int {
	used := make(map[int]bool)
	for _, s := range m.sessions {
		used[s.TtydPort] = true
	}
	port := m.basePort
	for used[port] {
		port++
	}
	return port
}

func (m *Manager) uniqueName(base string) string {
	// If base is already used, append a number
	name := base
	for i := 1; m.sessions[name] != nil; i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}
	return name
}

// ── system helpers ────────────────────────────────────────────────

// checkDeps verifies that tmux and ttyd are installed.
func checkDeps() error {
	for _, bin := range []string{"tmux", "ttyd"} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s is not installed (%w); please install it first", bin, err)
		}
	}
	return nil
}

// checkClaudeDeps verifies that tmux, ttyd, and jq are installed
// (jq is required by the Claude statusline command).
func checkClaudeDeps() error {
	for _, bin := range []string{"tmux", "ttyd", "jq"} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s is not installed (%w); please install it first", bin, err)
		}
	}
	return nil
}

// ensureTmux creates a tmux session if it doesn't already exist.
// The session runs bash in the given working directory.
func ensureTmux(name, workDir string) error {
	// Check if session already exists
	if err := exec.Command("tmux", "has-session", "-t", name).Run(); err == nil {
		return nil // session exists
	}

	// Create new detached session running bash
	create := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", workDir, "bash")
	if out, err := create.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux new-session failed: %w (output: %s)", err, string(out))
	}
	return nil
}

// startTtyd starts a ttyd process bound to the given port.
// Command: ttyd -p <port> -i <host> tmux attach -t <name>
func startTtyd(port int, sessionName, host string) (*exec.Cmd, error) {
	args := []string{
		"-p", strconv.Itoa(port),
		"-i", host,
		"tmux", "attach", "-t", sessionName,
	}
	cmd := exec.Command("ttyd", args...)

	// Forward stderr for diagnostics
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ttyd start failed: %w", err)
	}

	// Give ttyd a moment to bind the port
	time.Sleep(300 * time.Millisecond)

	return cmd, nil
}

// tmuxSessionExists returns true if a tmux session with the given name exists.
func tmuxSessionExists(name string) bool {
	return exec.Command("tmux", "has-session", "-t", name).Run() == nil
}

// sanitizeName converts a path component into a valid tmux session name.
func sanitizeName(s string) string {
	// Replace characters that are problematic for tmux session names
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_' || r == '.':
			return r
		default:
			return '-'
		}
	}, s)
	// Trim leading/trailing dashes
	s = strings.Trim(s, "-")
	if s == "" {
		s = "pflow"
	}
	return "pflow-" + s
}
