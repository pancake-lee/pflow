// Package session provides tmux + ttyd session management for pflow.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	plogger "github.com/pancake-lee/pgo/pkg/plogger"
)

// ── ~/.claude/sessions/ JSON file structure ──────────────────────────

// claudeSessionFile mirrors the JSON structure of a file in
// ~/.claude/sessions/<pid>.json.
//
// When started with `claude -n <name>`, the "name" field is populated
// and persists across /clear and /resume — only sessionId changes.
type claudeSessionFile struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	CWD       string `json:"cwd"`
	Status    string `json:"status"`
}

// ── Directory scanning ───────────────────────────────────────────────

// claudeSessionsDir returns the path to ~/.claude/sessions/.
func claudeSessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "sessions"), nil
}

// readClaudeSessionFile reads and parses a single ~/.claude/sessions/<pid>.json file.
func readClaudeSessionFile(path string) (*claudeSessionFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sf claudeSessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, err
	}
	// Extract PID from filename if not present in JSON
	if sf.PID == 0 {
		base := filepath.Base(path)
		pidStr := strings.TrimSuffix(base, ".json")
		if pid, err := strconv.Atoi(pidStr); err == nil {
			sf.PID = pid
		}
	}
	return &sf, nil
}

// scanClaudeSessions reads all JSON files from ~/.claude/sessions/ and
// returns parsed session info keyed by PID (filename).
func scanClaudeSessions() (map[int]*claudeSessionFile, error) {
	dir, err := claudeSessionsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no sessions directory yet
		}
		return nil, err
	}

	result := make(map[int]*claudeSessionFile)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		sf, err := readClaudeSessionFile(filepath.Join(dir, e.Name()))
		if err != nil {
			plogger.Debugf("claude_scan: skipping unreadable file %s: %v", e.Name(), err)
			continue
		}
		result[sf.PID] = sf
	}
	return result, nil
}

// findClaudeSessionByName scans ~/.claude/sessions/ for a session file
// whose "name" field matches the given name. Returns the session file
// and a boolean indicating whether it was found.
func findClaudeSessionByName(name string) (*claudeSessionFile, bool) {
	sessions, err := scanClaudeSessions()
	if err != nil {
		plogger.Debugf("claude_scan: scan error: %v", err)
		return nil, false
	}

	for _, sf := range sessions {
		if sf.Name == name {
			return sf, true
		}
	}
	return nil, false
}

// captureClaudeSessionIDByName polls ~/.claude/sessions/ until a session
// file with a matching "name" field appears, or maxWait is exceeded.
//
// Poll frequency: 1 second. Max 10 attempts by default (10 seconds).
//
// Returns (sessionID, pid, error). The pid is recorded for robustness
// — if tmux→pid→name mapping breaks, it indicates an unexpected condition.
func captureClaudeSessionIDByName(name string, maxWait time.Duration) (sessionID string, pid int, err error) {
	deadline := time.Now().Add(maxWait)
	attempt := 0

	for time.Now().Before(deadline) {
		attempt++
		sf, found := findClaudeSessionByName(name)
		if found {
			plogger.Infof("claude_scan: found session name=%s sessionId=%s pid=%d (attempt %d)",
				name, sf.SessionID, sf.PID, attempt)
			return sf.SessionID, sf.PID, nil
		}

		if attempt == 1 || attempt%5 == 0 {
			plogger.Debugf("claude_scan: attempt %d: name=%s not yet found in ~/.claude/sessions/",
				attempt, name)
		}
		time.Sleep(1 * time.Second)
	}

	return "", 0, fmt.Errorf("timeout after %.0fs: no session with name=%q found in ~/.claude/sessions/", maxWait.Seconds(), name)
}

// ── Background monitoring ────────────────────────────────────────────

// claudeSessionWatcher monitors ~/.claude/sessions/ for changes to a
// specific named session and invokes onChanged when the sessionId changes.
type claudeSessionWatcher struct {
	mu           sync.Mutex
	name         string
	lastSessionID string
	lastPID      int
	interval     time.Duration
	onChanged    func(name, oldSessionID, newSessionID string, pid int)
	stopCh       chan struct{}
}

