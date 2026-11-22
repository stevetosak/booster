package ptyhost

import (
	"crypto/rand"
	"encoding/hex"
	"slices"
	"strings"
)

// Program is the command line of one session kind. It is data, so a new
// agent, or the Operator's own agent configuration, needs no new code.
type Program struct {
	Argv []string
	// Start follows Argv for a new session of an agent with its own session
	// id, and Resume follows it after a core restart (Start when Resume is
	// empty); "{id}" in them is that id. A program with no Start (a shell)
	// has no id and starts plain again.
	Start, Resume []string
}

const idArg = "{id}"

func (p Program) hasSession() bool { return len(p.Start) > 0 }

// command is the command line of a session whose agent has the session id
// agent; resume picks the Resume arguments.
func (p Program) command(agent string, resume bool) []string {
	extra := p.Start
	if resume && len(p.Resume) > 0 {
		extra = p.Resume
	}
	argv := slices.Clone(p.Argv)
	for _, a := range extra {
		argv = append(argv, strings.ReplaceAll(a, idArg, agent))
	}
	return argv
}

// newAgentID is a random UUID (version 4), the form Claude Code needs.
func newAgentID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // RFC 4122 variant
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// newID is booster's own id of a session.
func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
