package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchRootFromList(t *testing.T) {
	roots := []Root{
		{Path: "/home/user/code/pflow", Priority: "primary", Slot: SlotPrimary},
		{Path: "/home/user/code/hermes", Priority: "secondary", Slot: SlotSecondary1},
		{Path: "/home/user", Priority: "normal"},
	}

	tests := []struct {
		name string
		cwd  string
		want string // expected matched path, empty = nil
	}{
		{"exact match", "/home/user/code/pflow", "/home/user/code/pflow"},
		{"subdirectory match", "/home/user/code/pflow/internal/api", "/home/user/code/pflow"},
		{"deeper match wins", "/home/user/code/hermes/src", "/home/user/code/hermes"},
		{"shorter match", "/home/user/projects", "/home/user"},
		{"no match", "/other/path", ""},
		{"empty cwd", "", ""},
		{"root cwd", "/", ""},
		{"question mark", "?", ""},
		{"trailing slash", "/home/user/code/pflow/", "/home/user/code/pflow"},
		{"no partial component match", "/home/userx", ""},
		{"subdir should not match partial", "/home/user/code/pflow-ext", "/home/user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchRootFromList(roots, tt.cwd)
			if tt.want == "" {
				if got != nil {
					t.Errorf("MatchRootFromList(%q) = %v, want nil", tt.cwd, got)
				}
			} else {
				if got == nil {
					t.Errorf("MatchRootFromList(%q) = nil, want %q", tt.cwd, tt.want)
				} else if got.Path != tt.want {
					t.Errorf("MatchRootFromList(%q) = %q, want %q", tt.cwd, got.Path, tt.want)
				}
			}
		})
	}
}

// newTestManager creates a Manager that uses a temp file for testing.
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	tmpDir := t.TempDir()
	return &Manager{
		filePath: filepath.Join(tmpDir, "project_roots.json"),
	}
}

func TestSetSlot_NewAssignment(t *testing.T) {
	m := newTestManager(t)

	// Assign a project to secondary_1
	err := m.SetSlot("/home/user/code/projectA", SlotSecondary1)
	if err != nil {
		t.Fatalf("SetSlot failed: %v", err)
	}

	rf, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if rf.Slots[SlotSecondary1] != "/home/user/code/projectA" {
		t.Errorf("slot secondary_1 = %q, want /home/user/code/projectA", rf.Slots[SlotSecondary1])
	}
	if len(rf.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(rf.Roots))
	}
	if rf.Roots[0].Priority != PrioritySecondary {
		t.Errorf("root priority = %q, want secondary", rf.Roots[0].Priority)
	}
	if rf.Roots[0].Slot != SlotSecondary1 {
		t.Errorf("root slot = %q, want secondary_1", rf.Roots[0].Slot)
	}
}

func TestSetSlot_ReplaceExistingSlot(t *testing.T) {
	m := newTestManager(t)

	// Assign projectA to secondary_1
	if err := m.SetSlot("/home/user/code/projectA", SlotSecondary1); err != nil {
		t.Fatalf("SetSlot 1 failed: %v", err)
	}

	// Assign projectB to secondary_1 — should replace projectA
	if err := m.SetSlot("/home/user/code/projectB", SlotSecondary1); err != nil {
		t.Fatalf("SetSlot 2 (replace) failed: %v", err)
	}

	rf, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Slot should now point to projectB
	if rf.Slots[SlotSecondary1] != "/home/user/code/projectB" {
		t.Errorf("slot secondary_1 = %q, want /home/user/code/projectB", rf.Slots[SlotSecondary1])
	}

	// projectA should be demoted to normal
	foundA := false
	for _, r := range rf.Roots {
		if r.Path == "/home/user/code/projectA" {
			foundA = true
			if r.Priority != PriorityNormal {
				t.Errorf("projectA priority = %q, want normal", r.Priority)
			}
			if r.Slot != "" {
				t.Errorf("projectA slot = %q, want empty", r.Slot)
			}
		}
	}
	if !foundA {
		t.Error("projectA not found in roots after demotion")
	}

	// projectB should be secondary with slot secondary_1
	foundB := false
	for _, r := range rf.Roots {
		if r.Path == "/home/user/code/projectB" {
			foundB = true
			if r.Priority != PrioritySecondary {
				t.Errorf("projectB priority = %q, want secondary", r.Priority)
			}
			if r.Slot != SlotSecondary1 {
				t.Errorf("projectB slot = %q, want secondary_1", r.Slot)
			}
		}
	}
	if !foundB {
		t.Error("projectB not found in roots")
	}
}

