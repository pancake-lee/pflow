package timetrack

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FocusEvent is one line from ~/.pflow/focus_history.log.
//
// Format: <timestamp_ns>|<action>|<session>|<cwd>|<project>|<pid>
type FocusEvent struct {
	Timestamp time.Time
	Action    string // "focus-in" or "focus-out"
	Session   string // tmux session name
	CWD       string
	Project   string // project name extracted from cwd
	PID       int
}

// FocusLog holds parsed focus events, grouped by project for fast lookup.
type FocusLog struct {
	Events   []FocusEvent
	ByProject map[string][]FocusEvent // project → events for that project
}

const (
	// MinFocusSegmentSeconds is the minimum duration (in seconds) for a
	// focus segment to be counted. Shorter segments are treated as noise
	// (e.g., accidental focus switches between terminals).
	MinFocusSegmentSeconds = 10.0
)

// ResolveProjects rebuilds the ByProject index using a mapping from tmux
// session name to project path. This is needed because focus hooks record
// session names, not project paths — the project is resolved at read time
// from the pflow tmux mappings.
func (fl *FocusLog) ResolveProjects(sessionToProject map[string]string) {
	if fl == nil || len(sessionToProject) == 0 {
		return
	}
	fl.ByProject = make(map[string][]FocusEvent)
	for _, ev := range fl.Events {
		proj, ok := sessionToProject[ev.Session]
		if !ok || proj == "" {
			continue
		}
		fl.ByProject[proj] = append(fl.ByProject[proj], ev)
	}
}

// DefaultFocusLogPath returns the standard path to the focus history log.
func DefaultFocusLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home + "/.pflow/focus_history.log", nil
}

// ReadFocusLog reads and parses the focus history log file.
// Returns nil if the file does not exist (not an error — focus events
// are an optional enhancement).
func ReadFocusLog(path string) (*FocusLog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var events []FocusEvent
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ev, ok := parseFocusLine(line)
		if !ok {
			continue
		}
		events = append(events, ev)
	}

	// Sort by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	// Group by project
	byProject := make(map[string][]FocusEvent)
	for _, ev := range events {
		key := ev.Project
		if key == "" {
			key = ev.CWD
		}
		if key == "" {
			continue
		}
		byProject[key] = append(byProject[key], ev)
	}

	return &FocusLog{
		Events:    events,
		ByProject: byProject,
	}, nil
}

// parseFocusLine parses one line of the focus history log.
// Format: <timestamp_ns>|<action>|<session>|<cwd>|<project>|<pid>
func parseFocusLine(line string) (FocusEvent, bool) {
	parts := strings.Split(line, "|")
	if len(parts) < 5 {
		return FocusEvent{}, false
	}

	tsNs, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return FocusEvent{}, false
	}

	ev := FocusEvent{
		Timestamp: time.Unix(0, tsNs),
		Action:    parts[1],
		Session:   parts[2],
		CWD:       parts[3],
		Project:   parts[4],
	}
	if len(parts) >= 6 {
		ev.PID, _ = strconv.Atoi(parts[5])
	}
	return ev, true
}

// ProjectMinutes calculates total focus time for a project today by
// aggregating focus-in/focus-out event pairs.
//
// Algorithm (from docs/design/09-active-time-calculation.md §6.4):
//  1. Get today's focus-in/focus-out events for the project, sorted by time
//  2. Iterate: focus-in starts a segment, focus-out ends it
//  3. If the last event is focus-in (unclosed), close at now
//  4. Filter out segments shorter than MinFocusSegmentSeconds
//  5. Return total minutes
func (fl *FocusLog) ProjectMinutes(projectPath string, todayStart, now time.Time) float64 {
	events, ok := fl.ByProject[projectPath]
	if !ok || len(events) == 0 {
		return 0
	}

	// Filter to today's events only
	var todayEvents []FocusEvent
	for _, ev := range events {
		if !ev.Timestamp.Before(todayStart) {
			todayEvents = append(todayEvents, ev)
		}
	}
	if len(todayEvents) == 0 {
		return 0
	}

	var totalSeconds float64
	var segmentStart *time.Time

	for _, ev := range todayEvents {
		switch ev.Action {
		case "focus-in":
			if segmentStart == nil {
				t := ev.Timestamp
				segmentStart = &t
			}
		case "focus-out":
			if segmentStart != nil {
				dur := ev.Timestamp.Sub(*segmentStart).Seconds()
				if dur >= MinFocusSegmentSeconds {
					totalSeconds += dur
				}
				segmentStart = nil
			}
		}
	}

	// If last event was focus-in, close at now
	if segmentStart != nil {
		dur := now.Sub(*segmentStart).Seconds()
		if dur >= MinFocusSegmentSeconds {
			totalSeconds += dur
		}
	}

	return totalSeconds / 60.0
}
