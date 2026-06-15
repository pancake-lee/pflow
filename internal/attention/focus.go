package attention

import (
	"sync"
	"time"
)

// FocusState tracks the user's explicit focus mode on a specific project.
// When focus mode is active, the reminder score algorithm applies a
// protection period — reminders for other projects are suppressed until
// the user has focused for at least the configured duration. The focused
// project is always clear (no fog), and all other projects receive
// protection fog during the protection period.
//
// Focus mode is per-project and explicitly opt-in: the user must click
// "Focus" on a specific project card. Without it, no protection period
// is applied.
type FocusState struct {
	mu          sync.Mutex
	Active      bool
	ProjectPath string  // which project the user is focusing on
	Minutes     float64 // accumulated protection minutes
	Since       time.Time
}

var globalFocus = &FocusState{}

// GetFocus returns the global FocusState singleton.
func GetFocus() *FocusState {
	return globalFocus
}

// Extend activates focus mode for the given project (if not already active)
// and adds 15 minutes to the protection window. If focus is already active
// on a different project, it switches to the new project and resets the timer.
// Returns the new state.
func (f *FocusState) Extend(projectPath string) (active bool, minutes float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.Active || f.ProjectPath != projectPath {
		f.Active = true
		f.ProjectPath = projectPath
		f.Since = time.Now()
		f.Minutes = 15
	} else {
		f.Minutes += 15
	}
	return f.Active, f.Minutes
}

// Stop deactivates focus mode and resets the protection window.
func (f *FocusState) Stop() (active bool, minutes float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Active = false
	f.ProjectPath = ""
	f.Minutes = 0
	return f.Active, f.Minutes
}

// Snapshot returns a copy of the current focus state (thread-safe).
func (f *FocusState) Snapshot() (active bool, projectPath string, minutes float64, since time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Active, f.ProjectPath, f.Minutes, f.Since
}

// FocusSnapshot is the JSON-serializable representation of FocusState.
type FocusSnapshot struct {
	Active         bool    `json:"active"`
	FocusedProject string  `json:"focused_project"`
	Minutes        float64 `json:"minutes"`
	Since          string  `json:"since"` // ISO 8601, when the current focus period started
}
