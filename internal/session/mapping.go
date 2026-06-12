// Package session provides tmux + ttyd session management for pflow.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Mapping records the association between a pflow-managed tmux session
// and a Claude Code session (via the 8-char session ID prefix shown in
// Claude's status line).
type Mapping struct {
	TmuxName     string    `json:"tmux_name"`
	WorkDir      string    `json:"work_dir"`
	ClaudePrefix string    `json:"claude_prefix"` // first 8 chars of Claude session ID
	CreatedAt    time.Time `json:"created_at"`
}

// mappingStore is the persisted state file.
type mappingStore struct {
	Version  int       `json:"version"`
	Mappings []Mapping `json:"mappings"`
}

// mappingManager handles persistence of tmux↔Claude session mappings.
type mappingManager struct {
	mu   sync.Mutex
	path string
}

// newMappingManager returns a mappingManager backed by ~/.pflow/mappings.json.
func newMappingManager() (*mappingManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find home directory: %w", err)
	}
	pflowDir := filepath.Join(home, ".pflow")
	if err := os.MkdirAll(pflowDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create %s: %w", pflowDir, err)
	}
	return &mappingManager{
		path: filepath.Join(pflowDir, "mappings.json"),
	}, nil
}

// load reads all mappings from disk. Returns empty slice if file doesn't exist.
func (mm *mappingManager) load() (*mappingStore, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	store := &mappingStore{Version: 1}
	data, err := os.ReadFile(mm.path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, store); err != nil {
		return nil, fmt.Errorf("corrupt mappings file %s: %w", mm.path, err)
	}
	if store.Version == 0 {
		store.Version = 1
	}
	return store, nil
}

// save writes the store to disk atomically.
func (mm *mappingManager) save(store *mappingStore) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := mm.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, mm.path)
}

// addMapping appends a new mapping and persists.
func (mm *mappingManager) addMapping(m Mapping) error {
	store, err := mm.load()
	if err != nil {
		return err
	}

	// Remove any existing mapping with the same tmux name (upsert).
	filtered := make([]Mapping, 0, len(store.Mappings))
	for _, existing := range store.Mappings {
		if existing.TmuxName != m.TmuxName {
			filtered = append(filtered, existing)
		}
	}
	filtered = append(filtered, m)
	store.Mappings = filtered

	return mm.save(store)
}

// findByTmuxName looks up a mapping by tmux session name.
func (mm *mappingManager) findByTmuxName(tmuxName string) (*Mapping, error) {
	store, err := mm.load()
	if err != nil {
		return nil, err
	}
	for i := range store.Mappings {
		if store.Mappings[i].TmuxName == tmuxName {
			return &store.Mappings[i], nil
		}
	}
	return nil, nil
}

// findByClaudePrefix looks up a mapping by Claude session ID prefix (8 chars).
func (mm *mappingManager) findByClaudePrefix(prefix string) ([]Mapping, error) {
	store, err := mm.load()
	if err != nil {
		return nil, err
	}
	var matches []Mapping
	for _, m := range store.Mappings {
		if m.ClaudePrefix == prefix {
			matches = append(matches, m)
		}
	}
	return matches, nil
}

// findByWorkDir returns all mappings for a given working directory.
func (mm *mappingManager) findByWorkDir(workDir string) ([]Mapping, error) {
	store, err := mm.load()
	if err != nil {
		return nil, err
	}
	var matches []Mapping
	for _, m := range store.Mappings {
		if m.WorkDir == workDir {
			matches = append(matches, m)
		}
	}
	return matches, nil
}

// removeByTmuxName deletes a mapping by tmux session name.
func (mm *mappingManager) removeByTmuxName(tmuxName string) error {
	store, err := mm.load()
	if err != nil {
		return err
	}
	filtered := make([]Mapping, 0, len(store.Mappings))
	for _, m := range store.Mappings {
		if m.TmuxName != tmuxName {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) == len(store.Mappings) {
		return fmt.Errorf("mapping %q not found", tmuxName)
	}
	store.Mappings = filtered
	return mm.save(store)
}

// cleanStale removes mappings whose tmux sessions no longer exist.
// Returns the count of removed entries.
func (mm *mappingManager) cleanStale() (int, error) {
	store, err := mm.load()
	if err != nil {
		return 0, err
	}
	var alive []Mapping
	removed := 0
	for _, m := range store.Mappings {
		if tmuxSessionExists(m.TmuxName) {
			alive = append(alive, m)
		} else {
			removed++
		}
	}
	if removed == 0 {
		return 0, nil
	}
	store.Mappings = alive
	return removed, mm.save(store)
}
