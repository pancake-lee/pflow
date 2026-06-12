// Package api provides the pflow HTTP API server and managed-session infrastructure.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/chenhg5/cc-connect/agent/claudecode"
	"github.com/chenhg5/cc-connect/core"
)

// SessionManager owns managed Claude Code sessions (backed by cc-connect's
// claudecode.Agent). It is safe for concurrent use.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*ManagedSession
}

// ManagedSession is one pflow-managed Claude Code session.
type ManagedSession struct {
	SessionID string
	Project   string

	agent  core.Agent
	sess   core.AgentSession
	cancel context.CancelFunc

	mu      sync.Mutex
	pending *PendingPermission

	lastReq  string
	lastResp string

	startedAt time.Time
}

// PendingPermission holds the details of a pending permission request
// that the user must approve or deny.
type PendingPermission struct {
	RequestID    string         `json:"request_id"`
	ToolName     string         `json:"tool_name"`
	ToolInput    string         `json:"tool_input"`     // human-readable summary
	ToolInputRaw map[string]any `json:"tool_input_raw"` // full structured input
}

// ManagedSessionSnapshot is a point-in-time view of a managed session,
// safe for JSON serialization in the dashboard API.
type ManagedSessionSnapshot struct {
	SessionID   string             `json:"session_id"`
	Project     string             `json:"project"`
	IsAlive     bool               `json:"is_alive"`
	PendingPerm *PendingPermission `json:"pending_permission,omitempty"`
	LastReq     string             `json:"last_req"`
	LastResp    string             `json:"last_resp"`
	StartedAt   time.Time          `json:"started_at"`
}

// NewSessionManager creates a new empty SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*ManagedSession),
	}
}

// Start creates a new Claude Code session in the given project directory.
// If prompt is non-empty it is sent as the first user message.
func (m *SessionManager) Start(parentCtx context.Context, projectDir, prompt string) (*ManagedSessionSnapshot, error) {
	agent, err := claudecode.New(map[string]any{
		"work_dir": projectDir,
	})
	if err != nil {
		return nil, fmt.Errorf("create claude agent: %w", err)
	}

	ctx, cancel := context.WithCancel(parentCtx)

	sess, err := agent.StartSession(ctx, "")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("start claude session: %w", err)
	}

	ms := &ManagedSession{
		SessionID: sess.CurrentSessionID(),
		Project:   projectDir,
		agent:     agent,
		sess:      sess,
		cancel:    cancel,
		startedAt: time.Now(),
	}

	// If the session ID is empty initially, wait for it to appear via events.
	// Claude Code sends a system event with the session_id early in the handshake.
	if ms.SessionID == "" {
		// Read the first few events to capture the session ID.
		// Use a short timeout — the session ID typically arrives within 1-2 events.
		idCtx, idCancel := context.WithTimeout(ctx, 10*time.Second)
		defer idCancel()
		for {
			select {
			case ev, ok := <-sess.Events():
				if !ok {
					cancel()
					return nil, fmt.Errorf("session closed before session_id was received")
				}
				if ev.SessionID != "" {
					ms.SessionID = ev.SessionID
				}
				if ms.SessionID != "" {
					goto haveID
				}
			case <-idCtx.Done():
				cancel()
				return nil, fmt.Errorf("timed out waiting for session_id")
			}
		}
	haveID:
	}

	m.mu.Lock()
	m.sessions[ms.SessionID] = ms
	m.mu.Unlock()

	// Start background event loop
	go ms.eventLoop()

	// Send initial prompt if provided
	if prompt != "" {
		if err := ms.Send(prompt); err != nil {
			slog.Warn("session_manager: initial prompt failed", "session_id", ms.SessionID, "error", err)
		}
	}

	slog.Info("session_manager: session started", "session_id", ms.SessionID, "project", projectDir)
	return ms.Snapshot(), nil
}

// Send sends a user message to a managed session.
func (m *SessionManager) Send(sessionID, prompt string) error {
	ms, ok := m.get(sessionID)
	if !ok {
		return fmt.Errorf("session %q not found", sessionID)
	}
	return ms.Send(prompt)
}

// Approve approves a pending permission request.
func (m *SessionManager) Approve(sessionID, requestID string) error {
	ms, ok := m.get(sessionID)
	if !ok {
		return fmt.Errorf("session %q not found", sessionID)
	}
	return ms.approve(requestID)
}

