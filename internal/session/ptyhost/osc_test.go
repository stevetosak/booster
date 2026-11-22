package ptyhost

import (
	"strings"
	"testing"
)

func TestOSCFilter(t *testing.T) {
	cases := []struct {
		name, in, out, title string
	}{
		{"plain text", "hello", "hello", ""},
		{"UTF-8 title with 0x9C, BEL", "a\x1b]0;✳ Claude Code\x07b", "ab", "✳ Claude Code"},
		{"title with ';', ST", "\x1b]2;git log; ls\x1b\\b", "b", "git log; ls"},
		{"icon name dropped", "\x1b]1;icon\x07b", "b", ""},
		{"hyperlink passes", "\x1b]8;;http://x\x07L\x1b]8;;\x07", "\x1b]8;;http://x\x07L\x1b]8;;\x07", ""},
		{"other OSC with 0x9C dropped", "\x1b]9;✳ done\x07b", "b", ""},
		{"CSI passes", "\x1b[31mred\x1b[m", "\x1b[31mred\x1b[m", ""},
		{"double ESC", "\x1b\x1b[m", "\x1b\x1b[m", ""},
		{"aborted OSC passes as it came", "\x1b]2;x\x1b[mz", "\x1b]2;x\x1b[mz", ""},
	}
	for _, c := range cases {
		// Every split of the input must give the same result.
		for cut := 0; cut <= len(c.in); cut++ {
			var title string
			f := oscFilter{onTitle: func(s string) { title = s }}
			out := string(f.filter([]byte(c.in[:cut]))) + string(f.filter([]byte(c.in[cut:])))
			if out != c.out || title != c.title {
				t.Errorf("%s, cut at %d: out %q title %q; want %q %q", c.name, cut, out, title, c.out, c.title)
				break
			}
		}
	}
}

// TestTitleDoesNotReachScreen checks the whole path: a UTF-8 title neither
// prints onto the emulator's screen nor gets lost.
func TestTitleDoesNotReachScreen(t *testing.T) {
	scr := newScreen(40, 3, 100)
	go func() { _, _ = scr.emu.Read(make([]byte, 64)) }()
	scr.write([]byte("\x1b]0;✳ Claude Code\x07ready"))
	if got := scr.emu.String(); strings.Contains(got, "Claude") {
		t.Errorf("the title reached the screen: %q", got)
	}
	if !strings.Contains(string(scr.snapshot()), "\x1b]2;✳ Claude Code\x07") {
		t.Errorf("the snapshot lost the title: %q", scr.snapshot())
	}
}
