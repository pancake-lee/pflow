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

	plogger "github.com/pancake-lee/pgo/pkg/plogger"
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

// claudePrefixRegex matches the 8-char hex session ID prefix in the status line.
// Format: "  3ca06c7d | model | ctx ..." (may have leading whitespace).
var claudePrefixRegex = regexp.MustCompile(`(?m)^\s*([a-f0-9]{8})\s*[| ]`)

// captureClaudePrefix uses tmux capture-pane to extract the 8-char Claude
// session ID prefix from the status line within a tmux session.
// Returns empty string if the prefix cannot be parsed within maxWait.
func captureClaudePrefix(tmuxName string, maxWait time.Duration) (string, error) {
	deadline := time.Now().Add(maxWait)
	pane := tmuxName + ":0.0"
	attempt := 0

	for time.Now().Before(deadline) {
		attempt++
		out, err := exec.Command("tmux", "capture-pane", "-t", pane, "-p").Output()
		if err != nil {
			plogger.Debugf("captureClaudePrefix: attempt %d: capture-pane error: %v", attempt, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		text := string(out)
		matches := claudePrefixRegex.FindStringSubmatch(text)
		if len(matches) >= 2 {
			plogger.Debugf("captureClaudePrefix: attempt %d: found prefix=%s", attempt, matches[1])
			return matches[1], nil
		}

		// Log first attempt and every 5th to see what's on screen
		if attempt == 1 || attempt%5 == 0 {
			lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
			total := len(lines)
			// Capture last 10 lines — statusline might need more context
			tail := lines
			if len(tail) > 10 {
				tail = tail[len(tail)-10:]
			}
			plogger.Debugf("captureClaudePrefix: attempt %d: no prefix match, total=%d lines, tail %d: %q",
				attempt, total, len(tail), strings.Join(tail, "\\n"))
		}

		time.Sleep(500 * time.Millisecond)
	}

	plogger.Debugf("captureClaudePrefix: timeout after %d attempts (%.1fs)", attempt, maxWait.Seconds())
	return "", nil
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

	// 1. Configure statusline BEFORE starting Claude.
	//    Claude reads ~/.claude/settings.json at startup — if we write it
	//    after Claude starts, the statusline won't appear until Claude restarts.
	statuslineReady := false
	status, err := checkStatusline()
	if err != nil {
		plogger.Warnf("StartClaudeSession: cannot check statusline: %v", err)
	} else {
		plogger.Debugf("StartClaudeSession: statusline status=%d (0=OK, 1=NotSet, 2=Different)", status)
		if err := setupStatusline(forceStatusline); err != nil {
			plogger.Warnf("StartClaudeSession: statusline setup skipped: %v", err)
		} else {
			statuslineReady = true
			plogger.Debugf("StartClaudeSession: statusline is ready")
		}
	}

	// 2. Create tmux session (matches mytmux pattern)
	plogger.Debugf("StartClaudeSession: creating tmux session name=%s workDir=%s", name, absDir)
	create := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", absDir)
	if out, err := create.CombinedOutput(); err != nil {
		return nil, "", fmt.Errorf("tmux new-session failed: %w (output: %s)", err, string(out))
	}

	// 3. Start Claude inside tmux (matches mytmux: send-keys with claude command)
	//    Claude picks up the statusline from settings.json at startup.
	claudeCmd := fmt.Sprintf("cd %s && claude --permission-mode acceptEdits", absDir)
	plogger.Debugf("StartClaudeSession: starting claude command in tmux %s", name)
	send := exec.Command("tmux", "send-keys", "-t", name, claudeCmd, "C-m")
	if out, err := send.CombinedOutput(); err != nil {
		exec.Command("tmux", "kill-session", "-t", name).Run()
		return nil, "", fmt.Errorf("failed to start claude in tmux %s: %w (output: %s)", name, err, string(out))
	}

	// 4. Launch async prefix capture in background.
	//    Claude's statusline takes a few seconds to render — we return
	//    immediately so the user can attach to tmux without waiting.
	//    The mapping is saved asynchronously when the capture succeeds.
	if statuslineReady {
		tmuxName := name
		absWorkDir := absDir
		go func() {
			plogger.Debugf("StartClaudeSession: async capture started for tmux %s (max 10s)", tmuxName)
			start := time.Now()
			prefix, _ := captureClaudePrefix(tmuxName, 10*time.Second)
			elapsed := time.Since(start)
			if prefix != "" {
				plogger.Infof("StartClaudeSession: async captured prefix=%s for tmux=%s (took %.1fs)", prefix, tmuxName, elapsed.Seconds())
				if mm, err := newMappingManager(); err == nil {
					mm.addMapping(Mapping{
						TmuxName:     tmuxName,
						WorkDir:      absWorkDir,
						ClaudePrefix: prefix,
						CreatedAt:    time.Now(),
					})
				}
			} else {
				plogger.Warnf("StartClaudeSession: async capture timeout for tmux=%s after %.1fs", tmuxName, elapsed.Seconds())
			}
		}()
	} else {
		plogger.Warn("StartClaudeSession: statusline not ready, skipping prefix capture")
	}

	return &Session{
		Name:    name,
		WorkDir: absDir,
	}, "", nil
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
	plogger.Debugf("lookup: searching for claude session prefix=%s (full=%s)", prefix, claudeSessionID)

	mm, err := newMappingManager()
	if err != nil {
		return nil, err
	}

	matches, err := mm.findByClaudePrefix(prefix)
	if err != nil {
		return nil, err
	}
	plogger.Debugf("lookup: prefix=%s found %d saved mapping(s)", prefix, len(matches))
	if len(matches) == 0 {
		plogger.Infof("lookup: no mapping for prefix=%s — session may not be pflow-managed", prefix)
		return &LookupResult{}, nil
	}

	// Walk matches and find the first with a living tmux session.
	for _, match := range matches {
		tmuxAlive := tmuxSessionExists(match.TmuxName)
		plogger.Debugf("lookup: checking match tmux=%s prefix=%s tmuxAlive=%v", match.TmuxName, match.ClaudePrefix, tmuxAlive)
		if !tmuxAlive {
			plogger.Infof("lookup: skipping dead tmux session %s (prefix=%s)", match.TmuxName, match.ClaudePrefix)
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
			plogger.Infof("lookup: VERIFIED tmux=%s prefix=%s (live capture matches)", match.TmuxName, currentPrefix)
			return &LookupResult{Session: sess, Verified: true}, nil
		}

		// Live verification failed — Claude might be in auth mode
		// (statusline not visible). Return the saved mapping with a warning.
		plogger.Infof("lookup: UNVERIFIED tmux=%s saved_prefix=%s current_prefix=%q — returning mapping with warning",
			match.TmuxName, prefix, currentPrefix)
		return &LookupResult{
			Session:  sess,
			Verified: false,
			Warning:  "Unable to live-verify the Claude session prefix (Claude may be in auth mode). The terminal session is based on saved data and may not match the current Claude session.",
		}, nil
	}

	plogger.Infof("lookup: all %d mapping(s) for prefix=%s have dead tmux sessions", len(matches), prefix)
	return &LookupResult{}, nil // all sessions dead
}

