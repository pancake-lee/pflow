// Package session provides tmux + ttyd session management for pflow.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ── Statusline configuration ───────────────────────────────────────

// claudeSettingsPath returns the path to ~/.claude/settings.json.
func claudeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// statuslineCommand is the Claude statusline command that displays the
// 8-char session ID prefix as the first element in the status line.
// Format: "sid8 | model | ctx | tok | session"
const statuslineCommand = `input=$(cat); sid=$(echo "$input" | jq -r '.session_id // "-"'); sid8=$(printf "%.8s" "$sid"); model=$(echo "$input" | jq -r '.model.display_name // empty'); used=$(echo "$input" | jq -r '.context_window.used_percentage // empty'); rem=$(echo "$input" | jq -r '.context_window.remaining_percentage // empty'); in_tok=$(echo "$input" | jq -r '.context_window.current_usage.input_tokens // 0'); out_tok=$(echo "$input" | jq -r '.context_window.current_usage.output_tokens // 0'); total_in=$(echo "$input" | jq -r '.context_window.total_input_tokens // 0'); total_out=$(echo "$input" | jq -r '.context_window.total_output_tokens // 0'); [ -n "$used" ] && [ -n "$rem" ] && ctx="ctx ${used}%/ ${rem}%" || ctx="ctx --"; tok="in:${in_tok} out:${out_tok}"; session="total:${total_in}/${total_out}"; [ -n "$model" ] && echo "${sid8} | ${model} | ${ctx} | ${tok} | ${session}" || echo "${sid8} | ${ctx} | ${tok} | ${session}"`

// StatuslineStatus describes the current statusline configuration state.
type StatuslineStatus int

const (
	StatuslineOK          StatuslineStatus = iota // already configured correctly
	StatuslineNotSet                               // no statusline configured
	StatuslineDifferent                            // has statusline but doesn't show session prefix
)

// checkStatusline reads ~/.claude/settings.json and determines whether the
// statusline is configured to show the session ID prefix.
func checkStatusline() (StatuslineStatus, error) {
	path, err := claudeSettingsPath()
	if err != nil {
		return StatuslineNotSet, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return StatuslineNotSet, nil
		}
		return StatuslineNotSet, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return StatuslineNotSet, fmt.Errorf("cannot parse %s: %w", path, err)
	}

	sl, ok := settings["statusLine"]
	if !ok {
		return StatuslineNotSet, nil
	}

	// Check if it's the pflow-managed command
	slMap, ok := sl.(map[string]any)
	if !ok {
		return StatuslineDifferent, nil
	}

	if slMap["type"] != "command" {
		return StatuslineDifferent, nil
	}

	cmd, _ := slMap["command"].(string)
	// Check that the command includes the session ID prefix pattern
	if strings.Contains(cmd, "sid8") && strings.Contains(cmd, "session_id") {
		return StatuslineOK, nil
	}

	return StatuslineDifferent, nil
}

// setupStatusline configures Claude's statusline to show the session ID prefix.
// If a statusline already exists but differs, returns an error describing the conflict.
// force=true overrides an existing statusline.
func setupStatusline(force bool) error {
	status, err := checkStatusline()
	if err != nil {
		return err
	}

	switch status {
	case StatuslineOK:
		return nil // already good
	case StatuslineNotSet:
		// Proceed with configuration
	case StatuslineDifferent:
		if !force {
			return fmt.Errorf("existing statusline found in ~/.claude/settings.json; use -force to override, or ensure the statusline shows the 8-char session ID prefix first (e.g. | sid8 | ...)")
		}
		// force=true: overwrite
	}

	path, err := claudeSettingsPath()
	if err != nil {
		return err
	}

	// Ensure ~/.claude/ exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("cannot create ~/.claude: %w", err)
	}

	// Read existing settings (or start fresh)
	var settings map[string]any
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			// Corrupt file — start fresh
			settings = make(map[string]any)
		}
	} else {
		settings = make(map[string]any)
	}

	settings["statusLine"] = map[string]any{
		"type":    "command",
		"command": statuslineCommand,
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal settings: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0644); err != nil {
		return fmt.Errorf("cannot write settings: %w", err)
	}
	return os.Rename(tmpPath, path)
}

// ── Claude session management ──────────────────────────────────────

// claudePrefixRegex matches the 8-char hex session ID prefix at the beginning
// of the status line. Format: "c50e1b2e | model | ctx ..."
var claudePrefixRegex = regexp.MustCompile(`(?m)^([a-f0-9]{8})\s*[| ]`)

