package session

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/pancake-lee/pflow/internal/codex"
)

// StartCodexSession creates a tmux session and starts Codex CLI inside it.
// Its rollout record is matched by working directory and launch time.
func (m *Manager) StartCodexSession(name, workDir string) (*Session, error) {
	absDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve path %q: %w", workDir, err)
	}
	started := time.Now().Add(-time.Second)
	return m.launch(launchConfig{
		name:      name,
		agentName: name,
		workDir:   absDir,
		agentType: "codex",
		command:   "codex --approve-for-me",
		captureSessionID: func(_ string, maxWait time.Duration) (string, error) {
			deadline := time.Now().Add(maxWait)
			for time.Now().Before(deadline) {
				id, err := codex.FindSessionStartedAfter(absDir, started)
				if err != nil || id != "" {
					return id, err
				}
				time.Sleep(500 * time.Millisecond)
			}
			return "", nil
		},
	})
}
