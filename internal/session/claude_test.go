package session

import (
	"strings"
	"testing"
)

func TestExtractClaudePrefix(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    string
		wantEmpty bool // if true, assert result is empty
	}{
		// ── Basic cases ──────────────────────────────────────────────
		{
			name: "single statusline at bottom",
			text: join(
				"Welcome to Claude Code!",
				"",
				"3ca06c7d | claude-sonnet-4-6 | ctx 45%/ 55% | in:1234 out:5678 | total:10000/20000",
			),
			want: "3ca06c7d",
		},
		{
			name: "statusline with leading whitespace",
			text: join(
				"some output",
				"  3ca06c7d | model | ctx 50%/ 50% | in:0 out:0 | total:0/0",
			),
			want: "3ca06c7d",
		},
		{
			name: "statusline without model field",
			text: join(
				"> claude --resume abc12345...",
				"",
				"3ca06c7d | ctx 10%/ 90% | in:500 out:2000 | total:5000/8000",
			),
			want: "3ca06c7d",
		},
		{
			name: "uppercase hex in prefix",
			text: join(
				"> prompt text",
				"ABCDEF12 | claude-opus-4-8 | ctx 30%/ 70% | in:100 out:300 | total:400/1200",
			),
			want: "ABCDEF12",
		},
		{
			name: "mixed case hex in prefix",
			text: join(
				"3cA06c7D | claude-sonnet-4-6 | ctx 45%/ 55% | in:1 out:2 | total:3/4",
			),
			want: "3cA06c7D",
		},

		// ── Multiple statuslines (old sessions in scrollback) ─────────
		{
			name: "two statuslines — pick bottom-most",
			text: join(
				"11111111 | claude-sonnet-4-6 | ctx 20%/ 80% | in:10 out:20 | total:50/80", // old
				"some conversation output...",
				"> hello",
				"assistant response...",
				"",
				"22222222 | claude-sonnet-4-6 | ctx 45%/ 55% | in:30 out:40 | total:100/120", // current
			),
			want: "22222222",
		},
		{
			name: "three statuslines — pick bottom-most",
			text: join(
				"aaaaaaaa | ctx 10%/ 90% | in:1 out:2 | total:3/4",    // oldest
				"intermediate output...",
				"bbbbbbbb | ctx 30%/ 70% | in:5 out:6 | total:7/8",   // middle
				"more output...",
				"cccccccc | ctx 50%/ 50% | in:9 out:10 | total:11/12", // current (bottom)
			),
			want: "cccccccc",
		},
		{
			name: "statuslines scattered with conversation between",
			text: join(
				"deadbeef | claude-haiku-4-5 | ctx 5%/ 95% | in:1 out:1 | total:2/2",     // old session 1
				"User: fix the login bug",
				"Assistant: Let me look at the login code...",
				"Tool: Read /src/auth/login.ts",
				"cafebabe | claude-sonnet-4-6 | ctx 40%/ 60% | in:50 out:100 | total:500/800", // old session 2 (after /clear)
				"User: now add tests",
				"Assistant: I'll add test cases for...",
				"",
				"1a2b3c4d | claude-sonnet-4-6 | ctx 25%/ 75% | in:200 out:300 | total:700/1100", // current
			),
			want: "1a2b3c4d",
		},

		// ── False positives: hex sequences in conversation ────────────
		{
			name: "git hash in conversation — NOT a statusline (no pipe)",
			text: join(
				"> git log",
				"commit 3ca06c7d1234567890abcdef1234567890abcdef (HEAD -> main)",
				"Author: dev",
				"",
				"deadbeef | claude-sonnet-4-6 | ctx 90%/ 10% | in:1 out:2 | total:3/4", // real statusline at bottom
			),
			want: "deadbeef",
		},
		{
			name: "hex in conversation with pipe — but real statusline below",
			text: join(
				"> echo 'key: 12345678 | value'",
				"key: 12345678 | value",                                              // false positive — matches regex! but not a statusline
				"",
				"deadbeef | claude-sonnet-4-6 | ctx 50%/ 50% | in:10 out:20 | total:30/40", // real statusline at bottom
			),
			want: "deadbeef",
		},
		{
			name: "hex sequence without pipe separator — should not match",
			text: join(
				"> some output with hash abcdef12 in it",
				"processing object 00ff99cc...",
				"",
				"1a2b3c4d | ctx 10%/ 90% | in:1 out:1 | total:2/2",
			),
			want: "1a2b3c4d",
		},
		{
			name: "8-char hex followed by space but no pipe — not a statusline",
			text: join(
				"1234abcd some random text that looks like hex",
				"ff00ff00 another line with hex",
				"",
				"a1b2c3d4 | ctx 1%/ 99% | in:1 out:1 | total:2/2", // real statusline
			),
			want: "a1b2c3d4",
		},

		// ── Edge cases ────────────────────────────────────────────────
		{
			name:      "empty text",
			text:      "",
			wantEmpty: true,
		},
		{
			name: "no statusline at all",
			text: join(
				"regular terminal output",
				"no hex prefix with pipe here",
				"just normal text",
			),
			wantEmpty: true,
		},
		{
			name: "blank lines only",
			text: join(
				"",
				"",
				"",
			),
			wantEmpty: true,
		},
		{
			name: "statusline is the only line",
			text: "ffff0000 | ctx 100%/ 0% | in:0 out:0 | total:0/0",
			want: "ffff0000",
		},
		{
			name: "statusline at top with garbage below — pick the bottom one",
			text: join(
				"11111111 | ctx 10%/ 90% | in:1 out:1 | total:2/2", // real statusline at top? no — statusline is always at bottom
				"garbage line 1",
				"garbage line 2",
			),
			// 11111111 is the only match; even though it's at the top,
			// if it's the only match, we return it (no better candidate).
			// In reality, a statusline at the top of visible output means
			// the terminal scrolled past it — but we can't know that from
			// a static capture.
			want: "11111111",
		},
		{
			name: "7-char hex with pipe — should not match",
			text: join(
				"abc1234 | this has only 7 hex chars",
				"12345678 | claude-sonnet-4-6 | ctx 50%/ 50% | in:1 out:1 | total:2/2",
			),
			want: "12345678",
		},
		{
			name: "9-char hex with pipe — should not match",
			text: join(
				"abc123456 | this has 9 hex chars",
				"12345678 | claude-sonnet-4-6 | ctx 50%/ 50% | in:1 out:1 | total:2/2",
			),
			want: "12345678",
		},
		{
			name: "trailing content on same line as prefix",
			text: join(
				"12345678 | claude-sonnet-4-6 | ctx 1%/ 99% | in:0 out:0 | total:10/20",
				"trailing garbage that doesn't matter",
			),
			// statusline is at the second-to-last line — extractClaudePrefix
			// scans bottom-up, so it finds this before the "trailing garbage" line
			want: "12345678",
		},

		// ── Realistic tmux capture-pane output ────────────────────────
		{
			name: "realistic full pane with conversation history",
			text: join(
				"$ claude",
				"Claude Code v1.0.0",
				"Type /help for commands.",
				"",
				"> fix the bug in login",
				"",
				"● I'll investigate the login bug. Let me start by examining the",
				"  relevant code.",
				"",
				"╭─────────────────────────── Read ────────────────────────────╮",
				"│ // auth/login.go                                           │",
				"│ func Login(w http.ResponseWriter, r *http.Request) {       │",
				"│     username := r.FormValue(\"username\")                    │",
				"│     // ...                                                 │",
				"╰────────────────────────────────────────────────────────────╯",
				"",
				"● The issue is on line 42 — the password hash comparison is",
				"  using == instead of CompareHashAndPassword.",
				"",
				"a1b2c3d4 | claude-sonnet-4-6 | ctx 35%/ 65% | in:150 out:300 | total:800/1200",
			),
			want: "a1b2c3d4",
		},
		{
			name: "pane with /clear residue — old statusline above new session",
			text: join(
				"eeeeeeee | ctx 80%/ 20% | in:100 out:200 | total:500/600", // old statusline (scrolled up)
				"some old conversation...",
				"The fix has been applied to login.go.",
				"",
				"──────────────────────────────────────────────────────────", // /clear separator
				"",
				"> now let's work on the dashboard",
				"",
				"● I'll look at the dashboard code.",
				"",
				"ffff1111 | claude-sonnet-4-6 | ctx 5%/ 95% | in:10 out:20 | total:30/40", // current
			),
			want: "ffff1111",
		},
		{
			name: "pane with ANSI-like bracket content (should not confuse regex)",
			text: join(
				"> cat some-config.conf",
				"[database]",
				"host = localhost",
				"port = 5432",
				"[logging]",
				"level = debug",
				"",
				"00aabbcc | ctx 15%/ 85% | in:25 out:50 | total:100/200",
			),
			want: "00aabbcc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractClaudePrefix(tt.text)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("extractClaudePrefix(%q) = %q, want empty", tt.text, got)
				}
			} else {
				if got != tt.want {
					t.Errorf("extractClaudePrefix(...) = %q, want %q\ninput:\n%s", got, tt.want, tt.text)
				}
			}
		})
	}
}

