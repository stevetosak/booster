// Package session defines what booster's core needs from a session host: it
// owns each agent's terminal, keeps its screen while no window shows it, and
// takes input and Action prompts. The PTY host (package ptyhost) is the first
// Backend; tmux may become a second one.
package session

import (
	"errors"
	"time"
)

// Kind names a program the core may start. A client names a kind, never a
// command line, so a page cannot make the core run an arbitrary program.
type Kind string

const (
	KindClaude Kind = "claude"
	KindPi     Kind = "pi"
	KindShell  Kind = "shell"
)

// Spec is what a client asks for when it creates a session.
type Spec struct {
	Kind Kind   `json:"kind"`
	Dir  string `json:"dir"`
	Size Size   `json:"size"`
}

// Size is a terminal size in cells.
type Size struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// Info describes one live session.
type Info struct {
	ID      string    `json:"id"`
	Kind    Kind      `json:"kind"`
	Dir     string    `json:"dir"`
	Agent   string    `json:"agent,omitempty"` // the agent's own session id (Claude Code, pi)
	Title   string    `json:"title"`
	Size    Size      `json:"size"`
	Started time.Time `json:"started"`
	Clients int       `json:"clients"`
}

// Attachment is one client's view of a session: the screen as it is now, then
// every byte the program writes after it, with no gap and no overlap.
type Attachment interface {
	// Snapshot is the screen, the scrollback and the terminal modes as one
	// ANSI stream, to write into an empty terminal of Size.
	Snapshot() []byte
	// Size is the terminal size the snapshot was made for.
	Size() Size
	// Output carries the program's output after the snapshot. It is closed
	// when the session ends or when the client reads too slowly; Err tells
	// which.
	Output() <-chan []byte
	// Err is nil while Output is open.
	Err() error
	// Detach stops the output. It is safe to call more than once.
	Detach()
}

// Backend is a session host.
type Backend interface {
	Create(Spec) (Info, error)
	List() []Info
	Attach(id string) (Attachment, error)
	// Write sends a client's input (keys, pastes, terminal replies) to the
	// program.
	Write(id string, input []byte) error
	Resize(id string, size Size) error
	// Prompt delivers an Action prompt to the agent as if the Operator had
	// pasted it, then submits it when submit is true.
	Prompt(id string, text string, submit bool) error
	// Close ends the session's program.
	Close(id string) error
	// Shutdown ends every session.
	Shutdown()
}

var (
	ErrNoSession   = errors.New("no such session")
	ErrUnknownKind = errors.New("unknown session kind")
	// ErrEnded closes an Attachment whose program ended.
	ErrEnded = errors.New("the session ended")
	// ErrTooSlow closes an Attachment that did not keep up with the output;
	// the client attaches again and gets a fresh snapshot.
	ErrTooSlow = errors.New("the client did not keep up with the output")
)