// Deny denies a pending permission request.
func (m *SessionManager) Deny(sessionID, requestID string) error {
	ms, ok := m.get(sessionID)
	if !ok {
		return fmt.Errorf("session %q not found", sessionID)
	}
	return ms.deny(requestID)
}

// Stop terminates a managed session.
func (m *SessionManager) Stop(sessionID string) error {
	ms, ok := m.get(sessionID)
	if !ok {
		return fmt.Errorf("session %q not found", sessionID)
	}
	ms.cancel()
	if err := ms.sess.Close(); err != nil {
		slog.Warn("session_manager: close error", "session_id", sessionID, "error", err)
	}
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
	slog.Info("session_manager: session stopped", "session_id", sessionID)
	return nil
}

// Snapshot returns a point-in-time view of a managed session.
func (m *SessionManager) Snapshot(sessionID string) *ManagedSessionSnapshot {
	ms, ok := m.get(sessionID)
	if !ok {
		return nil
	}
	return ms.Snapshot()
}

// GetSession returns the ManagedSession for the given ID, or nil.
func (m *SessionManager) GetSession(sessionID string) *ManagedSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[sessionID]
}

// ListSnapshots returns snapshots of all managed sessions.
func (m *SessionManager) ListSnapshots() []ManagedSessionSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ManagedSessionSnapshot
	for _, ms := range m.sessions {
		out = append(out, *ms.Snapshot())
	}
	return out
}

// ── private helpers ──────────────────────────────────────────────

func (m *SessionManager) get(sessionID string) (*ManagedSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ms, ok := m.sessions[sessionID]
	return ms, ok
}

// ── ManagedSession methods ────────────────────────────────────────

func (ms *ManagedSession) Send(prompt string) error {
	if !ms.sess.Alive() {
		return fmt.Errorf("session is not running")
	}
	ms.mu.Lock()
	ms.lastReq = prompt
	ms.mu.Unlock()
	return ms.sess.Send(prompt, nil, nil)
}

func (ms *ManagedSession) approve(requestID string) error {
	ms.mu.Lock()
	p := ms.pending
	if p == nil || p.RequestID != requestID {
		ms.mu.Unlock()
		return fmt.Errorf("no pending permission with id %q", requestID)
	}
	ms.pending = nil
	ms.mu.Unlock()

	slog.Info("session_manager: approving permission", "session_id", ms.SessionID, "request_id", requestID)
	return ms.sess.RespondPermission(requestID, core.PermissionResult{
		Behavior:     "allow",
		UpdatedInput: p.ToolInputRaw,
	})
}

func (ms *ManagedSession) deny(requestID string) error {
	ms.mu.Lock()
	p := ms.pending
	if p == nil || p.RequestID != requestID {
		ms.mu.Unlock()
		return fmt.Errorf("no pending permission with id %q", requestID)
	}
	ms.pending = nil
	ms.mu.Unlock()

	slog.Info("session_manager: denying permission", "session_id", ms.SessionID, "request_id", requestID)
	return ms.sess.RespondPermission(requestID, core.PermissionResult{
		Behavior: "deny",
		Message:  "User denied this tool use.",
	})
}

// Events returns the event channel from the underlying cc-connect session.
func (ms *ManagedSession) Events() <-chan core.Event {
	return ms.sess.Events()
}

// RespondPermission sends a permission decision to the underlying session.
func (ms *ManagedSession) RespondPermission(requestID string, result core.PermissionResult) error {
	return ms.sess.RespondPermission(requestID, result)
}

// Snapshot returns a point-in-time view of this managed session.
func (ms *ManagedSession) Snapshot() *ManagedSessionSnapshot {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return &ManagedSessionSnapshot{
		SessionID:   ms.SessionID,
		Project:     ms.Project,
		IsAlive:     ms.sess.Alive(),
		PendingPerm: ms.pending,
		LastReq:     ms.lastReq,
		LastResp:    ms.lastResp,
		StartedAt:   ms.startedAt,
	}
}

