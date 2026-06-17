// Package session provides tmux + ttyd session management for pflow.
package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	plogger "github.com/pancake-lee/pgo/pkg/plogger"
)

// HermesOptions holds optional parameters for starting a Hermes agent session.
type HermesOptions struct {
	Model  string // --model flag (e.g., "deepseek-v4-flash")
	Resume string // --resume flag (existing session ID to resume)
}

// StartHermesSession creates a tmux session and starts a Hermes agent inside it.
//
// The flow is the same as StartClaudeSession but for the hermes CLI:
//
//	tmux new-session -d -s <name> -c <workDir>
//	tmux send-keys -t <name> "cd <workDir> && hermes chat [options]" C-m
//
// Session ID capture uses `hermes sessions export` to find the newly created
// session after startup, rather than parsing terminal output.
//
// Parameters:
//   - name: desired tmux session name (will be sanitized)
//   - workDir: working directory for the session
//   - opts: optional parameters (model, resume session)
//
// Returns the created Session. Session ID capture is asynchronous — the
// mapping is saved in the background when capture succeeds.
func (m *Manager) StartHermesSession(name, workDir string, opts HermesOptions) (*Session, error) {
	absDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve path %q: %w", workDir, err)
	}

	// Build the hermes chat command
	hermesCmd := buildHermesCommand(absDir, opts)

	return m.launch(launchConfig{
		name:      name,
		workDir:   workDir,
		agentType: "hermes",
		command:   hermesCmd,
		preLaunch: nil, // Hermes needs no pre-launch configuration
		captureSessionID: func(tmuxName string, maxWait time.Duration) (string, error) {
			return captureHermesSessionID(maxWait)
		},
	})
}

// buildHermesCommand builds the full shell command to start hermes in tmux.
func buildHermesCommand(workDir string, opts HermesOptions) string {
	var parts []string
	parts = append(parts, "cd", workDir, "&&", "hermes", "chat")

	if opts.Model != "" {
		parts = append(parts, "--model", opts.Model)
	}
	if opts.Resume != "" {
		parts = append(parts, "--resume", opts.Resume)
	}

	return strings.Join(parts, " ")
}

// ── Hermes session ID capture ──────────────────────────────────────

// exportEntry mirrors one JSONL line from `hermes sessions export`.
type exportEntry struct {
	ID         string  `json:"id"`
	Source     string  `json:"source"`
	StartedAt  float64 `json:"started_at"`
	LastActive float64 `json:"last_active"`
}

// captureHermesSessionID runs `hermes sessions export` and finds the most
// recently started CLI session whose started_at is within the capture window.
func captureHermesSessionID(maxWait time.Duration) (string, error) {
	deadline := time.Now().Add(maxWait)
	var lastErr error

	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second) // Give hermes time to initialize

		sessions, err := fetchRecentCLISessions()
		if err != nil {
			lastErr = err
			plogger.Debugf("captureHermesSessionID: fetch error: %v (will retry)", err)
			continue
		}

		if len(sessions) == 0 {
			plogger.Debugf("captureHermesSessionID: no CLI sessions found (will retry)")
			continue
		}

		// The most recent session should be the one we just started.
		// Verify it was started within the last 60 seconds.
		newest := sessions[0]
		startedAt := time.Unix(int64(newest.StartedAt), 0)
		if time.Since(startedAt) < 60*time.Second {
			plogger.Debugf("captureHermesSessionID: found session %s started at %s (%.0fs ago)",
				newest.ID, startedAt.Format(time.TimeOnly), time.Since(startedAt).Seconds())
			return newest.ID, nil
		}

		plogger.Debugf("captureHermesSessionID: newest session %s started too long ago (%.0fs), will retry",
			newest.ID, time.Since(startedAt).Seconds())
	}

	if lastErr != nil {
		return "", fmt.Errorf("failed to capture hermes session ID: %w", lastErr)
	}
	return "", fmt.Errorf("timeout capturing hermes session ID after %.0fs", maxWait.Seconds())
}

// fetchRecentCLISessions runs `hermes sessions export` to a temp file and
// returns CLI sessions sorted by started_at descending.
func fetchRecentCLISessions() ([]exportEntry, error) {
	hd, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find home directory: %w", err)
	}

	tmpFile := filepath.Join(hd, ".hermes", ".pflow_capture_export.jsonl")
	os.Remove(tmpFile) // Remove stale file if exists

	cmd := exec.Command("hermes", "sessions", "export", tmpFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("hermes sessions export failed: %w\n%s", err, string(out))
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("open export file: %w", err)
	}
	defer f.Close()

	var sessions []exportEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)

	for scanner.Scan() {
		var entry exportEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Source == "cli" {
			sessions = append(sessions, entry)
		}
	}

	os.Remove(tmpFile)

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read export file: %w", err)
	}

	// Sort by started_at descending (most recent first)
	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[j].StartedAt > sessions[i].StartedAt {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	return sessions, nil
}
