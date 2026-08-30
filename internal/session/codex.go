package session

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/pancake-lee/pflow/internal/codex"
	plogger "github.com/pancake-lee/pgo/pkg/plogger"
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
		// command:   "codex --approve-for-me",
		command: "codex --dangerously-bypass-approvals-and-sandbox",
		captureSessionID: func(_ string, maxWait time.Duration) (string, error) {
			_ = maxWait // Codex rollout visibility is controlled by the first user request.
			for _, delay := range []time.Duration{5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second} {
				time.Sleep(delay)
				id, err := codex.FindSessionStartedAfter(absDir, started)
				if err != nil {
					plogger.Warnf("codex mapping: scan failed for workDir=%s: %v", absDir, err)
					continue
				}
				if id != "" {
					return id, nil
				}
			}
			for {
				time.Sleep(60 * time.Second)
				id, err := codex.FindSessionStartedAfter(absDir, started)
				if err != nil {
					plogger.Warnf("codex mapping: scan failed for workDir=%s: %v", absDir, err)
					continue
				}
				if id != "" {
					return id, nil
				}
			}
		},
	})
}
