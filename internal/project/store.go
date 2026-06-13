// Package project manages project root paths and their priorities.
// "Path is Project" — working directories are the natural identity;
// no separate project ID/name entities are introduced.
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	plogger "github.com/pancake-lee/pgo/pkg/plogger"
)

// Priority represents the attention priority of a project root.
type Priority string

const (
	PriorityPrimary   Priority = "primary"
	PrioritySecondary Priority = "secondary"
	PriorityNormal    Priority = "normal"
)

// MaxPrimary is the maximum number of primary project roots.
const MaxPrimary = 1

// MaxSecondary is the maximum number of secondary project roots.
const MaxSecondary = 2

// Root represents a single project root with its priority.
type Root struct {
	Path     string   `json:"path"`
	Priority Priority `json:"priority"`
}

// RootsFile is the on-disk representation of project_roots.json.
type RootsFile struct {
	Version int    `json:"version"`
	Roots   []Root `json:"roots"`
}

// Manager provides read/write access to project roots with validation.
type Manager struct {
	mu       sync.RWMutex
	filePath string
}

// NewManager creates a new Manager with the default storage path.
func NewManager() *Manager {
	return &Manager{
		filePath: filepath.Join(pflowDir(), "project_roots.json"),
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

// Load reads the project roots from disk.
// Returns an empty RootsFile if the file doesn't exist.
func (m *Manager) Load() (*RootsFile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &RootsFile{Version: 1, Roots: []Root{}}, nil
		}
		return nil, fmt.Errorf("read project roots: %w", err)
	}

	var rf RootsFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parse project roots: %w", err)
	}
	if rf.Version < 1 {
		rf.Version = 1
	}
	if rf.Roots == nil {
		rf.Roots = []Root{}
	}
	return &rf, nil
}

// Save atomically writes the project roots to disk.
func (m *Manager) Save(rf *RootsFile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rf.Version < 1 {
		rf.Version = 1
	}
	if rf.Roots == nil {
		rf.Roots = []Root{}
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0755); err != nil {
		return fmt.Errorf("create pflow config dir: %w", err)
	}

	data, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project roots: %w", err)
	}

	// Atomic write: tmp file + rename
	tmpPath := m.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write project roots tmp: %w", err)
	}
	if err := os.Rename(tmpPath, m.filePath); err != nil {
		os.Remove(tmpPath) // best-effort cleanup
		return fmt.Errorf("rename project roots: %w", err)
	}

	plogger.Debugf("project: saved %d roots to %s", len(rf.Roots), m.filePath)
	return nil
}
