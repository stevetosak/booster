package ptyhost

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"

	"github.com/stevetosak/booster/internal/session"
)

func newTestHost(t *testing.T, script string) (*Host, session.Info) {
	t.Helper()
	h := New(Options{
		Programs: map[session.Kind][]string{"test": {"bash", "-c", script}},
		Env:      os.Environ(),
	})
	t.Cleanup(h.Shutdown)
	info, err := h.Create(session.Spec{Kind: "test", Dir: t.TempDir(), Size: session.Size{Cols: 40, Rows: 10}})
	if err != nil {
		t.Fatal(err)
	}
	return h, info
}

// waitSnapshot attaches until the snapshot contains want.
func waitSnapshot(t *testing.T, h *Host, id, want string) session.Attachment {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		a, err := h.Attach(id)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(a.Snapshot()), want) {
			return a
		}
		a.Detach()
		if time.Now().After(deadline) {
			t.Fatalf("no %q in the snapshot:\n%q", want, a.Snapshot())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// readUntil reads the attachment's output until it contains want.
func readUntil(t *testing.T, a session.Attachment, want string) string {
	t.Helper()
	var got strings.Builder
	timeout := time.After(5 * time.Second)
	for !strings.Contains(got.String(), want) {
		select {
		case chunk, ok := <-a.Output():
			if !ok {
				t.Fatalf("output closed (%v) before %q; got %q", a.Err(), want, got.String())
			}
			got.Write(chunk)
		case <-timeout:
			t.Fatalf("no %q in the output; got %q", want, got.String())
		}
	}
	return got.String()
}

// TestSnapshotRestoresScreenAndScrollback writes the snapshot into a second
// emulator and compares every line with the session's own emulator.
func TestSnapshotRestoresScreenAndScrollback(t *testing.T) {
	h, info := newTestHost(t, `for i in $(seq 1 30); do printf '\033[1;3%dmline %d\033[0m\n' $((i%7)) $i; done; printf 'prompt> '; sleep 30`)
	a := waitSnapshot(t, h, info.ID, "prompt>")
	defer a.Detach()

	restored := vt.NewEmulator(int(a.Size().Cols), int(a.Size().Rows))
	if _, err := restored.Write(a.Snapshot()); err != nil {
		t.Fatal(err)
	}
	s, _ := h.get(info.ID)
	s.mu.Lock()
	defer s.mu.Unlock()
	orig := s.scr.emu
	if got, want := restored.ScrollbackLen(), orig.ScrollbackLen(); got != want {
		t.Errorf("scrollback has %d lines, want %d", got, want)
	}
	for i := 0; i < min(restored.ScrollbackLen(), orig.ScrollbackLen()); i++ {
		if got, want := restored.Scrollback().Line(i).Render(), orig.Scrollback().Line(i).Render(); got != want {
			t.Errorf("scrollback line %d = %q, want %q", i, got, want)
		}
	}
	if got, want := restored.Render(), orig.Render(); got != want {
		t.Errorf("screen differs:\n got %q\nwant %q", got, want)
	}
	if got, want := restored.CursorPosition(), orig.CursorPosition(); got != want {
		t.Errorf("cursor at %v, want %v", got, want)
	}
}

// TestOutputFollowsSnapshot checks that input reaches the program and that
// its answer arrives on the output after the snapshot.
func TestOutputFollowsSnapshot(t *testing.T) {
	h, info := newTestHost(t, `printf 'ready\n'; read -r line; printf 'got %s\n' "$line"; sleep 30`)
	a := waitSnapshot(t, h, info.ID, "ready")
	defer a.Detach()
	if err := h.Write(info.ID, []byte("hello\r")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, a, "got hello")
}

// TestPromptPastesInBrackets checks that an Action prompt arrives as one
// bracketed paste followed by Enter when the program asked for brackets.
func TestPromptPastesInBrackets(t *testing.T) {
	h, info := newTestHost(t, `stty raw -echo; printf '\033[?2004hready'; head -c 40 | od -An -c | tr -s ' '; sleep 30`)
	a := waitSnapshot(t, h, info.ID, "ready")
	defer a.Detach()
	if err := h.Prompt(info.ID, "line one\nline two", true); err != nil {
		t.Fatal(err)
	}
	// 6 + 17 + 6 + 1 bytes; head waits for 40, so pad with input.
	_ = h.Write(info.ID, []byte("0123456789"))
	out := strings.Join(strings.Fields(readUntil(t, a, "9")), "")
	for _, want := range []string{`033[200~lineone\nlinetwo033[201~\r0123456789`} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in the program's view of the input:\n%s", want, out)
		}
	}
}

