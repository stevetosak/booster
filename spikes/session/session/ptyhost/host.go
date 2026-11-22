// Package ptyhost is a session.Backend on PTYs: each session is a program on
// its own PTY whose output also feeds a terminal emulator (charmbracelet/x/vt),
// so a client that attaches later gets the screen as it is.
package ptyhost

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"

	"booster/spikes/session/session"
)

const hangup = syscall.SIGHUP

// Options configure a Host.
type Options struct {
	// Programs maps each kind to the command line it starts.
	Programs map[session.Kind][]string
	// Env is the base environment of every program; identity variables are
	// removed from it (see childEnv).
	Env []string
	// Scrollback is how many lines each session keeps above its screen.
	Scrollback int
	// Record, when set, makes a Recorder for each new session.
	Record func(session.Info) (Recorder, error)
}

// Host is the PTY session host.
type Host struct {
	opts Options

	mu       sync.Mutex
	sessions map[string]*ptySession
}

var _ session.Backend = (*Host)(nil)

func New(opts Options) *Host {
	if opts.Scrollback <= 0 {
		opts.Scrollback = 10000
	}
	return &Host{opts: opts, sessions: map[string]*ptySession{}}
}

func (h *Host) Create(spec session.Spec) (session.Info, error) {
	argv, ok := h.opts.Programs[spec.Kind]
	if !ok || len(argv) == 0 {
		return session.Info{}, fmt.Errorf("%w: %q", session.ErrUnknownKind, spec.Kind)
	}
	if st, err := os.Stat(spec.Dir); err != nil || !st.IsDir() {
		return session.Info{}, fmt.Errorf("the folder %q does not exist", spec.Dir)
	}
	if spec.Size.Cols == 0 || spec.Size.Rows == 0 {
		spec.Size = session.Size{Cols: 120, Rows: 36}
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = spec.Dir
	cmd.Env = childEnv(h.opts.Env)
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: spec.Size.Cols, Rows: spec.Size.Rows})
	if err != nil {
		return session.Info{}, fmt.Errorf("start %s: %w", argv[0], err)
	}

	info := session.Info{ID: newID(), Kind: spec.Kind, Dir: spec.Dir, Size: spec.Size, Started: time.Now()}
	rec := Recorder(nopRecorder{})
	if h.opts.Record != nil {
		if rec, err = h.opts.Record(info); err != nil {
			_ = cmd.Process.Kill()
			_ = f.Close()
			return session.Info{}, fmt.Errorf("record: %w", err)
		}
	}
	s := &ptySession{
		info:    info,
		pty:     f,
		cmd:     cmd,
		rec:     rec,
		scr:     newScreen(int(spec.Size.Cols), int(spec.Size.Rows), h.opts.Scrollback),
		clients: map[*attachment]struct{}{},
		done:    make(chan struct{}),
	}
	h.mu.Lock()
	h.sessions[info.ID] = s
	h.mu.Unlock()

	go s.answerQueries()
	go s.readOutput(func() {
		h.mu.Lock()
		delete(h.sessions, info.ID)
		h.mu.Unlock()
	})
	return info, nil
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

func (h *Host) Shutdown() {
	h.mu.Lock()
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

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
