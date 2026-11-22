# Session spike

Part 2 of [the booster plan](../../docs/plan.md): can a session host in Go
keep Claude Code and pi alive while the window closes and opens again,
restore each screen on reattach, and deliver an Action prompt to each agent?

One binary has two roles. `session core` is the core: it owns each agent's
PTY, feeds every output byte to a terminal emulator (`charmbracelet/x/vt`)
and to the attached windows, and serves one HTTP/WebSocket API on two
listeners: a Unix socket (mode 0600) for local tools, and loopback TCP for the
window's page, which must carry the token from `core.json` (mode 0600) and
come from the page's origin. `session` (the window) starts a detached core
when none runs, then opens a Wails window whose page talks to the core
directly. Closing the window leaves the core and its sessions running.

The core keeps the host behind `session.Backend`; `session/ptyhost` is the
PTY implementation, and tmux could become a second one.

## Build and run

```sh
wails3 task build                                  # bin/session (GTK3 build)
bin/session -dir ~/Projects/Active/booster         # the window; starts the core
bin/session list                                   # the core's sessions
bin/session new -kind claude -dir ~/Projects/Active/booster
bin/session prompt <id> 'text'                     # an Action prompt; -submit=false types it only
bin/session stop                                   # ends the core and every session
```

The core's files (socket, `core.json`, `core.log`) are in
`$XDG_RUNTIME_DIR/booster-session-spike`.

## On attach

The core sends a `hello` with the session's size, then one snapshot: the
scrollback and the screen rendered as ANSI, the cursor, the terminal modes the
program set (bracketed paste, mouse tracking, application cursor keys, …) and
the title. Then the live output follows with no gap: the snapshot and the
subscription are taken under the same lock as the output. A window that falls
1024 chunks behind is dropped and attaches again with a fresh snapshot.

While no window is attached, the emulator answers the program's terminal
queries (cursor position, device attributes); while one is attached, xterm.js
answers.

## Action prompts

`prompt` writes the text as a bracketed paste when the program turned
bracketed paste on, waits 80 ms and sends Enter. A multi-line prompt to a
program without bracketed paste is refused, as it would submit at its first
line. Text already typed in the agent's input stays, and the prompt joins it.

## Comparing the restore with xterm.js

```sh
bin/session core -record /tmp/rec                  # records <id>.raw and <id>.json per session
bin/session replay -raw /tmp/rec/<id>.raw > /tmp/rec/<id>.snap
(cd compare && npm ci) && node compare/compare.mjs /tmp/rec/<id>.raw /tmp/rec/<id>.snap
```

`compare.mjs` feeds the recording to `@xterm/headless` (what a window attached
from the start shows) and the snapshot to a second one, and counts the lines,
styled cells, cursor, modes and title that differ. A control restores the same
recording through xterm.js's own serialize addon.

## Known gaps

- On the alternate screen the main screen's lines are not restored (the
  emulator hands out only the active screen); its scrollback is.
- A program on the alternate screen (Claude Code with `"tui": "fullscreen"`,
  pi) keeps its history itself, so the search covers only its screen as it is,
  and a match's mark stays on the cells while the program draws other text
  there.
- Soft-wrapped lines come back as hard line ends, so they do not reflow when
  the window resizes, and a search does not match across the wrap.
- The pen (the style of the next character), character sets, the keypad mode
  and the kitty keyboard flags are not restored.
- Start-up queries are answered by the emulator, because the program starts
  before a window attaches; its colours and device attributes are not
  xterm.js's.
