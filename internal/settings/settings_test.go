package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadMergesOnlyPresentFields(t *testing.T) {
	testList := []struct {
		name string
		json string
		edit func(*File)
	}{
		{"missing dashboard fields", `{"version":1,"dashboard":{"window":"7d"}}`, func(f *File) { f.Dashboard.Window = "7d" }},
		{"explicit zero and false", `{"version":1,"dashboard":{"max_inactive":0,"refresh_seconds":0,"daily_boot_enabled":false}}`, func(f *File) {
			f.Dashboard.MaxInactive = 0
			f.Dashboard.RefreshSeconds = 0
			f.Dashboard.DailyBootEnabled = false
		}},
		{"partial attention", `{"version":1,"attention":{"mask_strength":0.5}}`, func(f *File) { f.Attention.MaskStrength = 0.5 }},
		{"missing sections", `{"version":1,"future":true}`, func(*File) {}},
	}
	for _, tc := range testList {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			err := os.WriteFile(path, []byte(tc.json), 0600)
			if err != nil {
				t.Fatal(err)
			}
			manager := NewManagerAt(path)
			actual, err := manager.Load()
			if err != nil {
				t.Fatal(err)
			}
			expected := Default()
			tc.edit(&expected)
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("got %+v want %+v", actual, expected)
			}
			err = manager.Save(actual)
			if err != nil {
				t.Fatal(err)
			}
			reloaded, err := NewManagerAt(path).Load()
			expected.DashboardRevision = reloaded.DashboardRevision
			if err != nil || !reflect.DeepEqual(reloaded, expected) {
				t.Fatalf("reload=%+v err=%v", reloaded, err)
			}
		})
	}
}

func TestLoadRejectsPresentInvalidValues(t *testing.T) {
	for _, data := range []string{
		`{"version":1,"dashboard":{"max_inactive":-1}}`,
		`{"version":1,"dashboard":{"refresh_seconds":-1}}`,
		`{"version":1,"attention":{"mask_strength":0}}`,
		`{"version":1,"time_estimate":{"fallback_ratio":-1}}`,
	} {
		path := filepath.Join(t.TempDir(), "settings.json")
		err := os.WriteFile(path, []byte(data), 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = NewManagerAt(path).Load()
		if err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}

func TestManagerDefaultsAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	m := NewManagerAt(path)
	got, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Dashboard.Window != "1d" || got.TimeEstimate.MinutesPerMessage != 3 {
		t.Fatalf("defaults=%+v", got)
	}
	got.Dashboard.RefreshSeconds = 60
	got.Dashboard.AgentFilter = "codex"
	if _, err := m.Update(got); err != nil {
		t.Fatal(err)
	}
	reloaded, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Dashboard.RefreshSeconds != 60 {
		t.Fatalf("refresh=%d", reloaded.Dashboard.RefreshSeconds)
	}
	if reloaded.Dashboard.AgentFilter != "codex" {
		t.Fatalf("agent filter=%s", reloaded.Dashboard.AgentFilter)
	}
}

func TestAgentFilterDefaultsAndValidation(t *testing.T) {
	manager := NewManagerAt(filepath.Join(t.TempDir(), "settings.json"))
	value := Default()
	value.Dashboard.AgentFilter = ""
	saved, err := manager.Update(value)
	if err != nil || saved.Dashboard.AgentFilter != "all" {
		t.Fatalf("legacy filter normalization: %+v %v", saved.Dashboard, err)
	}
	value.Dashboard.AgentFilter = "unknown"
	if err := manager.Save(value); err == nil {
		t.Fatal("unknown agent filter accepted")
	}
}

func TestManagerRejectsInvalidAndUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"dashboard":{"window":"1d","max_inactive":1,"refresh_seconds":30,"daily_boot_enabled":true},"attention":{"protect_minutes":5,"focus_add_minutes":15,"mask_strength":1},"time_estimate":{"minutes_per_message":3,"fallback_ratio":0.3},"future":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManagerAt(path)
	if _, err := m.Load(); err != nil {
		t.Fatal(err)
	}
	bad := Default()
	bad.Dashboard.RefreshSeconds = 17
	if _, err := m.Update(bad); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestMaxActiveCoercedToMinimum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	// Legacy file where max_active=0 meant "no limit": load must normalize to 1.
	if err := os.WriteFile(path, []byte(`{"version":1,"dashboard":{"window":"1d","max_active":0,"max_inactive":0,"refresh_seconds":30,"daily_boot_enabled":true},"attention":{"protect_minutes":5,"focus_add_minutes":15,"mask_strength":1},"time_estimate":{"minutes_per_message":3,"fallback_ratio":0.3}}`), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManagerAt(path)
	got, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Dashboard.MaxActive != 1 {
		t.Fatalf("legacy max_active=0 loaded as %d, want 1", got.Dashboard.MaxActive)
	}
	if got.Dashboard.MaxInactive != 0 {
		t.Fatalf("max_inactive=0 must stay 0 (hide), got %d", got.Dashboard.MaxInactive)
	}

	// Saving 0 (stale client) persists 1, so a reload keeps showing 1.
	toSave := Default()
	toSave.Dashboard.MaxActive = 0
	toSave.Dashboard.MaxInactive = 2
	saved, err := m.Update(toSave)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Dashboard.MaxActive != 1 {
		t.Fatalf("saved max_active=%d, want 1", saved.Dashboard.MaxActive)
	}
	reloaded, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Dashboard.MaxActive != 1 {
		t.Fatalf("reloaded max_active=%d, want 1", reloaded.Dashboard.MaxActive)
	}
}

func TestExplicitSaveInvalidatesBrowserRevisionAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	manager := NewManagerAt(path)
	first, err := manager.Update(Default())
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Update(first)
	if err != nil {
		t.Fatal(err)
	}
	if first.DashboardRevision == "" || second.DashboardRevision == first.DashboardRevision {
		t.Fatal("explicit no-op save did not update revision")
	}
	reloaded, err := NewManagerAt(path).Load()
	if err != nil || reloaded.DashboardRevision != second.DashboardRevision {
		t.Fatalf("revision not persisted: %+v %v", reloaded, err)
	}
	bad := second
	bad.Dashboard.MaxInactive = -1
	err = manager.Save(bad)
	if err == nil {
		t.Fatal("invalid save accepted")
	}
	reloaded, err = manager.Load()
	if err != nil || reloaded.DashboardRevision != second.DashboardRevision {
		t.Fatal("failed save invalidated overrides")
	}
}

