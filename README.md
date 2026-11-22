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
make            # bin/booster, the dev instance: the page built and embedded, the GTK3 window
make install    # ~/.local/bin/booster, the main instance: the daily booster
make check      # gofmt, go vet and the race tests
```

One binary has every role. `booster` opens the window and starts the core
when none runs; closing the window leaves the core and its sessions running.

Each build belongs to an instance with its own core and folders: the one
`make install` builds is the main instance, every other build is the dev
instance, so a booster built while working on booster never reaches the
daily core. `BOOSTER_INSTANCE=<name>` names another instance for one run.

```sh
bin/booster                         # the window; -dir sets the folder for new sessions
bin/booster core                    # the core in the foreground
bin/booster list                    # the core's sessions
bin/booster new -kind claude -dir ~/Projects/Active/booster
bin/booster prompt <id> 'text'      # an Action prompt; -submit=false types it only
bin/booster close <id>
bin/booster stop                    # ends the core and every session
bin/booster project import          # each repository in ~/Projects/Active once; -dir sets another folder
bin/booster project list            # each Project's worktrees, branches and kalfa project keys; -json
bin/booster project add [folder]    # the repository around a folder, by its main checkout; -name sets the name
bin/booster project remove <name>   # off the list; the files stay
```

The main instance's folders are named `booster`, another instance's
`booster-<instance>`. The core's socket, `core.json` (its TCP address and
token) and `core.log` are in `$XDG_RUNTIME_DIR/booster`. The core keeps the
Projects in `$XDG_CONFIG_HOME/booster/projects.json` (each name and main
checkout, and when the import last ran) and reads it again on each request,
so an edit by hand counts at once. The worktrees come from `git worktree list`, and each
worktree's kalfa project key is computed as kalfa computes it.

The agents end with the core, so the core keeps the session list in
`$XDG_STATE_HOME/booster/sessions.json` and the next core starts each session
again with the same id: Claude Code with `--resume` and pi with
`--session-id` on the agent session id the core gave it at start, a shell new
in its folder. A Claude Code session that got no message has nothing to
resume and starts new with the same id. A session the Operator closes, or
whose program ends, leaves the list.

## The code

| Path | What it holds |
|---|---|
| `main.go` | Embeds the built page and hands the command line to `internal/cli` |
| `internal/cli` | The commands |
| `internal/core` | The core: the HTTP/WebSocket API on a Unix socket (mode 0600) and on loopback TCP with a token, the client, the detached start |
| `internal/project` | The Project registry, the worktrees from git, the kalfa project key, the import |
| `internal/gittest` | Git repositories for tests, with git cut off from the user's configuration |
| `internal/instance` | The instance and its runtime, state and config folders |
| `internal/jsonfile` | Small JSON files, written whole |
| `internal/session` | `session.Backend`, what the core needs from a session host |
| `internal/session/ptyhost` | The PTY session host: each program on its own PTY, its output fed to an emulator (`charmbracelet/x/vt`) so a window that attaches later gets the screen; the saved session list and the resume after a core restart |
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
