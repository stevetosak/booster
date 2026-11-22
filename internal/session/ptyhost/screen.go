package ptyhost

import (
	"fmt"
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

// screen keeps what a client needs to show a session it attaches to: the
// emulator's cells, and the state the emulator does not hand out (the terminal
// modes and the title). It is not safe for concurrent use; the session's lock
// guards it.
type screen struct {
	emu   *vt.Emulator
	osc   oscFilter
	modes map[modeKey]bool
	order []modeKey // modes in the order the program first set them
	title string
}

type modeKey struct {
	private bool // a DEC private mode (CSI ? n h), else an ANSI mode (CSI n h)
	n       int
}

// notReplayed are modes the snapshot sets in its own way or leaves out:
// the alternate screen (47, 1047, 1049) and the cursor (25) come from the
// emulator's state; saving the cursor (1048) has no state to restore;
// synchronized output (2026) would hold the client's drawing until the next
// frame ends.
var notReplayed = map[modeKey]bool{
	{true, 47}: true, {true, 1047}: true, {true, 1049}: true,
	{true, 25}: true, {true, 1048}: true, {true, 2026}: true,
}

func newScreen(cols, rows, scrollback int) *screen {
	s := &screen{
		emu:   vt.NewEmulator(cols, rows),
		modes: map[modeKey]bool{},
	}
	s.emu.SetScrollbackSize(scrollback)
	s.emu.SetCallbacks(vt.Callbacks{
		EnableMode:  func(m ansi.Mode) { s.setMode(m, true) },
		DisableMode: func(m ansi.Mode) { s.setMode(m, false) },
	})
	s.osc.onTitle = func(t string) { s.title = t }
	return s
}

// write feeds the program's output to the emulator, titles taken out (see
// oscFilter).
func (s *screen) write(p []byte) {
	_, _ = s.emu.Write(s.osc.filter(p))
}

func (s *screen) setMode(m ansi.Mode, on bool) {
	k := modeKey{n: m.Mode()}
	if _, ok := m.(ansi.DECMode); ok {
		k.private = true
	}
	if _, seen := s.modes[k]; !seen {
		s.order = append(s.order, k)
	}
	s.modes[k] = on
}

// bracketedPaste reports whether the program asked for pastes in brackets.
func (s *screen) bracketedPaste() bool { return s.modes[modeKey{true, 2004}] }

// snapshot renders the scrollback, the screen, the cursor, the modes and the
// title as one ANSI stream. Written into an empty terminal of the same size,
// it scrolls the scrollback lines up into the terminal's own scrollback and
// leaves the screen as the emulator has it.
//
// Known gaps: on the alternate screen the main screen's visible
// lines are lost (the emulator hands out only the active screen); soft
// wraps become hard line ends; the pen (the style of the next character),
// the character sets and the keypad and keyboard protocol modes are not
// restored.
func (s *screen) snapshot() []byte {
	var b strings.Builder
	sb := s.emu.Scrollback()
	lines := 0
	writeLine := func(l uv.Line) {
		if lines > 0 {
			b.WriteString("\r\n")
		}
		b.WriteString(l.Render())
		lines++
	}
	if sb != nil {
		for _, l := range sb.Lines() {
			writeLine(l)
		}
	}
	if s.emu.IsAltScreen() {
		// The scrollback belongs to the main screen; the alternate screen
		// starts clear and is drawn below.
		b.WriteString("\x1b[?1049h\x1b[H")
		lines = 0
	}
	w, h := s.emu.Width(), s.emu.Height()
	for y := 0; y < h; y++ {
		l := make(uv.Line, w)
		for x := 0; x < w; x++ {
			if c := s.emu.CellAt(x, y); c != nil {
				l[x] = *c
			}
		}
		writeLine(l)
	}

	cur := s.emu.CursorPosition()
	fmt.Fprintf(&b, "\x1b[%d;%dH", cur.Y+1, cur.X+1)
	if s.emu.CursorHidden() {
		b.WriteString("\x1b[?25l")
	}
	for _, k := range s.order {
		if notReplayed[k] {
			continue
		}
		b.WriteString(modeSequence(k, s.modes[k]))
	}
	if s.title != "" {
		fmt.Fprintf(&b, "\x1b]2;%s\x07", s.title)
	}
	return []byte(b.String())
}

func modeSequence(k modeKey, on bool) string {
	q, f := "", "l"
	if k.private {
		q = "?"
	}
	if on {
		f = "h"
	}
	return fmt.Sprintf("\x1b[%s%d%s", q, k.n, f)
}
