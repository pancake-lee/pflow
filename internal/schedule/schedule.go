// Package schedule persists daily plans and reusable templates.
package schedule

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrRecordedItemsNeedConfirmation is returned when a save would replace or
// remove an item that already has an execution record.
var ErrRecordedItemsNeedConfirmation = errors.New("recorded schedule items require replacement confirmation")

type Item struct {
	ID           string    `json:"id"`
	Start        string    `json:"start"`
	DayOffset    int       `json:"day_offset,omitempty"`
	Title        string    `json:"title"`
	FocusMinutes int       `json:"focus_minutes"`
	BreakMinutes int       `json:"break_minutes"`
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

// NewManagerAt creates a manager rooted at path. It is used by isolated tests.
func NewManagerAt(path string) *Manager {
	return &Manager{path: path}
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
	seenIDMap := make(map[string]bool, len(items))
	for i := range items {
		item := items[i]
		if item.ID == "" || seenIDMap[item.ID] || strings.TrimSpace(item.Title) == "" || item.BreakMinutes < 0 {
			return fmt.Errorf("invalid schedule item")
		}
		seenIDMap[item.ID] = true
		parsed, err := time.Parse("15:04", item.Start)
		if err != nil {
			return fmt.Errorf("invalid start time")
		}
		if item.DayOffset < 0 || item.DayOffset > 1 {
			return fmt.Errorf("invalid schedule day offset")
		}
		startMinute := item.DayOffset*24*60 + parsed.Hour()*60 + parsed.Minute()
		if startMinute <= last {
			return fmt.Errorf("items must be ordered")
		}
		if item.FocusMinutes < 1 || startMinute+item.FocusMinutes+item.BreakMinutes > 2*24*60 {
			return fmt.Errorf("invalid schedule duration")
		}
		if i+1 < len(items) {
			nextParsed, err := time.Parse("15:04", items[i+1].Start)
			nextMinute := items[i+1].DayOffset*24*60 + nextParsed.Hour()*60 + nextParsed.Minute()
			if err == nil && nextMinute < startMinute+item.FocusMinutes+item.BreakMinutes {
				return fmt.Errorf("schedule items overlap")
			}
		}
		last = startMinute
	}
	return nil
}

// CleanTemplate returns a reusable template snapshot without execution data.
func CleanTemplate(template Template) Template {
	clean := template
	clean.Items = make([]Item, len(template.Items))
	for i, item := range template.Items {
		item.Status = ""
		item.CompletedAt = time.Time{}
		clean.Items[i] = item
	}
	return clean
}

// CleanTemplateList removes execution data from every reusable template.
func CleanTemplateList(templateList []Template) []Template {
	cleanList := make([]Template, len(templateList))
	for i, template := range templateList {
		cleanList[i] = CleanTemplate(template)
	}
	return cleanList
}

// PrepareDaySave keeps unchanged execution records and rejects accidental
// replacement or deletion of recorded items unless explicitly confirmed.
func PrepareDaySave(previous Day, next Day, replaceRecorded bool) (Day, error) {
	prepared := next
	prepared.Items = append([]Item(nil), next.Items...)
	for i := range prepared.Items {
		prepared.Items[i].Status = ""
		prepared.Items[i].CompletedAt = time.Time{}
	}
	recordedByIDMap := make(map[string]Item)
	for _, item := range previous.Items {
		if item.Status != "" {
			recordedByIDMap[item.ID] = item
		}
	}
	nextIndexByIDMap := make(map[string]int, len(prepared.Items))
	for i, item := range prepared.Items {
		nextIndexByIDMap[item.ID] = i
	}
	for id, previousItem := range recordedByIDMap {
		nextIndex, exists := nextIndexByIDMap[id]
		if !exists || !samePlan(previousItem, prepared.Items[nextIndex]) {
			if !replaceRecorded {
				return Day{}, ErrRecordedItemsNeedConfirmation
			}
			continue
		}
		prepared.Items[nextIndex].Status = previousItem.Status
		prepared.Items[nextIndex].CompletedAt = previousItem.CompletedAt
	}
	if prepared.CurrentID == "" && containsItem(prepared.Items, previous.CurrentID) {
		prepared.CurrentID = previous.CurrentID
	}
	return prepared, nil
}

func samePlan(left, right Item) bool {
	return left.ID == right.ID && left.Start == right.Start && left.Title == right.Title &&
		left.DayOffset == right.DayOffset && left.FocusMinutes == right.FocusMinutes && left.BreakMinutes == right.BreakMinutes
}

func containsItem(itemList []Item, id string) bool {
	if id == "" {
		return false
	}
	for _, item := range itemList {
		if item.ID == id {
			return true
		}
	}
	return false
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
	f.Templates = CleanTemplateList(f.Templates)
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
	sortItems(day.Items)
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
	date := day.Date
	if date == "" {
		date = now.Format("2006-01-02")
	}
	for _, item := range day.Items {
		start, err := ItemStart(date, item, now.Location())
		if err == nil && item.Status == "" && !start.After(now) {
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

// ItemStart returns the real local start time for an item in a dated plan.
func ItemStart(date string, item Item, location *time.Location) (time.Time, error) {
	if date == "" {
		date = time.Now().In(location).Format("2006-01-02")
	}
	dayStart, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil {
		return time.Time{}, err
	}
	parsed, err := time.Parse("15:04", item.Start)
	if err != nil {
		return time.Time{}, err
	}
	return dayStart.AddDate(0, 0, item.DayOffset).Add(time.Duration(parsed.Hour()*60+parsed.Minute()) * time.Minute), nil
}

// HasNextDayPending reports whether a plan still owns work on date.
func HasNextDayPending(day Day, date string) bool {
	if day.Date == "" {
		return false
	}
	planDate, err := time.Parse("2006-01-02", day.Date)
	if err != nil || planDate.AddDate(0, 0, 1).Format("2006-01-02") != date {
		return false
	}
	for _, item := range day.Items {
		endMinute := timelineMinute(item) + item.FocusMinutes + item.BreakMinutes
		if item.Status == "" && (item.DayOffset == 1 || endMinute > 24*60) {
			return true
		}
	}
	return false
}

func sortItems(itemList []Item) {
	sort.Slice(itemList, func(i, j int) bool {
		return timelineMinute(itemList[i]) < timelineMinute(itemList[j])
	})
}

func timelineMinute(item Item) int {
	parsed, err := time.Parse("15:04", item.Start)
	if err != nil {
		return item.DayOffset * 24 * 60
	}
	return item.DayOffset*24*60 + parsed.Hour()*60 + parsed.Minute()
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

// Advance pins the item shown as "next" after fromID as the new current, so a
// user complete/skip immediately promotes it regardless of wall time. When no
// pending item remains it clears the pin and falls back to the free schedule.
func Advance(day *Day, fromID string) (Item, *Item) {
	next := nextItem(day.Items, fromID)
	if next == nil {
		day.CurrentID = ""
		return Item{Title: "自由日程"}, nil
	}
	day.CurrentID = next.ID
	return *next, nextItem(day.Items, next.ID)
}

func SetCurrent(day *Day, now time.Time) (Item, *Item) {
	current, next := Current(*day, now)
	if day.CurrentID == "" && current.ID != "" {
		day.CurrentID = current.ID
	}
	return current, next
}
