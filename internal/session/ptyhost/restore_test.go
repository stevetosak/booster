package ptyhost

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stevetosak/booster/internal/jsonfile"
	"github.com/stevetosak/booster/internal/session"
)

// The test agent prints how it was started, then echoes its input until
// Ctrl-D. The picky one cannot resume, as Claude Code cannot resume a session
// id with no conversation.
const (
	agentScript = `printf 'args:%s\n' "$*"; cat`
	pickyScript = `if [ "$1" = resume ]; then exit 3; fi; printf 'args:%s\n' "$*"; cat`
	kindAgent   = session.Kind("agent")
	kindPicky   = session.Kind("picky")
	kindShell   = session.Kind("shell")
)

// testSize is wide enough for a whole agent id on one line.
var testSize = session.Size{Cols: 80, Rows: 10}

func restoreOptions(t *testing.T) Options {
	t.Helper()
	agent := func(script string) Program {
		return Program{Argv: []string{"bash", "-c", script, "agent"}, Start: []string{"start", idArg}, Resume: []string{"resume", idArg}}
	}
	return Options{
		Programs: map[session.Kind]Program{
			kindAgent: agent(agentScript),
			kindPicky: agent(pickyScript),
			kindShell: {Argv: []string{"bash", "-c", `printf 'shell\n'; cat`}},
		},
		Env:       os.Environ(),
		StateFile: filepath.Join(t.TempDir(), "state", "sessions.json"),
	}
}

