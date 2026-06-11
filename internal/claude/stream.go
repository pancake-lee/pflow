// Package claude provides access to Claude Code's local data and subprocess management.
package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Event is a single stream-json event from Claude Code's stdout.
// The Type field discriminates the event kind; other fields are populated
// depending on the type.
type Event struct {
	Type      string          `json:"type"`
	Message   json.RawMessage `json:"message,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	UUID      string          `json:"uuid,omitempty"`
	ParentUUID string         `json:"parentUuid,omitempty"`

	// user event fields
	CWD      string `json:"cwd,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`

	// mode event
	Mode string `json:"mode,omitempty"`

	// permission-mode event
	PermissionMode string `json:"permissionMode,omitempty"`
}

// UserEvent is the parsed message content of a "user" event.
type UserEvent struct {
	Role    string      `json:"role"`
	Content UserContent `json:"content"`
}

// UserContent is either a plain string or an array of content blocks.
type UserContent []UserContentBlock

// UnmarshalJSON handles both string and array forms of user content.
func (uc *UserContent) UnmarshalJSON(b []byte) error {
	// Try string first
	var s string
	if json.Unmarshal(b, &s) == nil {
		*uc = UserContent{{Type: "text", Text: s}}
		return nil
	}
	// Otherwise treat as array
	var blocks []UserContentBlock
	if err := json.Unmarshal(b, &blocks); err != nil {
		return err
	}
	*uc = UserContent(blocks)
	return nil
}

// UserContentBlock is a content block within a user message.
type UserContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Text returns the plain-text content of a user event, joining all text blocks.
func (e *UserEvent) Text() string {
	var parts []string
	for _, b := range e.Content {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// AssistantEvent is the parsed message content of an "assistant" event.
type AssistantEvent struct {
	ID        string              `json:"id"`
	Role      string              `json:"role"`
	Model     string              `json:"model"`
	Content   []AssistantBlock    `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage     *UsageInfo          `json:"usage,omitempty"`
}

// AssistantBlock is a content block within an assistant message.
type AssistantBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Name string `json:"name,omitempty"`
	// tool_use input is arbitrary JSON, ignored for now
}

// UsageInfo holds token usage counters.
type UsageInfo struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Text returns the plain-text content of an assistant event, joining all text blocks.
func (e *AssistantEvent) Text() string {
	var parts []string
	for _, b := range e.Content {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// ParseAssistant parses the Message field of an Event into an AssistantEvent.
func (ev *Event) ParseAssistant() (*AssistantEvent, error) {
	if ev.Type != "assistant" {
		return nil, fmt.Errorf("event type is %q, not assistant", ev.Type)
	}
	var ae AssistantEvent
	if err := json.Unmarshal(ev.Message, &ae); err != nil {
		return nil, fmt.Errorf("parsing assistant message: %w", err)
	}
	return &ae, nil
}

// ParseUser parses the Message field of an Event into a UserEvent.
func (ev *Event) ParseUser() (*UserEvent, error) {
	if ev.Type != "user" {
		return nil, fmt.Errorf("event type is %q, not user", ev.Type)
	}
	var ue UserEvent
	if err := json.Unmarshal(ev.Message, &ue); err != nil {
		return nil, fmt.Errorf("parsing user message: %w", err)
	}
	return &ue, nil
}

// ParseEvents reads stream-json lines from r and sends each parsed Event to the
// returned channel. The channel is closed when the reader reaches EOF or an
// unrecoverable parse error occurs.
func ParseEvents(r io.Reader) <-chan Event {
	ch := make(chan Event)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		// Claude Code stream-json lines can be large (tool results, file contents).
		// 10 MB buffer should be enough for most cases.
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var ev Event
			if err := json.Unmarshal([]byte(line), &ev); err != nil {
				// Skip unparseable lines; the caller should inspect errors separately
				// if needed. For monitoring purposes, a missed event is acceptable.
				continue
			}
			ch <- ev
		}
	}()
	return ch
}

// writeJSON writes v as a single JSON line to w.
func writeJSON(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}