func TestSetSlot_TwoSecondarySlots(t *testing.T) {
	m := newTestManager(t)

	// Assign projectA to secondary_1, projectB to secondary_2
	if err := m.SetSlot("/home/user/code/projectA", SlotSecondary1); err != nil {
		t.Fatalf("SetSlot secondary_1 failed: %v", err)
	}
	if err := m.SetSlot("/home/user/code/projectB", SlotSecondary2); err != nil {
		t.Fatalf("SetSlot secondary_2 failed: %v", err)
	}

	rf, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if rf.Slots[SlotSecondary1] != "/home/user/code/projectA" {
		t.Errorf("secondary_1 = %q, want projectA", rf.Slots[SlotSecondary1])
	}
	if rf.Slots[SlotSecondary2] != "/home/user/code/projectB" {
		t.Errorf("secondary_2 = %q, want projectB", rf.Slots[SlotSecondary2])
	}

	// Replace secondary_2 with projectC
	if err := m.SetSlot("/home/user/code/projectC", SlotSecondary2); err != nil {
		t.Fatalf("SetSlot replace secondary_2 failed: %v", err)
	}

	rf, err = m.Load()
	if err != nil {
		t.Fatalf("Load after replace failed: %v", err)
	}

	// secondary_1 should be unchanged
	if rf.Slots[SlotSecondary1] != "/home/user/code/projectA" {
		t.Errorf("secondary_1 = %q, want projectA (unchanged)", rf.Slots[SlotSecondary1])
	}
	// secondary_2 should now be projectC
	if rf.Slots[SlotSecondary2] != "/home/user/code/projectC" {
		t.Errorf("secondary_2 = %q, want projectC", rf.Slots[SlotSecondary2])
	}
	// projectB should be demoted to normal
	foundB := false
	for _, r := range rf.Roots {
		if r.Path == "/home/user/code/projectB" {
			foundB = true
			if r.Priority != PriorityNormal {
				t.Errorf("projectB priority = %q, want normal (demoted)", r.Priority)
			}
		}
	}
	if !foundB {
		t.Error("projectB not found after demotion")
	}
}

func TestSetSlot_CrossSlotMove(t *testing.T) {
	m := newTestManager(t)

	// Assign projectA to secondary_1, projectB to secondary_2
	if err := m.SetSlot("/home/user/code/projectA", SlotSecondary1); err != nil {
		t.Fatalf("SetSlot secondary_1 failed: %v", err)
	}
	if err := m.SetSlot("/home/user/code/projectB", SlotSecondary2); err != nil {
		t.Fatalf("SetSlot secondary_2 failed: %v", err)
	}

	// Move projectA from secondary_1 to secondary_2
	if err := m.SetSlot("/home/user/code/projectA", SlotSecondary2); err != nil {
		t.Fatalf("SetSlot move to secondary_2 failed: %v", err)
	}

	rf, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// secondary_1 should be empty (projectB demoted when projectA moved to secondary_2)
	if _, ok := rf.Slots[SlotSecondary1]; ok {
		t.Errorf("secondary_1 should be empty, got %q", rf.Slots[SlotSecondary1])
	}
	// secondary_2 should be projectA
	if rf.Slots[SlotSecondary2] != "/home/user/code/projectA" {
		t.Errorf("secondary_2 = %q, want projectA", rf.Slots[SlotSecondary2])
	}
}

func TestSetSlot_PrimarySlot(t *testing.T) {
	m := newTestManager(t)

	// Assign to primary
	if err := m.SetSlot("/home/user/code/mainProject", SlotPrimary); err != nil {
		t.Fatalf("SetSlot primary failed: %v", err)
	}

	// Replace primary
	if err := m.SetSlot("/home/user/code/newMain", SlotPrimary); err != nil {
		t.Fatalf("SetSlot replace primary failed: %v", err)
	}

	rf, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if rf.Slots[SlotPrimary] != "/home/user/code/newMain" {
		t.Errorf("primary = %q, want /home/user/code/newMain", rf.Slots[SlotPrimary])
	}

	// Old primary should be demoted to normal
	found := false
	for _, r := range rf.Roots {
		if r.Path == "/home/user/code/mainProject" {
			found = true
			if r.Priority != PriorityNormal {
				t.Errorf("old primary priority = %q, want normal", r.Priority)
			}
		}
	}
	if !found {
		t.Error("old primary not found after demotion")
	}
}

func TestClearSlot(t *testing.T) {
	m := newTestManager(t)

	// Assign and then clear
	if err := m.SetSlot("/home/user/code/projectA", SlotSecondary1); err != nil {
		t.Fatalf("SetSlot failed: %v", err)
	}
	if err := m.ClearSlot(SlotSecondary1); err != nil {
		t.Fatalf("ClearSlot failed: %v", err)
	}

	rf, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if _, ok := rf.Slots[SlotSecondary1]; ok {
		t.Errorf("secondary_1 should be cleared, got %q", rf.Slots[SlotSecondary1])
	}

	// Path should still exist as normal
	found := false
	for _, r := range rf.Roots {
		if r.Path == "/home/user/code/projectA" {
			found = true
			if r.Priority != PriorityNormal {
				t.Errorf("priority = %q, want normal", r.Priority)
			}
			if r.Slot != "" {
				t.Errorf("slot = %q, want empty", r.Slot)
			}
		}
	}
	if !found {
		t.Error("path should still exist as normal root after ClearSlot")
	}
}