func create(t *testing.T, h *Host, kind session.Kind, dir string) session.Info {
	t.Helper()
	info, err := h.Create(session.Spec{Kind: kind, Dir: dir, Size: testSize})
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func savedIDs(t *testing.T, path string) []string {
	t.Helper()
	var f stateFile
	if _, err := jsonfile.Read(path, &f); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, r := range f.Sessions {
		ids = append(ids, r.ID)
	}
	return ids
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("not within 5 s: %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestProgramCommand(t *testing.T) {
	claude := Program{Argv: []string{"claude"}, Start: []string{"--session-id", idArg}, Resume: []string{"--resume", idArg}}
	pi := Program{Argv: []string{"pi"}, Start: []string{"--session-id", idArg}}
	cases := []struct {
		p      Program
		resume bool
		want   []string
	}{
		{claude, false, []string{"claude", "--session-id", "u"}},
		{claude, true, []string{"claude", "--resume", "u"}},
		{pi, true, []string{"pi", "--session-id", "u"}},
		{Program{Argv: []string{"bash", "-l"}}, true, []string{"bash", "-l"}},
	}
	for _, c := range cases {
		if got := c.p.command("u", c.resume); !reflect.DeepEqual(got, c.want) {
			t.Errorf("command(resume %v) = %q, want %q", c.resume, got, c.want)
		}
	}
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if id := newAgentID(); !uuid.MatchString(id) || id == newAgentID() {
		t.Errorf("newAgentID: %q", id)
	}
}

// TestSavedListFollowsTheSessions: a session the Operator closes and one
// whose program ends leave the list; Shutdown keeps it.
func TestSavedListFollowsTheSessions(t *testing.T) {
	opts := restoreOptions(t)
	h := New(opts)
	t.Cleanup(h.Shutdown)
	dir := t.TempDir()
	a := create(t, h, kindAgent, dir)
	b := create(t, h, kindShell, dir)
	c := create(t, h, kindAgent, dir)
	if a.Agent == "" || b.Agent != "" || a.Agent == c.Agent {
		t.Fatalf("agent ids: %q, %q, %q", a.Agent, b.Agent, c.Agent)
	}
	if got := savedIDs(t, opts.StateFile); !reflect.DeepEqual(got, []string{a.ID, b.ID, c.ID}) {
		t.Fatalf("saved %v", got)
	}
	if err := h.Close(b.ID); err != nil {
		t.Fatal(err)
	}
	waitSnapshot(t, h, c.ID, "args:start "+c.Agent).Detach()
	if err := h.Write(c.ID, []byte{4}); err != nil { // Ctrl-D: cat ends
		t.Fatal(err)
	}
	eventually(t, "only a stays saved", func() bool { return reflect.DeepEqual(savedIDs(t, opts.StateFile), []string{a.ID}) })

	h.Shutdown()
	if got := savedIDs(t, opts.StateFile); !reflect.DeepEqual(got, []string{a.ID}) {
		t.Fatalf("after Shutdown: saved %v, want [%s]", got, a.ID)
	}
}

func TestRestoreResumesAfterShutdown(t *testing.T) {
	opts := restoreOptions(t)
	first := New(opts)
	dir := t.TempDir()
	a := create(t, first, kindAgent, dir)
	s := create(t, first, kindShell, dir)
	first.Shutdown()

	next := New(opts)
	t.Cleanup(next.Shutdown)
	started, errs := next.Restore()
	if len(errs) != 0 || len(started) != 2 {
		t.Fatalf("Restore: %+v, %v", started, errs)
	}
	waitSnapshot(t, next, a.ID, "args:resume "+a.Agent).Detach()
	waitSnapshot(t, next, s.ID, "shell").Detach()
	list := next.List()
	if len(list) != 2 || list[0].ID != a.ID || list[0].Agent != a.Agent || !list[0].Started.Equal(a.Started) || list[1].ID != s.ID {
		t.Fatalf("List after Restore: %+v", list)
	}
}

func TestRestoreStartsNewWhenTheResumeFails(t *testing.T) {
	opts := restoreOptions(t)
	first := New(opts)
	p := create(t, first, kindPicky, t.TempDir())
	first.Shutdown()

	next := New(opts)
	t.Cleanup(next.Shutdown)
	if _, errs := next.Restore(); len(errs) != 0 {
		t.Fatal(errs)
	}
	eventually(t, "the agent starts new", func() bool {
		a, err := next.Attach(p.ID) // the failed resume may still be the one listed
		if err != nil {
			return false
		}
		defer a.Detach()
		return strings.Contains(string(a.Snapshot()), "args:start "+p.Agent)
	})
	if got := savedIDs(t, opts.StateFile); !reflect.DeepEqual(got, []string{p.ID}) {
		t.Fatalf("saved %v", got)
	}
}

func TestCloseAfterRestoreDoesNotRestart(t *testing.T) {
	opts := restoreOptions(t)
	first := New(opts)
	a := create(t, first, kindAgent, t.TempDir())
	first.Shutdown()

	next := New(opts)
	t.Cleanup(next.Shutdown)
	next.Restore()
	if err := next.Close(a.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the list is empty", func() bool { return len(next.List()) == 0 && len(savedIDs(t, opts.StateFile)) == 0 })
	time.Sleep(200 * time.Millisecond)
	if list := next.List(); len(list) != 0 {
		t.Fatalf("a closed session came back: %+v", list)
	}
}

func TestRestoreDropsWhatCannotStart(t *testing.T) {
	opts := restoreOptions(t)
	err := jsonfile.Write(opts.StateFile, stateFile{Version: stateVersion, Sessions: []record{
		{ID: "gone", Kind: kindShell, Dir: filepath.Join(t.TempDir(), "gone"), Size: testSize},
		{ID: "odd", Kind: "nosuch", Dir: t.TempDir(), Size: testSize},
	}})
	if err != nil {
		t.Fatal(err)
	}
	h := New(opts)
	t.Cleanup(h.Shutdown)
	if started, errs := h.Restore(); len(started) != 0 || len(errs) != 2 {
		t.Fatalf("Restore: %+v, %v", started, errs)
	}
	if got := savedIDs(t, opts.StateFile); len(got) != 0 {
		t.Fatalf("saved %v, want none", got)
	}
}