// startClaudeSessionWatcher begins polling ~/.claude/sessions/ at the
// configured interval. When the sessionId changes for the tracked name,
// onChanged is called. Call Stop() to halt the watcher.
func startClaudeSessionWatcher(name, currentSessionID string, pid int, interval time.Duration, onChanged func(name, oldSessionID, newSessionID string, pid int)) *claudeSessionWatcher {
	w := &claudeSessionWatcher{
		name:          name,
		lastSessionID: currentSessionID,
		lastPID:       pid,
		interval:      interval,
		onChanged:     onChanged,
		stopCh:        make(chan struct{}),
	}

	go w.run()
	plogger.Infof("claude_scan: watcher started for name=%s sessionId=%s pid=%d interval=%s",
		name, currentSessionID, pid, interval)
	return w
}

func (w *claudeSessionWatcher) run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.check()
		}
	}
}

func (w *claudeSessionWatcher) check() {
	sf, found := findClaudeSessionByName(w.name)
	if !found {
		plogger.Debugf("claude_scan: watcher: name=%s not found (session may have exited)", w.name)
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Check PID consistency — if tmux→pid→name relationship changed unexpectedly
	if w.lastPID > 0 && sf.PID != w.lastPID {
		plogger.Warnf("claude_scan: PID changed for name=%s: %d → %d (tmux may have been recreated?)",
			w.name, w.lastPID, sf.PID)
	}

	if sf.SessionID != w.lastSessionID {
		oldID := w.lastSessionID
		w.lastSessionID = sf.SessionID
		w.lastPID = sf.PID

		plogger.Infof("claude_scan: sessionId changed for name=%s: %s → %s (pid=%d)",
			w.name, oldID, sf.SessionID, sf.PID)

		if w.onChanged != nil {
			w.onChanged(w.name, oldID, sf.SessionID, sf.PID)
		}
	}
}

// Stop halts the background watcher.
func (w *claudeSessionWatcher) Stop() {
	close(w.stopCh)
	plogger.Infof("claude_scan: watcher stopped for name=%s", w.name)
}

// ── Sync helper (new approach via directory scan) ────────────────────

// syncClaudeMappingByName re-scans ~/.claude/sessions/ for a named Claude
// mapping and returns the updated session info. Returns nil if the session
// is no longer found (dead tmux session or Claude exited).
func syncClaudeMappingByName(m *Mapping) *claudeSessionFile {
	if m.AgentName == "" {
		return nil
	}

	sf, found := findClaudeSessionByName(m.AgentName)
	if !found {
		plogger.Debugf("claude_scan: sync: name=%s no longer in sessions dir", m.AgentName)
		return nil
	}

	return sf
}

// SyncClaudeMappingsDirScan refreshes Claude mappings using the directory
// scanning approach. For each Claude mapping with an agent name, it checks
// ~/.claude/sessions/ and updates the mapping if sessionId or PID changed.
//
// Returns the number of updated mappings.
func SyncClaudeMappingsDirScan() (int, error) {
	mm, err := newMappingManager()
	if err != nil {
		return 0, err
	}

	store, err := mm.load()
	if err != nil {
		return 0, err
	}

	updated := 0
	for i := range store.Mappings {
		m := &store.Mappings[i]
		if m.AgentType != "claude" || m.AgentName == "" {
			continue
		}

		sf := syncClaudeMappingByName(m)
		if sf == nil {
			continue
		}

		changed := false
		if sf.SessionID != m.AgentSessionID {
			plogger.Infof("claude_scan: sync: name=%s sessionId changed: %s → %s",
				m.AgentName, m.AgentSessionID, sf.SessionID)
			m.AgentSessionID = sf.SessionID
			m.ClaudePrefix = sf.SessionID[:min(8, len(sf.SessionID))]
			changed = true
		}
		if sf.PID != m.PID {
			plogger.Infof("claude_scan: sync: name=%s pid changed: %d → %d",
				m.AgentName, m.PID, sf.PID)
			m.PID = sf.PID
			changed = true
		}
		if sf.Status != m.Status {
			m.Status = sf.Status
			changed = true
		}
		if changed {
			m.LastUpdated = time.Now()
			updated++
		}
	}

	if updated > 0 {
		if err := mm.save(store); err != nil {
			return updated, fmt.Errorf("sync: save failed after %d updates: %w", updated, err)
		}
		plogger.Infof("claude_scan: sync: updated %d Claude mapping(s)", updated)
	}

	// Clean stale mappings (tmux sessions no longer alive)
	if cleaned, _ := mm.cleanStale(); cleaned > 0 {
		plogger.Infof("claude_scan: sync: cleaned %d stale mapping(s)", cleaned)
	}

	return updated, nil
}