// eventLoop reads events from the Claude Code session and updates state.
func (ms *ManagedSession) eventLoop() {
	for ev := range ms.sess.Events() {
		switch ev.Type {
		case core.EventPermissionRequest:
			ms.mu.Lock()
			ms.pending = &PendingPermission{
				RequestID:    ev.RequestID,
				ToolName:     ev.ToolName,
				ToolInput:    ev.ToolInput,
				ToolInputRaw: ev.ToolInputRaw,
			}
			ms.mu.Unlock()
			slog.Info("session_manager: permission requested",
				"session_id", ms.SessionID,
				"request_id", ev.RequestID,
				"tool", ev.ToolName,
				"input", ev.ToolInput,
			)

		case core.EventResult:
			ms.mu.Lock()
			if ms.pending != nil {
				// Permission was auto-resolved or cancelled; clear it.
				ms.pending = nil
			}
			if ev.Content != "" {
				ms.lastResp = ev.Content
			}
			ms.mu.Unlock()

		case core.EventText:
			if ev.Content != "" {
				ms.mu.Lock()
				ms.lastResp = ev.Content
				ms.mu.Unlock()
			}

		case core.EventError:
			slog.Error("session_manager: session error",
				"session_id", ms.SessionID,
				"error", ev.Error,
			)

		case core.EventToolUse:
			slog.Info("session_manager: tool use",
				"session_id", ms.SessionID,
				"tool", ev.ToolName,
				"input", ev.ToolInput,
			)
		}

		// Update session ID if it changed
		if ev.SessionID != "" && ev.SessionID != ms.SessionID {
			ms.SessionID = ev.SessionID
		}
	}

	// Event channel closed — session ended.
	slog.Info("session_manager: event loop ended", "session_id", ms.SessionID)
}

// ── External Permission Store ─────────────────────────────────────
//
// External permissions are registered by CLI processes (pflow start)
// and resolved through the web dashboard. The CLI polls pflow serve
// for the decision.
// Keyed by the Claude Code permission request_id for simplicity.

// ExternalPerm is a permission request from an external (non-serve) session.
type ExternalPerm struct {
	RequestID   string             `json:"request_id"`
	SessionID   string             `json:"session_id"`
	Project     string             `json:"project"`
	Perm        PendingPermission  `json:"permission"`
	Decision    *string            `json:"decision,omitempty"` // "allow" or "deny", nil = pending
	CreatedAt   time.Time          `json:"created_at"`
}

// ExternalPermStore holds permission requests from external CLI sessions.
type ExternalPermStore struct {
	mu    sync.Mutex
	perms map[string]*ExternalPerm // request_id → perm
}

// NewExternalPermStore creates a new external permission store.
func NewExternalPermStore() *ExternalPermStore {
	return &ExternalPermStore{perms: make(map[string]*ExternalPerm)}
}

// Register adds a new external permission request.
func (s *ExternalPermStore) Register(sessionID, project string, perm PendingPermission) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.perms[perm.RequestID] = &ExternalPerm{
		RequestID: perm.RequestID,
		SessionID: sessionID,
		Project:   project,
		Perm:      perm,
		CreatedAt: time.Now(),
	}
	slog.Info("external_perm: registered", "request_id", perm.RequestID, "session_id", sessionID, "tool", perm.ToolName)
}

// GetDecision returns the decision for a permission request. Returns ("", false) if
// still pending or not found; ("allow", true) or ("deny", true) if decided.
func (s *ExternalPermStore) GetDecision(requestID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.perms[requestID]
	if !ok {
		return "", false
	}
	if p.Decision == nil {
		return "", false
	}
	return *p.Decision, true
}

// SetDecisionByRequestID sets the decision for a permission request by its request_id.
func (s *ExternalPermStore) SetDecisionByRequestID(requestID, decision string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.perms[requestID]
	if !ok {
		return false
	}
	p.Decision = &decision
	slog.Info("external_perm: decision set", "request_id", requestID, "decision", decision)
	return true
}

// ListPending returns all still-pending external permission requests.
func (s *ExternalPermStore) ListPending() []ExternalPerm {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ExternalPerm
	for _, p := range s.perms {
		if p.Decision == nil {
			out = append(out, *p)
		}
	}
	return out
}

// Cleanup removes decided requests older than the given duration.
func (s *ExternalPermStore) Cleanup(age time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-age)
	for id, p := range s.perms {
		if p.Decision != nil && p.CreatedAt.Before(cutoff) {
			delete(s.perms, id)
		}
	}
}
