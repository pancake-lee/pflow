// Package session provides tmux + ttyd session management for pflow.
package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	plogger "github.com/pancake-lee/pgo/pkg/plogger"
)

// launchConfig holds the parameters for starting an AI agent in a tmux session.
// Agent-specific behavior is injected via the PreLaunch and CaptureSessionID
// callbacks, keeping the core tmux lifecycle logic shared across agent types.
type launchConfig struct {
	name             string // tmux session name (will be sanitized; may be empty for auto-derivation)
	agentName        string // stable agent display name (Claude -n name, Hermes session name)
	workDir          string
	agentType        string // "claude", "hermes", or "codex"
	command          string // full command to send to tmux (e.g., "claude -n foo --permission-mode acceptEdits")
	preLaunch        func() error
	captureSessionID func(tmuxName string, maxWait time.Duration) (string, error)
}

// launch creates a tmux session and starts an AI agent inside it using the
// shared tmux lifecycle logic. Agent-specific setup and session ID capture
// strategy is injected via the config callbacks.
//
// The flow mirrors the simplicity of a hand-written tmux script:
//
//	tmux new-session -d -s <name> -c <workDir>
//	tmux send-keys -t <name> "<command>" C-m
//
// Session ID capture and mapping persistence are handled asynchronously so the
// caller can attach to tmux immediately without waiting for the agent to finish
// initializing.
func (m *Manager) launch(cfg launchConfig) (*Session, error) {
	// Only tmux is required for the core flow.
	if _, err := exec.LookPath("tmux"); err != nil {
		return nil, fmt.Errorf("tmux is not installed (%w); please install it first", err)
	}

	// Resolve workDir
	absDir, err := filepath.Abs(cfg.workDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve path %q: %w", cfg.workDir, err)
	}
	if info, err := os.Stat(absDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("directory %q does not exist or is not a directory", absDir)
	}

	// Sanitize tmux session name
	name := cfg.name
	if name == "" {
		name = sanitizeName(filepath.Base(absDir))
	} else {
		name = sanitizeName(name)
	}

	// Compute agent display name for the mapping:
	//   1. cfg.agentName if explicitly set (e.g., Claude -n name)
	//   2. cfg.name if the user provided -name
	//   3. fallback: raw base name of the working directory
	displayName := cfg.agentName
	if displayName == "" {
		displayName = cfg.name
	}
	if displayName == "" {
		displayName = filepath.Base(absDir)
	}

	// Ensure uniqueness
	m.mu.Lock()
	name = m.uniqueName(name)
	m.mu.Unlock()

	// If tmux session already exists, check if it's orphaned (no clients attached)
	// or actively in use by another terminal. Existing sessions still go through
	// asynchronous mapping capture so a restarted pflow codex can finish a
	// mapping that was pending before the restart.
	sessionExists := false
	if tmuxSessionExists(name) {
		if tmuxHasClients(name) {
			return nil, fmt.Errorf(
				"tmux session %q is already in use by another terminal; use -name to create a separate session, or attach manually: tmux attach -t %s",
				name, name,
			)
		}
		// Orphaned session: reconnect without restarting the agent.
		sessionExists = true
		if cfg.agentType == "codex" {
			m.registerPendingMapping(name, absDir, displayName)
		}
	}

	if !sessionExists {
		// 1. Run agent-specific pre-launch hook.
		if cfg.preLaunch != nil {
			if err := cfg.preLaunch(); err != nil {
				plogger.Warnf("launch: preLaunch hook failed for %s: %v", cfg.agentType, err)
				// Non-fatal — don't block tmux+agent startup.
			}
		}

		// 2. Create tmux session with auto-destroy and Ctrl+Z protection.
		//
		// The shell command:
		//   trap '' TSTP        — block Ctrl+Z (prevents suspending the agent)
		//   <agent_command>     — start the agent
		//   tmux kill-session   — auto-destroy the container when agent exits
		//
		// No separate send-keys step needed — the command runs as the
		// tmux session's initial shell, not sent via send-keys.
		shellCmd := fmt.Sprintf("trap '' TSTP; %s; tmux kill-session -t %s", cfg.command, name)
		plogger.Debugf("launch: creating tmux session name=%s workDir=%s agent=%s", name, absDir, cfg.agentType)
		create := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", absDir, shellCmd)
		if out, err := create.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("tmux new-session failed: %w (output: %s)", err, string(out))
		}
		if cfg.agentType == "codex" {
			m.registerPendingMapping(name, absDir, displayName)
		}
	}

	// 4. Launch async session ID capture in background.
	//    The agent takes a few seconds to initialize and display its session
	//    identifier — we return immediately so the user can attach to tmux
	//    without waiting. The mapping is saved asynchronously when the
	//    capture succeeds.
	if cfg.captureSessionID != nil {
		tmuxName := name
		absWorkDir := absDir
		agentType := cfg.agentType
		go func() {
			plogger.Debugf("launch: async capture started for tmux=%s agent=%s", tmuxName, agentType)
			start := time.Now()
			sessionID, err := cfg.captureSessionID(tmuxName, 15*time.Second)
			elapsed := time.Since(start)
			if err != nil {
				plogger.Warnf("launch: async capture error for tmux=%s agent=%s: %v", tmuxName, agentType, err)
				return
			}
			if sessionID != "" {
				plogger.Infof("launch: async captured sessionID=%s for tmux=%s agent=%s (took %.1fs)",
					sessionID, tmuxName, agentType, elapsed.Seconds())
				if mm, err := newMappingManager(); err == nil {
					mm.addMapping(Mapping{
						TmuxName:       tmuxName,
						WorkDir:        absWorkDir,
						AgentType:      agentType,
						AgentName:      displayName,
						AgentSessionID: sessionID,
						ClaudePrefix:   claudePrefixFromSessionID(sessionID, agentType),
						Status:         "active",
						CreatedAt:      time.Now(),
						LastUpdated:    time.Now(),
					})
				}
			} else {
				plogger.Warnf("launch: async capture returned empty for tmux=%s agent=%s after %.1fs",
					tmuxName, agentType, elapsed.Seconds())
			}
		}()
	}

	return &Session{
		Name:    name,
		WorkDir: absDir,
	}, nil
}

// registerPendingMapping records a managed Codex tmux session before its
// rollout becomes visible. Codex may not write the rollout until the first
// user request, so the mapping must exist while the session is idle.
func (m *Manager) registerPendingMapping(tmuxName, workDir, agentName string) {
	mm, err := newMappingManager()
	if err != nil {
		plogger.Warnf("mapping: cannot create pending Codex mapping for tmux=%s: %v", tmuxName, err)
		return
	}
	now := time.Now()
	if err := mm.addMapping(Mapping{
		TmuxName:    tmuxName,
		WorkDir:     workDir,
		AgentType:   "codex",
		AgentName:   agentName,
		Status:      "pending",
		CreatedAt:   now,
		LastUpdated: now,
	}); err != nil {
		plogger.Warnf("mapping: cannot save pending Codex mapping for tmux=%s: %v", tmuxName, err)
	}
}

// claudePrefixFromSessionID returns an 8-char prefix from a session ID, used
// to populate the legacy ClaudePrefix field for dashboard backward compatibility.
func claudePrefixFromSessionID(sessionID, agentType string) string {
	if agentType == "claude" {
		if len(sessionID) >= 8 {
			return sessionID[:8]
		}
		return sessionID
	}
	return ""
}