func TestClearSlot_EmptySlot(t *testing.T) {
	m := newTestManager(t)

	// Clearing an empty slot should not error
	if err := m.ClearSlot(SlotSecondary1); err != nil {
		t.Fatalf("ClearSlot on empty slot should not error: %v", err)
	}
}

func TestSetSlot_InvalidSlot(t *testing.T) {
	m := newTestManager(t)

	err := m.SetSlot("/home/user/code/projectA", SlotID("invalid_slot"))
	if err == nil {
		t.Error("expected error for invalid slot")
	}
}

func TestSetSlot_RootRejected(t *testing.T) {
	m := newTestManager(t)

	err := m.SetSlot("/", SlotPrimary)
	if err == nil {
		t.Error("expected error for root directory")
	}
}

func TestSetPriority_LegacySecondaryRouting(t *testing.T) {
	m := newTestManager(t)

	// Legacy SetPriority("secondary") should route to SetSlot
	err := m.SetPriority("/home/user/code/projectA", PrioritySecondary)
	if err != nil {
		t.Fatalf("SetPriority secondary failed: %v", err)
	}

	rf, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should have filled secondary_1
	if rf.Slots[SlotSecondary1] != "/home/user/code/projectA" {
		t.Errorf("secondary_1 = %q, want /home/user/code/projectA", rf.Slots[SlotSecondary1])
	}

	// Second legacy call should fill secondary_2
	err = m.SetPriority("/home/user/code/projectB", PrioritySecondary)
	if err != nil {
		t.Fatalf("SetPriority second secondary failed: %v", err)
	}

	rf, err = m.Load()
	if err != nil {
		t.Fatalf("Load after second call failed: %v", err)
	}

	if rf.Slots[SlotSecondary2] != "/home/user/code/projectB" {
		t.Errorf("secondary_2 = %q, want /home/user/code/projectB", rf.Slots[SlotSecondary2])
	}

	// Third legacy call: both slots occupied, falls through to secondary_2 (replaces it)
	err = m.SetPriority("/home/user/code/projectC", PrioritySecondary)
	if err != nil {
		t.Fatalf("SetPriority third secondary failed: %v", err)
	}

	rf, err = m.Load()
	if err != nil {
		t.Fatalf("Load after third call failed: %v", err)
	}

	// secondary_1 should be unchanged (still projectA)
	if rf.Slots[SlotSecondary1] != "/home/user/code/projectA" {
		t.Errorf("secondary_1 = %q, want /home/user/code/projectA (unchanged)", rf.Slots[SlotSecondary1])
	}
	// secondary_2 replaced with projectC
	if rf.Slots[SlotSecondary2] != "/home/user/code/projectC" {
		t.Errorf("secondary_2 = %q, want /home/user/code/projectC", rf.Slots[SlotSecondary2])
	}
}

func TestSetPriority_Normal(t *testing.T) {
	m := newTestManager(t)

	err := m.SetPriority("/home/user/code/normalProject", PriorityNormal)
	if err != nil {
		t.Fatalf("SetPriority normal failed: %v", err)
	}

	rf, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	found := false
	for _, r := range rf.Roots {
		if r.Path == "/home/user/code/normalProject" {
			found = true
			if r.Priority != PriorityNormal {
				t.Errorf("priority = %q, want normal", r.Priority)
			}
		}
	}
	if !found {
		t.Error("normal project not found")
	}
}

func TestRemoveRoot_AlsoClearsSlot(t *testing.T) {
	m := newTestManager(t)

	if err := m.SetSlot("/home/user/code/projectA", SlotSecondary1); err != nil {
		t.Fatalf("SetSlot failed: %v", err)
	}
	if err := m.RemoveRoot("/home/user/code/projectA"); err != nil {
		t.Fatalf("RemoveRoot failed: %v", err)
	}

	rf, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if _, ok := rf.Slots[SlotSecondary1]; ok {
		t.Errorf("slot secondary_1 should be cleared when root is removed, got %q", rf.Slots[SlotSecondary1])
	}
	if len(rf.Roots) != 0 {
		t.Errorf("roots should be empty, got %d", len(rf.Roots))
	}
}

func TestValidate_SlotConsistency(t *testing.T) {
	m := newTestManager(t)

	// Valid setup
	if err := m.SetSlot("/home/user/code/main", SlotPrimary); err != nil {
		t.Fatalf("SetSlot failed: %v", err)
	}
	if err := m.SetSlot("/home/user/code/side1", SlotSecondary1); err != nil {
		t.Fatalf("SetSlot failed: %v", err)
	}

	if err := m.Validate(); err != nil {
		t.Errorf("Validate should pass for consistent data: %v", err)
	}
}

