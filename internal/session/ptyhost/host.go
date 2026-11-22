// Package ptyhost is a session.Backend on PTYs: each session is a program on
// its own PTY whose output also feeds a terminal emulator (charmbracelet/x/vt),
// so a client that attaches later gets the screen as it is. The programs end
// with the core, so the host keeps the session list in a file and the next
// core starts each session again, its agent resumed.
package ptyhost

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"

	"github.com/stevetosak/booster/internal/session"
)

const hangup = syscall.SIGHUP

// Options configure a Host.
type Options struct {
	// Programs maps each kind to the program it starts.
	Programs map[session.Kind]Program
	// Env is the base environment of every program; identity variables are
	// removed from it (see childEnv).
	Env []string
	// Scrollback is how many lines each session keeps above its screen.
	Scrollback int
	// StateFile keeps the session list for the next core; "" keeps none.
	StateFile string
}

// Host is the PTY session host.
type Host struct {
	opts Options

	mu       sync.Mutex
	sessions map[string]*ptySession
	stopping bool // Shutdown runs: the saved list stays for the next core
}

// resumeGrace is how soon after a resume an agent that ends with an error is
// taken as one that could not resume. Claude Code ends at once when its
// session id has no conversation, as when no message was sent before the
// restart; the host then starts it new with the same id.
const resumeGrace = 10 * time.Second

var _ session.Backend = (*Host)(nil)

func New(opts Options) *Host {
	if opts.Scrollback <= 0 {
		opts.Scrollback = 10000
	}
	return &Host{opts: opts, sessions: map[string]*ptySession{}}
}

func (h *Host) Create(spec session.Spec) (session.Info, error) {
	if spec.Size.Cols == 0 || spec.Size.Rows == 0 {
		spec.Size = session.Size{Cols: 120, Rows: 36}
	}
	r := record{ID: newID(), Kind: spec.Kind, Dir: spec.Dir, Size: spec.Size, Started: time.Now()}
	if h.opts.Programs[spec.Kind].hasSession() {
		r.Agent = newAgentID()
	}
	info, err := h.start(r, false)
	if err == nil {
		h.save()
	}
	return info, err
}

// Restore starts each session of the list the last core saved again, with
// the same id and its agent resumed. A session that cannot start leaves the
// list; Restore returns why. The core calls it once, holding its lock,
// before any Create.
func (h *Host) Restore() ([]session.Info, []error) {
	list, err := h.load()
	if err != nil {
		return nil, []error{err}
	}
	var started []session.Info
	var errs []error
	for _, r := range list {
		info, err := h.start(r, true)
		if err != nil {
			errs = append(errs, fmt.Errorf("session %s (%s in %s): %w", r.ID, r.Kind, r.Dir, err))
			continue
		}
		started = append(started, info)
	}
	h.save()
	return started, errs
}

// start runs the program of a session: new, or with its agent resumed.
func (h *Host) start(r record, resume bool) (session.Info, error) {
	prog, ok := h.opts.Programs[r.Kind]
	if !ok || len(prog.Argv) == 0 {
		return session.Info{}, fmt.Errorf("%w: %q", session.ErrUnknownKind, r.Kind)
	}
	if st, err := os.Stat(r.Dir); err != nil || !st.IsDir() {
		return session.Info{}, fmt.Errorf("the folder %q does not exist", r.Dir)
	}
	argv := prog.command(r.Agent, resume)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = r.Dir
	cmd.Env = childEnv(h.opts.Env)
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: r.Size.Cols, Rows: r.Size.Rows})
	if err != nil {
		return session.Info{}, fmt.Errorf("start %s: %w", argv[0], err)
	}

	info := r.info()
	s := &ptySession{
		info:     info,
		pty:      f,
		cmd:      cmd,
		resumed:  resume,
		launched: time.Now(),
		scr:      newScreen(int(r.Size.Cols), int(r.Size.Rows), h.opts.Scrollback),
		clients:  map[*attachment]struct{}{},
		done:     make(chan struct{}),
	}
	h.mu.Lock()
	h.sessions[info.ID] = s
	h.mu.Unlock()

	go s.answerQueries()
	go s.readOutput(func(exitErr error) { h.ended(s, exitErr) })
	return info, nil
}

// ended takes a session whose program ended off the list; during Shutdown
// save keeps the file as it was. An agent that could not resume starts new
// with the same ids, in the same place in the list; a client attached to the
// failed resume sees it end and attaches again. Close marks a session
// closed, so a closed agent never starts new.
func (h *Host) ended(s *ptySession, exitErr error) {
	info := s.snapshotInfo()
	h.mu.Lock()
	current := h.sessions[info.ID] == s
	h.mu.Unlock()
	if !current {
		return
	}
	if s.resumed && info.Agent != "" && !s.closed.Load() && exitErr != nil && time.Since(s.launched) < resumeGrace {
		log.Printf("session %s: the agent did not resume (%v); starting it new", info.ID, exitErr)
		_, err := h.start(recordOf(info), false)
		if err == nil {
			return
		}
		log.Printf("session %s: %v", info.ID, err)
	}
	h.mu.Lock()
	if h.sessions[info.ID] == s {
		delete(h.sessions, info.ID)
	}
	h.mu.Unlock()
	h.save()
}

func (h *Host) List() []session.Info {
	h.mu.Lock()
	all := make([]*ptySession, 0, len(h.sessions))
	for _, s := range h.sessions {
		all = append(all, s)
	}
	h.mu.Unlock()
	out := make([]session.Info, 0, len(all))
	for _, s := range all {
		out = append(out, s.snapshotInfo())
	}
	slices.SortFunc(out, func(a, b session.Info) int { return a.Started.Compare(b.Started) })
	return out
}

func (h *Host) Attach(id string) (session.Attachment, error) {
	s, err := h.get(id)
	if err != nil {
		return nil, err
	}
	return s.attach()
}

func (h *Host) Write(id string, input []byte) error {
	s, err := h.get(id)
	if err != nil {
		return err
	}
	return s.write(input)
}

func (h *Host) Resize(id string, size session.Size) error {
	if size.Cols == 0 || size.Rows == 0 {
		return fmt.Errorf("size %dx%d is empty", size.Cols, size.Rows)
	}
	s, err := h.get(id)
	if err != nil {
		return err
	}
	return s.resize(size)
}

func (h *Host) Prompt(id string, text string, submit bool) error {
	s, err := h.get(id)
	if err != nil {
		return err
	}
	return s.prompt(text, submit)
}

func (h *Host) Close(id string) error {
	s, err := h.get(id)
	if err != nil {
		return err
	}
	return s.close()
}

// Shutdown ends every session and keeps the saved list for the next core.
func (h *Host) Shutdown() {
	h.mu.Lock()
	h.stopping = true
	all := make([]*ptySession, 0, len(h.sessions))
	for _, s := range h.sessions {
		all = append(all, s)
	}
	h.mu.Unlock()
	var wg sync.WaitGroup
	for _, s := range all {
		wg.Go(func() { _ = s.close() })
	}
	wg.Wait()
}

func (h *Host) get(id string) (*ptySession, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.sessions[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", session.ErrNoSession, id)
	}
	return s, nil
}