// TestPromptRefusesMultilineWithoutBrackets checks that a multi-line prompt is
// refused when the program would read it line by line.
func TestPromptRefusesMultilineWithoutBrackets(t *testing.T) {
	h, info := newTestHost(t, `printf 'ready'; sleep 30`)
	waitSnapshot(t, h, info.ID, "ready").Detach()
	if err := h.Prompt(info.ID, "a\nb", true); err == nil {
		t.Fatal("a multi-line prompt without bracketed paste was sent")
	}
}

// TestQueryAnsweredWhileDetached checks that the emulator answers a cursor
// position query when no client is attached.
func TestQueryAnsweredWhileDetached(t *testing.T) {
	h, info := newTestHost(t, `stty -echo; printf 'ab\033[6n'; IFS= read -r -s -d R reply; printf '\ngot %s\n' "${reply:2}"; sleep 30`)
	waitSnapshot(t, h, info.ID, "got 1;3").Detach()
}

// TestSnapshotReplaysModes checks that modes the program set reach the client.
func TestSnapshotReplaysModes(t *testing.T) {
	h, info := newTestHost(t, `printf '\033[?2004h\033[?1h\033[?1000h\033[?1000l\033]2;agent title\007ready'; sleep 30`)
	a := waitSnapshot(t, h, info.ID, "ready")
	defer a.Detach()
	snap := string(a.Snapshot())
	for _, want := range []string{"\x1b[?2004h", "\x1b[?1h", "\x1b[?1000l", "\x1b]2;agent title\x07"} {
		if !strings.Contains(snap, want) {
			t.Errorf("no %q in the snapshot tail %q", want, snap[max(0, len(snap)-120):])
		}
	}
}

// TestEndClosesAttachment checks that a client learns that the program ended.
func TestEndClosesAttachment(t *testing.T) {
	h, info := newTestHost(t, `printf 'ready'; read -r x`)
	a := waitSnapshot(t, h, info.ID, "ready")
	_ = h.Write(info.ID, []byte("\r"))
	for range a.Output() {
	}
	if a.Err() != session.ErrEnded {
		t.Fatalf("Err() = %v, want ErrEnded", a.Err())
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(h.List()) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := len(h.List()); n != 0 {
		t.Fatalf("%d sessions listed after the end", n)
	}
}

func TestChildEnvDropsIdentity(t *testing.T) {
	env := childEnv([]string{"PATH=/bin", "CLAUDECODE=1", "CLAUDE_CODE_SESSION_ID=x", "CLAUDE_CODE_IDLE_COMPACT_MIN_TOKENS=5", "HERDR_PANE_ID=p", "TERM=screen"})
	got := strings.Join(env, " ")
	for _, gone := range []string{"CLAUDECODE", "CLAUDE_CODE_SESSION_ID", "HERDR_PANE_ID", "TERM=screen"} {
		if strings.Contains(got, gone) {
			t.Errorf("%s reached the child: %s", gone, got)
		}
	}
	for _, kept := range []string{"PATH=/bin", "CLAUDE_CODE_IDLE_COMPACT_MIN_TOKENS=5", "TERM=xterm-256color"} {
		if !strings.Contains(got, kept) {
			t.Errorf("%s is missing: %s", kept, got)
		}
	}
}
