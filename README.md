# booster

booster is an IDE for the agentic workflow: it hosts Claude Code and pi in
full terminals beside a classic IDE, and it opens projects, not folders. The
plan is in [docs/plan.md](docs/plan.md), the choice of each layer in
[docs/stack.md](docs/stack.md) and the domain language in
[CONTEXT.md](CONTEXT.md).

## Build and run

The build needs Go 1.26, Node 24 and, on Ubuntu 22.04, the GTK3 and WebKitGTK
4.1 headers (`libgtk-3-dev libwebkit2gtk-4.1-dev`): Wails' default GTK4
backend needs GTK 4.10 or later.

```sh
make            # bin/booster: the page built and embedded, the GTK3 window
make check      # gofmt, go vet and the race tests
```

One binary has every role. `booster` opens the window and starts the core
when none runs; closing the window leaves the core and its sessions running.

```sh
bin/booster                         # the window; -dir sets the folder for new sessions
bin/booster core                    # the core in the foreground
bin/booster list                    # the core's sessions
bin/booster new -kind claude -dir ~/Projects/Active/booster
bin/booster prompt <id> 'text'      # an Action prompt; -submit=false types it only
bin/booster close <id>
bin/booster stop                    # ends the core and every session
```

The core's socket, `core.json` (its TCP address and token) and `core.log`
are in `$XDG_RUNTIME_DIR/booster`.

## The code

| Path | What it holds |
|---|---|
| `main.go` | Embeds the built page and hands the command line to `internal/cli` |
| `internal/cli` | The commands |
| `internal/core` | The core: the HTTP/WebSocket API on a Unix socket (mode 0600) and on loopback TCP with a token, the client, the detached start |
| `internal/session` | `session.Backend`, what the core needs from a session host |
| `internal/session/ptyhost` | The PTY session host: each program on its own PTY, its output fed to an emulator (`charmbracelet/x/vt`) so a window that attaches later gets the screen |
| `internal/window` | The Wails window; its page talks to the core directly |
| `internal/webviewgpu` | Draws the webview on the integrated GPU of a hybrid laptop |
| `frontend` | The page: React, xterm.js |
| `spikes` | The platform and session spikes, kept as the record of Parts 1 and 2 |

## Known gaps

The restored screen matches xterm.js for a shell, Claude Code and pi (see
the session spike in [docs/stack.md](docs/stack.md)), with these gaps: on the
alternate screen the main screen's visible lines are not restored; soft-wrapped
lines come back as hard line ends; the pen, the character sets, the keypad
mode and the kitty keyboard flags are not restored; and the emulator, not
xterm.js, answers the queries a program sends before a window attaches. A
fullscreen agent (Claude Code with `"tui": "fullscreen"`, pi) keeps its own
history, so a search covers only its screen as it is.
