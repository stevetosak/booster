---
title: "booster: the stack"
description: "Options and the choice for each layer of the app: the webview, how much Code-OSS to build on, the editor, the terminal, the core, the UI framework and the classic IDE services."
type: design
status: proposed
topics: [ide, stack]
---

booster is its own desktop app on Wails 3 with a Go core in the same binary,
a React frontend, and an own session host (decisions of 2026-11-19). This
page records how each layer was weighed. Versions were read from the npm
registry, crates.io and the Go module proxy on 2026-11-19.

## The shape

```
shell (Wails 3, Go, thin)         windows, menus, native dialogs, starts the core
  └─ web frontend (TypeScript)    the IDE: layout, tool windows, editor, terminals, Board
        │  one socket protocol
booster core (Go, same binary)    Projects, session host, Actions, config, analytics,
                                  Board, git, search, LSP and DAP brokering
  └─ agent PTYs, language servers, debug adapters, git, ripgrep
```

The core outlives the window, because it owns the agent sessions. The
frontend reaches native features through one `Platform` interface, so the
webview runtime (Tauri with WebKitGTK, Tauri with CEF, or Electron) is a
replaceable adapter and not a property of the IDE code.

## The webview on Linux

Tauri 2 (2.12.7 stable) uses the system webview: WebKitGTK on Linux, WebView2
on Windows, WKWebView on macOS. Tauri's own Linux graphics page says WebGL and
canvas can "silently land on a slow path", seen as "high input latency or low
frame rates in WebGL heavy views (terminal emulators, editors …)", most often
on NVIDIA, and WebKitGTK hides the real renderer, so the app cannot detect it.
Its workarounds (`WEBKIT_DISABLE_DMABUF_RENDERER=1`, then disabling
compositing) give up the fast path. The Operator's laptop runs WebKitGTK
2.50.4 on X11, with the internal panel on the AMD GPU and the external
monitor on the NVIDIA GPU.

Full terminals are the condition of the GUI decision, so this is the first
risk to measure. If WebKitGTK fails it, two fallbacks keep the frontend as it
is: Tauri 3's CEF runtime (`tauri-runtime-cef` 3.0.0-alpha.7, alpha; it ships
Chromium, hundreds of MB larger), or Electron (45.4.1), which ships Chromium
and Node together.

### Measured on the laptop (2026-11-22)

The platform spike (`spikes/platform`, `grid.sh`) ran xterm.js 6.0.0 in Wails
3.1 with the GTK3 build (WebKitGTK 2.50.4) on both monitors, with the WebGL
and the DOM renderer, in four GL modes. WebKit hides the GPU from the page
(WebGL reports "Apple GPU"), so the GPU was read from the driver libraries the
web process maps and from `nvidia-smi`. That showed the first surprise: by
default the web process renders on the NVIDIA GPU, because glvnd tries
NVIDIA's EGL before Mesa's, and Wails turns the DMA-BUF renderer off on X11
whenever the nvidia module is loaded. Only `__EGL_VENDOR_LIBRARY_FILENAMES`
set to Mesa's file makes the AMD GPU render, and then Wails keeps DMA-BUF on.

WebGL, one run per cell, the laptop panel / the external monitor:

| GL mode | Renders on | DMA-BUF | Plain text MB/s | Coloured text MB/s | Typing beside a 2 MB/s stream, ms p50 / p95 |
|---|---|---|---|---|---|
| default | NVIDIA | off | 11.6 / 16.6 | 15.0 / 14.1 | 26 / 40, 26 / 39 |
| DMA-BUF kept on | NVIDIA | on | 22.9 / 12.0 | 11.9 / 8.1 | 24 / 33, 24 / 33 |
| PRIME offload | NVIDIA | off | 18.9 / 19.3 | 14.7 / 14.9 | 26 / 35, 26 / 42 |
| Mesa only | AMD | on | 22.4 / 22.5 | 11.9 / 11.3 | 24 / 32, 24 / 33 |

WebGL never fell to a software path. Typing with no other output takes 24–25
ms at p50 and 32–33 ms at p95 in every cell, from the key to the frame after
the echo is drawn: about two frames at 60 Hz. The measurement starts at
xterm.js's input, so the path from the keyboard through GTK and WebKit is not
in it. Beside a streaming terminal WebGL keeps that latency, while the DOM
renderer climbs to 29–84 ms at p50 and 58–133 ms at p95, so the terminal needs
WebGL, and WebGL works on both GPUs. Throughput is bound by xterm.js's parser
on the page's one thread; single runs differ up to two times between the
monitors, and every value is far above an agent's output. The AMD path is as
fast to type on and keeps the NVIDIA GPU idle, but it drops more frames below
20 per second beside a stream (6–12 frames over 50 ms in a ten-second test,
against 1–5 on NVIDIA). The app and WebKit's processes use 206–247 MB (PSS)
idle and 312–375 MB with two terminals, one of them streaming.

