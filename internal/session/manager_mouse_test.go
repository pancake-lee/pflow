package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigureTmuxMouseIsolatesManagedBindings(t *testing.T) {
	_, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is not installed")
	}
	socket := filepath.Join(t.TempDir(), "tmux.sock")
	run := func(args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		args = append([]string{"-S", socket}, args...)
		out, err := exec.CommandContext(ctx, "tmux", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("tmux %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	t.Cleanup(func() {
		exec.Command("tmux", "-S", socket, "kill-server").Run()
	})
	run("-f", "/dev/null", "new-session", "-d", "-s", "managed", "sleep 120")
	run("new-session", "-d", "-s", "ordinary", "sleep 120")
	run("set-option", "-g", "mouse", "off")
	run("bind-key", "-T", "root", "WheelUpPane", "copy-mode", "-e")
	run("bind-key", "-r", "-T", "root", "F12", "display-message",
		`user shortcut with "quotes"; -T root stays literal`)
	rootBefore := run("list-keys", "-T", "root")
	prefixBefore := run("list-keys", "-T", "prefix")

	err = configureTmuxMouse("managed", "-S", socket)
	if err != nil {
		t.Fatal(err)
	}
	if got := run("list-keys", "-T", "root"); got != rootBefore {
		t.Fatal("user root bindings changed")
	}
	if got := run("list-keys", "-T", "prefix"); got != prefixBefore {
		t.Fatal("user prefix bindings changed")
	}
	for _, option := range []struct{ session, key, want string }{
		{"managed", "key-table", managedKeyTable},
		{"managed", "mouse", "on"},
		{"ordinary", "key-table", "root"},
		{"ordinary", "mouse", "off"},
	} {
		got := run("show-options", "-A", "-v", "-t", option.session, option.key)
		if got != option.want {
			t.Errorf("%s %s = %q, want %q",
				option.session, option.key, got, option.want)
		}
	}
	shortcut := run("list-keys", "-T", managedKeyTable)
	if !strings.Contains(shortcut, "-r") ||
		!strings.Contains(shortcut, "-T root stays literal") {
		t.Fatalf("custom shortcut not preserved: %s", shortcut)
	}

	// Reconfiguration refreshes the snapshot and removes stale custom keys.
	run("unbind-key", "-T", "root", "F12")
	err = configureTmuxMouse("managed", "-S", socket)
	if err != nil {
		t.Fatal(err)
	}
	if got := run("list-keys", "-T", managedKeyTable); strings.Contains(got, "F12") {
		t.Fatalf("deleted shortcut survived refresh: %s", got)
	}
	t.Run("terminal mouse and keyboard events", func(t *testing.T) {
		python, err := exec.LookPath("python3")
		if err != nil {
			t.Skip("Python PTY driver is not installed")
		}
		dir := t.TempDir()
		script := filepath.Join(dir, "mouse.py")
		log := filepath.Join(dir, "input")
		err = os.WriteFile(script, []byte(tmuxMouseDriver), 0600)
		if err != nil {
			t.Fatal(err)
		}
		run("bind-key", "-T", "root", "F12", "set-option",
			"-t", "managed", "@shortcut", "received")
		err = configureTmuxMouse("managed", "-S", socket)
		if err != nil {
			t.Fatal(err)
		}
		// Only test-generated paths are interpolated, with shell quoting.
		quote := func(s string) string {
			return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
		}
		command := quote(python) + " " + quote(script) + " pane " + quote(log)
		run("respawn-pane", "-k", "-t", "managed", command)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, python, script, "drive", socket, log)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("terminal event verification: %v\n%s", err, out)
		}
		t.Log(strings.TrimSpace(string(out)))
	})
	t.Run("empty root table", func(t *testing.T) {
		run("unbind-key", "-a", "-T", "root")
		err := configureTmuxMouse("managed", "-S", socket)
		if err != nil {
			t.Fatal(err)
		}
		bindings := run("list-keys", "-T", managedKeyTable)
		if !strings.Contains(bindings, "WheelUpPane") ||
			!strings.Contains(bindings, "WheelDownPane") {
			t.Fatalf("wheel dispatch missing: %s", bindings)
		}
		if got := run("list-keys", "-T", "root"); got != "" {
			t.Fatal("empty user table was modified")
		}
		if got := run("list-keys", "-T", "prefix"); got != prefixBefore {
			t.Fatal("prefix table was modified")
		}
	})
	err = configureTmuxMouse("missing", "-S", socket)
	if err == nil || !strings.Contains(err.Error(), "configure mouse") {
		t.Fatalf("missing session error = %v", err)
	}
}

// A real PTY client exercises tmux's event dispatch, not just binding text.
// Python is optional test tooling; production uses only tmux and Go.
const tmuxMouseDriver = `import os, pty, select, subprocess, sys, time, tty

if sys.argv[1] == "pane":
    tty.setraw(0)
    log = open(sys.argv[2], "ab", buffering=0)
    os.write(1, b"\x1b[?1049h\x1b[?1000h\x1b[?1006hready\r\n")
    while True:
        data = os.read(0, 4096)
        log.write(data)
        if b"disable" in data:
            os.write(1, b"\x1b[?1000l\x1b[?1006l\x1b[?1049l")
            os.write(1, b"history line\r\n" * 100)
else:
    socket, log = sys.argv[2:]
    base = ["tmux", "-S", socket]
    def run(*args):
        return subprocess.check_output(base + list(args), text=True).strip()
    def wait_for(check):
        deadline = time.monotonic() + 3
        while time.monotonic() < deadline:
            drain()
            if check():
                return
            time.sleep(.02)
        raise AssertionError("timed out waiting for terminal state")
    def read_log():
        return open(log, "rb").read() if os.path.exists(log) else b""
    def state(format):
        return run("display-message", "-p", "-t", "managed", format)
    master, slave = pty.openpty()
    env = dict(os.environ, TERM="xterm-256color")
    env.pop("TMUX", None)
    client = subprocess.Popen(base + ["attach-session", "-t", "managed"],
                              stdin=slave, stdout=slave, stderr=slave, env=env)
    os.close(slave)
    def drain():
        while select.select([master], [], [], 0)[0]:
            os.read(master, 65536)
    def send(data):
        os.write(master, data)
        time.sleep(.1)
        drain()
    try:
        wait_for(lambda: state("#{mouse_any_flag}") == "1")
        time.sleep(.2)
        drain()
        up, down = b"\x1b[<64;10;5M", b"\x1b[<65;10;5M"
        send(up)
        send(down)
        wait_for(lambda: up in read_log() and down in read_log())
        assert state("#{pane_in_mode}") == "0", "wheel entered copy mode"
        send(b"\x1b[24~")
        wait_for(lambda: run("show-options", "-v", "-t", "managed", "@shortcut") == "received")
        send(b"plain text")
        wait_for(lambda: b"plain text" in read_log())
        send(b"disable")
        wait_for(lambda: state("#{mouse_any_flag}") == "0")
        send(up)
        wait_for(lambda: state("#{pane_in_mode}") == "1")
        position = int(state("#{scroll_position}"))
        send(up)
        wait_for(lambda: int(state("#{scroll_position}")) > position)
        send(down)
        wait_for(lambda: int(state("#{scroll_position}") or "0") == position)
        os.write(master, b"\x02d")
        client.wait(timeout=3)
        assert client.returncode == 0, "prefix detach failed"
        print("PASS: wheel passthrough, plain keys, custom shortcut, copy scrolling, prefix detach")
    finally:
        if client.poll() is None:
            client.terminate()
            client.wait(timeout=3)
        os.close(master)
`