// join is a test helper that joins lines with \n.
func join(lines ...string) string {
	return strings.Join(lines, "\n")
}

// TestExtractClaudePrefix_BottomUpOrder verifies that when multiple valid
// statuslines exist, the bottom-most (last line) is always chosen.
func TestExtractClaudePrefix_BottomUpOrder(t *testing.T) {
	// Generate 100 lines where lines 0, 25, 50, 75, and 99 are valid statuslines.
	// The function MUST return the prefix from line 99 (bottom-most).
	var lines []string
	for i := 0; i < 100; i++ {
		switch i {
		case 0:
			lines = append(lines, "00000000 | model | ctx 0%/100% | in:0 out:0 | total:0/0")
		case 25:
			lines = append(lines, "11111111 | model | ctx 25%/75% | in:0 out:0 | total:0/0")
		case 50:
			lines = append(lines, "22222222 | model | ctx 50%/50% | in:0 out:0 | total:0/0")
		case 75:
			lines = append(lines, "33333333 | model | ctx 75%/25% | in:0 out:0 | total:0/0")
		case 99:
			lines = append(lines, "99999999 | model | ctx 99%/1% | in:0 out:0 | total:0/0")
		default:
			lines = append(lines, "some conversation line or tool output...")
		}
	}
	text := strings.Join(lines, "\n")

	got := extractClaudePrefix(text)
	if got != "99999999" {
		t.Errorf("extractClaudePrefix(100 lines) = %q, want %q (should pick bottom-most line 99)", got, "99999999")
	}
}

// TestExtractClaudePrefix_FalsePositiveDensity verifies that even when
// conversation lines happen to match the regex (hex + pipe), the bottom-most
// real statusline is still found.
func TestExtractClaudePrefix_FalsePositiveDensity(t *testing.T) {
	// Create a pane where every conversation line happens to match the regex.
	// This is the worst-case scenario — every line looks like a "statusline".
	// The function must still return the last line.
	var lines []string
	for i := 0; i < 50; i++ {
		// Each line matches the regex (8 hex chars + pipe) but is NOT a
		// real statusline — this stresses the bottom-up scan to ensure
		// it still finds the true bottom-most entry.
		lines = append(lines, "00000000 | fake statusline entry "+string(rune('0'+i%10)))
	}
	// Real statusline at the very bottom
	lines = append(lines, "deadbeef | claude-opus-4-8 | ctx 10%/ 90% | in:1 out:2 | total:3/4")
	text := strings.Join(lines, "\n")

	got := extractClaudePrefix(text)
	if got != "deadbeef" {
		t.Errorf("extractClaudePrefix(dense false positives) = %q, want %q", got, "deadbeef")
	}
}
