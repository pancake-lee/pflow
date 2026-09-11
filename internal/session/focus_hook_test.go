package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureFocusEventsConfigPreservesExistingSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.conf")
	for _, content := range []string{
		"set -g status on\n",
		"set -g focus-events on\nset -g status on\n",
		"set -g focus-events off\nset -g status on\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if err := ensureFocusEventsConfig(path); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(content, "focus-events") {
			if string(got) != content {
				t.Fatalf("existing config changed:\n%s", got)
			}
		} else if !strings.Contains(string(got), "set -g focus-events on") || !strings.Contains(string(got), "set -g status on") {
			t.Fatalf("missing added or preserved setting:\n%s", got)
		}
	}
}

func TestEnsureFocusEventsConfigCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tmux.conf")
	if err := ensureFocusEventsConfig(path); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# Added by pflow for accurate active-time tracking.\nset -g focus-events on\n" {
		t.Fatalf("unexpected config: %q", got)
	}
}

func TestFindHookEntriesOnlySelectsPflowHook(t *testing.T) {
	hooks := `client-focus-in[0] run-shell "echo user"
client-focus-in[1] run-shell "case '#{session_name}' in pflow-*) /tmp/focus-log.sh '#{session_name}' ;; esac"
client-focus-out[0] run-shell "case '#{session_name}' in pflow-*) /tmp/focus-log-out.sh '#{session_name}' ;; esac"
`
	got := findHookEntries("client-focus-in", "/tmp/focus-log.sh", hooks)
	if len(got) != 1 || got[0] != "client-focus-in[1]" {
		t.Fatalf("entries=%v", got)
	}
}