These numbers end where the page has drawn a frame, not where the screen shows
it, and a continuous stream hides the difference. The Operator's own check
found it: when the terminal is otherwise idle, the window shows each update
one update late, so a typed key appears only with the next key. A trace showed
the keys reach the program once and in order, and the page parses and draws
each echo within 20 ms; the delay is between WebKit's web process and the
window. The cause is the DMA-BUF renderer turned off inside a Wails window:
Wails turns it off itself on X11 whenever the nvidia module is loaded. With
`WEBKIT_DISABLE_DMABUF_RENDERER=0`, or with EGL limited to Mesa (the AMD GPU,
where Wails keeps DMA-BUF on), the delay is gone; a plain WebKitGTK window
(`webkitcheck`) shows no delay with DMA-BUF on or off. So booster keeps the
DMA-BUF renderer on and does not leave that choice to Wails: on a hybrid
laptop it limits EGL to Mesa before Wails starts, so the integrated GPU draws
and the discrete one stays idle (decision of 2026-11-22). Whether the WebGL
canvas or Wails' window setup is the second condition is open.

## Tauri or Wails

Wails is the Go counterpart of Tauri: a Go backend, the system webview, and
generated TypeScript bindings for Go methods. Wails v3 became stable on
2026-10-26 (3.1.0 on 2026-11-01) with multiple windows, services, typed
events, menus, a tray and an MCP service that lets an agent drive and inspect
a running app. On Linux it uses GTK4 and WebKitGTK 6.0 by default; the GTK3
stack was kept behind a build tag "until v3.1". It has no Chromium runtime.
The GTK4 backend needs GTK 4.10+ (`GtkFileDialog`, `gdk_monitor_get_scale`)
and does not build on Ubuntu 22.04 (GTK 4.6.9), so booster builds with
`-tags gtk3` there; the tag may go in a later Wails release.

| | Tauri 2 | Wails 3 |
|---|---|---|
| Shell language | Rust | Go |
| Linux webview | WebKitGTK, GTK3 API | WebKitGTK, GTK4 API (webkitgtk-6.0) |
| Chromium fallback | CEF runtime in Tauri 3 (alpha) | None; Electron only |
| Maturity | Stable since 2024, large plugin set | Stable for three weeks |
| On the laptop | Rust not installed | Go 1.26.2 installed (v3 needs 1.26+); `libwebkitgtk-6.0-4` 2.50.4 available, GTK 4.6.9 |

Both run the same WebKit engine on Linux (2.50.4 on the laptop for either
API), so the terminal risk below is shared; the GTK4 path renders through a
different toolkit, which can change the result in either direction. With a
thin shell behind the `Platform` interface and the core in its own process,
the shell language carries little code, so the choice between them is cheap
to reverse. The difference that counts is the fallback: Tauri keeps a
Chromium path inside the same framework, Wails does not.

Wails 3 is the choice (2026-11-22): with the webview on the integrated GPU
and the DMA-BUF renderer on, Claude Code and pi run in the spike window on
both monitors with no lag, so the fallbacks were not measured.

## How much Code-OSS

| Level | What it means | Gain | Cost |
|---|---|---|---|
| 1. Fork Code-OSS | The Cursor path, on Electron | Everything works on day one | Electron, not Tauri; the VS Code look; a monthly merge of a very large code base |
| 2. The Code-OSS workbench in Tauri | SideX, vscodeee, or the workbench through `monaco-vscode-api` | The whole workbench, the extension UI, SCM, search | Every project is early (SideX: "the extension host and debugger are still in progress"); the extension host needs a Node sidecar; booster's own UI must fit the workbench model; the VS Code look stays |
| 3. Code-OSS parts as libraries | Own shell and layout; Monaco as the editor; chosen services from `@codingame/monaco-vscode-api` | VS Code's editor quality: diff editor, peek, inlay hints, code lens, find, multi-cursor; TextMate grammars and themes; LSP wiring that maps 1:1 | Monaco is large and has VS Code's own style of extension points; the shell, tool windows and debugger UI are booster's own work |
| 4. No Code-OSS | Own shell with CodeMirror 6 | Small, modular, simple to style; own LSP glue | Diff, peek, inlay hints, code lens and the language wiring are own work |

Level 3 is the decision (2026-11-22). The JetBrains feel comes from the shell (tool
windows, Search Everywhere, keymaps, the debugger and run windows), not from
the editor widget, so the shell is booster's own; the editor is where VS
Code's ten years of work are worth taking as they are. Monaco's diff editor
is also the natural base for the later review Plan. `@codingame/monaco-vscode-api`
(39.0.5, released 2026-11-18) is active and follows VS Code's releases; it is
adopted service by service (grammars, themes and languages first), never the
whole workbench.

