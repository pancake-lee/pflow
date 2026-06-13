package project

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SetPriority sets the priority of a root path, handling promotion/demotion rules.
// If a path is not yet a root, it is added. If it already exists, the priority is updated.
// Returns an error if the operation would violate priority limits.
func (m *Manager) SetPriority(path string, priority Priority) error {
	// Guard: root directory protection
	if path == "/" {
		return fmt.Errorf("cannot mark root directory '/' as a project root")
	}
	path = filepath.Clean(path)

	rf, err := m.Load()
	if err != nil {
		return err
	}

	// Validate priority value
	switch priority {
	case PriorityPrimary, PrioritySecondary, PriorityNormal:
		// valid
	default:
		return fmt.Errorf("invalid priority %q: must be primary, secondary, or normal", priority)
	}

	// Find existing entry
	existingIdx := -1
	for i, r := range rf.Roots {
		if r.Path == path {
			existingIdx = i
			break
		}
	}

	if existingIdx >= 0 && rf.Roots[existingIdx].Priority == priority {
		// No change needed
		return nil
	}

	// Count current priorities (excluding the target if it already exists)
	primaryCount := 0
	secondaryCount := 0
	for i, r := range rf.Roots {
		if i == existingIdx {
			continue
		}
		switch r.Priority {
		case PriorityPrimary:
			primaryCount++
		case PrioritySecondary:
			secondaryCount++
		}
	}

	switch priority {
	case PriorityPrimary:
		// Demote existing primary to normal (primary swap)
		if primaryCount >= MaxPrimary {
			for i := range rf.Roots {
				if i != existingIdx && rf.Roots[i].Priority == PriorityPrimary {
					rf.Roots[i].Priority = PriorityNormal
					break
				}
			}
		}
	case PrioritySecondary:
		if secondaryCount >= MaxSecondary {
			return fmt.Errorf("secondary slots full (max %d): demote an existing secondary first", MaxSecondary)
		}
	}

	if existingIdx >= 0 {
		rf.Roots[existingIdx].Priority = priority
	} else {
		rf.Roots = append(rf.Roots, Root{Path: path, Priority: priority})
	}

	return m.Save(rf)
}

// RemoveRoot removes a path from the project roots.
// Sessions previously matched to this root will fall back to a shorter match
// or become unmatched.
func (m *Manager) RemoveRoot(path string) error {
	rf, err := m.Load()
	if err != nil {
		return err
	}

	path = filepath.Clean(path)
	found := false
	filtered := make([]Root, 0, len(rf.Roots))
	for _, r := range rf.Roots {
		if r.Path == path {
			found = true
			continue
		}
		filtered = append(filtered, r)
	}

	if !found {
		return nil // already removed, not an error
	}

	rf.Roots = filtered
	return m.Save(rf)
}

// Validate returns an error if the current roots configuration is invalid.
func (m *Manager) Validate() error {
	rf, err := m.Load()
	if err != nil {
		return err
	}

	primaryCount := 0
	secondaryCount := 0
	seen := make(map[string]bool)

	for _, r := range rf.Roots {
		// Root directory check
		if r.Path == "/" {
			return fmt.Errorf("root directory '/' is not allowed as a project root")
		}

		// Duplicate check
		if seen[r.Path] {
			return fmt.Errorf("duplicate root path: %s", r.Path)
		}
		seen[r.Path] = true

		switch r.Priority {
		case PriorityPrimary:
			primaryCount++
		case PrioritySecondary:
			secondaryCount++
		case PriorityNormal:
			// unlimited
		default:
			return fmt.Errorf("invalid priority %q for root %s", r.Priority, r.Path)
		}
	}

	if primaryCount > MaxPrimary {
		return fmt.Errorf("too many primary roots: %d (max %d)", primaryCount, MaxPrimary)
	}
	if secondaryCount > MaxSecondary {
		return fmt.Errorf("too many secondary roots: %d (max %d)", secondaryCount, MaxSecondary)
	}

	return nil
}

// MatchRoot finds the best matching project root for a session working directory.
// Uses longest prefix match: if both /a and /a/b are roots and the session cwd is
// /a/b/c, /a/b wins (more specific). Returns nil if no root matches.
func (m *Manager) MatchRoot(sessionCwd string) *Root {
	rf, err := m.Load()
	if err != nil {
		return nil
	}

	return MatchRootFromList(rf.Roots, sessionCwd)
}

// MatchRootFromList performs longest prefix matching against a root slice.
// Exported for use in response construction (avoids reloading).
func MatchRootFromList(roots []Root, sessionCwd string) *Root {
	if sessionCwd == "" || sessionCwd == "?" || sessionCwd == "/" {
		return nil
	}

	cwd := filepath.Clean(sessionCwd)
	var bestMatch *Root
	bestLen := 0

	for i := range roots {
		r := &roots[i]
		rootPath := filepath.Clean(r.Path)

		// Check if cwd starts with rootPath (prefix match).
		// Must match on path component boundary: /a/b matches /a/b/c but not /a/bc
		if cwd == rootPath {
			// Exact match — always best
			if len(rootPath) > bestLen {
				bestMatch = r
				bestLen = len(rootPath)
			}
			continue
		}

		if strings.HasPrefix(cwd, rootPath+string(filepath.Separator)) {
			if len(rootPath) > bestLen {
				bestMatch = r
				bestLen = len(rootPath)
			}
		}
	}

	return bestMatch
}
