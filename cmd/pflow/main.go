package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pancake-lee/pflow/internal/claude"
	"github.com/pancake-lee/pflow/internal/hermes"
)

func main() {
	window := 24 * time.Hour

	// Scan Claude Code sessions
	claudeResult, claudeErr := claude.Scan(window)
	// Scan Hermes sessions
	hermesResult, hermesErr := hermes.Scan(window)

	// Handle total failure
	if claudeErr != nil && hermesErr != nil {
		fmt.Fprintf(os.Stderr, "pflow: claude: %v\n", claudeErr)
		fmt.Fprintf(os.Stderr, "pflow: hermes: %v\n", hermesErr)
		os.Exit(1)
	}

	// Print header
	fmt.Printf("pflow — Agent Activity Dashboard (last %s)\n", window)
	fmt.Printf("Now: %s\n\n", time.Now().Format(time.DateTime))

	totalSessions := 0

	// ── Claude Code ──────────────────────────────────────────────
	if claudeErr == nil && len(claudeResult.Sessions) > 0 {
		fmt.Println("── Claude Code ──────────────────────────────────────────────────────")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SESSION ID\tPROJECT\tSTATUS\tPID\tMSGS\tLAST ACTIVE")

		for _, s := range claudeResult.Sessions {
			status := claudeStatusIcon(s)
			if s.IsRunning {
				status += " (alive)"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n",
				shortID(s.SessionID, 16),
				s.Project,
				status,
				s.PID,
				s.MessageCount,
				formatSince(s.LastActive),
			)
		}
		w.Flush()
		totalSessions += len(claudeResult.Sessions)
		fmt.Println()
	} else if claudeErr != nil {
		fmt.Printf("── Claude Code: %v ──────────────────────────────\n\n", claudeErr)
	} else {
		fmt.Println("── Claude Code: no active sessions ─────────────────────────────────")
	}

	// ── Hermes Agent ──────────────────────────────────────────────
	if hermesErr == nil && len(hermesResult.Sessions) > 0 {
		fmt.Println("── Hermes Agent ─────────────────────────────────────────────────────")
		if hermesResult.GatewayAlive {
			fmt.Println("Gateway: ✓ running")
		}
		fmt.Println()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SESSION ID\tPLATFORM\tSTATUS\tCHAT\tTOKENS\tLAST ACTIVE")

		for _, s := range hermesResult.Sessions {
			tokens := ""
			if s.InputTokens > 0 || s.OutputTokens > 0 {
				tokens = fmt.Sprintf("↓%d ↑%d", s.InputTokens, s.OutputTokens)
			}

			display := s.DisplayName
			if display == "" {
				display = s.ChatType
			}

			fmt.Fprintf(w, "%s\t%s %s\t%s\t%s\t%s\t%s\n",
				s.ShortID(),
				s.PlatformIcon(),
				s.Platform,
				s.StatusLabel(),
				display,
				tokens,
				formatSince(s.LastActive),
			)
		}
		w.Flush()
		totalSessions += len(hermesResult.Sessions)
		fmt.Println()
	} else if hermesErr != nil {
		fmt.Printf("── Hermes Agent: %v ─────────────────────────────────\n\n", hermesErr)
	} else {
		fmt.Println("── Hermes Agent: no active sessions ─────────────────────────────────")
	}

	// Footer
	fmt.Println(strings.Repeat("─", 70))
	fmt.Printf("%d active sessions total\n", totalSessions)
	fmt.Println("🟢 busy  🟡 waiting  ⚪ idle  ▶ running  ⏸ suspended")
	fmt.Println("(alive) = process is running  ↓↑ = token usage")
}

func claudeStatusIcon(s claude.SessionSummary) string {
	switch s.Status {
	case "busy":
		return "🟢 busy"
	case "waiting":
		return "🟡 waiting"
	case "idle":
		return "⚪ idle"
	default:
		return fmt.Sprintf("? %s", s.Status)
	}
}

func shortID(id string, n int) string {
	if len(id) <= n {
		return id
	}
	return id[:n]
}

func formatSince(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if hours == 1 && mins == 0 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh %dm ago", hours, mins)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}
