// Package session provides tmux + ttyd session management for pflow.
package session

import (
	"fmt"
	"path/filepath"
	"time"

	plogger "github.com/pancake-lee/pgo/pkg/plogger"
)

// ── Claude session management ──────────────────────────────────────

// ── Full Claude session startup ────────────────────────────────────

// StartClaudeSession creates a tmux session and starts Claude Code inside it.
// Uses claude -n <name> --permission-mode acceptEdits and scans
// ~/.claude/sessions/ for JSON files with a matching "name" field to
// extract the session ID + PID.
//
// Parameters:
//   - name: desired tmux session name (will be sanitized). Also used as
//     Claude's -n name for stable identity across /clear and /resume.
//   - workDir: working directory for the session
//
// Returns the created Session. Session ID capture is asynchronous — the
// mapping is saved in the background when capture succeeds.
func (m *Manager) StartClaudeSession(name, workDir string) (*Session, string, error) {
	absDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, "", fmt.Errorf("cannot resolve path %q: %w", workDir, err)
	}
	return m.startClaudeSessionDirScan(name, absDir)
}

// startClaudeSessionDirScan starts Claude using the directory scanning approach.
// Uses claude -n <name> for stable identity across /clear and /resume.
func (m *Manager) startClaudeSessionDirScan(name, absDir string) (*Session, string, error) {
	// The claudeName is the stable identifier — used both for -n and for
	// scanning ~/.claude/sessions/. We use the raw name (before sanitization
	// for tmux) as the Claude -n value.
	claudeName := name
	if claudeName == "" {
		claudeName = filepath.Base(absDir)
	}

	sess, err := m.launch(launchConfig{
		name:      name,
		agentName: claudeName,
		workDir:   absDir,
		agentType: "claude",
		command:   fmt.Sprintf("cd %s && claude -n %s --permission-mode acceptEdits", absDir, claudeName),
		preLaunch: nil,
		captureSessionID: func(tmuxName string, maxWait time.Duration) (string, error) {
			sessionID, pid, err := captureClaudeSessionIDByName(claudeName, maxWait)
			if err != nil {
				return "", err
			}
			_ = pid
			return sessionID, nil
		},
	})
	if err != nil {
		return nil, "", err
	}
	return sess, "", nil
}

// ── Tmux ↔ Claude lookup ──────────────────────────────────────────

// LookupResult describes the result of looking up a tmux session by
// agent session ID.
type LookupResult struct {
	Session  *Session // nil if no match
	Verified bool     // always false (no live verification without statusline)
	Warning  string   // non-empty when the mapping might be stale
}

// LookupByClaudeSessionID finds a pflow-managed tmux session that is
// associated with the given Claude session ID. It matches by the first
// 8 characters of the session ID.
func (m *Manager) LookupByClaudeSessionID(claudeSessionID string) (*LookupResult, error) {
	if len(claudeSessionID) < 8 {
		return nil, fmt.Errorf("session ID too short: %q", claudeSessionID)
	}
	prefix := claudeSessionID[:8]
	plogger.Debugf("lookup: searching for claude session prefix=%s (full=%s)", prefix, claudeSessionID)

	mm, err := newMappingManager()
	if err != nil {
		return nil, err
	}

	matches, err := mm.findByClaudePrefix(prefix)
	if err != nil {
		return nil, err
	}
	plogger.Debugf("lookup: prefix=%s found %d saved mapping(s)", prefix, len(matches))
	if len(matches) == 0 {
		plogger.Infof("lookup: no mapping for prefix=%s — session may not be pflow-managed", prefix)
		return &LookupResult{}, nil
	}

	// Walk matches and find the first with a living tmux session.
	for _, match := range matches {
		tmuxAlive := tmuxSessionExists(match.TmuxName)
		plogger.Debugf("lookup: checking match tmux=%s prefix=%s tmuxAlive=%v", match.TmuxName, match.ClaudePrefix, tmuxAlive)
		if !tmuxAlive {
			plogger.Infof("lookup: skipping dead tmux session %s (prefix=%s)", match.TmuxName, match.ClaudePrefix)
			continue
		}

		var sess *Session
		m.mu.Lock()
		existing, ok := m.sessions[match.TmuxName]
		m.mu.Unlock()
		if ok && m.isAlive(existing) {
			sess = existing
		} else {
			sess = &Session{
				Name:    match.TmuxName,
				WorkDir: match.WorkDir,
			}
		}

		plogger.Infof("lookup: FOUND tmux=%s for prefix=%s (saved mapping)", match.TmuxName, prefix)
		return &LookupResult{Session: sess, Verified: false}, nil
	}

	plogger.Infof("lookup: all %d mapping(s) for prefix=%s have dead tmux sessions", len(matches), prefix)
	return &LookupResult{}, nil
}

// LookupBySessionID finds a pflow-managed tmux session associated with the
// given agent session. It supports both Claude and Hermes session IDs.
func (m *Manager) LookupBySessionID(agentType, sessionID string) (*LookupResult, error) {
	mm, err := newMappingManager()
	if err != nil {
		return nil, err
	}

	plogger.Debugf("lookup: searching for agent=%s sessionID=%s", agentType, sessionID)

	// Try full session ID match first (for Hermes and new Claude mappings)
	matches, err := mm.findBySessionID(agentType, sessionID)
	if err != nil {
		return nil, err
	}

	// Fall back to prefix-based search (for legacy Claude mappings)
	if len(matches) == 0 && agentType == "claude" && len(sessionID) >= 8 {
		prefix := sessionID[:8]
		matches, err = mm.findByClaudePrefix(prefix)
		if err != nil {
			return nil, err
		}
		plogger.Debugf("lookup: prefix=%s found %d saved mapping(s) via legacy search", prefix, len(matches))
	} else {
		plogger.Debugf("lookup: found %d saved mapping(s) by session ID", len(matches))
	}

	if len(matches) == 0 {
		plogger.Infof("lookup: no mapping for agent=%s sessionID=%s", agentType, truncate8(sessionID))
		return &LookupResult{}, nil
	}

	// Walk matches and find the first with a living tmux session.
	for _, match := range matches {
		if !tmuxSessionExists(match.TmuxName) {
			plogger.Infof("lookup: skipping dead tmux session %s", match.TmuxName)
			continue
		}

		var sess *Session
		m.mu.Lock()
		existing, ok := m.sessions[match.TmuxName]
		m.mu.Unlock()
		if ok && m.isAlive(existing) {
			sess = existing
		} else {
			sess = &Session{
				Name:    match.TmuxName,
				WorkDir: match.WorkDir,
			}
		}

		plogger.Infof("lookup: FOUND tmux=%s for agent=%s sessionID=%s (saved mapping)",
			match.TmuxName, agentType, truncate8(sessionID))
		return &LookupResult{
			Session:  sess,
			Verified: false,
		}, nil
	}

	plogger.Infof("lookup: all %d mapping(s) for agent=%s sessionID=%s have dead tmux sessions",
		len(matches), agentType, truncate8(sessionID))
	return &LookupResult{}, nil
}

// truncate8 returns the first 8 chars of s, or s itself if shorter.
func truncate8(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