## The terminal

xterm.js (6.0.0) is the terminal VS Code uses, and `@xterm/headless` with the
serialize addon is the same emulator without a screen, which makes the session
host's screen restore exact. Its fast renderer is WebGL, which is the slow
path above; the platform spike measures the WebGL and DOM renderers on both
GPUs.

## The core

| Option | Gain | Cost |
|---|---|---|
| Go, the same binary as a Wails shell (`booster core`) | The Operator's preferred language; one binary to ship, no runtime; `creack/pty` (1.1.24) and `charmbracelet/x/vt` (0.1.0, with `Render()` of the screen as ANSI and scrollback access) | The emulator is young and differs from xterm.js, so a restored screen may differ in details; scrollback-to-ANSI is own code; kalfa is used through its CLI, not its code |
| Node 24 service in TypeScript | `node-pty` (1.1.0) with `@xterm/headless`: the same emulator on both sides, exact restore with the serialize addon; kalfa and symbiosis code reusable | A second language beside the shell; a Node runtime to ship |
| Rust inside or beside Tauri | One language with a Tauri shell; `portable-pty` (0.9.0), `alacritty_terminal` (0.26.0) | Own serialization from a different emulator; not the preferred language |

Go is chosen: the Operator prefers it, and one binary is simpler to
ship once booster is shared. Only the live bytes must be exact, and they pass
through raw; the emulator serves only the restore, which the session spike
compares against `@xterm/headless`. TypeScript stays where the host decides
it: the frontend and the pi extension.

### Measured: the session spike (2026-11-22)

The session spike (`spikes/session`) runs the core as its own process that the
window starts detached; closing the window leaves the core and its sessions
running, and a new window attaches to the same sessions. Every output byte
goes to `x/vt` and to the attached windows under one lock, so a snapshot and
the output after it neither overlap nor leave a gap.

The restore was compared with xterm.js on the same recorded output
(`compare/compare.mjs`, `@xterm/headless` 6.0.0). For a shell with 453 lines,
417 of them scrollback, and for Claude Code and pi on the alternate screen,
the restored text, the styled cells, the cursor, the modes and the title all
match; xterm.js's own serialize addon, as a control, differed in 9 styled
cells of Claude Code's screen. The snapshot is smaller than the output it
replaces (34 KB for 39 KB of shell output). Soft wraps come back as hard line
ends, so restored lines do not reflow on a resize.

Two faults of `x/vt` 0.1.0 needed a filter in front of the emulator. Its
parser (`x/ansi` up to 0.11.10) ends an OSC string at byte 0x9C, the 8-bit
String Terminator, which is also a UTF-8 continuation byte: Claude Code's
title `✳ Claude Code` ends after one byte and the rest prints onto the
emulator's screen, where Claude Code, which redraws only the cells it
changed, never clears it. The title handler also drops any title with a `;`.
The core reads the titles itself and drops other OSC strings with a 0x9C byte
before the emulator sees them; the windows get the bytes unchanged. Separately,
`Emulator.Close` races with `Read`, so the core closes the reply pipe instead.

With the Operator's `"tui": "fullscreen"`, Claude Code draws on the alternate
screen, like pi, so its history scrolls inside Claude Code and xterm.js has no
scrollback to search for it. In the Operator's check, a search in a shell
found a line restored from the scrollback; in Claude Code the first match was
marked on the right text and the next ones on other text, because the search
marks cells while the agent scrolls by drawing its rows again. A search over
an agent's output needs the agent's own transcript. The agents send their
start-up queries (device attributes, the kitty keyboard flags, pi's colour
queries) before a window attaches, so the emulator, not xterm.js, answers
them.

An Action prompt is written as a bracketed paste and submitted with Enter 80
ms later; a multi-line prompt arrived whole in Claude Code's and pi's input.
Text the Operator has already typed there stays, and the prompt joins it.

## The classic IDE services

- **Go to definition and code intelligence:** language servers started by the
  core, wired to Monaco through `monaco-languageclient`; the symbiosis
  tree-sitter outlines serve as a fallback where no server runs.
- **Debugger:** the Debug Adapter Protocol, with the adapters VS Code uses
  (js-debug, debugpy, CodeLLDB, netcoredbg), and an own debugger tool window.
- **Git:** the `git` CLI, as JetBrains and VS Code use it, so hooks, config and
  credentials behave as in a terminal; `gix` (0.90.0) only if status becomes
  slow on large repositories.
- **Search:** ripgrep for text; a fuzzy matcher for Search Everywhere.
- **Run configurations:** an own model, with symbiosis's launch.json import.

## The UI framework

React with TypeScript, chosen over Solid and Svelte 5 for its ecosystem
(docking layouts, virtualised lists and trees for an IDE shell). The editor
and the terminal are imperative widgets that React mounts and does not
re-render.
