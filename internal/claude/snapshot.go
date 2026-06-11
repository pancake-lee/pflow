package claude

import (
	"sync"
	"time"
)

// State represents the inferred state of a Claude Code session.
type State string

const (
	// StateBusy means the assistant is actively processing (generating text or executing tools).
	StateBusy State = "busy"
	// StateWaiting means the assistant is waiting for permission (tool approval prompt).
	StateWaiting State = "waiting"
	// StateIdle means the session is waiting for user input.
	StateIdle State = "idle"
	// StateUnknown means we don't have enough information to determine the state.
	StateUnknown State = "unknown"
)

// String returns the string representation of the state.
func (s State) String() string { return string(s) }

// Emoji returns an emoji icon for the state.
func (s State) Emoji() string {
	switch s {
	case StateBusy:
		return "🟢"
	case StateWaiting:
		return "🟡"
	case StateIdle:
		return "⚪"
	default:
		return "❓"
	}
}

// Snapshot is a point-in-time view of a Claude Code session inferred from stream events.
type Snapshot struct {
	SessionID  string
	State      State
	Project    string
	LastReq    string    // first ~N chars of the latest user message
	LastResp   string    // first ~N chars of the latest assistant text response
	LastActive time.Time
	Model      string
}

// Tracker maintains per-session state by consuming stream-json events.
// It is safe for concurrent use.
type Tracker struct {
	mu        sync.Mutex
	snapshots map[string]*Snapshot
}

// NewTracker creates a new Tracker.
func NewTracker() *Tracker {
	return &Tracker{
		snapshots: make(map[string]*Snapshot),
	}
}

// Process consumes a stream-json Event and updates the corresponding session snapshot.
func (t *Tracker) Process(ev Event) {
	t.mu.Lock()
	defer t.mu.Unlock()

	sid := ev.SessionID
	if sid == "" {
		return
	}

	ss := t.snapshots[sid]
	if ss == nil {
		ss = &Snapshot{SessionID: sid}
		t.snapshots[sid] = ss
	}

	switch ev.Type {
	case "user":
		ue, err := ev.ParseUser()
		if err != nil {
			return
		}
		ss.LastReq = truncateText(ue.Text(), 15)
		ss.State = StateBusy // user just sent a message, assistant will process
		ss.Project = ev.CWD

	case "assistant":
		ae, err := ev.ParseAssistant()
		if err != nil {
			return
		}
		text := ae.Text()
		if text != "" {
			ss.LastResp = truncateText(text, 15)
		}
		ss.Model = ae.Model
		// Determine state from stop_reason
		switch ae.StopReason {
		case "end_turn":
			ss.State = StateIdle // assistant finished, waiting for user
		case "tool_use":
			ss.State = StateBusy // tools are executing, still busy
		case "stop_sequence":
			ss.State = StateIdle
		default:
			// If the assistant sent text and we don't know what's next,
			// assume idle (most common: text response + end of turn)
			ss.State = StateIdle
		}

	case "mode":
		// Mode changes don't affect state directly
	case "permission-mode":
		// Permission mode changes don't affect state directly
	}

	// Update timestamp if available
	if ev.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, ev.Timestamp); err == nil {
			ss.LastActive = t
		}
	}
	if ss.LastActive.IsZero() {
		ss.LastActive = time.Now()
	}
}

// Get returns the current snapshot for a session, or nil if not found.
func (t *Tracker) Get(sessionID string) *Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snapshots[sessionID]
}

// All returns all current snapshots.
func (t *Tracker) All() []*Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	var result []*Snapshot
	for _, ss := range t.snapshots {
		result = append(result, ss)
	}
	return result
}

// Remove deletes a session from the tracker.
func (t *Tracker) Remove(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.snapshots, sessionID)
}

// truncateText truncates text to maxLen characters (runes), adding "..." if truncated.
func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}
