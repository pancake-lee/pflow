package project

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SetSlot assigns a path to a specific slot.
// If another path already occupies the slot, it is demoted to normal.
// If the path already occupies a different slot, that slot is cleared first.
// If the path is not yet a root, it is added.
func (m *Manager) SetSlot(path string, slot SlotID) error {
	// Guard: root directory protection
	if path == "/" {
		return fmt.Errorf("cannot mark root directory '/' as a project root")
	}
	path = filepath.Clean(path)

	// Validate slot
	if !isValidSlot(slot) {
		return fmt.Errorf("invalid slot %q: must be primary, secondary_1, or secondary_2", slot)
	}

	rf, err := m.Load()
	if err != nil {
		return err
	}

	// Determine the priority for this slot
	priority := slotToPriority(slot)

	// Step 1: If path currently occupies a different slot, clear that slot
	for slotID, curPath := range rf.Slots {
		if curPath == path && slotID != slot {
			delete(rf.Slots, slotID)
			break
		}
	}

	// Step 2: If another path occupies the target slot, demote it to normal
	if existingPath, occupied := rf.Slots[slot]; occupied && existingPath != path {
		delete(rf.Slots, slot)
		for i := range rf.Roots {
			if rf.Roots[i].Path == existingPath {
				rf.Roots[i].Priority = PriorityNormal
				rf.Roots[i].Slot = ""
				break
			}
		}
	}

	// Step 3: Assign the slot
	rf.Slots[slot] = path

	// Step 4: Update or add the root entry
	found := false
	for i := range rf.Roots {
		if rf.Roots[i].Path == path {
			rf.Roots[i].Priority = priority
			rf.Roots[i].Slot = slot
			found = true
			break
		}
	}
	if !found {
		rf.Roots = append(rf.Roots, Root{
			Path:     path,
			Priority: priority,
			Slot:     slot,
		})
	}

	return m.Save(rf)
}

// ClearSlot removes a path from its slot, demoting it to normal.
// The path remains as a project root (for matching purposes).
func (m *Manager) ClearSlot(slot SlotID) error {
	rf, err := m.Load()
	if err != nil {
		return err
	}

	path, ok := rf.Slots[slot]
	if !ok {
		return nil // slot already empty
	}

	delete(rf.Slots, slot)

	for i := range rf.Roots {
		if rf.Roots[i].Path == path {
			rf.Roots[i].Priority = PriorityNormal
			rf.Roots[i].Slot = ""
			break
		}
	}

	return m.Save(rf)
}

// SetPriority sets the priority of a root path to "normal" only.
// For slot assignments (primary/secondary), use SetSlot instead.
func (m *Manager) SetPriority(path string, priority Priority) error {
	// Guard: root directory protection
	if path == "/" {
		return fmt.Errorf("cannot mark root directory '/' as a project root")
	}
	path = filepath.Clean(path)

	// Validate priority value
	switch priority {
	case PriorityPrimary, PrioritySecondary, PriorityNormal:
		// valid
	default:
		return fmt.Errorf("invalid priority %q: must be primary, secondary, or normal", priority)
	}

	rf, err := m.Load()
	if err != nil {
		return err
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

	switch priority {
	case PriorityPrimary:
		// Route to SetSlot
		return m.SetSlot(path, SlotPrimary)
	case PrioritySecondary:
		// Route to SetSlot — use first available secondary slot
		if _, occupied := rf.Slots[SlotSecondary1]; !occupied {
			return m.SetSlot(path, SlotSecondary1)
		}
		return m.SetSlot(path, SlotSecondary2)
	case PriorityNormal:
		// Demote: clear any slot this path occupies
		for slotID, slotPath := range rf.Slots {
			if slotPath == path {
				delete(rf.Slots, slotID)
				break
			}
		}

		if existingIdx >= 0 {
			rf.Roots[existingIdx].Priority = PriorityNormal
			rf.Roots[existingIdx].Slot = ""
		} else {
			rf.Roots = append(rf.Roots, Root{Path: path, Priority: PriorityNormal})
		}
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
			// Remove slot mapping if present
			if r.Slot != "" {
				delete(rf.Slots, r.Slot)
			}
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

	seen := make(map[string]bool)
	seenPriorities := make(map[SlotID]string) // slot -> path (detect duplicate slot assignments)

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
		case PriorityPrimary, PrioritySecondary, PriorityNormal:
			// valid
		default:
			return fmt.Errorf("invalid priority %q for root %s", r.Priority, r.Path)
		}

		// Slot consistency: if root has a slot, the slots map must agree
		if r.Slot != "" {
			if existing, ok := seenPriorities[r.Slot]; ok {
				return fmt.Errorf("slot %s assigned to multiple paths: %s and %s", r.Slot, existing, r.Path)
			}
			seenPriorities[r.Slot] = r.Path
		}
	}

	// Verify slots map consistency
	for slotID, path := range rf.Slots {
		if !seen[path] {
			return fmt.Errorf("slot %s points to path %s which is not in roots list", slotID, path)
		}
		// Verify the Root has a matching slot field
		found := false
		for _, r := range rf.Roots {
			if r.Path == path && r.Slot == slotID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("slot %s → %s has no matching root entry with slot field", slotID, path)
		}
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

// ── Helper functions ──────────────────────────────────────────────

// isValidSlot returns true if the slot ID is a known slot.
func isValidSlot(slot SlotID) bool {
	for _, s := range ValidSlotIDs() {
		if slot == s {
			return true
		}
	}
	return false
}

// slotToPriority returns the Priority for a given slot.
func slotToPriority(slot SlotID) Priority {
	switch slot {
	case SlotPrimary:
		return PriorityPrimary
	case SlotSecondary1, SlotSecondary2:
		return PrioritySecondary
	default:
		return PriorityNormal
	}
}
