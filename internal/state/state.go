// Package state manages the pflow runtime state file (~/.pflow/state.json).
//
// This file stores per-date, cross-session state such as daily boot
// completion status and the user's goal for the day. It is distinct from
// project_roots.json (project configuration) and mappings.json (session
// mappings).
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	plogger "github.com/pancake-lee/pgo/pkg/plogger"
)

// DailyBootEntry holds the daily boot state for a specific date.
type DailyBootEntry struct {
	Completed bool   `json:"completed"`
	Goal      string `json:"goal"`
}

// StateFile is the on-disk representation of state.json.
type StateFile struct {
	DailyBoot map[string]DailyBootEntry `json:"daily_boot"`
}

// Manager provides thread-safe read/write access to the pflow state file.
type Manager struct {
	mu       sync.RWMutex
	filePath string
}

// NewManager creates a new Manager with the default storage path.
func NewManager() *Manager {
	return &Manager{
		filePath: filepath.Join(pflowDir(), "state.json"),
	}
}

// pflowDir returns the pflow config directory (~/.pflow).
func pflowDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".pflow")
}

// Load reads the state file from disk. Returns an empty StateFile if the
// file doesn't exist.
func (m *Manager) Load() (*StateFile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &StateFile{DailyBoot: make(map[string]DailyBootEntry)}, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	var sf StateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}

	if sf.DailyBoot == nil {
		sf.DailyBoot = make(map[string]DailyBootEntry)
	}
	return &sf, nil
}

// Save atomically writes the state file to disk.
func (m *Manager) Save(sf *StateFile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sf.DailyBoot == nil {
		sf.DailyBoot = make(map[string]DailyBootEntry)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0755); err != nil {
		return fmt.Errorf("create pflow config dir: %w", err)
	}

	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	// Atomic write: tmp file + rename
	tmpPath := m.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write state tmp: %w", err)
	}
	if err := os.Rename(tmpPath, m.filePath); err != nil {
		os.Remove(tmpPath) // best-effort cleanup
		return fmt.Errorf("rename state: %w", err)
	}

	plogger.Debugf("state: saved to %s", m.filePath)
	return nil
}

// GetTodayBoot returns the daily boot entry for today, and whether today's
// boot has been completed. If no entry exists for today, returns an empty
// entry with completed=false.
func (m *Manager) GetTodayBoot() (*DailyBootEntry, error) {
	sf, err := m.Load()
	if err != nil {
		return nil, err
	}

	today := time.Now().Format("2006-01-02")
	entry, exists := sf.DailyBoot[today]
	if !exists {
		return &DailyBootEntry{}, nil
	}
	return &entry, nil
}

// CompleteTodayBoot marks today's daily boot as completed and saves the goal.
// The goal string may be empty if the user skipped without entering one.
func (m *Manager) CompleteTodayBoot(goal string) error {
	sf, err := m.Load()
	if err != nil {
		return err
	}

	today := time.Now().Format("2006-01-02")
	entry := sf.DailyBoot[today]
	entry.Completed = true
	if goal != "" {
		entry.Goal = goal
	}
	sf.DailyBoot[today] = entry

	return m.Save(sf)
}

// UpdateTodayGoal updates only the goal for today's daily boot entry.
// Creates the entry if it doesn't exist (without marking completed).
func (m *Manager) UpdateTodayGoal(goal string) error {
	sf, err := m.Load()
	if err != nil {
		return err
	}

	today := time.Now().Format("2006-01-02")
	entry := sf.DailyBoot[today]
	entry.Goal = goal
	sf.DailyBoot[today] = entry

	return m.Save(sf)
}
