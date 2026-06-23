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

// SlotID identifies a specific priority slot.
type SlotID string

const (
	SlotPrimary    SlotID = "primary"
	SlotSecondary1 SlotID = "secondary_1"
	SlotSecondary2 SlotID = "secondary_2"
)

// ValidSlotIDs returns all valid slot identifiers.
func ValidSlotIDs() []SlotID {
	return []SlotID{SlotPrimary, SlotSecondary1, SlotSecondary2}
}

// Slots maps slot identifiers to project paths.
type Slots map[SlotID]string

// Root represents a single project root with its priority.
type Root struct {
	Path     string   `json:"path"`
	Priority Priority `json:"priority"`
	Slot     SlotID   `json:"slot,omitempty"` // slot assignment; empty for normal/unslotted
}

// RootsFile is the on-disk representation of project_roots.json.
type RootsFile struct {
	Version int    `json:"version"`
	Slots   Slots  `json:"slots,omitempty"` // slot → path mapping (authoritative for slot positioning)
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
// Automatically migrates v1 files to v2 (slot-based) format.
func (m *Manager) Load() (*RootsFile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &RootsFile{Version: 2, Slots: Slots{}, Roots: []Root{}}, nil
		}
		return nil, fmt.Errorf("read project roots: %w", err)
	}

	var rf RootsFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parse project roots: %w", err)
	}

	// Migration from v1 to v2
	if rf.Version < 2 {
		rf = migrateV1ToV2(rf)
	}

	if rf.Slots == nil {
		rf.Slots = Slots{}
	}
	if rf.Roots == nil {
		rf.Roots = []Root{}
	}
	return &rf, nil
}

// migrateV1ToV2 converts a v1 flat list to v2 slot-based format.
func migrateV1ToV2(rf RootsFile) RootsFile {
	rf.Version = 2
	rf.Slots = make(Slots)

	secondaryCount := 0
	for i, r := range rf.Roots {
		switch r.Priority {
		case PriorityPrimary:
			rf.Slots[SlotPrimary] = r.Path
			rf.Roots[i].Slot = SlotPrimary
		case PrioritySecondary:
			secondaryCount++
			slot := SlotSecondary1
			if secondaryCount == 2 {
				slot = SlotSecondary2
			}
			rf.Slots[slot] = r.Path
			rf.Roots[i].Slot = slot
		default:
			// normal — no slot
		}
	}

	return rf
}

// Save atomically writes the project roots to disk.
func (m *Manager) Save(rf *RootsFile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rf.Version < 2 {
		rf.Version = 2
	}
	if rf.Slots == nil {
		rf.Slots = Slots{}
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

	plogger.Debugf("project: saved %d roots, %d slots to %s", len(rf.Roots), len(rf.Slots), m.filePath)
	return nil
}
