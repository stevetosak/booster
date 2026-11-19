package main

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// hangupGrace is how long a program gets to exit after SIGHUP before it is killed.
const hangupGrace = 2 * time.Second

// ptySession is one program on a pseudo-terminal.
type ptySession struct {
	cmd     *exec.Cmd
	file    *os.File
	exited  chan struct{}
	closing sync.Once
}

func startPTY(argv []string, dir string, cols, rows uint16) (*ptySession, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = terminalEnv(os.Environ())
	file, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: cols, Rows: rows})
	if err != nil {
		return nil, err
	}
	s := &ptySession{cmd: cmd, file: file, exited: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		close(s.exited)
	}()
	return s, nil
}

func (s *ptySession) Read(p []byte) (int, error)  { return s.file.Read(p) }
func (s *ptySession) Write(p []byte) (int, error) { return s.file.Write(p) }

func (s *ptySession) resize(cols, rows uint16) error {
	return pty.Setsize(s.file, &pty.Winsize{Cols: cols, Rows: rows})
}

// close hangs the program up, kills it if it does not exit, and releases the
// terminal. It is safe to call more than once.
func (s *ptySession) close() {
	s.closing.Do(func() {
		_ = s.cmd.Process.Signal(syscall.SIGHUP)
		select {
		case <-s.exited:
		case <-time.After(hangupGrace):
			_ = s.cmd.Process.Kill()
			<-s.exited
		}
		_ = s.file.Close()
	})
}

// inheritedPrefixes name the variables of the terminal or multiplexer that
// started the spike; a program in the IDE's own terminal must not see them.
var inheritedPrefixes = []string{"HERDR_", "TMUX", "CLAUDECODE=", "CLAUDE_CODE_", "TERM_PROGRAM", "COLORTERM=", "TERM="}

func terminalEnv(parent []string) []string {
	env := make([]string, 0, len(parent)+3)
	for _, kv := range parent {
		if !hasAnyPrefix(kv, inheritedPrefixes) {
			env = append(env, kv)
		}
	}
	return append(env, "TERM=xterm-256color", "COLORTERM=truecolor", "TERM_PROGRAM=booster")
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