// captureClaudePrefix uses tmux capture-pane to extract the 8-char Claude
// session ID prefix from the status line within a tmux session.
// Returns empty string if the prefix cannot be parsed.
func captureClaudePrefix(tmuxName string, maxWait time.Duration) (string, error) {
	deadline := time.Now().Add(maxWait)
	pane := tmuxName + ":0.0"

	for time.Now().Before(deadline) {
		out, err := exec.Command("tmux", "capture-pane", "-t", pane, "-p").Output()
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		matches := claudePrefixRegex.FindStringSubmatch(string(out))
		if len(matches) >= 2 {
			return matches[1], nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return "", nil // timeout — Claude may not have started yet
}

// ── Full Claude session startup ────────────────────────────────────

// StartClaudeSession creates a tmux session and starts Claude Code inside it.
// It mirrors the simplicity of a hand-written tmux script:
//
//	tmux new-session -d -s <name> -c <workDir>
//	tmux send-keys -t <name> "cd <workDir> && claude --permission-mode acceptEdits" C-m
//
// Statusline setup and session-prefix capture are best-effort post-creation
// steps — they don't block the tmux+Claude startup.
//
// Parameters:
//   - name: desired tmux session name (will be sanitized)
//   - workDir: working directory for the session
//   - forceStatusline: overwrite existing Claude statusline if needed
//
// Returns the created Session and the captured 8-char Claude prefix (may be
// empty if statusline isn't configured yet).
func (m *Manager) StartClaudeSession(name, workDir string, forceStatusline bool) (*Session, string, error) {
	// Only tmux is required for the core flow. ttyd and jq are optional
	// (needed for web terminal and statusline respectively, but not for
	// creating a tmux+Claude session).
	if _, err := exec.LookPath("tmux"); err != nil {
		return nil, "", fmt.Errorf("tmux is not installed (%w); please install it first", err)
	}

	// Resolve workDir
	absDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, "", fmt.Errorf("cannot resolve path %q: %w", workDir, err)
	}
	if info, err := os.Stat(absDir); err != nil || !info.IsDir() {
		return nil, "", fmt.Errorf("directory %q does not exist or is not a directory", absDir)
	}

	// Sanitize name
	if name == "" {
		name = sanitizeName(filepath.Base(absDir))
	} else {
		name = sanitizeName(name)
	}

	// Ensure uniqueness
	m.mu.Lock()
	name = m.uniqueName(name)
	m.mu.Unlock()

	// If tmux session already exists, just attach
	if tmuxSessionExists(name) {
		return nil, "", fmt.Errorf("tmux session %q already exists; attach with: tmux attach -t %s", name, name)
	}

	// 1. Create tmux session (matches mytmux pattern)
	create := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", absDir)
	if out, err := create.CombinedOutput(); err != nil {
		return nil, "", fmt.Errorf("tmux new-session failed: %w (output: %s)", err, string(out))
	}

	// 2. Start Claude inside tmux (matches mytmux: send-keys with claude command)
	claudeCmd := fmt.Sprintf("cd %s && claude --permission-mode acceptEdits", absDir)
	send := exec.Command("tmux", "send-keys", "-t", name, claudeCmd, "C-m")
	if out, err := send.CombinedOutput(); err != nil {
		exec.Command("tmux", "kill-session", "-t", name).Run()
		return nil, "", fmt.Errorf("failed to start claude in tmux %s: %w (output: %s)", name, err, string(out))
	}

	// 3. Best-effort: try to configure statusline and capture prefix.
	//    These don't block the session — Claude is already running.
	claudePrefix := ""
	if err := setupStatusline(forceStatusline); err != nil {
		fmt.Fprintf(os.Stderr, "pflow: statusline setup skipped: %v\n", err)
		fmt.Fprintf(os.Stderr, "pflow: dashboard terminal integration will not be available for this session.\n")
	} else {
		// Statusline is configured, wait for Claude to show it
		prefix, _ := captureClaudePrefix(name, 10*time.Second)
		if prefix != "" {
			claudePrefix = prefix
			// Save mapping for dashboard lookup
			if mm, err := newMappingManager(); err == nil {
				mm.addMapping(Mapping{
					TmuxName:     name,
					WorkDir:      absDir,
					ClaudePrefix: prefix,
					CreatedAt:    time.Now(),
				})
			}
		}
	}

	return &Session{
		Name:    name,
		WorkDir: absDir,
	}, claudePrefix, nil
}

// ── Tmux ↔ Claude lookup ──────────────────────────────────────────

// LookupResult describes the result of looking up a tmux session by
// Claude session ID.
type LookupResult struct {
	Session  *Session // nil if no match
	Verified bool     // true if live capture-pane confirmed the prefix match
	Warning  string   // non-empty when found but not verified (e.g., Claude in auth mode)
}

// LookupByClaudeSessionID finds a pflow-managed tmux session that is
// associated with the given Claude session ID. It matches by the first
// 8 characters of the session ID.
//
// If a saved mapping exists but live capture-pane verification fails
// (e.g., Claude is in auth mode and the statusline isn't visible),
// it still returns the mapping with Verified=false and a Warning.
func (m *Manager) LookupByClaudeSessionID(claudeSessionID string) (*LookupResult, error) {
	if len(claudeSessionID) < 8 {
		return nil, fmt.Errorf("session ID too short: %q", claudeSessionID)
	}
	prefix := claudeSessionID[:8]

	mm, err := newMappingManager()
	if err != nil {
		return nil, err
	}

	matches, err := mm.findByClaudePrefix(prefix)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return &LookupResult{}, nil // no match found
	}

	// Use the first match.
	for _, match := range matches {
		if !tmuxSessionExists(match.TmuxName) {
			continue
		}

		// Try live verification via capture-pane
		currentPrefix, _ := captureClaudePrefix(match.TmuxName, 2*time.Second)

		var sess *Session
		m.mu.Lock()
		existing, ok := m.sessions[match.TmuxName]
		m.mu.Unlock()
		if ok && m.isAlive(existing) {
			sess = existing
		} else {
			sess = &Session{
				Name:    match.TmuxName,
				WorkDir: match.WorkDir,
			}
		}

		if currentPrefix == prefix {
			// Live verification passed
			return &LookupResult{Session: sess, Verified: true}, nil
		}

		// Live verification failed — Claude might be in auth mode
		// (statusline not visible). Return the saved mapping with a warning.
		return &LookupResult{
			Session:  sess,
			Verified: false,
			Warning:  "Unable to live-verify the Claude session prefix (Claude may be in auth mode). The terminal session is based on saved data and may not match the current Claude session.",
		}, nil
	}

	return &LookupResult{}, nil // all sessions dead
}

