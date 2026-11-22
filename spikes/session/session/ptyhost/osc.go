package ptyhost

import (
	"bytes"
	"strconv"
)

// maxOSC bounds a held OSC string; a longer one passes through unread.
const maxOSC = 64 * 1024

// oscFilter sits between the program's output and the emulator. x/vt
// v0.1.0 (with x/ansi up to v0.11.10) mishandles two things in OSC strings:
// it ends a string at byte 0x9C (the 8-bit String Terminator), which is also
// a UTF-8 continuation byte, so "✳ Claude Code" ends after one byte and the
// rest prints onto the screen; and it ignores a title with a ';' in it. The
// filter takes the titles (OSC 0, 1, 2) out of the stream and reads them
// itself, drops any other OSC string with a 0x9C byte, and passes every other
// byte through unchanged. It keeps its state across writes, so a string may
// arrive in pieces.
type oscFilter struct {
	state   oscState
	buf     []byte // the held OSC string, from ESC ] on
	onTitle func(string)
}

type oscState int

const (
	ground oscState = iota
	sawEsc          // ESC seen in ground
	inOSC           // inside ESC ] …
	oscEsc          // ESC seen inside an OSC string: ST, or an abort
)

// filter returns the bytes of p that the emulator should get.
func (f *oscFilter) filter(p []byte) []byte {
	out := make([]byte, 0, len(p))
	for _, b := range p {
		switch f.state {
		case ground:
			if b == 0x1b {
				f.state = sawEsc
				continue
			}
			out = append(out, b)
		case sawEsc:
			if b == ']' {
				f.state = inOSC
				f.buf = append(f.buf[:0], 0x1b, ']')
				continue
			}
			out = append(out, 0x1b)
			if b == 0x1b {
				continue // stay in sawEsc for this ESC
			}
			out = append(out, b)
			f.state = ground
		case inOSC:
			switch {
			case b == 0x07:
				out = f.finish(out, []byte{0x07})
			case b == 0x1b:
				f.state = oscEsc
			case len(f.buf) >= maxOSC:
				out = append(append(out, f.buf...), b)
				f.state = ground
			default:
				f.buf = append(f.buf, b)
			}
		case oscEsc:
			if b == '\\' {
				out = f.finish(out, []byte{0x1b, '\\'})
				continue
			}
			// An ESC other than ST aborts the string; pass it on as it came
			// and read the ESC again.
			out = append(out, f.buf...)
			f.state = sawEsc
			if b == ']' {
				f.state = inOSC
				f.buf = append(f.buf[:0], 0x1b, ']')
				continue
			}
			out = append(out, 0x1b)
			if b == 0x1b {
				continue
			}
			out = append(out, b)
			f.state = ground
		}
	}
	return out
}

// finish handles one whole OSC string held in buf.
func (f *oscFilter) finish(out, terminator []byte) []byte {
	f.state = ground
	body := f.buf[2:]
	cmd, text, _ := bytes.Cut(body, []byte{';'})
	switch n, err := strconv.Atoi(string(cmd)); {
	case err == nil && (n == 0 || n == 2):
		if f.onTitle != nil {
			f.onTitle(string(text))
		}
		return out
	case err == nil && n == 1:
		return out // the icon name; nothing shows it
	case bytes.IndexByte(body, 0x9c) >= 0:
		return out // the emulator would cut it at the 0x9C
	}
	out = append(out, f.buf...)
	return append(out, terminator...)
}