func TestCustomKnowledgeRoundTripAndReset(t *testing.T) {
	manager := NewManagerAt(filepath.Join(t.TempDir(), "settings.json"))
	value := Default()
	value.CustomKnowledgeTips = []CustomKnowledgeTip{
		{ID: "custom_one", Title: "自定义", Theory: "<script>literal</script>"},
		{ID: "custom_two", Title: "第二条", Theory: "正文", Design: "应用"},
	}
	err := manager.Save(value)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := manager.Load()
	if err != nil || !reflect.DeepEqual(reloaded.CustomKnowledgeTips, value.CustomKnowledgeTips) {
		t.Fatalf("round trip: %+v, %v", reloaded, err)
	}
	reset, err := manager.Reset("attention")
	if err != nil || !reflect.DeepEqual(reset.CustomKnowledgeTips, value.CustomKnowledgeTips) {
		t.Fatalf("unrelated reset lost knowledge: %+v, %v", reset, err)
	}
	reset, err = manager.Reset("all")
	if err != nil || len(reset.CustomKnowledgeTips) != 0 {
		t.Fatalf("reset all: %+v, %v", reset, err)
	}
}

func TestInvalidCustomKnowledgeDoesNotOverwriteSavedSettings(t *testing.T) {
	manager := NewManagerAt(filepath.Join(t.TempDir(), "settings.json"))
	valid := Default()
	valid.CustomKnowledgeTips = []CustomKnowledgeTip{{ID: "custom_good", Title: "标题", Theory: "正文"}}
	err := manager.Save(valid)
	if err != nil {
		t.Fatal(err)
	}
	testList := []struct {
		name string
		tips []CustomKnowledgeTip
	}{
		{"built-in ID", []CustomKnowledgeTip{{ID: "kt_chunking", Title: "t", Theory: "b"}}},
		{"empty ID", []CustomKnowledgeTip{{ID: "custom_", Title: "t", Theory: "b"}}},
		{"whitespace ID", []CustomKnowledgeTip{{ID: "custom_  ", Title: "t", Theory: "b"}}},
		{"blank title", []CustomKnowledgeTip{{ID: "custom_a", Title: " \n", Theory: "b"}}},
		{"blank body", []CustomKnowledgeTip{{ID: "custom_a", Title: "t", Theory: "\t"}}},
		{"duplicate ID", []CustomKnowledgeTip{{ID: "custom_a", Title: "t", Theory: "b"}, {ID: "custom_a", Title: "t2", Theory: "b2"}}},
	}
	for _, tc := range testList {
		t.Run(tc.name, func(t *testing.T) {
			invalid := valid
			invalid.CustomKnowledgeTips = tc.tips
			err := manager.Save(invalid)
			if err == nil {
				t.Fatal("accepted invalid knowledge")
			}
			saved, err := manager.Load()
			if err != nil || !reflect.DeepEqual(saved.CustomKnowledgeTips, valid.CustomKnowledgeTips) {
				t.Fatalf("changed on invalid save: %+v, %v", saved, err)
			}
		})
	}
	// A failed atomic replacement leaves the previous file intact.
	err = os.Mkdir(manager.path+".tmp", 0700)
	if err != nil {
		t.Fatal(err)
	}
	changed := valid
	changed.CustomKnowledgeTips = nil
	err = manager.Save(changed)
	if err == nil {
		t.Fatal("expected write failure")
	}
	saved, err := manager.Load()
	if err != nil || !reflect.DeepEqual(saved.CustomKnowledgeTips, valid.CustomKnowledgeTips) {
		t.Fatalf("changed on write failure: %+v, %v", saved, err)
	}
}
