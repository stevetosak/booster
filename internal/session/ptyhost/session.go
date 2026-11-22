package ptyhost

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creack/pty"

	"github.com/stevetosak/booster/internal/session"
)

// outputQueue is how many output chunks a client may fall behind before it is
// dropped and must attach again.
const outputQueue = 1024

// ptySession is one program on a PTY. Every output chunk goes to the emulator
// and to the attached clients under one lock, so a snapshot and the output
// after it never overlap or leave a gap.
type ptySession struct {
	info     session.Info
	pty      *os.File
	cmd      *exec.Cmd
	resumed  bool        // started with its agent resumed, after a core restart
	launched time.Time   // when this program started; Started keeps the session's first start
	closed   atomic.Bool // Close ended it, not the program itself

	mu      sync.Mutex
	scr     *screen
	clients map[*attachment]struct{}
	ended   bool

	attached atomic.Int32 // len(clients), read without the lock
	done     chan struct{}
}

func (s *ptySession) snapshotInfo() session.Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	info := s.info
	info.Title = s.scr.title
	info.Clients = len(s.clients)
	return info
}

// readOutput copies the program's output into the emulator and to the
// clients until the program ends, then calls onEnd with its exit error.
func (s *ptySession) readOutput(onEnd func(error)) {
	buf := make([]byte, 64*1024)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			s.mu.Lock()
			s.scr.write(chunk)
			for a := range s.clients {
				if !a.send(chunk) {
					delete(s.clients, a)
				}
			}
			s.attached.Store(int32(len(s.clients)))
			s.mu.Unlock()
		}
		if err != nil {
			break // EIO once the program and its children closed the PTY
		}
	}
	exitErr := s.cmd.Wait()
	s.mu.Lock()
	s.ended = true
	for a := range s.clients {
		a.end(session.ErrEnded)
	}
	clear(s.clients)
	s.attached.Store(0)
	s.mu.Unlock()
	// End answerQueries. Emulator.Close would race with its Read (x/vt
	// v0.1.0 keeps its closed flag in a plain bool), so close the reply pipe.
	if pw, ok := s.scr.emu.InputPipe().(*io.PipeWriter); ok {
		_ = pw.CloseWithError(io.EOF)
	}
	_ = s.pty.Close()
	close(s.done)
	onEnd(exitErr)
}

// answerQueries forwards the emulator's replies to terminal queries (cursor
// position, device attributes) to the program while no client is attached.
// An attached client's terminal answers on its own, so the emulator's reply is
// dropped then. The emulator writes its replies into a pipe that blocks until
// read, so this loop must run for the whole session.
func (s *ptySession) answerQueries() {
	buf := make([]byte, 4096)
	for {
		n, err := s.scr.emu.Read(buf)
		if n > 0 && s.attached.Load() == 0 {
			_, _ = s.pty.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func (s *ptySession) attach() (*attachment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return nil, session.ErrEnded
	}
	a := &attachment{
		snapshot: s.scr.snapshot(),
		size:     s.info.Size,
		out:      make(chan []byte, outputQueue),
	}
	a.detach = func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.clients[a]; ok {
			delete(s.clients, a)
			s.attached.Store(int32(len(s.clients)))
			a.end(nil)
		}
	}
	s.clients[a] = struct{}{}
	s.attached.Store(int32(len(s.clients)))
	return a, nil
}

func (s *ptySession) write(p []byte) error {
	_, err := s.pty.Write(p)
	return err
}

func (s *ptySession) resize(size session.Size) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := pty.Setsize(s.pty, &pty.Winsize{Cols: size.Cols, Rows: size.Rows}); err != nil {
		return err
	}
	s.scr.emu.Resize(int(size.Cols), int(size.Rows))
	s.info.Size = size
	return nil
}

// submitDelay separates a pasted prompt from the Enter that submits it, so an
// agent that reads the paste and the Enter in one chunk does not take the
// Enter as part of the paste.
var submitDelay = 80 * time.Millisecond

// prompt types an Action prompt into the program as a paste: in brackets when
// the program asked for them, so a multi-line prompt arrives whole.
func (s *ptySession) prompt(text string, submit bool) error {
	s.mu.Lock()
	bracketed := s.scr.bracketedPaste()
	s.mu.Unlock()
	if !bracketed && submit && containsNewline(text) {
		return errors.New("the program did not turn on bracketed paste, so a multi-line prompt would submit at its first line")
	}
	paste := text
	if bracketed {
		paste = "\x1b[200~" + text + "\x1b[201~"
	}
	if err := s.write([]byte(paste)); err != nil {
		return err
	}
	if !submit {
		return nil
	}
	time.Sleep(submitDelay)
	return s.write([]byte("\r"))
}

func containsNewline(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' || s[i] == '\r' {
			return true
		}
	}
	return false
}

func (s *ptySession) close() error {
	s.closed.Store(true)
	if s.cmd.Process == nil {
		return nil
	}
	// SIGHUP, as a terminal that closes sends it; the read loop sees the end.
	if err := s.cmd.Process.Signal(hangup); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	select {
	case <-s.done:
	case <-time.After(3 * time.Second):
		log.Printf("session %s: no exit 3 s after SIGHUP, killing it", s.info.ID)
		_ = s.cmd.Process.Kill()
	}
	return nil
}

// attachment is one client of a session.
type attachment struct {
	snapshot []byte
	size     session.Size
	out      chan []byte
	detach   func()

	once sync.Once
	err  atomic.Value // error
}

func (a *attachment) Snapshot() []byte      { return a.snapshot }
func (a *attachment) Size() session.Size    { return a.size }
func (a *attachment) Output() <-chan []byte { return a.out }
func (a *attachment) Detach()               { a.detach() }
func (a *attachment) Err() error {
	if err, ok := a.err.Load().(error); ok {
		return err
	}
	return nil
}

// send queues a chunk. A client that fell outputQueue chunks behind is ended
// with ErrTooSlow, and send reports false so the session drops it. The
// session's lock is held.
func (a *attachment) send(chunk []byte) bool {
	select {
	case a.out <- chunk:
		return true
	default:
		a.end(session.ErrTooSlow)
		return false
	}
}

// end closes the output once; err nil means the client detached.
func (a *attachment) end(err error) {
	a.once.Do(func() {
		if err != nil {
			a.err.Store(err)
		}
		close(a.out)
	})
}