func TestMigrateV1ToV2(t *testing.T) {
	// Simulate a v1 RootsFile
	rf := RootsFile{
		Version: 1,
		Roots: []Root{
			{Path: "/home/user/code/main", Priority: PriorityPrimary},
			{Path: "/home/user/code/side1", Priority: PrioritySecondary},
			{Path: "/home/user/code/side2", Priority: PrioritySecondary},
			{Path: "/home/user/code/normal1", Priority: PriorityNormal},
		},
	}

	result := migrateV1ToV2(rf)

	if result.Version != 2 {
		t.Errorf("version = %d, want 2", result.Version)
	}
	if result.Slots[SlotPrimary] != "/home/user/code/main" {
		t.Errorf("primary slot = %q, want /home/user/code/main", result.Slots[SlotPrimary])
	}
	if result.Slots[SlotSecondary1] != "/home/user/code/side1" {
		t.Errorf("secondary_1 slot = %q, want /home/user/code/side1", result.Slots[SlotSecondary1])
	}
	if result.Slots[SlotSecondary2] != "/home/user/code/side2" {
		t.Errorf("secondary_2 slot = %q, want /home/user/code/side2", result.Slots[SlotSecondary2])
	}

	// Check Slot fields on Roots
	for _, r := range result.Roots {
		switch r.Path {
		case "/home/user/code/main":
			if r.Slot != SlotPrimary {
				t.Errorf("main slot = %q, want primary", r.Slot)
			}
		case "/home/user/code/side1":
			if r.Slot != SlotSecondary1 {
				t.Errorf("side1 slot = %q, want secondary_1", r.Slot)
			}
		case "/home/user/code/side2":
			if r.Slot != SlotSecondary2 {
				t.Errorf("side2 slot = %q, want secondary_2", r.Slot)
			}
		case "/home/user/code/normal1":
			if r.Slot != "" {
				t.Errorf("normal slot = %q, want empty", r.Slot)
			}
		}
	}
}

func TestMigrateV1ToV2_OnlyOneSecondary(t *testing.T) {
	rf := RootsFile{
		Version: 1,
		Roots: []Root{
			{Path: "/home/user/code/main", Priority: PriorityPrimary},
			{Path: "/home/user/code/side1", Priority: PrioritySecondary},
		},
	}

	result := migrateV1ToV2(rf)

	if result.Slots[SlotSecondary1] != "/home/user/code/side1" {
		t.Errorf("secondary_1 = %q, want /home/user/code/side1", result.Slots[SlotSecondary1])
	}
	if _, ok := result.Slots[SlotSecondary2]; ok {
		t.Errorf("secondary_2 should be empty, got %q", result.Slots[SlotSecondary2])
	}
}

func TestMigrateV1ToV2_EmptyFile(t *testing.T) {
	m := newTestManager(t)

	// Empty file (no file exists) should return empty v2 structure
	rf, err := m.Load()
	if err != nil {
		t.Fatalf("Load empty file failed: %v", err)
	}

	if rf.Version != 2 {
		t.Errorf("version = %d, want 2", rf.Version)
	}
	if len(rf.Slots) != 0 {
		t.Errorf("slots should be empty, got %d entries", len(rf.Slots))
	}
	if len(rf.Roots) != 0 {
		t.Errorf("roots should be empty, got %d entries", len(rf.Roots))
	}
}

// Test actual file-based v1 migration
func TestLoad_MigrateV1File(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "project_roots.json")

	// Write a v1 file
	v1Content := `{"version":1,"roots":[{"path":"/a","priority":"primary"},{"path":"/b","priority":"secondary"},{"path":"/c","priority":"secondary"}]}`
	if err := os.WriteFile(filePath, []byte(v1Content), 0644); err != nil {
		t.Fatalf("write v1 file: %v", err)
	}

	m := &Manager{filePath: filePath}
	rf, err := m.Load()
	if err != nil {
		t.Fatalf("Load v1 file failed: %v", err)
	}

	if rf.Version != 2 {
		t.Errorf("version = %d, want 2", rf.Version)
	}
	if rf.Slots[SlotPrimary] != "/a" {
		t.Errorf("primary = %q, want /a", rf.Slots[SlotPrimary])
	}
	if rf.Slots[SlotSecondary1] != "/b" {
		t.Errorf("secondary_1 = %q, want /b", rf.Slots[SlotSecondary1])
	}
	if rf.Slots[SlotSecondary2] != "/c" {
		t.Errorf("secondary_2 = %q, want /c", rf.Slots[SlotSecondary2])
	}
}
