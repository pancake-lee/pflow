// Package schedule persists daily plans and reusable templates.
package schedule

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Item struct {
	ID           string    `json:"id"`
	Start        string    `json:"start"`
	Title        string    `json:"title"`
	FocusMinutes int       `json:"focus_minutes"`
	BreakMinutes int       `json:"break_minutes"`
	UntilEnd     bool      `json:"until_end,omitempty"`
	Status       string    `json:"status,omitempty"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
}
type Day struct {
	Date      string `json:"date"`
	Items     []Item `json:"items"`
	CurrentID string `json:"current_id,omitempty"`
}
type Template struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Items []Item `json:"items"`
}
type File struct {
	Version   int            `json:"version"`
	Days      map[string]Day `json:"days"`
	Templates []Template     `json:"templates"`
}
type Manager struct {
	mu   sync.Mutex
	path string
}

func NewManager() *Manager {
	home, _ := os.UserHomeDir()
	return &Manager{path: filepath.Join(home, ".pflow", "schedules.json")}
}
func (m *Manager) Load() (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := os.ReadFile(m.path)
	if os.IsNotExist(err) {
		return File{Version: 1, Days: map[string]Day{}}, nil
	}
	if err != nil {
		return File{}, err
	}
	var f File
	if err = json.Unmarshal(data, &f); err != nil {
		return File{}, err
	}
	if f.Days == nil {
		f.Days = map[string]Day{}
	}
	return f, nil
}
func validate(items []Item) error {
	last := -1
	for i := range items {
		p := strings.Split(items[i].Start, ":")
		if len(p) != 2 || items[i].Title == "" || items[i].FocusMinutes < 1 || items[i].BreakMinutes < 0 {
			return fmt.Errorf("invalid schedule item")
		}
		var h, min int
		if _, err := fmt.Sscanf(items[i].Start, "%d:%d", &h, &min); err != nil || h > 23 || min > 59 {
			return fmt.Errorf("invalid start time")
		}
		now := h*60 + min
		if now <= last {
			return fmt.Errorf("items must be ordered")
		}
		last = now
	}
	return nil
}
func (m *Manager) Save(f File) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f.Version = 1
	if f.Days == nil {
		f.Days = map[string]Day{}
	}
	for _, d := range f.Days {
		if err := validate(d.Items); err != nil {
			return err
		}
	}
	for _, t := range f.Templates {
		if err := validate(t.Items); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(f, "", "  ")
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}
func Current(day Day, now time.Time) (Item, *Item) {
	free := Item{Title: "自由日程"}
	if len(day.Items) == 0 {
		return free, nil
	}
	sort.Slice(day.Items, func(i, j int) bool { return day.Items[i].Start < day.Items[j].Start })
	current := free
	if day.CurrentID != "" {
		for _, item := range day.Items {
			if item.ID == day.CurrentID && item.Status == "" {
				current = item
				break
			}
		}
		if current.ID != "" {
			return current, nextItem(day.Items, current.ID)
		}
	}
	// First display is located by wall time, then the API persists CurrentID so
	// future time passing never advances the schedule without user action.
	minute := now.Hour()*60 + now.Minute()
	for _, item := range day.Items {
		var hour, min int
		_, _ = fmt.Sscanf(item.Start, "%d:%d", &hour, &min)
		if item.Status == "" && hour*60+min <= minute {
			current = item
		}
	}
	if current.ID != "" {
		return current, nextItem(day.Items, current.ID)
	}
	for _, item := range day.Items {
		if item.Status == "" {
			return free, &item
		}
	}
	return free, nil
}

func nextItem(items []Item, currentID string) *Item {
	found := false
	for _, item := range items {
		if found && item.Status == "" {
			candidate := item
			return &candidate
		}
		if item.ID == currentID {
			found = true
		}
	}
	return nil
}

func SetCurrent(day *Day, now time.Time) (Item, *Item) {
	current, next := Current(*day, now)
	if day.CurrentID == "" && current.ID != "" {
		day.CurrentID = current.ID
	}
	return current, next
}
